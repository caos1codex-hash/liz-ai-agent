# Liz AI Agent — Documentación Completa de Arquitectura

> **Este documento es la fuente de verdad del proyecto.** Si cambias de modelo de IA, 
> si olvidas algo, o si alguien nuevo se une al proyecto: **lee este archivo primero**. 
> Aquí está TODO lo que se ha decidido, discutido y planificado.

---

## 1. ¿Qué es Liz?

Liz es un agente de IA autónomo para Linux que controla completamente el sistema operativo 
mediante lenguaje natural. Combina las capacidades de Claude Code/Codex (manipulación de 
código y proyectos) con las de Google Assistant en Android (control del sistema, búsquedas, 
tareas automatizadas), pero con una arquitectura de contexto completamente nueva.

### Liz NO es:
- Un chatbot más
- Un asistente de código más
- Un wrapper de API de IA

### Liz SÍ es:
- Un **sistema operativo de IA** para Linux
- Un agente que **se auto-mejora permanentemente**
- Un orquestador que **elige inteligentemente** entre 8+ modelos
- Un sistema que **nunca dice "no puedo"**

---

## 2. Principios Fundamentales (INMUTABLES)

Estos principios NO se negocian. Si algo los viola, está mal.

### 2.1 Contexto Bajo Demanda — "El Catálogo de la Biblioteca"

**Problema que resuelve:** Todos los agentes actuales cargan TODO el contexto del proyecto 
(miles de archivos, millones de líneas) en el prompt del modelo. Esto satura el contexto, 
gasta tokens innecesarios y hace que el modelo pierda foco.

**Cómo funciona Liz:**
```
AGENTE TRADICIONAL:
  Modelo: "Necesito contexto del proyecto"
  Agente: "Aquí tienes los 500,000 líneas de código del proyecto completo"
  Modelo: *se ahoga, pierde rendimiento, gasta millones de tokens*

LIZ:
  Modelo: "Necesito contexto del proyecto"
  Liz: "Aquí tienes el MAPA del proyecto:"
  Liz: {
    "src/main.go": "Punto de entrada - servidor HTTP puerto 8080, 120 líneas",
    "src/auth.go": "Autenticación JWT, middleware, 85 líneas",
    "config.yaml": "Configuración: BD, puertos, API keys, 40 líneas"
  }
  Modelo: "Necesito ver src/auth.go"
  Liz: *entrega SOLO ese archivo*
  Modelo: *trabaja con exactamente lo que necesita*
```

**Por qué funciona:**
- El modelo ve la estructura completa sin leer todo
- Solo pide lo que necesita
- Cero saturación de contexto
- Escala a proyectos de millones de líneas
- Gasta una fracción de los tokens

### 2.2 Auto-Suficiencia — "Nunca Dice No"

**Problema que resuelve:** Los agentes actuales tienen un set fijo de herramientas. Si no 
tienen una para la tarea, simplemente dicen "no puedo hacer eso".

**Cómo funciona Liz:**
```
AGENTE TRADICIONAL:
  Usuario: "Monitorea la temperatura del CPU cada 5 segundos"
  Agente: "No tengo una herramienta para monitorear temperatura. No puedo hacer esto."

LIZ:
  Usuario: "Monitorea la temperatura del CPU cada 5 segundos"
  Liz: *revisa catálogo de herramientas* "No tengo herramienta de monitoreo continuo"
  Liz: *genera código Go para la herramienta* "Programando monitor_temperatura.go..."
  Liz: *compila* *valida* *registra*
  Liz: "CPU: 45°C, actualizando cada 5s..."
  Liz: *herramienta guardada para uso futuro*
```

**Liz crece con cada interacción:**
- Semana 1: 7 herramientas (las integradas)
- Semana 2: 15 herramientas (8 auto-creadas)
- Mes 3: 50+ herramientas
- Año 1: Cientos de herramientas

### 2.3 Permisos Una Vez

**Problema que resuelve:** Claude Code y similares piden permiso para CADA acción. "¿Puedo 
ejecutar este comando?" "¿Puedo editar este archivo?" Esto crea fricción constante.

**Cómo funciona Liz:**
```
AL INICIAR LIZ (primera vez):
  🔴 Liz necesita los siguientes permisos:
     ✓ Acceso completo al sistema de archivos
     ✓ Ejecución de comandos en terminal
     ✓ Gestión de procesos (listar, iniciar, detener, matar)
     ✓ Acceso a red (HTTP requests, descargas)
     ✓ Instalación de paquetes del sistema
     ✓ Acceso a /proc y /sys (monitor de sistema)
     
     [ Conceder todo ]  [ Seleccionar manualmente ]

DESPUÉS:
  Liz NUNCA vuelve a pedir permisos hasta el próximo reinicio.
  Todo fluye sin interrupciones.
```

### 2.4 Multi-Modelo Inteligente

**Problema que resuelve:** Claude Code usa solo Claude. Codex usa solo GPT. Un solo modelo 
no es el mejor en TODO. Llama es mejor para código general, Claude para razonamiento profundo, 
Mixtral para análisis rápido.

**Cómo funciona Liz:**
```
Usuario: "Resume este archivo de 10,000 líneas"
  → Orquestador: "Tarea de resumen, necesito velocidad y bajo costo"
    → Elige: Mixtral 8x22B (rápido, barato, buen resumen)

Usuario: "Diseña una arquitectura de microservicios completa"
  → Orquestador: "Tarea de razonamiento profundo, complejidad alta"
    → Elige: Llama 3.1 405B (razonamiento profundo)

Usuario: "¿Qué línea de este código tiene el bug?"
  → Orquestador: "Tarea de análisis de código específico"
    → Elige: CodeLlama 70B (especializado en código)

Si el modelo falla → fallback automático al siguiente mejor modelo.
```

---

## 3. Stack Tecnológico (Decisiones y Justificación)

| Componente | Tecnología | Por qué esta y no otra |
|-----------|-----------|----------------------|
| **Backend** | **Go 1.22** | Estabilidad máxima, binario estático (no dependencias), concurrency nativa (goroutines), perfecto para gestión de procesos del sistema, Docker/Kubernetes/Terraform están hechos en Go |
| **GUI de escritorio** | **Fyne v2.8 (OpenGL/GLFW)** | App 100% nativa, sin navegador ni WebView ni Electron. Un solo binario que arranca servidor HTTP + ventana. Pinta vía OpenGL, accesorios vía X11/Wayland. Dependencias mínimas (libGL + libX*) |
| **Markdown** | **Fyne RichText + goldmark** | Parser markdown integrado en Fyne, sin dependencias JS |
| **Streaming** | **Server-Sent Events (SSE)** | Más simple que WebSocket, perfecto para respuestas unidireccionales, soporte nativo HTTP |
| **Configuración** | **YAML** | Legible, editable a mano, soporta estructuras complejas |
| **IA** | **API NVIDIA** | 8+ modelos disponibles, orquestación flexible, endpoint estándar OpenAI-compatible |
| **Tests** | **Go testing** | Cobertura unitaria + integración en un solo toolchain |

