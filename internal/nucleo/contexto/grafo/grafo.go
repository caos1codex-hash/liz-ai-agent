// Package grafo implementa el grafo de dependencias de código y
// el cálculo de importancia de archivos vía PageRank.
//
// Inspirado en Aider's "repo map" importance ranking: archivos que son
// importados por muchos otros archivos son más "centrales" al proyecto
// y reciben un score de importancia más alto.
//
// El algoritmo:
//  1. Construir grafo dirigido: archivo_A → archivo_B si A importa a B
//  2. Ejecutar PageRank iterativo (50 iteraciones, damping 0.85)
//  3. Normalizar scores a [0.0, 1.0]
//
// Los scores se usan para:
//   - Ordenar archivos en el Repository Map
//   - Priorizar fragmentos en el Context Packer
//   - Sugerir "archivos importantes" al usuario
package grafo

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// Grafo es el grafo dirigido de dependencias entre archivos.
type Grafo struct {
	mu          sync.RWMutex
	archivo     string // ruta del archivo JSON de persistencia
	nodos       map[string]*Nodo
	aristas     map[string]map[string]bool // origen → set(destinos)
	persistente bool
}

// Nodo representa un archivo en el grafo.
type Nodo struct {
	Ruta         string  `json:"ruta"`
	Lenguaje     string  `json:"lenguaje"`
	Lineas       int     `json:"lineas"`
	Importancia  float64 `json:"importancia"`  // score PageRank [0.0, 1.0]
	Imports      int     `json:"imports"`      // cuántos archivos importa
	Importadores int     `json:"importadores"` // cuántos archivos lo importan
}

// EstadisticasGrafo resume el estado del grafo.
type EstadisticasGrafo struct {
	TotalArchivos   int     `json:"total_archivos"`
	TotalAristas    int     `json:"total_aristas"`
	Densidad        float64 `json:"densidad"` // aristas / (n*(n-1))
	PromedioImports float64 `json:"promedio_imports"`
	ArchivoTop      string  `json:"archivo_top"` // más importante
	ScoreTop        float64 `json:"score_top"`
}

// ═══════════════════════════════════════════════════════
// CONSTRUCTOR
// ═══════════════════════════════════════════════════════

// NuevoGrafo crea un nuevo grafo vacío.
func NuevoGrafo() *Grafo {
	return &Grafo{
		nodos:   make(map[string]*Nodo),
		aristas: make(map[string]map[string]bool),
	}
}

// ═══════════════════════════════════════════════════════
// CONSTRUCCIÓN
// ═══════════════════════════════════════════════════════

// AgregarArchivo registra un archivo en el grafo.
// Si ya existe, actualiza su metadata.
func (g *Grafo) AgregarArchivo(ruta, lenguaje string, lineas int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	nodo, existe := g.nodos[ruta]
	if !existe {
		nodo = &Nodo{Ruta: ruta}
		g.nodos[ruta] = nodo
	}
	nodo.Lenguaje = lenguaje
	nodo.Lineas = lineas
}

// AgregarImport registra que un archivo importa a otro.
// Ambas rutas deben ser rutas relativas al proyecto.
func (g *Grafo) AgregarImport(origen, destino string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// No self-loops
	if origen == destino {
		return
	}

	// Asegurar que ambos nodos existen
	if _, ok := g.nodos[origen]; !ok {
		g.nodos[origen] = &Nodo{Ruta: origen}
	}
	if _, ok := g.nodos[destino]; !ok {
		g.nodos[destino] = &Nodo{Ruta: destino}
	}

	if g.aristas[origen] == nil {
		g.aristas[origen] = make(map[string]bool)
	}
	if !g.aristas[origen][destino] {
		g.aristas[origen][destino] = true
		g.nodos[origen].Imports++
		g.nodos[destino].Importadores++
	}
}

// RemoverArchivo quita un archivo del grafo y todas sus aristas.
func (g *Grafo) RemoverArchivo(ruta string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Quitar aristas salientes
	for destino := range g.aristas[ruta] {
		if nodo, ok := g.nodos[destino]; ok {
			nodo.Importadores--
		}
	}
	delete(g.aristas, ruta)

	// Quitar aristas entrantes
	for origen, destinos := range g.aristas {
		if destinos[ruta] {
			delete(destinos, ruta)
			if nodo, ok := g.nodos[origen]; ok {
				nodo.Imports--
			}
		}
	}

	delete(g.nodos, ruta)
}

