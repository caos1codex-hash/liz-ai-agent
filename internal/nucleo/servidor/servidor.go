package servidor

import (
        "encoding/json"
        "fmt"
        "net/http"
        "path/filepath"
        "runtime"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
        "github.com/gorilla/mux"
)

// ═══════════════════════════════════════════════════════
// SERVIDOR
// ═══════════════════════════════════════════════════════

// Servidor es el servidor HTTP principal de Liz.
type Servidor struct {
        router    *mux.Router
        log       *logger.Logger
        gestorCfg *config.Gestor
        permisos  *permisos.Sistema
        coordinador *contexto.Coordinador
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
        Estado     string  `json:"estado"`
        Version    string  `json:"version"`
        Uptime     string  `json:"uptime"`
        GoVersion  string  `json:"go_version"`
        Goroutines int     `json:"goroutines"`
        MemoriaMB  float64 `json:"memoria_mb"`
        ConfigOrigen string `json:"config_origen,omitempty"`
        PermisosListos bool `json:"permisos_listos"`
}

// InfoConfigResponse es la respuesta pública del endpoint de configuración.
type InfoConfigResponse struct {
        Version             string `json:"version"`
        Puerto              int    `json:"puerto"`
        Host                string `json:"host"`
        Tema                string `json:"tema"`
        DirectorioTrabajo   string `json:"directorio_trabajo"`
        ModelosDisponibles  int    `json:"modelos_disponibles"`
        NVIDIAConfigurado   bool   `json:"nvidia_configurado"`
        PermisosSolicitar   bool   `json:"permisos_solicitar_al_iniciar"`
        PermisosRecordar   bool   `json:"permisos_recordar_entre_sesiones"`
        ConfigOrigen       string `json:"config_origen,omitempty"`
}

// Nuevo crea e inicializa el servidor HTTP de Liz.
func Nuevo(cfg *config.Configuracion, log *logger.Logger, sisPermisos *permisos.Sistema) *Servidor {
        s := &Servidor{
                router:   mux.NewRouter(),
                log:      log,
                gestorCfg: nil, // se asigna con AsignarGestor
                permisos: sisPermisos,
                inicio:   time.Now(),
                sesionID: generarSesionID(),
        }

        s.configurarRutas()
        return s
}

// NuevoConGestor crea el servidor usando el nuevo Gestor de configuración.
func NuevoConGestor(gestor *config.Gestor, log *logger.Logger, sisPermisos *permisos.Sistema) *Servidor {
        s := &Servidor{
                router:    mux.NewRouter(),
                log:       log,
                gestorCfg: gestor,
                permisos:  sisPermisos,
                inicio:    time.Now(),
                sesionID:  generarSesionID(),
        }

        s.configurarRutas()
        return s
}

// NuevoConContexto crea el servidor con soporte de contexto (Fase 3).
func NuevoConContexto(gestor *config.Gestor, log *logger.Logger, sisPermisos *permisos.Sistema, coord *contexto.Coordinador) *Servidor {
        s := &Servidor{
                router:      mux.NewRouter(),
                log:         log,
                gestorCfg:   gestor,
                permisos:    sisPermisos,
                coordinador: coord,
                inicio:      time.Now(),
                sesionID:    generarSesionID(),
        }

        s.configurarRutas()
        return s
}

// Config retorna la configuración actual (desde el gestor o legacy).
func (s *Servidor) Config() *config.Configuracion {
        if s.gestorCfg != nil {
                cfg := s.gestorCfg.Obtener()
                return &cfg
        }
        return config.Config
}

func generarSesionID() string {
        return fmt.Sprintf("ses_%s", time.Now().Format("20060102_150405"))
}

// SesionID retorna el ID de la sesión actual.
func (s *Servidor) SesionID() string {
        return s.sesionID
}

// ═══════════════════════════════════════════════════════
// RUTAS
// ═══════════════════════════════════════════════════════

