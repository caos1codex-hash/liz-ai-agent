// Package auto_creacion implementa la Fase 6 de Liz: el sistema mediante el cual
// Liz se programa a sí misma herramientas que no tiene.
//
// ============== VISIÓN GENERAL ==============
//
// Liz parte con 7 herramientas integradas (Fase 5). Cuando el usuario pide algo
// que requiere una capacidad que no está en el catálogo, la Fase 6 entra en
// acción: detecta la necesidad, genera código Go que implementa la interfaz
// `herramientas.Herramienta`, lo compila, lo valida, lo persiste y lo registra
// en el catálogo — todo automáticamente, sin intervención humana.
//
// Tras la Fase 6, Liz "nunca dice no puedo": si le falta una herramienta, la crea.
//
// ============== ARQUITECTURA ==============
//
// El sistema se compone de 6 piezas cooperantes:
//
//		┌─────────────────────────────────────────────────────────────────┐
//		│                       Gestor (orchestrator)                     │
//		│   detectar → generar → compilar → cargar → registrar            │
//		└───────┬──────────┬──────────┬──────────┬──────────┬─────────────┘
//		        │          │          │          │          │
//		        ▼          ▼          ▼          ▼          ▼
//		   Detector   Generador  Compilador  Cargador   Registro
//		   (LLM)      (LLM)      (go build)  (subproc)  (JSON en disco)
//
//	 1. DETECTOR (detector.go)
//	    Recibe la solicitud natural del usuario + el catálogo actual.
//	    Usa el Orquestador NVIDIA para preguntar al LLM:
//	    "Dada esta petición y estas herramientas existentes, ¿qué herramientas
//	    NUEVAS se necesitan? Devuelve JSON con nombre, descripción, params."
//	    Retorna una lista de specs de herramientas faltantes.
//
//	 2. GENERADOR (generador.go)
//	    Recibe la spec de una herramienta (nombre, descripción, parámetros).
//	    Usa el Orquestador para pedir al LLM el código Go completo.
//	    Retorna el fuente como string.
//
//	 3. COMPILADOR (compilador.go)
//	    Escribe el fuente a disco y ejecuta `go build -o <binario> <fuente>`.
//	    Captura stdout/stderr. Si falla, retorna error con logs.
//
//	 4. CARGADOR (cargador.go)
//	    Expone un `HerramientaSubproceso` que implementa `herramientas.Herramienta`.
//	    Cada llamada a Ejecutar() lanza el binario como subprocess, le envía JSON
//	    por stdin y lee JSON de stdout. Soporta timeout vía context.
//
//	 5. REGISTRO (registro.go)
//	    Persiste en `~/.liz/herramientas/auto_creadas/`:
//	    {nombre}/fuente.go        — código fuente
//	    {nombre}/herramienta      — binario compilado
//	    {nombre}/metadata.json    — descripción, params, timestamps
//	    {nombre}/compilacion.log  — log de la última compilación
//	    Mantiene también un índice `registro.json` con todas las tools.
//
//	 6. GESTOR (gestor.go)
//	    Orquesta el flujo completo: detectar → generar → compilar → cargar → registrar.
//	    Carga todas las herramientas auto-creadas al iniciar Liz.
//
// ============== PROTOCOLO SUBPROCESS ==============
//
// Cada herramienta auto-creada es un binario Go standalone (package main) que
// se comunica con Liz por JSON sobre stdin/stdout. Esto es más robusto que
// Go plugins (que requieren exactamente la misma versión de Go, mismas
// dependencias y mismo module path). El protocolo:
//
//	REQUEST (Liz → herramienta, una línea JSON por stdin):
//	  {
//	    "operacion": "info" | "validar" | "ejecutar",
//	    "parametros": { ... },          // solo para "ejecutar"
//	    "timeout_ms": 5000              // opcional
//	  }
//
//	RESPONSE (herramienta → Liz, una línea JSON por stdout):
//	  {
//	    "exito": true|false,
//	    "datos":  <any json>,           // payload específico
//	    "error":  "mensaje legible",    // vacío si exito=true
//	    "metadata": { ... }             // opcional
//	  }
//
//	Operaciones:
//	  - "info"    → datos = {nombre, descripcion, parametros}
//	  - "validar" → datos = {ok: true} si la herramienta está operativa
//	  - "ejecutar" → datos = resultado específico de la herramienta
//
// ============== DECISIONES DE DISEÑO ==============
//
//   - Subprocess vs Go plugin: subprocess. Razones:
//
//   - Aislamiento de fallos (un panic no tira a Liz)
//
//   - Independencia de versión de Go (cada tool se compila sola)
//
//   - Sin problemas de module path / dependencias compartidas
//
//   - El costo (fork+exec por llamada) es aceptable para herramientas
//     que típicamente hacen operaciones de sistema (ya son lentas)
//
//   - Solo stdlib en el código generado: el LLM está instruido de usar
//     únicamente paquetes de la stdlib (`os`, `net/http`, `encoding/json`,
//     `strconv`, etc.). Esto garantiza que `go build fuente.go` funcione
//     sin go.mod ni dependencias externas.
//
//   - Persistencia en ~/.liz: consistente con el resto del estado de Liz
//     (permisos, contexto, memoria). Inspeccionable, backup fácil.
//
//   - Hot-reload: el Gestor permite recompilar desde fuente sin reiniciar Liz.
//
// ============== SEGURIDAD ==============
//
//   - El código generado se compila y ejecuta con los permisos del usuario.
//   - Validar() debe hacer una prueba rápida sin side-effects.
//   - El Cargador captura panics del subprocess (exit code != 0) y los
//     convierte en Resultado.Exito=false con el stderr como Error.
//   - El timeout del context se transmite al subprocess vía SIGKILL tras
//     expirar (os/exec lo maneja con cmd.Cancel / WaitDelay en Go 1.20+).
//
// Referencias:
//   - docs/ARQUITECTURA.md sección 10 (Flujo de Auto-Creación)
//   - docs/DECISIONES.md D-005 (Auto-Creación de Herramientas)
//   - internal/nucleo/herramientas/interface.go (interfaz Herramienta)
package auto_creacion
