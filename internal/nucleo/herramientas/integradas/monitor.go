package integradas

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Monitor — Métricas de Sistema
// ============================================================================

// Monitor retorna métricas del sistema: CPU, RAM, disco, red.
// En Linux usa /proc y /sys; en otros sistemas usa comandos del sistema.
type Monitor struct{}

// NewMonitor crea una instancia.
func NewMonitor() *Monitor { return &Monitor{} }

var _ herramientas.Herramienta = (*Monitor)(nil)

func (m *Monitor) Nombre() string { return "monitor" }

func (m *Monitor) Descripcion() string {
	return "Retorna métricas del sistema en tiempo real: CPU (load avg, " +
		"uso por core), RAM (total/used/free), disco (espacio, inodos), " +
		"red (interfaces, bytes/errores), uptime y procesos activos."
}

func (m *Monitor) Parametros() []herramientas.Parametro {
	return []herramientas.Parametro{
		{
			Nombre:      "operacion",
			Tipo:        "string",
			Requerido:   true,
			Opciones:    []string{"completo", "cpu", "memoria", "disco", "red", "uptime"},
			Descripcion: "Tipo de métricas a retornar.",
			Default:     "completo",
		},
		{
			Nombre:      "ruta_disco",
			Tipo:        "string",
			Default:     "/",
			Descripcion: "Ruta del punto de montaje para métricas de disco (default '/').",
		},
		{
			Nombre:      "interfaz_red",
			Tipo:        "string",
			Descripcion: "Nombre de interfaz específica (default: todas).",
		},
	}
}

// ResultadoMonitor es el Datos de Monitor.
type ResultadoMonitor struct {
	Operacion string          `json:"operacion"`
	CPU       *InfoCPU        `json:"cpu,omitempty"`
	Memoria   *InfoMemoria    `json:"memoria,omitempty"`
	Disco     *InfoDisco      `json:"disco,omitempty"`
	Red       []InfoInterfaz  `json:"red,omitempty"`
	Uptime    *InfoUptime     `json:"uptime,omitempty"`
	Timestamp string          `json:"timestamp"`
}

// InfoCPU contiene métricas de CPU.
type InfoCPU struct {
	LoadAvg1      float64   `json:"load_avg_1"`
	LoadAvg5      float64   `json:"load_avg_5"`
	LoadAvg15     float64   `json:"load_avg_15"`
	NumCores      int       `json:"num_cores"`
	UsoPorcentaje float64   `json:"uso_porcentaje"`
	Procesos      int       `json:"procesos,omitempty"`
	Cores         []InfoCPUCore `json:"cores,omitempty"`
}

// InfoCPUCore info de un core individual.
type InfoCPUCore struct {
	ID       int     `json:"id"`
	Frecuencia int64  `json:"frecuencia_khz,omitempty"`
	Online   bool    `json:"online"`
}

// InfoMemoria contiene métricas de RAM.
type InfoMemoria struct {
	TotalKB       int64   `json:"total_kb"`
	LibreKB       int64   `json:"libre_kb"`
	DisponibleKB  int64   `json:"disponible_kb"`
	BuffersKB     int64   `json:"buffers_kb"`
	CachedKB      int64   `json:"cached_kb"`
	UsadaKB       int64   `json:"usada_kb"`
	UsadaPorcentaje float64 `json:"usada_porcentaje"`
	SwapTotalKB   int64   `json:"swap_total_kb"`
	SwapUsadaKB   int64   `json:"swap_usada_kb"`
}

// InfoDisco contiene métricas de un punto de montaje.
type InfoDisco struct {
	Ruta          string  `json:"ruta"`
	TotalBytes    int64   `json:"total_bytes"`
	LibreBytes    int64   `json:"libre_bytes"`
	UsadoBytes    int64   `json:"usado_bytes"`
	UsadoPorcentaje float64 `json:"usado_porcentaje"`
	InodosTotal   int64   `json:"inodos_total"`
	InodosLibres  int64   `json:"inodos_libres"`
}

