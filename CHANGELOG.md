# Changelog — Liz AI Agent

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