// ═══════════════════════════════════════════════════════
// PAGERANK
// ═══════════════════════════════════════════════════════

// CalcularImportancia ejecuta PageRank sobre el grafo.
//
// Fórmula clásica de PageRank:
//
//	PR(p) = (1 - d) / N + d * sum(PR(q) / L(q) for q in Backlinks(p))
//
// donde:
//   - d = damping factor (0.85 por defecto)
//   - N = total de nodos
//   - Backlinks(p) = archivos que importan a p
//   - L(q) = total de archivos que q importa
//
// Los scores se normalizan a [0.0, 1.0] dividiendo por el máximo.
func (g *Grafo) CalcularImportancia(iteraciones int, damping float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.nodos) == 0 {
		return
	}

	if iteraciones <= 0 {
		iteraciones = 50
	}
	if damping <= 0 || damping >= 1 {
		damping = 0.85
	}

	n := len(g.nodos)
	initialScore := 1.0 / float64(n)

	// Inicializar scores
	scores := make(map[string]float64, n)
	for ruta := range g.nodos {
		scores[ruta] = initialScore
	}

	// Construir índice de backlinks (reverse edges)
	backlinks := make(map[string][]string)
	for origen, destinos := range g.aristas {
		for destino := range destinos {
			backlinks[destino] = append(backlinks[destino], origen)
		}
	}

	// Iteraciones de PageRank
	for i := 0; i < iteraciones; i++ {
		newScores := make(map[string]float64, n)
		for ruta := range g.nodos {
			sum := 0.0
			for _, backlink := range backlinks[ruta] {
				// L(q) = número de aristas salientes de q
				lq := len(g.aristas[backlink])
				if lq > 0 {
					sum += scores[backlink] / float64(lq)
				}
			}
			// Dangling node handling: si un nodo no tiene salidas,
			// su score se redistribuye uniformemente
			danglingSum := 0.0
			for ruta2 := range g.nodos {
				if len(g.aristas[ruta2]) == 0 {
					danglingSum += scores[ruta2]
				}
			}
			sum += danglingSum / float64(n)

			newScores[ruta] = (1.0-damping)/float64(n) + damping*sum
		}
		scores = newScores
	}

	// Normalizar a [0.0, 1.0] dividiendo por el máximo
	maxScore := 0.0
	for _, s := range scores {
		if s > maxScore {
			maxScore = s
		}
	}
	if maxScore > 0 {
		for ruta, s := range scores {
			g.nodos[ruta].Importancia = s / maxScore
		}
	}
}

// ═══════════════════════════════════════════════════════
// CONSULTAS
// ═══════════════════════════════════════════════════════

// Obtener retorna el nodo de un archivo.
func (g *Grafo) Obtener(ruta string) (*Nodo, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	nodo, ok := g.nodos[ruta]
	if !ok {
		return nil, false
	}
	copia := *nodo
	return &copia, true
}

// ObtenerTodos retorna todos los nodos (copia).
func (g *Grafo) ObtenerTodos() []Nodo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	resultado := make([]Nodo, 0, len(g.nodos))
	for _, nodo := range g.nodos {
		resultado = append(resultado, *nodo)
	}
	return resultado
}

// TopN retorna los N archivos más importantes (descendente por score).
func (g *Grafo) TopN(n int) []Nodo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	todos := make([]Nodo, 0, len(g.nodos))
	for _, nodo := range g.nodos {
		todos = append(todos, *nodo)
	}

	sort.Slice(todos, func(i, j int) bool {
		return todos[i].Importancia > todos[j].Importancia
	})

	if n > len(todos) {
		n = len(todos)
	}
	return todos[:n]
}

// ImportanciasDe retorna un mapa ruta → score de todos los nodos.
func (g *Grafo) ImportanciasDe() map[string]float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	resultado := make(map[string]float64, len(g.nodos))
	for ruta, nodo := range g.nodos {
		resultado[ruta] = nodo.Importancia
	}
	return resultado
}

