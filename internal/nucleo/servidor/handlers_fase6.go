package servidor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/auto_creacion"
	"github.com/gorilla/mux"
)

// ============================================================================
// Fase 6: Handlers de Auto-Creación de Herramientas
// ============================================================================

// ConAutoGestor inyecta el gestor de auto-creación en el servidor.
// Debe llamarse antes de Iniciar().
func (s *Servidor) ConAutoGestor(g *auto_creacion.Gestor) *Servidor {
	s.autoGestor = g
	return s
}

// requiereAutoGestor verifica que el gestor de auto-creación esté disponible.
func (s *Servidor) requiereAutoGestor(w http.ResponseWriter) bool {
	if s.autoGestor == nil {
		s.responderError(w, http.StatusServiceUnavailable,
			"gestor de auto-creación no disponible (Fase 6 no inicializada)")
		return false
	}
	return true
}

// registrarRutasFase6 registra todos los endpoints de auto-creación.
// Se llama desde registrarRutas().
//
// Orden: rutas más específicas primero para que {nombre} no las capture.
func (s *Servidor) registrarRutasFase6() {
	// Operaciones globales (sin {nombre})
	s.router.HandleFunc("/api/v1/herramientas/auto-crear", s.handlerAutoCrear).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/v1/herramientas/detectar", s.handlerAutoDetectar).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/v1/herramientas/auto-creadas", s.handlerAutoCreadasListar).Methods("GET", "OPTIONS")

	// Operaciones específicas con {nombre}
	s.router.HandleFunc("/api/v1/herramientas/auto-creadas/{nombre}/probar", s.handlerAutoCreadasProbar).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/v1/herramientas/auto-creadas/{nombre}/recargar", s.handlerAutoCreadasRecargar).Methods("POST", "OPTIONS")
	s.router.HandleFunc("/api/v1/herramientas/auto-creadas/{nombre}/fuente", s.handlerAutoCreadasFuente).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/v1/herramientas/auto-creadas/{nombre}/log", s.handlerAutoCreadasLog).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/v1/herramientas/auto-creadas/{nombre}", s.handlerAutoCreadasInfo).Methods("GET", "OPTIONS")
	s.router.HandleFunc("/api/v1/herramientas/auto-creadas/{nombre}", s.handlerAutoCreadasEliminar).Methods("DELETE", "OPTIONS")
}

// ============================================================================
// POST /api/v1/herramientas/auto-crear
// ============================================================================

// BodyAutoCrear es el body para POST /api/v1/herramientas/auto-crear.
type BodyAutoCrear struct {
	// Descripcion en lenguaje natural de lo que el usuario quiere lograr.
	// Requerido si no se usa ForzarSpec/ForzarNombre.
	Descripcion string `json:"descripcion,omitempty"`

	// ForzarNombre permite crear una herramienta con nombre específico
	// sin pasar por el detector. Útil para tests o creación manual.
	ForzarNombre string `json:"forzar_nombre,omitempty"`

	// ForzarSpec permite pasar la spec completa y saltarse el detector.
	ForzarSpec *auto_creacion.SpecHerramienta `json:"forzar_spec,omitempty"`

	// TimeoutSegundos limita la duración total del flujo (default 120s).
	TimeoutSegundos int `json:"timeout_segundos,omitempty"`
}

