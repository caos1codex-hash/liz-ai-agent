# Análisis del Sistema de Memoria y Contexto de Liz vs. las Mejores Prácticas Actuales

> **Documento de análisis técnico** — compara la arquitectura actual de Liz (Fase 3.5)
> con los sistemas de memoria y contexto de referencia en 2025-2026.
>
> **Fecha:** 2026-07-25
> **Estado del arte comparado:** Mem0, Letta (MemGPT), Zep, LangGraph, Claude Code, Cursor, Aider, GitHub Copilot, Continue.dev, Sourcegraph Cody.

---

## 1. Resumen Ejecutivo

Liz ya implementó en la Fase 3.5 una arquitectura de contexto de **7 capas** inspirada en
lo mejor de Claude Code, Cursor, Aider, Copilot, Continue.dev y Cody. Es una base sólida
para búsqueda y recuperación de código, **pero aún no tiene un sistema de memoria conversacional
ni de agente**: no persiste turnos de chat, no extrae hechos del usuario, no mantiene
estado episódico entre sesiones y no integra embeddings vectoriales reales.

En la tabla siguiente se resume la brecha con la frontera actual:

| Dimensión | Liz 3.5 (hoy) | Mejor práctica 2025-2026 | Brecha |
|---|---|---|---|
| AST parsing | go/ast real para Go | tree-sitter multi-lenguaje | **Media** — solo Go |
| Code graph | PageRank sobre imports | PageRank + betweenness + LLM-scored centrality | **Baja** |
| Hybrid search | BM25 + RRF (vector-ready) | BM25 + dense vectors + cross-encoder re-ranker | **Alta** — sin embeddings reales |
| Repository map | Aider-style compact signatures | Igual + LLM-described sections | **Baja** |
| Context packing | 4 capas (mapa + frags + imports + recientes) | Igual + adaptive budget per intent | **Media** — capas 3 y 4 son stubs |
| Conversational memory | **No existe** | Mem0 / Letta / Zep | **Crítica** |
| Episodic memory | **No existe** | Letta core/archival blocks | **Crítica** |
| Semantic memory (facts) | **No existe** | Mem0 fact extraction + conflict resolution | **Crítica** |
| Session continuity | **No existe** | Zep temporal knowledge graph | **Alta** |
| Agent state / checkpoints | **No existe** | LangGraph checkpointer | **Alta** (Phase 7) |
| File-watch incremental | Reindex manual | fsnotify / inotify | **Media** |
| Multi-language AST | Solo Go | tree-sitter (Go, Python, JS/TS, Rust, Java, C/C++) | **Alta** |
| Embeddings vectoriales | Stub | nv-embed-v1, bge-m3, jina-v3 | **Alta** (Fase 4) |
| Re-ranking | No | bge-reranker-v2, jina-reranker | **Media** |
| Locality tracking | No implementado | mtime + git-blame + recently-touched | **Media** |

**Conclusión:** la base de contexto (capas 1-7) está bien hecha, pero le faltan
**(a) embeddings reales**, **(b) memoria conversacional** y **(c) completar los stubs
del empaquetador**. Estos son los próximos pasos críticos.

---

## 2. Comparación Detallada por Sistema de Referencia

### 2.1 vs. Mem0 (memoria para agentes LLM)

**Filosofía Mem0:** "memoria como capa" — extrae hechos del diálogo, los guarda en
un grafo, los consolida y los recupera por relevancia temporal + semántica.

| Componente Mem0 | Liz hoy | Acción |
|---|---|---|
| `add(messages, user_id)` — ingesta de mensajes | ❌ No existe | Crear `internal/nucleo/memoria/` |
| Extracción de hechos vía LLM ("el usuario prefiere Go", "trabaja en project X") | ❌ No existe | Necesita orquestador Fase 4 |
| Resolución de conflictos (hecho nuevo reemplaza viejo) | ❌ No existe | Implementar `Memoria.resolver()` |
| Memoria episódica vs. semántica vs. procedimental | ❌ No existe | Taxonomía de 3 tipos |
| Vector store + graph store híbrido | ❌ No existe | Reusar `buscador` para vectores |
| Búsqueda `search(query, user_id)` con re-ranking | ❌ No existe | Recuperación contextual |
| Decay temporal + consolidación nocturna | ❌ No existe | Job periódico de consolidación |