func (s *Servidor) configurarRutas() {
        // Middleware global
        s.router.Use(s.middlewareLogging)
        s.router.Use(s.middlewareCORS)
        s.router.Use(s.middlewareRecovery)

        // Middleware de permisos (después de CORS/recovery, antes de handlers)
        s.router.Use(s.permisos.MiddlewareHTTP)

        // ── Healthcheck (público) ──
        s.router.HandleFunc("/api/health", s.handleHealth).Methods("GET", "OPTIONS")

        // ── Configuración (lectura pública, escritura requiere permisos) ──
        s.router.HandleFunc("/api/config", s.handleGetConfig).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/config", s.handlePutConfig).Methods("PUT", "OPTIONS")

        // ── Permisos (públicos para gestión) ──
        s.router.HandleFunc("/api/permisos", s.handleGetPermisos).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/permisos", s.handlePostPermisos).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/permisos", s.handleDeletePermisos).Methods("DELETE", "OPTIONS")
        s.router.HandleFunc("/api/permisos/auditoria", s.handleGetAuditoria).Methods("GET", "OPTIONS")

        // ── Herramientas (protegido — Fase 5) ──
        s.router.HandleFunc("/api/tools", s.handleGetTools).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/tools/{nombre}", s.handleGetTool).Methods("GET", "OPTIONS")

        // ── Orquestador (público — solo lectura) ──
        s.router.HandleFunc("/api/orquestador/modelos", s.handleGetModelos).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/orquestador/metricas", s.handleGetMetricas).Methods("GET", "OPTIONS")

        // ── Conversaciones (protegido — Fase 7) ──
        s.router.HandleFunc("/api/conversations", s.handleGetConversations).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/conversations", s.handlePostConversations).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/conversations/{id}", s.handleDeleteConversation).Methods("DELETE", "OPTIONS")

        // ── Contexto (Fase 3) ──
        s.router.HandleFunc("/api/contexto/proyectos", s.handleGetContextoProyectos).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/contexto/indexar", s.handlePostContextoIndexar).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/contexto/mapa/{proyecto}", s.handleGetContextoMapa).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/contexto/indice/{proyecto}", s.handleGetContextoIndice).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/contexto/fragmento/{proyecto}/{id}", s.handleGetContextoFragmento).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/contexto/buscar/{proyecto}", s.handleGetContextoBuscar).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/contexto/archivo/{proyecto}", s.handleGetContextoArchivo).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/contexto/resumen/{proyecto}", s.handleGetContextoResumen).Methods("GET", "OPTIONS")

        // ── Chat (protegido — Fase 7) ──
        s.router.HandleFunc("/api/chat", s.handleChat).Methods("POST", "OPTIONS")
}

// ═══════════════════════════════════════════════════════
// MIDDLEWARES
// ═══════════════════════════════════════════════════════

func (s *Servidor) middlewareLogging(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                start := time.Now()
                next.ServeHTTP(w, r)
                s.log.Debug("%s %s → %v", r.Method, r.URL.Path, time.Since(start))
        })
}

func (s *Servidor) middlewareCORS(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Access-Control-Allow-Origin", "*")
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

                if r.Method == "OPTIONS" {
                        w.WriteHeader(http.StatusOK)
                        return
                }

                next.ServeHTTP(w, r)
        })
}

