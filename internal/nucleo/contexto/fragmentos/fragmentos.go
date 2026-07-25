package fragmentos

import (
        "crypto/sha256"
        "encoding/hex"
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

// Fragmento es un trozo inmutable de contenido de un archivo.
// Los fragmentos NUNCA se editan, solo se agregan nuevos.
// Esto es por decisión de diseño: el contexto crece acumulativamente.
type Fragmento struct {
        ID        string    `json:"id"`         // hash SHA256 del contenido
        Ruta      string    `json:"ruta"`       // ruta relativa del archivo origen
        LineaIni  int       `json:"linea_ini"`  // línea de inicio (1-indexed)
        LineaFin  int       `json:"linea_fin"`  // línea de fin (inclusive)
        Tipo      string    `json:"tipo"`       // "funcion", "estructura", "import", "config", "completo", etc.
        Lenguaje  string    `json:"lenguaje"`
        Contenido string    `json:"contenido"`  // el texto del fragmento
        Resumen   string    `json:"resumen"`    // resumen de una línea del fragmento
        Timestamp string    `json:"timestamp"`  // cuándo se creó
        Tamanio   int       `json:"tamanio"`    // bytes del contenido
}

// MetadataArchivo contiene la metadata de fragmentos de un archivo.
type MetadataArchivo struct {
        Ruta            string   `json:"ruta"`
        TotalFragmentos int      `json:"total_fragmentos"`
        IDs             []string `json:"ids"`
        UltimaActualizacion string `json:"ultima_actualizacion"`
}

// Almacen es el almacenamiento de fragmentos del sistema de contexto.
// Persiste fragmentos como archivos JSON individuales (uno por fragmento).
type Almacen struct {
        directorio string // ~/.liz/contexto/proyectos/<proyecto>/archivos/
        mu         sync.RWMutex
        logFunc    func(string, ...interface{})
}

// NuevoAlmacen crea un nuevo almacén de fragmentos para un proyecto.
func NuevoAlmacen(dirBase string, nombreProyecto string) (*Almacen, error) {
        dir := filepath.Join(dirBase, nombreProyecto, ".liz", "archivos")
        
        if err := os.MkdirAll(dir, 0755); err != nil {
                return nil, fmt.Errorf("error creando directorio de fragmentos: %w", err)
        }

        return &Almacen{
                directorio: dir,
                logFunc:    func(string, ...interface{}) {},
        }, nil
}

// ConLog asigna una función de log.
func (a *Almacen) ConLog(fn func(string, ...interface{})) *Almacen {
        if fn != nil {
                a.logFunc = fn
        }
        return a
}

// Directorio retorna la ruta base del almacén.
func (a *Almacen) Directorio() string {
        return a.directorio
}

// ═══════════════════════════════════════════════════════
// CREACIÓN DE FRAGMENTOS
// ═══════════════════════════════════════════════════════

// Agregar agrega un nuevo fragmento al almacén.
// Si ya existe un fragmento con el mismo ID (mismo contenido), no se duplica.
// Retorna el ID del fragmento.
func (a *Almacen) Agregar(ruta, contenido, tipo, lenguaje string, lineaIni, lineaFin int) (string, error) {
        a.mu.Lock()
        defer a.mu.Unlock()

        // Generar ID basado en contenido
        id := generarID(ruta, contenido, lineaIni, lineaFin)

        // Verificar si ya existe
        rutaFragmento := filepath.Join(a.directorio, id+".json")
        if _, err := os.Stat(rutaFragmento); err == nil {
                return id, nil // ya existe
        }

        // Generar resumen
        resumen := generarResumen(contenido, tipo)

        fragmento := Fragmento{
                ID:        id,
                Ruta:      ruta,
                LineaIni:  lineaIni,
                LineaFin:  lineaFin,
                Tipo:      tipo,
                Lenguaje:  lenguaje,
                Contenido: contenido,
                Resumen:   resumen,
                Timestamp: time.Now().UTC().Format(time.RFC3339),
                Tamanio:   len(contenido),
        }

        datos, err := json.MarshalIndent(fragmento, "", "  ")
        if err != nil {
                return "", fmt.Errorf("error serializando fragmento: %w", err)
        }

        if err := os.WriteFile(rutaFragmento, datos, 0644); err != nil {
                return "", fmt.Errorf("error guardando fragmento: %w", err)
        }

        a.logFunc("fragmento agregado: %s (%s, líneas %d-%d)", id, ruta, lineaIni, lineaFin)
        return id, nil
}

// AgregarArchivoCompleto agrega un archivo completo como un solo fragmento.
func (a *Almacen) AgregarArchivoCompleto(rutaRelativa, contenido, lenguaje string) (string, error) {
        lineas := strings.Count(contenido, "\n")
        if len(contenido) > 0 && contenido[len(contenido)-1] != '\n' {
                lineas++
        }
        return a.Agregar(rutaRelativa, contenido, "completo", lenguaje, 1, lineas)
}

// AgregarDesdeArchivo lee un archivo del filesystem y lo fragmenta.
// El fragmentado es inteligente: para Go, fragmenta por funciones/tipos.
// Para otros lenguajes, usa el archivo completo.
func (a *Almacen) AgregarDesdeArchivo(rutaRelativa, rutaAbsoluta string) ([]string, error) {
        contenido, err := os.ReadFile(rutaAbsoluta)
        if err != nil {
                return nil, fmt.Errorf("error leyendo archivo: %w", err)
        }

        est := filepath.Ext(rutaRelativa)
        lenguaje := detectarLenguajeExt(est)

        // Fragmentar inteligentemente según lenguaje
        fragmentos := fragmentarContenido(string(contenido), lenguaje)

        var ids []string
        for _, frag := range fragmentos {
                id, err := a.Agregar(rutaRelativa, frag.contenido, frag.tipo, lenguaje, frag.lineaIni, frag.lineaFin)
                if err != nil {
                        a.logFunc("error agregando fragmento de %s: %v", rutaRelativa, err)
                        continue
                }
                ids = append(ids, id)
        }

        // Si no se generaron fragmentos inteligentes, agregar completo
        if len(ids) == 0 {
                id, err := a.AgregarArchivoCompleto(rutaRelativa, string(contenido), lenguaje)
                if err != nil {
                        return nil, err
                }
                ids = []string{id}
        }

        return ids, nil
}

// ═══════════════════════════════════════════════════════
// LECTURA DE FRAGMENTOS
// ═══════════════════════════════════════════════════════

// Obtener lee un fragmento por su ID.
func (a *Almacen) Obtener(id string) (*Fragmento, error) {
        a.mu.RLock()
        defer a.mu.RUnlock()

        rutaFragmento := filepath.Join(a.directorio, id+".json")
        datos, err := os.ReadFile(rutaFragmento)
        if err != nil {
                return nil, fmt.Errorf("fragmento %s no encontrado: %w", id, err)
        }

        var frag Fragmento
        if err := json.Unmarshal(datos, &frag); err != nil {
                return nil, fmt.Errorf("error parseando fragmento: %w", err)
        }

        return &frag, nil
}

// ObtenerPorRuta retorna todos los fragmentos de una ruta específica.
func (a *Almacen) ObtenerPorRuta(ruta string) ([]Fragmento, error) {
        a.mu.RLock()
        defer a.mu.RUnlock()

        // Listar todos los fragmentos y filtrar por ruta
        entradas, err := os.ReadDir(a.directorio)
        if err != nil {
                return nil, fmt.Errorf("error leyendo directorio de fragmentos: %w", err)
        }

        var resultado []Fragmento
        for _, entrada := range entradas {
                if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".json") {
                        continue
                }

                rutaFrag := filepath.Join(a.directorio, entrada.Name())
                datos, err := os.ReadFile(rutaFrag)
                if err != nil {
                        continue
                }

                var frag Fragmento
                if json.Unmarshal(datos, &frag) != nil {
                        continue
                }

                if frag.Ruta == ruta {
                        resultado = append(resultado, frag)
                }
        }

        // Ordenar por línea de inicio
        sort.Slice(resultado, func(i, j int) bool {
                return resultado[i].LineaIni < resultado[j].LineaIni
        })

        return resultado, nil
}

