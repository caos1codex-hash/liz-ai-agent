# Changelog — Liz AI Agent

## [0.8.0] — Fase 8: Frontend

> **Liz ahora tiene cara. El frontend React+TypeScript trae el pipeline de chat
> a una interfaz ChatGPT-clásica con streaming SSE, sidebar de conversaciones,
> header con métricas en vivo y soporte responsive completo. Ya no hace falta
> curl para hablar con Liz — abres el navegador y listo.**

### Frontend (`web/`)

Nuevo directorio con aplicación Vite + React 18 + TypeScript + Tailwind CSS.

**Stack:**
- React 18 + TypeScript estricto
- Vite 5 (dev server + proxy a backend)
- Tailwind CSS 3 con plugin Typography
- react-markdown + remark-gfm (GFM)
- react-syntax-highlighter (lazy-loaded)
- clsx para classNames condicionales

**Estructura:**
```
web/
├── public/liz.svg                  # Logo / favicon
├── src/
│   ├── components/                  # 14 componentes reutilizables
│   │   ├── AppShell.tsx             # Layout responsive (sidebar drawer en móvil)
│   │   ├── Header.tsx               # Cabecera con status/modelo/métricas/theme toggle
│   │   ├── Sidebar.tsx              # Lista conversaciones + CRUD
│   │   ├── SidebarHeader.tsx        # Logo + botón nueva conversación
│   │   ├── SidebarItem.tsx          # Item conversación (hover delete, keyboard nav)
│   │   ├── ChatWindow.tsx           # Orquesta MessageList + MessageInput + useChat
│   │   ├── MessageList.tsx          # Lista mensajes con auto-scroll y welcome state
│   │   ├── Message.tsx              # Mensaje usuario/asistente con badges
│   │   ├── MessageInput.tsx         # Textarea auto-expandible, Enter=enviar
│   │   ├── MessageBadge.tsx         # Pills (modelo/herramienta/categoría/métrica)
│   │   ├── Markdown.tsx             # react-markdown + remark-gfm + CodeBlock lazy
│   │   ├── CodeBlock.tsx            # Syntax highlighter + botón copiar
│   │   ├── TypingIndicator.tsx      # 3 puntos animados "Liz está pensando…"
│   │   ├── Avatar.tsx               # Avatares usuario/asistente
│   │   ├── StatusDot.tsx            # Punto de color (online/offline/checking)
│   │   ├── ProjectSelector.tsx      # Dropdown proyectos indexados
│   │   ├── MetricsPanel.tsx         # Panel métricas pipeline (desplegable)
│   │   └── Toast.tsx                # Notificaciones efímeras (error/success/info)
│   ├── hooks/                       # 7 custom hooks
│   │   ├── useChat.ts               # Orquestación mensajes + SSE streaming
│   │   ├── useSesiones.ts           # CRUD sesiones + localStorage sesión activa
│   │   ├── useBackendHealth.ts      # Poll /api/v1/health cada 30s
│   │   ├── usePipelineMetricas.ts   # Poll /api/v1/chat cada 60s
│   │   ├── useModelos.ts            # Carga modelos NVIDIA (única vez)
│   │   ├── useProyectos.ts          # Carga proyectos indexados
│   │   ├── useTheme.ts              # Dark/light + persistencia localStorage
│   │   └── useAutoScroll.ts         # Auto-scroll con detección "usuario abajo"
│   ├── lib/
│   │   ├── api.ts                   # Fetch wrapper con timeout, AbortSignal, errores tipados
│   │   ├── sse.ts                   # Parser SSE para streaming del pipeline
│   │   ├── endpoints.ts             # chatApi, orquestadorApi, herramientasApi, contextoApi, healthApi
│   │   └── utils.ts                 # cn(), formatDuration(), formatRelative(), truncate(), shortId()
│   ├── types/api.ts                 # Espejo de structs Go (SolicitudChat, RespuestaPipeline, ChunkStream, etc.)
│   ├── styles/globals.css           # Tailwind + estilos base + scrollbar fino
│   ├── App.tsx                      # Root component con ToastProvider
│   └── main.tsx                     # Entry point
├── index.html
├── package.json
├── tailwind.config.js               # Paleta Liz (morado) + tokens semánticos + animaciones
├── tsconfig.json / tsconfig.node.json
├── vite.config.ts                   # Proxy /api → localhost:3000, code-splitting
├── postcss.config.js
└── README.md                        # Documentación específica del frontend
```

