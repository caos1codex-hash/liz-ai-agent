package integradas

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Buscador — Búsqueda de archivos por nombre y contenido
// ============================================================================

// MaxResultadosBuscador limita el número de resultados (archivos + matches).
const MaxResultadosBuscador = 5000

// MaxTamanoArchivoBuscador es el tamaño máximo de archivo para buscar
// contenido (10MB). Archivos más grandes se saltan.
const MaxTamanoArchivoBuscador = 10 * 1024 * 1024

// Buscador encuentra archivos por nombre (patrón glob) y/o contenido (regex/string).
//
// Operaciones:
//   - "archivos" — find por nombre (glob, extensión, mtime, tamaño)
//   - "contenido" — grep recursivo con contexto (líneas antes/después)
//   - "combinado" — find + grep en una pasada
//
// Es solo-lectura.
type Buscador struct{}

// NewBuscador crea una instancia.
func NewBuscador() *Buscador { return &Buscador{} }

var _ herramientas.Herramienta = (*Buscador)(nil)

func (b *Buscador) Nombre() string { return "buscador" }

func (b *Buscador) Descripcion() string {
	return "Busca archivos por nombre (patrón glob, extensión, mtime, tamaño) " +
		"y/o contenido (grep con contexto). Soporta regex, case-insensitive, " +
		"y búsqueda combinada en una pasada."
}

func (b *Buscador) Parametros() []herramientas.Parametro {
	return []herramientas.Parametro{
		{
			Nombre:      "operacion",
			Tipo:        "string",
			Requerido:   true,
			Opciones:    []string{"archivos", "contenido", "combinado"},
			Descripcion: "Tipo de búsqueda a realizar.",
		},
		{
			Nombre:      "ruta",
			Tipo:        "string",
			Requerido:   true,
			Descripcion: "Directorio raíz para la búsqueda (recursiva).",
		},
		{
			Nombre:      "patron",
			Tipo:        "string",
			Descripcion: "Patrón glob para filtrar por nombre. Ej: '*.go'.",
		},
		{
			Nombre:      "extensiones",
			Tipo:        "array",
			Items:       "string",
			Descripcion: "Lista de extensiones a incluir (sin punto).",
		},
		{
			Nombre:      "texto",
			Tipo:        "string",
			Descripcion: "Texto a buscar en el contenido (literal o regex).",
		},
		{
			Nombre:      "regex",
			Tipo:        "bool",
			Default:     false,
			Descripcion: "Si true, 'texto' se interpreta como regex (Go syntax).",
		},
		{
			Nombre:      "ignorar_case",
			Tipo:        "bool",
			Default:     true,
			Descripcion: "Si true, búsqueda case-insensitive.",
		},
		{
			Nombre:      "contexto_lineas",
			Tipo:        "int",
			Default:     0,
			Min:         float64Ptr(0),
			Max:         float64Ptr(10),
			Descripcion: "Líneas de contexto (antes y después) en búsqueda de contenido.",
		},
		{
			Nombre:      "modificado_desde",
			Tipo:        "string",
			Descripcion: "Filtrar archivos modificados después de esta fecha (RFC3339 o duración como '24h', '7d').",
		},
		{
			Nombre:      "tamano_min",
			Tipo:        "int",
			Min:         float64Ptr(0),
			Descripcion: "Tamaño mínimo en bytes.",
		},
		{
			Nombre:      "tamano_max",
			Tipo:        "int",
			Min:         float64Ptr(0),
			Descripcion: "Tamaño máximo en bytes.",
		},
		{
			Nombre:      "incluir_ocultos",
			Tipo:        "bool",
			Default:     false,
			Descripcion: "Si true, incluye archivos/dirs ocultos.",
		},
		{
			Nombre:      "limite",
			Tipo:        "int",
			Default:     100,
			Min:         float64Ptr(1),
			Max:         float64Ptr(MaxResultadosBuscador),
			Descripcion: "Máximo número de resultados.",
		},
		{
			Nombre:      "paralelo",
			Tipo:        "bool",
			Default:     true,
			Descripcion: "Si true, búsqueda de contenido en paralelo (más rápido).",
		},
	}
}