### Por qué NO Python para el backend:
Se consideró Python por conveniencia, pero se descartó porque:
- No compila a binario estático (necesita runtime)
- Gestión de procesos del sistema es menos robusta
- GIL limita concurrency real
- Más lento para operaciones de sistema
- Mayor superficie de ataque de seguridad

### Por qué NO Rust:
Se consideró Rust por estabilidad, pero se descartó porque:
- Curva de aprendizaje muy pronunciada
- Tiempo de desarrollo más largo
- Overkill para este caso de uso
- Go ofrece suficiente estabilidad con mucha más velocidad de desarrollo

---

## 4. Estructura del Proyecto

```
liz-ai-agent/
├── cmd/
│   └── liz/
│       └── main.go                    # Punto de entrada del binario
├── internal/                           # Código privado (no importable externamente)
│   ├── nucleo/                       # Módulo base
│   │   ├── servidor/                 # Servidor HTTP
│   │   │   └── servidor.go
│   │   ├── config/                    # Lectura de configuración YAML
│   │   │   └── config.go
│   │   ├── permisos/                  # Sistema de permisos
│   │   │   └── permisos.go
│   │   └── logger/                    # Logging estructurado
│   │       └── logger.go
│   ├── pipeline/                      # Pipeline de chat completo
│   │   ├── receptor.go                # Recibe mensajes del usuario
│   │   ├── clasificador.go            # Clasifica intenciones
│   │   ├── planificador.go            # Descompone tareas en pasos
│   │   ├── ejecutor.go               # Ejecuta herramientas
│   │   └── respondedor.go             # Envía respuesta con streaming
│   ├── orquestador/                   # Orquestador multi-modelo
│   │   ├── cliente_nvidia/            # Cliente HTTP para API NVIDIA
│   │   │   └── cliente.go
│   │   ├── catalogo/                  # Catálogo de modelos disponibles
│   │   │   └── catalogo.go
│   │   ├── clasificador/              # Clasifica tipo de tarea
│   │   │   └── clasificador.go
│   │   ├── selector/                  # Selecciona mejor modelo
│   │   │   └── selector.go
│   │   └── metricas/                  # Métricas de uso por modelo
│   │       └── metricas.go
│   ├── contexto/                       # Sistema de contexto inteligente
│   │   ├── mapa/                      # Generador de mapas de directorios
│   │   │   └── mapa.go
│   │   ├── fragmentos/                # Almacenamiento fragmentado
│   │   │   └── fragmentos.go
│   │   ├── indice/                    # Índice/árbol de fragmentos
│   │   │   └── indice.go
│   │   └── resumen/                   # Generador de resúmenes
│   │       └── resumen.go
│   └── herramientas/                   # Sistema de herramientas
│       ├── interface.go               # Interfaz estándar Herramienta
│       ├── integradas/                # Herramientas que vienen con Liz
│       │   ├── terminal.go
│       │   ├── navegador_archivos.go
│       │   ├── buscador.go
│       │   ├── editor.go
│       │   ├── procesos.go
│       │   ├── monitor.go
│       │   └── instalador.go
│       ├── auto_creadas/              # Herramientas que Liz se programa
│       │   └── (se van creando dinámicamente)
│       ├── auto_creacion/             # Sistema de auto-creación
│       │   ├── detector.go            # Detecta cuando falta herramienta
│       │   ├── generador.go           # Genera código Go de herramienta
│       │   ├── compilador.go         # Compila y valida
│       │   └── registro.go           # Registro persistente
│       └── registro/                 # Catálogo y métricas de herramientas
│           ├── catalogo.go
│           └── metricas.go
├── pkg/                                # Código reutilizable público
├── configs/
│   └── liz.yaml                       # Configuración principal
├── web/                                # Frontend React
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── components/
│       │   ├── Layout/
│       │   ├── Header/
│       │   ├── Sidebar/
│       │   ├── Chat/
│       │   │   ├── ChatArea.tsx
│       │   │   ├── MessageBubble.tsx
│       │   │   ├── CodeBlock.tsx
│       │   │   ├── ThinkingIndicator.tsx
│       │   │   └── ToolIndicator.tsx
│       │   └── Input/
│       │       └── ChatInput.tsx
│       ├── hooks/
│       │   ├── useChat.ts
│       │   ├── useStreaming.ts
│       │   └── useConversations.ts
│       ├── services/
│       │   └── api.ts
│       ├── types/
│       │   └── index.ts
│       └── styles/
│           └── globals.css
├── scripts/
│   ├── install.sh                     # Instalador automático
│   └── uninstall.sh                   # Desinstalador
├── docs/
│   ├── ARQUITECTURA.md
│   ├── CONFIGURACION.md
│   ├── MODELOS.md
│   ├── HERRAMIENTAS.md
│   └── CONTEXTO.md
├── tests/
│   └── e2e/
├── Makefile
├── go.mod
├── go.sum
├── README.md
├── CHANGELOG.md
└── LICENSE
```

---

## 5. Estructura del Contexto de Liz (en ~/.liz/)

Liz almacena TODO su estado en `~/.liz/`. Este directorio es el "cerebro" de persistencia.

