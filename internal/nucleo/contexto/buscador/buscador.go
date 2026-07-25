// Package buscador implementa búsqueda híbrida sobre fragmentos de código.
//
// Combina dos estrategias para encontrar fragmentos relevantes:
//
//   1. BM25 (Best Matching 25): algoritmo clásico de ranking por keyword
//      - Tokeniza contenido (lowercase + split por no-alfanumérico)
//      - Stopwords: 50+ palabras comunes en 5 lenguajes
//      - Inverted index: término → [(fragment_id, tf)]
//      - Scoring: IDF * (tf * (k1+1)) / (tf + k1*(1-b+b*|d|/avgdl))
//      - Parámetros: k1=1.5, b=0.75 (valores estándar de la literatura)
//
//   2. Vector embeddings (opcional, futuro): similitud coseno
//      - Cuando Fase 4 (NVIDIA) esté lista, se usarán embeddings de
//        nvidia/nv-embed-v1 (1024 dim)
//      - Por ahora, fallback a BM25 puro
//
//   3. Reciprocal Rank Fusion (RRF): combina ambos rankings
//      - score(d) = sum(1 / (k + rank_i(d))) para cada ranking i, k=60
//      - Resultado: ranking único que aprovecha fortalezas de ambos
package buscador

import (
        "fmt"
        "math"
        "sort"
        "strings"
        "sync"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// FragmentoBuscable es la interfaz mínima que un fragmento debe implementar
// para ser indexado por el buscador.
type FragmentoBuscable struct {
        ID       string `json:"id"`
        Ruta     string `json:"ruta"`
        Contenido string `json:"contenido"`
        Tipo     string `json:"tipo"`     // "funcion", "estructura", etc.
        Lenguaje string `json:"lenguaje"`
}

// ResultadoBusqueda es un fragmento encontrado con su score.
type ResultadoBusqueda struct {
        Fragmento  FragmentoBuscable `json:"fragmento"`
        Score      float64           `json:"score"`
        ScoreBM25  float64           `json:"score_bm25"`
        ScoreVector float64          `json:"score_vector"` // 0 si no hay embeddings
        RankBM25   int               `json:"rank_bm25"`    // 0 si no rankeó
        RankVector int               `json:"rank_vector"`  // 0 si no rankeó
}

// EstadisticasBuscador resume el estado del índice de búsqueda.
type EstadisticasBuscador struct {
        TotalFragmentos int     `json:"total_fragmentos"`
        TerminosUnicos  int     `json:"terminos_unicos"`
        PromedioLongitud float64 `json:"promedio_longitud"`
}

// Buscador es el buscador híbrido BM25 + vector.
type Buscador struct {
        mu sync.RWMutex

        // Documentos indexados
        fragmentos map[string]FragmentoBuscable // id → fragmento
        longitudes map[string]int               // id → cantidad de tokens

        // Inverted index: término → map de (fragmentID → frecuencia)
        indice map[string]map[string]int

        // Estadísticas BM25
        totalFragmentos int
        totalTokens     int
        avgdl           float64 // average document length

        // Vector embeddings (futuro, Fase 4)
        // embeddings map[string][]float32 // id → vector
        // clienteEmbeddings *nvidia.ClienteEmbeddings
}

// NuevoBuscador crea un nuevo buscador vacío.
func NuevoBuscador() *Buscador {
        return &Buscador{
                fragmentos: make(map[string]FragmentoBuscable),
                longitudes: make(map[string]int),
                indice:     make(map[string]map[string]int),
        }
}

// ═══════════════════════════════════════════════════════
// INDEXACIÓN
// ═══════════════════════════════════════════════════════

// Indexar agrega o reemplaza un fragmento en el índice.
// Si el fragmento ya existe (mismo ID), se re-indexa.
func (b *Buscador) Indexar(f FragmentoBuscable) {
        b.mu.Lock()
        defer b.mu.Unlock()

        // Si ya existe, eliminar del índice anterior
        if _, existe := b.fragmentos[f.ID]; existe {
                b.desindexar(f.ID)
        }

        // Indexar nuevo
        b.fragmentos[f.ID] = f
        tokens := tokenizar(f.Contenido)
        b.longitudes[f.ID] = len(tokens)

        for _, token := range tokens {
                if b.indice[token] == nil {
                        b.indice[token] = make(map[string]int)
                }
                b.indice[token][f.ID]++
        }

        b.totalFragmentos++
        b.totalTokens += len(tokens)
        b.recalcularAvgdl()
}

// Desindexar elimina un fragmento del índice.
func (b *Buscador) Desindexar(id string) {
        b.mu.Lock()
        defer b.mu.Unlock()
        b.desindexar(id)
}

// desindexar elimina un fragmento (sin lock, para uso interno).
func (b *Buscador) desindexar(id string) {
        frag, existe := b.fragmentos[id]
        if !existe {
                return
        }

        tokens := tokenizar(frag.Contenido)
        for _, token := range tokens {
                if docs, ok := b.indice[token]; ok {
                        delete(docs, id)
                        if len(docs) == 0 {
                                delete(b.indice, token)
                        }
                }
        }

        b.totalFragmentos--
        b.totalTokens -= b.longitudes[id]
        delete(b.longitudes, id)
        delete(b.fragmentos, id)
        b.recalcularAvgdl()
}

// recalcularAvgdl recalcula el promedio de longitud de documentos.
func (b *Buscador) recalcularAvgdl() {
        if b.totalFragmentos > 0 {
                b.avgdl = float64(b.totalTokens) / float64(b.totalFragmentos)
        } else {
                b.avgdl = 0
        }
}

// ═══════════════════════════════════════════════════════
// BÚSQUEDA BM25
// ═══════════════════════════════════════════════════════

// Parámetros BM25 estándar de la literatura.
const (
        bm25K1 = 1.5
        bm25B  = 0.75
)

// BuscarBM25 busca fragmentos por query usando el algoritmo BM25.
// Retorna hasta topK resultados ordenados por score descendente.
func (b *Buscador) BuscarBM25(query string, topK int) []ResultadoBusqueda {
        b.mu.RLock()
        defer b.mu.RUnlock()

        if b.totalFragmentos == 0 || topK <= 0 {
                return []ResultadoBusqueda{}
        }

        queryTokens := tokenizar(query)
        if len(queryTokens) == 0 {
                return []ResultadoBusqueda{}
        }

        // Calcular BM25 score para cada documento que contiene al menos un término
        scores := make(map[string]float64)
        for _, token := range queryTokens {
                docs, ok := b.indice[token]
                if !ok {
                        continue
                }

                // IDF (Inverse Document Frequency) — variante BM25 con suavizado
                // IDF(t) = ln(1 + (N - n(t) + 0.5) / (n(t) + 0.5))
                n := float64(b.totalFragmentos)
                nt := float64(len(docs))
                idf := math.Log(1 + (n-nt+0.5)/(nt+0.5))

                for docID, tf := range docs {
                        dl := float64(b.longitudes[docID])
                        // BM25 score:
                        // IDF * (tf * (k1+1)) / (tf + k1*(1 - b + b*dl/avgdl))
                        denominador := float64(tf) + bm25K1*(1-bm25B+bm25B*dl/b.avgdl)
                        if denominador == 0 {
                                continue
                        }
                        score := idf * (float64(tf) * (bm25K1 + 1)) / denominador
                        scores[docID] += score
                }
        }

        // Convertir a slice y ordenar
        resultados := make([]ResultadoBusqueda, 0, len(scores))
        for docID, score := range scores {
                frag := b.fragmentos[docID]
                resultados = append(resultados, ResultadoBusqueda{
                        Fragmento: frag,
                        Score:     score,
                        ScoreBM25: score,
                })
        }

        sort.Slice(resultados, func(i, j int) bool {
                return resultados[i].ScoreBM25 > resultados[j].ScoreBM25
        })

        if topK > len(resultados) {
                topK = len(resultados)
        }

        // Asignar rank
        for i := range resultados[:topK] {
                resultados[i].RankBM25 = i + 1
        }

        return resultados[:topK]
}

// ═══════════════════════════════════════════════════════
// BÚSQUEDA HÍBRIDA (RRF)
// ═══════════════════════════════════════════════════════

// Parámetro k de Reciprocal Rank Fusion (estándar de la literatura).
const rrfK = 60

// BuscarHibrido combina BM25 con búsqueda vector (cuando esté disponible)
// usando Reciprocal Rank Fusion (RRF).
//
// Por ahora solo usa BM25, pero la firma queda preparada para cuando
// Fase 4 (NVIDIA embeddings) esté lista.
//
// Fórmula RRF:
//   score(d) = sum(1 / (k + rank_i(d))) para cada ranking i
func (b *Buscador) BuscarHibrido(query string, topK int) []ResultadoBusqueda {
        if topK <= 0 {
                return []ResultadoBusqueda{}
        }

        // Por ahora, solo BM25 (vector search llegará con Fase 4)
        resultados := b.BuscarBM25(query, topK)

        // Recalcular score como RRF (con un solo ranking, RRF reduce a 1/(k+rank))
        for i := range resultados {
                rank := resultados[i].RankBM25
                if rank > 0 {
                        resultados[i].Score = 1.0 / float64(rrfK+rank)
                }
        }

        // Re-ordenar por score RRF (que ya está en orden descendente gracias al rank BM25)
        sort.Slice(resultados, func(i, j int) bool {
                return resultados[i].Score > resultados[j].Score
        })

        return resultados
}

// BuscarHibridoConVectores es el stub para cuando los embeddings estén disponibles.
// Recibe un ranking de vectores pre-computado y lo fusiona con BM25 vía RRF.
func (b *Buscador) BuscarHibridoConVectores(query string, topK int, vectorRanking []ResultadoBusqueda) []ResultadoBusqueda {
        bm25Resultados := b.BuscarBM25(query, topK*2) // amplio para mejor fusión

        // Construir mapa de rank por ID
        bm25Rank := make(map[string]int)
        for i, r := range bm25Resultados {
                bm25Rank[r.Fragmento.ID] = i + 1
        }

        vectorRank := make(map[string]int)
        vectorScore := make(map[string]float64)
        for i, r := range vectorRanking {
                vectorRank[r.Fragmento.ID] = i + 1
                vectorScore[r.Fragmento.ID] = r.ScoreVector
        }

        // Combinar IDs de ambos rankings
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

                // Tomar el fragmento (de BM25 o de vector)
                var frag FragmentoBuscable
                b.mu.RLock()
                if f, ok := b.fragmentos[id]; ok {
                        frag = f
                }
                b.mu.RUnlock()
                if frag.ID == "" {
                        // Buscar en vectorRanking
                        for _, r := range vectorRanking {
                                if r.Fragmento.ID == id {
                                        frag = r.Fragmento
                                        break
                                }
                        }
                }

                resultados = append(resultados, ResultadoBusqueda{
                        Fragmento:   frag,
                        Score:       score,
                        ScoreBM25:   0, // se puede llenar si se quiere
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
// CONSULTAS
// ═══════════════════════════════════════════════════════

// Estadisticas retorna métricas del índice.
func (b *Buscador) Estadisticas() EstadisticasBuscador {
        b.mu.RLock()
        defer b.mu.RUnlock()
        return EstadisticasBuscador{
                TotalFragmentos:  b.totalFragmentos,
                TerminosUnicos:   len(b.indice),
                PromedioLongitud: b.avgdl,
        }
}

// Total retorna el número de fragmentos indexados.
func (b *Buscador) Total() int {
        b.mu.RLock()
        defer b.mu.RUnlock()
        return b.totalFragmentos
}

// ═══════════════════════════════════════════════════════
// TOKENIZACIÓN
// ═══════════════════════════════════════════════════════

// tokenizar divide un texto en tokens normalizados.
//   1. Lowercase
//   2. Reemplazar no-alfanuméricos por espacios (excepto _ . /)
//   3. Separar camelCase (aunque lowercase ya no haya mayúsculas,
//      dejamos el código por si se invoca directo)
//   4. Split por espacios
//   5. Para cada token, separar adicionalmente por _ . /
//   6. Eliminar stopwords y tokens de 1 caracter
func tokenizar(texto string) []string {
        texto = strings.ToLower(texto)

        // Reemplazar caracteres no alfanuméricos por espacios
        // (mantener underscore y punto para rutas de archivos)
        var b strings.Builder
        for _, c := range texto {
                if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '/' {
                        b.WriteRune(c)
                } else {
                        b.WriteRune(' ')
                }
        }
        texto = b.String()

        // Separar camelCase (en lowercase no hace nada, pero se mantiene
        // para compatibilidad futura con tokenización pre-lowercase)
        texto = separarCamelCase(texto)

        // Tokenizar por espacios
        parts := strings.Fields(texto)

        // Filtrar stopwords y tokens cortos
        var tokens []string
        for _, p := range parts {
                // Separar por underscore, punto y slash para indexar paths y snake_case
                for _, sub := range splitBy(p, "_", ".", "/") {
                        if len(sub) <= 1 {
                                continue
                        }
                        if esStopword(sub) {
                                continue
                        }
                        tokens = append(tokens, sub)
                }
        }

        return tokens
}

// separarCamelCase inserta guiones bajos entre palabras camelCase.
// "GetUserName" → "get_user_name"
// "HTTPServer" → "http_server"
// "user_id" → "user_id" (sin cambios)
func separarCamelCase(s string) string {
        var b strings.Builder
        runes := []rune(s)
        for i, r := range runes {
                if i == 0 {
                        b.WriteRune(r)
                        continue
                }
                prev := runes[i-1]
                // Insertar _ entre minúscula y mayúscula: "userName" → "user_Name"
                if prev >= 'a' && prev <= 'z' && r >= 'A' && r <= 'Z' {
                        b.WriteRune('_')
                }
                // Insertar _ entre letras y números: "version2" → "version_2"
                if (prev >= 'a' && prev <= 'z') && (r >= '0' && r <= '9') {
                        b.WriteRune('_')
                }
                b.WriteRune(r)
        }
        return b.String()
}

// splitBy divide un string por múltiples separadores.
func splitBy(s string, seps ...string) []string {
        result := []string{s}
        for _, sep := range seps {
                var nuevo []string
                for _, r := range result {
                        nuevo = append(nuevo, strings.Split(r, sep)...)
                }
                result = nuevo
        }
        return result
}

// esStopword retorna true si el token es una palabra común que debe ignorarse.
// Lista curada de stopwords en inglés + español + lenguajes de programación.
var stopwords = map[string]bool{
        // Inglés
        "the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
        "is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
        "have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
        "will": true, "would": true, "could": true, "should": true, "may": true,
        "might": true, "must": true, "shall": true, "can": true, "need": true,
        "of": true, "in": true, "on": true, "at": true, "to": true, "for": true,
        "with": true, "by": true, "from": true, "as": true, "into": true,
        "this": true, "that": true, "these": true, "those": true, "it": true,
        "its": true, "they": true, "them": true, "their": true, "we": true,
        "us": true, "our": true, "you": true, "your": true, "he": true, "she": true,
        "his": true, "her": true, "if": true, "then": true, "else": true, "when": true,
        "where": true, "why": true, "how": true, "all": true, "any": true, "both": true,
        "each": true, "few": true, "more": true, "most": true, "other": true, "some": true,
        "such": true, "no": true, "not": true, "only": true, "own": true, "same": true,
        "so": true, "than": true, "too": true, "very": true, "just": true,
        // Español
        "el": true, "la": true, "los": true, "las": true, "un": true, "una": true,
        "unos": true, "unas": true, "y": true, "pero": true,
        "es": true, "son": true, "fue": true, "fueron": true, "ser": true,
        "de": true, "del": true, "al": true, "para": true,
        "con": true, "por": true, "sin": true, "sobre": true, "entre": true,
        "este": true, "esta": true, "estos": true, "estas": true, "eso": true,
        "esa": true, "esos": true, "esas": true, "su": true, "sus": true,
        // Lenguajes de programación (palabras clave comunes)
        "func": true, "function": true, "def": true, "fn": true, "method": true,
        "return": true, "returns": true, "var": true, "let": true,
        "type": true, "struct": true, "class": true, "interface": true,
        "import": true, "package": true, "module": true,
        "public": true, "private": true, "protected": true, "static": true,
        "void": true, "nil": true, "null": true, "none": true,
        "while": true, "switch": true, "case": true, "default": true, "break": true,
        // Misc
        "http": true, "https": true, "www": true, "com": true, "org": true,
}

func esStopword(token string) bool {
        return stopwords[token]
}

// String retorna una representación legible del buscador.
func (b *Buscador) String() string {
        stats := b.Estadisticas()
        return fmt.Sprintf("Buscador: %d fragmentos, %d términos únicos, avgdl=%.1f",
                stats.TotalFragmentos, stats.TerminosUnicos, stats.PromedioLongitud)
}
