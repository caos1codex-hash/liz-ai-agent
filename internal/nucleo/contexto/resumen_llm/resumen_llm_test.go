package resumen_llm

import (
	"testing"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
)

func TestGeneradorResumenLLM_NilOrquestador_RetornaVacio(t *testing.T) {
	g := NuevoGeneradorResumenLLM(t.TempDir(), nil)
	resumen := g.GenerarResumen(buscador.FragmentoBuscable{
		ID:        "f1",
		Contenido: "func hello() { println(\"hello\") }",
		Lenguaje:  "go",
	})
	if resumen != "" {
		t.Errorf("esperado vacio, got %q", resumen)
	}
}

func TestGeneradorResumenLLM_TieneOrquestador(t *testing.T) {
	g := NuevoGeneradorResumenLLM(t.TempDir(), nil)
	if g.TieneOrquestador() {
		t.Error("nil orch deberia retornar false")
	}
}

func TestGeneradorResumenLLM_ConLog(t *testing.T) {
	g := NuevoGeneradorResumenLLM(t.TempDir(), nil)
	g2 := g.ConLog(nil)
	if g != g2 {
		t.Error("ConLog(nil) debe retornar misma instancia")
	}
}
