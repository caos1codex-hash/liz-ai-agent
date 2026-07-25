package servidor

import (
        "context"
        "encoding/json"
        "fmt"
        "net/http"
        "os"
        "os/signal"
        "path/filepath"
        "strconv"
        "syscall"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/auto_creacion"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/registro"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/memoria"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
        "github.com/gorilla/mux"
)

// ============================================================================
// Tipos de Respuesta
// ============================================================================

// RespuestaAPI es el formato estándar de todas las respuestas de la API.
type RespuestaAPI struct {
        Exito    bool        `json:"exito"`
        Mensaje  string      `json:"mensaje,omitempty"`
        Datos    interface{} `json:"datos,omitempty"`
        Error    string      `json:"error,omitempty"`
        Timestamp string     `json:"timestamp"`
}

// RespuestaConfigPut es el body para modificar configuración.
type RespuestaConfigPut struct {
        Campos map[string]string `json:"campos"`
}

// RespuestaPermisoPost es el body para conceder un permiso.
type RespuestaPermisoPost struct {
        Tipo  string `json:"tipo"`
        Nivel string `json:"nivel"`
        Razon string `json:"razon"`
}

// ============================================================================
// Servidor HTTP
// ============================================================================

// Servidor es el servidor HTTP principal de Liz.
// Contiene el router, las dependencias inyectadas y la configuración.
type Servidor struct {
        router      *mux.Router
        httpServ    *http.Server
        gestorCfg   *config.Gestor
        gestorPer   *permisos.Gestor
        gestorCtx   *contexto.Coordinador    // opcional, se inyecta en Fase 3
        gestorMem   *memoria.Gestor          // opcional, se inyecta en Fase 3.5+ (memoria conversacional)
        orquestador *orquestador.Orquestador // opcional, se inyecta en Fase 4
        catalogo    *registro.Catalogo       // opcional, se inyecta en Fase 5 (herramientas)
        autoGestor  *auto_creacion.Gestor    // opcional, se inyecta en Fase 6 (auto-creación)
        log         *logger.Logger
        inicio      time.Time
}

// ============================================================================
// Inicialización
// ============================================================================

// Nuevo crea un nuevo servidor con todas las dependencias inyectadas.
// Recibe los gestores de configuración y permisos ya inicializados.
func Nuevo(gestorCfg *config.Gestor, gestorPer *permisos.Gestor, log *logger.Logger) *Servidor {
        s := &Servidor{
                router:    mux.NewRouter().StrictSlash(true),
                gestorCfg: gestorCfg,
                gestorPer: gestorPer,
                log:       log,
                inicio:    time.Now(),
        }

        s.registrarRutas()
        s.registrarMiddlewares()

        puerto := gestorCfg.ObtenerPuerto()
        host := gestorCfg.ObtenerHost()

        s.httpServ = &http.Server{
                Addr:         fmt.Sprintf("%s:%d", host, puerto),
                Handler:      s.router,
                ReadTimeout:  15 * time.Second,
                WriteTimeout: 30 * time.Second,
                IdleTimeout:  60 * time.Second,
        }

        return s
}

// ============================================================================
// Registro de Rutas
// ============================================================================