```
~/.liz/
├── permisos.json                      # Permisos concedidos por el usuario
├── config.json                        # Configuración activa (merge de liz.yaml + overrides)
├── contexto/
│   ├── sistema/
│   │   ├── estado/
│   │   │   ├── sesion_actual.json      # Estado de la sesión actual
│   │   │   └── herramientas_registradas.json  # Catálogo actualizado
│   │   └── config/
│   ├── chat/
│   │   ├── conversaciones/
│   │   │   ├── conv_001/
│   │   │   │   ├── metadata.json       # Resumen de la conversación
│   │   │   │   ├── turno_001.msg       # Cada turno es un archivo separado
│   │   │   │   ├── turno_002.msg
│   │   │   │   └── turno_003.msg
│   │   │   ├── conv_002/
│   │   │   └── conv_N/
│   │   ├── indice.json                # Índice maestro (árbol de todas las convos)
│   │   └── preferencias/
│   │       ├── estilo.json             # Preferencias de estilo del usuario
│   │       └── historial.json          # Historial de comandos frecuentes
│   └── proyectos/                      # SEPARADO del contexto del sistema
│       ├── proyecto_001/
│       │   ├── .liz/
│       │   │   ├── mapa.json           # El "mapa" que entrega al modelo
│       │   │   ├── archivos/
│       │   │   │   ├── src_main_go.json    # Resumen de src/main.go
│       │   │   │   ├── config_yaml.json   # Resumen de config.yaml
│       │   │   │   └── (un .json por archivo del proyecto)
│       │   │   └── historial.json       # Historial de ediciones en este proyecto
│       ├── proyecto_002/
│       └── indice_global.json          # Índice de todos los proyectos
├── herramientas/
│   ├── auto_creadas/                   # Código fuente de herramientas auto-creadas
│   │   ├── monitor_temperatura.go
│   │   ├── limpiador_logs.go
│   │   └── (crece con el tiempo)
│   └── registro/
│       ├── auto_creadas/
│       │   ├── monitor_temperatura.json  # Metadata de cada herramienta creada
│       │   └── ...
│       └── metricas.json
└── logs/
    └── liz.log                         # Log estructurado en JSON
```

### Formato del mapa que se entrega al modelo:
```json
{
  "version": "1.0",
  "proyecto": "mi-proyecto-go",
  "timestamp": "2026-07-25T01:30:00Z",
  "archivos": {
    "src/main.go": "Punto de entrada - servidor HTTP en puerto 8080, 245 líneas",
    "src/auth.go": "Autenticación JWT + OAuth2, middleware, manejo de tokens, 180 líneas",
    "src/database.go": "Conexión PostgreSQL, migraciones, modelos, 320 líneas",
    "config.yaml": "Configuración general: base de datos, puertos, claves API",
    "tests/": "Directorio de tests - 12 archivos pytest"
  },
  "estructura_directorios": "src/ → 8 archivos, tests/ → 12 archivos, docs/ → 3 archivos",
  "resumen": "Proyecto Go con servidor HTTP, autenticación JWT y base de datos PostgreSQL"
}
```

---

## 6. Interfaz de Herramientas (Go)

Toda herramienta en Liz DEBE implementar esta interfaz:

```go
package herramientas

import "context"

// Parametro describe un parámetro que una herramienta acepta
type Parametro struct {
    Nombre     string      `json:"nombre"`
    Tipo       string      `json:"tipo"`        // "string", "int", "bool", "array", "object"
    Requerido  bool        `json:"requerido"`
    Default    interface{} `json:"default,omitempty"`
    Descripcion string     `json:"descripcion"`
}

// Resultado es lo que retorna toda herramienta después de ejecutarse
type Resultado struct {
    Exito    bool                   `json:"exito"`
    Datos    interface{}            `json:"datos"`
    Error    string                 `json:"error,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Herramienta es la interfaz que TODA herramienta debe implementar
type Herramienta interface {
    Nombre() string                                          // Nombre único de la herramienta
    Descripcion() string                                      // Qué hace (para el catálogo)
    Parametros() []Parametro                                 // Parámetros que acepta
    Ejecutar(ctx context.Context, params map[string]interface{}) (Resultado, error)
    Validar() error                                           // Verifica que la herramienta funciona
}
```

---

## 7. Configuración (liz.yaml)

```yaml
liz:
  version: "0.1.0"
  
  servidor:
    puerto: 3000
    host: "localhost"
  
  nvidia:
    api_key: "nvapi-***"  # Se configura aquí, NUNCA en el código
    endpoint: "https://integrate.api.nvidia.com/v1"
    modelos:
      - id: "meta/llama-3.1-405b-instruct"
        nombre: "Llama 3.1 405B"
        tipo: ["razonamiento", "complejo"]
        velocidad: "lenta"
        prioridad: 1
      - id: "meta/llama-3.1-70b-instruct"
        nombre: "Llama 3.1 70B"
        tipo: ["codigo", "general"]
        velocidad: "media"
        prioridad: 2
      - id: "mistralai/mixtral-8x22b-instruct-v0.1"
        nombre: "Mixtral 8x22B"
        tipo: ["analisis", "rapido"]
        velocidad: "alta"
        prioridad: 3
      - id: "nvidia/nemotron-4-340b-instruct"
        nombre: "Nemotron 340B"
        tipo: ["creatividad", "razonamiento"]
        velocidad: "media"
        prioridad: 4
      - id: "google/gemma-2-27b-it"
        nombre: "Gemma 2 27B"
        tipo: ["general", "eficiente"]
        velocidad: "alta"
        prioridad: 5
      - id: "microsoft/phi-3-medium-128k-instruct"
        nombre: "Phi-3 Medium"
        tipo: ["contexto_largo", "resumen"]
        velocidad: "media"
        prioridad: 6
      - id: "meta/codellama-70b-instruct"
        nombre: "CodeLlama 70B"
        tipo: ["codigo", "especializado"]
        velocidad: "media"
        prioridad: 7
      - id: "nvidia/llama-3.1-nemotron-70b-instruct"
        nombre: "Nemotron 70B"
        tipo: ["general", "potente"]
        velocidad: "media"
        prioridad: 8
  
  directorio_trabajo: "~"
  tema: "oscuro"
  
  permisos:
    solicitar_al_iniciar: true
    recordar_entre_sesiones: false
```

---

## 8. API Endpoints del Backend

### Endpoints del servidor
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/health` | Estado de Liz (healthcheck) |
| GET | `/api/config` | Configuración actual |
| PUT | `/api/config` | Modificar configuración |
| GET | `/api/permisos` | Estado de permisos |
| POST | `/api/permisos` | Conceder permisos |
| GET | `/api/tools` | Listar todas las herramientas |
| GET | `/api/tools/:nombre` | Info de herramienta específica |
| GET | `/api/orquestador/modelos` | Listar modelos disponibles |
| GET | `/api/orquestador/metricas` | Métricas de uso de modelos |
| GET | `/api/conversations` | Listar conversaciones |
| POST | `/api/conversations` | Nueva conversación |
| GET | `/api/conversations/:id` | Mensajes de conversación |
| DELETE | `/api/conversations/:id` | Eliminar conversación |
| **POST** | **`/api/chat`** | **Enviar mensaje (retorna SSE stream)** |

---

## 9. Flujo Completo de una Interacción