### Características de UX

- **Streaming SSE progresivo**: la respuesta del LLM aparece token a token, con cursor parpadeante
- **Mensajes optimistas**: el mensaje del usuario aparece inmediatamente, marcado como "enviando…" hasta confirmación
- **Indicador "Liz está pensando…"**: 3 puntos animados mientras llega el primer chunk
- **Badges informativos**: modelo usado, herramientas invocadas, pasos ejecutados, duración, tokens
- **Welcome state con prompts de ejemplo**: 4 ideas clicables cuando no hay mensajes
- **Auto-scroll inteligente**: sigue el stream solo si el usuario está al fondo; botón "↓ Último mensaje" si hace scroll up
- **Sidebar CRUD completo**: listar, crear, seleccionar, eliminar con confirmación en 2 pasos
- **Sesión activa persistida**: localStorage, sobrevive recargas
- **Sesión anónima → con sesión**: el primer mensaje en una conversación nueva dispara la creación automática de sesión en el backend, y el sidebar se refresca solo
- **Selector de proyecto**: dropdown que lista proyectos indexados, opcional "Sin proyecto"
- **Panel de métricas**: desplegable desde el header, muestra mensajes procesados, duración promedio, modelo más usado, distribución por categoría
- **Theme toggle**: dark por defecto, claro alternativo, persistencia localStorage
- **Toasts**: notificaciones efímeras para errores (con auto-dismiss)
- **Responsive**:
  - **< md (móvil)**: sidebar como drawer overlay, header compacto (solo iconos), métricas ocultas
  - **md a lg**: sidebar fijo, header muestra project selector + modelo badge
  - **≥ lg**: header completo con métricas extendidas

### Integración con backend

- **Proxy Vite** — `/api/*` → `http://localhost:3000`, sin CORS en dev
- **Tipos espejo** — `types/api.ts` refleja los structs Go (SolicitudChat, RespuestaPipeline, ChunkStream, EstadoPipeline, SesionChat, ModeloOrquestador, InfoHerramienta, etc.)
- **Endpoints consumidos**:
  - `/api/v1/health` — status del backend
  - `/api/v1/chat` (POST, SSE) — pipeline streaming
  - `/api/v1/chat` (GET) — estado del pipeline + métricas
  - `/api/v1/chat/sesiones` (GET/POST) — listar/crear sesiones
  - `/api/v1/chat/sesiones/{id}` (GET/DELETE) — detalle/eliminar
  - `/api/v1/orquestador/modelos` — lista de modelos NVIDIA
  - `/api/v1/contexto/proyectos` — lista de proyectos indexados

### Makefile (targets nuevos)

```
make web-install    # npm install
make web-dev        # Vite dev server (:5173)
make web-build      # Build producción → web/dist/
make web-preview    # Servir build localmente
make web-typecheck  # Solo tsc --noEmit
make web-clean      # Limpiar node_modules + dist
make all            # build backend + frontend
```

### Optimizaciones

- **Code-splitting**: `react-vendor` y `markdown-vendor` en chunks separados
- **Lazy-load de CodeBlock**: `react-syntax-highlighter` (~800KB) solo se carga cuando hay code blocks
- **Memoización**: `Markdown` envuelto en `memo()` para evitar re-renders durante streaming
- **Tailwind purge**: solo los estilos usados terminan en el bundle CSS (~8KB gzip)

### Build verificado