func (s *Servidor) middlewareRecovery(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                defer func() {
                        if rec := recover(); rec != nil {
                                s.log.Error("panic recuperado en %s %s: %v", r.Method, r.URL.Path, rec)
                        w.Header().Set("Content-Type", "application/json")
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

func (s *Servidor) handleHealth(w http.ResponseWriter, r *http.Request) {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)

        cfg := s.Config()
        var configOrigen string
        if s.gestorCfg != nil {
                configOrigen = s.gestorCfg.RutaOrigen()
                if configOrigen == "" {
                        configOrigen = "defaults"
                }
        }

        resp := HealthResponse{
                Estado:        "operativo",
                Version:       cfg.Version,
                Uptime:        time.Since(s.inicio).Truncate(time.Second).String(),
                GoVersion:     runtime.Version(),
                Goroutines:    runtime.NumGoroutine(),
                MemoriaMB:     float64(m.Alloc) / 1024 / 1024,
                ConfigOrigen:  configOrigen,
                PermisosListos: s.permisos.TodosConcedidos(),
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: resp})
}

// ═══════════════════════════════════════════════════════
// HANDLERS — CONFIGURACIÓN
// ═══════════════════════════════════════════════════════

func (s *Servidor) handleGetConfig(w http.ResponseWriter, r *http.Request) {
        cfg := s.Config()

        resp := InfoConfigResponse{
                Version:            cfg.Version,
                Puerto:             cfg.Servidor.Puerto,
                Host:               cfg.Servidor.Host,
                Tema:               cfg.Tema,
                DirectorioTrabajo:  cfg.DirectorioTrabajo,
                ModelosDisponibles: len(cfg.NVIDIA.Modelos),
                NVIDIAConfigurado:  cfg.NVIDIA.APIKey != "",
                PermisosSolicitar:  cfg.Permisos.SolicitarAlIniciar,
                PermisosRecordar:  cfg.Permisos.RecordarEntreSesiones,
        }

        if s.gestorCfg != nil {
                r := s.gestorCfg.RutaOrigen()
                if r == "" {
                        r = "defaults"
                }
                resp.ConfigOrigen = r
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: resp})
}

// SolicitudPutConfig es el body para modificar configuración.
type SolicitudPutConfig struct {
        Puerto            *int    `json:"puerto,omitempty"`
        Host              *string `json:"host,omitempty"`
        Tema              *string `json:"tema,omitempty"`
        DirectorioTrabajo *string `json:"directorio_trabajo,omitempty"`
        NVIDIAEndpoint    *string `json:"nvidia_endpoint,omitempty"`
        SolicitarPermisos *bool   `json:"solicitar_permisos,omitempty"`
        RecordarPermisos  *bool   `json:"recordar_permisos,omitempty"`
}

func (s *Servidor) handlePutConfig(w http.ResponseWriter, r *http.Request) {
        var req SolicitudPutConfig
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                s.responderError(w, http.StatusBadRequest, "JSON inválido: %v", err)
                return
        }

        if s.gestorCfg != nil {
                // Fase 2: usar el Gestor con validación y persistencia
                cambios := &config.Configuracion{}
                if req.Puerto != nil {
                        cambios.Servidor.Puerto = *req.Puerto
                }
                if req.Host != nil {
                        cambios.Servidor.Host = *req.Host
                }
                if req.Tema != nil {
                        cambios.Tema = *req.Tema
                }
                if req.DirectorioTrabajo != nil {
                        cambios.DirectorioTrabajo = *req.DirectorioTrabajo
                }
                if req.NVIDIAEndpoint != nil {
                        cambios.NVIDIA.Endpoint = *req.NVIDIAEndpoint
                }
                if req.SolicitarPermisos != nil {
                        cambios.Permisos.SolicitarAlIniciar = *req.SolicitarPermisos
                }
                if req.RecordarPermisos != nil {
                        cambios.Permisos.RecordarEntreSesiones = *req.RecordarPermisos
                }

                nueva, err := s.gestorCfg.Modificar(cambios)
                if err != nil {
                        s.responderError(w, http.StatusBadRequest, "%v", err)
                        return
                }

                s.log.Info("configuración modificada y persistida vía Gestor")
                _ = nueva
        } else {
                // Legacy Fase 1: modificación directa sin persistencia
                cfg := config.Config
                if req.Puerto != nil {
                        if *req.Puerto < 1 || *req.Puerto > 65535 {
                                s.responderError(w, http.StatusBadRequest, "puerto debe estar entre 1 y 65535")
                                return
                        }
                        cfg.Servidor.Puerto = *req.Puerto
                }
                if req.Host != nil && *req.Host != "" {
                        cfg.Servidor.Host = *req.Host
                }
                if req.Tema != nil {
                        cfg.Tema = *req.Tema
                }
                if req.DirectorioTrabajo != nil {
                        cfg.DirectorioTrabajo = *req.DirectorioTrabajo
                }
                s.log.Info("configuración actualizada en runtime (legacy)")
        }

        s.handleGetConfig(w, r)
}

