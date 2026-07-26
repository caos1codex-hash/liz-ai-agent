package buscador

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// RerankerLLM rerankinga resultados de búsqueda usando un LLM.
//
// Toma los top-10 resultados de BM25/vector search y los reordena
// según relevancia real evaluada por el LLM.
//
// Si el orquestador es nil, no se aplica reranking (passthrough).
type RerankerLLM struct {
	mu      sync.Mutex
	llmFn   func(prompt string) (string, error) // función que llama al LLM
	maxTopK int                                 // máximo resultados a rerankear (default 10)
	logFunc func(string, ...interface{})
}

// LLMReranker es la interfaz que necesita el reranker para llamar al LLM.
// Se inyecta para desacoplar del paquete orquestador.
type LLMReranker interface {
	// Rerank toma una query y una lista de fragmentos candidatos, y retorna
	// los IDs ordenados por relevancia (más relevante primero).
	Rerank(query string, candidatos []RerankCandidato) ([]string, error)
}

// RerankCandidato es un fragmento candidato para reranking.
type RerankCandidato struct {
	ID        string
	Ruta      string
	Contenido string
	Lenguaje  string
}

// NuevoRerankerLLM crea un reranker. llmFn es la función que llama al LLM
// para obtener el ranking. Si llmFn es nil, el reranker es no-op.
func NuevoRerankerLLM(llmFn func(prompt string) (string, error)) *RerankerLLM {
	return &RerankerLLM{
		llmFn:   llmFn,
		maxTopK: 10,
		logFunc: func(string, ...interface{}) {},
	}
}

// ConLog asigna función de log.
func (r *RerankerLLM) ConLog(fn func(string, ...interface{})) *RerankerLLM {
	if fn != nil {
		r.logFunc = fn
	}
	return r
}

// ConMaxTopK establece el máximo de resultados a rerankear.
func (r *RerankerLLM) ConMaxTopK(n int) *RerankerLLM {
	if n > 0 {
		r.maxTopK = n
	}
	return r
}

// TieneLLM retorna true si hay función LLM disponible.
func (r *RerankerLLM) TieneLLM() bool {
	return r.llmFn != nil
}

// Rerank rerankinga los resultados de búsqueda usando el LLM.
//
// Pasos:
//  1. Tomar los primeros maxTopK resultados
//  2. Enviar al LLM con la query y los fragmentos
//  3. Parsear la respuesta del LLM para obtener el orden
//  4. Reordenar los resultados según el ranking del LLM
//
// Si el LLM falla, retorna los resultados originales (degradación graceful).
func (r *RerankerLLM) Rerank(query string, resultados []ResultadoBusqueda) []ResultadoBusqueda {
	if r.llmFn == nil || len(resultados) <= 1 {
		return resultados
	}

	// Limitar a maxTopK candidatos
	topK := r.maxTopK
	if topK > len(resultados) {
		topK = len(resultados)
	}

	candidatos := make([]RerankCandidato, topK)
	idToIndex := make(map[string]int, topK)
	for i := 0; i < topK; i++ {
		candidatos[i] = RerankCandidato{
			ID:        resultados[i].Fragmento.ID,
			Ruta:      resultados[i].Fragmento.Ruta,
			Contenido: truncarContenido(resultados[i].Fragmento.Contenido, 500),
			Lenguaje:  resultados[i].Fragmento.Lenguaje,
		}
		idToIndex[resultados[i].Fragmento.ID] = i
	}

	// Construir prompt para el LLM
	prompt := construirPromptRerank(query, candidatos)

	r.mu.Lock()
	respuesta, err := r.llmFn(prompt)
	r.mu.Unlock()

	if err != nil {
		r.logFunc("rerank LLM falló, usando ranking original: %v", err)
		return resultados
	}

	// Parsear respuesta: esperar IDs ordenados por relevancia (uno por línea)
	idsOrdenados := parsearRespuestaRerank(respuesta, idToIndex)

	if len(idsOrdenados) == 0 {
		r.logFunc("rerank LLM no retornó IDs válidos, usando ranking original")
		return resultados
	}

	// Reconstruir resultados rerankeados
	rerankeados := make([]ResultadoBusqueda, 0, len(idsOrdenados))
	vistos := make(map[string]bool)
	for _, id := range idsOrdenados {
		if vistos[id] {
			continue
		}
		vistos[id] = true
		if idx, ok := idToIndex[id]; ok {
			rerankeados = append(rerankeados, resultados[idx])
		}
	}

	// Agregar los que no fueron mencionados por el LLM (al final)
	for i := 0; i < len(resultados); i++ {
		if !vistos[resultados[i].Fragmento.ID] {
			rerankeados = append(rerankeados, resultados[i])
		}
	}

	r.logFunc("rerank LLM: %d resultados reordenados", len(idsOrdenados))
	return rerankeados
}

