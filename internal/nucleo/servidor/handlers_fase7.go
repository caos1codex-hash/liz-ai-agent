package servidor

import (
        "encoding/json"
        "fmt"
        "net/http"
        "strings"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/pipeline"
        "github.com/gorilla/mux"
)

// ============================================================
// Handlers Fase 7 — Pipeline de Chat
// End-to-end: mensaje → modelo → herramientas → respuesta
// ============================================================

// registrarRutasFase7 registra los endpoints del pipeline de chat.
func (s *Servidor) registrarRutasFase7() {
        // Pipeline de chat principal
        s.router.HandleFunc("/api/v1/chat", s.handlerChatGet).Methods("GET")
        s.router.HandleFunc("/api/v1/chat", s.handlerChatPost).Methods("POST")

        // Sesiones de chat
        s.router.HandleFunc("/api/v1/chat/sesiones", s.handlerChatSesiones).Methods("GET")
        s.router.HandleFunc("/api/v1/chat/sesiones", s.handlerChatCrearSesion).Methods("POST")

        // Detalle y cierre de sesión
        s.router.HandleFunc("/api/v1/chat/sesiones/{id}", s.handlerChatSesionDetalle).Methods("GET")
        s.router.HandleFunc("/api/v1/chat/sesiones/{id}", s.handlerChatCerrarSesion).Methods("DELETE")

        // Métricas del pipeline
        s.router.HandleFunc("/api/v1/chat/metricas", s.handlerChatMetricas).Methods("GET")
}