// InfoInterfaz contiene métricas de una interfaz de red.
type InfoInterfaz struct {
	Nombre       string `json:"nombre"`
	BytesRX      int64  `json:"bytes_rx"`
	BytesTX      int64  `json:"bytes_tx"`
	PaquetesRX   int64  `json:"paquetes_rx"`
	PaquetesTX   int64  `json:"paquetes_tx"`
	ErroresRX    int64  `json:"errores_rx"`
	ErroresTX    int64  `json:"errores_tx"`
	Up           bool   `json:"up"`
	DireccionMAC string `json:"mac,omitempty"`
	MTU          int    `json:"mtu,omitempty"`
}

// InfoUptime contiene info de uptime del sistema.
type InfoUptime struct {
	Segundos    int64  `json:"segundos"`
	Humano      string `json:"humano"`
	InicioBoot  string `json:"inicio_boot"`
}

func (m *Monitor) Validar() error { return nil }

func (m *Monitor) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	operacion, err := herramientas.ObtenerString(params, m.paramByName("operacion"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	if operacion == "" {
		operacion = "completo"
	}

	resultado := ResultadoMonitor{
		Operacion: operacion,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	switch operacion {
	case "completo":
		resultado.CPU = obtenerCPU()
		resultado.Memoria = obtenerMemoria()
		resultado.Disco, _ = obtenerDisco(params, m)
		resultado.Red = obtenerRed(params, m)
		resultado.Uptime = obtenerUptime()

	case "cpu":
		resultado.CPU = obtenerCPU()

	case "memoria":
		resultado.Memoria = obtenerMemoria()

	case "disco":
		resultado.Disco, err = obtenerDisco(params, m)
		if err != nil {
			return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
		}

	case "red":
		resultado.Red = obtenerRed(params, m)

	case "uptime":
		resultado.Uptime = obtenerUptime()

	default:
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("operación '%s' no soportada", operacion)}, nil
	}

	return herramientas.Resultado{
		Exito:    true,
		Datos:    resultado,
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// obtenerCPU lee métricas de CPU.
func obtenerCPU() *InfoCPU {
	info := &InfoCPU{NumCores: runtime.NumCPU()}

	if runtime.GOOS == "linux" {
		// /proc/loadavg
		if data, err := os.ReadFile("/proc/loadavg"); err == nil {
			campos := strings.Fields(string(data))
			if len(campos) >= 3 {
				fmt.Sscanf(campos[0], "%f", &info.LoadAvg1)
				fmt.Sscanf(campos[1], "%f", &info.LoadAvg5)
				fmt.Sscanf(campos[2], "%f", &info.LoadAvg15)
			}
			if len(campos) >= 4 {
				parts := strings.Split(campos[3], "/")
				if len(parts) == 2 {
					fmt.Sscanf(parts[0], "%d", &info.Procesos)
				}
			}
		}

		// /proc/stat para uso %
		info.UsoPorcentaje = calcularUsoCPU()

		// /sys/devices/system/cpu/cpu*/cpufreq para frecuencias
		info.Cores = obtenerCores()
	}

	// Calcular uso % a partir de loadavg / cores (estimación)
	if info.UsoPorcentaje == 0 && info.NumCores > 0 {
		info.UsoPorcentaje = (info.LoadAvg1 / float64(info.NumCores)) * 100
		if info.UsoPorcentaje > 100 {
			info.UsoPorcentaje = 100
		}
	}

	return info
}

// calcularUsoCPU calcula el % de uso de CPU desde /proc/stat.
// Lee dos muestras separadas por 100ms y calcula la diferencia.
func calcularUsoCPU() float64 {
	leer1, _ := leerCPUStat()
	time.Sleep(100 * time.Millisecond)
	leer2, _ := leerCPUStat()
	if leer1 == nil || leer2 == nil {
		return 0
	}
	total1 := leer1[0] + leer1[1] + leer1[2] + leer1[3]
	total2 := leer2[0] + leer2[1] + leer2[2] + leer2[3]
	idle1 := leer1[3]
	idle2 := leer2[3]
	totalDelta := total2 - total1
	idleDelta := idle2 - idle1
	if totalDelta == 0 {
		return 0
	}
	usado := float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	if usado < 0 {
		usado = 0
	}
	return usado
}

// leerCPUStat lee la primera línea de /proc/stat (cpu agregada).
// Retorna [user, nice, system, idle] (jiffies).
func leerCPUStat() ([]int64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}
	linea := strings.SplitN(string(data), "\n", 2)[0]
	campos := strings.Fields(linea)
	if len(campos) < 5 || campos[0] != "cpu" {
		return nil, fmt.Errorf("formato /proc/stat inesperado")
	}
	result := make([]int64, 4)
	for i := 0; i < 4; i++ {
		fmt.Sscanf(campos[i+1], "%d", &result[i])
	}
	return result, nil
}

// obtenerCores retorna info de cada core.
func obtenerCores() []InfoCPUCore {
	var cores []InfoCPUCore
	// Listar /sys/devices/system/cpu/cpu*/cpufreq
	entradas, err := os.ReadDir("/sys/devices/system/cpu")
	if err != nil {
		return nil
	}
	for _, e := range entradas {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "cpu") {
			continue
		}
		numStr := strings.TrimPrefix(e.Name(), "cpu")
		id, err := parseIntSafe(numStr)
		if err != nil {
			continue
		}
		core := InfoCPUCore{ID: id, Online: true}
		// Frecuencia
		path := fmt.Sprintf("/sys/devices/system/cpu/%s/cpufreq/scaling_cur_freq", e.Name())
		if data, err := os.ReadFile(path); err == nil {
			var freq int64
			fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &freq)
			core.Frecuencia = freq
		}
		// Online
		onlinePath := fmt.Sprintf("/sys/devices/system/cpu/%s/online", e.Name())
		if data, err := os.ReadFile(onlinePath); err == nil {
			s := strings.TrimSpace(string(data))
			if s == "0" {
				core.Online = false
			}
		}
		cores = append(cores, core)
	}
	return cores
}

