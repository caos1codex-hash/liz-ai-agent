// Package main es el punto de entrada del binario `liz`.
//
// Inicializa y cablea todas las dependencias del núcleo:
//   - logger: logging estructurado JSON + stdout coloreado
//   - config: carga YAML + env vars + validación
//   - permisos: sistema de permisos una vez con persistencia JSON
//   - contexto: coordinador de contexto (mapa, fragmentos, índice, resúmenes)
//   - memoria: sistema de memoria conversacional (sesiones, hechos)
//   - orquestador: multi-modelo NVIDIA con fallback (Fase 4)
//   - herramientas: catálogo + 7 integradas (Fase 5)
//   - servidor: HTTP API con todos los endpoints
//   - desktop: GUI nativa Fyne (Fase 8) — solo cuando NO se compila con -tags headless
//
// Build tags:
//   - default (sin tag): binario completo con GUI Fyne (requiere CGO + OpenGL dev headers)
//   - -tags headless:    binario servidor puro, sin dependencias Fyne/OpenGL,
//     cross-compilable a cualquier GOOS/GOARCH (Fase 10 — release v0.1.0)
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/auto_creacion"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/integradas"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/registro"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/memoria"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/servidor"
	"github.com/caos1codex-hash/liz-ai-agent/internal/pipeline"
)

// versión del binario. Se actualiza en cada release.
// Fase 10 (release v0.1.0): versionado semántico.
const version = "0.1.0"

