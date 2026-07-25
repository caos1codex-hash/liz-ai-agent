// Package memoria implementa el sistema de memoria conversacional de Liz.
//
// A diferencia del paquete `contexto` (que memoriza CÓDIGO), este paquete
// memoriza CONVERSACIONES, USUARIOS y HECHOS. Inspirado en:
//
//   - Mem0: extracción de hechos del diálogo + resolución de conflictos
//   - Letta (MemGPT): memoria episódica + core memory + archival
//   - Zep: continuidad entre sesiones + grafo de hechos temporal
//   - LangGraph: checkpoints de estado del agente
//
// Jerarquía de memoria:
//
//   1. SESIONES — una sesión por conversación (uuid, usuario, timestamps)
//      Persistencia: ~/.liz/memoria/sesiones/<uuid>.json
//
//   2. MENSAJES — cada turno de chat (rol, contenido, timestamp, metadata)
//      Persistencia: dentro del archivo de sesión
//
//   3. HECHOS — tripletas (sujeto, predicado, objeto) extraídas del diálogo
//      con confianza y timestamp. Resolución de conflictos: si un hecho nuevo
//      contradice uno viejo (mismo sujeto+predicado), el viejo se marca como
//      "obsoleto" y el nuevo lo reemplaza.
//      Persistencia: ~/.liz/memoria/hechos/<usuario>.json
//
//   4. RESÚMENES — resúmenes consolidados de sesiones cerradas
//      (cuando una sesión termina, se genera un resumen que alimenta futuras sesiones)
//      Persistencia: ~/.liz/memoria/resumenes/<usuario>.json
//
// Acceso:
//
//   - Sesión actual: en memoria + persistida al cierre de cada turno
//   - Últimos N turnos: buffer circular (recall memory estilo Letta)
//   - Hechos de un usuario: cargados al iniciar sesión, modificados incrementalmente
//   - Búsqueda sobre hechos: reutiliza buscador.Buscador (BM25 + RRF)
//
// Thread-safety: todos los tipos son thread-safe (sync.RWMutex).
package memoria