**Brecha:** CRÍTICA. Liz no tiene memoria de conversación.

### 2.2 vs. Letta (MemGPT)

**Filosofía Letta:** memoria virtual paginada — el LLM decide qué pasa de "core memory"
(en-contexto) a "archival" (vector store externo) y viceversa, vía function calls.

| Componente Letta | Liz hoy | Acción |
|---|---|---|
| Core memory blocks (persona, human, project) | ❌ No existe | Definir bloques editables |
| Archival memory (vector store) | ❌ No existe (stub BM25) | Integrar nv-embed-v1 |
| Recall memory (últimos N turnos) | ❌ No existe | Buffer circular persistido |
| `core_memory_append/replace` function calls | ❌ No existe | Exponer como herramientas Fase 5 |
| `archival_memory_insert/search` | ❌ No existe | Exponer como herramientas |
| Memory eviction LLM-managed | ❌ No existe | Delegar al orquestador |
| State checkpointing | ❌ No existe | `estado.json` + LangGraph-style |

**Brecha:** CRÍTICA. Letta demuestra que la memoria debe ser **agent-managed**, no solo
persistida. Liz debería exponer funciones `memoria_*` al orquestador.

### 2.3 vs. Zep (long-term memory service)

**Filosofía Zep:** "memory as a service" — extrae entidades temporalmente, mantiene
un grafo de conocimiento temporal, sirve contexto a múltiples sesiones.

| Componente Zep | Liz hoy | Acción |
|---|---|---|
| Temporal knowledge graph (entidades + hechos con timestamp) | ❌ No existe | `internal/nucleo/memoria/grafo_hechos.go` |
| Entity extraction + merging | ❌ No existe | Stub LLM (Fase 4) |
| Session continuity (mismo usuario, múltiples chats) | ❌ No existe | `usuario_id` + `sesion_id` |
| Automatic summarization on session end | ❌ No existe | Hook de cierre |
| Vector + graph hybrid retrieval | ❌ No existe | Reusar `buscador` + grafo de hechos |

**Brecha:** ALTA. Zep muestra que la memoria debe ser **multi-sesión** y **temporal**.

### 2.4 vs. LangGraph (agent orchestration + checkpointing)

**Filosofía LangGraph:** grafos de estado con checkpoint persistido — el agente puede
resumir tras N turnos, volver a estados anteriores, ejecutar tools en paralelo.

| Componente LangGraph | Liz hoy | Acción |
|---|---|---|
| `StateGraph` con nodos y edges | ❌ No existe | `internal/nucleo/pipeline/` (Fase 7) |
| `Checkpointer` (persiste estado entre pasos) | ❌ No existe | `estado.json` por run |
| `MemorySaver` / `SqliteSaver` | ❌ No existe | Adaptar `memoria` package |
| Tool node with parallel execution | ❌ No existe | Fase 5 |
| Human-in-the-loop interrupts | ❌ No existe | Futuro |

**Brecha:** ALTA pero corresponde a Fase 7 (pipeline de chat).

### 2.5 vs. Claude Code (Anthropic CLI)

**Filosofía Claude Code:** tree-sitter AST + locality bias + adaptive context.

| Componente Claude Code | Liz hoy | Acción |
|---|---|---|
| tree-sitter multi-lenguaje | ❌ Solo Go (go/ast) | **Integrar tree-sitter** (CGO) o `python-tree-sitter` bindings |
| Recently edited files tracking | ❌ Stub en empaquetador | Implementar tracker real |
| Adaptive token budget por intent | ❌ Presupuesto fijo 8000 | Detectar intent (code/qa/debug) |
| File-watch incremental reindex | ❌ Manual `reindexar` | fsnotify |
| Slash commands con contexto inyectado | ❌ No existe | Fase 7 (pipeline) |
| Sub-agent dispatch | ❌ No existe | Futuro |

**Brecha:** MEDIA. Los principales gaps son tree-sitter y file-watch.

### 2.6 vs. Cursor (IDE AI)

**Filosofía Cursor:** dense embeddings + codebase-wide retrieval + re-ranking.