// ═══════════════════════════════════════════════════════
// HANDLERS — PERMISOS
// ═══════════════════════════════════════════════════════

func (s *Servidor) handleGetPermisos(w http.ResponseWriter, r *http.Request) {
        estado := s.permisos.Estado()
        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: estado})
}

type SolicitudConcederPermisos struct {
        ConcederTodos bool     `json:"conceder_todos"`
        Permisos      []string `json:"permisos,omitempty"`
}

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

// handleDeletePermisos revoca/resetea los permisos. (Fase 2)
func (s *Servidor) handleDeletePermisos(w http.ResponseWriter, r *http.Request) {
        if err := s.permisos.Resetear(); err != nil {
                s.responderError(w, http.StatusInternalServerError, "error reseteando permisos: %v", err)
                return
        }

        s.log.Info("todos los permisos revocados")
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "mensaje": "todos los permisos han sido revocados",
                },
        })
}

// handleGetAuditoria retorna el log de auditoría de permisos. (Fase 2)
func (s *Servidor) handleGetAuditoria(w http.ResponseWriter, r *http.Request) {
        limit := 50
        if l := r.URL.Query().Get("limit"); l != "" {
                fmt.Sscanf(l, "%d", &limit)
        }

        auditoria := s.permisos.Auditoria(limit)

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "auditoria": auditoria,
                        "total":    s.permisos.TotalAuditoria(),
                        "mostrados": len(auditoria),
                },
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — HERRAMIENTAS (stubs Fase 5)
// ═══════════════════════════════════════════════════════

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

func (s *Servidor) handleGetTool(w http.ResponseWriter, r *http.Request) {
        nombre := mux.Vars(r)["nombre"]
        s.responderJSON(w, http.StatusNotFound, RespuestaAPI{
                Exito: false,
                Error: fmt.Sprintf("herramienta '%s' no encontrada (Fase 5 pendiente)", nombre),
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — ORQUESTADOR (stubs Fase 4)
// ═══════════════════════════════════════════════════════

func (s *Servidor) handleGetModelos(w http.ResponseWriter, r *http.Request) {
        cfg := s.Config()

        type ModeloRes struct {
                ID        string   `json:"id"`
                Nombre    string   `json:"nombre"`
                Tipo      []string `json:"tipo"`
                Velocidad string   `json:"velocidad"`
                Prioridad int      `json:"prioridad"`
        }

        modelos := make([]ModeloRes, len(cfg.NVIDIA.Modelos))
        for i, m := range cfg.NVIDIA.Modelos {
                modelos[i] = ModeloRes{
                        ID: m.ID, Nombre: m.Nombre, Tipo: m.Tipo,
                        Velocidad: m.Velocidad, Prioridad: m.Prioridad,
                }
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "modelos":  modelos,
                        "total":    len(modelos),
                        "endpoint": cfg.NVIDIA.Endpoint,
                },
        })
}

func (s *Servidor) handleGetMetricas(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{"metricas": map[string]interface{}{}, "fase": "pendiente (Fase 4)"},
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — CONVERSACIONES (stubs Fase 7)
// ═══════════════════════════════════════════════════════

func (s *Servidor) handleGetConversations(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{"conversaciones": []interface{}{}, "total": 0, "fase": "pendiente (Fase 7)"},
        })
}

func (s *Servidor) handlePostConversations(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusNotImplemented, RespuestaAPI{
                Exito: false, Error: "creación de conversaciones disponible en Fase 7",
        })
}

func (s *Servidor) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusNotImplemented, RespuestaAPI{
                Exito: false, Error: "eliminación de conversaciones disponible en Fase 7",
        })
}

