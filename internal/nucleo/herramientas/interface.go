// Package herramientas define la interfaz estándar que TODA herramienta de Liz
// debe implementar, ya sea una herramienta integrada o una auto-creada.
//
// La interfaz está diseñada para ser:
//   - Contrato verificable en compile-time (interface satisfaction check)
//   - Auto-descriptiva: cada herramienta expone su propio catálogo de parámetros
//   - Validable: Validar() permite verificar que la herramienta está operativa
//     antes de ser registrada
//   - Safe: Ejecutar recibe context.Context para cancellation/timeout
//
// Decisiones de diseño (ver docs/DECISIONES.md D-002):
//   - Patrón Interface Go (no plugins dinámicos, no scripts shell, no gRPC)
//   - Una sola interfaz uniforme para herramientas integradas y auto-creadas
//   - Validación por contrato: si no implementa la interfaz, no se registra
package herramientas

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// Tipos Públicos
// ============================================================================

// Parametro describe un parámetro que una herramienta acepta.
//
// El campo Tipo es un string libre, pero por convención se usan:
//   - "string"  — texto
//   - "int"     — entero
//   - "bool"    — booleano
//   - "float"   — número de coma flotante
//   - "array"   — lista de valores (tipados en Items)
//   - "object"  — mapa/diccionario
//
// El campo Default se aplica cuando el parámetro no viene en el mapa de
// parámetros de Ejecutar. Requerido=true hace que Ejecutar devuelva
// ErrParametroRequerido si el parámetro falta.
type Parametro struct {
	Nombre      string      `json:"nombre"`
	Tipo        string      `json:"tipo"`
	Requerido   bool        `json:"requerido"`
	Default     interface{} `json:"default,omitempty"`
	Descripcion string      `json:"descripcion"`
	// Items define el tipo de elementos cuando Tipo == "array".
	// Vacío significa "cualquier tipo". Ej: "string", "int".
	Items string `json:"items,omitempty"`
	// Opciones restringe los valores aceptados (enum). Vacío = sin restricción.
	Opciones []string `json:"opciones,omitempty"`
	// Min y Max aplican a enteros/floats (rango) o strings (longitud).
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// Resultado es lo que retorna toda herramienta después de ejecutarse.
//
// Convenciones:
//   - Exito=true indica que la operación se completó satisfactoriamente.
//   - Datos contiene el payload específico de la herramienta (output de comando,
//     lista de archivos, métricas, etc.).
//   - Error contiene un mensaje legible cuando Exito=false.
//   - Metadata携带 información adicional estandarizada:
//     "duracion_ms", "tokens_usados", "archivos_afectados", etc.
type Resultado struct {
	Exito    bool                   `json:"exito"`
	Datos    interface{}            `json:"datos,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Herramienta es la interfaz que TODA herramienta en Liz debe implementar.
//
// Compile-time check recomendado en cada implementación:
//
//	var _ herramientas.Herramienta = (*MiHerramienta)(nil)
//
// Las herramientas DEBEN ser seguras para uso concurrente. El catálogo las
// invoca desde múltiples goroutines (handlers HTTP, pipeline de chat, etc.).
type Herramienta interface {
	// Nombre retorna el identificador único de la herramienta.
	// Debe ser corto, en snake_case, sin espacios. Ej: "terminal", "buscador".
	// Dos herramientas no pueden tener el mismo nombre en el catálogo.
	Nombre() string

	// Descripcion retorna una explicación legible de qué hace la herramienta.
	// Se usa en el catálogo que se muestra al modelo y al usuario.
	Descripcion() string

	// Parametros retorna la lista de parámetros aceptados por la herramienta.
	// El catálogo usa esto para generar el JSON schema expuesto en /api/v1/herramientas.
	Parametros() []Parametro

	// Ejecutar invoca la herramienta con los parámetros dados.
	// Recibe context.Context para cancellation/timeout — las herramientas
	// DEBEN respetar ctx.Done() en operaciones largas.
	// El mapa params contiene claves string (nombres de parámetros) y valores
	// de cualquier tipo. La herramienta es responsable de validación y coercion.
	Ejecutar(ctx context.Context, params map[string]interface{}) (Resultado, error)

	// Validar verifica que la herramienta está operativa.
	// Se ejecuta al registrar la herramienta. Debe retornar error si la
	// herramienta no puede funcionar (dependencias faltantes, configuración
	// incorrecta, etc.). No debe tener side-effects.
	Validar() error
}

// ============================================================================
// Errores
// ============================================================================

// ErrParametroRequerido indica que un parámetro requerido no fue proporcionado.
type ErrParametroRequerido struct {
	Nombre string
}

func (e *ErrParametroRequerido) Error() string {
	return fmt.Sprintf("parámetro requerido '%s' no proporcionado", e.Nombre)
}

// ErrTipoParametro indica que un parámetro tiene un tipo incorrecto.
type ErrTipoParametro struct {
	Nombre     string
	Esperado   string
	Obtenido   string
}

func (e *ErrTipoParametro) Error() string {
	return fmt.Sprintf("parámetro '%s' debe ser %s, se obtuvo %s",
		e.Nombre, e.Esperado, e.Obtenido)
}

// ErrValorFueraDeRango indica que un parámetro numérico/string está fuera de rango.
type ErrValorFueraDeRango struct {
	Nombre string
	Min    string
	Max    string
}

func (e *ErrValorFueraDeRango) Error() string {
	return fmt.Sprintf("parámetro '%s' fuera de rango [%s, %s]",
		e.Nombre, e.Min, e.Max)
}

// ErrOpcionInvalida indica que el valor no está en la lista de opciones permitidas.
type ErrOpcionInvalida struct {
	Nombre   string
	Valor    string
	Opciones []string
}

func (e *ErrOpcionInvalida) Error() string {
	return fmt.Sprintf("parámetro '%s' valor '%s' no está en opciones [%s]",
		e.Nombre, e.Valor, strings.Join(e.Opciones, ", "))
}

// ============================================================================
// Helpers de Validación y Coerción
// ============================================================================

// ObtenerString extrae un parámetro string del mapa, aplicando default si falta.
// Retorna ErrParametroRequerido si Requerido=true y el parámetro no está.
// Retorna ErrTipoParametro si el valor existe pero no es string.
func ObtenerString(params map[string]interface{}, p Parametro) (string, error) {
	val, ok := params[p.Nombre]
	if !ok || val == nil {
		if p.Requerido {
			return "", &ErrParametroRequerido{Nombre: p.Nombre}
		}
		if p.Default != nil {
			if s, esString := p.Default.(string); esString {
				return s, nil
			}
		}
		return "", nil
	}

	s, ok := val.(string)
	if !ok {
		// Intentar coercion desde otros tipos numéricos
		if f, esFloat := val.(float64); esFloat {
			return fmt.Sprintf("%v", f), nil
		}
		if i, esInt := val.(int); esInt {
			return fmt.Sprintf("%d", i), nil
		}
		if i, esInt64 := val.(int64); esInt64 {
			return fmt.Sprintf("%d", i), nil
		}
		return "", &ErrTipoParametro{Nombre: p.Nombre, Esperado: "string",
			Obtenido: fmt.Sprintf("%T", val)}
	}

	// Validar opciones (enum)
	if len(p.Opciones) > 0 {
		encontrado := false
		for _, op := range p.Opciones {
			if s == op {
				encontrado = true
				break
			}
		}
		if !encontrado {
			return "", &ErrOpcionInvalida{Nombre: p.Nombre, Valor: s, Opciones: p.Opciones}
		}
	}

	// Validar longitud si Min/Max están definidos (interpretados como longitud)
	if p.Min != nil && float64(len(s)) < *p.Min {
		return "", &ErrValorFueraDeRango{
			Nombre: p.Nombre, Min: fmt.Sprintf("len>=%v", *p.Min), Max: "inf",
		}
	}
	if p.Max != nil && float64(len(s)) > *p.Max {
		return "", &ErrValorFueraDeRango{
			Nombre: p.Nombre, Min: "0", Max: fmt.Sprintf("len<=%v", *p.Max),
		}
	}

	return s, nil
}

// ObtenerInt extrae un parámetro entero. Acepta int, int64, float64 (json) y
// string numérico.
func ObtenerInt(params map[string]interface{}, p Parametro) (int, error) {
	val, ok := params[p.Nombre]
	if !ok || val == nil {
		if p.Requerido {
			return 0, &ErrParametroRequerido{Nombre: p.Nombre}
		}
		if p.Default != nil {
			if i, esInt := p.Default.(int); esInt {
				return i, nil
			}
			if f, esFloat := p.Default.(float64); esFloat {
				return int(f), nil
			}
		}
		return 0, nil
	}

	var n int
	switch v := val.(type) {
	case int:
		n = v
	case int64:
		n = int(v)
	case float64:
		n = int(v)
	case string:
		var parsed int
		_, err := fmt.Sscanf(v, "%d", &parsed)
		if err != nil {
			return 0, &ErrTipoParametro{Nombre: p.Nombre, Esperado: "int",
				Obtenido: "string no numérico"}
		}
		n = parsed
	default:
		return 0, &ErrTipoParametro{Nombre: p.Nombre, Esperado: "int",
			Obtenido: fmt.Sprintf("%T", val)}
	}

	// Validar rango
	if p.Min != nil && float64(n) < *p.Min {
		return 0, &ErrValorFueraDeRango{
			Nombre: p.Nombre, Min: fmt.Sprintf("%v", *p.Min), Max: "inf",
		}
	}
	if p.Max != nil && float64(n) > *p.Max {
		return 0, &ErrValorFueraDeRango{
			Nombre: p.Nombre, Min: "-inf", Max: fmt.Sprintf("%v", *p.Max),
		}
	}

	return n, nil
}

// ObtenerBool extrae un parámetro booleano. Acepta bool o string "true"/"false".
func ObtenerBool(params map[string]interface{}, p Parametro) (bool, error) {
	val, ok := params[p.Nombre]
	if !ok || val == nil {
		if p.Requerido {
			return false, &ErrParametroRequerido{Nombre: p.Nombre}
		}
		if p.Default != nil {
			if b, esBool := p.Default.(bool); esBool {
				return b, nil
			}
		}
		return false, nil
	}

	switch v := val.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(v) {
		case "true", "1", "yes", "si", "sí":
			return true, nil
		case "false", "0", "no":
			return false, nil
		}
		return false, &ErrTipoParametro{Nombre: p.Nombre, Esperado: "bool",
			Obtenido: fmt.Sprintf("string %q", v)}
	default:
		return false, &ErrTipoParametro{Nombre: p.Nombre, Esperado: "bool",
			Obtenido: fmt.Sprintf("%T", val)}
	}
}

// ObtenerArrayString extrae un parámetro array de strings.
// Acepta []string, []interface{} (cada elemento se convierte con fmt.Sprint).
func ObtenerArrayString(params map[string]interface{}, p Parametro) ([]string, error) {
	val, ok := params[p.Nombre]
	if !ok || val == nil {
		if p.Requerido {
			return nil, &ErrParametroRequerido{Nombre: p.Nombre}
		}
		if p.Default != nil {
			if arr, esArray := p.Default.([]string); esArray {
				return arr, nil
			}
			if arr, esArray := p.Default.([]interface{}); esArray {
				result := make([]string, 0, len(arr))
				for _, v := range arr {
					result = append(result, fmt.Sprint(v))
				}
				return result, nil
			}
		}
		return nil, nil
	}

	switch v := val.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprint(item))
		}
		return result, nil
	default:
		return nil, &ErrTipoParametro{Nombre: p.Nombre, Esperado: "array",
			Obtenido: fmt.Sprintf("%T", val)}
	}
}

// ObtenerFloat extrae un parámetro float. Acepta float64, int, int64, string numérico.
func ObtenerFloat(params map[string]interface{}, p Parametro) (float64, error) {
	val, ok := params[p.Nombre]
	if !ok || val == nil {
		if p.Requerido {
			return 0, &ErrParametroRequerido{Nombre: p.Nombre}
		}
		if p.Default != nil {
			if f, esFloat := p.Default.(float64); esFloat {
				return f, nil
			}
			if i, esInt := p.Default.(int); esInt {
				return float64(i), nil
			}
		}
		return 0, nil
	}

	var n float64
	switch v := val.(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case string:
		var parsed float64
		_, err := fmt.Sscanf(v, "%f", &parsed)
		if err != nil {
			return 0, &ErrTipoParametro{Nombre: p.Nombre, Esperado: "float",
				Obtenido: "string no numérico"}
		}
		n = parsed
	default:
		return 0, &ErrTipoParametro{Nombre: p.Nombre, Esperado: "float",
			Obtenido: fmt.Sprintf("%T", val)}
	}

	if p.Min != nil && n < *p.Min {
		return 0, &ErrValorFueraDeRango{
			Nombre: p.Nombre, Min: fmt.Sprintf("%v", *p.Min), Max: "inf",
		}
	}
	if p.Max != nil && n > *p.Max {
		return 0, &ErrValorFueraDeRango{
			Nombre: p.Nombre, Min: "-inf", Max: fmt.Sprintf("%v", *p.Max),
		}
	}

	return n, nil
}

// ============================================================================
// Metadata Helpers
// ============================================================================

// NuevaMetadata crea un map de metadata con la duración pre-establecida.
func NuevaMetadata(duracionMs float64) map[string]interface{} {
	return map[string]interface{}{
		"duracion_ms": duracionMs,
	}
}

// ErrHerramientaInvalida indica que una herramienta no pasó Validar().
type ErrHerramientaInvalida struct {
	Nombre string
	Causa  string
}

func (e *ErrHerramientaInvalida) Error() string {
	return fmt.Sprintf("herramienta '%s' inválida: %s", e.Nombre, e.Causa)
}

// ValidarNombre verifica que un nombre de herramienta cumple las reglas:
//   - No vacío
//   - Solo letras minúsculas, dígitos, guiones bajos
//   - Entre 2 y 64 caracteres
func ValidarNombre(nombre string) error {
	if len(nombre) < 2 || len(nombre) > 64 {
		return &ErrHerramientaInvalida{Nombre: nombre,
			Causa: "longitud debe estar entre 2 y 64 caracteres"}
	}
	for _, r := range nombre {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_' {
			return &ErrHerramientaInvalida{Nombre: nombre,
				Causa: fmt.Sprintf("carácter inválido %q (solo a-z, 0-9, _)", r)}
		}
	}
	return nil
}
