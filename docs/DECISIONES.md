# Registro de Decisiones de Diseño

> Todas las decisiones tomadas durante la planificación de Liz, con justificación.
> Formato: contexto → decisión → por qué → alternativas consideradas.

---

## D-001: Lenguaje del Backend → Go

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Necesitamos el lenguaje más estable posible para un agente que controla un sistema operativo completo: gestiona procesos, archivos, red, instala paquetes.

### Decisión
Go (Golang) como lenguaje principal del backend.

### Por qué
- Binario estático compilado: cero dependencias runtime
- Concurrency nativa con goroutines (múltiples tareas en paralelo)
- Manejo robusto de procesos del sistema
- Muy estable en producción (Docker, Kubernetes, Terraform son Go)
- Cross-compilation sencilla (Linux x64, ARM64, macOS)

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| Python | No binario estático, GIL limita concurrency, más lento para ops de sistema |
| Rust | Overkill, curva de aprendizaje pronunciada, desarrollo más lento |
| C++ | Demasiado complejo,unsafe memory management, tiempo de desarrollo largo |
| Node.js | Single-threaded, no ideal para control de sistema, dependencias npm |

---

## D-002: Interfaz de Herramientas → Patrón Interface Go

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Liz necesita un sistema de herramientas extensible donde tanto herramientas integradas como auto-creadas funcionen de forma uniforme.

### Decisión
Definir una interfaz Go `Herramienta` que toda herramienta DEBE implementar:
- `Nombre() string`
- `Descripcion() string`
- `Parametros() []Parametro`
- `Ejecutar(ctx, params) (Resultado, error)`
- `Validar() error`

### Por qué
- Contrato claro y verificable en compile-time
- Las herramientas auto-creadas se validan contra esta interfaz
- Fácil agregar nuevas herramientas sin modificar código existente
- Testing unitario por herramienta independiente

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| Plugins dinámicos (dlopen) | Complejidad innecesaria, problemas de seguridad |
| Scripts shell | No tipado, difícil de validar, errores silenciosos |
| gRPC microservicios | Overkill para herramientas locales |

---

## D-003: Sistema de Contexto → Mapa Bajo Demanda

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Los agentes actuales cargan todo el contexto del proyecto en el prompt del modelo. Esto satura el contexto, gasta tokens y pierde rendimiento.

### Decisión
Implementar sistema de "mapa bajo demanda":
1. Liz escanea el entorno y genera un MAPA (resumen de cada archivo)
2. El modelo recibe el mapa (no el contenido)
3. El modelo pide los archivos específicos que necesita
4. Liz entrega solo esos archivos

### Por qué
- No satura el contexto con información irrelevante
- Escala a proyectos de millones de líneas
- El modelo trabaja con exactamente lo que necesita
- Ahorra tokens (costo reducido)
- Más rápido (menos tokens = respuesta más rápida)

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| Cargar todo (claude code) | Satura contexto, caro, lento |
| RAG con embeddings | Complejidad alta, necesita vector DB, overkill |
| Resumen automático del proyecto | Pierde detalle, no permite acceso granular |

---

## D-004: Orquestación Multi-Modelo → Selector Inteligente

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Depender de un solo modelo de IA limita las capacidades. Claude es mejor razonando, Llama es mejor para código general, Mixtral es más rápido.

### Decisión
Crear un orquestador que:
1. Clasifica la tarea del usuario (código, análisis, razonamiento, etc.)
2. Selecciona el mejor modelo de los 8+ disponibles en API NVIDIA
3. Tiene fallback automático si el modelo falla
4. Registra métricas para mejorar selecciones futuras

### Por qué
- Ningún modelo es el mejor en TODO
- Optimización de costo (usar modelo barato para tareas simples)
- Optimización de velocidad (modelo rápido para tareas simples)
- Resiliencia (fallback si un modelo falla)
- Métricas permiten aprendizaje continuo

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| Modelo único (claude code) | Un modelo no es mejor en todo |
| Round-robin simple | No inteligente, no optimiza por tipo de tarea |
| usuario elige manualmente | Fricción, usuario no siempre sabe cuál es mejor |

---

## D-005: Auto-Creación de Herramientas → Generación + Compilación + Validación

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Liz debe ser auto-suficiente. Si necesita una herramienta que no tiene, debe crearla.

### Decisión
Flujo de auto-creación:
1. Detector identifica falta de herramienta
2. Orquestador genera código Go (implementa interfaz Herramienta)
3. Compilador compila el código
4. Validador verifica que funciona
5. Registro guarda la herramienta y su metadata
6. La herramienta queda disponible inmediatamente

