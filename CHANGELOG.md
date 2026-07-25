# Changelog — Liz AI Agent

## [0.4.1] — Fase 4.1: Embeddings nv-embed-v1 + Búsqueda Híbrida Real

> **Activa la búsqueda semántica real. Combina BM25 + vector + RRF
> usando NVIDIA nv-embed-v1 (1024 dimensiones).**

### Buscador con embeddings (`internal/nucleo/contexto/buscador/embeddings.go`)
- **Interfaz `EmbeddingsProvider`**: contrato limpio que cualquier provider
  de embeddings puede implementar (NVIDIA, OpenAI, Jina, etc.). El buscador
  no depende del paquete orquestador (clean dependency).
- **`BuscadorEmbeddings`**: extiende `Buscador` con índice vectorial
  paralelo al BM25. Mantiene todos los métodos heredados + nuevos:
  - `IndexarConEmbeddings(f)`: indexa en BM25 + genera embedding
  - `IndexarBatchConEmbeddings(frags)`: batch eficiente (1 llamada API)
  - `BuscarVector(query, topK)`: similitud coseno contra todos los embeddings
  - `BuscarHibridoConEmbeddings(query, topK)`: BM25 + vector + RRF
- **Degradación graceful**: si el provider falla, cae a BM25 puro.
- **`similitudCoseno`**: implementación stdlib (sin importar math).
- **`sqrt`**: método de Newton (10 iteraciones, precisión 0.001).

### Provider NVIDIA (`internal/nucleo/orquestador/provider_embeddings.go`)
- **`ProviderEmbeddingsNVIDIA`**: adapta `ClienteNVIDIA.Embeddings` a la
  interfaz `buscador.EmbeddingsProvider`.
- Soporta múltiples modelos: `nv-embed-v1` (1024 dim), `nv-embedqa-e5-v5`
  (1024), `nv-embedqa-mistral7b-v2` (4096), `snowflake/arctic-embed-l-v2.0`
  (1024).
- **`Orquestador.Cliente()`**: expone el cliente NVIDIA para que otros
  paquetes puedan construir providers.
- **Compile-time check**: `var _ buscador.EmbeddingsProvider =
  (*ProviderEmbeddingsNVIDIA)(nil)` garantiza que la interfaz se cumple.

### Tests
- **369 tests en total** (todos pasando)
- **+14 tests buscador embeddings**: indexación, batch, búsqueda vectorial,
  híbrida, degradación graceful, similitud coseno, sqrt
- **+7 tests provider NVIDIA**: generación exitosa, errores, dimensiones
  por modelo, integración end-to-end con buscador

### Arquitectura final de búsqueda híbrida

```
Usuario: "función de autenticación"
            ↓
    ┌───────┴───────┐
    ↓               ↓
  BM25           Vector
  (keyword)    (semántico)
    ↓               ↓
  rank 1         rank 1
  rank 2         rank 2
  rank 3         rank 3
    ↓               ↓
    └───────┬───────┘
            ↓
     Reciprocal Rank Fusion
     (k=60)
            ↓
     Ranking unificado
```

---

## [0.4.0] — Fase 4: Orquestador Multi-Modelo NVIDIA + Memoria Conversacional

> **Liz ahora puede conversar y recordar. Combina memoria de código
> (Fase 3.5) con memoria conversacional (Fase 3.5+) y orquestación
> multi-modelo NVIDIA (Fase 4).**

### Análisis de brechas (commit f44fef7)
- `docs/ANALISIS_MEMORIA.md`: comparativa detallada del sistema de memoria
  y contexto actual vs. Mem0, Letta, Zep, LangGraph, Claude Code, Cursor,
  Aider, Copilot, Continue.dev y Sourcegraph Cody.
- Identificación de 3 brechas críticas: memoria conversacional, embeddings
  vectoriales reales, stubs del empaquetador.
- Roadmap de 6 iteraciones para cerrar las brechas.

### Mejoras del empaquetador (commit 2089bff)
- **Capa 3 (Imports expandidos)**: BFS real sobre el grafo de dependencias.
  Usa `Grafo.Vecinos` + callback `ObtenerFragmentosPorRuta` para traer el
  código completo de los archivos importados. Profundidad configurable
  (1 = directos, 2 = transitivos, 0 = deshabilitado).