// registrarRutas configura todos los endpoints de la API.
func (s *Servidor) registrarRutas() {
        // --- Health ---
        s.router.HandleFunc("/api/v1/health", s.handlerHealth).Methods("GET", "OPTIONS")

        // --- Configuración ---
        s.router.HandleFunc("/api/v1/config", s.handlerConfigGet).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/config", s.handlerConfigPut).Methods("PUT", "OPTIONS")
        s.router.HandleFunc("/api/v1/config/esquema", s.handlerConfigEsquema).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/config/cambios", s.handlerConfigCambios).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/config/recargar", s.handlerConfigRecargar).Methods("POST", "OPTIONS")

        // --- Permisos ---
        s.router.HandleFunc("/api/v1/permisos", s.handlerPermisosGet).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/permisos", s.handlerPermisosPost).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/permisos/resumen", s.handlerPermisosResumen).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/permisos/auditoria", s.handlerPermisosAuditoria).Methods("GET", "OPTIONS")

        // --- Contexto (Fase 3) ---
        s.router.HandleFunc("/api/v1/contexto/proyectos", s.handlerContextoProyectos).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos", s.handlerContextoIndexar).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}", s.handlerContextoEliminar).Methods("DELETE", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/mapa", s.handlerContextoMapa).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/indice", s.handlerContextoIndice).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/arbol", s.handlerContextoArbol).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/fragmentos", s.handlerContextoFragmentosPorRuta).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/fragmentos/{id}", s.handlerContextoFragmento).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/buscar", s.handlerContextoBuscar).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/resumen", s.handlerContextoResumen).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/reindexar", s.handlerContextoReindexar).Methods("POST", "OPTIONS")

        // --- Contexto Fase 3.5 (sistema world-class) ---
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/simbolos", s.handlerContextoSimbolos).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/grafo", s.handlerContextoGrafo).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/importancia", s.handlerContextoImportancia).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/buscar-hibrido", s.handlerContextoBuscarHibrido).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/mapa-repo", s.handlerContextoMapaRepo).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/proyectos/{nombre}/empaquetar", s.handlerContextoEmpaquetar).Methods("POST", "OPTIONS")

        // --- Stubs Fase 4+ (sin implementar) ---
        // Fase 4 (orquestador) — implementado en iter5
        s.router.HandleFunc("/api/v1/orquestador", s.handlerOrquestadorEstado).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/orquestador/modelos", s.handlerOrquestadorModelos).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/orquestador/metricas", s.handlerOrquestadorMetricas).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/orquestador/completar", s.handlerOrquestadorCompletar).Methods("POST", "OPTIONS")

        // --- Memoria conversacional (Fase 3.5+) ---
        s.router.HandleFunc("/api/v1/memoria/sesiones", s.handlerMemoriaSesiones).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/sesiones", s.handlerMemoriaNuevaSesion).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/sesiones/{id}", s.handlerMemoriaObtenerSesion).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/sesiones/{id}/cerrar", s.handlerMemoriaCerrarSesion).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/sesiones/{id}/mensajes", s.handlerMemoriaAgregarMensaje).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/hechos", s.handlerMemoriaHechos).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/hechos", s.handlerMemoriaAgregarHecho).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/hechos/{id}", s.handlerMemoriaEliminarHecho).Methods("DELETE", "OPTIONS")
        s.router.HandleFunc("/api/v1/memoria/contexto", s.handlerMemoriaContexto).Methods("GET", "OPTIONS")

        // --- Tracker de ediciones (Fase 4) ---
        s.router.HandleFunc("/api/v1/contexto/tracker/edicion", s.handlerTrackerRegistrarEdicion).Methods("POST", "OPTIONS")
        s.router.HandleFunc("/api/v1/contexto/tracker/recientes", s.handlerTrackerRecientes).Methods("GET", "OPTIONS")

        // --- Herramientas (Fase 5) ---
        s.registrarRutasHerramientas()

        // --- Auto-creación de herramientas (Fase 6) ---
        s.registrarRutasFase6()

        // --- Stubs fases futuras ---
        s.router.HandleFunc("/api/v1/modelos", s.handlerStub("modelos")).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/conversations", s.handlerStub("conversations")).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/chat", s.handlerStub("chat")).Methods("POST", "OPTIONS")
}

// ConCoordinador inyecta el coordinador de contexto en el servidor.
// Debe llamarse antes de Iniciar().
func (s *Servidor) ConCoordinador(coordinador *contexto.Coordinador) *Servidor {
        s.gestorCtx = coordinador
        return s
}

// ConMemoria inyecta el gestor de memoria conversacional en el servidor.
// Debe llamarse antes de Iniciar().
func (s *Servidor) ConMemoria(g *memoria.Gestor) *Servidor {
        s.gestorMem = g
        return s
}

// ConOrquestador inyecta el orquestador multi-modelo en el servidor.
// Debe llamarse antes de Iniciar().
func (s *Servidor) ConOrquestador(o *orquestador.Orquestador) *Servidor {
        s.orquestador = o
        return s
}

