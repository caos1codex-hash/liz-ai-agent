# Changelog — Liz AI Agent

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