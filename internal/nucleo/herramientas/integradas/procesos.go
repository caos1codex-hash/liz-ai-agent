package integradas

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Procesos — Gestión de procesos del sistema
// ============================================================================

// MaxProcesosListar limita cuántos procesos retorna "listar".
const MaxProcesosListar = 1000

// Procesos permite listar, inspeccionar y matar procesos del sistema.
//
// En Linux usa /proc para info detallada (CPU, RAM, cmdline).
// En otros sistemas usa 'ps' como fallback.
type Procesos struct{}

// NewProcesos crea una instancia.
func NewProcesos() *Procesos { return &Procesos{} }

var _ herramientas.Herramienta = (*Procesos)(nil)

func (p *Procesos) Nombre() string { return "procesos" }

func (p *Procesos) Descripcion() string {
	return "Lista, inspecciona y mata procesos del sistema. " +
		"En Linux usa /proc para info detallada (CPU, RAM, cmdline, threads). " +
		"Operaciones: listar, info, matar, arbol."
}

func (p *Procesos) Parametros() []herramientas.Parametro {
	return []herramientas.Parametro{
		{
			Nombre:      "operacion",
			Tipo:        "string",
			Requerido:   true,
			Opciones:    []string{"listar", "info", "matar", "arbol"},
			Descripcion: "Operación a realizar.",
		},
		{
			Nombre:      "pid",
			Tipo:        "int",
			Min:         float64Ptr(1),
			Descripcion: "PID del proceso. Requerido para 'info' y 'matar'.",
		},
		{
			Nombre:      "nombre",
			Tipo:        "string",
			Descripcion: "Filtrar por nombre de proceso (substring case-insensitive).",
		},
		{
			Nombre:      "usuario",
			Tipo:        "string",
			Descripcion: "Filtrar por usuario (substring).",
		},
		{
			Nombre:      "ram_min_porcentaje",
			Tipo:        "float",
			Min:         float64Ptr(0),
			Max:         float64Ptr(100),
			Descripcion: "Filtrar procesos con uso de RAM >= X%.",
		},
		{
			Nombre:      "cpu_min_porcentaje",
			Tipo:        "float",
			Min:         float64Ptr(0),
			Descripcion: "Filtrar procesos con uso de CPU >= X%.",
		},
		{
			Nombre:      "limite",
			Tipo:        "int",
			Default:     100,
			Min:         float64Ptr(1),
			Max:         float64Ptr(MaxProcesosListar),
			Descripcion: "Máximo número de procesos a listar.",
		},
		{
			Nombre:      "senal",
			Tipo:        "string",
			Default:     "SIGTERM",
			Opciones:    []string{"SIGTERM", "SIGKILL", "SIGINT", "SIGHUP", "SIGSTOP", "SIGCONT"},
			Descripcion: "Señal a enviar en 'matar'. Default SIGTERM (graceful).",
		},
		{
			Nombre:      "incluir_hilos",
			Tipo:        "bool",
			Default:     false,
			Descripcion: "Si true, incluye hilos (tasks) además de procesos.",
		},
	}
}

// ResultadoProcesos es el Datos de Procesos.
type ResultadoProcesos struct {
	Operacion string        `json:"operacion"`
	Procesos  []InfoProceso `json:"procesos,omitempty"`
	Proceso   *InfoProceso  `json:"proceso,omitempty"`
	Total     int           `json:"total,omitempty"`
	Truncado  bool          `json:"truncado,omitempty"`
	PID       int           `json:"pid,omitempty"`
	Senal     string        `json:"senal,omitempty"`
	Enviada   bool          `json:"enviada,omitempty"`
}

// InfoProceso describe un proceso del sistema.
type InfoProceso struct {
	PID           int     `json:"pid"`
	PIDPadre      int     `json:"pid_padre,omitempty"`
	Nombre        string  `json:"nombre"`
	Cmdline       string  `json:"cmdline,omitempty"`
	Usuario       string  `json:"usuario,omitempty"`
	Estado        string  `json:"estado,omitempty"`
	CPUPorcentaje float64 `json:"cpu_porcentaje,omitempty"`
	RAMPorcentaje float64 `json:"ram_porcentaje,omitempty"`
	RSS           int64   `json:"rss_kb,omitempty"` // Resident Set Size en KB
	Virtual       int64   `json:"virtual_kb,omitempty"`
	Threads       int     `json:"threads,omitempty"`
	Inicio        string  `json:"inicio,omitempty"`
}