- **Capa 4 (Archivos recientes)**: usa `ObtenerFragmentosPorRuta` para
  incluir el código completo de los archivos recientemente editados.
  Deduplica fragmentos ya incluidos en capas anteriores.
- Antes: las capas 3 y 4 eran stubs (`TokensImports` y `TokensRecientes`
  siempre eran 0). Ahora entregan valor real.

### Sistema de Memoria Conversacional (commit e710bb7)
Nuevo paquete `internal/nucleo/memoria/` inspirado en Mem0 + Letta + Zep:

- **Sesiones**: conversaciones con uuid, usuario, timestamps, persistencia
  atómica en `~/.liz/memoria/sesiones/<uuid>.json`. Cache en memoria +
  reload automático de sesiones activas al iniciar. Cierre automático de
  sesión previa al crear nueva (estilo Zep). Título autogenerado del
  primer mensaje del usuario.
- **Mensajes**: turnos de chat con metadata y estimación de tokens.
- **Hechos**: tripletas (sujeto, predicado, objeto) con confianza.
  RESOLUCIÓN DE CONFLICTOS estilo Mem0: si un hecho nuevo tiene mismo
  (sujeto, predicado) que uno existente con objeto diferente, el viejo
  se marca `Obsoleto=true` y el nuevo lo reemplaza. Hechos duplicados
  (mismo objeto): se promedia la confianza.
- **Gestor unificado**: `ContextoParaLLM(usuarioID, n_mensajes, n_hechos)`
  ensambla memoria semántica + memoria episódica en un solo string.

### Orquestador NVIDIA (commit a36ed0e)
Nuevo paquete `internal/nucleo/orquestador/` (Fase 4 del roadmap):

- **ClienteNVIDIA**: HTTP client stdlib (sin dependencias externas),
  compatible con API OpenAI. Endpoints: `/chat/completions` (JSON y SSE),
  `/embeddings` (stub para Fase 4.1 con `nv-embed-v1`).
- **Selector**: `SeleccionarModelo(tarea, modeloEspecifico)` elige el
  mejor modelo por tarea (heurística ES→EN: codigo→code, razonamiento→
  nemotron, etc.) + métricas históricas (tasa de éxito desc, latencia asc).
- **Completar con fallback**: prueba cadena principal→fallback. Error
  reinterrable (429, 5xx) → siguiente modelo. Error no reinterrable
  (401, 403, 404) → fail fast.
- **CompletarStream**: SSE con fallback limitado (max 3 intentos, solo
  si falla antes del primer chunk).
- **Métricas**: exitos, fallos, tasa_exito, latencia_promedio,
  tokens_consumidos, ultimo_uso por modelo.

### Integración servidor + main (commit iter5)
- **Servidor**: 13 endpoints nuevos:
  - 4 del orquestador: estado, modelos, métricas, completar (JSON o SSE)
  - 9 de memoria: sesiones (listar, crear, obtener, cerrar, agregar mensaje),
    hechos (listar, crear, eliminar), contexto unificado
- **Builders**: `ConMemoria(gestor)`, `ConOrquestador(orch)` para DI.
- **Helpers**: `requiereMemoria(w)`, `requiereOrquestador(w)` retornan 503
  si la dependencia no está inyectada (orquestador es opcional — requiere
  API key NVIDIA válida).
- **main.go**: wiring completo. Orquestador opcional: si no hay API key
  configurada, queda deshabilitado y los endpoints responden 503.
  Versión 0.4.0.

### Config
- `NuevoGestorConConfig(cfg)`: constructor público para tests e inyección
  de dependencias (no requiere archivo YAML ni directorios).

### Tests
- **342 tests en total** (todos pasando)
- **+27 tests memoria**: sesiones, mensajes, hechos, conflictos, persistencia
- **+16 tests orquestador**: cliente HTTP, selección, fallback, métricas, streaming, embeddings
- **+11 tests empaquetador**: capas 3 y 4, integración completa
- **+2 tests servidor**: 503 cuando orquestador/memoria no inyectados
- **Fix flaky test**: `TestRRF_FusionDosRankings` era flaky (3 fragmentos
  con contenido idéntico → orden BM25 randomizado por iteración de mapa).
  Cambiado a contenidos distintos + aserción top-2 más robusta.