// Vecinos retorna los archivos que este archivo importa (salientes).
func (g *Grafo) Vecinos(ruta string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	destinos := g.aristas[ruta]
	resultado := make([]string, 0, len(destinos))
	for d := range destinos {
		resultado = append(resultado, d)
	}
	sort.Strings(resultado)
	return resultado
}

// Backlinks retorna los archivos que importan a este archivo (entrantes).
func (g *Grafo) Backlinks(ruta string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var resultado []string
	for origen, destinos := range g.aristas {
		if destinos[ruta] {
			resultado = append(resultado, origen)
		}
	}
	sort.Strings(resultado)
	return resultado
}

// Estadisticas retorna métricas resumidas del grafo.
func (g *Grafo) Estadisticas() EstadisticasGrafo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	totalAristas := 0
	for _, destinos := range g.aristas {
		totalAristas += len(destinos)
	}

	n := len(g.nodos)
	densidad := 0.0
	if n > 1 {
		densidad = float64(totalAristas) / float64(n*(n-1))
	}

	promedioImports := 0.0
	if n > 0 {
		totalImports := 0
		for _, nodo := range g.nodos {
			totalImports += nodo.Imports
		}
		promedioImports = float64(totalImports) / float64(n)
	}

	// Top archivo
	var topRuta string
	var topScore float64
	for ruta, nodo := range g.nodos {
		if nodo.Importancia > topScore {
			topScore = nodo.Importancia
			topRuta = ruta
		}
	}

	return EstadisticasGrafo{
		TotalArchivos:   n,
		TotalAristas:    totalAristas,
		Densidad:        densidad,
		PromedioImports: promedioImports,
		ArchivoTop:      topRuta,
		ScoreTop:        topScore,
	}
}

// TotalArchivos retorna el número de archivos en el grafo.
func (g *Grafo) TotalArchivos() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodos)
}

// TotalAristas retorna el número de aristas (imports) en el grafo.
func (g *Grafo) TotalAristas() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	total := 0
	for _, destinos := range g.aristas {
		total += len(destinos)
	}
	return total
}

// ═══════════════════════════════════════════════════════
// HELPERS DE IMPORTS GO
// ═══════════════════════════════════════════════════════

// ResolverImportGo resuelve un import path de Go (e.g. "github.com/foo/bar/baz")
// a una ruta relativa dentro del proyecto (e.g. "internal/baz/").
//
// Heurística simple: si el import empieza con el módulo del proyecto,
// se considera interno. Para encontrar la ruta relativa:
//  1. Tomar el último segmento del módulo (e.g. "liz-ai-agent")
//  2. Si el import contiene ese segmento, tomar lo que sigue como ruta relativa
//  3. Sino, no se puede resolver (es un import externo)
func ResolverImportGo(importPath, moduloProyecto string) string {
	if moduloProyecto == "" {
		return ""
	}
	// Buscar la posición del módulo en el import path
	idx := strings.Index(importPath, moduloProyecto)
	if idx < 0 {
		return ""
	}
	relativa := strings.TrimPrefix(importPath[idx+len(moduloProyecto):], "/")
	if relativa == "" {
		return ""
	}
	// Convertir a ruta de archivo Go (pkg → pkg/<algo>.go)
	// No podemos saber el archivo exacto sin más info, pero podemos
	// retornar el directorio del paquete.
	return relativa + "/"
}

// NormalizarRutaGo convierte una ruta de archivo Go a su "package path"
// (el directorio que lo contiene). Esto permite matching entre imports y archivos.
//
// Ejemplo: "internal/nucleo/contexto/mapa/mapa.go" → "internal/nucleo/contexto/mapa"
func NormalizarRutaGo(rutaArchivo string) string {
	dir := filepath.Dir(rutaArchivo)
	if dir == "." {
		return ""
	}
	return dir
}

