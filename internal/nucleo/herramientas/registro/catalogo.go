// Package registro implementa el catálogo y métricas de herramientas.
//
// El catálogo es thread-safe y actúa como el punto central de registro
// y lookup de herramientas. Las herramientas integradas se registran
// al iniciar, las auto-creadas (Fase 6) se registran dinámicamente.
//
// Las métricas se recopilan automáticamente en cada Ejecutar() vía
// el catálogo: éxito, fallo, latencia, tokens (si aplica).
package registro

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Catálogo
// ============================================================================

// Catalogo es el registro central de herramientas disponibles.
// Es thread-safe y permite lookup por nombre.
//
// Uso típico:
//
//	cat := registro.NuevoCatalogo()
//	cat.Registrar(&integradas.Terminal{})
//	h, ok := cat.Obtener("terminal")
//	if ok {
//	    res, err := cat.Ejecutar(ctx, "terminal", params)
//	}
type Catalogo struct {
	mu          sync.RWMutex
	herramientas map[string]herramientas.Herramienta
	metricas    *Metricas
	log         func(formato string, args ...interface{})
}

// NuevoCatalogo crea un catálogo vacío.
func NuevoCatalogo() *Catalogo {
	return &Catalogo{
		herramientas: make(map[string]herramientas.Herramienta),
		metricas:     NuevasMetricas(),
		log:          func(string, ...interface{}) {},
	}
}

// ConLog inyecta un logger opcional.
func (c *Catalogo) ConLog(log func(formato string, args ...interface{})) *Catalogo {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.log = log
	return c
}

// Registrar añade una herramienta al catálogo.
// Verifica:
//   - Nombre válido (reglas de ValidarNombre)
//   - Validar() de la herramienta pasa
//   - No haya duplicado (retorna ErrHerramientaDuplicada)
//
// Si la herramienta ya estaba registrada con el mismo nombre, se reemplaza.
// Esto permite hot-reload de herramientas auto-creadas en Fase 6.
func (c *Catalogo) Registrar(h herramientas.Herramienta) error {
	if h == nil {
		return fmt.Errorf("intentó registrar herramienta nil")
	}

	nombre := h.Nombre()
	if err := herramientas.ValidarNombre(nombre); err != nil {
		return err
	}

	if err := h.Validar(); err != nil {
		return &herramientas.ErrHerramientaInvalida{Nombre: nombre,
			Causa: "validación falló: " + err.Error()}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.herramientas[nombre] = h
	c.log("herramienta registrada: %s", nombre)
	return nil
}

// Obtener retorna la herramienta con el nombre dado, o false si no existe.
func (c *Catalogo) Obtener(nombre string) (herramientas.Herramienta, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	h, ok := c.herramientas[nombre]
	return h, ok
}

// Existe verifica si una herramienta está registrada.
func (c *Catalogo) Existe(nombre string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.herramientas[nombre]
	return ok
}

// Eliminar remueve una herramienta del catálogo.
// Retorna true si existía.
func (c *Catalogo) Eliminar(nombre string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.herramientas[nombre]; !ok {
		return false
	}
	delete(c.herramientas, nombre)
	c.log("herramienta eliminada: %s", nombre)
	return true
}

// Listar retorna todas las herramientas registradas, ordenadas por nombre.
func (c *Catalogo) Listar() []herramientas.Herramienta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]herramientas.Herramienta, 0, len(c.herramientas))
	for _, h := range c.herramientas {
		result = append(result, h)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Nombre() < result[j].Nombre()
	})
	return result
}

// Nombres retorna solo los nombres de las herramientas, ordenados.
func (c *Catalogo) Nombres() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0, len(c.herramientas))
	for nombre := range c.herramientas {
		result = append(result, nombre)
	}
	sort.Strings(result)
	return result
}

// Tamaño retorna el número de herramientas registradas.
func (c *Catalogo) Tamaño() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.herramientas)
}

// Ejecutar busca la herramienta por nombre y la ejecuta con los parámetros.
// Registra automáticamente métricas (éxito/fallo, latencia).
// Si la herramienta no existe, retorna ErrHerramientaNoEncontrada.
func (c *Catalogo) Ejecutar(ctx context.Context, nombre string,
	params map[string]interface{}) (herramientas.Resultado, error) {

	h, ok := c.Obtener(nombre)
	if !ok {
		return herramientas.Resultado{
			Exito: false,
			Error: fmt.Sprintf("herramienta '%s' no encontrada", nombre),
		}, &ErrHerramientaNoEncontrada{Nombre: nombre}
	}

	inicio := time.Now()
	res, err := h.Ejecutar(ctx, params)
	duracion := time.Since(inicio)

	// Registrar métricas
	c.metricas.RegistrarEjecucion(nombre, res.Exito, duracion)

	// Agregar duración a metadata del resultado
	if res.Metadata == nil {
		res.Metadata = make(map[string]interface{})
	}
	res.Metadata["duracion_ms"] = float64(duracion.Microseconds()) / 1000.0
	res.Metadata["herramienta"] = nombre

	if err != nil {
		c.log("herramienta %s FALLÓ: %v (%.2fms)", nombre, err,
			float64(duracion.Microseconds())/1000.0)
	} else if !res.Exito {
		c.log("herramienta %s ejecutó sin éxito: %s (%.2fms)", nombre,
			res.Error, float64(duracion.Microseconds())/1000.0)
	} else {
		c.log("herramienta %s OK (%.2fms)", nombre,
			float64(duracion.Microseconds())/1000.0)
	}

	return res, err
}

// Metricas retorna el colector de métricas asociado.
func (c *Catalogo) Metricas() *Metricas {
	return c.metricas
}

// ============================================================================
// Snapshot para serialización
// ============================================================================

// InfoHerramienta es la vista serializable de una herramienta.
type InfoHerramienta struct {
	Nombre      string                   `json:"nombre"`
	Descripcion string                   `json:"descripcion"`
	Parametros  []herramientas.Parametro `json:"parametros"`
}

// Snapshot retorna una vista serializable de todas las herramientas.
func (c *Catalogo) Snapshot() []InfoHerramienta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]InfoHerramienta, 0, len(c.herramientas))
	for _, h := range c.herramientas {
		result = append(result, InfoHerramienta{
			Nombre:      h.Nombre(),
			Descripcion: h.Descripcion(),
			Parametros:  h.Parametros(),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Nombre < result[j].Nombre
	})
	return result
}

// ============================================================================
// Errores
// ============================================================================

// ErrHerramientaNoEncontrada indica que la herramienta no está registrada.
type ErrHerramientaNoEncontrada struct {
	Nombre string
}

func (e *ErrHerramientaNoEncontrada) Error() string {
	return fmt.Sprintf("herramienta '%s' no encontrada en el catálogo", e.Nombre)
}

// ErrHerramientaDuplicada indica que ya existe una herramienta con ese nombre.
type ErrHerramientaDuplicada struct {
	Nombre string
}

func (e *ErrHerramientaDuplicada) Error() string {
	return fmt.Sprintf("herramienta '%s' ya está registrada", e.Nombre)
}
