package resumen

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

// ResumenArchivo es el resumen detallado de un archivo.
// Se almacena como JSON individual en ~/.liz/contexto/proyectos/<proyecto>/.liz/archivos/
type ResumenArchivo struct {
        Ruta        string `json:"ruta"`
        Lenguaje    string `json:"lenguaje"`
        Lineas      int    `json:"lineas"`
        Descripcion string `json:"descripcion"` // resumen de 1-2 líneas
        Exportados  []string `json:"exportados"`  // funciones/tipos/vars exportados
        Importados  []string `json:"importados"`  // paquetes importados
        Dependencias []string `json:"dependencias"` // archivos que importa internamente
        TipoArchivo string `json:"tipo_archivo"` // "codigo", "config", "docs", "test", etc.
        Complejidad string `json:"complejidad"`  // "baja", "media", "alta"
        Timestamp   string `json:"timestamp"`
}

// TipoArchivo detecta la categoría de un archivo.
func TipoArchivo(ruta string) string {
        base := strings.ToLower(filepath.Base(ruta))
        ext := strings.ToLower(filepath.Ext(ruta))

        // Tests: sufijos comunes (_test.go, _test.py, .test.js, .spec.ts, etc.)
        if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.py") ||
                strings.HasSuffix(base, "_test.js") || strings.HasSuffix(base, "_test.ts") ||
                strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".test.ts") ||
                strings.HasSuffix(base, ".spec.js") || strings.HasSuffix(base, ".spec.ts") ||
                strings.HasPrefix(base, "test_") {
                return "test"
        }

        switch ext {
        case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".java", ".c", ".cpp", ".h", ".hpp":
                return "codigo"
        case ".yaml", ".yml", ".toml", ".json", ".env", ".ini", ".cfg":
                return "config"
        case ".md", ".rst", ".txt":
                return "docs"
        case ".html", ".htm", ".css", ".scss", ".sass", ".less":
                return "frontend"
        case ".sh", ".bash", ".zsh", ".fish":
                return "script"
        default:
                // Archivos sin extensión relevantes
                switch base {
                case "makefile", "dockerfile", "justfile", "rakefile", "gemfile":
                        return "script"
                case "license", "readme", "contributing", "changelog", "authors", "notice":
                        return "docs"
                }
                return "otro"
        }
}

// Generador genera resúmenes detallados de archivos.
type Generador struct {
        logFunc    func(string, ...interface{})
        dirResumen string // directorio base para persistencia (opcional)
        mu         sync.Mutex
        cache      map[string]*ResumenArchivo // cache en memoria por ruta
}

// NuevoGenerador crea un nuevo generador de resúmenes.
func NuevoGenerador() *Generador {
        return &Generador{
                logFunc: func(string, ...interface{}) {},
                cache:   make(map[string]*ResumenArchivo),
        }
}

// ConLog asigna función de log.
func (g *Generador) ConLog(fn func(string, ...interface{})) *Generador {
        if fn != nil {
                g.logFunc = fn
        }
        return g
}

// ConDirResumen asigna el directorio donde se persistirán los resúmenes
// (e.g. ~/.liz/contexto/proyectos/<nombre>/.liz/resumenes/).
// Si no se asigna, Guardar() fallará con error.
func (g *Generador) ConDirResumen(dir string) *Generador {
        g.dirResumen = dir
        _ = os.MkdirAll(dir, 0755)
        return g
}

// rutaResumen retorna la ruta del archivo JSON para un resumen dado.
// Convierte "src/auth/jwt.go" → "src_auth_jwt.go.json".
func rutaResumen(rutaRelativa string) string {
        limpia := strings.ReplaceAll(rutaRelativa, "/", "_")
        limpia = strings.ReplaceAll(limpia, "\\", "_")
        limpia = strings.ReplaceAll(limpia, " ", "_")
        if !strings.HasSuffix(limpia, ".json") {
                limpia += ".json"
        }
        return limpia
}

