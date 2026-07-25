package indice

import (
        "crypto/sha256"
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

// EntradaIndice representa un archivo indexado con sus fragmentos.
type EntradaIndice struct {
        Ruta            string   `json:"ruta"`
        Lenguaje        string   `json:"lenguaje"`
        Lineas          int      `json:"lineas"`
        FragmentoIDs    []string `json:"fragmento_ids"`
        Resumen         string   `json:"resumen"`
        HashContenido   string   `json:"hash_contenido"`   // para detectar cambios
        UltimaActualizacion string `json:"ultima_actualizacion"`
}

// IndiceProyecto es el índice completo de un proyecto.
// Es el "árbol" que conecta el mapa con los fragmentos.
type IndiceProyecto struct {
        Version          string                  `json:"version"`
        Proyecto         string                  `json:"proyecto"`
        RutaAbsoluta     string                  `json:"ruta_absoluta"`
        Timestamp        string                  `json:"timestamp"`
        Archivos         map[string]EntradaIndice `json:"archivos"`
        TotalArchivos    int                     `json:"total_archivos"`
        TotalFragmentos  int                     `json:"total_fragmentos"`
        Lenguajes        map[string]int          `json:"lenguajes"` // lenguaje → cantidad
        UltimaReconstruccion string               `json:"ultima_reconstruccion"`
}

// OpcionesIndice configura el comportamiento del índice.
type OpcionesIndice struct {
        ExcluirExtensiones []string // extensiones a excluir del índice
        ExcluirDirs        []string // directorios a excluir
}

// OpcionesIndicePorDefecto retorna opciones por defecto.
func OpcionesIndicePorDefecto() OpcionesIndice {
        return OpcionesIndice{
                ExcluirExtensiones: []string{
                        ".log", ".tmp", ".bak", ".swp", ".swo",
                        ".min.js", ".min.css", ".map", ".lock",
                        ".sum", ".png", ".jpg", ".jpeg", ".gif",
                        ".ico", ".svg", ".woff", ".woff2", ".ttf",
                },
                ExcluirDirs: []string{
                        ".git", ".svn", ".hg", "node_modules", "vendor",
                        "__pycache__", ".pytest_cache", ".idea", ".vscode",
                        "dist", "build", "bin", ".next", "target", "go-local",
                },
        }
}

// GestorIndice gestiona el índice de un proyecto.
// El índice se reconstruye incrementalmente (nunca desde cero).
type GestorIndice struct {
        ruta      string // ruta del archivo indice_global.json
        mu        sync.RWMutex
        indice    *IndiceProyecto
        opciones  OpcionesIndice
        logFunc   func(string, ...interface{})
}

// NuevoGestor crea un nuevo gestor de índice.
// Si ya existe un índice en la ruta, lo carga.
func NuevoGestor(rutaArchivo string) (*GestorIndice, error) {
        g := &GestorIndice{
                ruta:     rutaArchivo,
                opciones: OpcionesIndicePorDefecto(),
                logFunc:  func(string, ...interface{}) {},
                indice: &IndiceProyecto{
                        Version:  "1.0",
                        Archivos: make(map[string]EntradaIndice),
                        Lenguajes: make(map[string]int),
                },
        }

        // Cargar existente
        if datos, err := os.ReadFile(rutaArchivo); err == nil {
                var existente IndiceProyecto
                if json.Unmarshal(datos, &existente) == nil && existente.Archivos != nil {
                        g.indice = &existente
                        g.logFunc("índice cargado: %d archivos, %d fragmentos",
                                existente.TotalArchivos, existente.TotalFragmentos)
                }
        }

        return g, nil
}

// ConLog asigna función de log.
func (g *GestorIndice) ConLog(fn func(string, ...interface{})) *GestorIndice {
        if fn != nil {
                g.logFunc = fn
        }
        return g
}

// ConOpciones asigna opciones personalizadas.
func (g *GestorIndice) ConOpciones(opts OpcionesIndice) *GestorIndice {
        g.opciones = opts
        return g
}

// ═══════════════════════════════════════════════════════
// RECONSTRUCCIÓN INCREMENTAL
// ═══════════════════════════════════════════════════════

// Reconstruir reconstruye el índice recorriendo el directorio del proyecto.
// Solo actualiza archivos que han cambiado (comparando hash).
// Nuevos archivos se agregan, archivos eliminados se marcan.
func (g *GestorIndice) Reconstruir(rutaProyecto string) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        g.logFunc("reconstruyendo índice incrementalmente: %s", rutaProyecto)

        rutaAbs, err := filepath.Abs(rutaProyecto)
        if err != nil {
                return fmt.Errorf("error resolviendo ruta: %w", err)
        }

        // Marcar todos los archivos existentes para detectar eliminados
        archivosVistos := make(map[string]bool)
        archivosModificados := 0
        archivosNuevos := 0
        archivosEliminados := 0

        err = filepath.WalkDir(rutaAbs, func(ruta string, d os.DirEntry, walkErr error) error {
                if walkErr != nil {
                        return nil
                }

                if d.IsDir() {
                        base := filepath.Base(ruta)
                        for _, excluir := range g.opciones.ExcluirDirs {
                                if base == excluir {
                                        return filepath.SkipDir
                                }
                        }
                        return nil
                }

                // Ignorar ocultos
                relativa, err := filepath.Rel(rutaAbs, ruta)
                if err != nil {
                        return nil
                }
                if relativa == "." || strings.HasPrefix(relativa, ".") {
                        return nil
                }

                // Verificar extensión
                ext := strings.ToLower(filepath.Ext(relativa))
                for _, excluir := range g.opciones.ExcluirExtensiones {
                        if ext == excluir {
                                return nil
                        }
                }

                archivosVistos[relativa] = true

                // Leer archivo y calcular hash
                contenido, err := os.ReadFile(ruta)
                if err != nil {
                        return nil
                }

                hashActual := hashContenido(contenido)
                lineas := strings.Count(string(contenido), "\n")
                if len(contenido) > 0 && contenido[len(contenido)-1] != '\n' {
                        lineas++
                }

                // Verificar si ya está indexado y sin cambios
                existente, existe := g.indice.Archivos[relativa]
                if existe && existente.HashContenido == hashActual {
                        return nil // sin cambios
                }

                // Crear o actualizar entrada
                lenguaje := detectarLenguajeIndice(ext)
                entrada := EntradaIndice{
                        Ruta:              relativa,
                        Lenguaje:          lenguaje,
                        Lineas:            lineas,
                        Resumen:           fmt.Sprintf("%s, %d líneas", lenguaje, lineas),
                        HashContenido:     hashActual,
                        UltimaActualizacion: time.Now().UTC().Format(time.RFC3339),
                }

                // Preservar fragmento IDs si el archivo ya existía
                if existe {
                        entrada.FragmentoIDs = existente.FragmentoIDs
                        archivosModificados++
                } else {
                        archivosNuevos++
                }

                g.indice.Archivos[relativa] = entrada
                return nil
        })

        if err != nil {
                return fmt.Errorf("error recorriendo directorio: %w", err)
        }

        // Detectar archivos eliminados
        for ruta := range g.indice.Archivos {
                if !archivosVistos[ruta] {
                        delete(g.indice.Archivos, ruta)
                        archivosEliminados++
                }
        }

        // Recalcular estadísticas
        g.recalcularEstadisticas(rutaAbs)

        g.indice.UltimaReconstruccion = time.Now().UTC().Format(time.RFC3339)

        // Persistir
        if err := g.guardar(); err != nil {
                return err
        }

        g.logFunc("índice reconstruido: +%d nuevos, ~%d modificados, -%d eliminados",
                archivosNuevos, archivosModificados, archivosEliminados)

        return nil
}

