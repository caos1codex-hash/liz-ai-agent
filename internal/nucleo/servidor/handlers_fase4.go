package servidor

import (
        "encoding/json"
        "net/http"
        "strconv"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/memoria"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
        "github.com/gorilla/mux"
)

// ============================================================================
// Handlers — Orquestador (Fase 4)
// ============================================================================

// handlerOrquestadorEstado retorna el estado del orquestador.
func (s *Servidor) handlerOrquestadorEstado(w http.ResponseWriter, r *http.Request) {
        if !s.requiereOrquestador(w) {
                return
        }

        modelos := s.orquestador.ModelosDisponibles()
        estado := map[string]interface{}{
                "disponible":       true,
                "total_modelos":    len(modelos),
                "modelos_configurados": len(modelos),
        }
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     estado,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerOrquestadorModelos retorna la lista de modelos disponibles (sin API keys).
func (s *Servidor) handlerOrquestadorModelos(w http.ResponseWriter, r *http.Request) {
        if !s.requiereOrquestador(w) {
                return
        }

        modelos := s.orquestador.ModelosDisponibles()
        // Sanitizar: remover API keys
        for i := range modelos {
                if modelos[i].APIKey != "" {
                        modelos[i].APIKey = "***"
                }
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     modelos,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerOrquestadorMetricas retorna las métricas de uso de cada modelo.
func (s *Servidor) handlerOrquestadorMetricas(w http.ResponseWriter, r *http.Request) {
        if !s.requiereOrquestador(w) {
                return
        }

        metricas := s.orquestador.Metricas()
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     metricas,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// SolicitudCompletar es el body para /api/v1/orquestador/completar.
type SolicitudCompletar struct {
        Modelo      string                  `json:"modelo,omitempty"`
        Tarea       string                  `json:"tarea,omitempty"`
        Mensajes    []orquestador.MensajeChat `json:"mensajes"`
        Temperatura float64                 `json:"temperatura,omitempty"`
        MaxTokens   int                     `json:"max_tokens,omitempty"`
        Stream      bool                    `json:"stream,omitempty"`
}

// handlerOrquestadorCompletar envía una solicitud de chat completion.
//
// Si Stream=true, responde con Server-Sent Events (SSE).
// Si Stream=false, responde con JSON.
func (s *Servidor) handlerOrquestadorCompletar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereOrquestador(w) {
                return
        }

        var req SolicitudCompletar
        if err := s.parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "body inválido: "+err.Error())
                return
        }

        if len(req.Mensajes) == 0 {
                s.responderError(w, http.StatusBadRequest, "mensajes no puede estar vacío")
                return
        }

        solicitud := orquestador.SolicitudChat{
                Modelo:       req.Modelo,
                Tarea:        orquestador.TipoTarea(req.Tarea),
                Mensajes:     req.Mensajes,
                Temperatura:  req.Temperatura,
                MaxTokens:    req.MaxTokens,
                Stream:       req.Stream,
        }

        // Streaming SSE
        if req.Stream {
                ch, err := s.orquestador.CompletarStream(solicitud)
                if err != nil {
                        s.responderError(w, http.StatusBadGateway, "error iniciando stream: "+err.Error())
                        return
                }

                w.Header().Set("Content-Type", "text/event-stream")
                w.Header().Set("Cache-Control", "no-cache")
                w.Header().Set("Connection", "keep-alive")
                w.WriteHeader(http.StatusOK)

                flusher, _ := w.(http.Flusher)
                for chunk := range ch {
                        data := map[string]interface{}{
                                "contenido":    chunk.Contenido,
                                "acabado":      chunk.Acabado,
                                "modelo_usado": chunk.ModeloUsado,
                        }
                        if chunk.Error != nil {
                                data["error"] = chunk.Error.Error()
                        }
                        if bytes, err := json.Marshal(data); err == nil {
                                w.Write([]byte("data: "))
                                w.Write(bytes)
                                w.Write([]byte("\n\n"))
                                if flusher != nil {
                                        flusher.Flush()
                                }
                        }
                }
                return
        }

        // No streaming
        resp, err := s.orquestador.Completar(solicitud)
        if err != nil {
                s.responderError(w, http.StatusBadGateway, "error del orquestador: "+err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     resp.Error == "",
                Datos:     resp,
                Error:     resp.Error,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// ============================================================================
// Handlers — Memoria Conversacional (Fase 3.5+)
// ============================================================================

// handlerMemoriaSesiones lista las sesiones de un usuario.
// Query params: usuario_id (requerido), solo_activas (opcional, "true"/"false").
func (s *Servidor) handlerMemoriaSesiones(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        usuarioID := r.URL.Query().Get("usuario_id")
        if usuarioID == "" {
                s.responderError(w, http.StatusBadRequest, "parámetro 'usuario_id' requerido")
                return
        }
        soloActivas := r.URL.Query().Get("solo_activas") == "true"

        sesiones, err := s.gestorMem.Sesiones().ListarSesiones(usuarioID, soloActivas)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, "error listando sesiones: "+err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     sesiones,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// SolicitudNuevaSesion es el body para crear una sesión.
type SolicitudNuevaSesion struct {
        UsuarioID string `json:"usuario_id"`
        Proyecto  string `json:"proyecto,omitempty"`
}

// handlerMemoriaNuevaSesion crea una nueva sesión.
func (s *Servidor) handlerMemoriaNuevaSesion(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        var req SolicitudNuevaSesion
        if err := s.parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "body inválido: "+err.Error())
                return
        }
        if req.UsuarioID == "" {
                s.responderError(w, http.StatusBadRequest, "usuario_id requerido")
                return
        }

        sesion, err := s.gestorMem.NuevaSesion(req.UsuarioID, req.Proyecto)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, "error creando sesión: "+err.Error())
                return
        }

        s.responderJSON(w, http.StatusCreated, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Sesión creada",
                Datos:     sesion,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerMemoriaObtenerSesion retorna una sesión por ID.
func (s *Servidor) handlerMemoriaObtenerSesion(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        vars := mux.Vars(r)
        id := vars["id"]
        if id == "" {
                s.responderError(w, http.StatusBadRequest, "id requerido")
                return
        }

        sesion, err := s.gestorMem.Sesiones().ObtenerSesion(id)
        if err != nil {
                s.responderError(w, http.StatusNotFound, "sesión no encontrada: "+err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     sesion,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerMemoriaCerrarSesion cierra una sesión activa por ID.
// Requiere query param usuario_id para autorizar.
func (s *Servidor) handlerMemoriaCerrarSesion(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        vars := mux.Vars(r)
        id := vars["id"]
        usuarioID := r.URL.Query().Get("usuario_id")
        if id == "" || usuarioID == "" {
                s.responderError(w, http.StatusBadRequest, "id y usuario_id requeridos")
                return
        }

        // Verificar que la sesión pertenece al usuario
        sesion, err := s.gestorMem.Sesiones().ObtenerSesion(id)
        if err != nil {
                s.responderError(w, http.StatusNotFound, "sesión no encontrada")
                return
        }
        if sesion.UsuarioID != usuarioID {
                s.responderError(w, http.StatusForbidden, "la sesión no pertenece al usuario")
                return
        }

        if err := s.gestorMem.CerrarSesion(usuarioID); err != nil {
                s.responderError(w, http.StatusInternalServerError, "error cerrando sesión: "+err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Sesión cerrada",
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// SolicitudAgregarMensaje es el body para agregar un mensaje a una sesión.
type SolicitudAgregarMensaje struct {
        UsuarioID string `json:"usuario_id"`
        Rol       string `json:"rol"`       // "usuario", "asistente", "sistema", "herramienta"
        Contenido string `json:"contenido"`
}

// handlerMemoriaAgregarMensaje agrega un mensaje a la sesión activa de un usuario.
// El parámetro {id} en la ruta se ignora: el mensaje se agrega a la sesión activa
// del usuario_id especificado en el body.
func (s *Servidor) handlerMemoriaAgregarMensaje(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        var req SolicitudAgregarMensaje
        if err := s.parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "body inválido: "+err.Error())
                return
        }
        if req.UsuarioID == "" {
                s.responderError(w, http.StatusBadRequest, "usuario_id requerido")
                return
        }
        if req.Contenido == "" {
                s.responderError(w, http.StatusBadRequest, "contenido requerido")
                return
        }

        rol := memoria.RolMensaje(req.Rol)
        switch rol {
        case memoria.RolUsuario, memoria.RolAsistente, memoria.RolSistema, memoria.RolHerramienta:
                // ok
        default:
                s.responderError(w, http.StatusBadRequest, "rol inválido (debe ser usuario/asistente/sistema/herramienta)")
                return
        }

        msg, err := s.gestorMem.AgregarMensaje(req.UsuarioID, rol, req.Contenido)
        if err != nil {
                s.responderError(w, http.StatusBadRequest, err.Error())
                return
        }

        s.responderJSON(w, http.StatusCreated, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Mensaje agregado",
                Datos:     msg,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerMemoriaHechos lista los hechos activos de un usuario.
// Query param: usuario_id (requerido).
func (s *Servidor) handlerMemoriaHechos(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        usuarioID := r.URL.Query().Get("usuario_id")
        if usuarioID == "" {
                s.responderError(w, http.StatusBadRequest, "usuario_id requerido")
                return
        }

        hechos, err := s.gestorMem.Hechos().HechosActivos(usuarioID)
        if err != nil {
                // Si no hay archivo, retornar lista vacía
                s.responderJSON(w, http.StatusOK, RespuestaAPI{
                        Exito:     true,
                        Datos:     []interface{}{},
                        Timestamp: time.Now().Format(time.RFC3339),
                })
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     hechos,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// SolicitudAgregarHecho es el body para crear un hecho.
type SolicitudAgregarHecho struct {
        UsuarioID    string  `json:"usuario_id"`
        Sujeto       string  `json:"sujeto"`
        Predicado    string  `json:"predicado"`
        Objeto       string  `json:"objeto"`
        Confianza    float64 `json:"confianza"`
        SesionOrigen string  `json:"sesion_origen,omitempty"`
}

// handlerMemoriaAgregarHecho crea un hecho (con resolución de conflictos).
func (s *Servidor) handlerMemoriaAgregarHecho(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        var req SolicitudAgregarHecho
        if err := s.parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "body inválido: "+err.Error())
                return
        }
        if req.UsuarioID == "" || req.Sujeto == "" || req.Predicado == "" || req.Objeto == "" {
                s.responderError(w, http.StatusBadRequest, "usuario_id, sujeto, predicado, objeto son requeridos")
                return
        }

        hecho, err := s.gestorMem.AgregarHecho(
                req.UsuarioID, req.Sujeto, req.Predicado, req.Objeto,
                req.Confianza, req.SesionOrigen,
        )
        if err != nil {
                s.responderError(w, http.StatusBadRequest, err.Error())
                return
        }

        s.responderJSON(w, http.StatusCreated, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Hecho agregado",
                Datos:     hecho,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerMemoriaEliminarHecho marca un hecho como obsoleto.
// Query param: usuario_id (requerido).
func (s *Servidor) handlerMemoriaEliminarHecho(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        vars := mux.Vars(r)
        id := vars["id"]
        usuarioID := r.URL.Query().Get("usuario_id")
        if id == "" || usuarioID == "" {
                s.responderError(w, http.StatusBadRequest, "id y usuario_id requeridos")
                return
        }

        if err := s.gestorMem.Hechos().EliminarHecho(usuarioID, id); err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Hecho marcado como obsoleto",
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerMemoriaContexto retorna el contexto completo de memoria (hechos + mensajes)
// para inyectar en el prompt del LLM.
// Query params: usuario_id (requerido), mensajes (default 10), hechos (default 20).
func (s *Servidor) handlerMemoriaContexto(w http.ResponseWriter, r *http.Request) {
        if !s.requiereMemoria(w) {
                return
        }

        usuarioID := r.URL.Query().Get("usuario_id")
        if usuarioID == "" {
                s.responderError(w, http.StatusBadRequest, "usuario_id requerido")
                return
        }

        mensajes := 10
        hechos := 20
        if v := r.URL.Query().Get("mensajes"); v != "" {
                if n, err := parseIntSafe(v); err == nil && n > 0 {
                        mensajes = n
                }
        }
        if v := r.URL.Query().Get("hechos"); v != "" {
                if n, err := parseIntSafe(v); err == nil && n > 0 {
                        hechos = n
                }
        }

        ctx, err := s.gestorMem.ContextoParaLLM(usuarioID, mensajes, hechos)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "contexto":      ctx,
                        "mensajes_n":    mensajes,
                        "hechos_n":      hechos,
                        "estadisticas":  s.gestorMem.Estadisticas(usuarioID),
                },
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// parseIntSafe parsea un string a int sin panic.
func parseIntSafe(s string) (int, error) {
        return strconv.Atoi(s)
}