// requiereCoordinador verifica que el coordinador esté disponible.
// Si no lo está, responde 503 Service Unavailable y retorna false.
func (s *Servidor) requiereCoordinador(w http.ResponseWriter) bool {
        if s.gestorCtx == nil {
                s.responderError(w, http.StatusServiceUnavailable,
                        "coordinador de contexto no disponible (Fase 3 no inicializada)")
                return false
        }
        return true
}

// requiereMemoria verifica que el gestor de memoria esté disponible.
func (s *Servidor) requiereMemoria(w http.ResponseWriter) bool {
        if s.gestorMem == nil {
                s.responderError(w, http.StatusServiceUnavailable,
                        "sistema de memoria no disponible (Fase 3.5+ no inicializada)")
                return false
        }
        return true
}

// requiereOrquestador verifica que el orquestador esté disponible.
func (s *Servidor) requiereOrquestador(w http.ResponseWriter) bool {
        if s.orquestador == nil {
                s.responderError(w, http.StatusServiceUnavailable,
                        "orquestador no disponible (Fase 4 no inicializada o sin API key NVIDIA)")
                return false
        }
        return true
}

// registrarMiddlewares aplica los middlewares globales al router.
func (s *Servidor) registrarMiddlewares() {
        s.router.Use(s.middlewareLogging)
        s.router.Use(s.middlewareCORS)
        s.router.Use(s.middlewareRecuperacionPanic)
}

// ============================================================================
// Handlers — Health
// ============================================================================