// obtenerMemoria lee /proc/meminfo.
func obtenerMemoria() *InfoMemoria {
	info := &InfoMemoria{}
	if runtime.GOOS != "linux" {
		return info
	}

	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return info
	}

	for _, linea := range strings.Split(string(data), "\n") {
		partes := strings.SplitN(linea, ":", 2)
		if len(partes) != 2 {
			continue
		}
		key := strings.TrimSpace(partes[0])
		campos := strings.Fields(strings.TrimSpace(partes[1]))
		if len(campos) == 0 {
			continue
		}
		var val int64
		fmt.Sscanf(campos[0], "%d", &val)

		switch key {
		case "MemTotal":
			info.TotalKB = val
		case "MemFree":
			info.LibreKB = val
		case "MemAvailable":
			info.DisponibleKB = val
		case "Buffers":
			info.BuffersKB = val
		case "Cached":
			info.CachedKB = val
		case "SwapTotal":
			info.SwapTotalKB = val
		case "SwapFree":
			// SwapUsed = SwapTotal - SwapFree
			info.SwapUsadaKB = info.SwapTotalKB - val
		}
	}

	// Calcular usado
	info.UsadaKB = info.TotalKB - info.LibreKB - info.BuffersKB - info.CachedKB
	if info.TotalKB > 0 {
		info.UsadaPorcentaje = (float64(info.UsadaKB) / float64(info.TotalKB)) * 100
	}

	return info
}

// obtenerDisco retorna métricas de un punto de montaje.
func obtenerDisco(params map[string]interface{}, m *Monitor) (*InfoDisco, error) {
	ruta, _ := herramientas.ObtenerString(params, m.paramByName("ruta_disco"))
	if ruta == "" {
		ruta = "/"
	}

	info := &InfoDisco{Ruta: ruta}

	// statvfs
	var stat syscallStatvfs
	if err := statvfs(ruta, &stat); err == nil {
		info.TotalBytes = int64(stat.f_blocks) * int64(stat.f_frsize)
		info.LibreBytes = int64(stat.f_bfree) * int64(stat.f_frsize)
		info.UsadoBytes = info.TotalBytes - info.LibreBytes
		if info.TotalBytes > 0 {
			info.UsadoPorcentaje = (float64(info.UsadoBytes) / float64(info.TotalBytes)) * 100
		}
		info.InodosTotal = int64(stat.f_files)
		info.InodosLibres = int64(stat.f_ffree)
	}

	return info, nil
}

