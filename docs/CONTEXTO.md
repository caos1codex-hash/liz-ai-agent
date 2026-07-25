# Sistema de Memoria de Liz — Arquitectura World-Class

> **Este documento describe el sistema de memoria unificado de Liz, que combina
> lo mejor de Claude Code, Cursor, Aider, GitHub Copilot, Continue.dev y
> Sourcegraph Cody en una sola arquitectura coherente.**

---

## 1. Análisis Competitivo

| Herramienta | Mejor feature | Limitación |
|---|---|---|
| **Claude Code** | tree-sitter AST real, símbolos indexados | Solo Claude, sin grafo de código |
| **Cursor** | Embeddings + hybrid search | Closed-source, depende de OpenAI |
| **Aider** | Repository Map compacto con PageRank | Sin búsqueda semántica |
| **GitHub Copilot** | Locality weighting (recencia) | Solo VS Code, sin RAG real |
| **Continue.dev** | Code graph + RAG configurable | Setup complejo |
| **Sourcegraph Cody** | Symbol search + reference graph | Enterprise, pesado |

---

## 2. Arquitectura Unificada — 7 Capas

```
┌─────────────────────────────────────────────────────────────┐
│  Capa 7: CONTEXT PACKER                                      │
│  Empaqueta contexto óptimo para el modelo (token-aware)      │
└─────────────────────────────────────────────────────────────┘
                            ▲
┌─────────────────────────────────────────────────────────────┐
│  Capa 6: REPOSITORY MAP (Aider-style)                        │
│  Compact signature-only view with importance ranking         │
└─────────────────────────────────────────────────────────────┘
                            ▲
┌─────────────────────────────────────────────────────────────┐
│  Capa 5: LLM SUMMARIES (deferred, cached)                    │
│  Resúmenes semánticos generados por LLM, cache permanente    │
└─────────────────────────────────────────────────────────────┘
                            ▲
┌─────────────────────────────────────────────────────────────┐
│  Capa 4: HYBRID SEARCH (BM25 + Vector)                       │
│  Búsqueda keyword exacta + similitud semántica + RRF fusion  │
└─────────────────────────────────────────────────────────────┘
                            ▲
┌─────────────────────────────────────────────────────────────┐
│  Capa 3: CODE GRAPH (PageRank over imports/calls)            │
│  Importancia de símbolos por centralidad de grafo             │
└─────────────────────────────────────────────────────────────┘
                            ▲
┌─────────────────────────────────────────────────────────────┐
│  Capa 2: SYMBOL TABLE (tree-sitter / go/ast)                 │
│  AST real: funciones, tipos, métodos con firmas y docstrings │
└─────────────────────────────────────────────────────────────┘
                            ▲
┌─────────────────────────────────────────────────────────────┐
│  Capa 1: FRAGMENTOS (inmutables, persistidos)                │
│  Atomic code units with content + metadata                   │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Detalle por Capa

### Capa 1: Fragmentos (ya existente, mejorada)
- Unidades inmutables de código
- ID por SHA-256 del contenido
- Persistencia individual en JSON
- **Mejora Fase 3.5**: índice en memoria ruta→[]id

### Capa 2: Symbol Table (NUEVO)
Reemplaza los fragmentadores regex por **AST parsing real**.

- **Go**: usa `go/parser` + `go/ast` (stdlib, sin CGO)
- **Otros lenguajes**: tree-sitter (cuando CGO disponible) o regex mejorado
- Por cada símbolo extrae:
  - Nombre
  - Firma completa (`func (s *Server) Handle(ctx context.Context, req *Request) (*Response, error)`)
  - Docstring (comentarios `//` antes del símbolo)
  - Receiver (para métodos)
  - Parámetros y retornos tipados
  - Líneas de inicio y fin
  - Símbolos importados (resueltos a rutas internas)

### Capa 3: Code Graph (NUEVO)
Grafo dirigido de dependencias entre archivos y símbolos.

- **Nodos**: archivos (y opcionalmente símbolos)
- **Aristas**: imports resueltos a archivos internos del proyecto
- **Algoritmo**: PageRank iterativo (50 iteraciones, damping 0.85)
- **Salida**: score de importancia por archivo (0.0 - 1.0)
- Archivos "hub" (mucho importados) reciben score alto
- Archivos leaf (tests, mains) reciben score bajo

### Capa 4: Hybrid Search (NUEVO)
Búsqueda que combina exactitud (BM25) con semántica (embeddings).

- **BM25** (pure Go, sin dependencias):
  - Tokenización: lowercase + split por no-alfanumérico
  - Stopwords: lista de 50+ palabras comunes en 5 lenguajes
  - Inverted index: término → [(fragment_id, tf)]
  - Scoring: BM25 estándar (k1=1.5, b=0.75)