// truncarContenido trunca el contenido a maxLen caracteres.
func truncarContenido(contenido string, maxLen int) string {
	if len(contenido) <= maxLen {
		return contenido
	}
	return contenido[:maxLen] + "..."
}

// construirPromptRerank construye el prompt para el LLM.
func construirPromptRerank(query string, candidatos []RerankCandidato) string {
	var b strings.Builder
	b.WriteString("Dada la siguiente pregunta de un programador, ordena los fragmentos de código por relevancia.\n\n")
	b.WriteString(fmt.Sprintf("PREGUNTA: %s\n\n", query))
	b.WriteString("FRAGMENTOS CANDIDATOS:\n\n")
	for i, c := range candidatos {
		b.WriteString(fmt.Sprintf("--- FRAGMENTO %d (ID: %s) ---\n", i+1, c.ID))
		b.WriteString(fmt.Sprintf("Archivo: %s\n", c.Ruta))
		b.WriteString(fmt.Sprintf("Lenguaje: %s\n", c.Lenguaje))
		b.WriteString(fmt.Sprintf("Contenido:\n%s\n\n", c.Contenido))
	}
	b.WriteString("INSTRUCCIONES:\n")
	b.WriteString("Retorna SOLO los IDs de los fragmentos ordenados por relevancia (más relevante primero), ")
	b.WriteString("un ID por línea. No incluyas explicaciones ni texto adicional.\n")
	b.WriteString("Ejemplo:\nabc123\ndef456\nghi789\n")
	return b.String()
}

// parsearRespuestaRerank extrae IDs de la respuesta del LLM.
// Espera un ID por línea. Filtra los que no están en el mapa.
func parsearRespuestaRerank(respuesta string, idToIndex map[string]int) []string {
	lineas := strings.Split(strings.TrimSpace(respuesta), "\n")
	var ids []string
	for _, linea := range lineas {
		id := strings.TrimSpace(linea)
		if id == "" {
			continue
		}
		if _, ok := idToIndex[id]; ok {
			ids = append(ids, id)
		}
	}

	// Si no encontramos ningún ID válido, intentar extraer de formato alternativo
	if len(ids) == 0 {
		// Intentar formato "1. ID" o "1) ID"
		for _, linea := range lineas {
			linea = strings.TrimSpace(linea)
			if len(linea) > 2 {
				// Remover prefijo numérico "1. " o "1) "
				if linea[1] == '.' || linea[1] == ')' {
					candidato := strings.TrimSpace(linea[2:])
					if _, ok := idToIndex[candidato]; ok {
						ids = append(ids, candidato)
					}
				}
			}
		}
	}

	return ids
}

// RerankerConLLM wraps un IBuscador con reranking vía LLM.
// No es un tipo embebido: es una función helper que se puede usar
// desde el coordinador o el empaquetador.
type RerankerConLLM struct {
	Buscador IBuscador
	Reranker *RerankerLLM
}

// NuevoRerankerConLLM crea un buscador con reranking LLM.
func NuevoRerankerConLLM(bus IBuscador, reranker *RerankerLLM) *RerankerConLLM {
	return &RerankerConLLM{
		Buscador: bus,
		Reranker: reranker,
	}
}

// BuscarHibrido busca híbridamente y luego aplica reranking LLM.
func (r *RerankerConLLM) BuscarHibrido(query string, topK int) []ResultadoBusqueda {
	resultados := r.Buscador.BuscarHibrido(query, topK)
	if r.Reranker != nil && r.Reranker.TieneLLM() {
		resultados = r.Reranker.Rerank(query, resultados)
	}
	return resultados
}

// BuscarBM25 busca por BM25 y luego aplica reranking LLM.
func (r *RerankerConLLM) BuscarBM25(query string, topK int) []ResultadoBusqueda {
	resultados := r.Buscador.BuscarBM25(query, topK)
	if r.Reranker != nil && r.Reranker.TieneLLM() {
		resultados = r.Reranker.Rerank(query, resultados)
	}
	return resultados
}

// SortByRelevanceScore reordena resultados por score (helper genérico).
func SortByRelevanceScore(resultados []ResultadoBusqueda) {
	sort.Slice(resultados, func(i, j int) bool {
		return resultados[i].Score > resultados[j].Score
	})
}