// handlerHealth retorna el estado del servidor y sus dependencias.
func (s *Servidor) handlerHealth(w http.ResponseWriter, r *http.Request) {
        health := map[string]interface{}{
                "estado":  "operativo",
                "nombre":  s.gestorCfg.ObtenerNombre(),
                "version": s.gestorCfg.ObtenerVersion(),
                "uptime":  time.Since(s.inicio).String(),
                "permisos": map[string]bool{
                        "habilitado": s.gestorPer.EstaHabilitado(),
                },
                "configuracion": map[string]interface{}{
                        "puerto": s.gestorCfg.ObtenerPuerto(),
                        "host":   s.gestorCfg.ObtenerHost(),
                },
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     health,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// ============================================================================
// Handlers — Configuración
// ============================================================================

// handlerConfigGet retorna la configuración completa (sin secrets).
func (s *Servidor) handlerConfigGet(w http.ResponseWriter, r *http.Request) {
        cfg := s.gestorCfg.Obtener()

        // Sanitizar: remover API keys de la respuesta
        for i := range cfg.Modelos {
                if cfg.Modelos[i].APIKey != "" {
                        cfg.Modelos[i].APIKey = "***"
                }
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Configuración obtenida exitosamente",
                Datos:     cfg,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerConfigPut modifica campos de configuración.
func (s *Servidor) handlerConfigPut(w http.ResponseWriter, r *http.Request) {
        var req RespuestaConfigPut
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                s.responderError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
                return
        }

        if len(req.Campos) == 0 {
                s.responderError(w, http.StatusBadRequest, "no se especificaron campos a modificar")
                return
        }

        // Validar cada campo antes de aplicar
        for ruta, valor := range req.Campos {
                if err := s.gestorCfg.ValidarCampo(ruta, valor); err != nil {
                        s.responderError(w, http.StatusBadRequest, fmt.Sprintf("validación falló para '%s': %s", ruta, err))
                        return
                }
        }

        // Aplicar cambios atómicamente
        if err := s.gestorCfg.EstablecerMultiple(req.Campos); err != nil {
                s.responderError(w, http.StatusUnprocessableEntity, err.Error())
                return
        }

        // Persistir
        if err := s.gestorCfg.Guardar(); err != nil {
                s.log.Warn("Error al guardar configuración: %v", err)
                // No fallar — el cambio está en memoria
        }

        cambios := s.gestorCfg.ObtenerCambios()
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   fmt.Sprintf("%d campos modificados exitosamente", len(req.Campos)),
                Datos:     cambios,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerConfigEsquema retorna el esquema de configuración para documentación.
func (s *Servidor) handlerConfigEsquema(w http.ResponseWriter, r *http.Request) {
        esquema := s.gestorCfg.Esquema()

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Esquema de configuración",
                Datos:     esquema,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerConfigCambios retorna el historial de cambios de configuración.
func (s *Servidor) handlerConfigCambios(w http.ResponseWriter, r *http.Request) {
        cambios := s.gestorCfg.ObtenerCambios()

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     cambios,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerConfigRecargar recarga la configuración desde disco.
func (s *Servidor) handlerConfigRecargar(w http.ResponseWriter, r *http.Request) {
        cambios, err := s.gestorCfg.Recargar()
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, "Error al recargar: "+err.Error())
                return
        }

        s.log.Info("Configuración recargada desde disco (%d cambios detectados)", len(cambios))

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   fmt.Sprintf("Configuración recargada, %d cambios detectados", len(cambios)),
                Datos:     cambios,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// ============================================================================
// Handlers — Permisos
// ============================================================================

// handlerPermisosGet retorna el estado completo de permisos.
func (s *Servidor) handlerPermisosGet(w http.ResponseWriter, r *http.Request) {
        datos := s.gestorPer.FormatearPermisosParaAPI()

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   "Estado de permisos",
                Datos:     datos,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerPermisosPost concede un permiso individual.
func (s *Servidor) handlerPermisosPost(w http.ResponseWriter, r *http.Request) {
        var req RespuestaPermisoPost
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                s.responderError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
                return
        }

        if req.Tipo == "" {
                s.responderError(w, http.StatusBadRequest, "campo 'tipo' es requerido")
                return
        }

        tipo := permisos.TipoPermiso(req.Tipo)

        // Validar que el tipo exista
        valido := false
        for _, t := range permisos.TodosLosPermisos {
                if t == tipo {
                        valido = true
                        break
                }
        }
        if !valido {
                s.responderError(w, http.StatusBadRequest, fmt.Sprintf("tipo de permiso inválido: %s", req.Tipo))
                return
        }

        nivel := permisos.NivelPermiso(req.Nivel)
        if nivel == "" {
                nivel = permisos.NivelTotal
        }

        razon := req.Razon
        if razon == "" {
                razon = "Concedido via API"
        }

        if err := s.gestorPer.Conceder(tipo, nivel, "api", razon); err != nil {
                s.responderError(w, http.StatusInternalServerError, err.Error())
                return
        }

        s.log.Info("Permiso %s concedido con nivel %s via API", tipo, nivel)

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   fmt.Sprintf("Permiso %s concedido", tipo),
                Datos:     s.gestorPer.ObtenerPermiso(tipo),
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerPermisosResumen retorna un resumen de los permisos.
func (s *Servidor) handlerPermisosResumen(w http.ResponseWriter, r *http.Request) {
        resumen := s.gestorPer.ObtenerResumen()

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     resumen,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerPermisosAuditoria retorna el historial de auditoría.
func (s *Servidor) handlerPermisosAuditoria(w http.ResponseWriter, r *http.Request) {
        auditoria := s.gestorPer.ObtenerAuditoriaReciente(50)

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     auditoria,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// ============================================================================
// Handlers — Stubs (Fases futuras)
// ============================================================================

// handlerStub retorna un 501 Not Implemented para endpoints de fases futuras.
func (s *Servidor) handlerStub(nombre string) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
                s.responderJSON(w, http.StatusNotImplemented, RespuestaAPI{
                        Exito:     false,
                        Mensaje:   fmt.Sprintf("Endpoint /%s no implementado aún — planificado para fases posteriores", nombre),
                        Timestamp: time.Now().Format(time.RFC3339),
                })
        }
}

// ============================================================================
// Middlewares
// ============================================================================

// middlewareLogging registra cada petición HTTP en el logger.
func (s *Servidor) middlewareLogging(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                inicio := time.Now()

                // Capturar el status code
                rec := &responseCapture{ResponseWriter: w, statusCode: http.StatusOK}
                next.ServeHTTP(rec, r)

                duracion := time.Since(inicio)
                s.log.Info("%s %s → %d (%s)", r.Method, r.URL.Path, rec.statusCode, duracion)
        })
}

// middlewareCORS agrega headers de CORS a todas las respuestas.
func (s *Servidor) middlewareCORS(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Access-Control-Allow-Origin", "*")
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
                w.Header().Set("Access-Control-Max-Age", "86400")

                if r.Method == "OPTIONS" {
                        w.WriteHeader(http.StatusOK)
                        return
                }

                next.ServeHTTP(w, r)
        })
}

// middlewareRecuperacionPanic recupera panics y retorna 500.
func (s *Servidor) middlewareRecuperacionPanic(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                defer func() {
                        if err := recover(); err != nil {
                                s.log.Error("PANIC recuperado en %s %s: %v", r.Method, r.URL.Path, err)
                                s.responderError(w, http.StatusInternalServerError, "Error interno del servidor")
                        }
                }()

                next.ServeHTTP(w, r)
        })
}

