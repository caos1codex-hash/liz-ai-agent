package buscador

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// ═══════════════════════════════════════════════════════
// PROVIDER DE EMBEDDINGS
// ═══════════════════════════════════════════════════════

// EmbeddingsProvider es la interfaz que debe implementar cualquier proveedor
// de embeddings (NVIDIA nv-embed-v1, OpenAI text-embedding-3-small, etc.).
//
// El buscador depende solo de esta interfaz, no del paquete orquestador,
// para mantener las dependencias limpias (buscador no importa orquestador).
//
// Implementación concreta: ver paquete orquestador (ClienteNVIDIA.Embeddings).
type EmbeddingsProvider interface {
	// GenerarEmbeddings toma una lista de textos y retorna sus vectores.
	// El slice retornado tiene el mismo orden y longitud que el input.
	// Si el provider falla, retorna error.
	GenerarEmbeddings(textos []string) ([][]float32, error)

	// Dimensiones retorna el tamaño del vector (e.g. 1024 para nv-embed-v1).
	// Útil para verificar consistencia.
	Dimensiones() int
}

// ═══════════════════════════════════════════════════════
// BUSCADOR CON EMBEDDINGS
// ═══════════════════════════════════════════════════════

// BuscadorEmbeddings es un Buscador extendido con soporte de embeddings.
// Mantiene el índice BM25 (heredado) + un índice vectorial.
//
// La búsqueda híbrida combina ambos rankings vía Reciprocal Rank Fusion (RRF).
type BuscadorEmbeddings struct {
	*Buscador
	mu          sync.RWMutex
	embeddings  map[string][]float32 // fragmentID → vector
	provider    EmbeddingsProvider
	dimensiones int
}

// NuevoBuscadorEmbeddings crea un buscador híbrido con soporte de embeddings.
// El provider puede ser nil (en cuyo caso se comporta como Buscador puro).
func NuevoBuscadorEmbeddings(provider EmbeddingsProvider) *BuscadorEmbeddings {
	be := &BuscadorEmbeddings{
		Buscador:   NuevoBuscador(),
		embeddings: make(map[string][]float32),
		provider:   provider,
	}
	if provider != nil {
		be.dimensiones = provider.Dimensiones()
	}
	return be
}

// ConProvider asigna o reemplaza el provider de embeddings.
// Los embeddings ya indexados se conservan (se asume misma dimensionalidad).
func (be *BuscadorEmbeddings) ConProvider(p EmbeddingsProvider) *BuscadorEmbeddings {
	be.mu.Lock()
	defer be.mu.Unlock()
	be.provider = p
	if p != nil && be.dimensiones == 0 {
		be.dimensiones = p.Dimensiones()
	}
	return be
}

// TieneProvider retorna true si hay un provider de embeddings configurado.
func (be *BuscadorEmbeddings) TieneProvider() bool {
	be.mu.RLock()
	defer be.mu.RUnlock()
	return be.provider != nil
}

// TotalEmbeddings retorna cuántos fragmentos tienen embedding calculado.
func (be *BuscadorEmbeddings) TotalEmbeddings() int {
	be.mu.RLock()
	defer be.mu.RUnlock()
	return len(be.embeddings)
}

// ═══════════════════════════════════════════════════════
// INDEXACIÓN CON EMBEDDINGS
// ═══════════════════════════════════════════════════════

// IndexarConEmbeddings agrega un fragmento al índice BM25 Y genera su embedding.
// Requiere un provider configurado.
//
// Si el provider falla, el fragmento se indexa en BM25 pero no en el índice
// vectorial (degradación graceful).
func (be *BuscadorEmbeddings) IndexarConEmbeddings(f FragmentoBuscable) error {
	// Indexar en BM25 (siempre)
	be.Indexar(f)

	if be.provider == nil {
		return ErrProviderNoConfigurado
	}

	// Generar embedding
	vectors, err := be.provider.GenerarEmbeddings([]string{f.Contenido})
	if err != nil {
		return fmt.Errorf("generando embeddings: %w", err)
	}
	if len(vectors) == 0 {
		return ErrEmbeddingsVacios
	}

	be.mu.Lock()
	defer be.mu.Unlock()
	be.embeddings[f.ID] = vectors[0]
	return nil
}

