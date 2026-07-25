package orquestador

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
)

func TestProviderEmbeddingsNVIDIA_GenerarEmbeddings_Exitoso(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"data": [
				{"embedding": [0.1, 0.2, 0.3], "index": 0},
				{"embedding": [0.4, 0.5, 0.6], "index": 1}
			],
			"model": "nvidia/nv-embed-v1",
			"usage": {"prompt_tokens": 10, "total_tokens": 10}
		}`))
	}))
	defer srv.Close()

	cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
	provider := NuevoProviderEmbeddings(cliente, "")

	vectors, err := provider.GenerarEmbeddings([]string{"hola", "mundo"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(vectors) != 2 {
		t.Errorf("debería retornar 2 vectores, got %d", len(vectors))
	}
	if len(vectors[0]) != 3 {
		t.Errorf("dim incorrecta: %d", len(vectors[0]))
	}
}

func TestProviderEmbeddingsNVIDIA_SinTextos_RetornaVacio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no debería llamar al servidor con 0 textos")
	}))
	defer srv.Close()

	cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
	provider := NuevoProviderEmbeddings(cliente, "")

	vectors, err := provider.GenerarEmbeddings([]string{})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(vectors) != 0 {
		t.Errorf("debería retornar slice vacío, got %d", len(vectors))
	}
}

func TestProviderEmbeddingsNVIDIA_ClienteNil_Error(t *testing.T) {
	provider := NuevoProviderEmbeddings(nil, "")
	_, err := provider.GenerarEmbeddings([]string{"test"})
	if err == nil {
		t.Error("debería error con cliente nil")
	}
}

func TestProviderEmbeddingsNVIDIA_APIFalla_PropagaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "internal"}}`))
	}))
	defer srv.Close()

	cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
	provider := NuevoProviderEmbeddings(cliente, "")

	_, err := provider.GenerarEmbeddings([]string{"test"})
	if err == nil {
		t.Error("debería propagar error de la API")
	}
}

func TestProviderEmbeddingsNVIDIA_Dimensiones(t *testing.T) {
	cliente := NuevoClienteNVIDIA("http://test", "key")

	casos := []struct {
		modelo    string
		esperado  int
	}{
		{"nvidia/nv-embed-v1", 1024},
		{"nvidia/nv-embedqa-e5-v5", 1024},
		{"nvidia/nv-embedqa-mistral7b-v2", 4096},
		{"snowflake/arctic-embed-l-v2.0", 1024},
		{"modelo-desconocido", 1024}, // default
		{"", 1024}, // default (se convierte a nv-embed-v1)
	}
	for _, c := range casos {
		provider := NuevoProviderEmbeddings(cliente, c.modelo)
		// Caso especial: modelo "" se convierte a nv-embed-v1
		modeloEfectivo := c.modelo
		if modeloEfectivo == "" {
			modeloEfectivo = "nvidia/nv-embed-v1"
		}
		if provider.Modelo() != modeloEfectivo {
			t.Errorf("modelo efectivo incorrecto: %s vs %s", provider.Modelo(), modeloEfectivo)
		}
		if provider.Dimensiones() != c.esperado {
			t.Errorf("modelo %s: dimensiones = %d, esperaba %d",
				c.modelo, provider.Dimensiones(), c.esperado)
		}
	}
}

// Test de integración: el provider NVIDIA cumple la interfaz buscador.EmbeddingsProvider
func TestProviderEmbeddingsNVIDIA_ImplementaInterfazBuscador(t *testing.T) {
	cliente := NuevoClienteNVIDIA("http://test", "key")
	provider := NuevoProviderEmbeddings(cliente, "")

	// Compile-time check
	var _ buscador.EmbeddingsProvider = provider

	// Runtime check via type assertion
	var iface buscador.EmbeddingsProvider = provider
	if iface.Dimensiones() != 1024 {
		t.Errorf("dim incorrecta: %d", iface.Dimensiones())
	}
}

// Test de integración end-to-end con buscador.BuscarHibridoConEmbeddings
func TestProviderEmbeddingsNVIDIA_IntegracionConBuscador(t *testing.T) {
	// Mock server que retorna embeddings predecibles
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SolicitudEmbeddings
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		// Generar embeddings basados en el primer char de cada texto
		data := []map[string]interface{}{}
		for i, texto := range req.Input {
			// Vector simple: dimensiones[i] = primer char mod 10
			vec := []float32{0, 0, 0, 0}
			if len(texto) > 0 {
				vec[0] = float32(texto[0]) / 100.0
				vec[1] = float32(texto[len(texto)-1]) / 100.0
			}
			data = append(data, map[string]interface{}{
				"embedding": vec,
				"index":     i,
			})
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"data":  data,
			"model": req.Model,
			"usage": map[string]int{"prompt_tokens": 10, "total_tokens": 10},
		})
		w.Write(resp)
	}))
	defer srv.Close()

	cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
	provider := NuevoProviderEmbeddings(cliente, "nvidia/nv-embed-v1")
	bus := buscador.NuevoBuscadorEmbeddings(provider)

	// Indexar fragmentos
	err := bus.IndexarConEmbeddings(buscador.FragmentoBuscable{
		ID:        "f1",
		Contenido: "auth function",
	})
	if err != nil {
		t.Fatalf("error indexando: %v", err)
	}

	if bus.TotalEmbeddings() != 1 {
		t.Errorf("debería tener 1 embedding, got %d", bus.TotalEmbeddings())
	}

	// Búsqueda híbrida
	resultados := bus.BuscarHibridoConEmbeddings("auth", 10)
	if len(resultados) == 0 {
		t.Fatal("debería encontrar resultados")
	}
}