// main es el punto de entrada del binario `liz`.
//
// Flujo:
//  1. Parsear flags (--version, --config, --headless)
//  2. Inicializar logger
//  3. Cargar configuración
//  4. Inicializar permisos (conceder todos al iniciar)
//  5. Inicializar coordinador de contexto
//  6. Inicializar gestor de memoria conversacional (Fase 3.5+)
//  7. Inicializar orquestador NVIDIA (Fase 4) — opcional, requiere API key
//  8. Crear servidor con todas las dependencias inyectadas
//  9. Iniciar servidor en goroutine (no bloqueante)
//  10. Si NO es --headless: arrancar GUI nativa (Fase 8 desktop)
//     Si --headless: bloquear hasta señal de terminación
func main() {
	// --- Flags ---
	configFlag := flag.String("config", "", "ruta al archivo de configuración YAML (default: ~/.liz/config.yaml)")
	versionFlag := flag.Bool("version", false, "mostrar versión y salir")
	headlessFlag := flag.Bool("headless", false, "modo servidor sin interfaz gráfica (sin GUI Fyne)")
	healthFlag := flag.Bool("health", false, "hacer petición HTTP al servidor y verificar estado (para Docker healthcheck)")
	healthURL := flag.String("health-url", "", "URL del endpoint de health (default: http://127.0.0.1:PUERTO_CONFIGURADO/api/v1/health)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("liz version %s\n", version)
		os.Exit(0)
	}

	// Modo healthcheck: hacer petición HTTP al servidor y salir.
	// Pensado para Docker HEALTHCHECK (issue #26). El binario distroless no
	// tiene curl/wget, pero puede hacer peticiones HTTP via Go stdlib.
	if *healthFlag {
		// Cargar config para saber qué puerto/host usar (solo si no se pasó --health-url).
		url := *healthURL
		if url == "" {
			rutaCfg := *configFlag
			if rutaCfg == "" {
				home, _ := os.UserHomeDir()
				rutaCfg = filepath.Join(home, ".liz", "config.yaml")
			}
			cfg, _ := config.Inicializar(rutaCfg)
			if cfg != nil {
				url = fmt.Sprintf("http://%s:%d/api/v1/health", "127.0.0.1", cfg.ObtenerPuerto())
			} else {
				url = "http://127.0.0.1:8080/api/v1/health"
			}
		}
		os.Exit(runHealthCheck(url))
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

	// --- Catálogo de Herramientas (Fase 5) ---
	// 7 herramientas integradas: terminal, navegador_archivos, buscador,
	// editor, procesos, monitor, instalador.
	log.Info("Inicializando catálogo de herramientas (Fase 5)")
	catalogo := registro.NuevoCatalogo().ConLog(func(formato string, args ...interface{}) {
		log.Info("[herramientas] "+formato, args...)
	})

	// Registrar las 7 herramientas integradas (cada una implementa herramientas.Herramienta).
	herramientasARegistrar := []struct {
		nombre string
		crea   func() herramientas.Herramienta
	}{
		{"terminal", func() herramientas.Herramienta { return integradas.NewTerminal() }},
		{"navegador_archivos", func() herramientas.Herramienta { return integradas.NewNavegadorArchivos() }},
		{"buscador", func() herramientas.Herramienta { return integradas.NewBuscador() }},
		{"editor", func() herramientas.Herramienta { return integradas.NewEditor() }},
		{"procesos", func() herramientas.Herramienta { return integradas.NewProcesos() }},
		{"monitor", func() herramientas.Herramienta { return integradas.NewMonitor() }},
		{"instalador", func() herramientas.Herramienta { return integradas.NewInstalador() }},
	}
	for _, h := range herramientasARegistrar {
		if err := catalogo.Registrar(h.crea()); err != nil {
			log.Error("Error al registrar herramienta %s: %v", h.nombre, err)
			continue
		}
		// No loguear éxito aquí: el propio catálogo ya lo hace via ConLog
		// (issue #24). Antes esto duplicaba cada línea en el log.
	}
	log.Info("Catálogo de herramientas: %d registradas", catalogo.Tamaño())

	// --- Gestor de Auto-Creación (Fase 6) ---
	// Permite a Liz programarse a sí misma herramientas que no tiene.
	// Las herramientas auto-creadas se almacenan en ~/.liz/herramientas/auto_creadas/
	// y se cargan al iniciar para que queden disponibles inmediatamente.
	dirAutoCreadas := filepath.Join(home, ".liz", "herramientas", "auto_creadas")
	log.Info("Inicializando gestor de auto-creación (Fase 6): %s", dirAutoCreadas)

	// El gestor usa el orquestador como LLM si está disponible
	var clienteLLM auto_creacion.ClienteLLM
	if orch != nil {
		clienteLLM = orch
		log.Info("Auto-creación: LLM disponible (orquestador NVIDIA)")
	} else {
		log.Warn("Auto-creación: LLM no disponible (orquestador no inicializado) — solo se permite crear vía 'forzar_spec'/'forzar_nombre'")
	}

	autoGestor, err := auto_creacion.NuevoGestor(clienteLLM, dirAutoCreadas, catalogo)
	if err != nil {
		log.Error("Error al inicializar gestor de auto-creación: %v", err)
		os.Exit(1)
	}
	autoGestor = autoGestor.ConLog(func(formato string, args ...interface{}) {
		log.Info("[auto-creación] "+formato, args...)
	})

	// Cargar herramientas auto-creadas existentes
	cargadas, errsCarga := autoGestor.CargarTodas()
	if cargadas > 0 {
		log.Info("Herramientas auto-creadas cargadas: %d", cargadas)
	}
	for _, e := range errsCarga {
		log.Warn("Carga de herramienta auto-creada: %v", e)
	}
	log.Info("Catálogo total (integradas + auto-creadas): %d herramientas", catalogo.Tamaño())

	// --- Pipeline de Chat (Fase 7) ---
	// End-to-end: mensaje → clasificación → planificación → ejecución → respuesta
	log.Info("Inicializando pipeline de chat (Fase 7)")
	pipelineMgr := pipeline.Nuevo(pipeline.NuevasOpciones{
		Orquestador:   crearAdaptadorOrquestador(orch),
		Catalogo:      crearAdaptadorCatalogo(catalogo),
		Memoria:       crearAdaptadorMemoria(gestorMem),
		AutoGestor:    crearAdaptadorAutoCreacion(autoGestor),
		ContextoCoord: crearAdaptadorContexto(coordinador),
	})
	log.Info("Pipeline de chat inicializado: receptor + clasificador + planificador + ejecutor + respondedor")

	// --- Servidor ---
	log.Info("Creando servidor HTTP")
	srv := servidor.Nuevo(gestorCfg, gestorPer, log).
		ConCoordinador(coordinador).
		ConMemoria(gestorMem).
		ConCatalogo(catalogo).
		ConAutoGestor(autoGestor).
		ConPipeline(pipelineMgr)
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
	log.Info("Endpoints de herramientas disponibles en /api/v1/herramientas/*")
	log.Info("Endpoints de auto-creación disponibles en /api/v1/herramientas/auto-creadas/* y /api/v1/herramientas/auto-crear")
	log.Info("Endpoints del pipeline de chat disponibles en /api/v1/chat/*")
	if orch != nil {
		log.Info("Endpoints del orquestador disponibles en /api/v1/orquestador/*")
	}

	// --- Modo de operación ---
	//   --headless: solo servidor HTTP, sin GUI (uso en servidores/Docker)
	//   default:    servidor HTTP en goroutine + GUI nativa Fyne (Fase 8)
	if *headlessFlag {
		log.Info("Modo --headless: servidor HTTP sin GUI")
		if err := srv.Iniciar(); err != nil {
			log.Error("Error al iniciar servidor: %v", err)
			os.Exit(1)
		}
		return
	}

	// Lanzar GUI nativa (o bloquear en servidor puro si se compiló con -tags headless).
	// La implementación está en desktop_desktop.go o desktop_headless.go según build tag.
	lanzarModoVisual(srv, gestorCfg, log)
}

// ============================================================================
// Adaptadores: conectan implementaciones reales con interfaces del pipeline
// ============================================================================

// crearAdaptadorOrquestador adapta el Orquestador real a la interfaz del pipeline.
func crearAdaptadorOrquestador(orch *orquestador.Orquestador) pipeline.OrquestadorCliente {
	if orch == nil {
		return nil
	}
	return &pipelineOrquestadorAdapter{orch: orch}
}

type pipelineOrquestadorAdapter struct {
	orch *orquestador.Orquestador
}

func (a *pipelineOrquestadorAdapter) Completar(ctx context.Context, prompt string, tipoTarea string) (string, error) {
	req := orquestador.SolicitudChat{
		Tarea: orquestador.TipoTarea(tipoTarea),
		Mensajes: []orquestador.MensajeChat{{
			Rol:       "system",
			Contenido: prompt,
		}},
	}
	resp, err := a.orch.Completar(req)
	if err != nil {
		return "", err
	}
	return resp.Contenido, nil
}

func (a *pipelineOrquestadorAdapter) CompletarStream(ctx context.Context, prompt string, tipoTarea string) (<-chan pipeline.ChunkOrquestador, error) {
	req := orquestador.SolicitudChat{
		Tarea:  orquestador.TipoTarea(tipoTarea),
		Stream: true,
		Mensajes: []orquestador.MensajeChat{{
			Rol:       "system",
			Contenido: prompt,
		}},
	}
	ch, err := a.orch.CompletarStream(ctx, req)
	if err != nil {
		return nil, err
	}
	// Adaptar el canal del orquestador real al del pipeline
	out := make(chan pipeline.ChunkOrquestador, 10)
	go func() {
		defer close(out)
		for chunk := range ch {
			out <- pipeline.ChunkOrquestador{
				Delta:  chunk.Contenido,
				Modelo: chunk.ModeloUsado,
				Error:  chunk.Error,
				Done:   chunk.Acabado,
			}
		}
	}()
	return out, nil
}

func (a *pipelineOrquestadorAdapter) ModeloActual() string {
	return "nvidia"
}

// crearAdaptadorCatalogo adapta el Catálogo real a la interfaz del pipeline.
func crearAdaptadorCatalogo(cat *registro.Catalogo) pipeline.CatalogoCliente {
	if cat == nil {
		return nil
	}
	return &pipelineCatalogoAdapter{cat: cat}
}

type pipelineCatalogoAdapter struct {
	cat *registro.Catalogo
}

func (a *pipelineCatalogoAdapter) Existe(nombre string) bool {
	return a.cat.Existe(nombre)
}

func (a *pipelineCatalogoAdapter) Ejecutar(ctx context.Context, nombre string, params map[string]interface{}) (*pipeline.ResultadoHerramienta, error) {
	res, err := a.cat.Ejecutar(ctx, nombre, params)
	if err != nil {
		return nil, err
	}
	return &pipeline.ResultadoHerramienta{
		Exito:    res.Exito,
		Datos:    res.Datos,
		Error:    res.Error,
		Metadata: res.Metadata,
	}, nil
}

func (a *pipelineCatalogoAdapter) Snapshot() []pipeline.InfoHerramientaSnapshot {
	snap := a.cat.Snapshot()
	result := make([]pipeline.InfoHerramientaSnapshot, len(snap))
	for i, s := range snap {
		result[i] = pipeline.InfoHerramientaSnapshot{
			Nombre:      s.Nombre,
			Descripcion: s.Descripcion,
			Parametros:  s.Parametros,
		}
	}
	return result
}

// crearAdaptadorMemoria adapta el Gestor de memoria real.
func crearAdaptadorMemoria(gm *memoria.Gestor) pipeline.MemoriaGestor {
	if gm == nil {
		return nil
	}
	return &pipelineMemoriaAdapter{gm: gm}
}

type pipelineMemoriaAdapter struct {
	gm *memoria.Gestor
}

func (a *pipelineMemoriaAdapter) ObtenerSesion(ctx context.Context, sesionID, usuarioID string) (*pipeline.InfoSesion, error) {
	s, err := a.gm.Sesiones().ObtenerSesion(sesionID)
	if err != nil {
		return nil, err
	}
	return &pipeline.InfoSesion{ID: s.ID, UsuarioID: s.UsuarioID, Proyecto: s.Proyecto, Titulo: s.Titulo}, nil
}

func (a *pipelineMemoriaAdapter) CrearSesion(ctx context.Context, usuarioID, proyecto string) (*pipeline.InfoSesion, error) {
	s, err := a.gm.NuevaSesion(usuarioID, proyecto)
	if err != nil {
		return nil, err
	}
	return &pipeline.InfoSesion{ID: s.ID, UsuarioID: s.UsuarioID, Proyecto: s.Proyecto}, nil
}

func (a *pipelineMemoriaAdapter) AgregarMensaje(ctx context.Context, sesionID, usuarioID, contenido string) error {
	_, err := a.gm.AgregarMensaje(usuarioID, memoria.RolUsuario, contenido)
	return err
}

func (a *pipelineMemoriaAdapter) ObtenerMensajesRecientes(sesionID string, limite int) []pipeline.InfoMensaje {
	// Buscar sesión por ID para obtener usuarioID
	sesiones := a.gm.Sesiones()
	// Obtenemos los mensajes más recientes usando el gestor de sesiones
	sesion, err := sesiones.ObtenerSesion(sesionID)
	if err != nil {
		return nil
	}
	msgs := sesiones.UltimosMensajes(sesion.UsuarioID, limite)
	result := make([]pipeline.InfoMensaje, len(msgs))
	for i, m := range msgs {
		result[i] = pipeline.InfoMensaje{Rol: string(m.Rol), Contenido: m.Contenido}
	}
	return result
}

func (a *pipelineMemoriaAdapter) ObtenerHechos(usuarioID string, limite int) string {
	ctx, err := a.gm.Hechos().FormatoContexto(usuarioID, limite)
	if err != nil {
		return ""
	}
	return ctx
}

func (a *pipelineMemoriaAdapter) ContextoParaLLM(usuarioID string, ultimosNMensajes int, limiteHechos int) string {
	ctx, err := a.gm.ContextoParaLLM(usuarioID, ultimosNMensajes, limiteHechos)
	if err != nil {
		return ""
	}
	return ctx
}

// crearAdaptadorAutoCreacion adapta el gestor de auto-creación.
func crearAdaptadorAutoCreacion(ag *auto_creacion.Gestor) pipeline.AutoCreacionGestor {
	if ag == nil {
		return nil
	}
	return &pipelineAutoCreacionAdapter{ag: ag}
}

type pipelineAutoCreacionAdapter struct {
	ag *auto_creacion.Gestor
}

func (a *pipelineAutoCreacionAdapter) Crear(ctx context.Context, descripcion string) (*pipeline.ResultadoAutoCreacion, error) {
	sol := auto_creacion.SolicitudCreacion{Descripcion: descripcion}
	_, err := a.ag.Crear(ctx, sol)
	if err != nil {
		return &pipeline.ResultadoAutoCreacion{Exito: false, Error: err.Error()}, nil
	}
	return &pipeline.ResultadoAutoCreacion{Exito: true, Datos: "herramienta creada"}, nil
}

// crearAdaptadorContexto adapta el coordinador de contexto.
func crearAdaptadorContexto(coord *contexto.Coordinador) pipeline.ContextoCoordinador {
	if coord == nil {
		return nil
	}
	return &pipelineContextoAdapter{coord: coord}
}

type pipelineContextoAdapter struct {
	coord *contexto.Coordinador
}

func (a *pipelineContextoAdapter) EmpaquetarContexto(ctx context.Context, proyecto, query string, maxTokens int) (string, error) {
	req := contexto.EmpaquetarSolicitud{
		Proyecto:          proyecto,
		Query:             query,
		PresupuestoTokens: maxTokens,
	}
	result, err := a.coord.EmpaquetarContexto(req)
	if err != nil {
		return "", err
	}
	return result.Contenido, nil
}

// runHealthCheck hace una petición HTTP GET al endpoint /api/v1/health
// del servidor Liz y retorna 0 si responde 200, 1 en caso contrario.
// Pensado para Docker HEALTHCHECK (issue #26).
//
// Uso: liz-server --health [--health-url http://host:puerto/api/v1/health]
func runHealthCheck(url string) int {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: error conectando a %s: %v\n", url, err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: %s retornó HTTP %d\n", url, resp.StatusCode)
		return 1
	}
	fmt.Printf("healthcheck: OK (%s)\n", url)
	return 0
}
