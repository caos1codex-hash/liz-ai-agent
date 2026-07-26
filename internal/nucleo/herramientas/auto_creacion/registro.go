package auto_creacion

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

// ============================================================================
// Registro — persistencia de herramientas auto-creadas
// ============================================================================

// Registro maneja la persistencia en disco de las herramientas auto-creadas.
//
// Estructura en disco (en dirRaiz, típicamente ~/.liz/herramientas/auto_creadas/):
//
//	dirRaiz/
//	├── registro.json              # índice global: lista de nombres + versión
//	├── {nombre1}/
//	│   ├── fuente.go              # código fuente Go
//	│   ├── herramienta            # binario compilado
//	│   ├── metadata.json          # spec + timestamps + estadísticas
//	│   └── compilacion.log        # log de última compilación
//	├── {nombre2}/
//	└── ...
//
// El registro.json es solo un índice para lookup rápido; la metadata
// completa vive en cada herramienta. Si se pierde el índice, se puede
// reconstruir escaneando los subdirectorios.
type Registro struct {
	mu      sync.RWMutex
	dirRaiz string
	logFunc func(formato string, args ...interface{})
}

// NuevoRegistro crea un Registro que persiste en dirRaiz.
// El directorio se crea si no existe.
func NuevoRegistro(dirRaiz string) (*Registro, error) {
	if dirRaiz == "" {
		return nil, fmt.Errorf("dirRaiz vacío")
	}
	if err := os.MkdirAll(dirRaiz, 0o755); err != nil {
		return nil, fmt.Errorf("creando dir raíz %s: %w", dirRaiz, err)
	}
	return &Registro{
		dirRaiz: dirRaiz,
		logFunc: func(string, ...interface{}) {},
	}, nil
}

// ConLog inyecta un logger opcional.
func (r *Registro) ConLog(fn func(formato string, args ...interface{})) *Registro {
	if fn != nil {
		r.logFunc = fn
	}
	return r
}

// ============================================================================
// Operaciones de metadata
// ============================================================================

// Guardar persiste la metadata de una herramienta en su directorio.
// Crea el directorio si no existe.
func (r *Registro) Guardar(meta *MetadataHerramienta) error {
	if meta == nil {
		return fmt.Errorf("metadata nil")
	}
	if meta.Nombre == "" {
		return fmt.Errorf("metadata sin nombre")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	dirHerr := r.dirHerramienta(meta.Nombre)
	if err := os.MkdirAll(dirHerr, 0o755); err != nil {
		return fmt.Errorf("creando dir de herramienta: %w", err)
	}

	rutaMeta := filepath.Join(dirHerr, "metadata.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(rutaMeta, data, 0o644); err != nil {
		return fmt.Errorf("escribiendo metadata.json: %w", err)
	}

	// Actualizar índice global
	if err := r.actualizarIndiceLocked(meta.Nombre); err != nil {
		r.logFunc("WARN: no se pudo actualizar índice: %v", err)
	}

	r.logFunc("metadata guardada: %s (v%d)", meta.Nombre, meta.VersionContador)
	return nil
}

// Obtener lee la metadata de una herramienta. Retorna error si no existe.
func (r *Registro) Obtener(nombre string) (*MetadataHerramienta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.obtenerLocked(nombre)
}

// obtenerLocked lee metadata asumiendo que ya se tiene el lock de lectura.
func (r *Registro) obtenerLocked(nombre string) (*MetadataHerramienta, error) {
	rutaMeta := filepath.Join(r.dirHerramienta(nombre), "metadata.json")
	data, err := os.ReadFile(rutaMeta)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ErrHerramientaNoCreada{Nombre: nombre}
		}
		return nil, fmt.Errorf("leyendo metadata: %w", err)
	}
	var meta MetadataHerramienta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return &meta, nil
}

// Existe verifica si una herramienta está registrada.
func (r *Registro) Existe(nombre string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rutaMeta := filepath.Join(r.dirHerramienta(nombre), "metadata.json")
	_, err := os.Stat(rutaMeta)
	return err == nil
}

// Listar retorna la metadata de todas las herramientas registradas.
func (r *Registro) Listar() ([]*MetadataHerramienta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entradas, err := os.ReadDir(r.dirRaiz)
	if err != nil {
		return nil, fmt.Errorf("leyendo dir raíz: %w", err)
	}

	result := make([]*MetadataHerramienta, 0, len(entradas))
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		nombre := e.Name()
		// Saltar directorios ocultos
		if strings.HasPrefix(nombre, ".") {
			continue
		}
		meta, err := r.obtenerLocked(nombre)
		if err != nil {
			r.logFunc("WARN: no se pudo leer metadata de %s: %v", nombre, err)
			continue
		}
		result = append(result, meta)
	}

	// Ordenar por nombre
	sort.Slice(result, func(i, j int) bool {
		return result[i].Nombre < result[j].Nombre
	})

	return result, nil
}

// Eliminar borra una herramienta y todos sus artifacts (fuente, binario, metadata).
// Retorna error si no existe.
func (r *Registro) Eliminar(nombre string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dirHerr := r.dirHerramienta(nombre)
	if _, err := os.Stat(dirHerr); err != nil {
		if os.IsNotExist(err) {
			return &ErrHerramientaNoCreada{Nombre: nombre}
		}
		return fmt.Errorf("stat dir herramienta: %w", err)
	}

	if err := os.RemoveAll(dirHerr); err != nil {
		return fmt.Errorf("eliminando dir: %w", err)
	}

	// Actualizar índice
	if err := r.reconstruirIndiceLocked(); err != nil {
		r.logFunc("WARN: no se pudo reconstruir índice: %v", err)
	}

	r.logFunc("herramienta eliminada: %s", nombre)
	return nil
}

