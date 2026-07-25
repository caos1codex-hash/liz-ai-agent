# Empaquetador de Contexto

> Sistema de ensamblaje de contexto en 4 capas para LLMs.
> Paquete: `internal/nucleo/contexto/empaquetador/`

## Visión General

El empaquetador es responsable de construir el contexto óptimo que se envía al
LLM junto con el prompt del usuario. En lugar de volcar todo el repositorio
en el contexto, selecciona estratégicamente qué información incluir respetando
un presupuesto de tokens, inspirado en el enfoque de Claude Code.

El resultado es un string con formato Markdown listo para inyectar en el prompt
y metadata detallada sobre qué se incluyó, cuántos tokens se usaron y de dónde
provino cada fragmento.

## Ensamblaje en 4 Capas

El contexto se construye en orden de prioridad, donde cada capa recibe un
porcentaje fijo del presupuesto total de tokens:

```
Presupuesto total de tokens (default: 8000)
├── 30% → Capa 1: Repository Map
├── 50% → Capa 2: Fragmentos relevantes (BM25 + RRF)
├── 15% → Capa 3: Imports expandidos (BFS sobre grafo)
└──  5% → Capa 4: Archivos recientes (locality bias)
```

### Capa 1 — Repository Map (30%)

Vista compacta del repositorio con solo firmas de símbolos (no código
completo), ordenada por importancia PageRank descendente. Permite al
modelo "ver todo el proyecto" sin saturar el contexto.

- **Fuente**: `mapa_repo.MapaRepo.FormatoTexto()`
- **Truncamiento**: se corta con `truncarATokens` si excede el presupuesto
- **Constante**: `pctMapaRepo = 30`

### Capa 2 — Fragmentos Relevantes (50%)

Búsqueda híbrida BM25 + RRF sobre la query del usuario. Se incluyen los
fragmentos con código completo de los resultados más relevantes.

- **Fuente**: `BuscadorHibrido.BuscarHibrido(query, topK)`
- **Top-K dinámico**: `presupuestoFragmentos / 500` (clamped a [1, 20])
- **Deduplicación**: se trackingea por ID para no repetir en capas posteriores
- **Constante**: `pctFragmentos = 50`

### Capa 3 — Imports Expandidos (15%)

Expansión BFS sobre el grafo de dependencias. Para cada archivo incluido
como fragmento relevante (Capa 2), se consultan sus vecinos en el grafo
(imports directos) y se traen sus fragmentos vía callback.

- **Fuente**: `Grafo.Vecinos(ruta)` + callback `ObtenerFragmentosPorRuta`
- **Profundidad configurable**: `SolicitudEmpaquetado.ProfundidadImports`
  - `0` = capa omitida
  - `1` = imports directos (default)
  - `2` = imports transitivos
- **Deduplicación**: respeta `fragmentosYaIncluidos` de capas anteriores
- **Constante**: `pctImports = 15`

### Capa 4 — Archivos Recientes (5%)

Archivos editados recientemente por el usuario (locality bias, estilo
Copilot). Se incluyen sus fragmentos reales, no solo las rutas.

- **Fuente**: `SolicitudEmpaquetado.ArchivosRecientes` + callback `ObtenerFragmentosPorRuta`
- **Deduplicación**: si todos los fragmentos ya estaban incluidos, se
  menciona la ruta con nota "ya incluido arriba"
- **Constante**: `pctRecientes = 5`

## Asignación del Presupuesto de Tokens

| Capa | Porcentaje | Constante | Descripción |
|------|-----------|-----------|-------------|
| Repository Map | 30% | `pctMapaRepo` | Vista compacta de firmas |
| Fragmentos | 50% | `pctFragmentos` | Código relevante por búsqueda |
| Imports | 15% | `pctImports` | Dependencias expandidas por BFS |
| Recientes | 5% | `pctRecientes` | Archivos editados (locality bias) |

**Presupuesto por defecto**: 8000 tokens (`SolicitudEmpaquetado.PresupuestoTokens`).

Estimación de tokens: 4 caracteres ≈ 1 token (función
`mapa_repo.EstimarTokensTexto`).

## Interfaz BuscadorHibrido

El empaquetador no depende de un tipo concreto de buscador. En su lugar,
usa la interfaz mínima `BuscadorHibrido`:

```go
type BuscadorHibrido interface {
    BuscarHibrido(query string, topK int) []buscador.ResultadoBusqueda
}
```

Esto permite inyectar cualquier implementación que implemente `BuscarHibrido`:

- `*buscador.Buscador` — búsqueda BM25 pura
- `*buscador.BuscadorEmbeddings` — búsqueda híbrida BM25 + vectorial + RRF
- `*buscador.RerankerConLLM` — wrapper que aplica reranking LLM sobre cualquier `IBuscador`

El `RerankerConLLM` también implementa `BuscarHibrido`, por lo que satisface
la interfaz del empaquetador directamente.

## Tracker y Locality Bias

El **Tracker de Ediciones** (`internal/nucleo/contexto/tracker/`) mantiene un
cing buffer thread-safe de las últimas N rutas de archivos editados por el
usuario.

```go
type TrackerEdiciones struct {
    mu        sync.RWMutex
    maxItems  int                    // límite del ring buffer (default: 20)
    ediciones []RegistroEdicion
}

type RegistroEdicion struct {
    Ruta      string `json:"ruta"`
    Timestamp string `json:"timestamp"`
}
```

**Funcionalidad**:

- `RegistrarEdicion(ruta)` — agrega al ring buffer, elimina el más viejo si excede el límite
- `ObtenerRecientes(n)` — retorna las últimas N ediciones (más recientes primero)
- `Guardar(rutaArchivo)` / `Cargar(rutaArchivo)` — persistencia JSON atómica (write + rename)

