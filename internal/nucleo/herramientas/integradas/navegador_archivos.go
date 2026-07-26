package integradas

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// NavegadorArchivos — Navegación de Directorios
// ============================================================================

// MaxArchivosListar limita cuántos archivos retorna Listar (previene OOM).
const MaxArchivosListar = 10000

// NavegadorArchivos permite explorar el sistema de archivos: listar
// directorios, stat de archivos, walk recursivo con filtros.
//
// Es solo-lectura — para escribir archivos usar la herramienta Editor.
type NavegadorArchivos struct{}

// NewNavegadorArchivos crea una instancia.
func NewNavegadorArchivos() *NavegadorArchivos { return &NavegadorArchivos{} }

var _ herramientas.Herramienta = (*NavegadorArchivos)(nil)

func (n *NavegadorArchivos) Nombre() string { return "navegador_archivos" }

func (n *NavegadorArchivos) Descripcion() string {
	return "Navega el sistema de archivos. Lista directorios, obtiene " +
		"metadata (stat), camina recursivamente con filtros por extensión " +
		"y profundidad. Solo lectura — para modificar usar 'editor'."
}

func (n *NavegadorArchivos) Parametros() []herramientas.Parametro {
	return []herramientas.Parametro{
		{
			Nombre:      "operacion",
			Tipo:        "string",
			Requerido:   true,
			Descripcion: "Operación a realizar.",
			Opciones:    []string{"listar", "stat", "arbol", "existe"},
		},
		{
			Nombre:      "ruta",
			Tipo:        "string",
			Requerido:   true,
			Descripcion: "Ruta absoluta o relativa al directorio de trabajo.",
		},
		{
			Nombre:      "patron",
			Tipo:        "string",
			Descripcion: "Patrón glob para filtrar (ej: '*.go'). Solo para 'listar' y 'arbol'.",
		},
		{
			Nombre:      "extensiones",
			Tipo:        "array",
			Items:       "string",
			Descripcion: "Lista de extensiones a incluir (sin punto). Ej: ['go', 'py'].",
		},
		{
			Nombre:      "profundidad_max",
			Tipo:        "int",
			Default:     1,
			Min:         float64Ptr(0),
			Max:         float64Ptr(20),
			Descripcion: "Profundidad máxima para 'arbol'. 0=solo raíz, 1=hijos directos.",
		},
		{
			Nombre:      "incluir_ocultos",
			Tipo:        "bool",
			Default:     false,
			Descripcion: "Si true, incluye archivos/dirs que empiezan con '.'.",
		},
		{
			Nombre:      "limite",
			Tipo:        "int",
			Default:     1000,
			Min:         float64Ptr(1),
			Max:         float64Ptr(MaxArchivosListar),
			Descripcion: "Máximo número de entradas a retornar.",
		},
	}
}

// ResultadoNavegador es el Datos de NavegadorArchivos.
type ResultadoNavegador struct {
	Operacion string           `json:"operacion"`
	Ruta      string           `json:"ruta"`
	Entradas  []EntradaArchivo `json:"entradas,omitempty"`
	Stat      *InfoArchivo     `json:"stat,omitempty"`
	Existe    bool             `json:"existe,omitempty"`
	Total     int              `json:"total,omitempty"`
	Truncado  bool             `json:"truncado,omitempty"`
}

// EntradaArchivo es la vista compacta de un archivo/directorio en una lista.
type EntradaArchivo struct {
	Nombre     string `json:"nombre"`
	Ruta       string `json:"ruta"`
	Tipo       string `json:"tipo"` // "archivo", "directorio", "symlink", "otro"
	Tamano     int64  `json:"tamano"`
	Modificado string `json:"modificado"`
	Permiso    string `json:"permiso"`
}

// InfoArchivo es la información detallada de un archivo (stat completo).
type InfoArchivo struct {
	Nombre       string `json:"nombre"`
	Ruta         string `json:"ruta"`
	RutaAbsoluta string `json:"ruta_absoluta"`
	Tipo         string `json:"tipo"`
	Tamano       int64  `json:"tamano"`
	Modificado   string `json:"modificado"`
	Creado       string `json:"creado,omitempty"`
	Accedido     string `json:"accedido,omitempty"`
	Permiso      string `json:"permiso"`
	Modo         uint32 `json:"modo"`
	EsDir        bool   `json:"es_dir"`
	EsSymlink    bool   `json:"es_symlink"`
	EsRegular    bool   `json:"es_regular"`
	UID          uint32 `json:"uid,omitempty"`
	GID          uint32 `json:"gid,omitempty"`
	SymlinkDest  string `json:"symlink_dest,omitempty"`
}

