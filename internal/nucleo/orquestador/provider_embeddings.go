package orquestador

import (
	"fmt"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
)

// ═══════════════════════════════════════════════════════
// ADAPTADOR: ClienteNVIDIA → buscador.EmbeddingsProvider
// ═══════════════════════════════════════════════════════

// ProviderEmbeddingsNVIDIA adapta ClienteNVIDIA.Embeddings a la interfaz
// buscador.EmbeddingsProvider. Permite que el BuscadorEmbeddings use
// nv-embed-v1 sin depender directamente del paquete orquestador.
//
// Uso típico:
//
//	orch, _ := orquestador.NuevoOrquestador(cfg)
//	provider := orquestador.NuevoProviderEmbeddings(orch.Cliente(), "nvidia/nv-embed-v1")
//	buscador := buscador.NuevoBuscadorEmbeddings(provider)
//	buscador.IndexarConEmbeddings(frag)
type ProviderEmbeddingsNVIDIA struct {
	cliente *ClienteNVIDIA
	modelo  string // "nvidia/nv-embed-v1" por defecto
}

// NuevoProviderEmbeddings crea un provider de embeddings NVIDIA.
// modelo puede ser "" → se usa "nvidia/nv-embed-v1" por defecto.
func NuevoProviderEmbeddings(cliente *ClienteNVIDIA, modelo string) *ProviderEmbeddingsNVIDIA {
	if modelo == "" {
		modelo = "nvidia/nv-embed-v1"
	}
	return &ProviderEmbeddingsNVIDIA{
		cliente: cliente,
		modelo:  modelo,
	}
}

// GenerarEmbeddings implementa buscador.EmbeddingsProvider.
// Llama a la API de NVIDIA para generar embeddings de los textos.
func (p *ProviderEmbeddingsNVIDIA) GenerarEmbeddings(textos []string) ([][]float32, error) {
	if p.cliente == nil {
		return nil, fmt.Errorf("cliente NVIDIA no inicializado")
	}
	if len(textos) == 0 {
		return [][]float32{}, nil
	}

	resp, apiErr, err := p.cliente.Embeddings(textos, p.modelo)
	if err != nil {
		return nil, fmt.Errorf("error de red: %w", err)
	}
	if apiErr != nil {
		return nil, apiErr
	}

	// Convertir respuesta al formato esperado
	resultado := make([][]float32, len(textos))
	for _, data := range resp.Data {
		if data.Index < len(resultado) {
			resultado[data.Index] = data.Embedding
		}
	}
	return resultado, nil
}

// Dimensiones retorna el tamaño del vector de embeddings.
// nv-embed-v1 retorna 1024 dimensiones.
func (p *ProviderEmbeddingsNVIDIA) Dimensiones() int {
	// NVIDIA nv-embed-v1 produce vectores de 1024 dimensiones.
	// Si se usa otro modelo, esto debería ser configurable.
	switch p.modelo {
	case "nvidia/nv-embed-v1":
		return 1024
	case "nvidia/nv-embedqa-e5-v5":
		return 1024
	case "nvidia/nv-embedqa-mistral7b-v2":
		return 4096
	case "snowflake/arctic-embed-l-v2.0":
		return 1024
	default:
		return 1024 // default para modelos NVIDIA
	}
}

// Modelo retorna el nombre del modelo configurado.
func (p *ProviderEmbeddingsNVIDIA) Modelo() string {
	return p.modelo
}

// Compile-time check: ProviderEmbeddingsNVIDIA implementa buscador.EmbeddingsProvider
var _ buscador.EmbeddingsProvider = (*ProviderEmbeddingsNVIDIA)(nil)
