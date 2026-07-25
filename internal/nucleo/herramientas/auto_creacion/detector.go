package auto_creacion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
)

// ============================================================================
// Interfaz LLM (para testabilidad)
// ============================================================================

// ClienteLLM es la abstracción mínima que el Detector y Generador necesitan
// para hablar con el orquestador. Cualquier tipo que implemente este método
// sirve (en producción: *orquestador.Orquestador; en tests: un mock).
type ClienteLLM interface {
	// Completar envía una solicitud de chat y retorna la respuesta completa.
	Completar(req orquestador.SolicitudChat) (*orquestador.RespuestaChat, error)
}

// ============================================================================
// Detector — identifica qué herramientas nuevas se necesitan
// ============================================================================

// Detector analiza una petición del usuario + el catálogo actual y devuelve
// la lista de herramientas nuevas que Liz debe crear.
//
// Usa un ClienteLLM (típicamente el Orquestador NVIDIA) para hacer el análisis.
// El prompt instruye al modelo para:
//   - Solo sugerir herramientas realmente faltantes
//   - Definir nombre, descripción, parámetros con tipo
//   - Justificar la necesidad
//   - Máximo 5 herramientas por detección
type Detector struct {
	llm     ClienteLLM
	logFunc func(formato string, args ...interface{})
}

// NuevoDetector crea un Detector que usa el LLM dado.
func NuevoDetector(llm ClienteLLM) *Detector {
	return &Detector{
		llm:     llm,
		logFunc: func(string, ...interface{}) {},
	}
}

// ConLog inyecta un logger opcional.
func (d *Detector) ConLog(fn func(formato string, args ...interface{})) *Detector {
	if fn != nil {
		d.logFunc = fn
	}
	return d
}

// Detectar analiza la petición del usuario y retorna la lista de herramientas
// nuevas que se necesitan crear.
//
// Si el LLM no está disponible (api key no configurada, red caída, etc.),
// retorna error y el caller debe decidir cómo manejarlo (típicamente: responder
// al usuario que la auto-creación no está disponible).
func (d *Detector) Detectar(ctx context.Context, descripcion string, catalogo []InfoCatalogo) (*ResultadoDeteccion, error) {
	if d.llm == nil {
		return nil, &ErrAutoCreacion{Etapa: "deteccion", Causa: "cliente LLM no configurado"}
	}
	if strings.TrimSpace(descripcion) == "" {
		return nil, &ErrAutoCreacion{Etapa: "deteccion", Causa: "descripción vacía"}
	}

	prompt := PlantillaPromptDeteccion(descripcion, catalogo)
	d.logFunc("enviando prompt de detección al LLM (%d chars)", len(prompt))

	sol := orquestador.SolicitudChat{
		Tarea: orquestador.TareaAnalisis,
		Mensajes: []orquestador.MensajeChat{
			{
				Rol:       "system",
				Contenido: "Eres un analista experto en sistemas Linux y herramientas CLI. Respondes SIEMPRE en formato JSON válido dentro de un bloque ```json ... ```.",
			},
			{
				Rol:       "user",
				Contenido: prompt,
			},
		},
		Temperatura: 0.2, // baja temperatura para análisis determinista
		MaxTokens:   2048,
	}

	resp, err := d.llm.Completar(sol)
	if err != nil {
		return nil, &ErrAutoCreacion{
			Etapa: "deteccion", Causa: "LLM falló", Interno: err,
		}
	}
	if resp == nil || resp.Contenido == "" {
		return nil, &ErrAutoCreacion{Etapa: "deteccion", Causa: "LLM retornó respuesta vacía"}
	}

	d.logFunc("LLM respondió (%d chars, modelo: %s, tokens: %d)",
		len(resp.Contenido), resp.ModeloUsado, resp.TokensTotal)

	resultado, err := parsearResultadoDeteccion(resp.Contenido)
	if err != nil {
		return nil, &ErrAutoCreacion{
			Etapa: "deteccion", Causa: "parseando respuesta del LLM", Interno: err,
		}
	}

	resultado.ModeloUsado = resp.ModeloUsado

	// Validar y normalizar cada spec detectada
	for i := range resultado.Faltantes {
		if err := normalizarSpec(&resultado.Faltantes[i]); err != nil {
			d.logFunc("WARN: spec %d inválida, saltando: %v", i, err)
			// Marcar como inválida eliminándola más tarde
			resultado.Faltantes[i].Nombre = ""
		}
	}

	// Filtrar las marcadas como inválidas
	validas := make([]SpecHerramienta, 0, len(resultado.Faltantes))
	for _, s := range resultado.Faltantes {
		if s.Nombre != "" {
			validas = append(validas, s)
		}
	}
	resultado.Faltantes = validas

	d.logFunc("detección completa: %d herramientas faltantes", len(resultado.Faltantes))
	return resultado, nil
}

