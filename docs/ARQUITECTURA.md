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
| **Backend** | **Go** | Estabilidad máxima, binario estático (no dependencias), concurrency nativa (goroutines), perfecto para gestión de procesos del sistema, Docker/Kubernetes/Terraform están hechos en Go |
| **Frontend** | **React + TypeScript + Vite** | Rico ecosistema, tipado seguro, build ultrarrápido, componentes reutilizables |
| **Estilos** | **Tailwind CSS** | Utilidad-first, desarrollo rápido, tema oscuro fácil |
| **Streaming** | **Server-Sent Events (SSE)** | Más simple que WebSocket, perfecto para respuestas unidireccionales, soporte nativo en browsers |
| **Configuración** | **YAML** | Legible, editable a mano, soporta estructuras complejas |
| **IA** | **API NVIDIA** | 8+ modelos disponibles, orquestación flexible, endpoint estándar OpenAI-compatible |
| **Tests** | **Go testing + Playwright** | Go testing para backend, Playwright para E2E del frontend |
| **Fontend Build** | **Vite** | HMR ultrarrápido, tree-shaking agresivo, configuración mínima |

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

## 11. Diseño del Frontend

Interfaz estilo ChatGPT clásico:

```
┌──────────────────────────────────────────────────────┐
│  🟣 Liz AI                          ⚙️ Model: Auto  🌙│
├──────────────┬───────────────────────────────────────┤
│              │                                       │
│ Conversaciones│  🧠 Mixtral 8x22B  │  🔧 Buscador    │
│ ─────────────│                                       │
│ ▸ Proyecto Web│  Hola, soy Liz. ¿En qué puedo        │
│   Setup DB   │  ayudarte hoy?                        │
│ ▸ Debug Server│                                      │
│   Config     │  ──────────────────────────────────  │
│   Nginx      │  👤 Elimina los .log de más de 30    │
│              │     días en /var/log                  │
│ ─────────────│                                      │
│ + Nueva       │  🤖 Encontré 47 archivos .log...     │
│              │     Eliminando...                     │
│              │     ✅ 47 archivos eliminados           │
│              │                                      │
│              │  ──────────────────────────────────  │
│              │  [ Escribe un mensaje... ]       ➤    │
└──────────────┴───────────────────────────────────────┘
```

Características del frontend:
- Sidebar con historial de conversaciones
- Streaming de respuestas (texto aparece gradualmente)
- Markdown renderizado (tablas, listas, código)
- Bloques de código con syntax highlighting y botón copiar
- Indicador del modelo usado (badge)
- Indicador de herramientas usadas durante ejecución
- Animación "Liz está pensando..."
- Tema oscuro por defecto, claro alternativo
- Responsive (desktop + tablet)

---

## 12. Roadmap — 10 Fases

| # | Fase | Issue | Qué produce |
|---|------|-------|-------------|
| 1 | Núcleo Base | #9 | Binario `liz` que arranca servidor en puerto 3000 |
| 2 | Permisos y Config | #10 | YAML config, permisos una vez, `~/.liz/` |
| 3 | Sistema de Contexto | #11 | Mapa, fragmentos, árbol, resúmenes bajo demanda |
| 4 | Orquestador NVIDIA | #12 | API NVIDIA conectada, 8+ modelos, selección inteligente |
| 5 | Herramientas Base | #13 | 7 herramientas integradas funcionando |
| 6 | Auto-Creación | #14 | Liz se programa herramientas que no tiene |
| 7 | Pipeline de Chat | #15 | End-to-end: mensaje → modelo → herramientas → respuesta |
| 8 | Frontend | #16 | Interfaz ChatGPT clásico con streaming |
| 9 | Testing y Docs | #17 | Tests, seguridad, documentación completa |
| 10 | Release v0.1.0 | #18 | Binarios, instalador, release en GitHub |

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

*Este documento es vivo. Se actualiza conforme avanza el desarrollo.*
*Última actualización: 2026-07-25*