// ResultadoBuscador es el Datos de Buscador.
type ResultadoBuscador struct {
	Operacion     string              `json:"operacion"`
	Ruta          string              `json:"ruta"`
	Archivos      []ArchivoEncontrado `json:"archivos,omitempty"`
	Matches       []MatchContenido    `json:"matches,omitempty"`
	TotalArchivos int                 `json:"total_archivos"`
	TotalMatches  int                 `json:"total_matches"`
	Truncado      bool                `json:"truncado,omitempty"`
	DuracionMs    float64             `json:"duracion_ms"`
}

// ArchivoEncontrado describe un archivo que pasó todos los filtros.
type ArchivoEncontrado struct {
	Ruta       string `json:"ruta"`
	Nombre     string `json:"nombre"`
	Tamano     int64  `json:"tamano"`
	Modificado string `json:"modificado"`
}

// MatchContenido describe una línea que contiene el texto buscado.
type MatchContenido struct {
	Archivo        string   `json:"archivo"`
	Linea          int      `json:"linea"`
	Columna        int      `json:"columna"`
	Contenido      string   `json:"contenido"`
	Contexto       []string `json:"contexto,omitempty"`
	NumLineaInicio int      `json:"num_linea_inicio,omitempty"`
}

func (b *Buscador) Validar() error { return nil }

func (b *Buscador) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	inicio := time.Now()

	operacion, err := herramientas.ObtenerString(params, b.paramByName("operacion"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	ruta, err := herramientas.ObtenerString(params, b.paramByName("ruta"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	rutaAbs, err := filepath.Abs(ruta)
	if err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("ruta inválida: %v", err)}, nil
	}

	filtros, err := construirFiltros(params, b)
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	limite, _ := herramientas.ObtenerInt(params, b.paramByName("limite"))
	if limite <= 0 {
		limite = 100
	}
	paralelo, _ := herramientas.ObtenerBool(params, b.paramByName("paralelo"))

	var resultado ResultadoBuscador
	resultado.Operacion = operacion
	resultado.Ruta = rutaAbs

	switch operacion {
	case "archivos":
		archs, trunc, err := b.buscarArchivos(ctx, rutaAbs, filtros, limite)
		if err != nil {
			return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
		}
		resultado.Archivos = archs
		resultado.TotalArchivos = len(archs)
		resultado.Truncado = trunc

	case "contenido":
		if filtros.texto == "" {
			return herramientas.Resultado{Exito: false,
				Error: "operación 'contenido' requiere parámetro 'texto'"}, nil
		}
		matches, trunc, err := b.buscarContenido(ctx, rutaAbs, filtros, limite, paralelo)
		if err != nil {
			return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
		}
		resultado.Matches = matches
		resultado.TotalMatches = len(matches)
		resultado.Truncado = trunc

	case "combinado":
		if filtros.texto == "" {
			return herramientas.Resultado{Exito: false,
				Error: "operación 'combinado' requiere parámetro 'texto'"}, nil
		}
		matches, trunc, err := b.buscarContenido(ctx, rutaAbs, filtros, limite, paralelo)
		if err != nil {
			return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
		}
		resultado.Matches = matches
		resultado.TotalMatches = len(matches)
		resultado.Truncado = trunc

	default:
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("operación '%s' no soportada", operacion)}, nil
	}

	resultado.DuracionMs = float64(time.Since(inicio).Microseconds()) / 1000.0

	return herramientas.Resultado{
		Exito:    true,
		Datos:    resultado,
		Metadata: herramientas.NuevaMetadata(resultado.DuracionMs),
	}, nil
}

// filtrosBusqueda agrupa todos los criterios de filtrado.
type filtrosBusqueda struct {
	patron          string
	extensiones     []string
	texto           string
	regex           bool
	ignorarCase     bool
	contextoLineas  int
	modificadoDesde time.Time
	tamanoMin       int64
	tamanoMax       int64
	incluirOcultos  bool
}