// IndexarBatchConEmbeddings indexa múltiples fragmentos y genera sus embeddings
// en una sola llamada al provider (más eficiente que indexar de a uno).
//
// Los fragmentos se indexan en BM25 individualmente pero los embeddings se
// generan en batch.
func (be *BuscadorEmbeddings) IndexarBatchConEmbeddings(frags []FragmentoBuscable) (int, error) {
	if len(frags) == 0 {
		return 0, nil
	}

	// Indexar todos en BM25 primero
	for _, f := range frags {
		be.Indexar(f)
	}

	if be.provider == nil {
		return 0, ErrProviderNoConfigurado
	}

	// Generar embeddings en batch
	textos := make([]string, len(frags))
	for i, f := range frags {
		textos[i] = f.Contenido
	}

	vectors, err := be.provider.GenerarEmbeddings(textos)
	if err != nil {
		return 0, fmt.Errorf("generando embeddings en batch: %w", err)
	}

	if len(vectors) != len(frags) {
		return 0, ErrEmbeddingsInconsistentes
	}

	be.mu.Lock()
	defer be.mu.Unlock()
	indexados := 0
	for i, f := range frags {
		if len(vectors[i]) > 0 {
			be.embeddings[f.ID] = vectors[i]
			indexados++
		}
	}
	return indexados, nil
}

// Desindexar elimina un fragmento del índice BM25 y del índice de embeddings.
func (be *BuscadorEmbeddings) Desindexar(id string) {
	be.Buscador.Desindexar(id)
	be.mu.Lock()
	defer be.mu.Unlock()
	delete(be.embeddings, id)
}

// ═══════════════════════════════════════════════════════
// BÚSQUEDA VECTORIAL
// ═══════════════════════════════════════════════════════

// BuscarVector busca fragmentos por similitud coseno contra el embedding de la query.
// Si el provider no está configurado, retorna error.
// Si no hay embeddings indexados, retorna lista vacía.
func (be *BuscadorEmbeddings) BuscarVector(query string, topK int) ([]ResultadoBusqueda, error) {
	if be.provider == nil {
		return nil, ErrProviderNoConfigurado
	}

	be.mu.RLock()
	if len(be.embeddings) == 0 {
		be.mu.RUnlock()
		return []ResultadoBusqueda{}, nil
	}
	be.mu.RUnlock()

	// Generar embedding de la query
	queryVecs, err := be.provider.GenerarEmbeddings([]string{query})
	if err != nil {
		return nil, fmt.Errorf("generando embedding de consulta: %w", err)
	}
	if len(queryVecs) == 0 {
		return nil, ErrEmbeddingsVacios
	}
	queryVec := queryVecs[0]

	// Calcular similitud coseno contra todos los embeddings
	be.mu.RLock()
	type scored struct {
		id    string
		score float64
	}
	resultados := make([]scored, 0, len(be.embeddings))
	for id, vec := range be.embeddings {
		score := similitudCoseno(queryVec, vec)
		resultados = append(resultados, scored{id, score})
	}
	be.mu.RUnlock()

	// Ordenar por score descendente
	sort.Slice(resultados, func(i, j int) bool {
		return resultados[i].score > resultados[j].score
	})

	if topK > len(resultados) {
		topK = len(resultados)
	}
	if topK <= 0 {
		return []ResultadoBusqueda{}, nil
	}

	// Convertir a ResultadoBusqueda (acquire Buscador.mu for be.fragmentos)
	be.Buscador.mu.RLock()
	out := make([]ResultadoBusqueda, 0, topK)
	for i := 0; i < topK; i++ {
		r := resultados[i]
		frag := be.fragmentos[r.id]
		out = append(out, ResultadoBusqueda{
			Fragmento:   frag,
			Score:       r.score,
			ScoreVector: r.score,
			RankVector:  i + 1,
		})
	}
	be.Buscador.mu.RUnlock()
	return out, nil
}

