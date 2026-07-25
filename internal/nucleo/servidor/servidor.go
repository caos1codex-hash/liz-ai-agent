package servidor

import (
        "encoding/json"
        "fmt"
        "net/http"
        "runtime"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
        "github.com/gorilla/mux"
)

// Servidor es el servidor HTTP principal de Liz.
// Gestiona todas las rutas de la API y coordina los módulos del nucleo.
type Servidor struct {
        router    *mux.Router
        log       *logger.Logger
        config    *config.Configuracion
        permisos  *permisos.Sistema
        inicio    time.Time
        sesionID  string
}

// RespuestaAPI es la estructura base de toda respuesta de la API.
type RespuestaAPI struct {
        Exito    bool        `json:"exito"`
        Datos    interface{} `json:"datos,omitempty"`
        Error    string      `json:"error,omitempty"`
        Metadata interface{} `json:"metadata,omitempty"`
}

// HealthResponse es la respuesta del endpoint de healthcheck.
type HealthResponse struct {
        Estado   string `json:"estado"`
        Version  string `json:"version"`
        Uptime   string `json:"uptime"`
        GoVersion string `json:"go_version"`
        Goroutines int   `json:"goroutines"`
        MemoriaMB float64 `json:"memoria_mb"`
}

// InfoConfigResponse es la respuesta del endpoint de configuración.
type InfoConfigResponse struct {
        Version        string `json:"version"`
        Puerto         int    `json:"puerto"`
        Host           string `json:"host"`
        Tema           string `json:"tema"`
        DirectorioTrabajo string `json:"directorio_trabajo"`
        ModelosDisponibles int  `json:"modelos_disponibles"`
        NVIDIAConfigurado bool `json:"nvidia_configurado"`
        PermisosSolicitados bool `json:"permisos_solicitar_al_iniciar"`
}

// Nuevo crea e inicializa el servidor HTTP de Liz.
// Configura todas las rutas de la API según la arquitectura definida.
func Nuevo(cfg *config.Configuracion, log *logger.Logger, sisPermisos *permisos.Sistema) *Servidor {
        s := &Servidor{
                router:   mux.NewRouter(),
                log:      log,
                config:   cfg,
                permisos: sisPermisos,
                inicio:   time.Now(),
                sesionID: generarSesionID(),
        }

        s.configurarRutas()
        return s
}

// generarSesionID crea un ID único para la sesión actual.
func generarSesionID() string {
        return fmt.Sprintf("ses_%s", time.Now().Format("20060102_150405"))
}

// configurarRutas registra todos los endpoints de la API de Liz.
// Estos endpoints están definidos en la sección 8 de ARQUITECTURA.md.
func (s *Servidor) configurarRutas() {
        // Middleware global: logging de requests, headers, recovery
        s.router.Use(s.middlewareLogging)
        s.router.Use(s.middlewareCORS)
        s.router.Use(s.middlewareRecovery)

        // ── Healthcheck ──
        s.router.HandleFunc("/api/health", s.handleHealth).Methods("GET", "OPTIONS")

        // ── Configuración ──
        s.router.HandleFunc("/api/config", s.handleGetConfig).Methods("GET")
        s.router.HandleFunc("/api/config", s.handlePutConfig).Methods("PUT")

        // ── Permisos ──
        s.router.HandleFunc("/api/permisos", s.handleGetPermisos).Methods("GET")
        s.router.HandleFunc("/api/permisos", s.handlePostPermisos).Methods("POST")

        // ── Herramientas (stub para Fase 5) ──
        s.router.HandleFunc("/api/tools", s.handleGetTools).Methods("GET")
        s.router.HandleFunc("/api/tools/{nombre}", s.handleGetTool).Methods("GET")

        // ── Orquestador (stub para Fase 4) ──
        s.router.HandleFunc("/api/orquestador/modelos", s.handleGetModelos).Methods("GET")
        s.router.HandleFunc("/api/orquestador/metricas", s.handleGetMetricas).Methods("GET")

        // ── Conversaciones (stub para Fase 7) ──
        s.router.HandleFunc("/api/conversations", s.handleGetConversations).Methods("GET")
        s.router.HandleFunc("/api/conversations", s.handlePostConversations).Methods("POST")
        s.router.HandleFunc("/api/conversations/{id}", s.handleDeleteConversation).Methods("DELETE")

        // ── Chat (stub para Fase 7) ──
        s.router.HandleFunc("/api/chat", s.handleChat).Methods("POST")
}

// ═══════════════════════════════════════════════════════
// MIDDLEWARES
// ═══════════════════════════════════════════════════════

// middlewareLogging loguea cada request entrante.
func (s *Servidor) middlewareLogging(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                start := time.Now()
                next.ServeHTTP(w, r)
                s.log.Debug("%s %s → %v", r.Method, r.URL.Path, time.Since(start))
        })
}

// middlewareCORS agrega headers CORS para el frontend.
func (s *Servidor) middlewareCORS(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Access-Control-Allow-Origin", "*")
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
                w.Header().Set("Content-Type", "application/json")

                if r.Method == "OPTIONS" {
                        w.WriteHeader(http.StatusOK)
                        return
                }

                next.ServeHTTP(w, r)
        })
}