### Endpoints nuevos

```
# Orquestador (Fase 4)
GET  /api/v1/orquestador                  # estado
GET  /api/v1/orquestador/modelos          # listar modelos (sanitizado)
GET  /api/v1/orquestador/metricas         # métricas de uso
POST /api/v1/orquestador/completar        # chat completion (JSON o SSE)

# Memoria conversacional (Fase 3.5+)
GET    /api/v1/memoria/sesiones           # ?usuario_id=X&solo_activas=true
POST   /api/v1/memoria/sesiones           # body: {usuario_id, proyecto}
GET    /api/v1/memoria/sesiones/{id}      # obtener sesión por uuid
POST   /api/v1/memoria/sesiones/{id}/cerrar  # ?usuario_id=X
POST   /api/v1/memoria/sesiones/{id}/mensajes  # body: {usuario_id, rol, contenido}
GET    /api/v1/memoria/hechos             # ?usuario_id=X
POST   /api/v1/memoria/hechos             # body: {usuario_id, sujeto, predicado, objeto, ...}
DELETE /api/v1/memoria/hechos/{id}        # ?usuario_id=X
GET    /api/v1/memoria/contexto           # ?usuario_id=X&mensajes=10&hechos=20
```

---

## [0.3.5] — Fase 3.5: Sistema de Memoria World-Class

> **Combina lo mejor de Claude Code, Cursor, Aider, GitHub Copilot, Continue.dev
> y Sourcegraph Cody en una sola arquitectura.**

### Arquitectura de 7 capas

1. **Fragmentos inmutables** (existente, mejorado)
2. **Symbol Table con AST real** (NUEVO — `arbol_ast`)
3. **Code Graph con PageRank** (NUEVO — `grafo`)
4. **Hybrid Search BM25 + RRF** (NUEVO — `buscador`)
5. **LLM Summaries (deferred, cached)** (parcial — pendiente Fase 4)
6. **Repository Map compacto** (NUEVO — `mapa_repo`)
7. **Context Packer token-aware** (NUEVO — `empaquetador`)

### Nuevo paquete `arbol_ast` — AST parsing real
- Usa `go/parser` + `go/ast` de la stdlib (sin CGO, sin dependencias)
- Extrae símbolos ricos: nombre, firma, docstring, receiver, parámetros, retornos
- Tipos: funcion, metodo, estructura, interface, tipo, constante, variable, import
- Distinción exported/unexported
- Manejo graceful de sintaxis inválida
- Soporte para funciones multi-línea con receiver
- **Reemplaza los fragmentadores regex** que eran frágiles ante strings y comentarios

### Nuevo paquete `grafo` — Code graph + PageRank
- Grafo dirigido de dependencias entre archivos
- PageRank iterativo (50 iteraciones, damping 0.85)
- Normalización a [0.0, 1.0]
- Manejo de dangling nodes (sin salidas)
- `TopN(n)`: archivos más importantes en orden descendente
- Backlinks y Vecinos para traversal
- Estadísticas: densidad, promedio imports, archivo top
- Helpers Go: `ResolverImportGo`, `MatchImportArchivo`, `NormalizarRutaGo`
- Thread-safe con `sync.RWMutex`

### Nuevo paquete `buscador` — Hybrid search BM25 + RRF
- BM25 implementado desde cero (sin dependencias)
  - Inverted index: término → map[fragment_id → tf]
  - IDF con suavizado: `ln(1 + (N - nt + 0.5) / (nt + 0.5))`
  - Score: `IDF * (tf * (k1+1)) / (tf + k1*(1-b+b*dl/avgdl))`
  - Parámetros estándar: k1=1.5, b=0.75
- Reciprocal Rank Fusion (RRF) para combinar rankings
  - `score(d) = sum(1 / (k + rank_i(d)))` con k=60
  - Preparado para fusionar con vector search cuando Fase 4 esté lista
- Tokenización robusta: lowercase, split por no-alfanumérico, separación de snake_case, 100+ stopwords en inglés/español/lenguajes
- Stub `BuscarHibridoConVectores` listo para Fase 4 (embeddings NVIDIA)