// Guardar persiste un resumen a disco en dirResumen.
// Requiere que ConDirResumen haya sido llamado.
func (g *Generador) Guardar(r *ResumenArchivo) error {
        if g.dirResumen == "" {
                return fmt.Errorf("dirResumen no configurado; usar ConDirResumen()")
        }
        if r == nil {
                return fmt.Errorf("resumen nil")
        }

        rutaArchivo := filepath.Join(g.dirResumen, rutaResumen(r.Ruta))
        datos, err := json.MarshalIndent(r, "", "  ")
        if err != nil {
                return fmt.Errorf("error serializando resumen: %w", err)
        }

        if err := os.WriteFile(rutaArchivo, datos, 0644); err != nil {
                return fmt.Errorf("error guardando resumen %s: %w", rutaArchivo, err)
        }

        // Actualizar cache
        g.mu.Lock()
        g.cache[r.Ruta] = r
        g.mu.Unlock()

        return nil
}

// Cargar lee un resumen desde disco. Retorna nil si no existe.
func (g *Generador) Cargar(rutaRelativa string) (*ResumenArchivo, error) {
        // Cache
        g.mu.Lock()
        if cached, ok := g.cache[rutaRelativa]; ok {
                g.mu.Unlock()
                return cached, nil
        }
        g.mu.Unlock()

        if g.dirResumen == "" {
                return nil, nil
        }

        rutaArchivo := filepath.Join(g.dirResumen, rutaResumen(rutaRelativa))
        datos, err := os.ReadFile(rutaArchivo)
        if err != nil {
                return nil, nil // no existe = nil
        }

        var r ResumenArchivo
        if err := json.Unmarshal(datos, &r); err != nil {
                return nil, fmt.Errorf("error parseando resumen %s: %w", rutaArchivo, err)
        }

        g.mu.Lock()
        g.cache[rutaRelativa] = &r
        g.mu.Unlock()

        return &r, nil
}

// Eliminar borra el resumen de un archivo de disco y de la cache.
func (g *Generador) Eliminar(rutaRelativa string) error {
        g.mu.Lock()
        delete(g.cache, rutaRelativa)
        g.mu.Unlock()

        if g.dirResumen == "" {
                return nil
        }
        rutaArchivo := filepath.Join(g.dirResumen, rutaResumen(rutaRelativa))
        if err := os.Remove(rutaArchivo); err != nil && !os.IsNotExist(err) {
                return err
        }
        return nil
}

// ═══════════════════════════════════════════════════════
// GENERACIÓN
// ═══════════════════════════════════════════════════════

// Generar genera un resumen detallado de un archivo.
func (g *Generador) Generar(rutaRelativa, rutaAbsoluta string) (*ResumenArchivo, error) {
        contenido, err := os.ReadFile(rutaAbsoluta)
        if err != nil {
                return nil, fmt.Errorf("error leyendo archivo: %w", err)
        }

        lenguaje := detectarLenguaje(filepath.Ext(rutaRelativa))
        lineas := strings.Count(string(contenido), "\n")
        if len(contenido) > 0 && contenido[len(contenido)-1] != '\n' {
                lineas++
        }

        texto := string(contenido)

        resumen := &ResumenArchivo{
                Ruta:        rutaRelativa,
                Lenguaje:    lenguaje,
                Lineas:      lineas,
                TipoArchivo: TipoArchivo(rutaRelativa),
                Timestamp:   time.Now().UTC().Format(time.RFC3339),
        }

        // Extraer según lenguaje
        switch lenguaje {
        case "go":
                g.analizarGo(texto, resumen)
        case "python":
                g.analizarPython(texto, resumen)
        default:
                resumen.Descripcion = fmt.Sprintf("Archivo %s, %d líneas", lenguaje, lineas)
        }

        // Calcular complejidad
        resumen.Complejidad = calcularComplejidad(resumen)

        return resumen, nil
}

// GenerarLote genera resúmenes para múltiples archivos.
// Retorna un mapa ruta → resumen.
func (g *Generador) GenerarLote(archivos []struct{ Relativa, Absoluta string }) map[string]*ResumenArchivo {
        var mu sync.Mutex
        resultados := make(map[string]*ResumenArchivo)

        var wg sync.WaitGroup
        for _, arch := range archivos {
                wg.Add(1)
                go func(rel, abs string) {
                        defer wg.Done()
                        r, err := g.Generar(rel, abs)
                        if err != nil {
                                g.logFunc("error generando resumen de %s: %v", rel, err)
                                return
                        }
                        mu.Lock()
                        resultados[rel] = r
                        mu.Unlock()
                }(arch.Relativa, arch.Absoluta)
        }
        wg.Wait()

        return resultados
}