```
USUARIO escribe: "Elimina todos los .log de más de 30 días en /var/log"
  │
  ▼
FRONTEND: Envía POST /api/chat con { mensaje, conversation_id }
  │
  ▼
PIPELINE - RECEPTOR: Recibe mensaje, obtiene/crea conversación
  │  Guarda turno_XXX.msg en contexto/chat/conversaciones/conv_N/
  │  Actualiza índice
  │
  ▼
PIPELINE - CLASIFICADOR: Analiza intención
  │  Tipo: "sistema" | Subtipo: "manipulación_archivos"
  │  Necesita herramientas: SÍ
  │
  ▼
PIPELINE - PLANIFICADOR: Descompone en pasos
  │  Paso 1: Buscar archivos .log con más de 30 días
  │  Paso 2: Listar archivos encontrados
  │  Paso 3: Eliminar archivos
  │
  ▼
ORQUESTADOR: Selecciona modelo
  │  Clasificación: "tarea_sistema_simple"
  │  Modelo elegido: Mixtral 8x22B (rápido, suficiente para esto)
  │  Envía prompt al modelo via API NVIDIA
  │
  ▼
PIPELINE - EJECUTOR: Ejecuta herramientas
  │  Buscador.Ejecutar({ patron: "*.log", ruta: "/var/log", edad_max: "30d" })
  │    → Resultado: 47 archivos encontrados
  │  Terminal.Ejecutar({ comando: "rm", args: [lista de archivos] })
  │    → Resultado: 47 archivos eliminados
  │
  ▼
CONTEXTO: Actualiza
  │  Crea fragmento: "Eliminación de logs en /var/log - 47 archivos"
  │  Actualiza índice de conversación
  │
  ▼
PIPELINE - RESPONDEDOR (streaming SSE):
  │  → "Encontré 47 archivos .log con más de 30 días..."
  │  → "Eliminando archivos..."
  │  → "✅ 47 archivos eliminados correctamente"
  │  → [Modelo: Mixtral 8x22B] [Herramientas: Buscador, Terminal]
  │
  ▼
FRONTEND: Muestra respuesta progresivamente
  │  Indicador de herramientas usado: ✅ Buscador, ✅ Terminal
  │  Badge de modelo: Mixtral 8x22B
  │
  ▼
MÉTRICAS: Registra
     Tiempo total: 2.3s
     Modelo: Mixtral 8x22B
     Tokens: 450 input, 120 output
     Herramientas: 2 ejecutadas, 2 exitosas
```

---

## 10. Flujo de Auto-Creación de Herramientas

```
USUARIO: "Comprime todos los .csv de /home/user/data y envíalos por SFTP a server.com"
  │
  ▼
DETECTOR DE NECESIDADES:
  │  Revisa catálogo de herramientas:
  │    ✓ Buscador (existe)
  │    ✓ Terminal (existe)
  │    ✗ Compresor de archivos (NO EXISTE)
  │    ✗ Cliente SFTP (NO EXISTE)
  │  → Dispara auto-creación
  │
  ▼
GENERADOR:
  │  → Orquestador: "Genera código Go que implemente la interfaz Herramienta
  │    para comprimir archivos (soporte zip, tar.gz)"
  │  Modelo genera: compresor.go (implementa Herramienta)
  │
  │  → Orquestador: "Genera código Go que implemente la interfaz Herramienta
  │    para transferencia SFTP"
  │  Modelo genera: cliente_sftp.go (implementa Herramienta)
  │
  ▼
COMPILADOR Y VALIDADOR:
  │  go build compresor.go → ✅ Compila
  │  go build cliente_sftp.go → ✅ Compila
  │  Validación: ✅ Implementan interfaz, no paniquean
  │
  ▼
REGISTRO:
  │  Guarda compresor.go en auto_creadas/
  │  Guarda cliente_sftp.go en auto_creadas/
  │  Guarda metadata JSON para cada una
  │  Actualiza catálogo
  │
  ▼
EJECUCIÓN:
  │  Buscador.find("*.csv", "/home/user/data")
  │  Compresor.Ejecutar({ archivos: [...], formato: "tar.gz" })
  │  ClienteSFTP.Ejecutar({ host: "server.com", archivo: "data.tar.gz" })
  │
  ▼
RESPUESTA:
  "Encontré 15 archivos .csv..."
  "Comprimiendo a data.tar.gz..."
  "Subiendo a server.com via SFTP..."
  "✅ Transferencia completada"
  "💡 Nota: Creé 2 nuevas herramientas para esta tarea:
     - compresor (compresión de archivos)
     - cliente_sftp (transferencia SFTP)
     Están disponibles para uso futuro."
```

---

## 11. Diseño de la GUI de Escritorio Nativa

Interfaz estilo ChatGPT clásico, implementada en **Fase 8** como app de escritorio
**100% nativa** con **Fyne v2.8** (Go + OpenGL + GLFW). Sin navegador, sin WebView,
sin Electron, sin Tauri. Un solo binario `liz` que arranca el servidor HTTP y abre
la ventana:

```
┌──────────────────────────────────────────────────────┐
│  🟣 Liz AI              ● online · 🧠 mixtral  · 🌙  │
├──────────────┬───────────────────────────────────────┤
│              │ [Proyecto: mi-repo  ▾]                │
│ Conversaciones│                                       │
│ ─────────────│  🤖 Hola, soy Liz. ¿En qué puedo      │
│ ▸ Proyecto Web│      ayudarte hoy?                    │
│   Setup DB   │                                       │
│ ▸ Debug Server│  ─────────────────────────────────   │
│   Config     │  👤 Elimina los .log de más de 30     │
│              │     días en /var/log                  │
│ ─────────────│                                      │
│ + Nueva       │  🤖 Encontré 47 archivos .log...     │
│              │     ✅ 47 archivos eliminados           │
│              │  ─────────────────────────────────   │
│              │  🧠 mixtral · 🔧 buscador · 1.2s      │
│              │  ─────────────────────────────────   │
│              │  [ Escribe un mensaje... ]       ➤    │
└──────────────┴───────────────────────────────────────┘
```