// ============================================================================
// Parsing del JSON retornado por el LLM
// ============================================================================

// regexJSONBlock extrae JSON dentro de bloques ```json ... ```
var regexJSONBlock = regexpCompileJSON()

func regexpCompileJSON() func(string) string {
	// Compilado en función para evitar inicialización global con regex
	// (mantener el orden de inicialización claro).
	return func(s string) string {
		// Buscar ```json ... ``` o ``` ... ```
		start := strings.Index(s, "```json")
		if start < 0 {
			start = strings.Index(s, "```")
		}
		if start < 0 {
			return s
		}
		// Mover al inicio del contenido
		start = strings.Index(s[start:], "\n")
		if start < 0 {
			return s
		}
		start += 1
		// Buscar el cierre
		end := strings.Index(s[start:], "```")
		if end < 0 {
			return s
		}
		return strings.TrimSpace(s[start : start+end])
	}
}

// parsearResultadoDeteccion extrae el JSON del output del LLM y lo convierte
// en ResultadoDeteccion.
func parsearResultadoDeteccion(raw string) (*ResultadoDeteccion, error) {
	jsonStr := regexJSONBlock(raw)

	// Si no había bloque markdown, intentar parsear directamente
	var resultado ResultadoDeteccion
	if err := json.Unmarshal([]byte(jsonStr), &resultado); err != nil {
		// Intento: extraer el primer { ... } de la respuesta
		jsonStr = extraerPrimerJSON(raw)
		if jsonStr == "" {
			return nil, fmt.Errorf("no se encontró JSON válido en la respuesta: %s",
				truncar(raw, 200))
		}
		if err := json.Unmarshal([]byte(jsonStr), &resultado); err != nil {
			return nil, fmt.Errorf("JSON inválido: %w", err)
		}
	}
	return &resultado, nil
}

// extraerPrimerJSON encuentra el primer { ... } balanceado en el string.
func extraerPrimerJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	profundidad := 0
	enString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			enString = !enString
			continue
		}
		if enString {
			continue
		}
		if c == '{' {
			profundidad++
		} else if c == '}' {
			profundidad--
			if profundidad == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// normalizarSpec valida y rellena campos por defecto de una spec.
func normalizarSpec(s *SpecHerramienta) error {
	s.Nombre = strings.ToLower(strings.TrimSpace(s.Nombre))
	s.Descripcion = strings.TrimSpace(s.Descripcion)
	s.Categoria = strings.TrimSpace(s.Categoria)
	s.Razon = strings.TrimSpace(s.Razon)

	if s.Nombre == "" {
		return fmt.Errorf("nombre vacío")
	}
	if err := herramientas.ValidarNombre(s.Nombre); err != nil {
		return fmt.Errorf("nombre inválido: %w", err)
	}
	if s.Descripcion == "" {
		return fmt.Errorf("descripción vacía para %s", s.Nombre)
	}
	if s.Categoria == "" {
		s.Categoria = "otro"
	}

	// Normalizar parámetros
	for i := range s.Parametros {
		p := &s.Parametros[i]
		p.Nombre = strings.TrimSpace(p.Nombre)
		p.Tipo = strings.TrimSpace(p.Tipo)
		p.Descripcion = strings.TrimSpace(p.Descripcion)
		if p.Tipo == "" {
			p.Tipo = "string"
		}
	}

	return nil
}

// truncar recorta un string a n caracteres con elipsis.
func truncar(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