// ============================================================================
// Utilidades de Respuesta
// ============================================================================

// responderJSON envía una respuesta JSON al cliente.
func (s *Servidor) responderJSON(w http.ResponseWriter, status int, datos interface{}) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(status)
        json.NewEncoder(w).Encode(datos)
}

// responderError envía una respuesta de error JSON al cliente.
func (s *Servidor) responderError(w http.ResponseWriter, status int, mensaje string) {
        s.responderJSON(w, status, RespuestaAPI{
                Exito:     false,
                Error:     mensaje,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// responseCapture captura el status code de la respuesta para el middleware de logging.
type responseCapture struct {
        http.ResponseWriter
        statusCode int
}

func (rc *responseCapture) WriteHeader(code int) {
        rc.statusCode = code
        rc.ResponseWriter.WriteHeader(code)
}

// ============================================================================
// Ciclo de Vida del Servidor
// ============================================================================

// inicio marca el momento en que se creó el servidor (para uptime).
var inicio = time.Now()

// Iniciar arranca el servidor HTTP con graceful shutdown.
// Bloquea hasta que el servidor se detenga.
func (s *Servidor) Iniciar() error {
        // Canal para capturar señales del OS
        chanSignal := make(chan os.Signal, 1)
        signal.Notify(chanSignal, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

        // Canal de errores del servidor
        chanError := make(chan error, 1)

        // Iniciar servidor en goroutine
        go func() {
                puerto := s.gestorCfg.ObtenerPuerto()
                host := s.gestorCfg.ObtenerHost()
                s.log.Info( "Servidor Liz iniciado en %s:%d", host, puerto)
                if err := s.httpServ.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                        chanError <- err
                }
        }()

        // Esperar señal o error
        select {
        case err := <-chanError:
                return fmt.Errorf("error del servidor: %w", err)
        case sig := <-chanSignal:
                s.log.Info( "Señal recibida: %v — iniciando shutdown graceful", sig)

                // SIGHUP → recargar configuración
                if sig == syscall.SIGHUP {
                        s.log.Info( "SIGHUP recibido — recargando configuración")
                        if cambios, err := s.gestorCfg.Recargar(); err != nil {
                                s.log.Error( "Error al recargar configuración: %v", err)
                        } else {
                                s.log.Info( "Configuración recargada: %d cambios", len(cambios))
                        }
                        // No cerrar, seguir ejecutando
                        return nil
                }

                // SIGINT/SIGTERM → shutdown graceful
                ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
                defer cancel()

                if err := s.httpServ.Shutdown(ctx); err != nil {
                        return fmt.Errorf("error en shutdown graceful: %w", err)
                }

                s.log.Info( "Servidor detenido correctamente")
        }

        return nil
}

// Detener fuerza la detención del servidor.
func (s *Servidor) Detener() error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return s.httpServ.Shutdown(ctx)
}
// ============================================================================
// Handlers — Contexto (Fase 3)
// ============================================================================

// SolicitudIndexarProyecto es el body para POST /api/v1/contexto/proyectos.
type SolicitudIndexarProyecto struct {
        Ruta string `json:"ruta"`
}

// SolicitudReindexar es el body para POST /api/v1/contexto/proyectos/{nombre}/reindexar.
type SolicitudReindexar struct {
        Ruta string `json:"ruta"` // ruta relativa del archivo a reindexar (opcional)
}

// handlerContextoProyectos — GET /api/v1/contexto/proyectos
// Lista todos los proyectos indexados.
func (s *Servidor) handlerContextoProyectos(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        proyectos := s.gestorCtx.ListarProyectos()
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     proyectos,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoIndexar — POST /api/v1/contexto/proyectos
// Indexa un nuevo proyecto (o re-indexa uno existente).
// Body: {"ruta": "/path/al/proyecto"}
func (s *Servidor) handlerContextoIndexar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        var req SolicitudIndexarProyecto
        if err := s.parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "body inválido: "+err.Error())
                return
        }

        if req.Ruta == "" {
                s.responderError(w, http.StatusBadRequest, "campo 'ruta' es requerido")
                return
        }

        estado, err := s.gestorCtx.IndexarProyecto(req.Ruta)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, "error indexando: "+err.Error())
                return
        }

        s.responderJSON(w, http.StatusCreated, RespuestaAPI{
                Exito:     true,
                Mensaje:   "proyecto indexado correctamente",
                Datos:     estado,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoEliminar — DELETE /api/v1/contexto/proyectos/{nombre}
// Elimina un proyecto y todos sus datos del disco.
func (s *Servidor) handlerContextoEliminar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        vars := mux.Vars(r)
        nombre := vars["nombre"]
        if nombre == "" {
                s.responderError(w, http.StatusBadRequest, "nombre de proyecto requerido")
                return
        }

        if err := s.gestorCtx.EliminarProyecto(nombre); err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   "proyecto eliminado",
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoMapa — GET /api/v1/contexto/proyectos/{nombre}/mapa
// Retorna el mapa (catálogo de la biblioteca) del proyecto.
func (s *Servidor) handlerContextoMapa(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        mapa, err := s.gestorCtx.ObtenerMapa(nombre)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     mapa,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoIndice — GET /api/v1/contexto/proyectos/{nombre}/indice
// Retorna el índice (mapa plano de archivos → fragmentos) del proyecto.
func (s *Servidor) handlerContextoIndice(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        indice, err := s.gestorCtx.ObtenerIndice(nombre)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     indice,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoArbol — GET /api/v1/contexto/proyectos/{nombre}/arbol
// Retorna la estructura jerárquica (árbol de directorios) del proyecto.
func (s *Servidor) handlerContextoArbol(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        arbol, err := s.gestorCtx.ObtenerArbol(nombre)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     arbol,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoFragmentosPorRuta — GET /api/v1/contexto/proyectos/{nombre}/fragmentos?ruta=X
// Retorna todos los fragmentos de un archivo (por su ruta relativa).
func (s *Servidor) handlerContextoFragmentosPorRuta(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        ruta := r.URL.Query().Get("ruta")
        if ruta == "" {
                s.responderError(w, http.StatusBadRequest, "parámetro 'ruta' es requerido")
                return
        }

        frags, err := s.gestorCtx.ObtenerFragmentosPorRuta(nombre, ruta)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     frags,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoFragmento — GET /api/v1/contexto/proyectos/{nombre}/fragmentos/{id}
// Retorna un fragmento por su ID.
// Esto es la mitad "Liz entrega solo ese archivo" del ciclo del catálogo.
func (s *Servidor) handlerContextoFragmento(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        vars := mux.Vars(r)
        nombre := vars["nombre"]
        id := vars["id"]

        frag, err := s.gestorCtx.ObtenerFragmento(nombre, id)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     frag,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoBuscar — GET /api/v1/contexto/proyectos/{nombre}/buscar?patron=X
// Busca en el índice por patrón (substring case-insensitive).
func (s *Servidor) handlerContextoBuscar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        patron := r.URL.Query().Get("patron")
        if patron == "" {
                s.responderError(w, http.StatusBadRequest, "parámetro 'patron' es requerido")
                return
        }

        resultados := s.gestorCtx.BuscarEnIndice(nombre, patron)
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     resultados,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoResumen — GET /api/v1/contexto/proyectos/{nombre}/resumen?ruta=X
// Retorna el resumen detallado de un archivo (exportados, importados, complejidad).
// Usa cache/persistencia; usar ?forzar=true para regenerar.
func (s *Servidor) handlerContextoResumen(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        rutaRelativa := r.URL.Query().Get("ruta")
        if rutaRelativa == "" {
                s.responderError(w, http.StatusBadRequest, "parámetro 'ruta' es requerido")
                return
        }
        forzar := r.URL.Query().Get("forzar") == "true"

        // Necesitamos la ruta absoluta del proyecto para resolver el archivo
        proyectos := s.gestorCtx.ListarProyectos()
        var rutaProyecto string
        for _, p := range proyectos {
                if p.Nombre == nombre {
                        rutaProyecto = p.Ruta
                        break
                }
        }
        if rutaProyecto == "" {
                s.responderError(w, http.StatusNotFound, "proyecto no encontrado")
                return
        }

        rutaAbsoluta := filepath.Join(rutaProyecto, rutaRelativa)

        var resumen interface{}
        var err error
        if forzar {
                resumen, err = s.gestorCtx.ForzarResumen(nombre, rutaRelativa, rutaAbsoluta)
        } else {
                resumen, err = s.gestorCtx.ObtenerResumen(nombre, rutaRelativa, rutaAbsoluta)
        }
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     resumen,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoReindexar — POST /api/v1/contexto/proyectos/{nombre}/reindexar
// Re-indexa todo el proyecto (si body.ruta == "") o un solo archivo (si body.ruta != "").
func (s *Servidor) handlerContextoReindexar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]

        var req SolicitudReindexar
        // Body opcional; si no se pasa, se asume reindexar todo
        _ = s.parsearBody(r, &req)

        if req.Ruta != "" {
                // Reindexar un solo archivo
                if err := s.gestorCtx.ReindexarArchivo(nombre, req.Ruta); err != nil {
                        s.responderError(w, http.StatusInternalServerError, err.Error())
                        return
                }
                s.responderJSON(w, http.StatusOK, RespuestaAPI{
                        Exito:     true,
                        Mensaje:   "archivo reindexado: " + req.Ruta,
                        Timestamp: time.Now().Format(time.RFC3339),
                })
                return
        }

        // Reindexar todo el proyecto: necesitamos la ruta absoluta del proyecto
        proyectos := s.gestorCtx.ListarProyectos()
        var rutaProyecto string
        for _, p := range proyectos {
                if p.Nombre == nombre {
                        rutaProyecto = p.Ruta
                        break
                }
        }
        if rutaProyecto == "" {
                s.responderError(w, http.StatusNotFound, "proyecto no encontrado")
                return
        }

        estado, err := s.gestorCtx.IndexarProyecto(rutaProyecto)
        if err != nil {
                s.responderError(w, http.StatusInternalServerError, err.Error())
                return
        }
        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Mensaje:   "proyecto reindexado",
                Datos:     estado,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// parsearBody decodifica JSON del body en dst. Retorna error si es inválido.
// Body vacío no es error (útil para endpoints con body opcional).
func (s *Servidor) parsearBody(r *http.Request, dst interface{}) error {
        if r.Body == nil {
                return nil
        }
        defer r.Body.Close()
        if r.ContentLength == 0 {
                return nil
        }
        return json.NewDecoder(r.Body).Decode(dst)
}

// ============================================================================
// Handlers — Contexto Fase 3.5 (sistema world-class)
// ============================================================================

// handlerContextoSimbolos — GET /api/v1/contexto/proyectos/{nombre}/simbolos?ruta=X
// Retorna los símbolos AST parseados de un archivo Go.
func (s *Servidor) handlerContextoSimbolos(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        ruta := r.URL.Query().Get("ruta")
        if ruta == "" {
                s.responderError(w, http.StatusBadRequest, "parámetro 'ruta' es requerido")
                return
        }

        ast, err := s.gestorCtx.ObtenerSimbolos(nombre, ruta)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     ast,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoGrafo — GET /api/v1/contexto/proyectos/{nombre}/grafo
// Retorna el grafo de dependencias con PageRank scores.
func (s *Servidor) handlerContextoGrafo(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        g, err := s.gestorCtx.ObtenerGrafo(nombre)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        nodos := g.ObtenerTodos()
        estadisticas := g.Estadisticas()

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito: true,
                Datos: map[string]interface{}{
                        "nodos":       nodos,
                        "estadisticas": estadisticas,
                },
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoImportancia — GET /api/v1/contexto/proyectos/{nombre}/importancia
// Retorna el mapa ruta → score PageRank.
func (s *Servidor) handlerContextoImportancia(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        importancias, err := s.gestorCtx.ObtenerImportancias(nombre)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     importancias,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// SolicitudBuscarHibrido es el body para POST /buscar-hibrido.
type SolicitudBuscarHibrido struct {
        Query string `json:"query"`
        TopK  int    `json:"top_k"`
}

// handlerContextoBuscarHibrido — POST /api/v1/contexto/proyectos/{nombre}/buscar-hibrido
// Búsqueda híbrida BM25 + RRF sobre fragmentos.
func (s *Servidor) handlerContextoBuscarHibrido(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]

        var req SolicitudBuscarHibrido
        if err := s.parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "body inválido: "+err.Error())
                return
        }
        if req.Query == "" {
                s.responderError(w, http.StatusBadRequest, "campo 'query' es requerido")
                return
        }

        resultados, err := s.gestorCtx.BuscarHibrido(nombre, req.Query, req.TopK)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     resultados,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoMapaRepo — GET /api/v1/contexto/proyectos/{nombre}/mapa-repo?max_tokens=X
// Retorna el repository map compacto (Aider-style).
func (s *Servidor) handlerContextoMapaRepo(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]
        maxTokens := 2000
        if v := r.URL.Query().Get("max_tokens"); v != "" {
                if n, err := strconv.Atoi(v); err == nil && n > 0 {
                        maxTokens = n
                }
        }

        mapa, err := s.gestorCtx.ObtenerMapaRepo(nombre, maxTokens)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        // Si se pide formato texto, retornar text/plain
        if r.URL.Query().Get("formato") == "texto" {
                w.Header().Set("Content-Type", "text/plain; charset=utf-8")
                w.WriteHeader(http.StatusOK)
                w.Write([]byte(mapa.FormatoTexto()))
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     mapa,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}

// handlerContextoEmpaquetar — POST /api/v1/contexto/proyectos/{nombre}/empaquetar
// Ensambla el contexto óptimo para un LLM.
// Body: {"query": "...", "presupuesto_tokens": 8000, "archivos_recientes": [...], "profundidad_imports": 1}
func (s *Servidor) handlerContextoEmpaquetar(w http.ResponseWriter, r *http.Request) {
        if !s.requiereCoordinador(w) {
                return
        }

        nombre := mux.Vars(r)["nombre"]

        var req struct {
                Query              string   `json:"query"`
                PresupuestoTokens  int      `json:"presupuesto_tokens"`
                ArchivosRecientes  []string `json:"archivos_recientes"`
                ProfundidadImports int      `json:"profundidad_imports"`
        }
        if err := s.parsearBody(r, &req); err != nil {
                s.responderError(w, http.StatusBadRequest, "body inválido: "+err.Error())
                return
        }

        solicitud := contexto.EmpaquetarSolicitud{
                Proyecto:           nombre,
                Query:              req.Query,
                PresupuestoTokens:  req.PresupuestoTokens,
                ArchivosRecientes:  req.ArchivosRecientes,
                ProfundidadImports: req.ProfundidadImports,
        }

        resultado, err := s.gestorCtx.EmpaquetarContexto(solicitud)
        if err != nil {
                s.responderError(w, http.StatusNotFound, err.Error())
                return
        }

        s.responderJSON(w, http.StatusOK, RespuestaAPI{
                Exito:     true,
                Datos:     resultado,
                Timestamp: time.Now().Format(time.RFC3339),
        })
}
