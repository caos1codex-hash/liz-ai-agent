package servidor

import (
        "context"
        "encoding/json"
        "fmt"
        "net/http"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/registro"
        "github.com/gorilla/mux"
)

// ============================================================================
// Fase 5: Handlers de Herramientas
// ============================================================================

// ConCatalogo inyecta el catálogo de herramientas en el servidor.
// Debe llamarse antes de Iniciar().
func (s *Servidor) ConCatalogo(cat *registro.Catalogo) *Servidor {
        s.catalogo = cat
        return s
}

// requiereCatalogo verifica que el catálogo esté disponible.
// Si no lo está, responde 503 Service Unavailable y retorna false.
func (s *Servidor) requiereCatalogo(w http.ResponseWriter) bool {
        if s.catalogo == nil {
                s.responderError(w, http.StatusServiceUnavailable,
                        "catálogo de herramientas no disponible (Fase 5 no inicializada)")
                return false
        }
        return true
}

// registrarRutasHerramientas registra todos los endpoints de /api/v1/herramientas/*.
// IMPORTANTE: las rutas más específicas (como /ejecutar, /metricas) deben
// registrarse ANTES que la ruta con {nombre} para evitar que {nombre}
// las capture.
// Se llama desde registrarRutas().
func (s *Servidor) registrarRutasHerramientas() {
        // Ejecutar herramienta (POST)
        s.router.HandleFunc("/api/v1/herramientas/ejecutar", s.handlerHerramientasEjecutar).Methods("POST", "OPTIONS")

        // Métricas (todas y por nombre)
        s.router.HandleFunc("/api/v1/herramientas/metricas", s.handlerHerramientasMetricas).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/herramientas/metricas/{nombre}", s.handlerHerramientasMetricasUna).Methods("GET", "OPTIONS")

        // Listar todas las herramientas (catálogo)
        s.router.HandleFunc("/api/v1/herramientas", s.handlerHerramientasListar).Methods("GET", "OPTIONS")

        // Info de una herramienta específica (DESPUÉS de /ejecutar y /metricas)
        s.router.HandleFunc("/api/v1/herramientas/{nombre}", s.handlerHerramientasInfo).Methods("GET", "OPTIONS")

        // Reemplazar el stub /api/v1/tools con la implementación real
        s.router.HandleFunc("/api/v1/tools", s.handlerHerramientasListar).Methods("GET", "OPTIONS")
}

// handlerHerramientasListar retorna el catálogo completo de herramientas.
//
// GET /api/v1/herramientas
//
// Respuesta:
//   200 OK — lista de herramientas con nombre, descripción, parámetros
//   503 Service Unavailable — catálogo no inicializado
func (s *Servidor) handlerHerramientasListar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCatalogo(w) {
                return
        }

        snapshot := s.catalogo.Snapshot()
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   fmt.Sprintf("%d herramientas disponibles", len(snapshot)),
                Datos:     snapshot,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerHerramientasInfo retorna info detallada de una herramienta.
//
// GET /api/v1/herramientas/{nombre}
//
// Respuesta:
//   200 OK — info de la herramienta
//   404 Not Found — herramienta no existe
func (s *Servidor) handlerHerramientasInfo(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCatalogo(w) {
                return
        }

        vars := mux.Vars(r)
        nombre := vars["nombre"]
        if nombre == "" {
                s.responderError(w, http.StatusBadRequest, "nombre de herramienta requerido")
                return
        }

        h, ok := s.catalogo.Obtener(nombre)
        if !ok {
                s.responderError(w, http.StatusNotFound,
                        fmt.Sprintf("herramienta '%s' no encontrada", nombre))
                return
        }

        info := registro.InfoHerramienta{
                Nombre:      h.Nombre(),
                Descripcion: h.Descripcion(),
                Parametros:  h.Parametros(),
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     info,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// BodyEjecutarHerramienta es el body para POST /api/v1/herramientas/ejecutar.
type BodyEjecutarHerramienta struct {
        Nombre      string                 `json:"nombre"`
        Parametros  map[string]interface{} `json:"parametros,omitempty"`
        TimeoutSeg  int                    `json:"timeout_segundos,omitempty"`
}

// handlerHerramientasEjecutar ejecuta una herramienta con los parámetros dados.
//
// POST /api/v1/herramientas/ejecutar
// Body: { "nombre": "terminal", "parametros": { "comando": "echo", "args": ["hola"] } }
//
// Respuesta:
//   200 OK — resultado de la herramienta (Exito puede ser true o false)
//   400 Bad Request — body inválido o nombre vacío
//   404 Not Found — herramienta no existe
//   503 Service Unavailable — catálogo no inicializado
func (s *Servidor) handlerHerramientasEjecutar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCatalogo(w) {
                return
        }

        var req BodyEjecutarHerramienta
        if err := parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
                return
        }

        if req.Nombre == "" {
                s.responderError(w, http.StatusBadRequest, "campo 'nombre' es requerido")
                return
        }

        if !s.catalogo.Existe(req.Nombre) {
                s.responderError(w, http.StatusNotFound,
                        fmt.Sprintf("herramienta '%s' no encontrada", req.Nombre))
                return
        }

        // Timeout opcional
        ctx := r.Context()
        if req.TimeoutSeg > 0 {
                var cancel context.CancelFunc
                ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeg)*time.Second)
                defer cancel()
        }

        // Ejecutar (el catálogo mide latencia y registra métricas automáticamente)
        res, err := s.catalogo.Ejecutar(ctx, req.Nombre, req.Parametros)
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

// handlerHerramientasMetricas retorna métricas agregadas de todas las herramientas.
//
// GET /api/v1/herramientas/metricas
func (s *Servidor) handlerHerramientasMetricas(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCatalogo(w) {
                return
        }

        resumen := s.catalogo.Metricas().Resumen()
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     resumen,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerHerramientasMetricasUna retorna métricas de una herramienta específica.
//
// GET /api/v1/herramientas/metricas/{nombre}
func (s *Servidor) handlerHerramientasMetricasUna(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCatalogo(w) {
                return
        }

        vars := mux.Vars(r)
        nombre := vars["nombre"]
        if nombre == "" {
                s.responderError(w, http.StatusBadRequest, "nombre requerido")
                return
        }

        m := s.catalogo.Metricas().Obtener(nombre)
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     m,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// parsearBody helper para decodificar JSON del body.
// Re-exportado aquí para evitar dependencia con el archivo principal.
func parsearBody(r *http.Request, v interface{}) error {
        if r.Body == nil {
                return fmt.Errorf("body vacío")
        }
        decoder := json.NewDecoder(r.Body)
        decoder.DisallowUnknownFields()
        if err := decoder.Decode(v); err != nil {
                // Intentar de nuevo sin DisallowUnknownFields para ser tolerante
                return json.NewDecoder(r.Body).Decode(v)
        }
        return nil
}

// CatalogoSnapshotExpuesto helper expuesto para tests que quieran inspeccionar
// el catálogo sin pasar por HTTP.
func (s *Servidor) CatalogoSnapshotExpuesto() []registro.InfoHerramienta {
        if s.catalogo == nil {
                return nil
        }
        return s.catalogo.Snapshot()
}

// Compile-time check de que implementamos la interfaz herramientas.Herramienta
// para evitar importaciones cíclicas con integradas — solo verificamos registro.
var _ = herramientas.Herramienta(nil)
