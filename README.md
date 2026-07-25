# Liz — AI Agent para Linux

> Agente de IA autonomo que controla completamente tu Linux mediante lenguaje natural.
> No es un chatbot. No es un asistente de codigo. Es un sistema operativo de IA.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8)
![Phase](https://img.shields.io/badge/fase-1%20de%2010-orange)

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

| # | Fase | Issue |
|---|------|-------|
| 1 | Nucleo Base | [#9](https://github.com/caos1codex-hash/liz-ai-agent/issues/9) |
| 2 | Permisos y Config | [#10](https://github.com/caos1codex-hash/liz-ai-agent/issues/10) |
| 3 | Sistema de Contexto | [#11](https://github.com/caos1codex-hash/liz-ai-agent/issues/11) |
| 4 | Orquestador NVIDIA | [#12](https://github.com/caos1codex-hash/liz-ai-agent/issues/12) |
| 5 | Herramientas Base | [#13](https://github.com/caos1codex-hash/liz-ai-agent/issues/13) |
| 6 | Auto-Creacion | [#14](https://github.com/caos1codex-hash/liz-ai-agent/issues/14) |
| 7 | Pipeline de Chat | [#15](https://github.com/caos1codex-hash/liz-ai-agent/issues/15) |
| 8 | Frontend | [#16](https://github.com/caos1codex-hash/liz-ai-agent/issues/16) |
| 9 | Testing y Docs | [#17](https://github.com/caos1codex-hash/liz-ai-agent/issues/17) |
| 10 | Release v0.1.0 | [#18](https://github.com/caos1codex-hash/liz-ai-agent/issues/18) |

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