func (n *NavegadorArchivos) Validar() error { return nil }

func (n *NavegadorArchivos) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	operacion, err := herramientas.ObtenerString(params, n.paramByName("operacion"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	ruta, err := herramientas.ObtenerString(params, n.paramByName("ruta"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	// Resolver ruta absoluta
	rutaAbs, err := filepath.Abs(ruta)
	if err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("ruta inválida: %v", err)}, nil
	}

	switch operacion {
	case "listar":
		return n.opListar(ctx, params, rutaAbs)
	case "stat":
		return n.opStat(rutaAbs)
	case "arbol":
		return n.opArbol(ctx, params, rutaAbs)
	case "existe":
		_, err := os.Stat(rutaAbs)
		existe := err == nil
		return herramientas.Resultado{
			Exito: true,
			Datos: ResultadoNavegador{
				Operacion: operacion,
				Ruta:      rutaAbs,
				Existe:    existe,
			},
			Metadata: herramientas.NuevaMetadata(0),
		}, nil
	default:
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("operación '%s' no soportada", operacion)}, nil
	}
}

// opListar lista el contenido de un directorio.
func (n *NavegadorArchivos) opListar(ctx context.Context, params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
	patron, _ := herramientas.ObtenerString(params, n.paramByName("patron"))
	extensiones, _ := herramientas.ObtenerArrayString(params, n.paramByName("extensiones"))
	incluirOcultos, _ := herramientas.ObtenerBool(params, n.paramByName("incluir_ocultos"))
	limite, _ := herramientas.ObtenerInt(params, n.paramByName("limite"))
	if limite <= 0 {
		limite = 1000
	}

	entradas, err := os.ReadDir(rutaAbs)
	if err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("error leyendo directorio: %v", err)}, nil
	}

	resultado := make([]EntradaArchivo, 0, len(entradas))
	truncado := false

	for _, e := range entradas {
		// Verificar cancelación
		select {
		case <-ctx.Done():
			return herramientas.Resultado{Exito: false, Error: "cancelado"}, ctx.Err()
		default:
		}

		nombre := e.Name()
		if !incluirOcultos && strings.HasPrefix(nombre, ".") {
			continue
		}

		// Filtro por patrón glob
		if patron != "" {
			matched, err := filepath.Match(patron, nombre)
			if err != nil || !matched {
				continue
			}
		}

		// Filtro por extensión
		if len(extensiones) > 0 {
			ext := strings.TrimPrefix(filepath.Ext(nombre), ".")
			encontrado := false
			for _, e := range extensiones {
				if strings.EqualFold(ext, e) {
					encontrado = true
					break
				}
			}
			if !encontrado {
				continue
			}
		}

		// Truncar
		if len(resultado) >= limite {
			truncado = true
			break
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		resultado = append(resultado, entradaDesdeInfo(filepath.Join(rutaAbs, nombre), info))
	}

	// Ordenar: directorios primero, luego alfabético
	sort.SliceStable(resultado, func(i, j int) bool {
		if (resultado[i].Tipo == "directorio") != (resultado[j].Tipo == "directorio") {
			return resultado[i].Tipo == "directorio"
		}
		return resultado[i].Nombre < resultado[j].Nombre
	})

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoNavegador{
			Operacion: "listar",
			Ruta:      rutaAbs,
			Entradas:  resultado,
			Total:     len(resultado),
			Truncado:  truncado,
		},
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// opStat retorna información detallada de un archivo/directorio.
func (n *NavegadorArchivos) opStat(rutaAbs string) (herramientas.Resultado, error) {
	info, err := os.Lstat(rutaAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return herramientas.Resultado{Exito: false,
				Error: fmt.Sprintf("archivo no existe: %s", rutaAbs)}, nil
		}
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("error en stat: %v", err)}, nil
	}

	detalle := &InfoArchivo{
		Nombre:       filepath.Base(rutaAbs),
		Ruta:         rutaAbs,
		RutaAbsoluta: rutaAbs,
		Tamano:       info.Size(),
		Modificado:   info.ModTime().Format(time.RFC3339),
		Permiso:      info.Mode().String(),
		Modo:         uint32(info.Mode()),
		EsDir:        info.IsDir(),
		EsSymlink:    info.Mode()&os.ModeSymlink != 0,
		EsRegular:    info.Mode().IsRegular(),
	}

	// Tipo
	switch {
	case detalle.EsDir:
		detalle.Tipo = "directorio"
	case detalle.EsSymlink:
		detalle.Tipo = "symlink"
		// Resolver destino del symlink
		dest, err := os.Readlink(rutaAbs)
		if err == nil {
			detalle.SymlinkDest = dest
		}
	case detalle.EsRegular:
		detalle.Tipo = "archivo"
	default:
		detalle.Tipo = "otro"
	}

	// Stat (no Lstat) para acceder a info de timestamps del sistema
	if stat, err := os.Stat(rutaAbs); err == nil {
		if sys, ok := stat.Sys().(interface {
			Atim() uint64
			Mtim() uint64
		}); ok {
			_ = sys // disponible en Unix
		}
	}

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoNavegador{
			Operacion: "stat",
			Ruta:      rutaAbs,
			Stat:      detalle,
		},
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// opArbol camina recursivamente hasta profundidad_max.
func (n *NavegadorArchivos) opArbol(ctx context.Context, params map[string]interface{}, rutaAbs string) (herramientas.Resultado, error) {
	profundidad, _ := herramientas.ObtenerInt(params, n.paramByName("profundidad_max"))
	if profundidad < 0 {
		profundidad = 1
	}
	patron, _ := herramientas.ObtenerString(params, n.paramByName("patron"))
	extensiones, _ := herramientas.ObtenerArrayString(params, n.paramByName("extensiones"))
	incluirOcultos, _ := herramientas.ObtenerBool(params, n.paramByName("incluir_ocultos"))
	limite, _ := herramientas.ObtenerInt(params, n.paramByName("limite"))
	if limite <= 0 {
		limite = 1000
	}

	var resultado []EntradaArchivo
	truncado := false

	err := filepath.WalkDir(rutaAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // continuar
		}

		// Verificar ctx
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Profundidad
		rel, err := filepath.Rel(rutaAbs, path)
		if err != nil {
			return nil
		}
		prof := 0
		if rel != "." {
			prof = strings.Count(rel, string(filepath.Separator)) + 1
		}
		if prof > profundidad {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		nombre := d.Name()
		if !incluirOcultos && strings.HasPrefix(nombre, ".") {
			if d.IsDir() && prof > 0 {
				return filepath.SkipDir
			}
			return nil
		}

		// Filtros solo en archivos
		if !d.IsDir() {
			if patron != "" {
				matched, err := filepath.Match(patron, nombre)
				if err != nil || !matched {
					return nil
				}
			}
			if len(extensiones) > 0 {
				ext := strings.TrimPrefix(filepath.Ext(nombre), ".")
				encontrado := false
				for _, e := range extensiones {
					if strings.EqualFold(ext, e) {
						encontrado = true
						break
					}
				}
				if !encontrado {
					return nil
				}
			}
		}

		if len(resultado) >= limite {
			truncado = true
			return filepath.SkipAll
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		resultado = append(resultado, entradaDesdeInfo(path, info))
		return nil
	})

	if err != nil && err != context.Canceled {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("error en walk: %v", err)}, nil
	}

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoNavegador{
			Operacion: "arbol",
			Ruta:      rutaAbs,
			Entradas:  resultado,
			Total:     len(resultado),
			Truncado:  truncado,
		},
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// entradaDesdeInfo convierte fs.FileInfo a EntradaArchivo.
func entradaDesdeInfo(ruta string, info fs.FileInfo) EntradaArchivo {
	tipo := "archivo"
	if info.IsDir() {
		tipo = "directorio"
	} else if info.Mode()&os.ModeSymlink != 0 {
		tipo = "symlink"
	} else if !info.Mode().IsRegular() {
		tipo = "otro"
	}
	return EntradaArchivo{
		Nombre:     filepath.Base(ruta),
		Ruta:       ruta,
		Tipo:       tipo,
		Tamano:     info.Size(),
		Modificado: info.ModTime().Format(time.RFC3339),
		Permiso:    info.Mode().String(),
	}
}

// paramByName busca un parámetro por nombre.
func (n *NavegadorArchivos) paramByName(nombre string) herramientas.Parametro {
	for _, p := range n.Parametros() {
		if p.Nombre == nombre {
			return p
		}
	}
	return herramientas.Parametro{Nombre: nombre}
}