// middlewareRecovery captura panics y retorna error 500.
func (s *Servidor) middlewareRecovery(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                defer func() {
                        if rec := recover(); rec != nil {
                                s.log.Error("panic recuperado en %s %s: %v", r.Method, r.URL.Path, rec)
                                w.WriteHeader(http.StatusInternalServerError)
                                json.NewEncoder(w).Encode(RespuestaAPI{
                                        Exito: false,
                                        Error: "Error interno del servidor",
                                })
                        }
                }()
                next.ServeHTTP(w, r)
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — HEALTH
// ═══════════════════════════════════════════════════════

// handleHealth retorna el estado de Liz (healthcheck).
func (s *Servidor) handleHealth(w http.ResponseWriter, r *http.Request) {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)

        resp := HealthResponse{
                Estado:     "operativo",
                Version:    s.config.Version,
                Uptime:     time.Since(s.inicio).Truncate(time.Second).String(),
                GoVersion:  runtime.Version(),
                Goroutines: runtime.NumGoroutine(),
                MemoriaMB:  float64(m.Alloc) / 1024 / 1024,
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: resp,
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — CONFIGURACIÓN
// ═══════════════════════════════════════════════════════

// handleGetConfig retorna la configuración actual (sin API keys).
func (s *Servidor) handleGetConfig(w http.ResponseWriter, r *http.Request) {
        resp := InfoConfigResponse{
                Version:           s.config.Version,
                Puerto:            s.config.Servidor.Puerto,
                Host:              s.config.Servidor.Host,
                Tema:              s.config.Tema,
                DirectorioTrabajo: s.config.DirectorioTrabajo,
                ModelosDisponibles: len(s.config.NVIDIA.Modelos),
                NVIDIAConfigurado: s.config.NVIDIA.APIKey != "",
                PermisosSolicitados: s.config.Permisos.SolicitarAlIniciar,
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: resp,
        })
}

// SolicitudPutConfig es el body para modificar configuración.
type SolicitudPutConfig struct {
        Puerto *int    `json:"puerto,omitempty"`
        Host   *string `json:"host,omitempty"`
        Tema   *string `json:"tema,omitempty"`
        DirectorioTrabajo *string `json:"directorio_trabajo,omitempty"`
}

// handlePutConfig modifica la configuración en runtime.
func (s *Servidor) handlePutConfig(w http.ResponseWriter, r *http.Request) {
        var req SolicitudPutConfig
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                s.responderError(w, http.StatusBadRequest, "JSON inválido: %v", err)
                return
        }

        if req.Puerto != nil {
                if *req.Puerto < 1 || *req.Puerto > 65535 {
                        s.responderError(w, http.StatusBadRequest, "puerto debe estar entre 1 y 65535")
                        return
                }
                s.config.Servidor.Puerto = *req.Puerto
        }

        if req.Host != nil && *req.Host != "" {
                s.config.Servidor.Host = *req.Host
        }

        if req.Tema != nil {
                s.config.Tema = *req.Tema
        }

        if req.DirectorioTrabajo != nil {
                s.config.DirectorioTrabajo = *req.DirectorioTrabajo
        }

        s.log.Info("configuración actualizada en runtime")
        s.handleGetConfig(w, r)
}

// ═══════════════════════════════════════════════════════
// HANDLERS — PERMISOS
// ═══════════════════════════════════════════════════════

// handleGetPermisos retorna el estado actual de permisos.
func (s *Servidor) handleGetPermisos(w http.ResponseWriter, r *http.Request) {
        estado := s.permisos.Estado()

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: estado,
        })
}

// SolicitudConcederPermisos es el body para conceder permisos.
type SolicitudConcederPermisos struct {
        ConcederTodos bool     `json:"conceder_todos"`
        Permisos      []string `json:"permisos,omitempty"` // lista de nombres individuales
}

// handlePostPermisos concede permisos (todos o individuales).
func (s *Servidor) handlePostPermisos(w http.ResponseWriter, r *http.Request) {
        var req SolicitudConcederPermisos
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                s.responderError(w, http.StatusBadRequest, "JSON inválido: %v", err)
                return
        }

        if req.ConcederTodos {
                if err := s.permisos.ConcederTodos(s.sesionID); err != nil {
                        s.responderError(w, http.StatusInternalServerError, "error concediendo permisos: %v", err)
                        return
                }
                s.log.Info("todos los permisos concedidos para sesión %s", s.sesionID)
        } else if len(req.Permisos) > 0 {
                for _, nombre := range req.Permisos {
                        if err := s.permisos.Conceder(nombre); err != nil {
                                s.responderError(w, http.StatusBadRequest, "error concediendo permiso %s: %v", nombre, err)
                                return
                        }
                }
                s.log.Info("permisos individuales concedidos: %v", req.Permisos)
        } else {
                s.responderError(w, http.StatusBadRequest, "debes especificar conceder_todos o una lista de permisos")
                return
        }

        s.handleGetPermisos(w, r)
}

