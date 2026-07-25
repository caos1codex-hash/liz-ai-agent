package mapa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// EntradaMapa representa un archivo o directorio en el mapa del proyecto.
type EntradaMapa struct {
	Ruta      string `json:"ruta"`
	Tipo      string `json:"tipo"`      // "archivo" o "directorio"
	Lineas    int    `json:"lineas"`    // líneas del archivo (0 para directorios)
	Lenguaje  string `json:"lenguaje"`  // "go", "python", "yaml", "", etc.
	Resumen   string `json:"resumen"`   // descripción corta generada
	Tamanio   int64  `json:"tamanio"`   // bytes
	Modificado string `json:"modificado"` // fecha de modificación
}

// MapaProyecto es el mapa completo de un proyecto.
// Este es el "catálogo de la biblioteca" que se entrega al modelo.
type MapaProyecto struct {
	Version     string               `json:"version"`
	Proyecto    string               `json:"proyecto"`
	Timestamp   string               `json:"timestamp"`
	Archivos    map[string]string    `json:"archivos"`     // ruta → resumen corto
	Estructura  string               `json:"estructura"`  // resumen de estructura de directorios
	Resumen     string               `json:"resumen"`     // descripción general del proyecto
	Entradas    []EntradaMapa        `json:"entradas"`    // lista detallada de entradas
	TotalArchivos int                `json:"total_archivos"`
	TotalDirs    int                 `json:"total_dirs"`
	TotalLineas  int                 `json:"total_lineas"`
}

// OpcionesMapa configura el comportamiento del generador de mapas.
type OpcionesMapa struct {
	IgnorarDirs    []string // directorios a ignorar
	IgnorarArchivos []string // patrones de archivos a ignorar
	MaxLineasMapa  int      // máximo de líneas por archivo para incluir en resumen
	IncluirOcultos bool     // si incluir archivos ocultos
	ProfundidadMax  int      // 0 = sin límite
	SoloLegibles    bool     // solo archivos que se pueden leer
}

// OpcionesPorDefecto retorna opciones por defecto para generar el mapa.
func OpcionesPorDefecto() OpcionesMapa {
	return OpcionesMapa{
		IgnorarDirs: []string{
			".git", ".svn", ".hg", "node_modules", "vendor",
			"__pycache__", ".pytest_cache", ".idea", ".vscode",
			"dist", "build", "bin", ".next", "target",
			"go-local",
		},
		IgnorarArchivos: []string{
			"*.log", "*.tmp", "*.bak", "*.swp", "*.swo",
			"*.min.js", "*.min.css", "*.map", "*.lock",
			"go.sum",
		},
		MaxLineasMapa: 100,
		IncluirOcultos: false,
		ProfundidadMax: 0,
		SoloLegibles:   true,
	}
}

// Generador es el generador de mapas de proyectos.
type Generador struct {
	opciones OpcionesMapa
	logFunc  func(string, ...interface{})
}

// NuevoGenerador crea un nuevo generador de mapas con opciones por defecto.
func NuevoGenerador(opts ...OpcionesMapa) *Generador {
	g := &Generador{
		logFunc: func(string, ...interface{}) {},
	}
	if len(opts) > 0 {
		g.opciones = opts[0]
	} else {
		g.opciones = OpcionesPorDefecto()
	}
	return g
}

// ConLog asigna una función de log al generador.
func (g *Generador) ConLog(fn func(string, ...interface{})) *Generador {
	if fn != nil {
		g.logFunc = fn
	}
	return g
}

// ═══════════════════════════════════════════════════════
// GENERACIÓN DEL MAPA
// ═══════════════════════════════════════════════════════