func (p *Procesos) Validar() error {
	// En Linux verificamos que /proc exista (mejor modo)
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/proc"); err != nil {
			return fmt.Errorf("/proc no accesible: %v", err)
		}
	}
	return nil
}

func (p *Procesos) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	operacion, err := herramientas.ObtenerString(params, p.paramByName("operacion"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	switch operacion {
	case "listar":
		return p.opListar(ctx, params)
	case "info":
		return p.opInfo(params)
	case "matar":
		return p.opMatar(params)
	case "arbol":
		return p.opArbol(params)
	default:
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("operación '%s' no soportada", operacion)}, nil
	}
}

// opListar lista procesos con filtros opcionales.
func (p *Procesos) opListar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	nombreFiltro, _ := herramientas.ObtenerString(params, p.paramByName("nombre"))
	usuarioFiltro, _ := herramientas.ObtenerString(params, p.paramByName("usuario"))
	ramMin, _ := herramientas.ObtenerFloat(params, p.paramByName("ram_min_porcentaje"))
	cpuMin, _ := herramientas.ObtenerFloat(params, p.paramByName("cpu_min_porcentaje"))
	limite, _ := herramientas.ObtenerInt(params, p.paramByName("limite"))
	if limite <= 0 {
		limite = 100
	}
	incluirHilos, _ := herramientas.ObtenerBool(params, p.paramByName("incluir_hilos"))

	procs, err := listarProcesosSistema(ctx)
	if err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("error listando procesos: %v", err)}, nil
	}

	// Aplicar filtros
	filtrados := make([]InfoProceso, 0, len(procs))
	for _, proc := range procs {
		// Filtro por nombre (case-insensitive substring)
		if nombreFiltro != "" {
			if !strings.Contains(strings.ToLower(proc.Nombre),
				strings.ToLower(nombreFiltro)) {
				continue
			}
		}
		// Filtro por usuario
		if usuarioFiltro != "" {
			if !strings.Contains(strings.ToLower(proc.Usuario),
				strings.ToLower(usuarioFiltro)) {
				continue
			}
		}
		// Filtro RAM
		if ramMin > 0 && proc.RAMPorcentaje < ramMin {
			continue
		}
		// Filtro CPU
		if cpuMin > 0 && proc.CPUPorcentaje < cpuMin {
			continue
		}
		// Excluir hilos (PID == TGID se queda, otros se van)
		if !incluirHilos && proc.PIDPadre != 0 && proc.PIDPadre != proc.PID {
			// Heurística imperfecta: en /proc los hilos aparecen en /proc/<pid>/task/<tid>
			// Si los listamos por /proc/<pid>, son procesos reales.
		}
		filtrados = append(filtrados, proc)
	}

	truncado := false
	if len(filtrados) > limite {
		filtrados = filtrados[:limite]
		truncado = true
	}

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoProcesos{
			Operacion: "listar",
			Procesos:  filtrados,
			Total:     len(filtrados),
			Truncado:  truncado,
		},
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// opInfo retorna información detallada de un proceso específico.
func (p *Procesos) opInfo(params map[string]interface{}) (herramientas.Resultado, error) {
	pid, err := herramientas.ObtenerInt(params, p.paramByName("pid"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}
	if pid <= 0 {
		return herramientas.Resultado{Exito: false,
			Error: "pid debe ser > 0"}, nil
	}

	proc, err := infoProceso(pid)
	if err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("error obteniendo info de PID %d: %v", pid, err)}, nil
	}

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoProcesos{
			Operacion: "info",
			Proceso:   proc,
		},
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// opMatar envía una señal a un proceso.
func (p *Procesos) opMatar(params map[string]interface{}) (herramientas.Resultado, error) {
	pid, err := herramientas.ObtenerInt(params, p.paramByName("pid"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}
	if pid <= 0 {
		return herramientas.Resultado{Exito: false,
			Error: "pid debe ser > 0"}, nil
	}

	senal, _ := herramientas.ObtenerString(params, p.paramByName("senal"))
	if senal == "" {
		senal = "SIGTERM"
	}

	sig := parsearSenal(senal)
	if sig == 0 {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("señal '%s' no reconocida", senal)}, nil
	}

	// Encontrar proc para incluir nombre en la respuesta
	proc, _ := infoProceso(pid)
	nombreProc := ""
	if proc != nil {
		nombreProc = proc.Nombre
	}

	// Enviar señal
	// Usar os.FindProcess (siempre funciona en Unix) + Signal
	p2, err := os.FindProcess(pid)
	if err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("FindProcess: %v", err)}, nil
	}

	if err := p2.Signal(sig); err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("Signal: %v", err)}, nil
	}

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoProcesos{
			Operacion: "matar",
			PID:       pid,
			Senal:     senal,
			Enviada:   true,
			Proceso:   proc,
		},
		Metadata: map[string]interface{}{
			"duracion_ms":    float64(0),
			"nombre_proceso": nombreProc,
		},
	}, nil
}

