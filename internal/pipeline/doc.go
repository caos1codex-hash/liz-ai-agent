// Package pipeline implementa el Pipeline de Chat de Liz (Fase 7).
//
// El pipeline conecta todos los subsistemas existentes en un flujo coherente
// end-to-end: mensaje del usuario → clasificación de intención → planificación
// de tareas → ejecución de herramientas → generación de respuesta con streaming.
//
// # Arquitectura del Pipeline
//
// ┌─────────────────────────────────────────────────────────────────┐
// │                      Pipeline (coordinador)                      │
// └────────┬──────────┬──────────┬──────────┬──────────┬─────────────┘
//          │          │          │          │          │
//          ▼          ▼          ▼          ▼          ▼
//     Receptor  Clasificador Planificador Ejecutor  Respondedor
//     (input)   (intención)   (pasos)    (tools)   (LLM+stream)
//          │          │          │          │          │
//          ▼          ▼          ▼          ▼          ▼
//       Memoria   Orquestador  Orquestador Catalogo  Orquestador
//                (clasificar)  (planificar)           (completar)
//                                           │
//                                           ▼
//                                   Auto-Creación
//                                   (si falta tool)
//
// # Flujo Completo
//
// 1. Receptor: valida entrada, crea/retoma sesión, almacena mensaje
// 2. Clasificador: determina tipo de tarea (conversación, código, sistema, etc.)
// 3. Planificador: descompone en pasos, selecciona herramientas necesarias
// 4. Ejecutor: ejecuta cada paso (con auto-creación si falta herramienta)
// 5. Respondedor: construye prompt final con contexto + resultados → LLM → stream
//
// # Integración con Subsistemas
//
//   - Memoria (Fase 3.5): sesiones, mensajes, hechos, contexto para LLM
//   - Orquestador (Fase 4): selección de modelo, LLM calls, streaming SSE
//   - Herramientas (Fase 5): catálogo de 7+ herramientas integradas
//   - Auto-Creación (Fase 6): creación automática de herramientas faltantes
//   - Contexto (Fase 3): empaquetamiento de contexto de código
//
// # Modos de Respuesta
//
//   - JSON: respuesta completa en un solo JSON response
//   - SSE: respuesta progresiva via Server-Sent Events (streaming)
//
// # Endpoints API
//
//   POST /api/v1/chat           — Enviar mensaje (JSON o SSE)
//   GET  /api/v1/chat           — Estado del pipeline + estadísticas
//   GET  /api/v1/chat/sesiones  — Listar sesiones de chat activas
package pipeline