// construirFiltros extrae los filtros comunes de params.
func construirFiltros(params map[string]interface{}, b *Buscador) (*filtrosBusqueda, error) {
	f := &filtrosBusqueda{}

	f.patron, _ = herramientas.ObtenerString(params, b.paramByName("patron"))
	f.extensiones, _ = herramientas.ObtenerArrayString(params, b.paramByName("extensiones"))
	f.texto, _ = herramientas.ObtenerString(params, b.paramByName("texto"))
	f.regex, _ = herramientas.ObtenerBool(params, b.paramByName("regex"))
	f.ignorarCase, _ = herramientas.ObtenerBool(params, b.paramByName("ignorar_case"))
	f.contextoLineas, _ = herramientas.ObtenerInt(params, b.paramByName("contexto_lineas"))
	f.incluirOcultos, _ = herramientas.ObtenerBool(params, b.paramByName("incluir_ocultos"))

	if modStr, _ := herramientas.ObtenerString(params, b.paramByName("modificado_desde")); modStr != "" {
		t, err := parsearFechaRelativa(modStr)
		if err != nil {
			return nil, fmt.Errorf("modificado_desde inválido: %v", err)
		}
		f.modificadoDesde = t
	}

	if min, _ := herramientas.ObtenerInt(params, b.paramByName("tamano_min")); min > 0 {
		f.tamanoMin = int64(min)
	}
	if max, _ := herramientas.ObtenerInt(params, b.paramByName("tamano_max")); max > 0 {
		f.tamanoMax = int64(max)
	}

	return f, nil
}

// parsearFechaRelativa acepta RFC3339, duración Go (24h, 7d), o "hoy".
func parsearFechaRelativa(s string) (time.Time, error) {
	if s == "hoy" {
		return time.Now().Truncate(24 * time.Hour), nil
	}
	// Intentar RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Intentar duración (24h, 7d, etc.)
	d, err := time.ParseDuration(s)
	if err == nil {
		return time.Now().Add(-d), nil
	}
	// "7d" no es válido para ParseDuration — convertir
	if strings.HasSuffix(s, "d") {
		var dias int
		if _, err := fmt.Sscanf(s, "%dd", &dias); err == nil {
			return time.Now().AddDate(0, 0, -dias), nil
		}
	}
	return time.Time{}, fmt.Errorf("formato no reconocido: %s", s)
}

// buscarArchivos lista archivos que pasan todos los filtros.
func (b *Buscador) buscarArchivos(ctx context.Context, raiz string, f *filtrosBusqueda,
	limite int) ([]ArchivoEncontrado, bool, error) {

	var resultado []ArchivoEncontrado
	truncado := false

	err := filepath.WalkDir(raiz, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			if !f.incluirOcultos && strings.HasPrefix(d.Name(), ".") && path != raiz {
				return filepath.SkipDir
			}
			return nil
		}

		// Filtros
		if !pasaFiltrosArchivo(path, d, f) {
			return nil
		}

		if len(resultado) >= limite {
			truncado = true
			return filepath.SkipAll
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		resultado = append(resultado, ArchivoEncontrado{
			Ruta:       path,
			Nombre:     d.Name(),
			Tamano:     info.Size(),
			Modificado: info.ModTime().Format(time.RFC3339),
		})
		return nil
	})

	sort.Slice(resultado, func(i, j int) bool {
		return resultado[i].Ruta < resultado[j].Ruta
	})

	return resultado, truncado, err
}

// pasaFiltrosArchivo verifica si un archivo pasa todos los filtros.
func pasaFiltrosArchivo(path string, d fs.DirEntry, f *filtrosBusqueda) bool {
	nombre := d.Name()

	// Ocultos
	if !f.incluirOcultos && strings.HasPrefix(nombre, ".") {
		return false
	}

	// Patrón glob
	if f.patron != "" {
		matched, err := filepath.Match(f.patron, nombre)
		if err != nil || !matched {
			return false
		}
	}

	// Extensión
	if len(f.extensiones) > 0 {
		ext := strings.TrimPrefix(filepath.Ext(nombre), ".")
		encontrado := false
		for _, e := range f.extensiones {
			if strings.EqualFold(ext, e) {
				encontrado = true
				break
			}
		}
		if !encontrado {
			return false
		}
	}

	info, err := d.Info()
	if err != nil {
		return false
	}

	// Tamaño
	if f.tamanoMin > 0 && info.Size() < f.tamanoMin {
		return false
	}
	if f.tamanoMax > 0 && info.Size() > f.tamanoMax {
		return false
	}

	// Mtime
	if !f.modificadoDesde.IsZero() && info.ModTime().Before(f.modificadoDesde) {
		return false
	}

	return true
}