### Nuevo paquete `mapa_repo` — Repository Map (Aider-style)
- Vista compacta: solo firmas de símbolos (no código completo)
- Ordenado por importancia PageRank descendente
- Limitado por presupuesto de tokens (4 chars ≈ 1 token)
- `FormatoTexto()` para prompts de LLM
- `FormatoMarkdown()` para UI/web con iconos por tipo
- Marca `Truncado=true` si no caben todos los archivos
- `ArchivosDesdeGrafo` helper de integración

### Nuevo paquete `empaquetador` — Context Packer
- Ensambla contexto óptimo para LLM en 4 capas:
  1. Repository Map compacto (~30% del presupuesto)
  2. Top-K fragmentos por hybrid search (~50% del presupuesto)
  3. Imports directos expandidos (~15% del presupuesto)
  4. Archivos recientes (locality bias, ~5% del presupuesto)
- `SolicitudEmpaquetado`: query, presupuesto, archivos recientes, profundidad
- `ContextoEmpaquetado`: contenido + metadata detallada
- `truncarATokens`: corta en límite de palabras (no a mitad)
- Defaults: presupuesto=8000 tokens, profundidad imports=1

### Coordinador (contexto.go)
- `ProyectoContexto` extendido con `Parser`, `Grafo`, `Buscador`, `GenMapaRepo`, `Empaquetador`, `Modulo`
- `IndexarProyecto` ahora construye grafo + indexa buscador automáticamente
- `construirGrafo`: parsea imports Go, resuelve internos vs externos, calcula PageRank
- `indexarBuscador`: indexa todos los fragmentos en BM25 con contenido completo
- `detectarModuloGo`: lee `go.mod` para resolver imports internos
- Nuevos métodos:
  - `ObtenerSimbolos(nombre, ruta)` — AST parsing de un archivo Go
  - `ObtenerGrafo(nombre)` — retorna grafo de dependencias
  - `ObtenerImportancias(nombre)` — mapa ruta → score PageRank
  - `BuscarHibrido(nombre, query, topK)` — búsqueda BM25 + RRF
  - `ObtenerMapaRepo(nombre, presupuestoTokens)` — repo map compacto
  - `EmpaquetarContexto(solicitud)` — ensambla contexto óptimo

### Servidor — 6 endpoints nuevos
- `GET /api/v1/contexto/proyectos/{nombre}/simbolos?ruta=X` — AST parsing
- `GET /api/v1/contexto/proyectos/{nombre}/grafo` — grafo + estadísticas
- `GET /api/v1/contexto/proyectos/{nombre}/importancia` — PageRank scores
- `POST /api/v1/contexto/proyectos/{nombre}/buscar-hibrido` — body: `{query, top_k}`
- `GET /api/v1/contexto/proyectos/{nombre}/mapa-repo?max_tokens=X&formato=texto` — repo map
- `POST /api/v1/contexto/proyectos/{nombre}/empaquetar` — ensamblado de contexto

### Comparativa: Fase 3 vs Fase 3.5

| Aspecto | Fase 3 (anterior) | Fase 3.5 (world-class) |
|---|---|---|
| Fragmentación | Regex (frágil) | go/ast real para Go, regex mejorado otros |
| Búsqueda | Substring case-insensitive | BM25 + RRF (vector-ready) |
| Importancia | Alfabética | PageRank sobre grafo de imports |
| Mapa del repo | Mapa completo (caro) | Compact signature-only (Aider-style) |
| Context packing | No existe | Token-aware layered packing |
| Granularidad | Solo fragmento | 4 niveles: 1-línea → párrafo → fragmento → archivo |
| Símbolos | No | AST con firma, docstring, receiver, parámetros |
| Grafo de código | No | PageRank + backlinks + vecinos |

### Tests
- **283 tests en total** (todos pasando)
- **78 tests nuevos** en Fase 3.5:
  - `arbol_ast`: 11 tests (funciones, métodos, structs, interfaces, imports, multi-línea, errores)
  - `grafo`: 12 tests (PageRank, TopN, vecinos, backlinks, estadísticas)
  - `buscador`: 17 tests (BM25, tokenización, RRF, stopwords, camelCase)
  - `mapa_repo`: 12 tests (generación, ordenamiento, truncado, formatos)
  - `empaquetador`: 9 tests (capas, presupuesto, defaults, truncado)
  - `contexto` integración: 17 tests (end-to-end de grafo, buscador, mapa repo, empaquetador)