// ═══════════════════════════════════════════════════════
// ACTUALIZAR FRAGMENTOS
// ═══════════════════════════════════════════════════════

// AsignarFragmentos asigna IDs de fragmentos a un archivo en el índice.
func (g *GestorIndice) AsignarFragmentos(rutaRelativa string, ids []string) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        entrada, existe := g.indice.Archivos[rutaRelativa]
        if !existe {
                return fmt.Errorf("archivo %s no está en el índice", rutaRelativa)
        }

        entrada.FragmentoIDs = ids
        entrada.UltimaActualizacion = time.Now().UTC().Format(time.RFC3339)
        g.indice.Archivos[rutaRelativa] = entrada

        // Recalcular total de fragmentos
        g.indice.TotalFragmentos = 0
        for _, e := range g.indice.Archivos {
                g.indice.TotalFragmentos += len(e.FragmentoIDs)
        }

        return g.guardar()
}

// ═══════════════════════════════════════════════════════
// CONSULTAS
// ═══════════════════════════════════════════════════════

// Obtener retorna el índice completo (copia).
func (g *GestorIndice) Obtener() *IndiceProyecto {
        g.mu.RLock()
        defer g.mu.RUnlock()
        
        copia := *g.indice
        copia.Archivos = make(map[string]EntradaIndice)
        for k, v := range g.indice.Archivos {
                copia.Archivos[k] = v
        }
        copia.Lenguajes = make(map[string]int)
        for k, v := range g.indice.Lenguajes {
                copia.Lenguajes[k] = v
        }
        return &copia
}

// Buscar busca archivos en el índice por patrón (substring case-insensitive).
func (g *GestorIndice) Buscar(patron string) []EntradaIndice {
        g.mu.RLock()
        defer g.mu.RUnlock()

        patron = strings.ToLower(patron)
        var resultado []EntradaIndice

        for _, entrada := range g.indice.Archivos {
                if strings.Contains(strings.ToLower(entrada.Ruta), patron) ||
                        strings.Contains(strings.ToLower(entrada.Resumen), patron) {
                        resultado = append(resultado, entrada)
                }
        }

        sort.Slice(resultado, func(i, j int) bool {
                return resultado[i].Ruta < resultado[j].Ruta
        })

        return resultado
}