- `npm run build` OK (1417 módulos)
- Bundle: 55KB app + 134KB react vendor + 795KB markdown vendor (lazy) = 984KB total / 337KB gzip
- `npx tsc --noEmit` OK (0 errores)
- `make all` OK (backend + frontend)

### Bump de versión

- `cmd/liz/main.go`: 0.6.0 → 0.7.0
- `web/package.json`: 0.7.0

### Próximas fases

| Fase | Issue | Estado |
|------|-------|--------|
| 9 — Testing y Docs | [#17](https://github.com/caos1codex-hash/liz-ai-agent/issues/17) | ⏳ |
| 10 — Release v0.1.0 | [#18](https://github.com/caos1codex-hash/liz-ai-agent/issues/18) | ⏳ |

---

## [0.7.0] — Fase 7: Pipeline de Chat

> **Liz ahora tiene un cerebro. El pipeline conecta todos los subsistemas en un
> flujo coherente end-to-end: mensaje → clasificación → planificación →
> ejecución de herramientas → respuesta con streaming. Es el sistema nervioso
> central que transforma inputs de lenguaje natural en acciones concretas del
> sistema operativo.**

### Pipeline de Chat (`internal/pipeline/`)

Paquete nuevo con 12 archivos que implementan el flujo completo:

| Archivo | Función |
|--------|---------|
| `doc.go` | Documentación del paquete |
| `tipos.go` | Categorías, mensajes, planes, respuestas, chunks SSE |
| `interfaces.go` | Interfaces desacopladas para testing |
| `adaptadores.go` | Adapters para tests (mocks) |
| `helpers.go` | Utilidades JSON |
| `receptor.go` | Validación, sesiones, almacenamiento |
| `clasificador.go` | 10 categorías, heurísticas + LLM |
| `planificador.go` | Descomposición en pasos con herramientas |
| `ejecutor.go` | Ejecución secuencial con dependencias |
| `respondedor.go` | Generación de respuesta + streaming SSE |
| `pipeline.go` | Coordinador principal thread-safe |
| `pipeline_test.go` | 30 tests unitarios |

**Características:**
- **Clasificador híbrido**: heurísticas rápidas (sin LLM) con fallback a LLM para ambigüedades
- **10 categorías**: conversación, código, archivos, procesos, monitorización, instalación, búsqueda, análisis, auto-creación, ejecución
- **Planificador inteligente**: usa LLM para descomponer tareas en pasos, selecciona herramientas, respeta dependencias
- **Ejecutor con auto-creación**: si una herramienta no existe, la crea automáticamente
- **Respondedor con streaming**: SSE progresivo, contexto de memoria, resultados de herramientas
- **Graceful degradation**: funciona sin LLM (modo "sin modelo"), sin catálogo, sin memoria
- **Thread-safe**:-safe para uso concurrente con `sync.RWMutex`
- **Métricas**: mensajes procesados, duración promedio, conteo por categoría, modelo más usado

### Endpoints API (7 endpoints)

```
POST   /api/v1/chat             # Enviar mensaje (JSON o SSE streaming)
GET    /api/v1/chat             # Estado del pipeline
GET    /api/v1/chat/metricas     # Métricas detalladas
GET    /api/v1/chat/sesiones      # Listar sesiones
POST   /api/v1/chat/sesiones      # Crear sesión
GET    /api/v1/chat/sesiones/{id}  # Detalle + mensajes
DELETE /api/v1/chat/sesiones/{id}  # Cerrar sesión
```

### Integración en Servidor (`internal/nucleo/servidor/`)

- `handlers_fase7.go`: 7 handlers HTTP implementados
- `handlers_fase7_test.go`: 8 tests de handlers
- `servidor.go`: campo `pipelineMgr` + `ConPipeline()` builder
- Reemplaza stub `/api/v1/chat` con implementación real
- El endpoint stub de chat ahora responde 503 (no implementado → pipeline no disponible)

### Integración en `cmd/liz/main.go`

- Inicialización del pipeline con las 5 dependencias ya existentes
- 5 adaptadores que conectan implementaciones reales con interfaces del pipeline:
  - `pipelineOrquestadorAdapter` → Orquestador NVIDIA
  - `pipelineCatalogoAdapter` → Catálogo de herramientas
  - `pipelineMemoriaAdapter` → Gestor de memoria
  - `pipelineAutoCreacionAdapter` → Gestor de auto-creación
  - `pipelineContextoAdapter` → Coordinador de contexto
- Builder: `.ConPipeline(pipelineMgr)`

### Tests

- **28 tests nuevos** en pipeline + 8 tests nuevos en handlers = **36 tests nuevos**
- Total: **616 tests pasando** (28 paquetes, 0 fallos)
- `go test ./... -count=1` → OK
- `go build ./...` → OK (0 warnings, 0 errores)

### Paquetes modificados

- `internal/pipeline/` — NUEVO (12 archivos)
- `internal/nucleo/servidor/servidor.go` — campo pipeline, import, rutas
- `internal/nucleo/servidor/handlers_fase7.go` — NUEVO (7 handlers)
- `internal/nucleo/servidor/handlers_fase7_test.go` — NUEVO (8 tests)
- `cmd/liz/main.go` — pipeline init + 5 adaptadores + import

---

## [0.6.0] — Fase 6: Auto-Creación de Herramientas

> **Liz ahora se programa a sí misma. Si necesita una herramienta que no tiene,
> la crea: detecta la necesidad, genera código Go con el LLM, lo compila, lo
> valida, lo persiste y lo registra en el catálogo — todo sin intervención
> humana. Liz nunca dice "no puedo".**

### Sistema de Auto-Creación (`internal/nucleo/herramientas/auto_creacion/`)

Nuevo paquete que implementa el principio D-005 (Auto-Suficiencia) con un
flujo completo **detectar → generar → compilar → cargar → registrar**:

- **`doc.go`**: documentación completa del paquete (visión, arquitectura,
  protocolo subprocess, decisiones de diseño, seguridad)
- **`tipos.go`**: tipos compartidos — `SpecHerramienta`,
  `MetadataHerramienta`, `SolicitudCreacion`, `ResultadoCreacion`,
  `SolicitudSubprocess`, `RespuestaSubprocess`, `ErrAutoCreacion`
- **`plantillas.go`**: prompts LLM para detector y generador, ejemplo del
  protocolo subprocess, helpers `ExtraerFuenteGo`/`ValidarFuenteGo`/
  `InyectarHeader`
- **`detector.go`**: analiza petición + catálogo → identifica herramientas
  faltantes vía LLM; parsing robusto de JSON (bloque markdown o plano);
  normalización y validación de specs
- **`generador.go`**: LLM produce código Go completo; fallback
  `GenerarDesdePlantilla` para tests sin API key
- **`compilador.go`**: ejecuta `go build -o herramienta fuente.go` con
  timeout, captura logs, gestiona path al binario `go`
- **`cargador.go`**: `HerramientaSubproceso` implementa `Herramienta`
  delegando al binario vía JSON stdin/stdout; lazy-info cacheada;
  thread-safe; estadísticas de uso
- **`registro.go`**: persistencia en disco con estructura por-herramienta
  (fuente.go, herramienta, metadata.json, compilacion.log) + índice
  global `registro.json`; thread-safe con `sync.RWMutex`
- **`gestor.go`**: orquesta flujo completo + `CargarTodas` (carga inicial) +
  `Recargar` (recompilar) + `Eliminar` + `Probar` + `Listar` + `Obtener` +
  `LeerFuente` + `LeerLogCompilacion`

### Protocolo Subprocess (decisión clave)

Cada herramienta auto-creada es un **binario Go standalone** (no Go plugin)
que se comunica con Liz por JSON sobre stdin/stdout:

```
REQUEST  (Liz → herramienta):  {"operacion": "info|validar|ejecutar", "parametros": {...}}
RESPONSE (herramienta → Liz):  {"exito": true|false, "datos": <any>, "error": "", "metadata": {}}
```

**Por qué subprocess y no Go plugins:**
- Aislamiento de fallos (un panic no tira a Liz)
- Independencia de versión de Go (cada tool se compila sola)
- Sin problemas de module path / dependencias compartidas
- Costo (fork+exec) aceptable para herramientas que hacen ops de sistema

### Persistencia entre sesiones

Las herramientas auto-creadas se guardan en
`~/.liz/herramientas/auto_creadas/{nombre}/`:

```
{nombre}/
├── fuente.go         # código fuente Go
├── herramienta       # binario compilado
├── metadata.json     # spec + timestamps + estadísticas
└── compilacion.log   # log de la última compilación
```

Al iniciar Liz, `Gestor.CargarTodas()` escanea el registro y carga todas
las tools en el catálogo. Si una falla, se marca en metadata pero no aborta
el arranque.

### Endpoints API (9 nuevos)

```
POST   /api/v1/herramientas/auto-crear                     # Flujo completo
POST   /api/v1/herramientas/detectar                       # Solo detectar (preview)
GET    /api/v1/herramientas/auto-creadas                   # Listar
GET    /api/v1/herramientas/auto-creadas/{nombre}          # Info + estadísticas
DELETE /api/v1/herramientas/auto-creadas/{nombre}          # Eliminar
POST   /api/v1/herramientas/auto-creadas/{nombre}/probar   # Ejecutar
POST   /api/v1/herramientas/auto-creadas/{nombre}/recargar # Recompilar
GET    /api/v1/herramientas/auto-creadas/{nombre}/fuente   # Ver código Go
GET    /api/v1/herramientas/auto-creadas/{nombre}/log      # Ver log compilación
```

### Modos de operación

| Modo | LLM | Cuándo se usa |
|------|-----|---------------|
| **Completo** | ✅ | Detector + Generador usan LLM → herramientas reales |
| **Forzado** | ✅/❌ | `forzar_spec` o `forzar_nombre` → salta detector |
| **Fallback stub** | ❌ | Sin LLM: stub compilable para probar el flujo sin API key |

### Integración con `main.go`

- Inicializa el `Gestor` con el orquestador NVIDIA como LLM (si está disponible)
- Crea el directorio `~/.liz/herramientas/auto_creadas/` si no existe
- Llama `CargarTodas()` para cargar tools existentes al iniciar
- Inyecta el gestor en el servidor via `ConAutoGestor()`
- Bump de versión: `0.5.0` → `0.6.0`

### Tests (32 tests nuevos, 588 totales)

- **18 unitarios** (`auto_creacion_test.go`): tipos, plantillas, detector con
  mock LLM, generador con mock LLM + stub fallback, parsing robusto de JSON
- **14 de integración** (`integracion_test.go`): compilador real con
  `go build`, cargador subprocess end-to-end (compile + info + validar +
  ejecutar), registro persistencia (guardar/obtener/listar/eliminar/
  estadísticas), gestor flujo completo (crear/cargar-todas/eliminar/probar/
  recargar con y sin nuevo fuente)

Todos los tests pasan con `go test ./...` en los 22 paquetes del proyecto.
`go vet ./...` limpio.

### Documentación

- **README.md**: nueva sección "Sistema de Auto-Creación de Herramientas
  (Fase 6)" con diagrama de flujo, arquitectura del paquete, protocolo
  subprocess, endpoints, ejemplos de uso curl, modos de operación,
  persistencia y seguridad
- **docs/ARQUITECTURA.md**: nueva sección "12.ter Fase 6 — Auto-Creación
  de Herramientas (Detalle)" con arquitectura del sistema, protocolo
  subprocess (con justificación vs Go plugins), operaciones del gestor,
  endpoints API, persistencia, modos, seguridad y tests
- Roadmap actualizado: Fase 6 marcada como ✅
- Badges actualizados: fase 6/10, 588 tests pasando

---

## [0.5.0] — Fase 5: Herramientas Base (7 integradas)

> **Liz ahora puede manipular el sistema operativo. 7 herramientas integradas
> dan control total: terminal, navegador de archivos, buscador, editor,
> procesos, monitor de sistema e instalador de paquetes.**

### Sistema de Herramientas (`internal/nucleo/herramientas/`)

Nuevo paquete con la **interfaz estándar Herramienta** (D-002):

- **`Herramienta`** interfaz: `Nombre()`, `Descripcion()`, `Parametros()`,
  `Ejecutar(ctx, params) (Resultado, error)`, `Validar() error`
- **`Parametro`** tipo rico: `Tipo` (string/int/bool/float/array/object),
  `Requerido`, `Default`, `Opciones` (enum), `Min`/`Max` (rango o longitud),
  `Items` (tipo de elementos para arrays)
- **`Resultado`** tipo: `Exito`, `Datos`, `Error`, `Metadata`
- **5 errores tipados**: `ErrParametroRequerido`, `ErrTipoParametro`,
  `ErrValorFueraDeRango`, `ErrOpcionInvalida`, `ErrHerramientaInvalida`
- **Helpers de validación**: `ObtenerString/Int/Bool/Float/ArrayString` con
  coercion automática desde JSON (float64), strings numéricos, etc.
- **`ValidarNombre`**: reglas a-z0-9_ longitud 2-64
- **Compile-time check** documentado: `var _ Herramienta = (*MiHerramienta)(nil)`

### Catálogo y Métricas (`internal/nucleo/herramientas/registro/`)

- **`Catalogo`**: thread-safe (sync.RWMutex), registrar/obtener/eliminar/
  listar/ejecutar
- **Validación al registrar**: `ValidarNombre` + `Validar()` de la herramienta
- **Reemplazo de duplicados** (hot-reload para Fase 6)
- **`Ejecutar`** mide latencia automáticamente + inyecta metadata
  (`duracion_ms`, `herramienta`)
- **`Metricas`** por herramienta: exitos, fallos, tasa_exito, latencia
  min/max/promedio, último_uso, último_error
- **`Resumen`** global: total_herramientas, total_ejecuciones, tasa_exito_global
- **`Snapshot`** serializable para API REST
- **Concurrencia**: tests con 100 goroutines registrando + ejecutando

### 7 Herramientas Integradas (`internal/nucleo/herramientas/integradas/`)

#### 1. `terminal` — Ejecución de comandos shell
- Timeout configurable (1-300s, default 30s)
- Modo shell=true para pipes (`|`), redirecciones (`>`), `&&`
- Captura stdout/stderr (combinados o separados)
- Working directory y variables de entorno extra
- **Detección de comandos peligrosos** (`rm -rf /`, `mkfs`, `dd of=/dev/sd*`,
  `shutdown`, `halt`, `reboot`, fork bomb `:(){:|:&};:`) — requiere flag
  `peligroso_confirma=true` explícito
- Limitación de output (1MB) con truncado signalizado
- Cancellation limpia vía `context.WithTimeout` + `exec.CommandContext`

#### 2. `navegador_archivos` — Navegación de directorios
- 4 operaciones: `listar`, `stat`, `arbol`, `existe`
- Filtros: patrón glob, extensiones, incluir_ocultos
- Profundidad configurable en `arbol` (0-20, default 1)
- Orden: directorios primero, luego alfabético
- Stat completo: tamaño, permisos, timestamps, tipo, symlink dest
- Limitación de resultados con flag `Truncado`

#### 3. `buscador` — Find + grep
- 3 operaciones: `archivos`, `contenido`, `combinado`
- Filtros: patrón, extensiones, mtime (RFC3339/duración como `24h`, `7d`),
  tamaño min/max
- Búsqueda de contenido: literal + regex (Go syntax)
- Case-insensitive configurable
- Contexto (líneas antes/después) hasta 10
- **Búsqueda paralela** para >10 archivos (8 workers)
- Salta archivos binarios por extensión (`.png`, `.zip`, `.pdf`, etc.)
- Salta archivos >10MB

#### 4. `editor` — Manipulación de archivos
- 10 operaciones: `leer`, `escribir`, `agregar`, `insertar`, `reemplazar`,
  `parchear`, `eliminar`, `crear_directorio`, `mover`, `copiar`
- Reemplazo literal + regex (Go syntax)
- Modo `todas` (todas las ocurrencias) o solo primera
- `parchear` falla si no encuentra el patrón (semántica estricta)
- **Backup automático** opcional (`.bak`)
- Creación automática de directorios padres
- Permisos configurables (octal string: `'0644'`, `'755'`, etc.)
- Limitación de líneas en `leer` (truncado signalizado)
- Copia preserva permisos del original
- Copia de directorios recursiva

#### 5. `procesos` — Gestión de procesos
- 4 operaciones: `listar`, `info`, `matar`, `arbol`
- Lee `/proc` en Linux (`comm`, `cmdline`, `stat`, `status`)
- Métricas: PID, PPID, CPU%, RAM%, RSS, Virtual, threads
- Filtros: nombre, usuario, ram_min, cpu_min
- 6 señales soportadas: SIGTERM, SIGKILL, SIGINT, SIGHUP, SIGSTOP, SIGCONT
- Árbol de procesos desde PID raíz
- Fallback a `ps` en no-Linux

#### 6. `monitor` — Métricas de sistema
- 6 operaciones: `completo`, `cpu`, `memoria`, `disco`, `red`, `uptime`
- **CPU**: load avg (1/5/15min), uso %, num cores, frecuencias por core
  (vía `/sys/devices/system/cpu/`)
- **Memoria**: total/libre/disponible/buffers/cached/used + swap (vía
  `/proc/meminfo`)
- **Disco**: statvfs (total/libre/usado + inodos)
- **Red**: bytes/paquetes/errores RX+TX por interfaz, MAC, MTU, operstate
  (vía `/proc/net/dev` + `/sys/class/net/`)
- **Uptime**: segundos + humanizado + btime (vía `/proc/uptime` + `/proc/stat`)
- Cálculo de uso de CPU con dos muestras de `/proc/stat` separadas 100ms

#### 7. `instalador` — Gestión de paquetes
- 7 operaciones: `instalar`, `desinstalar`, `actualizar`, `buscar`, `info`,
  `gestores`, `actualizar_todo`
- **16 gestores soportados**: apt, apt-get, snap, dnf, yum, pacman, zypper,
  apk, brew, pip, pip3, npm, yarn, cargo, gem, composer, go install
- **Autodetección**: heurística por tipo de paquete (`python-*` → pip,
  `github.com/` → go, `@` → npm)
- Modo **dry-run** (`solo_verificar`) retorna comando sin ejecutar
- **Sudo automático** para gestores del sistema (apt, dnf, etc.)
- Versión de cada gestor en `/gestores`
- Construcción de args específica por gestor y operación

### Integración Servidor (`internal/nucleo/servidor/handlers_fase5.go`)

- Campo `catalogo` añadido al struct `Servidor`
- Builder `ConCatalogo(cat)` para inyección de dependencias
- Helper `requiereCatalogo(w)` responde 503 si no inyectado
- 6 endpoints nuevos:
  - `GET /api/v1/herramientas` — listar catálogo
  - `GET /api/v1/herramientas/{nombre}` — info detallada
  - `POST /api/v1/herramientas/ejecutar` — ejecutar herramienta
  - `GET /api/v1/herramientas/metricas` — métricas globales
  - `GET /api/v1/herramientas/metricas/{nombre}` — métricas por herramienta
  - `GET /api/v1/tools` — compatibilidad hacia atrás
- Rutas ordenadas: específicas (`/ejecutar`, `/metricas`) antes que
  `{nombre}` para evitar captura errónea
- `Ejecutar` respeta timeout opcional del body
- Métricas se registran automáticamente via catálogo

### Wiring en `cmd/liz/main.go`

- Import paquetes `herramientas`, `integradas`, `registro`
- Inicializa catálogo con `ConLog`
- Registra las 7 herramientas integradas iterando sobre lista tipada
- Builder `ConCatalogo(catalogo)` en servidor
- Log informativo: "Endpoints de herramientas disponibles en
  /api/v1/herramientas/*"
- Versión **0.5.0** (bump desde 0.4.0)

### Tests

- **556 tests en total** (vs 369 en v0.4.0 = **+187 tests nuevos**)
- **+15 tests interfaz**: validación, coercion, rangos, opciones, errores
- **+22 tests registro**: catálogo + métricas + concurrencia
- **+14 tests terminal**: echo, timeout, peligroso, directorio, env, shell
- **+15 tests navegador_archivos**: listar, stat, arbol, existe, límites
- **+17 tests buscador**: archivos, contenido, regex, combinado, contexto
- **+24 tests editor**: leer, escribir, agregar, insertar, reemplazar,
  parchear, eliminar, mover, copiar, backup, permisos
- **+14 tests procesos**: listar, info, matar, arbol, señales, /proc
- **+9 tests monitor**: completo, cpu, memoria, disco, red, uptime
- **+14 tests instalador**: gestores, dry-run, args por gestor, autodetección
- **+19 tests servidor**: 503 sin catálogo, listar, info, ejecutar, métricas,
  integración completa end-to-end
- Todos pasando con `go test -race ./...`

### Endpoints nuevos

```
# Herramientas (Fase 5)
GET    /api/v1/herramientas                       # listar catálogo
GET    /api/v1/herramientas/{nombre}              # info de herramienta
POST   /api/v1/herramientas/ejecutar              # body: {nombre, parametros, timeout_segundos}
GET    /api/v1/herramientas/metricas              # métricas globales
GET    /api/v1/herramientas/metricas/{nombre}     # métricas por herramienta
GET    /api/v1/tools                              # alias (compatibilidad)
```

### Estructura de archivos nueva

```
internal/nucleo/herramientas/
├── interface.go                   # Herramienta, Parametro, Resultado + helpers
├── interface_test.go              # 15 tests
├── integradas/
│   ├── terminal.go                # 1: terminal
│   ├── terminal_test.go
│   ├── navegador_archivos.go      # 2: navegador_archivos
│   ├── navegador_archivos_test.go
│   ├── buscador.go                # 3: buscador
│   ├── buscador_test.go
│   ├── editor.go                  # 4: editor
│   ├── editor_test.go
│   ├── procesos.go                # 5: procesos
│   ├── procesos_test.go
│   ├── monitor.go                 # 6: monitor (cross-platform)
│   ├── monitor_linux.go           # statvfs para Linux
│   ├── monitor_otros.go           # stub para no-Linux
│   ├── monitor_test.go
│   ├── instalador.go              # 7: instalador (16 gestores)
│   └── instalador_test.go
└── registro/
    ├── catalogo.go                # catálogo thread-safe
    ├── catalogo_test.go           # 22 tests (incluye concurrencia)
    └── metricas.go                # métricas por herramienta
```

---

## [2025-07-25] — Fase 4: Polish

### Fixed
- P0: Decouple empaquetador from concrete Buscador type (BuscadorHibrido interface)
- P1.1: Fix tracker test typo, verify race-free code
- P1.2: Prevent goroutine leak in CompletarStream wrapper (ctx.Done() check)
- P3.3: Improve error handling with context wrapping and errors.As

### Added
- P2.3: HTTP endpoints for file edit tracker (POST/GET /api/v1/contexto/tracker/*)
- P2.4: PageRank cache with SHA-256 graph change detection
- P2.2: LLM re-ranking for search results (RerankerLLM)

### Tested
- P4: 15+ new tests (concurrency with -race, error paths, benchmarks)

---

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