// Listar retorna metadata de todos los fragmentos (sin el contenido completo).
func (a *Almacen) Listar() ([]Fragmento, error) {
        a.mu.RLock()
        defer a.mu.RUnlock()

        entradas, err := os.ReadDir(a.directorio)
        if err != nil {
                return nil, fmt.Errorf("error leyendo directorio: %w", err)
        }

        var resultado []Fragmento
        for _, entrada := range entradas {
                if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".json") {
                        continue
                }

                rutaFrag := filepath.Join(a.directorio, entrada.Name())
                datos, err := os.ReadFile(rutaFrag)
                if err != nil {
                        continue
                }

                var frag Fragmento
                if json.Unmarshal(datos, &frag) != nil {
                        continue
                }

                // No incluir el contenido completo en la lista
                frag.Contenido = ""
                resultado = append(resultado, frag)
        }

        sort.Slice(resultado, func(i, j int) bool {
                if resultado[i].Ruta != resultado[j].Ruta {
                        return resultado[i].Ruta < resultado[j].Ruta
                }
                return resultado[i].LineaIni < resultado[j].LineaIni
        })

        return resultado, nil
}

// Total retorna el número de fragmentos almacenados.
func (a *Almacen) Total() int {
        a.mu.RLock()
        defer a.mu.RUnlock()

        entradas, err := os.ReadDir(a.directorio)
        if err != nil {
                return 0
        }

        count := 0
        for _, e := range entradas {
                if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
                        count++
                }
        }
        return count
}

// EliminarPorRuta elimina todos los fragmentos asociados a una ruta.
// Retorna los IDs eliminados.
func (a *Almacen) EliminarPorRuta(ruta string) ([]string, error) {
        a.mu.Lock()
        defer a.mu.Unlock()

        fragmentos, err := a.listarInterno()
        if err != nil {
                return nil, err
        }

        var eliminados []string
        for _, frag := range fragmentos {
                if frag.Ruta == ruta {
                        rutaFrag := filepath.Join(a.directorio, frag.ID+".json")
                        if err := os.Remove(rutaFrag); err != nil {
                                a.logFunc("error eliminando fragmento %s: %v", frag.ID, err)
                                continue
                        }
                        eliminados = append(eliminados, frag.ID)
                }
        }

        return eliminados, nil
}