// Generar genera el mapa completo de un proyecto en la ruta dada.
// El mapa incluye todos los archivos con su resumen, líneas, y tipo.
func (g *Generador) Generar(rutaProyecto string) (*MapaProyecto, error) {
	info, err := os.Stat(rutaProyecto)
	if err != nil {
		return nil, fmt.Errorf("error accediendo a %s: %w", rutaProyecto, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s no es un directorio", rutaProyecto)
	}

	// Resolver ruta absoluta
	rutaAbs, err := filepath.Abs(rutaProyecto)
	if err != nil {
		return nil, fmt.Errorf("error resolviendo ruta: %w", err)
	}

	var (
		mu          sync.Mutex
		entradas    []EntradaMapa
		archivosMap = make(map[string]string)
		totalLineas int
		totalArch   int
		totalDirs   int
		errGen      error
	)

	g.logFunc("generando mapa del proyecto: %s", rutaAbs)

	err = filepath.WalkDir(rutaAbs, func(ruta string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			g.logFunc("advertencia: saltando %s: %v", ruta, walkErr)
			return nil // continuar
		}

		relativa, err := filepath.Rel(rutaAbs, ruta)
		if err != nil {
			return nil
		}
		if relativa == "." {
			return nil
		}

		// Verificar profundidad
		if g.opciones.ProfundidadMax > 0 {
			profundidad := len(strings.Split(relativa, string(filepath.Separator)))
			if profundidad > g.opciones.ProfundidadMax {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Ignorar ocultos
		if !g.opciones.IncluirOcultos && strings.HasPrefix(relativa, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Ignorar directorios específicos
		if d.IsDir() {
			base := filepath.Base(ruta)
			for _, ignorar := range g.opciones.IgnorarDirs {
				if base == ignorar {
					return filepath.SkipDir
				}
			}
			mu.Lock()
			totalDirs++
			mu.Unlock()
			return nil
		}

		// Ignorar archivos por patrón
		base := filepath.Base(ruta)
		for _, patron := range g.opciones.IgnorarArchivos {
			matched, _ := filepath.Match(patron, base)
			if matched {
				return nil
			}
		}

		// Verificar legibilidad
		if g.opciones.SoloLegibles {
			if info, err := d.Info(); err != nil || info.Mode().Perm()&0400 == 0 {
				return nil
			}
		}

		// Leer archivo para generar resumen
		entrada := g.analizarArchivo(ruta, relativa)

		mu.Lock()
		entradas = append(entradas, entrada)
		archivosMap[relativa] = entrada.Resumen
		totalLineas += entrada.Lineas
		totalArch++
		mu.Unlock()

		return nil
	})

	if err != nil {
		errGen = fmt.Errorf("error recorriendo directorio: %w", err)
	}

	// Ordenar entradas por ruta
	sort.Slice(entradas, func(i, j int) bool {
		return entradas[i].Ruta < entradas[j].Ruta
	})

	nombreProyecto := filepath.Base(rutaAbs)

	// Generar resumen de estructura
	estructura := g.generarEstructura(entradas)

	// Generar resumen general del proyecto
	resumen := g.generarResumen(nombreProyecto, totalArch, totalDirs, totalLineas, entradas)

	mapa := &MapaProyecto{
		Version:       "1.0",
		Proyecto:      nombreProyecto,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Archivos:      archivosMap,
		Estructura:    estructura,
		Resumen:       resumen,
		Entradas:      entradas,
		TotalArchivos: totalArch,
		TotalDirs:     totalDirs,
		TotalLineas:   totalLineas,
	}

	g.logFunc("mapa generado: %d archivos, %d líneas en %s", totalArch, totalLineas, nombreProyecto)

	return mapa, errGen
}

// ═══════════════════════════════════════════════════════
// ANÁLISIS DE ARCHIVOS
// ═══════════════════════════════════════════════════════

// analizarArchivo analiza un archivo individual y genera su entrada de mapa.
func (g *Generador) analizarArchivo(rutaAbsoluta, rutaRelativa string) EntradaMapa {
	entrada := EntradaMapa{
		Ruta:     rutaRelativa,
		Tipo:     "archivo",
		Lenguaje: detectarLenguaje(rutaRelativa),
	}

	info, err := os.Stat(rutaAbsoluta)
	if err != nil {
		entrada.Resumen = "(error accediendo al archivo)"
		return entrada
	}

	entrada.Tamanio = info.Size()
	entrada.Modificado = info.ModTime().UTC().Format(time.RFC3339)

	// Contar líneas y generar resumen
	contenido, err := os.ReadFile(rutaAbsoluta)
	if err != nil {
		entrada.Resumen = fmt.Sprintf("(error leyendo archivo, %d bytes)", info.Size())
		return entrada
	}

	lineas := strings.Count(string(contenido), "\n")
	if len(contenido) > 0 && contenido[len(contenido)-1] != '\n' {
		lineas++
	}
	entrada.Lineas = lineas

	// Generar resumen basado en el contenido
	entrada.Resumen = g.resumirArchivo(string(contenido), rutaRelativa, lineas)

	return entrada
}

// resumirArchivo genera un resumen corto del contenido de un archivo.
// El resumen debe ser lo suficientemente descriptivo para el modelo.
func (g *Generador) resumirArchivo(contenido, ruta string, lineas int) string {
	// Para archivos cortos, usar las primeras líneas como contexto
	if lineas <= g.opciones.MaxLineasMapa {
		// Buscar comentarios del encabezado o primera línea significativa
		primerasLineas := tomarPrimerasLineas(contenido, 3)
		primerasLineas = strings.TrimSpace(primerasLineas)
		if len(primerasLineas) > 80 {
			primerasLineas = primerasLineas[:80] + "..."
		}
		if primerasLineas != "" {
			return fmt.Sprintf("%d líneas — %s", lineas, limpiarComentario(primerasLineas))
		}
	}

	// Para archivos largos, intentar detectar estructura (Go: package/func/type)
	lang := detectarLenguaje(ruta)
	switch lang {
	case "go":
		return resumirGo(contenido, lineas)
	case "python":
		return resumirPython(contenido, lineas)
	case "yaml", "yml":
		return resumirYAML(contenido, lineas)
	case "markdown", "md":
		return fmt.Sprintf("documentación Markdown, %d líneas", lineas)
	case "json":
		return fmt.Sprintf("datos JSON, %d líneas", lineas)
	default:
		return fmt.Sprintf("archivo %s, %d líneas", lang, lineas)
	}
}

// ═══════════════════════════════════════════════════════
// FUNCIONES AUXILIARES
// ═══════════════════════════════════════════════════════

// generarEstructura crea un resumen de la estructura de directorios.
func (g *Generador) generarEstructura(entradas []EntradaMapa) string {
	// Agrupar por directorio
	dirs := make(map[string]int)
	for _, e := range entradas {
		dir := filepath.Dir(e.Ruta)
		dirs[dir]++
	}

	// Ordenar y generar descripción
	var dirNames []string
	for dir := range dirs {
		if dir != "." {
			dirNames = append(dirNames, fmt.Sprintf("%s/ → %d archivos", dir, dirs[dir]))
		}
	}
	sort.Strings(dirNames)
	
	// Archivos en raíz
	rootCount := 0
	for _, e := range entradas {
		if filepath.Dir(e.Ruta) == "." {
			rootCount++
		}
	}
	if rootCount > 0 {
		dirNames = append([]string{fmt.Sprintf("(raíz) → %d archivos", rootCount)}, dirNames...)
	}

	return strings.Join(dirNames, ", ")
}

// generarResumen crea un resumen general del proyecto.
func (g *Generador) generarResumen(nombre string, totalArch, totalDirs, totalLineas int, entradas []EntradaMapa) string {
	// Detectar tipo de proyecto basado en archivos
	var langs []string
	langCount := make(map[string]int)
	for _, e := range entradas {
		if e.Lenguaje != "" {
			langCount[e.Lenguaje]++
		}
	}
	for lang, count := range langCount {
		langs = append(langs, fmt.Sprintf("%d %s", count, lang))
	}
	sort.Strings(langs)

	return fmt.Sprintf("Proyecto '%s': %d archivos, %d directorios, %d líneas totales [%s]",
		nombre, totalArch, totalDirs, totalLineas, strings.Join(langs, ", "))
}

// ═══════════════════════════════════════════════════════
// PERSISTENCIA
// ═══════════════════════════════════════════════════════

// Guardar persiste el mapa en formato JSON.
func (g *Generador) Guardar(mapa *MapaProyecto, ruta string) error {
	datos, err := json.MarshalIndent(mapa, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando mapa: %w", err)
	}

	if err := os.WriteFile(ruta, datos, 0644); err != nil {
		return fmt.Errorf("error guardando mapa en %s: %w", ruta, err)
	}

	g.logFunc("mapa guardado en %s", ruta)
	return nil
}

// Cargar lee un mapa desde un archivo JSON.
func Cargar(ruta string) (*MapaProyecto, error) {
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return nil, fmt.Errorf("error leyendo mapa: %w", err)
	}

	var mapa MapaProyecto
	if err := json.Unmarshal(datos, &mapa); err != nil {
		return nil, fmt.Errorf("error parseando mapa: %w", err)
	}

	return &mapa, nil
}

// ═══════════════════════════════════════════════════════
// DETECCIÓN DE LENGUAJE
// ═══════════════════════════════════════════════════════

// detectarLenguaje retorna el lenguaje de programación basado en la extensión.
func detectarLenguaje(ruta string) string {
	ext := strings.ToLower(filepath.Ext(ruta))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".sass":
		return "css"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc":
		return "cpp"
	case ".sh", ".bash":
		return "shell"
	case ".sql":
		return "sql"
	case ".toml":
		return "toml"
	case ".mod", ".sum":
		return "go-module"
	case ".lock":
		return "lock"
	default:
		if filepath.Base(ruta) == "Makefile" {
			return "makefile"
		}
		if filepath.Base(ruta) == "Dockerfile" {
			return "docker"
		}
		if filepath.Base(ruta) == "LICENSE" {
			return "license"
		}
		return ""
	}
}

// ═══════════════════════════════════════════════════════
// RESUMEN POR LENGUAJE
// ═══════════════════════════════════════════════════════

func resumirGo(contenido string, lineas int) string {
	// Contar tipos principales
	var funciones, tipos, vars int
	lineasArr := strings.Split(contenido, "\n")

	for _, linea := range lineasArr {
		trim := strings.TrimSpace(linea)
		if strings.HasPrefix(trim, "func ") && !strings.HasPrefix(trim, "func (") {
			funciones++
		} else if strings.HasPrefix(trim, "func (") {
			funciones++ // método
		}
		if strings.HasPrefix(trim, "type ") {
			tipos++
		}
		if strings.HasPrefix(trim, "var ") || strings.HasPrefix(trim, "const ") {
			vars++
		}
	}

	var partes []string
	if funciones > 0 {
		partes = append(partes, fmt.Sprintf("%d func", funciones))
	}
	if tipos > 0 {
		partes = append(partes, fmt.Sprintf("%d type", tipos))
	}
	if vars > 0 {
		partes = append(partes, fmt.Sprintf("%d var/const", vars))
	}

	if len(partes) > 0 {
		return fmt.Sprintf("Go, %d líneas [%s]", lineas, strings.Join(partes, ", "))
	}
	return fmt.Sprintf("Go, %d líneas", lineas)
}

func resumirPython(contenido string, lineas int) string {
	var clases, funciones int
	for _, linea := range strings.Split(contenido, "\n") {
		trim := strings.TrimSpace(linea)
		if strings.HasPrefix(trim, "class ") {
			clases++
		}
		if strings.HasPrefix(trim, "def ") {
			funciones++
		}
	}
	return fmt.Sprintf("Python, %d líneas [%d class, %d func]", lineas, clases, funciones)
}

func resumirYAML(contenido string, lineas int) string {
	// Contar claves de primer nivel
	claves := 0
	for _, linea := range strings.Split(contenido, "\n") {
		trim := strings.TrimSpace(linea)
		if len(trim) > 0 && trim[0] != '#' && !strings.HasPrefix(trim, "-") &&
			strings.Contains(trim, ":") && !strings.HasPrefix(trim, " ") {
			claves++
		}
	}
	return fmt.Sprintf("configuración YAML, %d líneas, ~%d secciones", lineas, claves)
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// tomarPrimerasLineas retorna las primeras N líneas de un texto.
func tomarPrimerasLineas(texto string, n int) string {
	lineas := strings.SplitN(texto, "\n", n+1)
	return strings.Join(lineas, " \n ")
}

// limpiarComentario elimina prefijos de comentario comunes.
func limpiarComentario(texto string) string {
	for _, prefijo := range []string{"// ", "# ", "/* ", "* ", "-- ", "<!-- ", "> ", ""} {
		if strings.HasPrefix(texto, prefijo) {
			return strings.TrimPrefix(texto, prefijo)
		}
	}
	return texto
}