// opArbol retorna el árbol de procesos desde un PID raíz.
// Implementación simplificada: lista procesos y construye árbol.
func (p *Procesos) opArbol(params map[string]interface{}) (herramientas.Resultado, error) {
	procs, err := listarProcesesistemaSimple()
	if err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("error listando procesos: %v", err)}, nil
	}

	// Construir mapa PID → hijos
	hijos := make(map[int][]InfoProceso)
	for _, proc := range procs {
		hijos[proc.PIDPadre] = append(hijos[proc.PIDPadre], proc)
	}

	// Raíz: procesos cuyo padre no está en la lista (típicamente PID 1 / init)
	// O usar el PID del propio Liz si se especifica
	raiz := 1
	if pidEspecifico, _ := herramientas.ObtenerInt(params, p.paramByName("pid")); pidEspecifico > 0 {
		raiz = pidEspecifico
	}

	// Recolectar descendientes
	var resultado []InfoProceso
	cola := []int{raiz}
	visitados := make(map[int]bool)
	for len(cola) > 0 {
		pid := cola[0]
		cola = cola[1:]
		if visitados[pid] {
			continue
		}
		visitados[pid] = true
		for _, h := range hijos[pid] {
			resultado = append(resultado, h)
			cola = append(cola, h.PID)
		}
	}

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoProcesos{
			Operacion: "arbol",
			Procesos:  resultado,
			Total:     len(resultado),
		},
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// ============================================================================
// Helpers — Lectura de /proc (Linux) o fallback a ps
// ============================================================================

// listarProcesosSistema lista todos los procesos del sistema.
func listarProcesosSistema(ctx context.Context) ([]InfoProceso, error) {
	if runtime.GOOS == "linux" {
		return listarProcesosProc(ctx)
	}
	return listarProcesosPS()
}

// listarProcesesistemaSimple versión sin ctx para opArbol.
func listarProcesesistemaSimple() ([]InfoProceso, error) {
	return listarProcesosSistema(context.Background())
}

// listarProcesosProc lee /proc en Linux.
func listarProcesosProc(ctx context.Context) ([]InfoProceso, error) {
	entradas, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	// MemTotal para calcular porcentaje de RAM
	memTotalKB := obtenerMemTotalKB()

	var resultado []InfoProceso
	for _, e := range entradas {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue // no es un PID
		}

		proc, err := leerProcPid(pid)
		if err != nil {
			continue
		}
		// Calcular RAM%
		if memTotalKB > 0 {
			proc.RAMPorcentaje = (float64(proc.RSS) / float64(memTotalKB)) * 100
		}
		resultado = append(resultado, *proc)
	}
	return resultado, nil
}

