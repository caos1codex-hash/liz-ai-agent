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
	ID        string `json:"id"`         // hash SHA256 del contenido
	Ruta      string `json:"ruta"`       // ruta relativa del archivo origen
	LineaIni  int    `json:"linea_ini"`  // línea de inicio (1-indexed)
	LineaFin  int    `json:"linea_fin"`  // línea de fin (inclusive)
	Tipo      string `json:"tipo"`       // "funcion", "estructura", "import", "config", "completo", etc.
	Lenguaje  string `json:"lenguaje"`
	Contenido string `json:"contenido"`  // el texto del fragmento
	Resumen   string `json:"resumen"`    // resumen de una línea del fragmento
	Timestamp string `json:"timestamp"`  // cuándo se creó
	Tamanio   int    `json:"tamanio"`    // bytes del contenido
}

// MetadataArchivo contiene la metadata de fragmentos de un archivo.
type MetadataArchivo struct {
	Ruta                string   `json:"ruta"`
	TotalFragmentos     int      `json:"total_fragmentos"`
	IDs                 []string `json:"ids"`
	UltimaActualizacion string   `json:"ultima_actualizacion"`
}

// Almacen es el almacenamiento de fragmentos del sistema de contexto.
// Persiste fragmentos como archivos JSON individuales (uno por fragmento).
// Mantiene un índice en memoria (ruta → []id) para consultas O(1).
type Almacen struct {
	directorio string // ~/.liz/contexto/proyectos/<proyecto>/archivos/
	mu         sync.RWMutex
	logFunc    func(string, ...interface{})
	// Índice en memoria: ruta relativa → IDs de fragmentos.
	// Se carga al iniciar y se mantiene en escritura.
	indiceRuta map[string][]string
	cargado    bool
}

// NuevoAlmacen crea un nuevo almacén de fragmentos para un proyecto.
func NuevoAlmacen(dirBase string, nombreProyecto string) (*Almacen, error) {
	dir := filepath.Join(dirBase, nombreProyecto, ".liz", "archivos")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de fragmentos: %w", err)
	}

	a := &Almacen{
		directorio: dir,
		logFunc:    func(string, ...interface{}) {},
		indiceRuta: make(map[string][]string),
	}
	a.cargarIndice()

	return a, nil
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

