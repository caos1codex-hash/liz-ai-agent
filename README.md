# Liz — AI Agent para Linux

> Agente de IA autonomo que controla completamente tu Linux mediante lenguaje natural.
> No es un chatbot. No es un asistente de codigo. Es un sistema operativo de IA.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8)
![Phase](https://img.shields.io/badge/fase-4%20de%2010-orange)
![Tests](https://img.shields.io/badge/tests-342%20pasando-brightgreen)

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
| 5 | Herramientas Base | [#13](https://github.com/caos1codex-hash/liz-ai-agent/issues/13) | ⏳ |
| 6 | Auto-Creacion | [#14](https://github.com/caos1codex-hash/liz-ai-agent/issues/14) | ⏳ |
| 7 | Pipeline de Chat | [#15](https://github.com/caos1codex-hash/liz-ai-agent/issues/15) | ⏳ |
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
- **Embeddings**: stub para nv-embed-v1 (integracion con buscador en Fase 4.1)

```
GET  /api/v1/orquestador                  # estado
GET  /api/v1/orquestador/modelos          # listar modelos (sin API keys)
GET  /api/v1/orquestador/metricas         # metricas de uso
POST /api/v1/orquestador/completar        # chat completion (JSON o SSE)
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