// ============================================================================
// Operaciones de artifacts (fuente, binario, log)
// ============================================================================

// GuardarFuente escribe el fuente.go de una herramienta en su directorio.
func (r *Registro) GuardarFuente(nombre, fuente string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dirHerr := r.dirHerramienta(nombre)
	if err := os.MkdirAll(dirHerr, 0o755); err != nil {
		return err
	}
	ruta := filepath.Join(dirHerr, "fuente.go")
	if err := os.WriteFile(ruta, []byte(fuente), 0o644); err != nil {
		return fmt.Errorf("escribiendo fuente.go: %w", err)
	}
	return nil
}

// LeerFuente retorna el fuente.go de una herramienta.
func (r *Registro) LeerFuente(nombre string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ruta := filepath.Join(r.dirHerramienta(nombre), "fuente.go")
	data, err := os.ReadFile(ruta)
	if err != nil {
		return "", fmt.Errorf("leyendo fuente.go: %w", err)
	}
	return string(data), nil
}

// RutaBinario retorna la ruta absoluta al binario de una herramienta.
// No verifica que exista; el caller debe hacerlo.
func (r *Registro) RutaBinario(nombre string) string {
	return filepath.Join(r.dirHerramienta(nombre), "herramienta")
}

// RutaDirectorio retorna la ruta absoluta al directorio de una herramienta.
func (r *Registro) RutaDirectorio(nombre string) string {
	return r.dirHerramienta(nombre)
}

// BinarioExiste verifica si el binario compilado existe.
func (r *Registro) BinarioExiste(nombre string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ruta := r.RutaBinario(nombre)
	info, err := os.Stat(ruta)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// LeerLogCompilacion retorna el contenido del último compilacion.log.
func (r *Registro) LeerLogCompilacion(nombre string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ruta := filepath.Join(r.dirHerramienta(nombre), "compilacion.log")
	data, err := os.ReadFile(ruta)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// ============================================================================
// Estadísticas (llamado por el Cargador tras cada ejecución)
// ============================================================================

// IncrementarEstadisticas actualiza los contadores de uso de una herramienta.
// Se llama desde el Gestor tras cada Ejecutar (no desde el Cargador para
// mantener el Registro como única fuente de verdad persistente).
func (r *Registro) IncrementarEstadisticas(nombre string, exito bool, errStr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta, err := r.obtenerLocked(nombre)
	if err != nil {
		return err
	}

	meta.VecesEjecutada++
	if exito {
		meta.VecesExitosas++
		meta.UltimoError = ""
	} else {
		meta.UltimoError = errStr
	}

	// Reescribir metadata
	rutaMeta := filepath.Join(r.dirHerramienta(nombre), "metadata.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rutaMeta, data, 0o644)
}

// ============================================================================
// Helpers internos
// ============================================================================

// dirHerramienta retorna la ruta absoluta al directorio de una herramienta.
func (r *Registro) dirHerramienta(nombre string) string {
	return filepath.Join(r.dirRaiz, nombre)
}

// actualizarIndiceLocked añade/actualiza un nombre en el índice global.
func (r *Registro) actualizarIndiceLocked(nombre string) error {
	indice, err := r.leerIndiceLocked()
	if err != nil {
		// Si el índice no existe, crear uno nuevo
		indice = &indiceRegistro{Version: 1, Herramientas: []string{}}
	}

	// Añadir si no existe
	encontrado := false
	for _, n := range indice.Herramientas {
		if n == nombre {
			encontrado = true
			break
		}
	}
	if !encontrado {
		indice.Herramientas = append(indice.Herramientas, nombre)
		sort.Strings(indice.Herramientas)
	}
	indice.Actualizado = time.Now()

	return r.escribirIndiceLocked(indice)
}

// reconstruirIndiceLocked recrea el índice escaneando los subdirectorios.
func (r *Registro) reconstruirIndiceLocked() error {
	entradas, err := os.ReadDir(r.dirRaiz)
	if err != nil {
		return err
	}

	nombres := make([]string, 0, len(entradas))
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		// Verificar que tenga metadata.json
		rutaMeta := filepath.Join(r.dirRaiz, e.Name(), "metadata.json")
		if _, err := os.Stat(rutaMeta); err == nil {
			nombres = append(nombres, e.Name())
		}
	}
	sort.Strings(nombres)

	indice := &indiceRegistro{
		Version:      1,
		Herramientas: nombres,
		Actualizado:  time.Now(),
	}
	return r.escribirIndiceLocked(indice)
}

// indiceRegistro es el formato del archivo registro.json.
type indiceRegistro struct {
	Version      int       `json:"version"`
	Herramientas []string  `json:"herramientas"`
	Actualizado  time.Time `json:"actualizado"`
}

// leerIndiceLocked lee el índice global.
func (r *Registro) leerIndiceLocked() (*indiceRegistro, error) {
	ruta := filepath.Join(r.dirRaiz, "registro.json")
	data, err := os.ReadFile(ruta)
	if err != nil {
		return nil, err
	}
	var indice indiceRegistro
	if err := json.Unmarshal(data, &indice); err != nil {
		return nil, err
	}
	return &indice, nil
}

// escribirIndiceLocked escribe el índice global.
func (r *Registro) escribirIndiceLocked(indice *indiceRegistro) error {
	ruta := filepath.Join(r.dirRaiz, "registro.json")
	data, err := json.MarshalIndent(indice, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, data, 0o644)
}

// ============================================================================
// Errores
// ============================================================================

// ErrHerramientaNoCreada indica que la herramienta no está en el registro.
type ErrHerramientaNoCreada struct {
	Nombre string
}

func (e *ErrHerramientaNoCreada) Error() string {
	return fmt.Sprintf("herramienta auto-creada '%s' no encontrada en el registro", e.Nombre)
}
