package auto_creacion

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
)

// ============================================================================
// Mock LLM para tests
// ============================================================================

// mockLLM implementa ClienteLLM retornando respuestas pre-configuradas.
type mockLLM struct {
	respuesta string
	err       error
	// captura de la última solicitud recibida (para aserciones)
	ultimaSolicitud *orquestador.SolicitudChat
}

func (m *mockLLM) Completar(req orquestador.SolicitudChat) (*orquestador.RespuestaChat, error) {
	m.ultimaSolicitud = &req
	if m.err != nil {
		return nil, m.err
	}
	return &orquestador.RespuestaChat{
		Contenido:   m.respuesta,
		ModeloUsado: "mock-llm",
		TokensTotal: 100,
	}, nil
}

// ============================================================================
// Tests de tipos y helpers
// ============================================================================

func TestNormalizarSpec_Valida(t *testing.T) {
	s := SpecHerramienta{
		Nombre:      "test_herramienta",
		Descripcion: "Una herramienta de prueba",
		Categoria:   "test",
		Parametros: []herramientas.Parametro{
			{Nombre: "msg", Tipo: "string", Requerido: true, Descripcion: "mensaje"},
		},
	}
	// normalizarSpec está declarado en detector.go
	err := normalizarSpec(&s)
	if err != nil {
		t.Fatalf("esperaba sin error, got %v", err)
	}
	if s.Nombre != "test_herramienta" {
		t.Errorf("Nombre mal normalizado: %q", s.Nombre)
	}
}

func TestNormalizarSpec_Errores(t *testing.T) {
	casos := []struct {
		nombre string
		spec   SpecHerramienta
		err    string
	}{
		{"vacío", SpecHerramienta{}, "nombre vacío"},
		{"caracteres inválidos", SpecHerramienta{Nombre: "TEST-HERRAMIENTA", Descripcion: "x"}, "nombre inválido"},
		{"sin descripción", SpecHerramienta{Nombre: "valido"}, "descripción vacía"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := normalizarSpec(&c.spec)
			if err == nil {
				t.Fatalf("esperaba error conteniendo %q", c.err)
			}
			if !strings.Contains(err.Error(), c.err) {
				t.Errorf("error %q no contiene %q", err.Error(), c.err)
			}
		})
	}
}

// ============================================================================
// Tests de plantillas y helpers de fuente
// ============================================================================

func TestExtraerFuenteGo_BloqueMarkdown(t *testing.T) {
	raw := "Aquí está el código:\n```go\npackage main\n\nfunc main() {}\n```\nEspero que sirva."
	got := ExtraerFuenteGo(raw)
	if !strings.HasPrefix(got, "package main") {
		t.Errorf("esperaba que empezara con 'package main', got: %q", got)
	}
}

func TestExtraerFuenteGo_SinBloque(t *testing.T) {
	raw := "package main\n\nfunc main() {}"
	got := ExtraerFuenteGo(raw)
	if got != raw {
		t.Errorf("esperaba %q, got %q", raw, got)
	}
}

func TestValidarFuenteGo(t *testing.T) {
	casos := []struct {
		nombre string
		fuente string
		ok     bool
	}{
		{"vacío", "", false},
		{"sin package", "func main() {}", false},
		{"sin main", "package main", false},
		{"válido", "package main\nfunc main() {}", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := ValidarFuenteGo(c.fuente)
			if c.ok && err != nil {
				t.Errorf("esperaba OK, got %v", err)
			}
			if !c.ok && err == nil {
				t.Errorf("esperaba error, got nil")
			}
		})
	}
}

func TestPlantillaPrompt_ContieneElementos(t *testing.T) {
	spec := SpecHerramienta{
		Nombre:      "test_tool",
		Descripcion: "Una herramienta de test",
		Categoria:   "test",
	}
	prompt := PlantillaPrompt(spec)
	if !strings.Contains(prompt, "test_tool") {
		t.Error("prompt no contiene el nombre")
	}
	if !strings.Contains(prompt, "PROTOCOLO SUBPROCESS") {
		t.Error("prompt no contiene el protocolo")
	}
	if !strings.Contains(prompt, "package main") {
		t.Error("prompt no contiene el ejemplo")
	}
}

func TestPlantillaPromptDeteccion_ContieneCatalogo(t *testing.T) {
	catalogo := []InfoCatalogo{
		{Nombre: "terminal", Descripcion: "Ejecuta comandos shell"},
		{Nombre: "editor", Descripcion: "Edita archivos"},
	}
	prompt := PlantillaPromptDeteccion("Comprime archivos", catalogo)
	if !strings.Contains(prompt, "terminal") {
		t.Error("prompt no contiene tool del catálogo")
	}
	if !strings.Contains(prompt, "Comprime archivos") {
		t.Error("prompt no contiene la descripción del usuario")
	}
}

// ============================================================================
// Tests del Detector (con mock LLM)
// ============================================================================