- **Vector search** (opcional, cuando hay API key NVIDIA):
  - Modelo: `nvidia/nv-embed-v1` (cuando Fase 4 esté lista)
  - Embeddings de 1024 dim por fragmento
  - Similitud coseno
- **Reciprocal Rank Fusion (RRF)**: combina ambos rankings
  - `score(d) = sum(1 / (k + rank_i(d)))` para cada ranking i, k=60

### Capa 5: LLM Summaries (NUEVO, deferred)
Resúmenes semánticos generados por LLM, cacheados permanentemente.

- **Trigger**: la primera vez que un fragmento es accedido
- **Formato**: 1-2 oraciones describiendo qué hace y por qué importa
- **Storage**: `.liz/resumenes_llm/<fragment_id>.txt`
- **Cache**: permanente, solo se regenera si el contenido cambia
- **Fallback**: si no hay LLM disponible, usa el resumen regex (Capa 1)

### Capa 6: Repository Map (NUEVO, Aider-style)
Vista compacta del proyecto, solo firmas + descripciones cortas.

- **Por archivo**: lista de símbolos con firma de una línea
- **Orden**: por importancia (PageRank score descendente)
- **Token budget**: para 500 tokens, incluye top-20 archivos
- **Formato**:
  ```
  src/auth/jwt.go:
    func GenerateToken(userID string, claims map[string]interface{}) (string, error)
    func ValidateToken(token string) (*Claims, error)
    type Claims struct { UserID string; Exp int64 }
  
  src/auth/oauth.go:
    func HandleOAuthCallback(w http.ResponseWriter, r *http.Request)
  ```

### Capa 7: Context Packer (NUEVO)
Ensambla el contexto óptimo para el modelo dado un intent y token budget.

- **Input**: `(proyecto, query/intent, max_tokens)`
- **Output**: string listo para incluir en prompt + metadata
- **Estrategia en capas**:
  1. Siempre incluir Repository Map compacto (~500 tokens)
  2. Top-K fragmentos por hybrid search relevance (~70% del budget)
  3. Imports directos de esos fragmentos (~20% del budget)
  4. Archivos recientemente editados (locality bias, ~10% del budget)
- **Token counter**: aproximado (4 chars ≈ 1 token)
- **Retorna**: `ContextoEmpaquetado{ Contenido, TokensUsados, FragmentosIncluidos[] }`

---

## 4. Comparación con la Fase 3 anterior

| Aspecto | Fase 3 (anterior) | Fase 3.5 (world-class) |
|---|---|---|
| Fragmentación | Regex (frágil) | go/ast para Go, tree-sitter-ready |
| Búsqueda | Substring case-insensitive | BM25 + vector + RRF |
| Importancia | Alfabética | PageRank sobre grafo de imports |
| Resúmenes | Regex (exportados/importados) | LLM diferido + cache permanente |
| Mapa del repo | Mapa completo (caro) | Compact signature-only |
| Context packing | No existe | Token-aware layered packing |
| Granularidad | Solo fragmento | 4 niveles: 1-línea → párrafo → fragmento → archivo |

---

## 5. Paquetes Go

```
internal/nucleo/contexto/
├── contexto.go                  # Coordinador (existente, extendido)
├── mapa/                        # Mapa tradicional (existente)
├── fragmentos/                  # Fragmentos inmutables (existente)
├── indice/                      # Índice + árbol (existente)
├── resumen/                     # Resumen regex (existente)
├── arbol_ast/                   # NUEVO: AST parsing (go/ast)
├── grafo/                       # NUEVO: Code graph + PageRank
├── buscador/                    # NUEVO: BM25 + hybrid search
├── mapa_repo/                   # NUEVO: Aider-style compact map
└── empaquetador/                # NUEVO: Context packer
```

---

## 6. API Endpoints nuevos

```
GET  /api/v1/contexto/proyectos/{nombre}/simbolos       # symbol table
GET  /api/v1/contexto/proyectos/{nombre}/grafo          # code graph
GET  /api/v1/contexto/proyectos/{nombre}/importancia    # PageRank scores
POST /api/v1/contexto/proyectos/{nombre}/buscar-hibrido # body: {query, top_k}
GET  /api/v1/contexto/proyectos/{nombre}/mapa-repo      # ?max_tokens=500
POST /api/v1/contexto/proyectos/{nombre}/empaquetar     # body: {query, max_tokens}
```

---

## 7. Filosofía de Implementación

1. **Pure Go优先**: sin CGO cuando sea posible (go/ast está en stdlib)
2. **Sin dependencias externas**: BM25 y PageRank implementados desde cero
3. **Lazy evaluation**: embeddings y LLM summaries se generan on-demand
4. **Cache agresivo**: todo lo caro se cachea permanentemente a disco
5. **Tests primero**: cada módulo tiene tests antes de integrarse
6. **Backward compatible**: las APIs existentes siguen funcionando

---

*Documento vivo. Última actualización: Fase 3.5 — Sistema de Memoria World-Class.*