### Por qué
- Liz nunca dice "no puedo"
- Crece y mejora con cada interacción
- Herramientas se guardan entre sesiones
- Usuario ve el proceso (no es invisible)

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| Python scripts | No tipado, frágil, difícil de validar |
| Plugins compilados separados | Complejidad de build, gestión de dependencias |
| No auto-crear (pedir al usuario) | Viola principio de auto-suficiencia |

---

## D-006: Permisos → Una Vez al Iniciar

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Claude Code pide permiso para cada acción, lo cual es molesto y rompe el flujo.

### Decisión
Solicitar permisos completos una sola vez al iniciar. Guardarlos hasta el cierre.

### Por qué
- Cero fricción durante el uso
- Flujo de trabajo ininterrumpido
- El usuario confía en Liz (la instaló voluntariamente)
- Similar a cómo funciona sudo con timestamp_timeout

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| Pedir cada vez (claude code) | Molesto, rompe flujo |
| Sin permisos | Peligroso, sin control del usuario |
| Permisos granulares por accion | Complejo, lento |

---

## D-007: Frontend → React + TypeScript + Vite + Tailwind

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Necesitamos una interfaz estilo ChatGPT clásico: sidebar, chat area, streaming, markdown.

### Decisión
Stack: React + TypeScript + Vite + Tailwind CSS

### Por qué
- React: ecosistema más grande, componentes reutilizables
- TypeScript: tipado seguro, menos bugs
- Vite: build ultrarrápido, HMR instantáneo
- Tailwind: desarrollo rápido, tema oscuro fácil

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| Vue.js | Ecosistema más pequeño |
| Svelte | Ecosistema más pequeño, menos librerías |
| Next.js | Overkill (no necesitamos SSR) |
| Angular | Pesado, verboso |

---

## D-008: Streaming → Server-Sent Events (SSE)

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Las respuestas del modelo deben aparecer progresivamente en el frontend, no esperar a que termine.

### Decisión
Usar SSE (Server-Sent Events) para streaming de respuestas.

### Por qué
- Más simple que WebSocket
- Perfecto para flujo unidireccional (servidor → cliente)
- Soporte nativo en browsers
- API NVIDIA soporta streaming
- Fácil de implementar en Go

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| WebSocket | Overkill para flujo unidireccional |
| Long polling | Ineficiente, latencia alta |
| Polling | Mala UX, gasta recursos |

---

## D-009: API de IA → NVIDIA

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Necesitamos acceso a múltiples modelos de IA desde un solo proveedor.

### Decisión
API de NVIDIA (integrate.api.nvidia.com/v1)

### Por qué
- 8+ modelos disponibles desde un solo endpoint
- Compatible con formato OpenAI (misma API)
- Buena disponibilidad
- Endpoint estándar de chat completions

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| OpenAI directo | Menos modelos disponibles, más caro |
| Anthropic directo | Solo Claude, un solo modelo |
| Ollama local | Depende de hardware del usuario, no siempre disponible |
| HuggingFace | Más complejo, menos estable |

---

## D-010: Almacenamiento de Contexto → Archivos en ~/.liz/

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
¿Dónde almacenar el contexto fragmentado de Liz? ¿Base de datos? ¿Archivos?

### Decisión
Sistema de archivos en `~/.liz/contexto/` con fragmentos JSON.

### Por qué
- Sin dependencias externas (no necesita BD)
- Inspeccionable a mano (son archivos JSON)
- Fácil de backup (copiar directorio)
- Fácil de limpiar (borrar directorio)
- Rendimiento suficiente (lectura de archivos pequeños es rápida)
- Los archivos se pueden versionar con git si el usuario quiere

### Alternativas consideradas
| Alternativa | Por qué se descartó |
|------------|-------------------|
| SQLite | Bien, pero agrega dependencia |
| PostgreSQL | Overkill para almacenamiento local |
| Redis | Volátil, no persiste entre reinicios |
| BoltDB | Bien, pero menos inspeccionable |

---

## D-011: Nombre del Proyecto → Liz

**Fecha:** 2026-07-25
**Estado:** Aprobada

### Contexto
Necesitamos un nombre para el agente.

### Decisión
"Liz"

### Por qué
Corto, memorable, fácil de pronunciar, funciona como comando (`liz`), sin conflictos con nombres de herramientas existentes.

---

## D-012: Licencia → Pendiente

**Fecha:** 2026-07-25
**Estado:** Por decidir

### Contexto
¿Qué licencia usar para el proyecto?

### Opciones
- MIT: Libre, permisiva, cualquiera puede usar y modificar
- Apache 2.0: Libre con protección de patentes
- GPL v3: Libre, requiere que derivados sean también libres

### Nota
Pendiente de decisión del creador.

---

*Última actualización: 2026-07-25*