Características de la GUI (implementadas en Fase 8):
- Sidebar con historial de conversaciones (CRUD completo, persistencia en Preferences)
- Streaming de respuestas SSE (texto aparece gradualmente, token a token)
- Markdown nativo vía Fyne RichText + goldmark (bold, listas, código, headings)
- Indicador del modelo usado (label en header + badge por mensaje)
- Indicador de herramientas usadas durante ejecución (badge por mensaje)
- Mensaje placeholder "Liz está pensando…" antes del primer chunk
- Tema oscuro por defecto, claro alternativo (toggle persistente)
- Selector de proyecto de contexto (Select con proyectos indexados)
- Panel de métricas desplegable (mensajes procesados, modelo más usado, categorías)
- Toasts para errores globales (auto-dismiss a los 4s)
- Welcome state con prompts de ejemplo cuando no hay sesión
- Auto-scroll al fondo durante streaming
- Mensajes optimistas del usuario (aparecen inmediatamente, marcados "enviando…")
- Badges de métricas por mensaje (tokens, pasos, duración, herramientas)
- Atajos de teclado: Ctrl+N (nueva), Ctrl+K (focus input), Ctrl+R (refrescar)
- Modo `--headless`: servidor HTTP sin GUI (para Docker/servidores)

### Stack técnico

| Capa | Tecnología |
|------|-----------|
| Toolkit | Fyne v2.8 |
| Lenguaje | Go 1.22 |
| Renderizado | OpenGL (vía GLFW) |
| Markdown | Fyne RichText + goldmark |
| Iconos | Iconos nativos del tema Fyne |
| Persistencia | fyne.App.Preferences() (sesión activa + tema) |
| SSE | Parser propio sobre net/http + bufio.Scanner |

### Estructura del código

```
internal/desktop/
├── doc.go              # Documentación del paquete (filosofía "nativo nativo")
├── cliente.go          # ClienteBackend: HTTP/SSE espejo de endpoints Go
├── tema.go             # Tema personalizado (paleta morada Liz, dark/light)
├── iconos.go           # Helpers centralizados de iconos Fyne
├── app.go              # App struct orquestadora (Sidebar+Header+Chat+Toasts)
├── sidebar.go          # Sidebar conversaciones (CRUD sesiones)
├── header.go           # Header (status · modelo · métricas · theme toggle)
├── chat.go             # ChatWindow (lista mensajes + input + SSE streaming)
├── mensaje.go          # MensajeBubble (markdown + badges metadata)
├── status_dot.go       # StatusDot (canvas.Circle con 3 estados)
├── project_selector.go # Selector de proyecto de contexto
└── toast.go            # Notificaciones efímeras (PopUp auto-dismiss)
```

### Flujo SSE (streaming)

```
1. Usuario envía mensaje
   ↓
2. ChatWindow.enviarMensaje() crea mensaje optimista + placeholder asistente
   ↓
3. ClienteBackend.StreamChat() hace POST /api/v1/chat con stream=true,
   Accept: text/event-stream
   ↓
4. Backend responde con chunks SSE (text/event-stream):
   data: {"tipo":"estado","contenido":"Iniciando pipeline..."}\n\n
   data: {"tipo":"herramienta","contenido":"buscador"}\n\n
   data: {"tipo":"texto","contenido":"Encontré 47 archivos..."}\n\n
   data: {"tipo":"completado","sesion_id":"...","modelo":"...","tokens":1234}\n\n
   ↓
5. Goroutine procesa chunks, llama a fyne.Do() para actualizar UI en main thread:
   - tipo=estado → actualiza placeholder
   - tipo=herramienta → añade badge
   - tipo=texto → acumula en RichText
   - tipo=completado → marca mensaje completo con métricas
   ↓
6. Sidebar.SetSesionActiva() propaga la nueva sesión creada por el backend
```

### Integración con el backend

La GUI consume los endpoints ya implementados en fases anteriores:

| Endpoint | Fase | Uso en GUI |
|----------|------|------------|
| `GET /api/v1/health` | 1 | StatusDot (poll cada 30s) |
| `POST /api/v1/chat` (SSE) | 7 | Streaming de respuestas |
| `GET /api/v1/chat` | 7 | Métricas del pipeline (poll 60s) |
| `GET/POST /api/v1/chat/sesiones` | 7 | Sidebar CRUD |
| `GET/DELETE /api/v1/chat/sesiones/{id}` | 7 | Seleccionar/eliminar |
| `GET /api/v1/orquestador/modelos` | 4 | Badge de modelos disponibles |
| `GET /api/v1/contexto/proyectos` | 3 | Selector de proyecto |

### Dev workflow

```bash
# Modo desktop nativo (default) — backend + GUI en un solo proceso
make dev          # go run ./cmd/liz → servidor HTTP + ventana Fyne

# Modo servidor puro (sin GUI, para Docker/servidores)
make headless     # ./bin/liz --headless → solo HTTP en :3000
```

### Filosofía "nativo nativo"

La decisión de migrar de React+Vite (web app) a Fyne (GUI nativa) responde a:

1. **Filosofía "si no está en GitHub no existe"** — un solo binario es más fácil
   de distribuir y versionar que una app web + assets estáticos + servidor.
2. **Sin dependencia de navegador** — el usuario no necesita Chrome/Firefox.
3. **Mejor integración con el sistema** — atajos de teclado nativos, icono en
   taskbar, notificaciones del SO, arrastre de archivos.
4. **Menor superficie de ataque** — sin Chromium embebido (~200MB), sin
   vulnerabilidades JS, sin CORS.
5. **Mejor rendimiento** — OpenGL directo, sin overhead de DOM/JS engine.
6. **Identidad visual propia** — paleta morada Liz consistente en todas las
   plataformas, sin importar el tema del sistema.

### Dependencias de sistema (Linux)

La GUI nativa requiere las librerías de desarrollo de OpenGL y X11/Wayland:

```bash
# Debian/Ubuntu
sudo apt install libgl1-mesa-dev xorg-dev libxrandr-dev libxinerama-dev \
     libxcursor-dev libxi-dev libxxf86vm-dev libwayland-dev libxkbcommon-dev \
     libegl-dev libglx-dev
```

El binario resultante es ~30MB y no requiere runtime adicional (solo las
librerías dinámicas del sistema, que están en cualquier Linux desktop).

---

## 12. Roadmap — 10 Fases

| # | Fase | Issue | Qué produce | Estado |
|---|------|-------|-------------|--------|
| 1 | Núcleo Base | #9 | Binario `liz` que arranca servidor en puerto 3000 | ✅ |
| 2 | Permisos y Config | #10 | YAML config, permisos una vez, `~/.liz/` | ✅ |
| 3 | Sistema de Contexto | #11 | Mapa, fragmentos, árbol, resúmenes bajo demanda | ✅ |
| 4 | Orquestador NVIDIA | #12 | API NVIDIA conectada, 8+ modelos, selección inteligente | ✅ |
| 5 | Herramientas Base | #13 | 7 herramientas integradas funcionando | ✅ |
| 6 | Auto-Creación | #14 | Liz se programa herramientas que no tiene | ✅ |
| 7 | Pipeline de Chat | #15 | End-to-end: mensaje → modelo → herramientas → respuesta | ✅ |
| 8 | App de Escritorio Nativa | #16 | GUI Fyne nativa con streaming SSE, sin navegador | ✅ |
| 9 | Testing y Docs | #17 | Tests, seguridad, documentación completa | ✅ |
| 10 | Release v0.1.0 | #18 | Binarios, instalador, release en GitHub | ✅ |

