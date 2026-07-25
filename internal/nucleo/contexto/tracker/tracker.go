// Package tracker implementa un registro de ediciones de archivos (ring buffer).
//
// Mantiene las últimas N rutas de archivos editados, con persistencia JSON.
// Se usa en el empaquetador de contexto para la capa 4 (locality bias).
package tracker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RegistroEdicion representa una edición de archivo.
type RegistroEdicion struct {
	Ruta      string `json:"ruta"`
	Timestamp string `json:"timestamp"`
}

// TrackerEdiciones mantiene un ring buffer de las últimas N ediciones.
type TrackerEdiciones struct {
	mu        sync.RWMutex
	maxItems  int
	ediciones []RegistroEdicion
	logFunc   func(string, ...interface{})
}

// NuevoTracker crea un tracker con el límite dado (default 20).
func NuevoTracker(max int) *TrackerEdiciones {
	if max <= 0 {
		max = 20
	}
	return &TrackerEdiciones{
		maxItems:  max,
		ediciones: make([]RegistroEdicion, 0, max),
	}
}

// ConLog asigna función de log.
func (t *TrackerEdiciones) ConLog(fn func(string, ...interface{})) *TrackerEdiciones {
	if fn != nil {
		t.logFunc = fn
	}
	return t
}

// RegistrarEdicion registra una edición de archivo.
func (t *TrackerEdiciones) RegistrarEdicion(ruta string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	reg := RegistroEdicion{
		Ruta:      ruta,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	t.ediciones = append(t.ediciones, reg)

	// Ring buffer: eliminar el más viejo si excede el límite
	if len(t.ediciones) > t.maxItems {
		t.ediciones = t.ediciones[len(t.ediciones)-t.maxItems:]
	}
}

// ObtenerRecientes retorna las últimas N ediciones (más recientes primero).
func (t *TrackerEdiciones) ObtenerRecientes(n int) []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if n <= 0 || len(t.ediciones) == 0 {
		return nil
	}
	if n > len(t.ediciones) {
		n = len(t.ediciones)
	}

	resultado := make([]string, n)
	for i := 0; i < n; i++ {
		resultado[i] = t.ediciones[len(t.ediciones)-1-i].Ruta
	}
	return resultado
}

// Total retorna el número de ediciones registradas.
func (t *TrackerEdiciones) Total() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.ediciones)
}

// Guardar persiste el tracker a disco.
func (t *TrackerEdiciones) Guardar(rutaArchivo string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(rutaArchivo), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t.ediciones, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando tracker: %w", err)
	}
	tmp := rutaArchivo + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("escribiendo tracker: %w", err)
	}
	return os.Rename(tmp, rutaArchivo)
}

// Cargar carga el tracker desde disco.
func (t *TrackerEdiciones) Cargar(rutaArchivo string) error {
	data, err := os.ReadFile(rutaArchivo)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no hay tracker previo
		}
		return fmt.Errorf("leyendo tracker: %w", err)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if err := json.Unmarshal(data, &t.ediciones); err != nil {
		return fmt.Errorf("parseando tracker: %w", err)
	}

	// Aplicar límite
	if len(t.ediciones) > t.maxItems {
		t.ediciones = t.ediciones[len(t.ediciones)-t.maxItems:]
	}
	return nil
}