// obtenerRed retorna métricas de interfaces de red.
func obtenerRed(params map[string]interface{}, m *Monitor) []InfoInterfaz {
	filtro, _ := herramientas.ObtenerString(params, m.paramByName("interfaz_red"))
	var resultado []InfoInterfaz

	if runtime.GOOS == "linux" {
		// /proc/net/dev
		data, err := os.ReadFile("/proc/net/dev")
		if err != nil {
			return nil
		}
		lineas := strings.Split(string(data), "\n")
		for i, l := range lineas {
			if i < 2 {
				continue // headers
			}
			if strings.TrimSpace(l) == "" {
				continue
			}
			partes := strings.SplitN(l, ":", 2)
			if len(partes) != 2 {
				continue
			}
			nombre := strings.TrimSpace(partes[0])
			if filtro != "" && nombre != filtro {
				continue
			}
			stats := strings.Fields(strings.TrimSpace(partes[1]))
			if len(stats) < 16 {
				continue
			}
			iface := InfoInterfaz{Nombre: nombre}
			fmt.Sscanf(stats[0], "%d", &iface.BytesRX)
			fmt.Sscanf(stats[1], "%d", &iface.PaquetesRX)
			fmt.Sscanf(stats[2], "%d", &iface.ErroresRX)
			fmt.Sscanf(stats[8], "%d", &iface.BytesTX)
			fmt.Sscanf(stats[9], "%d", &iface.PaquetesTX)
			fmt.Sscanf(stats[10], "%d", &iface.ErroresTX)
			// Estado UP: /sys/class/net/<iface>/operstate
			if data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/operstate", nombre)); err == nil {
				iface.Up = strings.TrimSpace(string(data)) == "up"
			}
			// MAC
			if data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/address", nombre)); err == nil {
				iface.DireccionMAC = strings.TrimSpace(string(data))
			}
			// MTU
			if data, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/mtu", nombre)); err == nil {
				fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &iface.MTU)
			}
			resultado = append(resultado, iface)
		}
	}

	return resultado
}

// obtenerUptime retorna uptime del sistema.
func obtenerUptime() *InfoUptime {
	info := &InfoUptime{}
	if runtime.GOOS != "linux" {
		return info
	}
	// /proc/uptime: segundos segudos
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		campos := strings.Fields(string(data))
		if len(campos) >= 1 {
			var sec float64
			fmt.Sscanf(campos[0], "%f", &sec)
			info.Segundos = int64(sec)
		}
	}
	// Humanizar
	d := time.Duration(info.Segundos) * time.Second
	info.Humano = humanizarDuracion(d)
	// /proc/stat btime
	if data, err := os.ReadFile("/proc/stat"); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(l, "btime ") {
				var ts int64
				fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(l, "btime ")), "%d", &ts)
				info.InicioBoot = time.Unix(ts, 0).Format(time.RFC3339)
				break
			}
		}
	}
	return info
}

// humanizarDuracion retorna string legible.
func humanizarDuracion(d time.Duration) string {
	dias := int(d.Hours()) / 24
	horas := int(d.Hours()) % 24
	minutos := int(d.Minutes()) % 60
	segundos := int(d.Seconds()) % 60
	partes := []string{}
	if dias > 0 {
		partes = append(partes, fmt.Sprintf("%dd", dias))
	}
	if horas > 0 || dias > 0 {
		partes = append(partes, fmt.Sprintf("%dh", horas))
	}
	if minutos > 0 || horas > 0 || dias > 0 {
		partes = append(partes, fmt.Sprintf("%dm", minutos))
	}
	partes = append(partes, fmt.Sprintf("%ds", segundos))
	return strings.Join(partes, " ")
}

// parseIntSafe wrapper para strconv.Atoi sin import explícito.
func parseIntSafe(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// paramByName busca un parámetro por nombre.
func (m *Monitor) paramByName(nombre string) herramientas.Parametro {
	for _, p := range m.Parametros() {
		if p.Nombre == nombre {
			return p
		}
	}
	return herramientas.Parametro{Nombre: nombre}
}