**Uso en el empaquetador**: las rutas recientes del tracker se pasan como
`SolicitudEmpaquetado.ArchivosRecientes` para alimentar la Capa 4 (locality
bias), priorizando el código que el usuario está activamente modificando.

## Caché de PageRank

El grafo de dependencias (`internal/nucleo/contexto/grafo/`) implementa un
mecanismo de caché para los scores de PageRank con detección de cambios vía
hash SHA-256.

### Detección de Cambios

```go
func (g *Grafo) hashGrafo() string
```

Calcula un hash SHA-256 determinístico del estado completo del grafo:

1. Ordena alfabéticamente todos los nodos
2. Serializa cada nodo como `ruta|lenguaje|lineas|importancia|`
3. Ordena alfabéticamente todas las aristas por origen
4. Serializa cada arista como `origen->destino|`

### Flujo de Caché

```
ConstruirGrafo()
  → hashGrafo() actual
  → CargarPageRankCache(archivo)
    → hash coincide? → sí: aplicar scores cacheados (SKIP recálculo)
                       → no: recalcular PageRank (50 iteraciones, d=0.85)
                              → GuardarPageRankCache(archivo) con hash nuevo
```

La estructura persistida es:

```go
type pagerankCache struct {
    Hash   string             `json:"hash"`   // SHA-256 del estado del grafo
    Scores map[string]float64 `json:"scores"` // ruta → score normalizado
}
```

## Tipos Principales

### SolicitudEmpaquetado

Entrada del empaquetador:

```go
type SolicitudEmpaquetado struct {
    Proyecto           string   // nombre del proyecto
    Query              string   // intent del usuario
    PresupuestoTokens  int      // máximo tokens (default: 8000)
    ArchivosRecientes  []string // rutas editadas recientemente (Capa 4)
    ProfundidadImports int      // niveles de expansión de imports (default: 1)
}
```

### ContextoEmpaquetado

Resultado del empaquetador:

```go
type ContextoEmpaquetado struct {
    Contenido          string               `json:"contenido"`
    TokensUsados       int                  `json:"tokens_usados"
    PresupuestoTokens  int                  `json:"presupuesto_tokens"
    MapaRepoIncluido   bool                 `json:"mapa_repo_incluido"
    FragmentosIncluidos []FragmentoIncluido  `json:"fragmentos_incluidos"
    TokensMapaRepo     int                  `json:"tokens_mapa_repo"
    TokensFragmentos   int                  `json:"tokens_fragmentos"
    TokensImports      int                  `json:"tokens_imports"
    TokensRecientes    int                  `json:"tokens_recientes"
}
```

### DatosEmpaquetado

Datos pre-recolectados que el Coordinador pasa al empaquetador:

```go
type DatosEmpaquetado struct {
    MapaRepo                 *mapa_repo.MapaRepo
    Buscador                 BuscadorHibrido
    Grafo                    *grafo.Grafo
    ObtenerFragmento         func(id string) (buscador.FragmentoBuscable, bool)
    ObtenerFragmentosPorRuta func(ruta string) []buscador.FragmentoBuscable
}
```

## Reranking LLM

El `RerankerLLM` (`buscador/rerank_llm.go`) reordena los resultados de
búsqueda usando un LLM como juez de relevancia:

1. Toma los top-K resultados de BM25/vector search
2. Envía al LLM la query + fragmentos candidatos truncados a 500 chars
3. Parsea la respuesta esperando IDs ordenados por relevancia
4. Reordena los resultados; los no mencionados van al final

Degradación graceful: si el LLM falla, se usa el ranking original.

## Arquitectura de Componentes

```
                        ┌──────────────────────┐
                        │  Coordinador de       │
                        │  Contexto             │
                        └──────────┬───────────┘
                                   │
                    SolicitudEmpaquetado + DatosEmpaquetado
                                   │
                        ┌──────────▼───────────┐
                        │  Empaquetador         │
                        │  (empaquetador.go)    │
                        └──┬──────┬──────┬─────┘
                           │      │      │
                ┌──────────┘      │      └──────────┐
                ▼                 ▼                  ▼
        ┌──────────────┐ ┌──────────────┐  ┌──────────────┐
        │  mapa_repo   │ │  buscador    │  │   tracker    │
        │  (Capa 1)    │ │  (Capa 2)    │  │  (Capa 4)    │
        └──────────────┘ └──────┬───────┘  └──────────────┘
                               │
                        ┌──────▼───────┐
                        │    grafo     │
                        │  (Capa 3)    │
                        │  + PageRank  │
                        │  + SHA-256   │
                        └──────────────┘
```

## Ubicación en el Código

| Componente | Archivo |
|------------|--------|
| Empaquetador | `internal/nucleo/contexto/empaquetador/empaquetador.go` |
| Buscador (BM25) | `internal/nucleo/contexto/buscador/buscador.go` |
| Buscador (embeddings) | `internal/nucleo/contexto/buscador/embeddings.go` |
| Interfaz IBuscador | `internal/nucleo/contexto/buscador/ibuscador.go` |
| Reranker LLM | `internal/nucleo/contexto/buscador/rerank_llm.go` |
| Grafo + PageRank | `internal/nucleo/contexto/grafo/grafo.go` |
| Repository Map | `internal/nucleo/contexto/mapa_repo/mapa_repo.go` |
| Tracker | `internal/nucleo/contexto/tracker/tracker.go` |
| Coordinador | `internal/nucleo/contexto/contexto.go` |