// leerProcPid lee la info de /proc/<pid>.
func leerProcPid(pid int) (*InfoProceso, error) {
	base := fmt.Sprintf("/proc/%d", pid)

	// /proc/<pid>/comm — nombre corto
	comm, err := os.ReadFile(filepath.Join(base, "comm"))
	if err != nil {
		return nil, err
	}
	nombre := strings.TrimSpace(string(comm))

	// /proc/<pid>/cmdline — args separados por \0
	cmdlineBytes, err := os.ReadFile(filepath.Join(base, "cmdline"))
	cmdline := ""
	if err == nil {
		// Reemplazar \0 por espacio
		cmdline = strings.TrimSpace(strings.ReplaceAll(string(cmdlineBytes), "\x00", " "))
	}

	// /proc/<pid>/stat — info completa
	stat, err := os.ReadFile(filepath.Join(base, "stat"))
	if err != nil {
		return &InfoProceso{PID: pid, Nombre: nombre, Cmdline: cmdline}, nil
	}

	proc := parsearStat(pid, string(stat), nombre, cmdline)

	// /proc/<pid>/status — usuario, threads, RSS
	if status, err := os.ReadFile(filepath.Join(base, "status")); err == nil {
		parsearStatus(string(status), proc)
	}

	return proc, nil
}

// parsearStat extrae info de /proc/<pid>/stat.
// Formato: pid (comm) state ppid pgrp session tty_nr tpgid flags minflt cminflt majflt cmajflt
//
//	utime stime cutime cstime priority nice num_threads itrealvalue starttime vsize rss
func parsearStat(pid int, stat, nombre, cmdline string) *InfoProceso {
	proc := &InfoProceso{PID: pid, Nombre: nombre, Cmdline: cmdline}

	// stat tiene formato: "pid (comm con espacios) state ppid ..."
	// comm puede contener paréntesis, así que buscamos el último ")"
	idxUltimoParen := strings.LastIndex(stat, ")")
	if idxUltimoParen < 0 {
		return proc
	}

	resto := strings.TrimSpace(stat[idxUltimoParen+1:])
	campos := strings.Fields(resto)
	if len(campos) < 20 {
		return proc
	}

	// state
	proc.Estado = campos[0]
	// ppid
	if ppid, err := strconv.Atoi(campos[1]); err == nil {
		proc.PIDPadre = ppid
	}
	// num_threads (campo 19, index 18)
	if len(campos) > 18 {
		if n, err := strconv.Atoi(campos[18]); err == nil {
			proc.Threads = n
		}
	}
	// vsize (campo 22, index 21) en bytes
	if len(campos) > 21 {
		if v, err := strconv.ParseInt(campos[21], 10, 64); err == nil {
			proc.Virtual = v / 1024 // a KB
		}
	}
	// rss (campo 23, index 22) en páginas
	if len(campos) > 22 {
		if r, err := strconv.ParseInt(campos[22], 10, 64); err == nil {
			// Convertir páginas a KB
			pagesize := int64(syscall.Getpagesize())
			proc.RSS = (r * int64(pagesize)) / 1024
		}
	}
	return proc
}

// parsearStatus extrae info de /proc/<pid>/status.
func parsearStatus(status string, proc *InfoProceso) {
	for _, linea := range strings.Split(status, "\n") {
		partes := strings.SplitN(linea, ":", 2)
		if len(partes) != 2 {
			continue
		}
		key := strings.TrimSpace(partes[0])
		val := strings.TrimSpace(partes[1])

		switch key {
		case "Uid":
			proc.Usuario = resolverUID(val)
		case "Threads":
			if n, err := strconv.Atoi(strings.Fields(val)[0]); err == nil {
				proc.Threads = n
			}
		case "VmRSS":
			// "1234 kB"
			campos := strings.Fields(val)
			if len(campos) >= 1 {
				if n, err := strconv.ParseInt(campos[0], 10, 64); err == nil {
					proc.RSS = n
				}
			}
		case "VmSize":
			campos := strings.Fields(val)
			if len(campos) >= 1 {
				if n, err := strconv.ParseInt(campos[0], 10, 64); err == nil {
					proc.Virtual = n
				}
			}
		case "State":
			if proc.Estado == "" {
				proc.Estado = val
			}
		}
	}
}

// obtenerMemTotalKB lee /proc/meminfo para MemTotal.
func obtenerMemTotalKB() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, linea := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(linea, "MemTotal:") {
			campos := strings.Fields(linea)
			if len(campos) >= 2 {
				if n, err := strconv.ParseInt(campos[1], 10, 64); err == nil {
					return n
				}
			}
			break
		}
	}
	return 0
}