// handlerChatGet maneja GET /api/v1/chat — estado del pipeline.
func (s *Servidor) handlerChatGet(w http.ResponseWriter, r *http.Request) {
        if s.pipelineMgr == nil {
                s.responderError(w, http.StatusServiceUnavailable, "Pipeline de chat no disponible (Fase 7 no inicializada)")
                return
        }

        estado := s.pipelineMgr.Estado()

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:  true,
                Mensaje: "Estado del pipeline de chat",
                Datos: map[string]interface{}{
                        "mensajes_procesados": estado.MensajesProcesados,
                        "promedio_duracion":   estado.PromedioDuracion.String(),
                        "ultimo_uso":         estado.UltimoUso.Format(time.RFC3339),
                        "categorias":         estado.CategoriaCount,
                        "modelo_mas_usado":   estado.ModeloMasUsado,
                        "fase":               "7 (Pipeline de Chat)",
                },
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerChatPost maneja POST /api/v1/chat — enviar mensaje.
// Soporta JSON (respuesta completa) y SSE (streaming via Accept: text/event-stream).
func (s *Servidor) handlerChatPost(w http.ResponseWriter, r *http.Request) {
        if s.pipelineMgr == nil {
                s.responderError(w, http.StatusServiceUnavailable, "Pipeline de chat no disponible")
                return
        }

        var sol pipeline.SolicitudChat
        if err := json.NewDecoder(r.Body).Decode(&sol); err != nil {
                s.responderError(w, http.StatusBadRequest, fmt.Sprintf("Error decodificando solicitud: %v", err))
                return
        }

        if err := sol.Validar(); err != nil {
                s.responderError(w, http.StatusBadRequest, err.Error())
                return
        }

        // Modo streaming via SSE
        if sol.Stream || strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
                s.handlerChatSSE(w, r, &sol)
                return
        }

        // Modo JSON — respuesta completa
        s.log.Info("pipeline: Procesando mensaje (JSON) usuario=%s stream=false", sol.UsuarioID)

        resp, err := s.pipelineMgr.Procesar(r.Context(), &sol)
        if err != nil {
                s.log.Error("pipeline: Error procesando mensaje: %v", err)
                s.responderError(w, http.StatusInternalServerError, fmt.Sprintf("Error procesando: %v", err))
                return
        }

        s.log.Info("pipeline: Mensaje procesado sesion=%s duracion=%v modelo=%s", resp.SesionID, resp.DuracionTotal, resp.ModeloUsado)

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:  true,
                Mensaje: "Mensaje procesado exitosamente",
                Datos:  resp,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerChatSSE maneja el streaming de respuestas via Server-Sent Events.
func (s *Servidor) handlerChatSSE(w http.ResponseWriter, r *http.Request, sol *pipeline.SolicitudChat) {
        s.log.Info("pipeline: Procesando mensaje (SSE) usuario=%s stream=true", sol.UsuarioID)

        // Headers SSE
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")
        w.Header().Set("X-Accel-Buffering", "no") // Nginx

        flusher, ok := w.(http.Flusher)
        if !ok {
                s.responderError(w, http.StatusInternalServerError, "Streaming no soportado")
                return
        }

        ctx := r.Context()

        // Callback para enviar chunks SSE
        enviarChunk := func(chunk *pipeline.ChunkStream) {
                data, err := chunk.Serializar()
                if err != nil {
                        return
                }
                fmt.Fprintf(w, "data: %s\n\n", string(data))
                flusher.Flush()
        }

        // Enviar chunk inicial
        enviarChunk(pipeline.NuevoChunk("estado", "Iniciando pipeline de chat..."))

        // Ejecutar pipeline con streaming
        resp, err := s.pipelineMgr.ProcesarStream(ctx, sol, enviarChunk)
        if err != nil {
                errData, _ := json.Marshal(map[string]string{"tipo": "error", "error": err.Error()})
                fmt.Fprintf(w, "data: %s\n\n", string(errData))
                flusher.Flush()
                return
        }

        // Enviar resumen final
        resumen := map[string]interface{}{
                "tipo":             "completado",
                "sesion_id":        resp.SesionID,
                "modelo":           resp.ModeloUsado,
                "tokens":           resp.TokensUsados,
                "duracion_ms":      resp.DuracionTotal.Milliseconds(),
                "pasos_ejecutados": resp.PasosEjecutados,
        }
        resumenData, _ := json.Marshal(resumen)
        fmt.Fprintf(w, "data: %s\n\n", string(resumenData))
        flusher.Flush()

        s.log.Info("pipeline: Stream completado sesion=%s duracion=%v tokens=%d", resp.SesionID, resp.DuracionTotal, resp.TokensUsados)
}

// handlerChatSesiones maneja GET /api/v1/chat/sesiones — listar sesiones.
func (s *Servidor) handlerChatSesiones(w http.ResponseWriter, r *http.Request) {
        if s.gestorMem == nil {
                s.responderError(w, http.StatusServiceUnavailable, "Sistema de memoria no disponible")
                return
        }

        usuarioID := r.URL.Query().Get("usuario_id")
        if usuarioID == "" {
                usuarioID = "usuario_default"
        }

        sesiones, err := s.gestorMem.Sesiones().ListarSesiones(usuarioID, false)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, fmt.Sprintf("Error listando sesiones: %v", err))
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:  true,
                Mensaje: fmt.Sprintf("%d sesiones encontradas", len(sesiones)),
                Datos:  sesiones,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerChatCrearSesion maneja POST /api/v1/chat/sesiones — crear nueva sesión.
func (s *Servidor) handlerChatCrearSesion(w http.ResponseWriter, r *http.Request) {
        if s.gestorMem == nil {
                s.responderError(w, http.StatusServiceUnavailable, "Sistema de memoria no disponible")
                return
        }

        var req struct {
                UsuarioID string `json:"usuario_id"`
                Proyecto  string `json:"proyecto"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                s.responderError(w, http.StatusBadRequest, fmt.Sprintf("Error decodificando: %v", err))
                return
        }

        if req.UsuarioID == "" {
                req.UsuarioID = "usuario_default"
        }

        sesion, err := s.gestorMem.NuevaSesion(req.UsuarioID, req.Proyecto)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, fmt.Sprintf("Error creando sesión: %v", err))
                return
        }

        s.responderJSON(w, http.StatusCreated, RespuestaAPI{
                Exito:  true,
                Mensaje: "Sesión creada",
                Datos:  sesion,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerChatSesionDetalle maneja GET /api/v1/chat/sesiones/{id} — detalle de sesión con mensajes.
func (s *Servidor) handlerChatSesionDetalle(w http.ResponseWriter, r *http.Request) {
        if s.gestorMem == nil {
                s.responderError(w, http.StatusServiceUnavailable, "Sistema de memoria no disponible")
                return
        }

        vars := mux.Vars(r)
        sesionID := vars["id"]

        sesion, err := s.gestorMem.Sesiones().ObtenerSesion(sesionID)
        if err != nil {
                s.responderError(w, http.StatusNotFound, fmt.Sprintf("Sesión no encontrada: %v", err))
                return
        }

        // Obtener mensajes de la sesión
        usuarioID := s.gestorMem.Sesiones().SesionActiva(r.URL.Query().Get("usuario_id"))
        var mensajes interface{}
        if usuarioID != nil {
                mensajes = s.gestorMem.Sesiones().UltimosMensajes(sesion.UsuarioID, 50)
        } else {
                mensajes = []interface{}{}
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:  true,
                Mensaje: "Detalle de sesión",
                Datos: map[string]interface{}{
                        "sesion":   sesion,
                        "mensajes": mensajes,
                },
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerChatCerrarSesion maneja DELETE /api/v1/chat/sesiones/{id} — cerrar sesión.
func (s *Servidor) handlerChatCerrarSesion(w http.ResponseWriter, r *http.Request) {
        if s.gestorMem == nil {
                s.responderError(w, http.StatusServiceUnavailable, "Sistema de memoria no disponible")
                return
        }

        vars := mux.Vars(r)
        sesionID := vars["id"]

        // Para cerrar una sesión específica, primero obtenemos la sesión para conocer el usuarioID
        sesion, err := s.gestorMem.Sesiones().ObtenerSesion(sesionID)
        if err != nil {
                s.responderError(w, http.StatusNotFound, fmt.Sprintf("Sesión no encontrada: %v", err))
                return
        }

        err = s.gestorMem.Sesiones().CerrarSesion(sesion.UsuarioID)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, fmt.Sprintf("Error cerrando sesión: %v", err))
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:  true,
                Mensaje: "Sesión cerrada",
                Datos: map[string]interface{}{
                        "sesion_id": sesionID,
                        "estado":    "cerrada",
                },
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerChatMetricas maneja GET /api/v1/chat/metricas — métricas detalladas del pipeline.
func (s *Servidor) handlerChatMetricas(w http.ResponseWriter, r *http.Request) {
        if s.pipelineMgr == nil {
                s.responderError(w, http.StatusServiceUnavailable, "Pipeline no disponible")
                return
        }

        estado := s.pipelineMgr.Estado()

        datos := map[string]interface{}{
                "pipeline": map[string]interface{}{
                        "mensajes_procesados": estado.MensajesProcesados,
                        "promedio_duracion":   estado.PromedioDuracion.String(),
                        "ultimo_uso":         estado.UltimoUso.Format(time.RFC3339),
                        "modelo_mas_usado":   estado.ModeloMasUsado,
                },
                "categorias": estado.CategoriaCount,
                "componentes": map[string]bool{
                        "receptor":      true,
                        "clasificador":   true,
                        "planificador":   true,
                        "ejecutor":       true,
                        "respondedor":    true,
                        "orquestador":    s.orquestador != nil,
                        "catalogo":       s.catalogo != nil,
                        "memoria":        s.gestorMem != nil,
                        "auto_creacion":  s.autoGestor != nil,
                },
                "fase": "7",
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:  true,
                Mensaje: "Métricas del pipeline de chat",
                Datos:  datos,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}
