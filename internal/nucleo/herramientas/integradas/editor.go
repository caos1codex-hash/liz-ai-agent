package integradas

import (
        "context"
        "fmt"
        "io/ioutil"
        "os"
        "path/filepath"
        "regexp"
        "strings"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Editor — Lectura/Escritura/Modificación de Archivos
// ============================================================================

// MaxTamanoEditor es el tamaño máximo de archivo que el Editor maneja (10MB).
const MaxTamanoEditor = 10 * 1024 * 1024

// Editor permite leer, escribir, agregar y parchear archivos.
// También soporta operaciones de directorio (crear, mover, copiar).
type Editor struct{}

// NewEditor crea una instancia.
func NewEditor() *Editor { return &Editor{} }

var _ herramientas.Herramienta = (*Editor)(nil)

func (e *Editor) Nombre() string { return "editor" }

func (e *Editor) Descripcion() string {
        return "Lee, escribe, modifica y parchea archivos. Operaciones: leer, " +
                "escribir, agregar, insertar, reemplazar, parchear, eliminar, " +
                "crear_directorio, mover, copiar. Soporta backups automáticos."
}

func (e *Editor) Parametros() []herramientas.Parametro {
        return []herramientas.Parametro{
                {
                        Nombre:      "operacion",
                        Tipo:        "string",
                        Requerido:   true,
                        Opciones: []string{
                                "leer", "escribir", "agregar", "insertar",
                                "reemplazar", "parchear", "eliminar",
                                "crear_directorio", "mover", "copiar",
                        },
                        Descripcion: "Operación a realizar.",
                },
                {
                        Nombre:      "ruta",
                        Tipo:        "string",
                        Requerido:   true,
                        Descripcion: "Ruta del archivo o directorio.",
                },
                {
                        Nombre:      "contenido",
                        Tipo:        "string",
                        Descripcion: "Contenido a escribir/agregar/insertar. Requerido para escribir, agregar, insertar.",
                },
                {
                        Nombre:      "linea",
                        Tipo:        "int",
                        Min:         float64Ptr(1),
                        Descripcion: "Número de línea (1-indexed) para insertar. Requerido para 'insertar'.",
                },
                {
                        Nombre:      "buscar",
                        Tipo:        "string",
                        Descripcion: "Texto a buscar (literal). Requerido para 'reemplazar' y 'parchear'.",
                },
                {
                        Nombre:      "reemplazar_con",
                        Tipo:        "string",
                        Descripcion: "Texto de reemplazo. Requerido para 'reemplazar' y 'parchear'.",
                },
                {
                        Nombre:      "regex",
                        Tipo:        "bool",
                        Default:     false,
                        Descripcion: "Si true, 'buscar' se interpreta como regex (Go syntax).",
                },
                {
                        Nombre:      "todas",
                        Tipo:        "bool",
                        Default:     true,
                        Descripcion: "Si true, reemplaza todas las ocurrencias. Si false, solo la primera.",
                },
                {
                        Nombre:      "destino",
                        Tipo:        "string",
                        Descripcion: "Ruta destino para 'mover' y 'copiar'.",
                },
                {
                        Nombre:      "backup",
                        Tipo:        "bool",
                        Default:     false,
                        Descripcion: "Si true, crea backup .bak antes de modificar archivo existente.",
                },
                {
                        Nombre:      "crear_dirs",
                        Tipo:        "bool",
                        Default:     true,
                        Descripcion: "Si true, crea directorios padres inexistentes al escribir.",
                },
                {
                        Nombre:      "permiso",
                        Tipo:        "string",
                        Default:     "0644",
                        Descripcion: "Permisos Unix para archivos creados (formato octal string).",
                },
                {
                        Nombre:      "max_lineas",
                        Tipo:        "int",
                        Default:     10000,
                        Min:         float64Ptr(1),
                        Max:         float64Ptr(1000000),
                        Descripcion: "Máximo de líneas a leer (solo operación 'leer').",
                },
        }
}

// ResultadoEditor es el Datos de Editor.
type ResultadoEditor struct {
        Operacion       string `json:"operacion"`
        Ruta            string `json:"ruta"`
        Contenido       string `json:"contenido,omitempty"`
        Bytes           int64  `json:"bytes,omitempty"`
        Lineas          int    `json:"lineas,omitempty"`
        Reemplazos      int    `json:"reemplazos,omitempty"`
        Destino         string `json:"destino,omitempty"`
        Backup          string `json:"backup,omitempty"`
        Truncado        bool   `json:"truncado,omitempty"`
}

func (e *Editor) Validar() error { return nil }

func (e *Editor) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
        operacion, err := herramientas.ObtenerString(params, e.paramByName("operacion"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        ruta, err := herramientas.ObtenerString(params, e.paramByName("ruta"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        rutaAbs, err := filepath.Abs(ruta)
        if err != nil {
                return herramientas.Resultado{Exito: false,
                        Error: fmt.Sprintf("ruta inválida: %v", err)}, nil
        }

        switch operacion {
        case "leer":
                return e.opLeer(params, rutaAbs)
        case "escribir":
                return e.opEscribir(params, rutaAbs)
        case "agregar":
                return e.opAgregar(params, rutaAbs)
        case "insertar":
                return e.opInsertar(params, rutaAbs)
        case "reemplazar":
                return e.opReemplazar(params, rutaAbs, false)
        case "parchear":
                return e.opReemplazar(params, rutaAbs, true)
        case "eliminar":
                return e.opEliminar(rutaAbs)
        case "crear_directorio":
                return e.opCrearDirectorio(params, rutaAbs)
        case "mover":
                return e.opMover(params, rutaAbs)
        case "copiar":
                return e.opCopiar(params, rutaAbs)
        default:
                return herramientas.Resultado{Exito: false,
                        Error: fmt.Sprintf("operación '%s' no soportada", operacion)}, nil
        }
}

// opLeer lee el contenido de un archivo.
func (e *Editor) opLeer(params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
        info, err := os.Stat(rutaAbs)
        if err != nil {
                if os.IsNotExist(err) {
                        return herramientas.Resultado{Exito: false,
                                Error: fmt.Sprintf("archivo no existe: %s", rutaAbs)}, nil
                }
                return herramientas.Resultado{Exito: false,
                        Error: fmt.Sprintf("stat: %v", err)}, nil
        }
        if info.IsDir() {
                return herramientas.Resultado{Exito: false,
                        Error: "no se puede leer un directorio con 'leer' (usar 'navegador_archivos')"}, nil
        }
        if info.Size() > MaxTamanoEditor {
                return herramientas.Resultado{Exito: false,
                        Error: fmt.Sprintf("archivo muy grande: %d bytes (max %d)", info.Size(), MaxTamanoEditor)}, nil
        }

        data, err := ioutil.ReadFile(rutaAbs)
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        maxLineas, _ := herramientas.ObtenerInt(params, e.paramByName("max_lineas"))
        if maxLineas <= 0 {
                maxLineas = 10000
        }

        contenido := string(data)
        lineas := strings.Split(contenido, "\n")
        truncado := false
        if len(lineas) > maxLineas {
                lineas = lineas[:maxLineas]
                contenido = strings.Join(lineas, "\n")
                truncado = true
        }

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "leer",
                        Ruta:      rutaAbs,
                        Contenido: contenido,
                        Bytes:     info.Size(),
                        Lineas:    len(lineas),
                        Truncado:  truncado,
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// opEscribir crea o sobrescribe un archivo.
func (e *Editor) opEscribir(params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
        contenido, err := herramientas.ObtenerString(params, e.paramByName("contenido"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        crearDirs, _ := herramientas.ObtenerBool(params, e.paramByName("crear_dirs"))
        backup, _ := herramientas.ObtenerBool(params, e.paramByName("backup"))
        permisoStr, _ := herramientas.ObtenerString(params, e.paramByName("permiso"))

        permiso := os.FileMode(0644)
        if permisoStr != "" {
                var p uint32
                if _, err := fmt.Sscanf(permisoStr, "%o", &p); err == nil {
                        permiso = os.FileMode(p)
                }
        }

        // Backup si el archivo existe
        backupPath := ""
        if backup {
                if _, err := os.Stat(rutaAbs); err == nil {
                        backupPath = rutaAbs + ".bak"
                        if err := copiarArchivo(rutaAbs, backupPath); err != nil {
                                return herramientas.Resultado{Exito: false,
                                        Error: fmt.Sprintf("backup falló: %v", err)}, nil
                        }
                }
        }

        // Crear directorios padres
        if crearDirs {
                dir := filepath.Dir(rutaAbs)
                if err := os.MkdirAll(dir, 0755); err != nil {
                        return herramientas.Resultado{Exito: false,
                                Error: fmt.Sprintf("mkdir: %v", err)}, nil
                }
        }

        if err := ioutil.WriteFile(rutaAbs, []byte(contenido), permiso); err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        lineas := strings.Count(contenido, "\n") + 1

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "escribir",
                        Ruta:      rutaAbs,
                        Bytes:     int64(len(contenido)),
                        Lineas:    lineas,
                        Backup:    backupPath,
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// opAgregar añade contenido al final del archivo (lo crea si no existe).
func (e *Editor) opAgregar(params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
        contenido, err := herramientas.ObtenerString(params, e.paramByName("contenido"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        crearDirs, _ := herramientas.ObtenerBool(params, e.paramByName("crear_dirs"))

        if crearDirs {
                dir := filepath.Dir(rutaAbs)
                if err := os.MkdirAll(dir, 0755); err != nil {
                        return herramientas.Resultado{Exito: false,
                                Error: fmt.Sprintf("mkdir: %v", err)}, nil
                }
        }

        // Abrir en modo append, crear si no existe
        f, err := os.OpenFile(rutaAbs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        defer f.Close()

        // Si el archivo no está vacío y el contenido no empieza con newline, agregar uno
        if info, _ := f.Stat(); info != nil && info.Size() > 0 && !strings.HasPrefix(contenido, "\n") {
                if _, err := f.WriteString("\n"); err != nil {
                        return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
                }
        }

        n, err := f.WriteString(contenido)
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        lineas := strings.Count(contenido, "\n") + 1

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "agregar",
                        Ruta:      rutaAbs,
                        Bytes:     int64(n),
                        Lineas:    lineas,
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// opInsertar inserta contenido en una línea específica (desplaza las demás).
func (e *Editor) opInsertar(params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
        contenido, err := herramientas.ObtenerString(params, e.paramByName("contenido"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        linea, err := herramientas.ObtenerInt(params, e.paramByName("linea"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        if linea < 1 {
                return herramientas.Resultado{Exito: false,
                        Error: "línea debe ser >= 1"}, nil
        }

        // Leer archivo existente (puede no existir)
        data, err := ioutil.ReadFile(rutaAbs)
        if err != nil && !os.IsNotExist(err) {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        lineas := []string{}
        if len(data) > 0 {
                lineas = strings.Split(string(data), "\n")
        }

        // Insertar
        idx := linea - 1
        if idx > len(lineas) {
                idx = len(lineas)
        }
        nuevasLineas := strings.Split(contenido, "\n")
        // Expandir slice
        lineas = append(lineas, "")
        copy(lineas[idx+len(nuevasLineas):], lineas[idx:])
        copy(lineas[idx:], nuevasLineas)

        nuevoContenido := strings.Join(lineas, "\n")
        if err := ioutil.WriteFile(rutaAbs, []byte(nuevoContenido), 0644); err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "insertar",
                        Ruta:      rutaAbs,
                        Bytes:     int64(len(nuevoContenido)),
                        Lineas:    len(lineas),
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// opReemplazar busca y reemplaza texto en un archivo.
// parchear = true permite regex y tiene semántica más estricta (un match exacto esperado).
func (e *Editor) opReemplazar(params map[string]interface{}, rutaAbs string,
        parchear bool) (herramientas.Resultado, error) {

        buscar, err := herramientas.ObtenerString(params, e.paramByName("buscar"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        reemplazo, err := herramientas.ObtenerString(params, e.paramByName("reemplazar_con"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        regex, _ := herramientas.ObtenerBool(params, e.paramByName("regex"))
        todas, _ := herramientas.ObtenerBool(params, e.paramByName("todas"))

        data, err := ioutil.ReadFile(rutaAbs)
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        contenido := string(data)
        var nuevoContenido string
        reemplazos := 0

        if regex {
                // Regex
                r, err := regexp.Compile(buscar)
                if err != nil {
                        return herramientas.Resultado{Exito: false,
                                Error: fmt.Sprintf("regex inválido: %v", err)}, nil
                }
                if todas {
                        count := len(r.FindAllStringIndex(contenido, -1))
                        nuevoContenido = r.ReplaceAllString(contenido, reemplazo)
                        reemplazos = count
                } else {
                        loc := r.FindStringIndex(contenido)
                        if loc != nil {
                                nuevoContenido = contenido[:loc[0]] + reemplazo + contenido[loc[1]:]
                                reemplazos = 1
                        } else {
                                nuevoContenido = contenido
                        }
                }
        } else {
                // Literal
                if todas {
                        nuevoContenido = strings.ReplaceAll(contenido, buscar, reemplazo)
                        reemplazos = strings.Count(contenido, buscar)
                } else {
                        nuevoContenido = strings.Replace(contenido, buscar, reemplazo, 1)
                        if strings.Contains(contenido, buscar) {
                                reemplazos = 1
                        }
                }
        }

        // Para parchear: si no se encontró el patrón, fallar
        if parchear && reemplazos == 0 {
                return herramientas.Resultado{Exito: false,
                        Error: fmt.Sprintf("patrón no encontrado: %s", buscar)}, nil
        }

        if nuevoContenido == contenido {
                // Sin cambios
                return herramientas.Resultado{
                        Exito: true,
                        Datos: ResultadoEditor{
                                Operacion:  "reemplazar",
                                Ruta:       rutaAbs,
                                Reemplazos: 0,
                        },
                        Metadata: herramientas.NuevaMetadata(0),
                }, nil
        }

        // Backup
        backup, _ := herramientas.ObtenerBool(params, e.paramByName("backup"))
        backupPath := ""
        if backup {
                backupPath = rutaAbs + ".bak"
                if err := copiarArchivo(rutaAbs, backupPath); err != nil {
                        return herramientas.Resultado{Exito: false,
                                Error: fmt.Sprintf("backup falló: %v", err)}, nil
                }
        }

        if err := ioutil.WriteFile(rutaAbs, []byte(nuevoContenido), 0644); err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion:  "reemplazar",
                        Ruta:       rutaAbs,
                        Reemplazos: reemplazos,
                        Backup:     backupPath,
                        Bytes:      int64(len(nuevoContenido)),
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// opEliminar elimina un archivo o directorio (recursivo).
func (e *Editor) opEliminar(rutaAbs string) (herramientas.Resultado, error) {
        info, err := os.Stat(rutaAbs)
        if err != nil {
                if os.IsNotExist(err) {
                        return herramientas.Resultado{Exito: false,
                                Error: "archivo no existe"}, nil
                }
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        if err := os.RemoveAll(rutaAbs); err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        tipo := "archivo"
        if info.IsDir() {
                tipo = "directorio"
        }

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "eliminar",
                        Ruta:      rutaAbs,
                        Bytes:     info.Size(),
                },
                Metadata: map[string]interface{}{
                        "duracion_ms": float64(0),
                        "tipo":        tipo,
                },
        }, nil
}

// opCrearDirectorio crea un directorio (con padres).
func (e *Editor) opCrearDirectorio(params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
        if err := os.MkdirAll(rutaAbs, 0755); err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "crear_directorio",
                        Ruta:      rutaAbs,
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// opMover mueve un archivo/directorio.
func (e *Editor) opMover(params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
        destino, err := herramientas.ObtenerString(params, e.paramByName("destino"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        if destino == "" {
                return herramientas.Resultado{Exito: false,
                        Error: "parámetro 'destino' requerido"}, nil
        }
        destinoAbs, err := filepath.Abs(destino)
        if err != nil {
                return herramientas.Resultado{Exito: false,
                        Error: fmt.Sprintf("destino inválido: %v", err)}, nil
        }

        if err := os.Rename(rutaAbs, destinoAbs); err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "mover",
                        Ruta:      rutaAbs,
                        Destino:   destinoAbs,
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// opCopiar copia un archivo o directorio recursivamente.
func (e *Editor) opCopiar(params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
        destino, err := herramientas.ObtenerString(params, e.paramByName("destino"))
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }
        if destino == "" {
                return herramientas.Resultado{Exito: false,
                        Error: "parámetro 'destino' requerido"}, nil
        }
        destinoAbs, err := filepath.Abs(destino)
        if err != nil {
                return herramientas.Resultado{Exito: false,
                        Error: fmt.Sprintf("destino inválido: %v", err)}, nil
        }

        info, err := os.Stat(rutaAbs)
        if err != nil {
                return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
        }

        if info.IsDir() {
                if err := copiarDirectorio(rutaAbs, destinoAbs); err != nil {
                        return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
                }
        } else {
                if err := copiarArchivo(rutaAbs, destinoAbs); err != nil {
                        return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
                }
        }

        return herramientas.Resultado{
                Exito: true,
                Datos: ResultadoEditor{
                        Operacion: "copiar",
                        Ruta:      rutaAbs,
                        Destino:   destinoAbs,
                        Bytes:     info.Size(),
                },
                Metadata: herramientas.NuevaMetadata(0),
        }, nil
}

// copiarArchivo copia un archivo individual preservando permisos.
func copiarArchivo(origen, destino string) error {
        data, err := ioutil.ReadFile(origen)
        if err != nil {
                return err
        }
        info, err := os.Stat(origen)
        if err != nil {
                return err
        }
        return ioutil.WriteFile(destino, data, info.Mode())
}

// copiarDirectorio copia un directorio recursivamente.
func copiarDirectorio(origen, destino string) error {
        if err := os.MkdirAll(destino, 0755); err != nil {
                return err
        }
        entradas, err := ioutil.ReadDir(origen)
        if err != nil {
                return err
        }
        for _, e := range entradas {
                o := filepath.Join(origen, e.Name())
                d := filepath.Join(destino, e.Name())
                if e.IsDir() {
                        if err := copiarDirectorio(o, d); err != nil {
                                return err
                        }
                } else {
                        if err := copiarArchivo(o, d); err != nil {
                                return err
                        }
                }
        }
        return nil
}

// compilarRegex helper eliminado — usamos regexp.Compile directamente.

// paramByName busca un parámetro por nombre.
func (e *Editor) paramByName(nombre string) herramientas.Parametro {
        for _, p := range e.Parametros() {
                if p.Nombre == nombre {
                        return p
                }
        }
        return herramientas.Parametro{Nombre: nombre}
}