// ═══════════════════════════════════════════════════════
// ANÁLISIS POR LENGUAJE
// ═══════════════════════════════════════════════════════

// analizarGo extrae información de un archivo Go.
func (g *Generador) analizarGo(contenido string, r *ResumenArchivo) {
        lineas := strings.Split(contenido, "\n")

        // Extraer package
        var nombrePaquete string
        for _, linea := range lineas {
                trim := strings.TrimSpace(linea)
                if strings.HasPrefix(trim, "package ") {
                        nombrePaquete = strings.TrimPrefix(trim, "package ")
                        break
                }
        }

        // Extraer imports
        imports := make(map[string]bool)
        enImportBloque := false
        for _, linea := range lineas {
                trim := strings.TrimSpace(linea)
                if trim == "import (" {
                        enImportBloque = true
                        continue
                }
                if enImportBloque {
                        if trim == ")" {
                                enImportBloque = false
                                continue
                        }
                        imp := strings.Trim(trim, "\"\t")
                        imports[imp] = true
                } else if strings.HasPrefix(trim, "import ") {
                        imp := strings.Trim(strings.TrimPrefix(trim, "import "), "\"")
                        imports[imp] = true
                }
        }

        // Convertir imports a lista ordenada
        for imp := range imports {
                // Extraer solo la última parte del path (el paquete)
                partes := strings.Split(imp, "/")
                nombre := partes[len(partes)-1]
                if nombre != "" {
                        r.Importados = append(r.Importados, nombre)
                }
        }
        sort.Strings(r.Importados)

        // Extraer exportados (func/type/var/const con primera letra mayúscula)
        for _, linea := range lineas {
                trim := strings.TrimSpace(linea)
                if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "/*") {
                        continue
                }

                // Funciones y métodos
                if strings.HasPrefix(trim, "func ") {
                        nombre := extraerNombreExportado(strings.TrimPrefix(trim, "func "))
                        if nombre != "" && esExportado(nombre) {
                                r.Exportados = append(r.Exportados, nombre)
                        }
                }
                if strings.HasPrefix(trim, "func (") {
                        // Método: extraer el nombre después del tipo receptor
                        parts := strings.Fields(trim)
                        if len(parts) >= 3 {
                                nombre := extraerNombreExportado(parts[2])
                                if nombre != "" && esExportado(nombre) {
                                        r.Exportados = append(r.Exportados, nombre)
                                }
                        }
                }

                // Tipos
                if strings.HasPrefix(trim, "type ") {
                        partes := strings.Fields(trim)
                        if len(partes) >= 2 {
                                nombre := partes[1]
                                if esExportado(nombre) {
                                        r.Exportados = append(r.Exportados, nombre)
                                }
                        }
                }

                // Variables y constantes
                if strings.HasPrefix(trim, "var ") || strings.HasPrefix(trim, "const ") {
                        partes := strings.Fields(trim)
                        if len(partes) >= 2 {
                                nombre := partes[1]
                                // Quitar paréntesis de agrupación
                                nombre = strings.TrimRight(nombre, "(")
                                if esExportado(nombre) {
                                        r.Exportados = append(r.Exportados, nombre)
                                }
                        }
                }
        }

        // Eliminar duplicados
        r.Exportados = eliminarDuplicados(r.Exportados)

        // Generar descripción
        var partes []string
        if nombrePaquete != "" {
                partes = append(partes, fmt.Sprintf("paquete %s", nombrePaquete))
        }
        funcCount := 0
        typeCount := 0
        for _, linea := range lineas {
                trim := strings.TrimSpace(linea)
                if strings.HasPrefix(trim, "func ") || strings.HasPrefix(trim, "func (") {
                        funcCount++
                }
                if strings.HasPrefix(trim, "type ") {
                        typeCount++
                }
        }

        if funcCount > 0 {
                partes = append(partes, fmt.Sprintf("%d funciones", funcCount))
        }
        if typeCount > 0 {
                partes = append(partes, fmt.Sprintf("%d tipos", typeCount))
        }

        r.Descripcion = strings.Join(partes, ", ")
}