// ═══════════════════════════════════════════════════════
// HANDLERS — CONTEXTO (Fase 3)
// ═══════════════════════════════════════════════════════

// handleGetContextoProyectos lista todos los proyectos indexados.
func (s *Servidor) handleGetContextoProyectos(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderJSON(w, http.StatusOK, RespuestaAPI{
                        Exito: true,
                        Datos: map[string]interface{}{"proyectos": []interface{}{}, "total": 0},
                })
                return
        }

        proyectos := s.coordinador.ListarProyectos()
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{"proyectos": proyectos, "total": len(proyectos)},
        })
}

// handlePostContextoIndexar indexa un proyecto (genera mapa + fragmentos + índice).
func (s *Servidor) handlePostContextoIndexar(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderError(w, http.StatusServiceUnavailable, "sistema de contexto no inicializado")
                return
        }

        var req contexto.SolicitudIndexar
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                s.responderError(w, http.StatusBadRequest, "JSON inválido: %v", err)
                return
        }

        if req.Ruta == "" {
                s.responderError(w, http.StatusBadRequest, "debes especificar 'ruta' del proyecto")
                return
        }

        estado, err := s.coordinador.IndexarProyecto(req.Ruta)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, "error indexando proyecto: %v", err)
                return
        }

        s.log.Info("proyecto indexado: %s (%d archivos, %d fragmentos)",
                estado.Nombre, estado.TotalArchivos, estado.TotalFragmentos)

        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: estado})
}

// handleGetContextoMapa retorna el mapa de un proyecto.
func (s *Servidor) handleGetContextoMapa(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderError(w, http.StatusServiceUnavailable, "sistema de contexto no inicializado")
                return
        }

        proyecto := mux.Vars(r)["proyecto"]
        mapaProy, err := s.coordinador.ObtenerMapa(proyecto)
        if err != nil {
                s.responderJSON(w, http.StatusNotFound, RespuestaAPI{
                        Exito: false,
                        Error: fmt.Sprintf("mapa del proyecto '%s' no encontrado: %v", proyecto, err),
                })
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: mapaProy})
}

// handleGetContextoIndice retorna el índice de un proyecto.
func (s *Servidor) handleGetContextoIndice(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderError(w, http.StatusServiceUnavailable, "sistema de contexto no inicializado")
                return
        }

        proyecto := mux.Vars(r)["proyecto"]
        ind, err := s.coordinador.ObtenerIndice(proyecto)
        if err != nil {
                s.responderJSON(w, http.StatusNotFound, RespuestaAPI{
                        Exito: false,
                        Error: fmt.Sprintf("índice del proyecto '%s' no encontrado: %v", proyecto, err),
                })
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: ind})
}

// handleGetContextoFragmento retorna un fragmento específico.
func (s *Servidor) handleGetContextoFragmento(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderError(w, http.StatusServiceUnavailable, "sistema de contexto no inicializado")
                return
        }

        proyecto := mux.Vars(r)["proyecto"]
        id := mux.Vars(r)["id"]

        frag, err := s.coordinador.ObtenerFragmento(proyecto, id)
        if err != nil {
                s.responderJSON(w, http.StatusNotFound, RespuestaAPI{
                        Exito: false,
                        Error: fmt.Sprintf("fragmento '%s' no encontrado en proyecto '%s'", id, proyecto),
                })
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: frag})
}

// handleGetContextoBuscar busca en el índice de un proyecto.
func (s *Servidor) handleGetContextoBuscar(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderError(w, http.StatusServiceUnavailable, "sistema de contexto no inicializado")
                return
        }

        proyecto := mux.Vars(r)["proyecto"]
        patron := r.URL.Query().Get("q")
        if patron == "" {
                s.responderError(w, http.StatusBadRequest, "debes especificar parámetro 'q' para buscar")
                return
        }

        resultados := s.coordinador.BuscarEnIndice(proyecto, patron)
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "resultados": resultados,
                        "total":    len(resultados),
                        "patron":   patron,
                },
        })
}

