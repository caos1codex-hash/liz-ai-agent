package auto_creacion

import (
	"context"
	"fmt"
	"strings"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
)

// ============================================================================
// Generador — produce código Go para una herramienta dada su spec
// ============================================================================

// Generador toma la spec de una herramienta y usa el LLM para generar el
// código Go completo (package main, solo stdlib, siguiendo el protocolo
// subprocess de Liz).
//
// El flujo:
//  1. Construye un prompt con la spec + protocolo + ejemplo
//  2. Llama al LLM vía el Orquestador
//  3. Extrae el código Go de la respuesta (bloque markdown o texto plano)
//  4. Valida que tenga package main y func main()
//  5. Inyecta un header con metadata
//  6. Retorna el fuente listo para compilar
type Generador struct {
	llm     ClienteLLM
	logFunc func(formato string, args ...interface{})
}

// NuevoGenerador crea un Generador que usa el LLM dado.
func NuevoGenerador(llm ClienteLLM) *Generador {
	return &Generador{
		llm:     llm,
		logFunc: func(string, ...interface{}) {},
	}
}

// ConLog inyecta un logger opcional.
func (g *Generador) ConLog(fn func(formato string, args ...interface{})) *Generador {
	if fn != nil {
		g.logFunc = fn
	}
	return g
}

// Generar produce el código Go de la herramienta descrita por spec.
//
// Retorna el fuente como string + metadatos (modelo usado, tokens).
// Si el LLM falla o el código no cumple los requisitos mínimos, retorna error.
func (g *Generador) Generar(ctx context.Context, spec SpecHerramienta) (*ResultadoGeneracion, error) {
	if g.llm == nil {
		return nil, &ErrAutoCreacion{Etapa: "generacion", Causa: "cliente LLM no configurado"}
	}
	if err := normalizarSpec(&spec); err != nil {
		return nil, &ErrAutoCreacion{Etapa: "generacion", Causa: "spec inválida", Interno: err}
	}

	prompt := PlantillaPrompt(spec)
	g.logFunc("enviando prompt de generación al LLM (%d chars, tool: %s)",
		len(prompt), spec.Nombre)

	sol := orquestador.SolicitudChat{
		Tarea: orquestador.TareaCodigo,
		Mensajes: []orquestador.MensajeChat{
			{
				Rol: "system",
				Contenido: "Eres un ingeniero Go senior experto en herramientas CLI y sistemas Linux. " +
					"Generas código Go limpio, idiomático y siempre compilable. " +
					"Respondes SIEMPRE con código Go dentro de un bloque ```go ... ```.",
			},
			{
				Rol:       "user",
				Contenido: prompt,
			},
		},
		Temperatura: 0.3, // algo creativo pero mayormente determinista
		MaxTokens:   4096,
	}

	resp, err := g.llm.Completar(sol)
	if err != nil {
		return nil, &ErrAutoCreacion{
			Etapa: "generacion", Causa: "LLM falló", Interno: err,
		}
	}
	if resp == nil || resp.Contenido == "" {
		return nil, &ErrAutoCreacion{Etapa: "generacion", Causa: "LLM retornó respuesta vacía"}
	}

	g.logFunc("LLM respondió (%d chars, modelo: %s, tokens: %d)",
		len(resp.Contenido), resp.ModeloUsado, resp.TokensTotal)

	// Extraer código Go de la respuesta
	fuente := ExtraerFuenteGo(resp.Contenido)

	// Validar estructura mínima
	if err := ValidarFuenteGo(fuente); err != nil {
		return nil, &ErrAutoCreacion{
			Etapa:   "generacion",
			Causa:   "código generado no cumple requisitos mínimos",
			Interno: err,
		}
	}

	// Inyectar header con metadata
	fuente = InyectarHeader(fuente, spec, resp.ModeloUsado)

	g.logFunc("fuente generado: %d chars, válido, listo para compilar", len(fuente))

	return &ResultadoGeneracion{
		FuenteGo:         fuente,
		ModeloUsado:      resp.ModeloUsado,
		TokensConsumidos: resp.TokensTotal,
	}, nil
}

// ============================================================================
// Generación sin LLM (para tests / fallback)
// ============================================================================

// GenerarDesdePlantilla construye una herramienta mínima a partir de la spec
// SIN usar el LLM. Útil para:
//   - Tests unitarios del Compilador (no requieren API key)
//   - Fallback cuando el LLM no está disponible
//   - Crear herramientas "shell" que solo envuelven un comando
//
// La herramienta resultante solo retorna info/validar correctamente; en
// ejecutar retorna "no implementado" — el caller puede luego regenerar
// con LLM. Esto garantiza que el flujo compile→carga→registro se pueda
// probar end-to-end sin LLM.
func GenerarDesdePlantilla(spec SpecHerramienta) (*ResultadoGeneracion, error) {
	if err := normalizarSpec(&spec); err != nil {
		return nil, &ErrAutoCreacion{Etapa: "generacion", Causa: "spec inválida", Interno: err}
	}

	fuente := construirFuenteStub(spec)
	return &ResultadoGeneracion{
		FuenteGo:         fuente,
		ModeloUsado:      "stub-local",
		TokensConsumidos: 0,
	}, nil
}

