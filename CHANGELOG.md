# Changelog

Todos los cambios notables en este proyecto se documentan en este archivo.

El formato se basa en [Keep a Changelog](https://keepachangelog.com/).

---

## [0.1.0] — Pendiente

### Agregado
- Planificacion completa de 10 fases de desarrollo
- Arquitectura definida y documentada
- Decisiones de diseño documentadas (12 decisiones)
- Stack tecnologico definido: Go + React + NVIDIA API
- Estructura del proyecto definida
- Sistema de contexto inteligente disenado
- Sistema de auto-creacion de herramientas disenado
- Orquestador multi-modelo disenado
- Interfaz de herramientas estandar (Go interface)

### Fase 1 — Nucleo Base (completada)
- **cmd/liz/main.go** — Punto de entrada del binario con banner ASCII y graceful shutdown
- **internal/nucleo/logger/** — Logging estructurado en JSON con colores, niveles filtrables, escritura a ~/.liz/logs/liz.log
- **internal/nucleo/config/** — Lectura de configuracion YAML con soporte para wrapper `liz:`, valores por defecto, overrides por env vars (LIZ_PUERTO, NVIDIA_API_KEY, LIZ_HOST), expansion de ~, y creacion automatica de ~/.liz/
- **internal/nucleo/permisos/** — Sistema de permisos "una vez" (D-006): 6 permisos predefinidos, concesion individual o total, persistencia en ~/.liz/permisos.json, verificacion en tiempo real
- **internal/nucleo/servidor/** — Servidor HTTP con gorilla/mux, 14 endpoints de API (health, config, permisos, tools, orquestador, conversations, chat), middleware CORS, logging, y recovery de panics
- **Makefile** — Targets: build, run, dev, test, vet, fmt, lint, clean, install, help
- **19 tests unitarios** pasando (logger: 5, config: 4, permisos: 8, servidor: 8)
- Endpoints stub preparados para fases futuras (tools → Fase 5, orquestador → Fase 4, chat → Fase 7)

### Fase 2 — Permisos y Configuración (completada)
- **config.Gestor** — Gestor thread-safe con validación completa y persistencia a ~/.liz/config.json
- **Validación** — Puerto (1-65535), host no vacío, tema (oscuro/claro/auto), endpoint HTTPS, modelo ID/nombre/tipo/velocidad
- **Persistencia PUT /api/config** — Cambios se guardan y se cargan en el próximo inicio (merge: YAML + overrides + env vars)
- **Middleware de permisos** — Rutas protegidas retornan 403 sin permisos; `/api/chat` requiere `terminal`, `/api/tools` requiere `sistema`, `/api/conversations` requiere `archivos`
- **DELETE /api/permisos** — Revoca todos los permisos
- **GET /api/permisos/auditoria** — Log de verificaciones de permisos con paginación
- **Revocar permisos individuales** — Método `Revocar(nombre)`
- **recordar_entre_sesiones** — Si false (default), permisos se limpian al iniciar
- **~/.liz/contexto/sistema/estado/sesion_actual.json** — Estado de sesión (ID, PID, versión, config origen)
- **~/.liz/contexto/sistema/estado/herramientas_registradas.json** — Registro de herramientas
- **Health mejorado** — Incluye `config_origen` y `permisos_listos`
- **49 tests unitarios** pasando (logger: 5, config: 14, permisos: 18, servidor: 12)

### Documentacion
- docs/ARQUITECTURA.md — Documentacion completa del proyecto
- docs/DECISIONES.md — Registro de 12 decisiones de diseno
- README.md — Descripcion, arquitectura, roadmap
- CONTRIBUTING.md — Guia de contribuciones
- configs/liz.yaml.example — Configuracion de ejemplo
- .gitignore — Archivos excluidos del repo

---

## Fases (Roadmap)

| Fase | Nombre | Estado |
|------|--------|--------|
| 1 | Nucleo Base | Completada |
| 2 | Permisos y Configuracion | Completada |
| 3 | Sistema de Contexto | Pendiente |
| 4 | Orquestador NVIDIA | Pendiente |
| 5 | Herramientas Base | Pendiente |
| 6 | Auto-Creacion de Herramientas | Pendiente |
| 7 | Pipeline de Chat | Pendiente |
| 8 | Frontend | Pendiente |
| 9 | Testing y Documentacion | Pendiente |
| 10 | Release v0.1.0 | Pendiente |

---

*Proyecto iniciado el 2026-07-25*