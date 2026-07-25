# Liz — AI Agent para Linux

> Agente de IA autonomo que controla completamente tu Linux mediante lenguaje natural.
> No es un chatbot. No es un asistente de codigo. Es un sistema operativo de IA.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8)
![Phase](https://img.shields.io/badge/fase-7%20de%2010-orange)
![Tests](https://img.shields.io/badge/tests-616%20pasando-brightgreen)

## Que hace Liz?

| Capacidad | Ejemplo |
|-----------|---------|
| **Control total del sistema** | "Cierra todas las apps que consuman mas de 2GB de RAM" |
| **Manipulacion de archivos** | "Busca todos los .log del mes pasado y eliminalos" |
| **Gestion de procesos** | "Mata el proceso en el puerto 8080" |
| **Instalacion de software** | "Instala Docker y configura el daemon" |
| **Escritura de codigo** | "Crea un servidor HTTP en Go con auth JWT" |
| **Si no tiene la herramienta...** | **La programa ella misma** |

## Que hace diferente a Liz?

### 1. Contexto Bajo Demanda
Liz entrega un MAPA del entorno, el modelo pide solo lo que necesita. Cero saturacion.

### 2. Se Auto-Programa
Si necesita una herramienta que no tiene, la escribe, compila y registra. Nunca dice "no puedo".

### 3. Multi-Modelo Inteligente
8+ modelos de NVIDIA. Elige automaticamente el mejor para cada tarea.

### 4. Permisos Una Vez
Permisos completos al iniciar. Nunca vuelve a preguntar.

## Arquitectura

```
FRONTEND (React) ──SSE──> PIPELINE ──> ORQUESTADOR (8+ modelos NVIDIA)
                        │                  │
                        └──> CONTEXTO      └──> HERRAMIENTAS (7 + auto-creadas)
```

**[Arquitectura completa](docs/ARQUITECTURA.md)** | **[Decisiones de diseno](docs/DECISIONES.md)**

## Roadmap

| # | Fase | Issue | Estado |
|---|------|-------|--------|
| 1 | Nucleo Base | [#9](https://github.com/caos1codex-hash/liz-ai-agent/issues/9) | ✅ |
| 2 | Permisos y Config | [#10](https://github.com/caos1codex-hash/liz-ai-agent/issues/10) | ✅ |
| 3 | Sistema de Contexto | [#11](https://github.com/caos1codex-hash/liz-ai-agent/issues/11) | ✅ |
| 4 | Orquestador NVIDIA | [#12](https://github.com/caos1codex-hash/liz-ai-agent/issues/12) | ✅ |
| 5 | Herramientas Base | [#13](https://github.com/caos1codex-hash/liz-ai-agent/issues/13) | ✅ |
| 6 | Auto-Creacion | [#14](https://github.com/caos1codex-hash/liz-ai-agent/issues/14) | ✅ |
| 7 | Pipeline de Chat | [#15](https://github.com/caos1codex-hash/liz-ai-agent/issues/15) | End-to-end: mensaje → modelo → herramientas → respuesta | ✅ |
| 8 | Frontend | [#16](https://github.com/caos1codex-hash/liz-ai-agent/issues/16) | ⏳ |
| 9 | Testing y Docs | [#17](https://github.com/caos1codex-hash/liz-ai-agent/issues/17) | ⏳ |
| 10 | Release v0.1.0 | [#18](https://github.com/caos1codex-hash/liz-ai-agent/issues/18) | ⏳ |

## Sistema de Memoria World-Class (Fase 3.5)

Liz combina lo mejor de **Claude Code, Cursor, Aider, GitHub Copilot,
Continue.dev y Sourcegraph Cody** en una arquitectura de 7 capas:

1. **Fragmentos inmutables** — unidades atómicas de código (SHA-256)
2. **Symbol Table con AST real** — go/ast de la stdlib (sin CGO)
3. **Code Graph con PageRank** — importancia por centralidad de grafo
4. **Hybrid Search BM25 + RRF** — búsqueda keyword + vector-ready
5. **LLM Summaries** — pendiente Fase 4 (NVIDIA)
6. **Repository Map compacto** — firmas solo, token-aware (Aider-style)
7. **Context Packer** — ensamblado óptimo en 4 capas (Claude Code-style)

### Endpoints disponibles (Fase 3 + 3.5)

```
# Fase 3 (básico)
GET    /api/v1/contexto/proyectos                      # listar proyectos
POST   /api/v1/contexto/proyectos                      # indexar nuevo
DELETE /api/v1/contexto/proyectos/{nombre}             # eliminar proyecto
GET    /api/v1/contexto/proyectos/{nombre}/mapa        # mapa (catalogo)
GET    /api/v1/contexto/proyectos/{nombre}/indice      # indice plano
GET    /api/v1/contexto/proyectos/{nombre}/arbol       # arbol jerarquico
GET    /api/v1/contexto/proyectos/{nombre}/fragmentos?ruta=X
GET    /api/v1/contexto/proyectos/{nombre}/fragmentos/{id}
GET    /api/v1/contexto/proyectos/{nombre}/buscar?patron=X
GET    /api/v1/contexto/proyectos/{nombre}/resumen?ruta=X
POST   /api/v1/contexto/proyectos/{nombre}/reindexar   # refresh

# Fase 3.5 (world-class)
GET    /api/v1/contexto/proyectos/{nombre}/simbolos?ruta=X  # AST parsing
GET    /api/v1/contexto/proyectos/{nombre}/grafo            # code graph + PageRank
GET    /api/v1/contexto/proyectos/{nombre}/importancia      # scores
POST   /api/v1/contexto/proyectos/{nombre}/buscar-hibrido   # BM25 + RRF
GET    /api/v1/contexto/proyectos/{nombre}/mapa-repo        # Aider-style
POST   /api/v1/contexto/proyectos/{nombre}/empaquetar       # context packing
```

### Fragmentadores inteligentes

Soportados: Go (con AST real), Python, JavaScript/TypeScript, Rust, Java, C/C++.
Go usa `go/parser` para extraer funciones, métodos, structs, interfaces, tipos,
constantes y variables con firma completa y docstrings.

## Sistema de Memoria Conversacional (Fase 3.5+)

Memoria world-class inspirada en Mem0 + Letta + Zep:

- **Sesiones**: conversaciones con uuid, usuario, timestamps, persistencia atomica
- **Mensajes**: turnos de chat con metadata y estimacion de tokens
- **Hechos**: tripletas (sujeto, predicado, objeto) con resolucion de conflictos
  (estilo Mem0 — hecho nuevo reemplaza viejo si mismo sujeto+predicado)
- **Recall memory**: ultimos N mensajes como buffer circular (estilo Letta)
- **Contexto unificado**: `ContextoParaLLM()` ensambla memoria semantica + episodica

```
GET    /api/v1/memoria/sesiones?usuario_id=X          # listar sesiones
POST   /api/v1/memoria/sesiones                       # nueva sesion
GET    /api/v1/memoria/sesiones/{id}                  # obtener sesion
POST   /api/v1/memoria/sesiones/{id}/cerrar           # cerrar sesion
POST   /api/v1/memoria/sesiones/{id}/mensajes         # agregar mensaje
GET    /api/v1/memoria/hechos?usuario_id=X            # listar hechos
POST   /api/v1/memoria/hechos                         # crear hecho
DELETE /api/v1/memoria/hechos/{id}?usuario_id=X       # eliminar hecho
GET    /api/v1/memoria/contexto?usuario_id=X          # contexto para LLM
```

## Orquestador Multi-Modelo NVIDIA (Fase 4)

Conecta con la API de NVIDIA (compatible OpenAI) y orquesta 8+ modelos
con seleccion inteligente + fallback automatico + metricas:

- **Seleccion**: por tipo de tarea (codigo, razonamiento, general, etc.)
- **Fallback**: si un modelo falla con error reinterrable (429/5xx),
  intenta el siguiente en la cadena
- **Metricas**: exitos, fallos, tasa de exito, latencia promedio por modelo
- **Streaming SSE**: respuesta progresiva via Server-Sent Events
- **Embeddings**: integración real con `nv-embed-v1` (1024 dim) vía `ProviderEmbeddingsNVIDIA` — búsqueda híbrida BM25 + vector + RRF

```
GET  /api/v1/orquestador                  # estado
GET  /api/v1/orquestador/modelos          # listar modelos (sin API keys)
GET  /api/v1/orquestador/metricas         # metricas de uso
POST /api/v1/orquestador/completar        # chat completion (JSON o SSE)
```

## Sistema de Herramientas (Fase 5)

7 herramientas integradas que dan a Liz control total sobre el sistema.
Cada herramienta implementa la interfaz estándar `Herramienta` (D-002)
y se registra en un catálogo thread-safe con métricas automáticas.

| Herramienta | Operaciones | Descripción |
|-------------|-------------|-------------|
| **terminal** | ejecutar | Comandos shell con timeout, captura stdout/stderr, detección de peligrosos (`rm -rf /`, `mkfs`, `shutdown`, fork bomb) |
| **navegador_archivos** | listar, stat, arbol, existe | Navegación de directorios con filtros glob/extensión, profundidad configurable |
| **buscador** | archivos, contenido, combinado | Find por patrón + grep recursivo con regex, contexto, paralelización |
| **editor** | leer, escribir, agregar, insertar, reemplazar, parchear, eliminar, crear_directorio, mover, copiar | Manipulación completa de archivos con backup automático y permisos octal |
| **procesos** | listar, info, matar, arbol | Gestión de procesos vía /proc en Linux (CPU%, RAM%, threads, cmdline) |
| **monitor** | completo, cpu, memoria, disco, red, uptime | Métricas en tiempo real: load avg, cores/frecuencias, RAM/SWAP, statvfs, /proc/net/dev |
| **instalador** | instalar, desinstalar, actualizar, buscar, info, gestores, actualizar_todo | 16 gestores soportados: apt, snap, dnf, pacman, brew, pip, npm, cargo, gem, composer, go, etc. |

### Endpoints

```
GET    /api/v1/herramientas                       # listar catálogo
GET    /api/v1/herramientas/{nombre}              # info de herramienta
POST   /api/v1/herramientas/ejecutar              # ejecutar (body: {nombre, parametros, timeout_segundos})
GET    /api/v1/herramientas/metricas              # métricas globales
GET    /api/v1/herramientas/metricas/{nombre}     # métricas por herramienta
```

### Ejemplo de uso

```bash
# Listar herramientas disponibles
curl http://localhost:3000/api/v1/herramientas

# Ejecutar la herramienta 'terminal' para un echo
curl -X POST http://localhost:3000/api/v1/herramientas/ejecutar \
  -H "Content-Type: application/json" \
  -d '{"nombre": "terminal", "parametros": {"comando": "echo", "args": ["hola"]}}'

# Ver métricas de uso
curl http://localhost:3000/api/v1/herramientas/metricas
```

### Arquitectura de herramientas

```
┌─────────────────────────────────────────────────────┐
│  HTTP: POST /api/v1/herramientas/ejecutar           │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  Catalogo.Ejecutar(ctx, nombre, params)             │
│  - lookup por nombre (thread-safe)                  │
│  - mide latencia automáticamente                    │
│  - inyecta metadata (duracion_ms, herramienta)      │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  Herramienta.Ejecutar(ctx, params) → Resultado      │
│  - valida params con helpers (ObtenerString/Int/…)  │
│  - respeta ctx.Done() (cancellation/timeout)        │
│  - retorna Resultado{Exito, Datos, Error, Metadata} │
└──────────────────────┬──────────────────────────────┘
                       ▼
┌─────────────────────────────────────────────────────┐
│  Metricas.RegistrarEjecucion(nombre, exito, dur)    │
│  - exitos, fallos, tasa, latencia min/max/prom      │
│  - último error, último uso                         │
└─────────────────────────────────────────────────────┘
```

## Sistema de Auto-Creación de Herramientas (Fase 6)

Liz **nunca dice "no puedo"**: si necesita una herramienta que no tiene, la crea.
La Fase 6 implementa el principio D-005 (Auto-Suficiencia) mediante un flujo
completo **detectar → generar → compilar → cargar → registrar** que produce
herramientas Go funcionales y las integra al catálogo existente.

### Cómo funciona

```
USUARIO: "Comprime todos los .csv de /home/user/data"
  │
  ▼
DETECTOR (LLM): "Reviso el catálogo... no tengo 'compresor'. Hace falta."
  │  → Genera SpecHerramienta {nombre, descripción, parámetros}
  ▼
GENERADOR (LLM): "Escribo código Go que implementa la interfaz Herramienta
                  vía protocolo subprocess (JSON stdin/stdout)."
  │  → Produce fuente.go completo (solo stdlib)
  ▼
COMPILADOR: `go build -o herramienta fuente.go`
  │  → Binario standalone (~2MB)
  ▼
CARGADOR: HerramientaSubproceso wraps el binario, implementa Herramienta
  │  → Validar() invoca op="validar", Ejecutar() invoca op="ejecutar"
  ▼
REGISTRO: persiste en ~/.liz/herramientas/auto_creadas/{nombre}/
  │  ├── fuente.go        (código fuente)
  │  ├── herramienta      (binario)
  │  ├── metadata.json    (spec + timestamps + stats)
  │  └── compilacion.log  (logs)
  ▼
CATÁLOGO: registra la nueva herramienta → disponible inmediatamente
  │
  ▼
"✅ Compresor creado y registrado. 1 herramienta nueva disponible."
```

### Arquitectura del paquete `auto_creacion`

```
internal/nucleo/herramientas/auto_creacion/
├── doc.go              # Documentación completa del paquete
├── tipos.go            # SpecHerramienta, MetadataHerramienta, SolicitudCreacion, ResultadoCreacion
├── plantillas.go       # Prompts LLM + ejemplo protocolo + helpers ExtraerFuenteGo/ValidarFuenteGo
├── detector.go         # Analiza petición + catálogo → identifica herramientas faltantes
├── generador.go        # LLM produce código Go; fallback GenerarDesdePlantilla (sin LLM)
├── compilador.go       # Ejecuta `go build` con timeout, captura logs
├── cargador.go         # HerramientaSubproceso implementa Herramienta (subprocess JSON)
├── registro.go         # Persistencia en disco + índice global
├── gestor.go           # Orquesta flujo completo + CargarTodas + Recargar + Eliminar + Probar
├── auto_creacion_test.go       # 18 tests unitarios
└── integracion_test.go         # 14 tests de integración (compilación real + subprocess)
```

### Protocolo subprocess (Liz ↔ herramienta)

Cada herramienta auto-creada es un binario Go standalone que se comunica
con Liz por JSON sobre stdin/stdout. Esto es **más robusto que Go plugins**
(no requiere versión exacta de Go, mismo module path, ni dependencias iguales)
y aísla fallos (un panic no tira a Liz).

```
REQUEST (Liz → herramienta, una línea JSON por stdin):
  {"operacion": "info|validar|ejecutar", "parametros": {...}, "timeout_ms": 5000}

RESPONSE (herramienta → Liz, una línea JSON por stdout):
  {"exito": true, "datos": <any>, "error": "", "metadata": {...}}
```

### Endpoints

```
# Detección y creación
POST   /api/v1/herramientas/auto-crear                    # Flujo completo (detect→generar→compilar→registrar)
POST   /api/v1/herramientas/detectar                      # Solo detectar (preview, sin crear)

# Gestión de herramientas auto-creadas
GET    /api/v1/herramientas/auto-creadas                  # Listar todas
GET    /api/v1/herramientas/auto-creadas/{nombre}         # Info detallada + estadísticas
DELETE /api/v1/herramientas/auto-creadas/{nombre}         # Eliminar (registro + catálogo + artifacts)

# Operaciones
POST   /api/v1/herramientas/auto-creadas/{nombre}/probar  # Ejecutar con parámetros de prueba
POST   /api/v1/herramientas/auto-creadas/{nombre}/recargar # Recompilar desde fuente (con o sin LLM)

# Inspección
GET    /api/v1/herramientas/auto-creadas/{nombre}/fuente  # Ver código Go (texto plano)
GET    /api/v1/herramientas/auto-creadas/{nombre}/log     # Ver log de última compilación
```

### Ejemplos de uso

```bash
# Detectar qué herramientas faltan para una petición
curl -X POST http://localhost:3000/api/v1/herramientas/detectar \
  -H "Content-Type: application/json" \
  -d '{"descripcion": "Comprime todos los .csv y súbelos por SFTP a backup.example.com"}'

# Crear una herramienta automáticamente (flujo completo)
curl -X POST http://localhost:3000/api/v1/herramientas/auto-crear \
  -H "Content-Type: application/json" \
  -d '{"descripcion": "Compresor de archivos ZIP"}'

# Crear con spec forzada (sin detector, útil para tests o creación manual)
curl -X POST http://localhost:3000/api/v1/herramientas/auto-crear \
  -H "Content-Type: application/json" \
  -d '{
    "forzar_spec": {
      "nombre": "saludador",
      "descripcion": "Saluda al usuario",
      "categoria": "test",
      "parametros": [
        {"nombre": "nombre", "tipo": "string", "requerido": true, "descripcion": "A quién saludar"}
      ]
    }
  }'

# Listar herramientas auto-creadas
curl http://localhost:3000/api/v1/herramientas/auto-creadas

# Probar una herramienta
curl -X POST http://localhost:3000/api/v1/herramientas/auto-creadas/saludador/probar \
  -H "Content-Type: application/json" \
  -d '{"parametros": {"nombre": "Mundo"}}'

# Ver el código fuente generado
curl http://localhost:3000/api/v1/herramientas/auto-creadas/saludador/fuente

# Recompilar desde fuente (editar fuente.go a mano y recompilar)
curl -X POST http://localhost:3000/api/v1/herramientas/auto-creadas/saludador/recargar \
  -H "Content-Type: application/json" \
  -d '{"usar_llm": false}'

# Eliminar
curl -X DELETE http://localhost:3000/api/v1/herramientas/auto-creadas/saludador
```

### Persistencia entre sesiones

Las herramientas auto-creadas se guardan en `~/.liz/herramientas/auto_creadas/`
y se **cargan automáticamente al iniciar Liz**. Si una herramienta no compila
o falla al cargar, se marca en su metadata pero no aborta el arranque — las
demás se cargan normalmente.

### Modos de operación

| Modo | LLM | Descripción |
|------|-----|-------------|
| **Completo** | ✅ | Detector+Generador usan LLM (NVIDIA) → herramientas reales y funcionales |
| **Forzado** | ✅/❌ | Caller pasa `forzar_spec` o `forzar_nombre` → salta detector, usa LLM si hay |
| **Fallback stub** | ❌ | Sin LLM: genera un stub compilable que responde info/validar pero en ejecutar retorna "no implementado" — útil para probar el flujo sin API key |

### Seguridad

- El código generado se compila y ejecuta con los permisos del usuario que corre Liz.
- `Validar()` hace una prueba controlada (op="validar") sin side-effects.
- El `Cargador` captura panics del subprocess (exit code != 0) y los convierte en `Resultado.Exito=false` con stderr como Error.
- El timeout del context se transmite al subprocess (SIGKILL tras expirar).
- Solo se permite código con stdlib (sin imports externos) → minimiza superficie de ataque.

## Sistema de Pipeline de Chat (Fase 7)

El pipeline conecta todos los subsistemas existentes en un flujo end-to-end coherente:
mensaje del usuario → clasificación de intención → planificación → ejecución de herramientas → respuesta.

### Arquitectura

```
USUARIO: "Mata todos los procesos que consuman mas de 2GB de RAM"
  │
  ▼
RECEPTOR: valida → crea/retoma sesión → almacena mensaje
  │
  ▼
CLASIFICADOR: "gestión de procesos" (confianza: 0.85)
  │  Heurísticas rápidas → LLM para casos ambiguos
  ▼
PLANIFICADOR: "paso 1: listar procesos, paso 2: filtrar por RAM > 2GB, paso 3: matar"
  │  Selecciona herramientas necesarias, respeta dependencias
  ▼
EJECUTOR: ejecuta monitor → procesos (con auto-creación si falta herramienta)
  │  Maneja timeouts, captura resultados, resuelve dependencias
  ▼
RESPONDEDOR: construye prompt con contexto + resultados → LLM → stream SSE
  │
  ▼
"✅ Procesos terminados. Se encontraron 3 procesos, 2 eliminados."
```

### Componentes

| Componente | Función |
|-----------|---------|
| **Receptor** | Valida entrada, gestiona sesiones, almacena mensajes |
| **Clasificador** | 10 categorías, heurísticas + LLM, confianza 0-1 |
| **Planificador** | Descompone en pasos, selecciona herramientas, respeta dependencias |
| **Ejecutor** | Ejecuta secuencialmente, maneja timeouts, auto-creación |
| **Respondedor** | Ensambla prompt completo, genera respuesta, streaming SSE |

### Endpoints

```
POST   /api/v1/chat             # Enviar mensaje (JSON o SSE)
GET    /api/v1/chat             # Estado del pipeline + métricas
GET    /api/v1/chat/metricas     # Métricas detalladas (por categoría, modelo)
GET    /api/v1/chat/sesiones      # Listar sesiones de chat
POST   /api/v1/chat/sesiones      # Crear nueva sesión
GET    /api/v1/chat/sesiones/{id}  # Detalle + mensajes de sesión
DELETE /api/v1/chat/sesiones/{id}  # Cerrar sesión
```

### Ejemplos de uso

```bash
# Chat simple (JSON)
curl -X POST http://localhost:3000/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"mensaje": "hola liz, que puedes hacer?"}'

# Chat con streaming (SSE)
curl -X POST http://localhost:3000/api/v1/chat \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -d '{"mensaje": "lista los procesos que consumen mas RAM", "stream": true}'

# Chat con contexto de proyecto
curl -X POST http://localhost:3000/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"mensaje": "que hace el main.go?", "proyecto": "mi-proyecto"}'

# Ver estado del pipeline
curl http://localhost:3000/api/v1/chat

# Ver métricas
curl http://localhost:3000/api/v1/chat/metricas
```

## Stack

| Componente | Tecnologia |
|-----------|-----------|
| Backend | Go |
| Frontend | React + TypeScript + Vite |
| IA | API NVIDIA (8+ modelos) |
| Streaming | Server-Sent Events |

---

> **IMPORTANTE: Si cambias de modelo de IA, lee `docs/ARQUITECTURA.md` primero.**
> Ahi esta TODO: principios, decisiones, flujos, estructura. El repo se autodocumenta.