// ═══════════════════════════════════════════════════════
// HANDLERS — HERRAMIENTAS (stubs para Fase 5)
// ═══════════════════════════════════════════════════════

// handleGetTools lista todas las herramientas registradas.
// Stub: retorna lista vacía hasta la Fase 5.
func (s *Servidor) handleGetTools(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "herramientas": []interface{}{},
                        "total":        0,
                        "fase":         "pendiente (Fase 5)",
                },
        })
}

// handleGetTool retorna info de una herramienta específica.
// Stub: retorna not found hasta la Fase 5.
func (s *Servidor) handleGetTool(w http.ResponseWriter, r *http.Request) {
        nombre := mux.Vars(r)["nombre"]
        s.responderJSON(w, http.StatusNotFound, RespuestaAPI{
                Exito: false,
                Error: fmt.Sprintf("herramienta '%s' no encontrada (Fase 5 pendiente)", nombre),
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — ORQUESTADOR (stubs para Fase 4)
// ═══════════════════════════════════════════════════════

// handleGetModelos lista los modelos disponibles configurados.
func (s *Servidor) handleGetModelos(w http.ResponseWriter, r *http.Request) {
        type ModeloRes struct {
                ID        string   `json:"id"`
                Nombre    string   `json:"nombre"`
                Tipo      []string `json:"tipo"`
                Velocidad string   `json:"velocidad"`
                Prioridad int      `json:"prioridad"`
        }

        modelos := make([]ModeloRes, len(s.config.NVIDIA.Modelos))
        for i, m := range s.config.NVIDIA.Modelos {
                modelos[i] = ModeloRes{
                        ID:        m.ID,
                        Nombre:    m.Nombre,
                        Tipo:      m.Tipo,
                        Velocidad: m.Velocidad,
                        Prioridad: m.Prioridad,
                }
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "modelos": modelos,
                        "total":   len(modelos),
                        "endpoint": s.config.NVIDIA.Endpoint,
                },
        })
}

// handleGetMetricas retorna métricas de uso de modelos.
// Stub: retorna vacío hasta la Fase 4.
func (s *Servidor) handleGetMetricas(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "metricas": map[string]interface{}{},
                        "fase":    "pendiente (Fase 4)",
                },
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — CONVERSACIONES (stubs para Fase 7)
// ═══════════════════════════════════════════════════════

// handleGetConversations lista las conversaciones.
func (s *Servidor) handleGetConversations(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "conversaciones": []interface{}{},
                        "total":         0,
                        "fase":          "pendiente (Fase 7)",
                },
        })
}

// handlePostConversations crea una nueva conversación.
func (s *Servidor) handlePostConversations(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusNotImplemented, RespuestaAPI{
                Exito: false,
                Error: "creación de conversaciones disponible en Fase 7",
        })
}

// handleDeleteConversation elimina una conversación.
func (s *Servidor) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusNotImplemented, RespuestaAPI{
                Exito: false,
                Error: "eliminación de conversaciones disponible en Fase 7",
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — CHAT (stub para Fase 7)
// ═══════════════════════════════════════════════════════

// handleChat es el endpoint principal de chat con SSE streaming.
// Stub: retorna not implemented hasta la Fase 7.
func (s *Servidor) handleChat(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusNotImplemented, RespuestaAPI{
                Exito: false,
                Error: "chat disponible en Fase 7 (Pipeline de Chat)",
        })
}

// ═══════════════════════════════════════════════════════
// UTILIDADES
// ═══════════════════════════════════════════════════════

// responderJSON escribe una respuesta JSON al cliente.
func (s *Servidor) responderJSON(w http.ResponseWriter, statusCode int, resp RespuestaAPI) {
        w.WriteHeader(statusCode)
        json.NewEncoder(w).Encode(resp)
}

// responderError escribe una respuesta de error JSON al cliente.
func (s *Servidor) responderError(w http.ResponseWriter, statusCode int, formato string, args ...interface{}) {
        mensaje := fmt.Sprintf(formato, args...)
        s.log.Error(mensaje)

        w.WriteHeader(statusCode)
        json.NewEncoder(w).Encode(RespuestaAPI{
                Exito: false,
                Error: mensaje,
        })
}

// Iniciar arranca el servidor HTTP de Liz.
// Bloquea el hilo principal. Retorna error si no puede bindear el puerto.
func (s *Servidor) Iniciar() error {
        dir := fmt.Sprintf("%s:%d", s.config.Servidor.Host, s.config.Servidor.Puerto)

        s.log.Info("═══════════════════════════════════════════════")
        s.log.Info("  Liz AI Agent v%s", s.config.Version)
        s.log.Info("  Servidor arrancando en http://%s", dir)
        s.log.Info("  Sesión: %s", s.sesionID)
        s.log.Info("═══════════════════════════════════════════════")

        return http.ListenAndServe(dir, s.router)
}

// Router retorna el router mux para testing.
func (s *Servidor) Router() *mux.Router {
        return s.router
}