func TestDetector_Detectar_OK(t *testing.T) {
	llm := &mockLLM{
		respuesta: "```json\n" +
			`{"faltantes":[{"nombre":"compresor","descripcion":"Comprime archivos","categoria":"archivos","razon":"Falta compresión","parametros":[{"nombre":"archivos","tipo":"array","requerido":true,"descripcion":"lista"}]}],"razon":"Análisis"}` + "\n```",
	}
	d := NuevoDetector(llm)

	resultado, err := d.Detectar(context.Background(), "comprime archivos", nil)
	if err != nil {
		t.Fatalf("inesperado error: %v", err)
	}
	if len(resultado.Faltantes) != 1 {
		t.Fatalf("esperaba 1 faltante, got %d", len(resultado.Faltantes))
	}
	if resultado.Faltantes[0].Nombre != "compresor" {
		t.Errorf("nombre mal: %q", resultado.Faltantes[0].Nombre)
	}
	if resultado.ModeloUsado != "mock-llm" {
		t.Errorf("modelo mal: %q", resultado.ModeloUsado)
	}
}

func TestDetector_Detectar_JSONPlano(t *testing.T) {
	llm := &mockLLM{
		respuesta: `{"faltantes":[],"razon":"Ya hay herramientas suficientes"}`,
	}
	d := NewDetector(llm)

	resultado, err := d.Detectar(context.Background(), "lista archivos", nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(resultado.Faltantes) != 0 {
		t.Errorf("esperaba 0 faltantes, got %d", len(resultado.Faltantes))
	}
}

func TestDetector_Detectar_SinLLM(t *testing.T) {
	d := NuevoDetector(nil)
	_, err := d.Detectar(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("esperaba error")
	}
	if !strings.Contains(err.Error(), "no configurado") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestDetector_Detectar_DescripcionVacia(t *testing.T) {
	d := NuevoDetector(&mockLLM{respuesta: "{}"})
	_, err := d.Detectar(context.Background(), "   ", nil)
	if err == nil {
		t.Fatal("esperaba error")
	}
}

func TestDetector_Detectar_LLMFalla(t *testing.T) {
	llm := &mockLLM{err: fmt.Errorf("API error 500")}
	d := NuevoDetector(llm)
	_, err := d.Detectar(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("esperaba error")
	}
}

func TestDetector_Detectar_JSONMalformado(t *testing.T) {
	llm := &mockLLM{respuesta: "esto no es JSON"}
	d := NuevoDetector(llm)
	_, err := d.Detectar(context.Background(), "x", nil)
	if err == nil {
		t.Fatal("esperaba error")
	}
}

// ============================================================================
// Tests del Generador
// ============================================================================

func TestGenerador_Generar_DesdePlantilla_SinLLM(t *testing.T) {
	spec := SpecHerramienta{
		Nombre:      "stub_tool",
		Descripcion: "Tool de prueba",
		Categoria:   "test",
		Parametros: []herramientas.Parametro{
			{Nombre: "msg", Tipo: "string", Requerido: true, Descripcion: "mensaje"},
		},
	}
	res, err := GenerarDesdePlantilla(spec)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(res.FuenteGo, "package main") {
		t.Error("fuente no contiene package main")
	}
	if !strings.Contains(res.FuenteGo, "stub_tool") {
		t.Error("fuente no contiene el nombre")
	}
	if !strings.Contains(res.FuenteGo, "func main()") {
		t.Error("fuente no contiene func main()")
	}
	if !strings.Contains(res.FuenteGo, "mensaje") {
		t.Error("fuente no contiene la descripción del parámetro")
	}
	if res.ModeloUsado != "stub-local" {
		t.Errorf("modelo mal: %q", res.ModeloUsado)
	}
}

func TestGenerador_Generar_ConLLM(t *testing.T) {
	llm := &mockLLM{
		respuesta: "```go\npackage main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n```",
	}
	g := NuevoGenerador(llm)

	spec := SpecHerramienta{
		Nombre:      "test_llm_tool",
		Descripcion: "Generada por LLM",
		Categoria:   "test",
	}
	res, err := g.Generar(context.Background(), spec)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(res.FuenteGo, "package main") {
		t.Error("fuente no contiene package main")
	}
	if res.ModeloUsado != "mock-llm" {
		t.Errorf("modelo mal: %q", res.ModeloUsado)
	}
	if !strings.Contains(res.FuenteGo, "Herramienta auto-creada por Liz") {
		t.Error("fuente no tiene header inyectado")
	}
}

func TestGenerador_Generar_LLMRespondeBasura(t *testing.T) {
	llm := &mockLLM{respuesta: "no soy un generador de código"}
	g := NuevoGenerador(llm)

	spec := SpecHerramienta{Nombre: "test", Descripcion: "x", Categoria: "test"}
	_, err := g.Generar(context.Background(), spec)
	if err == nil {
		t.Fatal("esperaba error porque el fuente no tiene package main")
	}
}

func TestGenerador_Generar_SinLLM(t *testing.T) {
	g := NuevoGenerador(nil)
	_, err := g.Generar(context.Background(), SpecHerramienta{Nombre: "x", Descripcion: "y", Categoria: "z"})
	if err == nil {
		t.Fatal("esperaba error")
	}
}

// ============================================================================
// Helpers para tests
// ============================================================================

// NewDetector es un alias para NuevoDetector para usar en tests sin tildes.
func NewDetector(llm ClienteLLM) *Detector { return NuevoDetector(llm) }

// NewGenerador es un alias para NuevoGenerador.
func NewGenerador(llm ClienteLLM) *Generador { return NuevoGenerador(llm) }