---

## [0.3.0] — Fase 3: Sistema de Contexto

### Coordinador de Contexto (`internal/nucleo/contexto/`)
- **Coordinador central**: `NuevoCoordinador`, `IndexarProyecto`, `EliminarProyecto`, `ReindexarArchivo`
- **Consultas**: `ObtenerMapa`, `ObtenerIndice`, `ObtenerArbol`, `ObtenerFragmento`, `ObtenerFragmentosPorRuta`, `BuscarEnIndice`, `ObtenerResumen`, `ForzarResumen`
- **Persistencia de estado**: cada proyecto tiene `estado.json` que sobrevive reinicios
- **Carga automática**: al iniciar, los proyectos indexados previamente se cargan desde caché

### Mapa (`internal/nucleo/contexto/mapa/`)
- Genera el "catálogo de la biblioteca" — mapa de archivos con resúmenes cortos
- Exclusiones por defecto: `.git/`, `node_modules/`, `vendor/`, `__pycache__/`, `dist/`, `build/`, etc.
- Detecta lenguajes: Go, Python, JS/TS, Rust, Java, C/C++, HTML, CSS, YAML, JSON, Markdown, etc.
- Persistible como `mapa.json`

### Fragmentos (`internal/nucleo/contexto/fragmentos/`)
- **Fragmentadores inteligentes** para 6 lenguajes:
  - **Go**: funciones, tipos, imports (rastreo de profundidad de llaves)
  - **Python**: clases, funciones (por indentación)
  - **JS/TS**: functions, classes, exports, interfaces, types, arrow functions
  - **Rust**: fn, struct, enum, trait, impl (con `pub`)
  - **Java**: class, interface, enum, methods (con modifiers)
  - **C/C++**: struct, union, enum, funciones (por paréntesis + bloque)
- `contarLlaves` ignora strings y comentarios para precisión
- **Índice en memoria** (ruta → []id): consultas O(1) en vez de O(n)
- Persistencia entre sesiones (índice se reconstruye desde disco al iniciar)
- Lenguajes detectados: 26+ extensiones (incluyendo Ruby, PHP, Kotlin, Swift, Scala, Lua, R, SQL)