> **Fase 10 completada** — v0.10.0 publicado. Ver [CHANGELOG.md](../CHANGELOG.md)
> y [docs/RELEASE.md](RELEASE.md) para el detalle del release.

---

## 12.bis Fase 5 — Sistema de Herramientas (Detalle)

### Visión General

La Fase 5 implementa el sistema de herramientas que da a Liz control total
sobre el sistema operativo. 7 herramientas integradas cubren todas las
operaciones necesarias: ejecutar comandos, navegar archivos, buscar,
editar, gestionar procesos, monitorear sistema e instalar software.

### Interfaz Estándar (D-002)

Toda herramienta implementa la misma interfaz Go, lo que permite:

- Contrato verificable en compile-time (`var _ Herramienta = (*X)(nil)`)
- Catálogo uniforme para herramientas integradas y auto-creadas (Fase 6)
- Testing unitario independiente por herramienta
- Validación por contrato: si no implementa la interfaz, no se registra

```go
type Herramienta interface {
    Nombre() string
    Descripcion() string
    Parametros() []Parametro
    Ejecutar(ctx context.Context, params map[string]interface{}) (Resultado, error)
    Validar() error
}
```

### Catálogo y Métricas

- `Catalogo` thread-safe (sync.RWMutex) es el registro central
- `Registrar(h)` valida nombre + Validar() antes de aceptar
- `Ejecutar(ctx, nombre, params)` mide latencia automáticamente + inyecta metadata
- `Metricas` por herramienta: exitos, fallos, tasa_exito, latencia min/max/prom
- Resumen global para dashboards

### 7 Herramientas Integradas

| # | Herramienta | Operaciones | Implementación |
|---|-------------|-------------|----------------|
| 1 | `terminal` | ejecutar | `exec.CommandContext` + timeout + detección de peligrosos |
| 2 | `navegador_archivos` | listar, stat, arbol, existe | `os.ReadDir` + `filepath.WalkDir` |
| 3 | `buscador` | archivos, contenido, combinado | `filepath.WalkDir` + `bufio.Scanner` + regex + paralelización |
| 4 | `editor` | 10 operaciones | `io/ioutil` + `regexp` + backup automático |
| 5 | `procesos` | listar, info, matar, arbol | `/proc` en Linux, `ps` fallback |
| 6 | `monitor` | completo, cpu, memoria, disco, red, uptime | `/proc/stat`, `/proc/meminfo`, `/proc/net/dev`, `statvfs(2)` |
| 7 | `instalador` | instalar, desinstalar, actualizar, buscar, info, gestores | 16 gestores soportados con autodetección |

### Endpoints API

```
GET    /api/v1/herramientas                       # listar catálogo completo
GET    /api/v1/herramientas/{nombre}              # info detallada de una herramienta
POST   /api/v1/herramientas/ejecutar              # ejecutar herramienta
GET    /api/v1/herramientas/metricas              # métricas globales
GET    /api/v1/herramientas/metricas/{nombre}     # métricas de una herramienta
```

### Flujo de Ejecución

```
POST /api/v1/herramientas/ejecutar
  body: { "nombre": "terminal", "parametros": {"comando": "ls", "args": ["-la"]} }
       │
       ▼
[Servidor] requiereCatalogo(w) → 503 si no inyectado
       │
       ▼
[Catalogo] Obtener("terminal") → Herramienta
       │
       ▼
[Catalogo] Ejecutar(ctx, "terminal", params)
       │  inicio := time.Now()
       │
       ▼
[Terminal] Ejecutar(ctx, params)
       │  - validar params (ObtenerString, ObtenerArrayString, …)
       │  - detectar comando peligroso (rm -rf /, mkfs, …)
       │  - ejecutar con context.WithTimeout
       │  - capturar stdout/stderr
       │  - truncar a 1MB
       │  - retornar Resultado{Exito, Datos, Error, Metadata}
       │
       ▼
[Catalogo] Metricas.RegistrarEjecucion("terminal", exito, duracion)
       │  - incrementa exitos o fallos
       │  - actualiza latencia min/max/prom
       │  - registra ultimo_uso, ultimo_error
       │  - inyecta metadata: duracion_ms, herramienta
       │
       ▼
[Servidor] responderJSON(200, RespuestaAPI{Exito, Datos, Metadata})
```

---

## 12.ter Fase 6 — Auto-Creación de Herramientas (Detalle)

### Visión General

La Fase 6 implementa el principio D-005 (Auto-Suficiencia): Liz **nunca dice
"no puedo"**. Si necesita una herramienta que no tiene, la crea automáticamente
usando el LLM para generar código Go, lo compila, lo valida, lo persiste y lo
registra en el catálogo — todo sin intervención humana.

### Arquitectura del Sistema

```
┌──────────────────────────────────────────────────────────────────┐
│                       Gestor (orchestrator)                       │
│   detectar → generar → compilar → cargar → registrar              │
└───────┬──────────┬──────────┬──────────┬──────────┬───────────────┘
        │          │          │          │          │
        ▼          ▼          ▼          ▼          ▼
   Detector   Generador  Compilador  Cargador   Registro
   (LLM)      (LLM)      (go build)  (subproc)  (JSON disco)
```

**6 componentes cooperantes** en `internal/nucleo/herramientas/auto_creacion/`:

1. **Detector** (`detector.go`): analiza la petición del usuario + el catálogo
   actual y usa el LLM para identificar qué herramientas faltan. Retorna una
   lista de `SpecHerramienta` (nombre, descripción, parámetros, categoría,
   razón).

2. **Generador** (`generador.go`): toma una `SpecHerramienta` y pide al LLM
   el código Go completo (package main, solo stdlib). Inyecta header con
   metadata, valida estructura mínima. Fallback `GenerarDesdePlantilla`
   produce un stub sin LLM (útil para tests y cuando no hay API key).

