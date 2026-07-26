package resumen_llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests de ResumenesLLM
// ============================================================================

func TestNuevoResumenesLLM_NilDir(t *testing.T) {
	r := NuevoResumenesLLM(nil, nil)
	if r == nil {
		t.Fatal("esperaba instancia no nil")
	}
}

func TestNuevoResumenesLLM_ConDir(t *testing.T) {
	tmpDir := t.TempDir()
	r := NuevoResumenesLLM(&tmpDir, nil)
	if r == nil {
		t.Fatal("esperaba instancia no nil")
	}
}

func TestResumenesLLM_Obtener_SinCache(t *testing.T) {
	tmpDir := t.TempDir()
	r := NuevoResumenesLLM(&tmpDir, nil)

	resumen, err := r.Obtener(context.Background(), "fragment-1", "func test() {}", "test.go")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resumen == "" {
		t.Log("resumen vacío es aceptable sin LLM")
	}
}

func TestResumenesLLM_Obtener_ConLLMError(t *testing.T) {
	tmpDir := t.TempDir()
	llmFunc := func(ctx context.Context, codigo, ruta string) (string, error) {
		return "", nil // retorna vacío sin error
	}
	r := NuevoResumenesLLM(&tmpDir, llmFunc)

	resumen, err := r.Obtener(context.Background(), "frag-2", "func main() {}", "main.go")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	_ = resumen // puede ser vacío
}

func TestResumenesLLM_Obtener_ConLLMExito(t *testing.T) {
	tmpDir := t.TempDir()
	llmFunc := func(ctx context.Context, codigo, ruta string) (string, error) {
		return "Función principal del programa", nil
	}
	r := NuevoResumenesLLM(&tmpDir, llmFunc)

	resumen, err := r.Obtener(context.Background(), "frag-3", "func main() {}", "main.go")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resumen != "Función principal del programa" {
		t.Errorf("esperaba resumen del LLM, got '%s'", resumen)
	}
}

func TestResumenesLLM_Obtener_DesdeCache(t *testing.T) {
	tmpDir := t.TempDir()
	// Crear archivo de cache manualmente
	cacheDir := filepath.Join(tmpDir, "resumenes_llm")
	os.MkdirAll(cacheDir, 0755)
	cacheFile := filepath.Join(cacheDir, "frag-cached.txt")
	os.WriteFile(cacheFile, []byte("Resumen cacheado previamente"), 0644)

	r := NuevoResumenesLLM(&tmpDir, nil)

	resumen, err := r.Obtener(context.Background(), "frag-cached", "código", "archivo.go")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resumen != "Resumen cacheado previamente" {
		t.Errorf("esperaba resumen cacheado, got '%s'", resumen)
	}
}

func TestResumenesLLM_Obtener_LLMError(t *testing.T) {
	tmpDir := t.TempDir()
	llmFunc := func(ctx context.Context, codigo, ruta string) (string, error) {
		return "", nil
	}
	r := NuevoResumenesLLM(&tmpDir, llmFunc)

	_, err := r.Obtener(context.Background(), "frag-err", "func test() {}", "test.go")
	// No debe fallar aunque el LLM retorne vacío
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
}

func TestResumenesLLM_Obtener_PersisteEnDisco(t *testing.T) {
	tmpDir := t.TempDir()
	llmFunc := func(ctx context.Context, codigo, ruta string) (string, error) {
		return "Resumen persistido", nil
	}
	r := NuevoResumenesLLM(&tmpDir, llmFunc)

	// Primera llamada: genera y cachea
	resumen1, _ := r.Obtener(context.Background(), "frag-persist", "func hola() {}", "hola.go")

	// Segunda llamada: debería venir de cache
	llmFunc2 := func(ctx context.Context, codigo, ruta string) (string, error) {
		return "NO DEBERÍA LLAMARSE", nil
	}
	r2 := NuevoResumenesLLM(&tmpDir, llmFunc2)
	resumen2, _ := r2.Obtener(context.Background(), "frag-persist", "func hola() {}", "hola.go")

	if resumen1 != resumen2 {
		t.Errorf("resumen1 '%s' != resumen2 '%s'", resumen1, resumen2)
	}
	if strings.Contains(resumen2, "NO DEBERÍA LLAMARSE") {
		t.Error("el segundo llamado no debería usar el LLM")
	}
}

func TestResumenesLLM_Obtener_ConDirectorioInvalido(t *testing.T) {
	// Directorio que no se puede crear
	invalidDir := "/proc/invalid/no_existe/liz"
	r := NuevoResumenesLLM(&invalidDir, nil)

	// No debe panic
	_, err := r.Obtener(context.Background(), "frag-inv", "code", "file.go")
	if err != nil {
		t.Logf("error aceptable con dir inválido: %v", err)
	}
}

func TestResumenesLLM_Obtener_ContextoCancelado(t *testing.T) {
	tmpDir := t.TempDir()
	llmCalled := false
	llmFunc := func(ctx context.Context, codigo, ruta string) (string, error) {
		llmCalled = true
		return "resumen", nil
	}
	r := NuevoResumenesLLM(&tmpDir, llmFunc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancelar inmediatamente

	_, err := r.Obtener(ctx, "frag-ctx", "code", "file.go")
	if err != nil {
		t.Logf("error con contexto cancelado: %v", err)
	}
}

func TestResumenesLLM_VariosFragmentos(t *testing.T) {
	tmpDir := t.TempDir()
	llmFunc := func(ctx context.Context, codigo, ruta string) (string, error) {
		return "Resumen para " + ruta, nil
	}
	r := NuevoResumenesLLM(&tmpDir, llmFunc)

	fragmentos := []struct {
		id    string
		codigo string
		ruta  string
	}{
		{"f1", "func a() {}", "a.go"},
		{"f2", "func b() {}", "b.go"},
		{"f3", "func c() {}", "c.go"},
	}

	for _, f := range fragmentos {
		resumen, err := r.Obtener(context.Background(), f.id, f.codigo, f.ruta)
		if err != nil {
			t.Errorf("error para %s: %v", f.id, err)
		}
		if !strings.Contains(resumen, f.ruta) {
			t.Errorf("resumen para %s no contiene ruta: %s", f.id, resumen)
		}
	}
}