### Índice (`internal/nucleo/contexto/indice/`)
- **Estructura de árbol** (`NodoArbol`): vista jerárquica de directorios con totales por nodo
- **Optimización mtime/size**: `ArchivosModificados` primero hace `stat` (O(1)) y solo hashea si cambió
- `EntradaIndice` ahora persiste `Tamano` y `Mtime` para detección rápida
- `detectarLenguajeIndice` coherente con `fragmentos.detectarLenguajeExt` (bug #12)
- Reconstrucción incremental: preserva fragmento_ids de archivos no modificados

### Resumen (`internal/nucleo/contexto/resumen/`)
- **Persistencia a disco**: `.liz/resumenes/<ruta>.json` (uno por archivo)
- **Cache en memoria**: `Cargar`, `Guardar`, `Eliminar`
- `ConDirResumen()` builder method
- `TipoArchivo` mejorado: detecta `test_*`, `.test.js`, `.spec.ts`, Makefile, Dockerfile, etc.
- Resúmenes solo se regeneran cuando el archivo cambia

### Servidor (`internal/nucleo/servidor/`)
- **11 endpoints nuevos** de contexto bajo `/api/v1/contexto/*`:
  - `GET /api/v1/contexto/proyectos` — listar proyectos
  - `POST /api/v1/contexto/proyectos` — indexar nuevo
  - `DELETE /api/v1/contexto/proyectos/{nombre}` — eliminar
  - `GET /api/v1/contexto/proyectos/{nombre}/mapa` — catálogo de biblioteca
  - `GET /api/v1/contexto/proyectos/{nombre}/indice` — índice plano
  - `GET /api/v1/contexto/proyectos/{nombre}/arbol` — árbol jerárquico
  - `GET /api/v1/contexto/proyectos/{nombre}/fragmentos?ruta=X` — por ruta
  - `GET /api/v1/contexto/proyectos/{nombre}/fragmentos/{id}` — por ID
  - `GET /api/v1/contexto/proyectos/{nombre}/buscar?patron=X` — búsqueda
  - `GET /api/v1/contexto/proyectos/{nombre}/resumen?ruta=X` — resumen cacheado
  - `POST /api/v1/contexto/proyectos/{nombre}/reindexar` — refresh total o parcial
- `ConCoordinador()` builder method para DI
- `requiereCoordinador()` helper: responde 503 si no hay coordinador
- `parsearBody()` tolerante a body vacío
- Servidor ahora trackea `inicio time.Time` para uptime en `/health`

### Entry Point (`cmd/liz/main.go`)
- Wiring completo: logger → config → permisos → contexto → servidor
- Flags: `--version`, `--config <ruta>`
- Concede todos los permisos al iniciar (D-006)
- Carga proyectos indexados previamente desde caché
- Maneja SIGHUP para recarga de configuración en caliente
- Versión 0.3.0

### Bugs Críticos Arreglados
1. **#1 `IndexarProyecto`**: orden `Reconstruir`/`ArchivosModificados` estaba invertido, haciendo que la reindexación incremental nunca detectara archivos modificados.
2. **#2 `fragmentarGo`**: el check `!strings.HasSuffix(trim, "{")` impedía que cualquier función `func name() {` se fragmentara — todo Go caía al fallback "archivo completo".
3. **#5 Coherencia archivos ocultos**: el coordinador excluiría `.gitignore` mientras el mapa lo incluía; ahora ambos respetan las mismas reglas.
4. **#6 Dead code**: `limpiarComentario` tenía un `""` en la lista de prefijos que hacía el `return` final inalcanzable.
5. **#10 Test Python inválido**: usaba sintaxis Scala en lugar de Python.
6. **#12 `detectarLenguaje` inconsistente**: `indice` y `fragmentos` devolvían lenguajes distintos para la misma extensión. Unificado a 26+ extensiones.
7. **#8 `ArchivosModificados` O(n)**: leía todos los archivos para comparar hashes. Ahora usa `stat` primero (mtime/size) y solo hashea si esos cambiaron.
8. **#9 `ObtenerPorRuta`/`Listar` O(n)**: leían todos los archivos `.json`. Ahora usan índice en memoria.
9. **`ValidarCampo` nil-interface bug**: retornaba `(*ErrorValidacion)(nil)` envuelto en `error`, que no era `nil` para el llamador.

### Tests
- **205 tests** en total (todos pasando)
- **16 tests nuevos** del coordinador (`contexto_test.go`):
  - Indexación básica, persistencia, reindexación incremental
  - Obtener mapa/índice/árbol/fragmento/resumen
  - Búsqueda, eliminación, reindexación selectiva
  - Operaciones en proyecto inexistente, múltiples proyectos
- **15 tests nuevos** de endpoints de contexto (`servidor_test.go`)
- **14 tests nuevos** de fragmentadores multi-lenguaje (`fragmentos_test.go`): Go, Python, JS, TS, Rust, Java, C
- **4 tests nuevos** de estructura de árbol y optimización mtime (`indice_test.go`)

---

## [0.2.0] — Fase 2: Permisos y Configuración

### Configuración (`internal/nucleo/config/`)
- **Tipos avanzados**: `ConfiguracionModelo`, `ConfiguracionHerramienta`, `ConfiguracionSeguridad`, `ConfiguracionLogging`, `ConfiguracionContexto`
- **Gestor thread-safe**: Singleton con `sync.RWMutex` para acceso concurrente
- **Validación**: 13+ reglas predefinidas (puerto, rangos, uno_de, regex, URL, path)
- **Modificación dinámica**: `Establecer()` por dot-notation con reflexión, `EstablecerMultiple()` atómico
- **Hot-reload**: `Recargar()` lee config desde disco, detecta diferencias (preparado para SIGHUP)
- **Auditoría de cambios**: `CambioConfiguracion` con valor anterior/nuevo y timestamp
- **Esquema auto-generado**: `EsquemaConfig` para documentación de API
- **Env vars extendidas**: `LIZ_PUERTO`, `LIZ_HOST`, `LIZ_NOMBRE`, `LIZ_VERSION`, `LIZ_NIVEL_LOG`, `LIZ_DIRECTORIO_BASE`, `NVIDIA_API_KEY`, `OPENAI_API_KEY`

### Permisos (`internal/nucleo/permisos/`)
- **6 categorías**: archivos, red, sistema, terminal, herramientas, modelos
- **Sub-permisos granulares**: leer/escribir/eliminar/ejecutar, http/dns/sockets, etc.
- **Niveles**: total, lectura, escritura, restringido, denegado
- **Gestor thread-safe**: Singleton con `sync.RWMutex`
- **Auditoría completa**: cada verificación, concesión y revocación registrada
- **Persistencia**: JSON en `~/.liz/permisos/permisos.json`, sobrevive reinicios
- **D-006**: "Permisos Una Vez" — todos concedidos al inicio
- **API formatting**: `FormatearPermisosParaAPI()` para respuestas JSON

### Servidor (`internal/nucleo/servidor/`)
- **GET /api/v1/config**: configuración completa (API keys sanitizadas)
- **PUT /api/v1/config**: modificar campos con validación previa y cambios atómicos
- **GET /api/v1/config/esquema**: esquema de validación
- **GET /api/v1/config/cambios**: historial de cambios
- **POST /api/v1/config/recargar**: hot-reload desde disco
- **GET /api/v1/permisos**: estado completo con sub-permisos
- **POST /api/v1/permisos**: conceder permisos individuales con validación
- **GET /api/v1/permisos/resumen**: resumen estadístico
- **GET /api/v1/permisos/auditoria**: historial de auditoría
- **/health** mejorado: uptime, estado de permisos, info de configuración
- **SIGHUP**: recarga configuración sin apagar el servidor
- **responseCapture**: middleware de logging captura status code real

### Entry Point (`cmd/liz/main.go`)
- Inyección de dependencias completa (gestorCfg + gestorPer → servidor)
- Logging detallado del proceso de inicio
- Manejador de SIGHUP para recarga en caliente
- Versión 0.2.0

### Tests
- **70+ tests** en 6 archivos de test
- `config_test.go`: 30 tests (carga, overrides, gestor, utilidades)
- `validador_test.go`: 20 tests (validación, campos, esquema, utilidades)
- `permisos_test.go`: 25+ tests (inicialización, concesión, verificación, auditoría, persistencia)
- `servidor_test.go`: 20+ tests (todos los endpoints, CORS, stubs, errores)
- Todos pasando

---

## [0.1.0] — Fase 1: Fundación

### Estructura del Proyecto
- Módulo Go: `github.com/caos1codex-hash/liz-ai-agent`
- Go 1.21 con dependencias: `gorilla/mux` v1.8.1, `yaml.v3`
- Estructura: `cmd/liz/main.go` + `internal/nucleo/` (logger, config, permisos, servidor)

### Logger (`internal/nucleo/logger/`)
- Logging estructurado en JSON a `~/.liz/logs/liz.log`
- Salida coloreada a stdout
- 5 niveles: DEBUG, INFO, ADVERTENCIA, ERROR, SILENCIO
- Thread-safe con `sync.Mutex`

### Configuración (`internal/nucleo/config/`)
- Carga desde YAML con wrapper `liz:`
- Variables de entorno: `LIZ_PUERTO`, `NVIDIA_API_KEY`, `LIZ_HOST`
- Expansión de `~` a home directory
- Creación automática de directorios de runtime

### Permisos (`internal/nucleo/permisos/`)
- 6 tipos de permisos: archivos, red, sistema, terminal, herramientas, modelos
- `ConcederTodos()` / `Conceder()` / `Verificar()` / `Resetear()`
- Persistencia JSON en `~/.liz/permisos.json`

### Servidor (`internal/nucleo/servidor/`)
- 14 rutas API con gorilla/mux
- 3 middlewares: logging, CORS, recuperación de panic
- Stubs para fases futuras (501 Not Implemented)

### Build & CI
- Makefile con targets: build, run, dev, test, vet, fmt, lint, clean, install
- 19 tests iniciales todos pasando