// ═══════════════════════════════════════════════════════
// BÚSQUEDA HÍBRIDA (BM25 + Vector + RRF)
// ═══════════════════════════════════════════════════════

// BuscarHibridoConEmbeddings combina BM25 + búsqueda vectorial vía RRF.
// Requiere provider de embeddings.
//
// Si el provider falla al embeddear la query, cae a BM25 puro (degradación graceful).
func (be *BuscadorEmbeddings) BuscarHibridoConEmbeddings(query string, topK int) []ResultadoBusqueda {
	// BM25 siempre
	bm25Resultados := be.BuscarBM25(query, topK*2) // amplio para mejor fusión

	// Vector (puede fallar)
	vectorResultados, err := be.BuscarVector(query, topK*2)
	if err != nil || len(vectorResultados) == 0 {
		// Sin vector: usar solo BM25 con score RRF simple
		for i := range bm25Resultados {
			rank := bm25Resultados[i].RankBM25
			if rank > 0 {
				bm25Resultados[i].Score = 1.0 / float64(rrfK+rank)
			}
		}
		if topK < len(bm25Resultados) {
			bm25Resultados = bm25Resultados[:topK]
		}
		return bm25Resultados
	}

	// RRF fusion
	return be.fusionRRF(bm25Resultados, vectorResultados, topK)
}

// fusionRRF combina dos rankings vía Reciprocal Rank Fusion.
func (be *BuscadorEmbeddings) fusionRRF(bm25, vector []ResultadoBusqueda, topK int) []ResultadoBusqueda {
	bm25Rank := make(map[string]int)
	for i, r := range bm25 {
		bm25Rank[r.Fragmento.ID] = i + 1
	}
	vectorRank := make(map[string]int)
	vectorScore := make(map[string]float64)
	for i, r := range vector {
		vectorRank[r.Fragmento.ID] = i + 1
		vectorScore[r.Fragmento.ID] = r.ScoreVector
	}

	// Combinar IDs
	todosIDs := make(map[string]bool)
	for id := range bm25Rank {
		todosIDs[id] = true
	}
	for id := range vectorRank {
		todosIDs[id] = true
	}

	// Calcular score RRF
	resultados := make([]ResultadoBusqueda, 0, len(todosIDs))
	for id := range todosIDs {
		score := 0.0
		if rank, ok := bm25Rank[id]; ok {
			score += 1.0 / float64(rrfK+rank)
		}
		if rank, ok := vectorRank[id]; ok {
			score += 1.0 / float64(rrfK+rank)
		}

		var frag FragmentoBuscable
		be.Buscador.mu.RLock()
		if f, ok := be.fragmentos[id]; ok {
			frag = f
		}
		be.Buscador.mu.RUnlock()

		resultados = append(resultados, ResultadoBusqueda{
			Fragmento:   frag,
			Score:       score,
			ScoreVector: vectorScore[id],
			RankBM25:    bm25Rank[id],
			RankVector:  vectorRank[id],
		})
	}

	// Ordenar por score RRF descendente
	sort.Slice(resultados, func(i, j int) bool {
		return resultados[i].Score > resultados[j].Score
	})

	if topK > len(resultados) {
		topK = len(resultados)
	}
	return resultados[:topK]
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// similitudCoseno calcula la similitud coseno entre dos vectores.
// Retorna 0.0 si uno de los vectores está vacío o tiene norma 0.
//
// Fórmula: cos(A, B) = (A · B) / (|A| * |B|)
func similitudCoseno(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct float64
	var normA float64
	var normB float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dotProduct += af * bf
		normA += af * af
		normB += bf * bf
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ═══════════════════════════════════════════════════════
// ERRORES
// ═══════════════════════════════════════════════════════

// Errores del buscador con embeddings.
type buscadorError string

func (e buscadorError) Error() string { return string(e) }

const (
	ErrProviderNoConfigurado    buscadorError = "provider de embeddings no configurado"
	ErrEmbeddingsVacios         buscadorError = "provider retornó lista de embeddings vacía"
	ErrEmbeddingsInconsistentes buscadorError = "provider retornó número inconsistente de embeddings"
)