// ObtenerArchivo retorna la entrada de un archivo específico.
func (g *GestorIndice) ObtenerArchivo(ruta string) (*EntradaIndice, error) {
        g.mu.RLock()
        defer g.mu.RUnlock()

        entrada, existe := g.indice.Archivos[ruta]
        if !existe {
                return nil, fmt.Errorf("archivo %s no encontrado en el índice", ruta)
        }
        return &entrada, nil
}

// ObtenerPorLenguaje retorna todas las entradas de un lenguaje.
func (g *GestorIndice) ObtenerPorLenguaje(lenguaje string) []EntradaIndice {
        g.mu.RLock()
        defer g.mu.RUnlock()

        var resultado []EntradaIndice
        for _, entrada := range g.indice.Archivos {
                if entrada.Lenguaje == lenguaje {
                        resultado = append(resultado, entrada)
                }
        }

        sort.Slice(resultado, func(i, j int) bool {
                return resultado[i].Ruta < resultado[j].Ruta
        })

        return resultado
}

// ObtenerFragmentosIDs retorna los IDs de fragmentos de un archivo.
func (g *GestorIndice) ObtenerFragmentosIDs(ruta string) []string {
        g.mu.RLock()
        defer g.mu.RUnlock()

        entrada, existe := g.indice.Archivos[ruta]
        if !existe {
                return nil
        }
        return entrada.FragmentoIDs
}

// ArchivosModificados retorna los archivos que han cambiado desde la última indexación.
// Compara hashes contra el filesystem actual.
func (g *GestorIndice) ArchivosModificados(rutaProyecto string) ([]string, error) {
        g.mu.RLock()
        defer g.mu.RUnlock()

        rutaAbs, _ := filepath.Abs(rutaProyecto)
        var modificados []string

        for ruta, entrada := range g.indice.Archivos {
                rutaCompleta := filepath.Join(rutaAbs, ruta)
                contenido, err := os.ReadFile(rutaCompleta)
                if err != nil {
                        // Archivo eliminado
                        modificados = append(modificados, ruta)
                        continue
                }

                hashActual := hashContenido(contenido)
                if hashActual != entrada.HashContenido {
                        modificados = append(modificados, ruta)
                }
        }

        return modificados, nil
}

// ═══════════════════════════════════════════════════════
// PERSISTENCIA
// ═══════════════════════════════════════════════════════

// guardar persiste el índice a JSON.
func (g *GestorIndice) guardar() error {
        if err := os.MkdirAll(filepath.Dir(g.ruta), 0755); err != nil {
                return err
        }

        datos, err := json.MarshalIndent(g.indice, "", "  ")
        if err != nil {
                return fmt.Errorf("error serializando índice: %w", err)
        }

        if err := os.WriteFile(g.ruta, datos, 0644); err != nil {
                return fmt.Errorf("error guardando índice: %w", err)
        }

        return nil
}

// GuardarEn guarda el índice en una ruta específica (para testing).
func (g *GestorIndice) GuardarEn(ruta string) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        rutaAnterior := g.ruta
        g.ruta = ruta
        err := g.guardar()
        g.ruta = rutaAnterior
        return err
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// recalcularEstadisticas actualiza los contadores del índice.
func (g *GestorIndice) recalcularEstadisticas(rutaAbs string) {
        g.indice.TotalArchivos = len(g.indice.Archivos)
        g.indice.TotalFragmentos = 0
        g.indice.Lenguajes = make(map[string]int)
        g.indice.RutaAbsoluta = rutaAbs
        g.indice.Proyecto = filepath.Base(rutaAbs)
        g.indice.Timestamp = time.Now().UTC().Format(time.RFC3339)

        for _, entrada := range g.indice.Archivos {
                g.indice.TotalFragmentos += len(entrada.FragmentoIDs)
                if entrada.Lenguaje != "" {
                        g.indice.Lenguajes[entrada.Lenguaje]++
                }
        }
}

// hashContenido genera un hash SHA256 del contenido.
func hashContenido(contenido []byte) string {
        h := sha256.Sum256(contenido)
        return fmt.Sprintf("%x", h[:8]) // primeros 8 bytes = 16 chars hex
}

// detectarLenguajeIndice detecta lenguaje por extensión.
func detectarLenguajeIndice(ext string) string {
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
        case ".html":
                return "html"
        case ".css", ".scss":
                return "css"
        case ".rs":
                return "rust"
        case ".toml":
                return "toml"
        default:
                if ext == "" {
                        return ""
                }
                return strings.TrimPrefix(ext, ".")
        }
}