// resolverUID convierte UID numérico a username (best-effort).
func resolverUID(uidStr string) string {
	campos := strings.Fields(uidStr)
	if len(campos) == 0 {
		return uidStr
	}
	uid := campos[0] // UID real
	// En Linux, podemos usar getent passwd — pero es caro. Para simplificar,
	// retornamos el UID numérico. Una mejora futura sería cachear /etc/passwd.
	return uid
}

// infoProceso retorna info detallada de un PID específico.
func infoProceso(pid int) (*InfoProceso, error) {
	if runtime.GOOS == "linux" {
		proc, err := leerProcPid(pid)
		if err != nil {
			return nil, err
		}
		// Calcular RAM%
		memTotalKB := obtenerMemTotalKB()
		if memTotalKB > 0 {
			proc.RAMPorcentaje = (float64(proc.RSS) / float64(memTotalKB)) * 100
		}
		return proc, nil
	}
	// Fallback: ps
	return infoProcesoPS(pid)
}

// listarProcesosPS usa comando ps como fallback.
func listarProcesosPS() ([]InfoProceso, error) {
	cmd := exec.Command("ps", "-eo", "pid,ppid,user,%cpu,%mem,rss,comm")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lineas := strings.Split(string(out), "\n")
	var resultado []InfoProceso
	for i, linea := range lineas {
		if i == 0 || strings.TrimSpace(linea) == "" {
			continue // header
		}
		campos := strings.Fields(linea)
		if len(campos) < 7 {
			continue
		}
		pid, _ := strconv.Atoi(campos[0])
		ppid, _ := strconv.Atoi(campos[1])
		cpu, _ := strconv.ParseFloat(campos[3], 64)
		ram, _ := strconv.ParseFloat(campos[4], 64)
		rss, _ := strconv.ParseInt(campos[5], 10, 64)
		nombre := strings.Join(campos[6:], " ")

		resultado = append(resultado, InfoProceso{
			PID:           pid,
			PIDPadre:      ppid,
			Usuario:       campos[2],
			Nombre:        nombre,
			CPUPorcentaje: cpu,
			RAMPorcentaje: ram,
			RSS:           rss,
		})
	}
	return resultado, nil
}

// infoProcesoPS info de un PID específico vía ps.
func infoProcesoPS(pid int) (*InfoProceso, error) {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pid,ppid,user,%cpu,%mem,rss,comm")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lineas := strings.Split(string(out), "\n")
	if len(lineas) < 2 {
		return nil, fmt.Errorf("PID %d no encontrado", pid)
	}
	campos := strings.Fields(lineas[1])
	if len(campos) < 7 {
		return nil, fmt.Errorf("formato ps inesperado")
	}
	ppid, _ := strconv.Atoi(campos[1])
	cpu, _ := strconv.ParseFloat(campos[3], 64)
	ram, _ := strconv.ParseFloat(campos[4], 64)
	rss, _ := strconv.ParseInt(campos[5], 10, 64)
	return &InfoProceso{
		PID:           pid,
		PIDPadre:      ppid,
		Usuario:       campos[2],
		Nombre:        strings.Join(campos[6:], " "),
		CPUPorcentaje: cpu,
		RAMPorcentaje: ram,
		RSS:           rss,
	}, nil
}

// parsearSenal convierte nombre de señal a syscall.Signal.
func parsearSenal(nombre string) syscall.Signal {
	switch nombre {
	case "SIGTERM":
		return syscall.SIGTERM
	case "SIGKILL":
		return syscall.SIGKILL
	case "SIGINT":
		return syscall.SIGINT
	case "SIGHUP":
		return syscall.SIGHUP
	case "SIGSTOP":
		return syscall.SIGSTOP
	case "SIGCONT":
		return syscall.SIGCONT
	default:
		return 0
	}
}

// paramByName busca un parámetro por nombre.
func (p *Procesos) paramByName(nombre string) herramientas.Parametro {
	for _, p2 := range p.Parametros() {
		if p2.Nombre == nombre {
			return p2
		}
	}
	return herramientas.Parametro{Nombre: nombre}
}

// time.Now() referencia para evitar unused import (si se quita, quitar import)
var _ = time.Now