// buscarContenido busca texto en archivos recursivamente.
// Soporta paralelización para acelerar la búsqueda.
func (b *Buscador) buscarContenido(ctx context.Context, raiz string, f *filtrosBusqueda,
	limite int, paralelo bool) ([]MatchContenido, bool, error) {

	// Primero: recolectar archivos candidatos
	archs, _, err := b.buscarArchivos(ctx, raiz, f, MaxResultadosBuscador)
	if err != nil {
		return nil, false, err
	}

	if len(archs) == 0 {
		return nil, false, nil
	}

	// Compilar patrón de búsqueda
	buscar, err := compilarBusquedaContenido(f)
	if err != nil {
		return nil, false, err
	}

	var (
		mu        sync.Mutex
		matches   []MatchContenido
		truncado  bool
		ctxCancel error
	)

	procesarArchivo := func(arch ArchivoEncontrado) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Saltar archivos muy grandes
		if arch.Tamano > MaxTamanoArchivoBuscador {
			return nil
		}
		// Saltar archivos binarios (heurística: extensión)
		if esBinario(arch.Nombre) {
			return nil
		}

		ms, err := buscarEnArchivo(arch.Ruta, f, buscar)
		if err != nil {
			return nil // continuar
		}

		mu.Lock()
		defer mu.Unlock()
		if ctxCancel != nil {
			return nil
		}
		for _, m := range ms {
			if len(matches) >= limite {
				truncado = true
				return nil
			}
			matches = append(matches, m)
		}
		return nil
	}

	if paralelo && len(archs) > 10 {
		// Búsqueda paralela
		sem := make(chan struct{}, 8) // 8 workers
		var wg sync.WaitGroup
		for _, arch := range archs {
			select {
			case <-ctx.Done():
				ctxCancel = ctx.Err()
				break
			default:
			}
			if ctxCancel != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(a ArchivoEncontrado) {
				defer wg.Done()
				defer func() { <-sem }()
				_ = procesarArchivo(a)
			}(arch)
		}
		wg.Wait()
	} else {
		// Búsqueda secuencial
		for _, arch := range archs {
			if ctxCancel != nil {
				break
			}
			if err := procesarArchivo(arch); err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					ctxCancel = err
					break
				}
			}
		}
	}

	// Ordenar matches por archivo + línea
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Archivo != matches[j].Archivo {
			return matches[i].Archivo < matches[j].Archivo
		}
		return matches[i].Linea < matches[j].Linea
	})

	return matches, truncado, nil
}

// compilarBusquedaContenido prepara la función de búsqueda.
// Retorna una función que recibe el texto de una línea y retorna
// (columna, true) si hay match.
func compilarBusquedaContenido(f *filtrosBusqueda) (func(string) (int, bool), error) {
	if f.texto == "" {
		return func(string) (int, bool) { return 0, false }, nil
	}

	agujas := []string{f.texto}
	if f.ignorarCase {
		agujas[0] = strings.ToLower(f.texto)
	}

	if f.regex {
		// Para regex usamos regexp.Compile
		// (lo manejamos en buscarEnArchivo con un compilado)
		return nil, nil // signal que es regex
	}

	return func(linea string) (int, bool) {
		l := linea
		if f.ignorarCase {
			l = strings.ToLower(l)
		}
		idx := strings.Index(l, agujas[0])
		if idx < 0 {
			return 0, false
		}
		return idx + 1, true // 1-indexed
	}, nil
}

