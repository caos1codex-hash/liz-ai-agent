// Package main es el punto de entrada del binario `liz`.
//
// Inicializa y cablea todas las dependencias del núcleo:
//   - logger: logging estructurado JSON + stdout coloreado
//   - config: carga YAML + env vars + validación
//   - permisos: sistema de permisos una vez con persistencia JSON
//   - contexto: coordinador de contexto (mapa, fragmentos, índice, resúmenes)
//   - memoria: sistema de memoria conversacional (sesiones, hechos)
//   - orquestador: multi-modelo NVIDIA con fallback (Fase 4)
//   - servidor: HTTP API con todos los endpoints
package main

import (
        "flag"
        "fmt"
        "os"
        "os/signal"
        "path/filepath"
        "syscall"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/memoria"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/servidor"
)

// versión del binario. Se actualiza en cada release.
const version = "0.4.0"

// main es el punto de entrada del binario `liz`.
//
// Flujo:
//   1. Parsear flags (--version, --config)
//   2. Inicializar logger
//   3. Cargar configuración
//   4. Inicializar permisos (conceder todos al iniciar)
//   5. Inicializar coordinador de contexto
//   6. Inicializar gestor de memoria conversacional (Fase 3.5+)
//   7. Inicializar orquestador NVIDIA (Fase 4) — opcional, requiere API key
//   8. Crear servidor con todas las dependencias inyectadas
//   9. Iniciar servidor (bloquea hasta señal de terminación)
func main() {
        // --- Flags ---
        configFlag := flag.String("config", "", "ruta al archivo de configuración YAML (default: ~/.liz/config.yaml)")
        versionFlag := flag.Bool("version", false, "mostrar versión y salir")
        flag.Parse()

        if *versionFlag {
                fmt.Printf("liz version %s\n", version)
                os.Exit(0)
        }

        // --- Logger ---
        log, err := logger.Nueva("liz")
        if err != nil {
                fmt.Fprintf(os.Stderr, "ERROR: no se pudo inicializar el logger: %v\n", err)
                os.Exit(1)
        }
        defer log.Cerrar()

        log.Info("Iniciando Liz v%s", version)

        // --- Configuración ---
        rutaConfig := *configFlag
        if rutaConfig == "" {
                // Default: ~/.liz/config.yaml
                home, err := os.UserHomeDir()
                if err != nil {
                        log.Error("Error obteniendo directorio home: %v", err)
                        os.Exit(1)
                }
                rutaConfig = filepath.Join(home, ".liz", "config.yaml")
        }

        log.Info("Cargando configuración desde: %s", rutaConfig)
        gestorCfg, err := config.Inicializar(rutaConfig)
        if err != nil {
                log.Error("Error al inicializar configuración: %v", err)
                os.Exit(1)
        }

        log.Info("Configuración cargada: %s v%s (puerto %d)",
                gestorCfg.ObtenerNombre(), gestorCfg.ObtenerVersion(), gestorCfg.ObtenerPuerto())

        // --- Permisos ---
        // D-006: "Permisos Una Vez" — todos los permisos se conceden al iniciar.
        home, _ := os.UserHomeDir()
        dirPermisos := filepath.Join(home, ".liz")

        log.Info("Inicializando sistema de permisos")
        gestorPer, err := permisos.Inicializar(dirPermisos)
        if err != nil {
                log.Error("Error al inicializar permisos: %v", err)
                os.Exit(1)
        }

        // Conceder todos los permisos al iniciar
        gestorPer.ConcederTodos("usuario", "Permisos concedidos al iniciar Liz")
        log.Info("Permisos concedidos: todas las categorías")

        // --- Coordinador de contexto (Fase 3) ---
        dirContexto := filepath.Join(home, ".liz", "contexto", "proyectos")
        log.Info("Inicializando coordinador de contexto: %s", dirContexto)
        coordinador, err := contexto.NuevoCoordinador(dirContexto)
        if err != nil {
                log.Error("Error al inicializar coordinador de contexto: %v", err)
                os.Exit(1)
        }

        // Cargar proyectos existentes
        proyectosCargados := coordinador.ListarProyectos()
        if len(proyectosCargados) > 0 {
                log.Info("Proyectos cargados desde caché: %d", len(proyectosCargados))
        }

        // --- Gestor de memoria conversacional (Fase 3.5+) ---
        log.Info("Inicializando sistema de memoria conversacional: %s", dirPermisos)
        gestorMem, err := memoria.NuevoGestor(dirPermisos)
        if err != nil {
                log.Error("Error al inicializar memoria conversacional: %v", err)
                os.Exit(1)
        }
        gestorMem = gestorMem.ConLog(func(formato string, args ...interface{}) {
                log.Info("[memoria] "+formato, args...)
        })

        // --- Orquestador NVIDIA (Fase 4) ---
        // Opcional: si no hay API key configurada, el orquestador queda deshabilitado
        // y los endpoints /api/v1/orquestador/* responden 503.
        log.Info("Inicializando orquestador NVIDIA (Fase 4)")
        var orch *orquestador.Orquestador
        orch, err = orquestador.NuevoOrquestador(gestorCfg)
        if err != nil {
                log.Warn("Orquestador NVIDIA no inicializado: %v (endpoints /api/v1/orquestador/* responderán 503)", err)
                log.Info("Búsqueda híbrida no disponible (sin orquestador NVIDIA). Usando solo BM25.")
        } else {
                orch = orch.ConLog(func(formato string, args ...interface{}) {
                        log.Info("[orquestador] "+formato, args...)
                })
                log.Info("Orquestador inicializado: %d modelos disponibles", len(orch.ModelosDisponibles()))

                // Conectar búsqueda híbrida (BM25 + embeddings vectoriales)
                provider := orquestador.NuevoProviderEmbeddings(orch.Cliente(), "nvidia/nv-embed-v1")
                coordinador = coordinador.ConProviderEmbeddings(provider)
                log.Info("Búsqueda híbrida (BM25+vector) habilitada con nvidia/nv-embed-v1")
        }

        // --- Servidor ---
        log.Info("Creando servidor HTTP")
        srv := servidor.Nuevo(gestorCfg, gestorPer, log).
                ConCoordinador(coordinador).
                ConMemoria(gestorMem)
        if orch != nil {
                srv = srv.ConOrquestador(orch)
        }

        // --- Manejo de señales (SIGHUP para reload) ---
        // El servidor ya maneja SIGINT/SIGTERM/SIGHUP internamente en Iniciar(),
        // pero registramos un handler adicional para logging.
        chanSignal := make(chan os.Signal, 1)
        signal.Notify(chanSignal, syscall.SIGHUP)
        go func() {
                for sig := range chanSignal {
                        if sig == syscall.SIGHUP {
                                log.Info("SIGHUP recibido — recargando configuración")
                                if _, err := gestorCfg.Recargar(); err != nil {
                                        log.Error("Error al recargar configuración: %v", err)
                                }
                        }
                }
        }()

        // --- Iniciar (bloquea) ---
        log.Info("Liz lista para aceptar conexiones en %s:%d",
                gestorCfg.ObtenerHost(), gestorCfg.ObtenerPuerto())
        log.Info("Endpoints de contexto disponibles en /api/v1/contexto/*")
        log.Info("Endpoints de memoria disponibles en /api/v1/memoria/*")
        if orch != nil {
                log.Info("Endpoints del orquestador disponibles en /api/v1/orquestador/*")
        }

        if err := srv.Iniciar(); err != nil {
                log.Error("Error al iniciar servidor: %v", err)
                os.Exit(1)
        }
}