3. **Compilador** (`compilador.go`): escribe `fuente.go` a disco y ejecuta
   `go build -o herramienta fuente.go` con timeout configurable (default
   60s). Captura stdout+stderr combinados en `compilacion.log`.

4. **Cargador** (`cargador.go`): `HerramientaSubproceso` implementa la
   interfaz `Herramienta` delegando al binario vía JSON sobre stdin/stdout.
   Lazy-info cacheada, thread-safe, con estadísticas de uso (veces
   ejecutada, exitosas, último error, último uso).

5. **Registro** (`registro.go`): persistencia en `~/.liz/herramientas/
   auto_creadas/{nombre}/` con estructura `{fuente.go, herramienta,
   metadata.json, compilacion.log}` + índice global `registro.json`.

6. **Gestor** (`gestor.go`): orquesta el flujo completo + carga inicial +
   recarga + eliminación + prueba. Thread-safe con `sync.Mutex`.

### Protocolo Subprocess (decisión clave)

Cada herramienta auto-creada es un **binario Go standalone** (no un Go plugin)
que se comunica con Liz por JSON sobre stdin/stdout:

```
REQUEST  (Liz → herramienta):  {"operacion": "info|validar|ejecutar", "parametros": {...}}
RESPONSE (herramienta → Liz):  {"exito": true|false, "datos": <any>, "error": "", "metadata": {}}
```

**Por qué subprocess y no Go plugins:**
- Aislamiento de fallos: un panic en la herramienta no tira a Liz.
- Independencia de versión de Go: cada tool se compila sola.
- Sin problemas de module path / dependencias compartidas.
- Costo (fork+exec por llamada) aceptable para herramientas que típicamente
  hacen operaciones de sistema (ya son lentas).

### Operaciones del Gestor

| Operación | Descripción |
|-----------|-------------|
| `Crear(ctx, SolicitudCreacion)` | Flujo completo: detect→generar→compilar→cargar→registrar |
| `CargarTodas()` | Escanea el registro y carga todas las tools en el catálogo (al iniciar Liz) |
| `Recargar(ctx, nombre, nuevoFuente, usarLLM)` | Recompila desde fuente (existente, manual o regenerada por LLM) |
| `Eliminar(nombre)` | Quita del registro + catálogo + limpia artifacts en disco |
| `Probar(ctx, nombre, params)` | Ejecuta con params arbitrarios, actualiza estadísticas |
| `Listar()` | Metadata de todas las tools |
| `Obtener(nombre)` | Metadata de una tool |
| `LeerFuente(nombre)` | Código fuente Go |
| `LeerLogCompilacion(nombre)` | Log de la última compilación |

### Endpoints API (9 endpoints)

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

### Persistencia entre sesiones

Al iniciar Liz, el `Gestor.CargarTodas()` escanea el registro y carga todas
las herramientas auto-creadas en el catálogo. Si una tool no compila o falla
al cargar, se marca en su metadata pero no aborta el arranque — las demás
se cargan normalmente.

### Modos de operación

| Modo | LLM | Cuándo se usa |
|------|-----|---------------|
| **Completo** | ✅ | Flujo normal: detector + generador usan LLM → herramientas reales |
| **Forzado** | ✅/❌ | Caller pasa `forzar_spec` o `forzar_nombre` → salta detector |
| **Fallback stub** | ❌ | Sin LLM: stub compilable que responde info/validar pero en ejecutar retorna "no implementado" |

### Seguridad

- El código generado se compila y ejecuta con los permisos del usuario.
- `Validar()` hace una prueba controlada (op="validar") sin side-effects.
- El `Cargador` captura panics del subprocess (exit code != 0) y los
  convierte en `Resultado.Exito=false` con stderr como Error.
- El timeout del context se transmite al subprocess (SIGKILL tras expirar).
- Solo se permite código con stdlib (sin imports externos) → minimiza
  superficie de ataque y garantiza `go build fuente.go` sin go.mod.

### Tests (32 tests)

- **18 unitarios**: tipos, plantillas, detector con mock LLM, generador con
  mock LLM + stub fallback, parsing robusto de JSON.
- **14 de integración**: compilador real con `go build`, cargador subprocess
  end-to-end (compile + info + validar + ejecutar), registro persistencia
  (guardar/obtener/listar/eliminar/estadísticas), gestor flujo completo
  (crear/cargar-todas/eliminar/probar/recargar con y sin nuevo fuente).

Todos los tests pasan con `go test -race ./...` en los 22 paquetes del proyecto.

---

## 13. Comparación con la Competencia

| Aspecto | Claude Code | Codex | Liz |
|---------|-------------|-------|-----|
| Modelo | Solo Claude | Solo GPT | 8+ modelos (elige el mejor) |
| Contexto | Carga todo | Carga todo | Mapa bajo demanda |
| Herramientas | Set fijo | Set fijo | Fijas + auto-creadas (∞) |
| Permisos | Pide cada vez | Pide cada vez | Una vez al iniciar |
| Control sistema | Solo código | Solo código | Control TOTAL de Linux |
| Costo | Suscripción cara | Suscripción cara | Paga lo que usas (API) |
| Código abierto | ❌ | ❌ | ✅ |
| Personalizable | ❌ | ❌ | ✅ 100% |
| Auto-mejora | ❌ | ❌ | ✅ Crea sus propias herramientas |

---

## 14. Notas Importantes para Desarrolladores

1. **NUNCA exponer API keys** en el código. Siempre en `liz.yaml` o variables de entorno.
2. **NUNCA decir "no puedo"**. Si falta una herramienta, programarla.
3. **SIEMPRE actualizar el contexto** después de cada interacción.
4. **SIEMPRE elegir el modelo más eficiente** para la tarea, no el más potente.
5. **SIEMPRE mantener el código compilable**. Las herramientas auto-creadas DEBEN compilar.
6. Los fragmentos de contexto son **inmutables** (nunca se editan, solo se agregan nuevos).
7. El índice del contexto se reconstruye **incrementalmente**, no desde cero.
8. Las métricas del orquestador sirven para **mejorar selecciones futuras** de modelos.

---

## 15. Fase 10 — Arquitectura de Release y Distribución

### 15.1 Build tag `headless` — dual-mode compilation

Para habilitar cross-compilation y Docker ligero sin refactorizar el paquete
`internal/desktop` (Fase 8), se introdujo un build tag en `cmd/liz/`:

```
cmd/liz/
├── main.go                 # Entry point común (sin importar desktop)
├── desktop_desktop.go      # //go:build !headless — importa internal/desktop, GUI Fyne
└── desktop_headless.go     # //go:build headless    — stub, solo servidor HTTP
```