// cargarIndice recorre el directorio una vez al iniciar y construye
// el índice en memoria ruta → []id.
func (a *Almacen) cargarIndice() {
	a.mu.Lock()
	defer a.mu.Unlock()

	entradas, err := os.ReadDir(a.directorio)
	if err != nil {
		a.cargado = true
		return
	}

	for _, entrada := range entradas {
		if entrada.IsDir() || !strings.HasSuffix(entrada.Name(), ".json") {
			continue
		}
		// El ID es el nombre del archivo sin la extensión .json
		id := strings.TrimSuffix(entrada.Name(), ".json")
		// Leer solo los campos necesarios para el índice
		datos, err := os.ReadFile(filepath.Join(a.directorio, entrada.Name()))
		if err != nil {
			continue
		}
		// Parseo parcial: solo necesitamos el campo "ruta"
		var parcial struct {
			Ruta string `json:"ruta"`
		}
		if json.Unmarshal(datos, &parcial) != nil {
			continue
		}
		if parcial.Ruta != "" {
			a.indiceRuta[parcial.Ruta] = append(a.indiceRuta[parcial.Ruta], id)
		}
	}

	a.cargado = true
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
		// Ya existe en disco, pero asegurarse de que esté en el índice
		if !contiene(a.indiceRuta[ruta], id) {
			a.indiceRuta[ruta] = append(a.indiceRuta[ruta], id)
		}
		return id, nil
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

	// Actualizar índice en memoria
	a.indiceRuta[ruta] = append(a.indiceRuta[ruta], id)

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
// El fragmentado es inteligente: para Go, Python, JS/TS, Rust, Java, C/C++
// fragmenta por funciones/clases/estructuras. Para otros lenguajes, usa
// el archivo completo.
func (a *Almacen) AgregarDesdeArchivo(rutaRelativa, rutaAbsoluta string) ([]string, error) {
	contenido, err := os.ReadFile(rutaAbsoluta)
	if err != nil {
		return nil, fmt.Errorf("error leyendo archivo: %w", err)
	}

	est := filepath.Ext(rutaRelativa)
	lenguaje := detectarLenguajeExt(est)

	// Fragmentar inteligentemente según lenguaje
	frags := fragmentarContenido(string(contenido), lenguaje)

	var ids []string
	for _, frag := range frags {
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
// Usa el índice en memoria para evitar escanear todo el directorio.
func (a *Almacen) ObtenerPorRuta(ruta string) ([]Fragmento, error) {
	a.mu.RLock()
	ids := make([]string, len(a.indiceRuta[ruta]))
	copy(ids, a.indiceRuta[ruta])
	a.mu.RUnlock()

	if len(ids) == 0 {
		return []Fragmento{}, nil
	}

	// Leer cada fragmento por ID
	resultado := make([]Fragmento, 0, len(ids))
	for _, id := range ids {
		frag, err := a.Obtener(id)
		if err != nil {
			continue
		}
		resultado = append(resultado, *frag)
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

	total := 0
	for _, ids := range a.indiceRuta {
		total += len(ids)
	}
	return total
}

// EliminarPorRuta elimina todos los fragmentos asociados a una ruta.
// Retorna los IDs eliminados.
func (a *Almacen) EliminarPorRuta(ruta string) ([]string, error) {
	a.mu.Lock()
	ids := a.indiceRuta[ruta]
	delete(a.indiceRuta, ruta)
	a.mu.Unlock()

	var eliminados []string
	for _, id := range ids {
		rutaFrag := filepath.Join(a.directorio, id+".json")
		if err := os.Remove(rutaFrag); err != nil {
			a.logFunc("error eliminando fragmento %s: %v", id, err)
			continue
		}
		eliminados = append(eliminados, id)
	}

	return eliminados, nil
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
	case "python":
		return fragmentarPython(contenido)
	case "javascript", "typescript":
		return fragmentarJS(contenido)
	case "rust":
		return fragmentarRust(contenido)
	case "java":
		return fragmentarJava(contenido)
	case "c", "cpp":
		return fragmentarC(contenido)
	default:
		return nil // sin fragmentación inteligente, se usa completo
	}
}

// fragmentarGo divide código Go en fragmentos por funciones, tipos y variables.
// Rastrea la profundidad de llaves para detectar correctamente el final de cada
// bloque (incluyendo funciones multi-línea con firmas largas).
func fragmentarGo(contenido string) []fragmentoInterno {
	lineas := strings.Split(contenido, "\n")

	type bloque struct {
		tipo      string
		lineaIni  int
		contenido strings.Builder
		// profunidad de llaves al iniciar el bloque (típicamente 0)
		profInicial int
		// true cuando hemos visto la primera "{" del bloque
		iniciado bool
	}

	var resultado []fragmentoInterno
	var actual *bloque
	profundidad := 0 // nivel de {} actual

	cerrarBloque := func(lineaFin int) {
		if actual == nil {
			return
		}
		if actual.contenido.Len() == 0 {
			actual = nil
			return
		}
		texto := strings.TrimRight(actual.contenido.String(), "\n")
		if strings.TrimSpace(texto) != "" {
			resultado = append(resultado, fragmentoInterno{
				contenido: texto,
				tipo:      actual.tipo,
				lineaIni:  actual.lineaIni,
				lineaFin:  lineaFin,
			})
		}
		actual = nil
	}

	for i, linea := range lineas {
		numLinea := i + 1
		trim := strings.TrimSpace(linea)

		// Detectar inicio de tipos a nivel superior
		if profundidad == 0 && strings.HasPrefix(trim, "type ") {
			cerrarBloque(numLinea - 1)
			actual = &bloque{tipo: "estructura", lineaIni: numLinea}
		}

		// Detectar inicio de funciones a nivel superior
		if profundidad == 0 && (strings.HasPrefix(trim, "func ") || strings.HasPrefix(trim, "func(")) {
			cerrarBloque(numLinea - 1)
			actual = &bloque{tipo: "funcion", lineaIni: numLinea}
		}

		// Detectar imports a nivel superior
		if profundidad == 0 && (trim == "import (" || strings.HasPrefix(trim, "import ")) {
			cerrarBloque(numLinea - 1)
			actual = &bloque{tipo: "import", lineaIni: numLinea}
		}

		// Acumular línea
		if actual != nil {
			actual.contenido.WriteString(linea)
			actual.contenido.WriteString("\n")
		}

		// Actualizar profundidad basada en llaves de esta línea
		// (ignorando strings y comentarios para mayor precisión)
		abre, cierra := contarLlaves(linea)
		if actual != nil && !actual.iniciado && abre > 0 {
			actual.iniciado = true
			actual.profInicial = profundidad
		}
		profundidad += abre - cierra

		// Si el bloque ya está iniciado y volvemos a su profundidad inicial, cerrar
		if actual != nil && actual.iniciado && profundidad <= actual.profInicial && numLinea > actual.lineaIni {
			cerrarBloque(numLinea)
		}
	}

	// Cerrar último bloque
	cerrarBloque(len(lineas))

	return resultado
}

// fragmentarPython divide código Python por clases y funciones.
// Python usa indentación (no llaves), así que detectamos por `def`/`class`/`async def`
// y cortamos cuando la indentación vuelve al nivel anterior.
func fragmentarPython(contenido string) []fragmentoInterno {
	lineas := strings.Split(contenido, "\n")

	var resultado []fragmentoInterno
	type bloque struct {
		tipo       string
		lineaIni   int
		indentBase int // indentación de la línea `def`/`class`
		contenido  strings.Builder
	}
	var actual *bloque

	cerrarBloque := func(lineaFin int) {
		if actual == nil {
			return
		}
		texto := strings.TrimRight(actual.contenido.String(), "\n")
		if strings.TrimSpace(texto) != "" {
			resultado = append(resultado, fragmentoInterno{
				contenido: texto,
				tipo:      actual.tipo,
				lineaIni:  actual.lineaIni,
				lineaFin:  lineaFin,
			})
		}
		actual = nil
	}

	for i, linea := range lineas {
		numLinea := i + 1
		if strings.TrimSpace(linea) == "" {
			if actual != nil {
				actual.contenido.WriteString("\n")
			}
			continue
		}

		indent := len(linea) - len(strings.TrimLeft(linea, " \t"))
		trim := strings.TrimSpace(linea)

		// Detectar inicio de clase/función a nivel superior
		esDef := strings.HasPrefix(trim, "def ") || strings.HasPrefix(trim, "async def ")
		esClass := strings.HasPrefix(trim, "class ")
		if (esDef || esClass) && indent == 0 {
			cerrarBloque(numLinea - 1)
			tipo := "funcion"
			if esClass {
				tipo = "estructura"
			}
			actual = &bloque{tipo: tipo, lineaIni: numLinea, indentBase: 0}
			actual.contenido.WriteString(linea)
			actual.contenido.WriteString("\n")
			continue
		}

		// Si hay bloque activo y la indentación vuelve al nivel base (o menor), cerrar
		if actual != nil && indent <= actual.indentBase {
			cerrarBloque(numLinea - 1)
		}

		if actual != nil {
			actual.contenido.WriteString(linea)
			actual.contenido.WriteString("\n")
		}
	}

	cerrarBloque(len(lineas))

	return resultado
}

// fragmentarJS divide código JavaScript/TypeScript por funciones, clases y exports.
func fragmentarJS(contenido string) []fragmentoInterno {
	lineas := strings.Split(contenido, "\n")

	var resultado []fragmentoInterno
	type bloque struct {
		tipo        string
		lineaIni    int
		contenido   strings.Builder
		profInicial int
		iniciado    bool
	}
	var actual *bloque
	profundidad := 0

	cerrarBloque := func(lineaFin int) {
		if actual == nil {
			return
		}
		texto := strings.TrimRight(actual.contenido.String(), "\n")
		if strings.TrimSpace(texto) != "" {
			resultado = append(resultado, fragmentoInterno{
				contenido: texto,
				tipo:      actual.tipo,
				lineaIni:  actual.lineaIni,
				lineaFin:  lineaFin,
			})
		}
		actual = nil
	}

	palabrasClave := []string{"function ", "function*", "async function", "class ", "export function",
		"export async function", "export class", "export default function", "const ", "let ", "var ", "interface ", "type "}

	for i, linea := range lineas {
		numLinea := i + 1
		trim := strings.TrimSpace(linea)

		// Detectar inicio de bloque a nivel superior
		if profundidad == 0 {
			for _, kw := range palabrasClave {
				if strings.HasPrefix(trim, kw) {
					cerrarBloque(numLinea - 1)
					tipo := "funcion"
					if strings.HasPrefix(trim, "class ") || strings.HasPrefix(trim, "export class") {
						tipo = "estructura"
					} else if strings.HasPrefix(trim, "interface ") || strings.HasPrefix(trim, "type ") {
						tipo = "tipo"
					} else if strings.HasPrefix(trim, "const ") || strings.HasPrefix(trim, "let ") || strings.HasPrefix(trim, "var ") {
						// Solo fragmentar si es una declaración de función flecha
						if !strings.Contains(trim, "=>") {
							tipo = ""
						} else {
							tipo = "funcion"
						}
					}
					if tipo != "" {
						actual = &bloque{tipo: tipo, lineaIni: numLinea}
					}
					break
				}
			}
		}

		if actual != nil {
			actual.contenido.WriteString(linea)
			actual.contenido.WriteString("\n")
		}

		abre, cierra := contarLlaves(linea)
		if actual != nil && !actual.iniciado && abre > 0 {
			actual.iniciado = true
			actual.profInicial = profundidad
		}
		profundidad += abre - cierra

		if actual != nil && actual.iniciado && profundidad <= actual.profInicial && numLinea > actual.lineaIni {
			cerrarBloque(numLinea)
		}
	}

	cerrarBloque(len(lineas))

	return resultado
}

// fragmentarRust divide código Rust por fn, struct, enum, impl, trait.
func fragmentarRust(contenido string) []fragmentoInterno {
	lineas := strings.Split(contenido, "\n")

	var resultado []fragmentoInterno
	type bloque struct {
		tipo        string
		lineaIni    int
		contenido   strings.Builder
		profInicial int
		iniciado    bool
	}
	var actual *bloque
	profundidad := 0

	cerrarBloque := func(lineaFin int) {
		if actual == nil {
			return
		}
		texto := strings.TrimRight(actual.contenido.String(), "\n")
		if strings.TrimSpace(texto) != "" {
			resultado = append(resultado, fragmentoInterno{
				contenido: texto,
				tipo:      actual.tipo,
				lineaIni:  actual.lineaIni,
				lineaFin:  lineaFin,
			})
		}
		actual = nil
	}

	palabrasClave := map[string]string{
		"fn ":         "funcion",
		"pub fn ":     "funcion",
		"pub(crate) fn ": "funcion",
		"struct ":     "estructura",
		"pub struct ": "estructura",
		"enum ":       "estructura",
		"pub enum ":   "estructura",
		"trait ":      "estructura",
		"pub trait ":  "estructura",
		"impl ":       "impl",
	}

	for i, linea := range lineas {
		numLinea := i + 1
		trim := strings.TrimSpace(linea)

		if profundidad == 0 {
			for kw, tipo := range palabrasClave {
				if strings.HasPrefix(trim, kw) {
					cerrarBloque(numLinea - 1)
					actual = &bloque{tipo: tipo, lineaIni: numLinea}
					break
				}
			}
		}

		if actual != nil {
			actual.contenido.WriteString(linea)
			actual.contenido.WriteString("\n")
		}

		abre, cierra := contarLlaves(linea)
		if actual != nil && !actual.iniciado && abre > 0 {
			actual.iniciado = true
			actual.profInicial = profundidad
		}
		profundidad += abre - cierra

		if actual != nil && actual.iniciado && profundidad <= actual.profInicial && numLinea > actual.lineaIni {
			cerrarBloque(numLinea)
		}
	}

	cerrarBloque(len(lineas))

	return resultado
}

// fragmentarJava divide código Java por class, interface, enum, method.
func fragmentarJava(contenido string) []fragmentoInterno {
	lineas := strings.Split(contenido, "\n")

	var resultado []fragmentoInterno
	type bloque struct {
		tipo        string
		lineaIni    int
		contenido   strings.Builder
		profInicial int
		iniciado    bool
	}
	var actual *bloque
	profundidad := 0

	cerrarBloque := func(lineaFin int) {
		if actual == nil {
			return
		}
		texto := strings.TrimRight(actual.contenido.String(), "\n")
		if strings.TrimSpace(texto) != "" {
			resultado = append(resultado, fragmentoInterno{
				contenido: texto,
				tipo:      actual.tipo,
				lineaIni:  actual.lineaIni,
				lineaFin:  lineaFin,
			})
		}
		actual = nil
	}

	palabrasClave := map[string]string{
		"class ":            "estructura",
		"public class ":     "estructura",
		"private class ":    "estructura",
		"protected class ":  "estructura",
		"interface ":        "estructura",
		"public interface ": "estructura",
		"enum ":             "estructura",
		"public enum ":      "estructura",
	}

	// Métodos: cualquier línea con paréntesis y que empiece con un modifier
	modificadores := []string{"public ", "private ", "protected ", "static ", "final ", "void ", "synchronized "}

	for i, linea := range lineas {
		numLinea := i + 1
		trim := strings.TrimSpace(linea)

		// Detectar clases/interfaces/enums
		if profundidad == 0 {
			for kw, tipo := range palabrasClave {
				if strings.HasPrefix(trim, kw) {
					cerrarBloque(numLinea - 1)
					actual = &bloque{tipo: tipo, lineaIni: numLinea}
					break
				}
			}
		}

		// Detectar métodos (cuando estamos dentro de una clase, profundidad 1)
		if actual == nil && profundidad == 1 {
			esMetodo := false
			for _, mod := range modificadores {
				if strings.HasPrefix(trim, mod) && strings.Contains(trim, "(") {
					esMetodo = true
					break
				}
			}
			if esMetodo {
				actual = &bloque{tipo: "funcion", lineaIni: numLinea}
			}
		}

		if actual != nil {
			actual.contenido.WriteString(linea)
			actual.contenido.WriteString("\n")
		}

		abre, cierra := contarLlaves(linea)
		if actual != nil && !actual.iniciado && abre > 0 {
			actual.iniciado = true
			actual.profInicial = profundidad
		}
		profundidad += abre - cierra

		if actual != nil && actual.iniciado && profundidad <= actual.profInicial && numLinea > actual.lineaIni {
			cerrarBloque(numLinea)
		}
	}

	cerrarBloque(len(lineas))

	return resultado
}

// fragmentarC divide código C/C++ por funciones, structs, clases (C++).
func fragmentarC(contenido string) []fragmentoInterno {
	lineas := strings.Split(contenido, "\n")

	var resultado []fragmentoInterno
	type bloque struct {
		tipo        string
		lineaIni    int
		contenido   strings.Builder
		profInicial int
		iniciado    bool
	}
	var actual *bloque
	profundidad := 0

	cerrarBloque := func(lineaFin int) {
		if actual == nil {
			return
		}
		texto := strings.TrimRight(actual.contenido.String(), "\n")
		if strings.TrimSpace(texto) != "" {
			resultado = append(resultado, fragmentoInterno{
				contenido: texto,
				tipo:      actual.tipo,
				lineaIni:  actual.lineaIni,
				lineaFin:  lineaFin,
			})
		}
		actual = nil
	}

	for i, linea := range lineas {
		numLinea := i + 1
		trim := strings.TrimSpace(linea)

		// Detectar structs/unions/enums a nivel superior
		if profundidad == 0 {
			if strings.HasPrefix(trim, "struct ") || strings.HasPrefix(trim, "typedef struct") ||
				strings.HasPrefix(trim, "union ") || strings.HasPrefix(trim, "enum ") {
				cerrarBloque(numLinea - 1)
				actual = &bloque{tipo: "estructura", lineaIni: numLinea}
			} else if strings.Contains(trim, "(") && strings.Contains(trim, ")") &&
				(strings.Contains(trim, "{") || strings.HasSuffix(trim, "{")) &&
				!strings.Contains(trim, "=") && !strings.Contains(trim, ";") {
				// Función a nivel superior: tiene paréntesis y abre bloque
				cerrarBloque(numLinea - 1)
				actual = &bloque{tipo: "funcion", lineaIni: numLinea}
			}
		}

		if actual != nil {
			actual.contenido.WriteString(linea)
			actual.contenido.WriteString("\n")
		}

		abre, cierra := contarLlaves(linea)
		if actual != nil && !actual.iniciado && abre > 0 {
			actual.iniciado = true
			actual.profInicial = profundidad
		}
		profundidad += abre - cierra

		if actual != nil && actual.iniciado && profundidad <= actual.profInicial && numLinea > actual.lineaIni {
			cerrarBloque(numLinea)
		}
	}

	cerrarBloque(len(lineas))

	return resultado
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// contarLlaves cuenta cuántas llaves { y } hay en una línea, ignorando
// las que están dentro de strings y comentarios (versión simple).
func contarLlaves(linea string) (abre, cierra int) {
	enString := byte(0) // 0 = no en string, '"' o '\'' o '`' = en string
	for i := 0; i < len(linea); i++ {
		c := linea[i]
		if enString != 0 {
			if c == enString && (i == 0 || linea[i-1] != '\\') {
				enString = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			enString = c
		case '/':
			if i+1 < len(linea) && linea[i+1] == '/' {
				return // comentario de una línea
			}
		case '{':
			abre++
		case '}':
			cierra++
		}
	}
	return
}

// contiene retorna true si el slice contiene el elemento.
func contiene(slice []string, elem string) bool {
	for _, s := range slice {
		if s == elem {
			return true
		}
	}
	return false
}

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
	case ".js", ".mjs", ".cjs":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".sass", ".less":
		return "css"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc", ".cxx":
		return "cpp"
	case ".sh", ".bash":
		return "shell"
	case ".toml":
		return "toml"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".scala":
		return "scala"
	case ".lua":
		return "lua"
	case ".r":
		return "r"
	case ".sql":
		return "sql"
	default:
		return ""
	}
}