| Componente Cursor | Liz hoy | Acción |
|---|---|---|
| Embeddings reales (OpenAI/Voyage/Jina) | ❌ Stub | **nv-embed-v1** (Fase 4) |
| Vector index persistido (FAISS/Chroma) | ❌ Solo BM25 in-memory | `embeddings.db` SQLite |
| Cross-encoder re-ranking | ❌ No | bge-reranker-v2 (futuro) |
| Codebase Q&A con citations | ❌ No | Fase 7 |
| Long-context re-prompting (aider-style) | ❌ No | Futuro |

**Brecha:** ALTA. Sin embeddings no hay búsqueda semántica real.

### 2.7 vs. Aider

**Filosofía Aider:** repository map compacto + PageRank + git-aware editing.

| Componente Aider | Liz hoy | Estado |
|---|---|---|
| Repository Map (signature-only) | ✅ Implementado | Igual o mejor |
| PageRank para importancia | ✅ Implementado | Igual |
| Token budget para repo map | ✅ Implementado | Igual |
| Git-aware diff parsing | ❌ No usa git | Agregar `git diff` tracking |
| Multi-language AST | ❌ Solo Go | tree-sitter pendiente |

**Brecha:** BAJA. Liz ya iguala a Aider en el núcleo de repository map.

### 2.8 vs. GitHub Copilot / Continue.dev / Cody

| Componente | Liz hoy | Estado |
|---|---|---|
| Locality weighting (archivos recientes) | ❌ Stub | Implementar tracker |
| Code graph + reference traversal | ✅ Implementado | Igual o mejor |
| Symbol search | ✅ Implementado | Igual |
| Multi-language | ❌ Solo Go AST | tree-sitter pendiente |
| LSP integration | ❌ No | Futuro |
| Enterprise scale (Sourcegraph Cody) | ❌ No | No es objetivo |

**Brecha:** MEDIA. La base es sólida, falta multi-lenguaje.

---

## 3. Hallazgos Críticos en el Código Actual

### 3.1 El Empaquetador tiene stubs no resueltos

Revisando `internal/nucleo/contexto/empaquetador/empaquetador.go`:

**Capa 3 (Imports expandidos)** — Líneas 206-220:
```go
for ruta := range archivosIncluidos {
    if tokensImportsUsados >= presupuestoImports {
        break
    }
    vecinos := datos.Grafo.Vecinos(ruta)
    for _, vecino := range vecinos {
        if tokensImportsUsados >= presupuestoImports {
            break
        }
        // Tomar el primer fragmento del archivo vecino
        // (en una implementación real, usaríamos el almacén para listar)
        // Por simplicidad, omitimos la expansión real aquí
        _ = vecino
    }
}
```

**Capa 4 (Archivos recientes)** — Líneas 231-244:
```go
for _, ruta := range req.ArchivosRecientes {
    // En una implementación real, buscaríamos los fragmentos de esta ruta
    // en el almacén. Por simplicidad, solo mencionamos la ruta.
    entrada := fmt.Sprintf("- `%s`\n", ruta)
    ...
}
```

**Impacto:** el empaquetador afirma hacer 4 capas pero realmente solo hace 2 (mapa + BM25).
El `TokensImports` y `TokensRecientes` siempre son 0. El usuario paga el costo del
presupuesto (30% + 50% + 15% + 5%) pero solo recibe el 80% del valor real.

### 3.2 El Buscador tiene stub de embeddings

`buscador.go` líneas 77-79:
```go
// Vector embeddings (futuro, Fase 4)
// embeddings map[string][]float32 // id → vector
// clienteEmbeddings *nvidia.ClienteEmbeddings
```

`BuscarHibrido` solo usa BM25 — cuando llega Fase 4, deberá activar `BuscarHibridoConVectores`
con embeddings reales.

### 3.3 No existe paquete `memoria`

No hay `internal/nucleo/memoria/`. Toda la "memoria" actual es de **código**
(fragments, repo map). No hay memoria de **conversación**, **usuario** o **hechos**.

### 3.4 No hay tracker de archivos recientes

El empaquetador recibe `ArchivosRecientes []string` pero nadie lo llena. Falta un
`TrackerEdiciones` que registre qué archivos fueron editados en los últimos N minutos.

### 3.5 PageRank está bien implementado pero no se persiste