// construirFuenteStub genera un programa Go mínimo y compilable para la spec
// dada. Implementa info y validar correctamente; ejecutar retorna un mensaje
// indicando que es un stub.
func construirFuenteStub(spec SpecHerramienta) string {
	var paramsDecl strings.Builder
	for i, p := range spec.Parametros {
		paramsDecl.WriteString("        {Nombre: ")
		paramsDecl.WriteString(fmt.Sprintf("%q", p.Nombre))
		paramsDecl.WriteString(", Tipo: ")
		paramsDecl.WriteString(fmt.Sprintf("%q", p.Tipo))
		paramsDecl.WriteString(", Requerido: ")
		paramsDecl.WriteString(fmt.Sprintf("%v", p.Requerido))
		paramsDecl.WriteString(", Descripcion: ")
		paramsDecl.WriteString(fmt.Sprintf("%q", p.Descripcion))
		if p.Default != nil {
			paramsDecl.WriteString(", Default: ")
			paramsDecl.WriteString(fmt.Sprintf("%q", fmt.Sprintf("%v", p.Default)))
		}
		paramsDecl.WriteString("}")
		if i < len(spec.Parametros)-1 {
			paramsDecl.WriteString(",\n")
		} else {
			paramsDecl.WriteString(",\n")
		}
	}

	header := fmt.Sprintf(`// ============================================================================
// Herramienta auto-creada por Liz (Fase 6) — STUB local
//   Nombre:    %s
//   Categoría: %s
//   Generada sin LLM (fallback). Recompile con /recargar para usar LLM.
// ============================================================================

`, spec.Nombre, spec.Categoria)

	source := header + fmt.Sprintf(`package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type solicitud struct {
	Operacion  string                 `+"`json:\"operacion\"`"+`
	Parametros map[string]interface{} `+"`json:\"parametros\"`"+`
}

type respuesta struct {
	Exito    bool                   `+"`json:\"exito\"`"+`
	Datos    interface{}            `+"`json:\"datos,omitempty\"`"+`
	Error    string                 `+"`json:\"error,omitempty\"`"+`
	Metadata map[string]interface{} `+"`json:\"metadata,omitempty\"`"+`
}

type parametro struct {
	Nombre      string      `+"`json:\"nombre\"`"+`
	Tipo        string      `+"`json:\"tipo\"`"+`
	Requerido   bool        `+"`json:\"requerido\"`"+`
	Default     interface{} `+"`json:\"default,omitempty\"`"+`
	Descripcion string      `+"`json:\"descripcion\"`"+`
}

type info struct {
	Nombre      string      `+"`json:\"nombre\"`"+`
	Descripcion string      `+"`json:\"descripcion\"`"+`
	Parametros  []parametro `+"`json:\"parametros\"`"+`
}

var nombreHerramienta = %q
var descripcionHerramienta = %q
var parametrosHerramienta = []parametro{
%s    }

func main() {
	defer func() {
		if r := recover(); r != nil {
			out, _ := json.Marshal(respuesta{Exito: false, Error: fmt.Sprintf("panic: %%v", r)})
			fmt.Println(string(out))
			os.Exit(0)
		}
	}()

	reader := bufio.NewReader(os.Stdin)
	linea, err := reader.ReadString('\n')
	if err != nil {
		out, _ := json.Marshal(respuesta{Exito: false, Error: "leyendo stdin: " + err.Error()})
		fmt.Println(string(out))
		return
	}

	var sol solicitud
	if err := json.Unmarshal([]byte(linea), &sol); err != nil {
		out, _ := json.Marshal(respuesta{Exito: false, Error: "JSON inválido: " + err.Error()})
		fmt.Println(string(out))
		return
	}

	var res respuesta
	switch sol.Operacion {
	case "info":
		res = manejarInfo()
	case "validar":
		res = manejarValidar()
	case "ejecutar":
		res = manejarEjecutar(sol.Parametros)
	default:
		res = respuesta{Exito: false, Error: "operación desconocida: " + sol.Operacion}
	}

	out, _ := json.Marshal(res)
	fmt.Println(string(out))
}

func manejarInfo() respuesta {
	return respuesta{
		Exito: true,
		Datos: info{
			Nombre:      nombreHerramienta,
			Descripcion: descripcionHerramienta,
			Parametros:  parametrosHerramienta,
		},
	}
}

func manejarValidar() respuesta {
	return respuesta{Exito: true, Datos: map[string]bool{"ok": true}}
}

func manejarEjecutar(params map[string]interface{}) respuesta {
	return respuesta{
		Exito: false,
		Error: "herramienta stub — recompile con LLM para implementación real",
		Metadata: map[string]interface{}{
			"parametros_recibidos": params,
			"nombre":               nombreHerramienta,
		},
	}
}
`, spec.Nombre, spec.Descripcion, paramsDecl.String())

	return source
}