// handlerAutoCrear ejecuta el flujo completo de auto-creación.
//
// POST /api/v1/herramientas/auto-crear
// Body: {
//   "descripcion": "Comprime archivos CSV y envíalos por SFTP",
//   "forzar_nombre": "compresor_csv",  // opcional, salta detector
//   "forzar_spec": { ... }              // opcional, salta detector
// }
//
// Respuesta:
//   201 Created — herramienta creada exitosamente
//   400 Bad Request — body inválido o sin descripción
//   422 Unprocessable Entity — alguna etapa falló (compilación, carga, etc.)
//   503 Service Unavailable — gestor no inicializado
func (s *Servidor) handlerAutoCrear(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	var req BodyAutoCrear
	if err := parsearBody(r, &req); err != nil {
		s.responderError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	// Validar que tenga al menos una forma de determinar la spec
	if req.Descripcion == "" && req.ForzarNombre == "" && req.ForzarSpec == nil {
		s.responderError(w, http.StatusBadRequest,
			"se requiere 'descripcion', 'forzar_nombre' o 'forzar_spec'")
		return
	}

	// Construir catálogo actual para el detector
	var catalogoActual []auto_creacion.InfoCatalogo
	if s.catalogo != nil {
		for _, h := range s.catalogo.Snapshot() {
			catalogoActual = append(catalogoActual, auto_creacion.InfoCatalogo{
				Nombre: h.Nombre, Descripcion: h.Descripcion,
			})
		}
	}

	// Timeout
	timeout := 120 * time.Second
	if req.TimeoutSegundos > 0 {
		timeout = time.Duration(req.TimeoutSegundos) * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	sol := auto_creacion.SolicitudCreacion{
		Descripcion:    req.Descripcion,
		CatalogoActual: catalogoActual,
		ForzarNombre:   req.ForzarNombre,
		ForzarSpec:     req.ForzarSpec,
	}

	s.log.Info("[fase6] auto-crear: iniciando flujo")

	resultado, err := s.autoGestor.Crear(ctx, sol)
	if err != nil {
		s.log.Warn("[fase6] auto-crear falló: %v", err)
		// Si hay resultado parcial, devolverlo con error 422
		status := http.StatusUnprocessableEntity
		if resultado == nil {
			s.responderError(w, status, err.Error())
			return
		}
		s.responderJSON(w, status, RespuestaAPI{
			Exito:     false,
			Error:     err.Error(),
			Datos:     resultado,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	if !resultado.Registrada {
		s.log.Warn("[fase6] auto-crear: herramienta no registrada: %s", resultado.Error)
		s.responderJSON(w, http.StatusUnprocessableEntity, RespuestaAPI{
			Exito:     false,
			Error:     resultado.Error,
			Datos:     resultado,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	s.log.Info("[fase6] auto-crear OK: %s", resultado.Especificacion.Nombre)

	s.responderJSON(w, http.StatusCreated, RespuestaAPI{
		Exito:     true,
		Mensaje:   fmt.Sprintf("herramienta '%s' creada y registrada", resultado.Especificacion.Nombre),
		Datos:     resultado,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// POST /api/v1/herramientas/detectar
// ============================================================================

// BodyAutoDetectar es el body para POST /api/v1/herramientas/detectar.
type BodyAutoDetectar struct {
	Descripcion string `json:"descripcion"`
}

// handlerAutoDetectar analiza una petición y devuelve qué herramientas faltan,
// SIN crearlas. Útil para preview antes de confirmar creación.
//
// POST /api/v1/herramientas/detectar
// Body: {"descripcion": "..."}
func (s *Servidor) handlerAutoDetectar(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	var req BodyAutoDetectar
	if err := parsearBody(r, &req); err != nil {
		s.responderError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}
	if req.Descripcion == "" {
		s.responderError(w, http.StatusBadRequest, "campo 'descripcion' requerido")
		return
	}

	if !s.autoGestor.LLMDisponible() {
		s.responderError(w, http.StatusServiceUnavailable,
			"LLM no disponible (orquestador NVIDIA no inicializado)")
		return
	}

	// Construir catálogo actual
	var catalogoActual []auto_creacion.InfoCatalogo
	if s.catalogo != nil {
		for _, h := range s.catalogo.Snapshot() {
			catalogoActual = append(catalogoActual, auto_creacion.InfoCatalogo{
				Nombre: h.Nombre, Descripcion: h.Descripcion,
			})
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resultado, err := s.autoGestor.Detector().Detectar(ctx, req.Descripcion, catalogoActual)
	if err != nil {
		s.responderError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	s.responderJSON(w, http.StatusOK, RespuestaAPI{
		Exito:     true,
		Mensaje:   fmt.Sprintf("%d herramientas faltantes detectadas", len(resultado.Faltantes)),
		Datos:     resultado,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// GET /api/v1/herramientas/auto-creadas
// ============================================================================

// handlerAutoCreadasListar lista todas las herramientas auto-creadas.
//
// GET /api/v1/herramientas/auto-creadas
func (s *Servidor) handlerAutoCreadasListar(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	metas, err := s.autoGestor.Listar()
	if err != nil {
		s.responderError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.responderJSON(w, http.StatusOK, RespuestaAPI{
		Exito:     true,
		Mensaje:   fmt.Sprintf("%d herramientas auto-creadas", len(metas)),
		Datos:     metas,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// GET /api/v1/herramientas/auto-creadas/{nombre}
// ============================================================================

// handlerAutoCreadasInfo retorna la metadata de una herramienta específica.
//
// GET /api/v1/herramientas/auto-creadas/{nombre}
func (s *Servidor) handlerAutoCreadasInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	nombre := mux.Vars(r)["nombre"]
	if nombre == "" {
		s.responderError(w, http.StatusBadRequest, "nombre requerido")
		return
	}

	meta, err := s.autoGestor.Obtener(nombre)
	if err != nil {
		s.responderError(w, http.StatusNotFound, err.Error())
		return
	}

	s.responderJSON(w, http.StatusOK, RespuestaAPI{
		Exito:     true,
		Datos:     meta,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// DELETE /api/v1/herramientas/auto-creadas/{nombre}
// ============================================================================

// handlerAutoCreadasEliminar elimina una herramienta auto-creada.
//
// DELETE /api/v1/herramientas/auto-creadas/{nombre}
func (s *Servidor) handlerAutoCreadasEliminar(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	nombre := mux.Vars(r)["nombre"]
	if nombre == "" {
		s.responderError(w, http.StatusBadRequest, "nombre requerido")
		return
	}

	if err := s.autoGestor.Eliminar(nombre); err != nil {
		s.responderError(w, http.StatusNotFound, err.Error())
		return
	}

	s.log.Info("[fase6] herramienta eliminada: %s", nombre)

	s.responderJSON(w, http.StatusOK, RespuestaAPI{
		Exito:     true,
		Mensaje:   fmt.Sprintf("herramienta '%s' eliminada", nombre),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// POST /api/v1/herramientas/auto-creadas/{nombre}/probar
// ============================================================================

// BodyAutoProbar es el body para POST /api/v1/herramientas/auto-creadas/{nombre}/probar.
type BodyAutoProbar struct {
	Parametros map[string]interface{} `json:"parametros,omitempty"`
	TimeoutSeg int                    `json:"timeout_segundos,omitempty"`
}

// handlerAutoCreadasProbar ejecuta una herramienta con parámetros de prueba.
//
// POST /api/v1/herramientas/auto-creadas/{nombre}/probar
// Body: {"parametros": {...}, "timeout_segundos": 10}
func (s *Servidor) handlerAutoCreadasProbar(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	nombre := mux.Vars(r)["nombre"]
	if nombre == "" {
		s.responderError(w, http.StatusBadRequest, "nombre requerido")
		return
	}

	var req BodyAutoProbar
	if err := parsearBody(r, &req); err != nil {
		// Body es opcional; si falla, usar params vacíos
		req = BodyAutoProbar{}
	}

	timeout := 30 * time.Second
	if req.TimeoutSeg > 0 {
		timeout = time.Duration(req.TimeoutSeg) * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	res, err := s.autoGestor.Probar(ctx, nombre, req.Parametros)
	if err != nil {
		s.responderJSON(w, http.StatusOK, RespuestaAPI{
			Exito:     false,
			Error:     err.Error(),
			Datos:     res,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	s.responderJSON(w, http.StatusOK, RespuestaAPI{
		Exito:     res.Exito,
		Mensaje: func() string {
			if res.Exito {
				return "herramienta ejecutada exitosamente"
			}
			return res.Error
		}(),
		Datos:     res,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// POST /api/v1/herramientas/auto-creadas/{nombre}/recargar
// ============================================================================

// BodyAutoRecargar es el body para POST /api/v1/herramientas/auto-creadas/{nombre}/recargar.
type BodyAutoRecargar struct {
	// NuevoFuente: si se proporciona, se usa este fuente en lugar del existente.
	NuevoFuente string `json:"nuevo_fuente,omitempty"`

	// UsarLLM: si true y hay LLM, regenera el fuente vía LLM antes de compilar.
	UsarLLM bool `json:"usar_llm,omitempty"`
}

// handlerAutoCreadasRecargar recompila una herramienta desde su fuente.
//
// POST /api/v1/herramientas/auto-creadas/{nombre}/recargar
// Body: {"nuevo_fuente": "...", "usar_llm": false}
func (s *Servidor) handlerAutoCreadasRecargar(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	nombre := mux.Vars(r)["nombre"]
	if nombre == "" {
		s.responderError(w, http.StatusBadRequest, "nombre requerido")
		return
	}

	var req BodyAutoRecargar
	// Body es opcional
	_ = parsearBody(r, &req)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	resultado, err := s.autoGestor.Recargar(ctx, nombre, req.NuevoFuente, req.UsarLLM)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if resultado == nil {
			s.responderError(w, status, err.Error())
			return
		}
		s.responderJSON(w, status, RespuestaAPI{
			Exito:     false,
			Error:     err.Error(),
			Datos:     resultado,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

	s.log.Info("[fase6] herramienta recargada: %s", nombre)

	s.responderJSON(w, http.StatusOK, RespuestaAPI{
		Exito:     true,
		Mensaje:   fmt.Sprintf("herramienta '%s' recargada (v%d)", nombre, resultado.Metadata.VersionContador),
		Datos:     resultado,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ============================================================================
// GET /api/v1/herramientas/auto-creadas/{nombre}/fuente
// ============================================================================

// handlerAutoCreadasFuente retorna el código fuente Go de una herramienta.
//
// GET /api/v1/herramientas/auto-creadas/{nombre}/fuente
func (s *Servidor) handlerAutoCreadasFuente(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	nombre := mux.Vars(r)["nombre"]
	if nombre == "" {
		s.responderError(w, http.StatusBadRequest, "nombre requerido")
		return
	}

	fuente, err := s.autoGestor.LeerFuente(nombre)
	if err != nil {
		s.responderError(w, http.StatusNotFound, err.Error())
		return
	}

	// Devolver como texto plano para facilitar lectura
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fuente))
}

// ============================================================================
// GET /api/v1/herramientas/auto-creadas/{nombre}/log
// ============================================================================

// handlerAutoCreadasLog retorna el log de la última compilación.
//
// GET /api/v1/herramientas/auto-creadas/{nombre}/log
func (s *Servidor) handlerAutoCreadasLog(w http.ResponseWriter, r *http.Request) {
	if !s.requiereAutoGestor(w) {
		return
	}

	nombre := mux.Vars(r)["nombre"]
	if nombre == "" {
		s.responderError(w, http.StatusBadRequest, "nombre requerido")
		return
	}

	log, err := s.autoGestor.LeerLogCompilacion(nombre)
	if err != nil {
		s.responderError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if log == "" {
		_, _ = w.Write([]byte("(sin log de compilación)"))
	} else {
		_, _ = w.Write([]byte(log))
	}
}

// ============================================================================
// Helper expuesto para tests
// ============================================================================

// AutoGestorExpuesto retorna el gestor de auto-creación (para tests).
func (s *Servidor) AutoGestorExpuesto() *auto_creacion.Gestor {
	return s.autoGestor
}

// compile-time check de que usamos encoding/json
var _ = json.Marshal