// buscarEnArchivo escanea un archivo línea por línea buscando coincidencias.
func buscarEnArchivo(ruta string, f *filtrosBusqueda, buscarFn func(string) (int, bool)) ([]MatchContenido, error) {
	archivo, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer archivo.Close()

	// Si regex, compilar patrón
	var regex *regexp.Regexp
	if f.regex {
		patron := f.texto
		if f.ignorarCase {
			patron = "(?i)" + patron
		}
		r, err := regexp.Compile(patron)
		if err != nil {
			return nil, fmt.Errorf("regex inválido: %v", err)
		}
		regex = r
	}

	var matches []MatchContenido
	scanner := bufio.NewScanner(archivo)
	// Buffer grande para líneas largas
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	lineaNum := 0
	var lineasPrevias []string // buffer circular para contexto

	for scanner.Scan() {
		lineaNum++
		linea := scanner.Text()

		var col int
		var ok bool
		if regex != nil {
			loc := regex.FindStringIndex(linea)
			if loc != nil {
				col = loc[0] + 1
				ok = true
			}
		} else if buscarFn != nil {
			col, ok = buscarFn(linea)
		}

		if ok {
			m := MatchContenido{
				Archivo:   ruta,
				Linea:     lineaNum,
				Columna:   col,
				Contenido: truncarLinea(linea, 500),
			}
			// Contexto
			if f.contextoLineas > 0 {
				ctxInicio := max(1, lineaNum-f.contextoLineas)
				ctxFin := lineaNum + f.contextoLineas
				_ = ctxFin
				m.NumLineaInicio = ctxInicio
				// Solo incluir contexto anterior (lo siguiente se agrega en scan siguiente)
				if len(lineasPrevias) > 0 {
					m.Contexto = append(m.Contexto, lineasPrevias...)
				}
			}
			matches = append(matches, m)
		}

		// Mantener buffer de líneas previas para contexto
		if f.contextoLineas > 0 {
			lineasPrevias = append(lineasPrevias, truncarLinea(linea, 500))
			if len(lineasPrevias) > f.contextoLineas {
				lineasPrevias = lineasPrevias[1:]
			}
		}
	}

	// Para matches que requieren contexto posterior, necesitamos segundo scan
	// Simplificación: contexto posterior se agrega después
	if f.contextoLineas > 0 && len(matches) > 0 {
		agregarContextoPosterior(ruta, matches, f.contextoLineas)
	}

	return matches, scanner.Err()
}

// agregarContextoPosterior reescanea el archivo para agregar líneas
// posteriores al match (versión simplificada: agrega a cada match las
// líneas hasta linea+contextoLineas).
func agregarContextoPosterior(ruta string, matches []MatchContenido, contextoLineas int) {
	archivo, err := os.Open(ruta)
	if err != nil {
		return
	}
	defer archivo.Close()

	// Cargar todas las líneas (asumimos archivos pequeños para contexto)
	var todas []string
	scanner := bufio.NewScanner(archivo)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		todas = append(todas, scanner.Text())
	}

	for i := range matches {
		m := &matches[i]
		// Contexto anterior ya está en m.Contexto
		ctxAntes := len(m.Contexto)
		// Líneas posteriores
		fin := m.Linea + contextoLineas
		if fin > len(todas) {
			fin = len(todas)
		}
		for j := m.Linea + 1; j <= fin; j++ {
			if j-1 < len(todas) {
				m.Contexto = append(m.Contexto, truncarLinea(todas[j-1], 500))
			}
		}
		_ = ctxAntes
	}
}

// esBinario heurística: extensiones conocidas de archivos binarios.
func esBinario(nombre string) bool {
	ext := strings.ToLower(filepath.Ext(nombre))
	binarios := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
		".ico": true, ".webp": true, ".tiff": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
		".7z": true, ".rar": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".mkv": true,
		".wav": true, ".flac": true, ".ogg": true,
		".o": true, ".a": true, ".lib": true,
		".class": true, ".jar": true, ".war": true,
		".pyc": true, ".pyo": true,
		".bin": true, ".dat": true, ".db": true, ".sqlite": true, ".db3": true,
	}
	return binarios[ext]
}

// truncarLinea recorta una línea a maxChars caracteres.
func truncarLinea(s string, maxChars int) string {
	if len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + "..."
}

// max helper.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// paramByName busca un parámetro por nombre.
func (b *Buscador) paramByName(nombre string) herramientas.Parametro {
	for _, p := range b.Parametros() {
		if p.Nombre == nombre {
			return p
		}
	}
	return herramientas.Parametro{Nombre: nombre}
}

// import regexp aquí abajo para evitar dependencia en caliente
// (lo compilamos solo cuando regex=true).
// regexp es stdlib así que no agrega dependencias.
