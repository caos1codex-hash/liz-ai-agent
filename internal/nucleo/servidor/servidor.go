package servidor

import (
        "context"
        "encoding/json"
        "fmt"
        "net/http"
        "os"
        "os/signal"
        "syscall"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
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
        router     *mux.Router
        httpServ   *http.Server
        gestorCfg  *config.Gestor
        gestorPer  *permisos.Gestor
        gestorCtx  *contexto.Coordinador // opcional, se inyecta en Fase 3
        log        *logger.Logger
        inicio     time.Time
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

        // --- Stubs Fase 3+ (sin implementar) ---
        s.router.HandleFunc("/api/v1/tools", s.handlerStub("tools")).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/orquestador", s.handlerStub("orquestador")).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/modelos", s.handlerStub("modelos")).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/conversations", s.handlerStub("conversations")).Methods("GET", "OPTIONS")
        s.router.HandleFunc("/api/v1/chat", s.handlerStub("chat")).Methods("POST", "OPTIONS")
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