// handleGetContextoArchivo retorna los fragmentos de un archivo específico.
func (s *Servidor) handleGetContextoArchivo(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderError(w, http.StatusServiceUnavailable, "sistema de contexto no inicializado")
                return
        }

        proyecto := mux.Vars(r)["proyecto"]
        rutaArchivo := r.URL.Query().Get("ruta")
        if rutaArchivo == "" {
                s.responderError(w, http.StatusBadRequest, "debes especificar parámetro 'ruta' del archivo")
                return
        }

        frags, err := s.coordinador.ObtenerFragmentosPorRuta(proyecto, rutaArchivo)
        if err != nil {
                s.responderJSON(w, http.StatusNotFound, RespuestaAPI{
                        Exito: false,
                        Error: fmt.Sprintf("fragmentos de '%s' no encontrados: %v", rutaArchivo, err),
                })
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "fragmentos": frags,
                        "total":    len(frags),
                        "ruta":     rutaArchivo,
                },
        })
}

// handleGetContextoResumen genera un resumen detallado de un archivo.
func (s *Servidor) handleGetContextoResumen(w http.ResponseWriter, r *http.Request) {
        if s.coordinador == nil {
                s.responderError(w, http.StatusServiceUnavailable, "sistema de contexto no inicializado")
                return
        }

        proyecto := mux.Vars(r)["proyecto"]
        rutaArchivo := r.URL.Query().Get("ruta")
        if rutaArchivo == "" {
                s.responderError(w, http.StatusBadRequest, "debes especificar parámetro 'ruta' del archivo")
                return
        }

        // Obtener la ruta absoluta del proyecto
        proyectos := s.coordinador.ListarProyectos()
        var rutaAbs string
        for _, p := range proyectos {
                if p.Nombre == proyecto {
                        rutaAbs = p.Ruta
                        break
                }
        }
        if rutaAbs == "" {
                s.responderError(w, http.StatusNotFound, "proyecto '%s' no encontrado", proyecto)
                return
        }

        rutaCompleta := filepath.Join(rutaAbs, rutaArchivo)
        res, err := s.coordinador.ObtenerResumen(proyecto, rutaArchivo, rutaCompleta)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, "error generando resumen: %v", err)
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{Exito: true, Datos: res})
}

// ═══════════════════════════════════════════════════════
// HANDLERS — CHAT (stub Fase 7)
// ═══════════════════════════════════════════════════════

func (s *Servidor) handleChat(w http.ResponseWriter, r *http.Request) {
        s.responderJSON(w, http.StatusNotImplemented, RespuestaAPI{
                Exito: false, Error: "chat disponible en Fase 7 (Pipeline de Chat)",
        })
}

// ═══════════════════════════════════════════════════════
// UTILIDADES
// ═══════════════════════════════════════════════════════

func (s *Servidor) responderJSON(w http.ResponseWriter, statusCode int, resp RespuestaAPI) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(statusCode)
        json.NewEncoder(w).Encode(resp)
}

func (s *Servidor) responderError(w http.ResponseWriter, statusCode int, formato string, args ...interface{}) {
        mensaje := fmt.Sprintf(formato, args...)
        s.log.Error(mensaje)
        s.responderJSON(w, statusCode, RespuestaAPI{Exito: false, Error: mensaje})
}

// Iniciar arranca el servidor HTTP de Liz.
func (s *Servidor) Iniciar() error {
        cfg := s.Config()
        dir := fmt.Sprintf("%s:%d", cfg.Servidor.Host, cfg.Servidor.Puerto)

        s.log.Info("═══════════════════════════════════════════════")
        s.log.Info("  Liz AI Agent v%s", cfg.Version)
        s.log.Info("  Servidor arrancando en http://%s", dir)
        s.log.Info("  Sesión: %s", s.sesionID)
        s.log.Info("═══════════════════════════════════════════════")

        return http.ListenAndServe(dir, s.router)
}

// Router retorna el router mux para testing.
func (s *Servidor) Router() *mux.Router {
        return s.router
}