// listarInterno lista todos los fragmentos (debe llamarse con lock).
func (a *Almacen) listarInterno() ([]Fragmento, error) {
        entradas, err := os.ReadDir(a.directorio)
        if err != nil {
                return nil, err
        }

        var resultado []Fragmento
        for _, entrada := range entradas {
                if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".json") {
                        continue
                }

                datos, err := os.ReadFile(filepath.Join(a.directorio, entrada.Name()))
                if err != nil {
                        continue
                }

                var frag Fragmento
                if json.Unmarshal(datos, &frag) == nil {
                        resultado = append(resultado, frag)
                }
        }
        return resultado, nil
}

// ═══════════════════════════════════════════════════════
// FRAGMENTADO INTELIGENTE POR LENGUAJE
// ═══════════════════════════════════════════════════════

// fragmentoInterno es un fragmento antes de persistirse.
type fragmentoInterno struct {
        contenido string
        tipo      string
        lineaIni  int
        lineaFin  int
}

// fragmentarContenido divide el contenido en fragmentos según el lenguaje.
func fragmentarContenido(contenido, lenguaje string) []fragmentoInterno {
        switch lenguaje {
        case "go":
                return fragmentarGo(contenido)
        default:
                return nil // sin fragmentación inteligente, se usa completo
        }
}

// fragmentarGo divide código Go en fragmentos por funciones, tipos y variables.
func fragmentarGo(contenido string) []fragmentoInterno {
        var resultado []fragmentoInterno
        lineas := strings.Split(contenido, "\n")

        var (
                bloqueActual  strings.Builder
                lineaInicio   int
                tipoBloque    string
        )

        empezarBloque := func(tipo string, lineaNum int) {
                // Guardar bloque anterior si tiene contenido
                if bloqueActual.Len() > 0 && tipoBloque != "" {
                        texto := strings.TrimRight(bloqueActual.String(), "\n")
                        resultado = append(resultado, fragmentoInterno{
                                contenido: texto,
                                tipo:      tipoBloque,
                                lineaIni:  lineaInicio,
                                lineaFin:  lineaNum - 1,
                        })
                }
                bloqueActual.Reset()
                lineaInicio = lineaNum
                tipoBloque = tipo
        }

        for i, linea := range lineas {
                numLinea := i + 1
                trim := strings.TrimSpace(linea)

                // Detectar inicio de funciones
                if (strings.HasPrefix(trim, "func ") || strings.HasPrefix(trim, "func (")) && !strings.HasSuffix(trim, "{") {
                        empezarBloque("funcion", numLinea)
                }

                // Detectar inicio de tipos
                if strings.HasPrefix(trim, "type ") {
                        empezarBloque("estructura", numLinea)
                }

                bloqueActual.WriteString(linea)
                bloqueActual.WriteString("\n")
        }

        // Último bloque
        if bloqueActual.Len() > 0 && tipoBloque != "" {
                texto := strings.TrimRight(bloqueActual.String(), "\n")
                resultado = append(resultado, fragmentoInterno{
                        contenido: texto,
                        tipo:      tipoBloque,
                        lineaIni:  lineaInicio,
                        lineaFin:  len(lineas),
                })
        }

        return resultado
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// generarID crea un hash SHA256 único para un fragmento.
func generarID(ruta, contenido string, lineaIni, lineaFin int) string {
        data := fmt.Sprintf("%s::%d-%d::%s", ruta, lineaIni, lineaFin, contenido)
        hash := sha256.Sum256([]byte(data))
        return hex.EncodeToString(hash[:16]) // primeros 16 bytes = 32 chars hex
}

// generarResumen crea un resumen de una línea para el fragmento.
func generarResumen(contenido, tipo string) string {
        primeras := strings.SplitN(contenido, "\n", 4)
        if len(primeras) == 0 {
                return fmt.Sprintf("(%s)", tipo)
        }

        // Buscar la primera línea no vacía y no de comentario
        for _, linea := range primeras {
                trim := strings.TrimSpace(linea)
                if trim == "" {
                        continue
                }
                // Saltar comentarios simples
                if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "#") {
                        continue
                }
                if len(trim) > 100 {
                        trim = trim[:100] + "..."
                }
                return trim
        }

        return fmt.Sprintf("(%s, %d líneas)", tipo, strings.Count(contenido, "\n"))
}

// detectarLenguajeExt detecta el lenguaje por extensión.
func detectarLenguajeExt(ext string) string {
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
        case ".html", ".htm":
                return "html"
        case ".css", ".scss":
                return "css"
        case ".rs":
                return "rust"
        case ".java":
                return "java"
        case ".c", ".h":
                return "c"
        case ".cpp", ".hpp":
                return "cpp"
        case ".sh":
                return "shell"
        case ".toml":
                return "toml"
        default:
                return ""
        }
}
