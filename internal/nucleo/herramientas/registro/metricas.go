package registro

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ============================================================================
// Métricas por Herramienta
// ============================================================================

// MetricaHerramienta resume las métricas de una herramienta.
type MetricaHerramienta struct {
	Nombre           string        `json:"nombre"`
	Ejecuciones      int64         `json:"ejecuciones"`
	Exitos           int64         `json:"exitos"`
	Fallos           int64         `json:"fallos"`
	TasaExito        float64       `json:"tasa_exito"`
	LatenciaPromedio time.Duration `json:"latencia_promedio_ms"`
	LatenciaMin      time.Duration `json:"latencia_min_ms"`
	LatenciaMax      time.Duration `json:"latencia_max_ms"`
	UltimoUso        time.Time     `json:"ultimo_uso"`
	UltimoError      string        `json:"ultimo_error,omitempty"`
}

// Metricas es un colector thread-safe de métricas por herramienta.
// Se invoca automáticamente desde Catalogo.Ejecutar().
type Metricas struct {
	mu      sync.RWMutex
	porHerr map[string]*metricasInternas
}

type metricasInternas struct {
	ejecuciones     int64
	exitos          int64
	fallos          int64
	latenciaTotalNs int64
	latenciaMinNs   int64
	latenciaMaxNs   int64
	ultimoUso       time.Time
	ultimoError     string
}

// NuevasMetricas crea un colector de métricas vacío.
func NuevasMetricas() *Metricas {
	return &Metricas{porHerr: make(map[string]*metricasInternas)}
}

// RegistrarEjecucion actualiza las métricas tras una ejecución.
// exito indica si la herramienta completó con éxito (no si hubo panic/error).
// duracion es el tiempo total de la ejecución.
func (m *Metricas) RegistrarEjecucion(nombre string, exito bool, duracion time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mi, ok := m.porHerr[nombre]
	if !ok {
		mi = &metricasInternas{
			latenciaMinNs: (1 << 62), // max int64
		}
		m.porHerr[nombre] = mi
	}

	mi.ejecuciones++
	mi.ultimoUso = time.Now()
	mi.latenciaTotalNs += int64(duracion)
	if int64(duracion) < mi.latenciaMinNs {
		mi.latenciaMinNs = int64(duracion)
	}
	if int64(duracion) > mi.latenciaMaxNs {
		mi.latenciaMaxNs = int64(duracion)
	}

	if exito {
		mi.exitos++
		mi.ultimoError = ""
	} else {
		mi.fallos++
	}
}

// RegistrarError registra el último mensaje de error de una herramienta.
// Se llama por separado para no acoplar el formato del error a la métrica.
func (m *Metricas) RegistrarError(nombre, mensaje string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mi, ok := m.porHerr[nombre]
	if !ok {
		mi = &metricasInternas{latenciaMinNs: (1 << 62)}
		m.porHerr[nombre] = mi
	}
	mi.ultimoError = mensaje
}

// Obtener retorna las métricas de una herramienta específica.
// Si la herramienta nunca se ha ejecutado, retorna ceros.
func (m *Metricas) Obtener(nombre string) MetricaHerramienta {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mi, ok := m.porHerr[nombre]
	if !ok {
		return MetricaHerramienta{Nombre: nombre}
	}

	return m.snapshotUna(mi, nombre)
}

// Listar retorna las métricas de todas las herramientas, ordenadas por nombre.
func (m *Metricas) Listar() []MetricaHerramienta {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]MetricaHerramienta, 0, len(m.porHerr))
	for nombre, mi := range m.porHerr {
		result = append(result, m.snapshotUna(mi, nombre))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Nombre < result[j].Nombre
	})
	return result
}

// Reset reinicia todas las métricas (uso en tests, no en producción).
func (m *Metricas) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.porHerr = make(map[string]*metricasInternas)
}

// snapshotUna genera un MetricaHerramienta desde metricasInternas.
// NO toma el lock (debe tomarse desde el llamador).
func (m *Metricas) snapshotUna(mi *metricasInternas, nombre string) MetricaHerramienta {
	var tasa float64
	if mi.ejecuciones > 0 {
		tasa = float64(mi.exitos) / float64(mi.ejecuciones)
	}

	latProm := time.Duration(0)
	if mi.ejecuciones > 0 {
		latProm = time.Duration(mi.latenciaTotalNs / mi.ejecuciones)
	}

	latMin := time.Duration(mi.latenciaMinNs)
	if mi.ejecuciones == 0 {
		latMin = 0
	}

	return MetricaHerramienta{
		Nombre:           nombre,
		Ejecuciones:      mi.ejecuciones,
		Exitos:           mi.exitos,
		Fallos:           mi.fallos,
		TasaExito:        tasa,
		LatenciaPromedio: latProm,
		LatenciaMin:      latMin,
		LatenciaMax:      time.Duration(mi.latenciaMaxNs),
		UltimoUso:        mi.ultimoUso,
		UltimoError:      mi.ultimoError,
	}
}

// Resumen retorna estadísticas agregadas de todas las herramientas.
type ResumenMetricas struct {
	TotalHerramientas int                  `json:"total_herramientas"`
	TotalEjecuciones  int64                `json:"total_ejecuciones"`
	TotalExitos       int64                `json:"total_exitos"`
	TotalFallos       int64                `json:"total_fallos"`
	TasaExitoGlobal   float64              `json:"tasa_exito_global"`
	PorHerramienta    []MetricaHerramienta `json:"por_herramienta"`
}

// Resumen retorna las métricas globales agregadas.
func (m *Metricas) Resumen() ResumenMetricas {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalEjec, totalExit, totalFallos int64
	porHerr := make([]MetricaHerramienta, 0, len(m.porHerr))

	for nombre, mi := range m.porHerr {
		totalEjec += mi.ejecuciones
		totalExit += mi.exitos
		totalFallos += mi.fallos
		porHerr = append(porHerr, m.snapshotUna(mi, nombre))
	}
	sort.Slice(porHerr, func(i, j int) bool {
		return porHerr[i].Nombre < porHerr[j].Nombre
	})

	var tasaGlobal float64
	if totalEjec > 0 {
		tasaGlobal = float64(totalExit) / float64(totalEjec)
	}

	return ResumenMetricas{
		TotalHerramientas: len(m.porHerr),
		TotalEjecuciones:  totalEjec,
		TotalExitos:       totalExit,
		TotalFallos:       totalFallos,
		TasaExitoGlobal:   tasaGlobal,
		PorHerramienta:    porHerr,
	}
}

// String para logging/debugging.
func (r ResumenMetricas) String() string {
	return fmt.Sprintf("Métricas: %d herramientas, %d ejecuciones (%.1f%% éxito)",
		r.TotalHerramientas, r.TotalEjecuciones, r.TasaExitoGlobal*100)
}