`Grafo.CalcularImportancia` recalcula en cada `IndexarProyecto`. Para proyectos grandes
(miles de archivos), 50 iteraciones de PageRank podrían ser costosas. Falta caché en disco.

### 3.6 Búsqueda no soporta fuzzy matching

BM25 exige match exacto de términos. No hay soporte para:
- Edit distance (typo tolerance)
- Stemming (Porter / Snowball)
- Sinónimos (WordNet / domain-specific)

### 3.7 El Grafo solo modela imports de Go

`construirGrafo` solo parsea imports `.go`. Para Python (`import foo`), JS (`require`/`import`),
Rust (`use`), el grafo queda vacío → PageRank no aporta nada para proyectos no-Go.

### 3.8 Sin observabilidad del sistema de memoria

No hay métricas de:
- Latencia de búsqueda
- Hit rate del caché de resúmenes
- Distribución de scores BM25
- Cobertura del repository map (% archivos incluidos)

---

## 4. Roadmap de Mejoras (Priorizado)

### Prioridad CRÍTICA (bloquea Fase 7 - chat)

1. **Crear paquete `internal/nucleo/memoria/`** — memoria conversacional
   - Sesiones (uuid, usuario, timestamp_inicio, timestamp_fin)
   - Mensajes (rol, contenido, timestamp, metadata)
   - Hechos (sujeto, predicado, objeto, confianza, timestamp)
   - Persistencia JSON en `~/.liz/memoria/`
   - Recuperación por similitud (reusar `buscador.Buscador`)

2. **Completar el empaquetador** — quitar stubs
   - Capa 3: expandir imports reales vía `Almacen.ObtenerPorRuta`
   - Capa 4: tracker de archivos editados (`~/.liz/memoria/ediciones.json`)
   - Profundidad configurable de expansión

3. **Stub de cliente NVIDIA embeddings** — `internal/nucleo/orquestador/embeddings.go`
   - Interface `ClienteEmbeddings`
   - Implementación `nv-embed-v1` (NVIDIA API)
   - Integración en `buscador.Buscador.Indexar()` (lazy: se generan on-demand)

### Prioridad ALTA (calidad de contexto)

4. **Tracker de archivos recientes** — `~/.liz/contexto/ediciones.json`
   - Registro mtime + git-status + LLM-touched
   - Hook para herramientas Fase 5 (cuando editan archivo, actualizar tracker)

5. **Persistencia de PageRank** — cachear scores en `~/.liz/contexto/proyectos/<n>/.liz/pagerank.json`
   - Solo recalcular si `go.sum` o imports cambiaron

6. **Multi-language AST** — tree-sitter para Python, JS/TS, Rust, Java
   - Opción A: CGO + `github.com/smacker/go-tree-sitter` (mejor rendimiento)
   - Opción B: subprocess a `tree-sitter` CLI (más simple, menos performance)

7. **Re-ranking** — después de BM25, aplicar cross-encoder
   - Stub LLM (Fase 4): prompt "rank these N snippets by relevance to query Q"
   - Modelo: `nvidia/nv-rerankqa-mistral4b-v3` (cuando esté disponible)

### Prioridad MEDIA (escalabilidad)

8. **File-watch incremental** — `fsnotify` watcher
   - Por archivo: re-fragmentar + actualizar índice + grafo + buscador
   - Debounce 500ms para no spamear reindex

9. **Stemming + fuzzy matching** en `buscador.tokenizar`
   - Porter stemmer para inglés
   - Distancia de Levenshtein ≤ 2 para typo tolerance

10. **Grafo multi-lenguaje** — parsear imports de Python, JS, Rust, Java
    - Python: regex sobre `^import` / `^from X import Y`
    - JS: regex sobre `require\(...\)` y `import ... from "..."`
    - Rust: regex sobre `use crate::...` y `use ::...`

### Prioridad BAJA (polish)

11. **Métricas Prometheus** — `/metrics` endpoint
12. **Cache LRU** para fragmentos frecuentes
13. **Compresión** de fragmentos antiguos (gzip)
14. **SQLite opcional** para proyectos grandes (>10k archivos)

---

## 5. Decisión de Implementación Inmediata

Basado en el análisis, las próximas iteraciones de trabajo serán:

| Iteración | Tarea | Commit esperado | Issue |
|---|---|---|---|
| 1 | Este documento de análisis | `docs: analisis memoria vs mejores practicas` | #19 |
| 2 | Completar empaquetador (capas 3+4) + tests | `feat(empaquetador): implementar capas 3 y 4` | #11 |
| 3 | Crear `internal/nucleo/memoria/` (sesiones + mensajes + hechos) + tests | `feat(memoria): memoria conversacional world-class` | nuevo issue |
| 4 | Crear `internal/nucleo/orquestador/` (cliente NVIDIA + selección + fallback) | `feat(orquestador): Fase 4 - multi-modelo NVIDIA` | #12 |
| 5 | Integrar orquestador en servidor + endpoints + main.go | `feat(servidor): integrar orquestador NVIDIA` | #12 |
| 6 | Stub embeddings nv-embed-v1 + integración en buscador | `feat(buscador): embeddings nv-embed-v1` | #12 |

Cada iteración se sube a GitHub inmediatamente después de pasar los tests.

---

## 6. Comparación Visual de Arquitecturas

```
MEM0                              LETTA                            ZEP
┌─────────────────────┐          ┌─────────────────────┐         ┌─────────────────────┐
│  User messages      │          │  Core memory        │         │  Sessions           │
│         ↓           │          │  (persona, human)   │         │         ↓           │
│  Fact extraction    │          │         ↓           │         │  Entity extraction  │
│         ↓           │          │  LLM-managed paging │         │         ↓           │
│  Graph + vector     │          │         ↓           │         │  Temporal KG        │
│         ↓           │          │  Archival (vector)  │         │         ↓           │
│  Retrieval + rerank │          │  Recall (last N)    │         │  Hybrid retrieval   │
└─────────────────────┘          └─────────────────────┘         └─────────────────────┘

LIZ (objetivo post-Fase 4)
┌──────────────────────────────────────────────────────────────────────────┐
│  CODE MEMORY (ya existe)              CONVERSATIONAL MEMORY (nuevo)      │
│  ┌──────────────────────────┐         ┌──────────────────────────┐      │
│  │ Fragments (SHA-256)      │         │ Sessions (uuid, user)    │      │
│  │ AST symbols (go/ast)     │         │ Messages (rol, ts)       │      │
│  │ Code graph (PageRank)    │         │ Facts (s,p,o,confianza)  │      │
│  │ BM25 + vector (RRF)      │◄───────►│ Vector index (nv-embed)  │      │
│  │ Repository map           │  share  │ Episodic buffer (last N) │      │
│  │ Context packer           │  busca- │ Semantic consolidation  │      │
│  └──────────────────────────┘  dor    └──────────────────────────┘      │
└──────────────────────────────────────────────────────────────────────────┘
```

La pieza faltante es la columna derecha: **memoria conversacional**. El sistema actual
está optimizado para "recordar código" pero no para "recordar al usuario".

---

## 7. Conclusión

Liz tiene un **sistema de contexto de código de nivel world-class** (capas 1-7
inspiradas en Aider + Claude Code + Cursor). El trabajo hecho en Fase 3.5 es
sólido y comparable a lo mejor del estado del arte.

**Pero** el sistema de memoria — entendido como "memoria de conversación, usuario,
hechos y sesiones" — **no existe**. Es la brecha más crítica y debe cerrarse antes
de la Fase 7 (pipeline de chat), porque el pipeline necesita persistir turnos,
recordar preferencias del usuario y mantener contexto entre sesiones.

El camino recomendado es:

1. **Primero:** completar los stubs del empaquetador (Iteración 2) — costo bajo,
   impacto inmediato en la calidad del contexto entregado al LLM.
2. **Segundo:** crear `internal/nucleo/memoria/` (Iteración 3) — sin dependencias
   externas, puramente Go, JSON persistido.
3. **Tercero:** crear el orquestador NVIDIA (Iteración 4-5) — desbloquea Fase 4.
4. **Cuarto:** integrar embeddings (Iteración 6) — activa búsqueda semántica real.

Con estas 4 iteraciones Liz pasa de "sistema de contexto de código excelente" a
"agente con memoria y contexto world-class", comparable a Mem0 + Claude Code
combinados.

---

*Documento vivo. Se actualizará después de cada iteración para reflejar el estado real.*