**Compilación default** (sin tag):
```bash
go build -o liz ./cmd/liz
```
- Importa `internal/desktop` → arrastra Fyne v2 + OpenGL + GLFW + Wayland
- Requiere CGO y librerías dev (libGL, libX11, libwayland, etc.)
- Binario ~30MB, solo Linux x86_64 (CGO nativo)
- Funciona en modo desktop (con GUI) o `--headless` (sin GUI)

**Compilación headless** (con tag):
```bash
CGO_ENABLED=0 go build -tags headless -o liz-server ./cmd/liz
```
- NO importa `internal/desktop` → sin Fyne, sin OpenGL, sin CGO
- Binario 100% estático (~7MB), cross-compilable a cualquier GOOS/GOARCH
- Solo modo servidor (`--headless` forzado)
- Ideal para: Docker, servidores, CI, macOS, ARM64, Windows (futuro)

### 15.2 Pipeline de publicación (CI/CD)

```
git tag v0.10.0 && git push origin v0.10.0
                       │
                       ▼
            ┌──────────────────────┐
            │ GitHub Actions:      │
            │ release.yml workflow │
            └──────────┬───────────┘
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
┌───────────────┐ ┌──────────┐ ┌───────────────┐
│ Job 1:        │ │ Job 2:   │ │ Job 3:        │
│ build-binaries│ │ package  │ │ docker        │
│ (matrix 5)    │ │          │ │ (multi-arch)  │
└───────┬───────┘ └────┬─────┘ └───────┬───────┘
        │              │               │
        └──────────────┼───────────────┘
                       ▼
            ┌──────────────────────┐
            │ Job 4: release       │
            │ GitHub Release +     │
            │ assets + notas       │
            └──────────────────────┘
```

**Job 1 (build-binaries)** — Matrix strategy, 5 builds paralelos:
- `desktop-linux-amd64`: CGO=1, instala deps OpenGL, binario con GUI
- `headless-linux-amd64`: CGO=0, estático
- `headless-linux-arm64`: cross-compile a ARM64
- `headless-darwin-amd64`: cross-compile a macOS Intel
- `headless-darwin-arm64`: cross-compile a macOS Apple Silicon

**Job 2 (package)** — Crea paquetes nativos:
- Tarballs: `<binario>-v<ver>.tar.gz` con binario + config + README + LICENSE + scripts
- Checksums: `checksums-v<ver>.txt` con SHA-256
- DEB: con `dpkg-deb`, control file, postinst, prerm, .desktop entry
- RPM: con `rpmbuild`, spec file completo

**Job 3 (docker)** — Imagen multi-arch:
- Multi-stage: `golang:1.22-alpine` → `distroless/static-debian12:nonroot`
- Plataformas: `linux/amd64, linux/arm64` vía buildx + QEMU
- Tags: `v<ver>`, `latest`, `sha-<commit>`
- Publica en `ghcr.io/caos1codex-hash/liz-ai-agent`

**Job 4 (release)** — Publica GitHub Release:
- Descarga todos los artifacts
- Genera notas con tabla de plataformas + instrucciones
- `softprops/action-gh-release@v2` crea el release
- Sube todos los assets (binarios, tarballs, DEB, RPM, checksums)

### 15.3 Distribución de binarios

| Plataforma | Binario | Modo | Distribución |
|------------|---------|------|--------------|
| Linux x86_64 | `liz-linux-amd64` (~30MB) | Desktop con GUI | GitHub Release + DEB + RPM + tarball |
| Linux x86_64 | `liz-server-linux-amd64` (~7MB) | Headless | GitHub Release + tarball + Docker |
| Linux ARM64 | `liz-server-linux-arm64` (~7MB) | Headless | GitHub Release + tarball + Docker |
| macOS Intel | `liz-server-darwin-amd64` (~8MB) | Headless | GitHub Release + tarball |
| macOS Apple Silicon | `liz-server-darwin-arm64` (~8MB) | Headless | GitHub Release + tarball |

### 15.4 Instalador multi-distro

`scripts/install.sh` detecta automáticamente:

1. **Plataforma** (`uname -s`): Linux o macOS
2. **Arquitectura** (`uname -m`): amd64 o arm64
3. **Distro** (`/etc/os-release`): debian, fedora, arch, opensuse

Y ejecuta:
1. Instala dependencias del sistema según distro (apt/dnf/pacman/zypper)
2. Descarga binario del release de GitHub correspondiente
3. Verifica que arranca con `--version`
4. Instala en `/usr/local/bin/` (con sudo si es necesario)
5. Crea `~/.liz/` y copia config de ejemplo
6. Crea entrada de menú `.desktop` (Linux desktop)
7. Muestra banner ASCII art al finalizar

### 15.5 Imagen Docker

Multi-stage build optimizado:

```
Stage 1: golang:1.22-alpine (~400MB)
  ├── go mod download (cached layer)
  ├── COPY . .
  └── CGO_ENABLED=0 go build -tags headless → /out/liz-server

Stage 2: gcr.io/distroless/static-debian12:nonroot (~2MB)
  ├── COPY --from=builder /out/liz-server
  ├── VOLUME /home/liz/.liz
  ├── EXPOSE 3000
  ├── HEALTHCHECK
  └── ENTRYPOINT ["/usr/local/bin/liz-server"] CMD ["--headless"]
```

Imagen final: ~10MB, sin shell, sin package manager, sin libs innecesarias.
Usuario `nonroot` (UID 65532). `security_opt: no-new-privileges:true`.

### 15.6 Filosofía de release

> **"Si no está en GitHub, no existe."**

1. **Todo cambio se commitea y pushea lo antes posible** — no hay trabajo
   "local" sin respaldo. Si se pierde el entorno, el código sigue en GitHub.
2. **Todo release se taggea** — `git tag -a vX.Y.Z -m "..."` + `git push origin vX.Y.Z`.
   El tag dispara el workflow de release que publica todo automáticamente.
3. **Todo asset es verificable** — checksums SHA-256 de cada tarball.
4. **Todo está documentado** — CHANGELOG.md, docs/INSTALACION.md, docs/RELEASE.md,
   docs/ARQUITECTURA.md (este archivo).
5. **Todo es reproducible** — el workflow de release se puede re-ejecutar sobre
   cualquier tag y producir binarios idénticos (gracias a `-trimpath` y Go modules).

---

*Este documento es vivo. Se actualiza conforme avanza el desarrollo.*
*Última actualización: 2026-07-26 — Fase 10: Release v0.1.0 completado*