// analizarPython extrae información de un archivo Python.
func (g *Generador) analizarPython(contenido string, r *ResumenArchivo) {
        lineas := strings.Split(contenido, "\n")

        for _, linea := range lineas {
                trim := strings.TrimSpace(linea)

                // Imports
                if strings.HasPrefix(trim, "import ") {
                        partes := strings.Fields(trim)
                        if len(partes) >= 2 {
                                r.Importados = append(r.Importados, partes[1])
                        }
                }
                if strings.HasPrefix(trim, "from ") {
                        partes := strings.Fields(trim)
                        if len(partes) >= 4 {
                                r.Importados = append(r.Importados, partes[1])
                        }
                }

                // Clases
                if strings.HasPrefix(trim, "class ") {
                        partes := strings.Fields(trim)
                        if len(partes) >= 2 {
                                nombre := strings.TrimRight(partes[1], "(:")
                                r.Exportados = append(r.Exportados, nombre)
                        }
                }

                // Funciones
                if strings.HasPrefix(trim, "def ") {
                        partes := strings.Fields(trim)
                        if len(partes) >= 2 {
                                nombre := strings.TrimRight(partes[1], "(")
                                // Solo exportados (sin underscore inicial)
                                if !strings.HasPrefix(nombre, "_") {
                                        r.Exportados = append(r.Exportados, nombre)
                                }
                        }
                }
        }

        r.Importados = eliminarDuplicados(r.Importados)
        r.Exportados = eliminarDuplicados(r.Exportados)

        classCount, funcCount := 0, 0
        for _, linea := range lineas {
                trim := strings.TrimSpace(linea)
                if strings.HasPrefix(trim, "class ") {
                        classCount++
                }
                if strings.HasPrefix(trim, "def ") {
                        funcCount++
                }
        }

        r.Descripcion = fmt.Sprintf("Python, %d clases, %d funciones", classCount, funcCount)
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// esExportado retorna true si el identificador empieza con mayúscula (Go convention).
func esExportado(nombre string) bool {
        if len(nombre) == 0 {
                return false
        }
        return nombre[0] >= 'A' && nombre[0] <= 'Z'
}

// extraerNombreExportado extrae el nombre de una declaración Go.
func extraerNombreExportado(declaracion string) string {
        // Eliminar parámetros genéricos, receptores, etc.
        partes := strings.Fields(declaracion)
        if len(partes) == 0 {
                return ""
        }

        nombre := partes[0]
        // Quitar paréntesis finales
        if idx := strings.Index(nombre, "("); idx > 0 {
                nombre = nombre[:idx]
        }
        // Quitar corchetes de arrays/slices
        nombre = strings.TrimRight(nombre, "[]")
        // Solo identificadores válidos
        for i, c := range nombre {
                if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
                        if i == 0 {
                                return "" // primer char no es identificador
                        }
                        nombre = nombre[:i]
                        break
                }
        }
        return nombre
}

// eliminarDuplicados elimina duplicados manteniendo orden.
func eliminarDuplicados(items []string) []string {
        vistos := make(map[string]bool)
        var resultado []string
        for _, item := range items {
                if !vistos[item] {
                        vistos[item] = true
                        resultado = append(resultado, item)
                }
        }
        return resultado
}

// calcularComplejidad determina la complejidad de un archivo.
func calcularComplejidad(r *ResumenArchivo) string {
        exportados := len(r.Exportados)
        importados := len(r.Importados)

        puntuacion := exportados + importados
        if r.Lineas > 200 {
                puntuacion += 2
        } else if r.Lineas > 100 {
                puntuacion += 1
        }

        if puntuacion > 15 {
                return "alta"
        }
        if puntuacion > 7 {
                return "media"
        }
        return "baja"
}

// detectarLenguaje detecta lenguaje por extensión.
func detectarLenguaje(ext string) string {
        switch strings.ToLower(ext) {
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
        case ".html":
                return "html"
        case ".css", ".scss":
                return "css"
        case ".rs":
                return "rust"
        case ".toml":
                return "toml"
        default:
                return ""
        }
}