// MatchImportArchivo verifica si un import path de Go corresponde a un archivo
// del proyecto, dado el módulo del proyecto.
//
// Ejemplo:
//
//	importPath = "github.com/caos1codex-hash/liz-ai-agent/internal/auth"
//	moduloProyecto = "github.com/caos1codex-hash/liz-ai-agent"
//	archivoCandidato = "internal/auth/jwt.go"
//	→ true (porque archivoCandidato empieza con "internal/auth")
func MatchImportArchivo(importPath, moduloProyecto, archivoCandidato string) bool {
	resuelta := ResolverImportGo(importPath, moduloProyecto)
	if resuelta == "" {
		return false
	}
	return strings.HasPrefix(archivoCandidato, resuelta)
}

// String retorna una representación legible del grafo (para debugging).
func (g *Grafo) String() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Grafo: %d archivos, %d aristas\n",
		len(g.nodos), g.TotalAristas()))

	top := g.TopN(5)
	b.WriteString("Top 5 por importancia:\n")
	for _, nodo := range top {
		b.WriteString(fmt.Sprintf("  %.3f  %s (imports=%d, importadores=%d)\n",
			nodo.Importancia, nodo.Ruta, nodo.Imports, nodo.Importadores))
	}
	return b.String()
}

// ═══════════════════════════════════════════════════════
// CACHE DE PAGERANK
// ═══════════════════════════════════════════════════════

// pagerankCache es la estructura para persistir los scores calculados.
type pagerankCache struct {
	Hash   string             `json:"hash"`   // hash del estado del grafo
	Scores map[string]float64 `json:"scores"` // ruta → score
}

// hashGrafo calcula un hash SHA-256 del estado del grafo (nodos + aristas).
// Se usa para detectar si el grafo cambió desde la última vez que se calculó PageRank.
func (g *Grafo) hashGrafo() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	h := sha256.New()
	// Ordenar nodos para hash determinístico
	rutas := make([]string, 0, len(g.nodos))
	for r := range g.nodos {
		rutas = append(rutas, r)
	}
	sort.Strings(rutas)
	for _, r := range rutas {
		n := g.nodos[r]
		fmt.Fprintf(h, "%s|%s|%d|%.6f|", r, n.Lenguaje, n.Lineas, n.Importancia)
	}
	// Aristas
	origenes := make([]string, 0, len(g.aristas))
	for o := range g.aristas {
		origenes = append(origenes, o)
	}
	sort.Strings(origenes)
	for _, o := range origenes {
		destinos := make([]string, 0, len(g.aristas[o]))
		for d := range g.aristas[o] {
			destinos = append(destinos, d)
		}
		sort.Strings(destinos)
		for _, d := range destinos {
			fmt.Fprintf(h, "%s->%s|", o, d)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// CargarPageRankCache intenta cargar scores de PageRank desde un archivo JSON.
// Retorna true si se cargaron scores válidos (el grafo no cambió).
func (g *Grafo) CargarPageRankCache(rutaArchivo string) bool {
	// Leer archivo sin locks (no modifica estado)
	data, err := os.ReadFile(rutaArchivo)
	if err != nil {
		return false
	}

	var cache pagerankCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return false
	}

	// Calcular hash actual (usa RLock internamente)
	hashActual := g.hashGrafo()
	if hashActual != cache.Hash {
		return false // el grafo cambió
	}

	// Aplicar scores cacheados
	g.mu.Lock()
	defer g.mu.Unlock()
	for ruta, score := range cache.Scores {
		if nodo, ok := g.nodos[ruta]; ok {
			nodo.Importancia = score
		}
	}
	return true
}

// GuardarPageRankCache persiste los scores de PageRank a JSON.
// Se debe llamar después de CalcularImportancia.
func (g *Grafo) GuardarPageRankCache(rutaArchivo string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(rutaArchivo), 0755); err != nil {
		return fmt.Errorf("creando directorio cache PageRank: %w", err)
	}

	cache := pagerankCache{
		Hash:   g.hashGrafo(),
		Scores: make(map[string]float64, len(g.nodos)),
	}
	for ruta, nodo := range g.nodos {
		cache.Scores[ruta] = nodo.Importancia
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando cache PageRank: %w", err)
	}

	tmp := rutaArchivo + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("escribiendo cache PageRank: %w", err)
	}
	return os.Rename(tmp, rutaArchivo)
}
