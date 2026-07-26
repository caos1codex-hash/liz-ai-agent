package auto_creacion

import (
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Tipos compartidos del sistema de auto-creación
// ============================================================================

// SpecHerramienta describe una herramienta que Liz necesita crear.
//
// Es el contrato entre el Detector (que identifica qué falta) y el Generador
// (que produce el código Go). Se persiste en metadata.json como fuente de
// verdad sobre la herramienta auto-creada.
type SpecHerramienta struct {
	// Nombre es el identificador único de la herramienta (snake_case, 2-64 chars).
	// Debe cumplir herramientas.ValidarNombre.
	Nombre string `json:"nombre"`

	// Descripcion legible de qué hace la herramienta. Se muestra en el catálogo.
	Descripcion string `json:"descripcion"`

	// Parametros que acepta la herramienta. Sigue el mismo formato que
	// herramientas.Parametro para que el catálogo los exponga uniformemente.
	Parametros []herramientas.Parametro `json:"parametros,omitempty"`

	// Razon explica por qué se necesita esta herramienta (la da el LLM detector).
	// Se guarda en metadata para trazabilidad.
	Razon string `json:"razon,omitempty"`

	// Categoria es un hint opcional del LLM: "archivos", "red", "sistema",
	// "datos", "codigo", "otro". Se usa para organizar el catálogo.
	Categoria string `json:"categoria,omitempty"`
}

// MetadataHerramienta es la información persistente de una herramienta
// auto-creada. Se guarda en `~/.liz/herramientas/auto_creadas/{nombre}/metadata.json`.
type MetadataHerramienta struct {
	SpecHerramienta `json:"spec"`

	// CreadoEn timestamp de la primera creación.
	CreadoEn time.Time `json:"creado_en"`

	// ActualizadoEn timestamp de la última recompilación.
	ActualizadoEn time.Time `json:"actualizado_en"`

	// ModeloGenerador es el ID del modelo NVIDIA que generó el código.
	ModeloGenerador string `json:"modelo_generador,omitempty"`

	// ModeloDetector es el ID del modelo NVIDIA que detectó la necesidad.
	ModeloDetector string `json:"modelo_detector,omitempty"`

	// VersionContador se incrementa en cada recompilación.
	VersionContador int `json:"version_contador"`

	// Compila indica si la última compilación fue exitosa.
	Compila bool `json:"compila"`

	// UltimoError de compilación o ejecución (vacío si todo OK).
	UltimoError string `json:"ultimo_error,omitempty"`

	// VecesEjecutada contador de invocaciones (se actualiza desde el Cargador).
	VecesEjecutada int `json:"veces_ejecutada"`

	// VecesExitosas contador de ejecuciones exitosas.
	VecesExitosas int `json:"veces_exitosas"`

	// FuenteHash SHA-256 del fuente.go (para detectar cambios manuales).
	FuenteHash string `json:"fuente_hash,omitempty"`
}

// SolicitudCreacion es el input del flujo completo de auto-creación.
type SolicitudCreacion struct {
	// Descripcion en lenguaje natural de lo que el usuario quiere lograr.
	// Ej: "Comprime todos los .csv de /home/user/data y envíalos por SFTP".
	Descripcion string `json:"descripcion"`

	// CatalogoActual es la lista de nombres+descripciones de herramientas ya
	// disponibles. El Detector la usa para no sugerir duplicados.
	CatalogoActual []InfoCatalogo `json:"catalogo_actual,omitempty"`

	// ForzarNombre permite al caller especificar el nombre de la herramienta
	// a crear (omits Detector, va directo al Generador). Útil para tests.
	ForzarNombre string `json:"forzar_nombre,omitempty"`

	// ForzarSpec permite al caller pasar la spec completa y saltarse Detector.
	// Si está seteado, ForzarNombre se ignora.
	ForzarSpec *SpecHerramienta `json:"forzar_spec,omitempty"`
}

// InfoCatalogo es la vista mínima de una herramienta del catálogo que se
// pasa al Detector para que sepa qué ya existe.
type InfoCatalogo struct {
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
}

// ResultadoDeteccion es lo que retorna el Detector.
type ResultadoDeteccion struct {
	// Faltantes es la lista de herramientas que el LLM considera necesarias.
	Faltantes []SpecHerramienta `json:"faltantes"`

	// Razon general del LLM sobre el análisis.
	Razon string `json:"razon,omitempty"`

	// ModeloUsado ID del modelo NVIDIA que hizo la detección.
	ModeloUsado string `json:"modelo_usado,omitempty"`
}

// ResultadoGeneracion es lo que retorna el Generador.
type ResultadoGeneracion struct {
	// FuenteGo es el código Go completo (package main, listo para compilar).
	FuenteGo string `json:"fuente_go"`

	// ModeloUsado ID del modelo que generó el código.
	ModeloUsado string `json:"modelo_usado,omitempty"`

	// TokensConsumidos total (prompt + completion).
	TokensConsumidos int `json:"tokens_consumidos,omitempty"`
}

// ResultadoCompilacion es lo que retorna el Compilador.
type ResultadoCompilacion struct {
	// Exito indica si `go build` terminó con código 0.
	Exito bool `json:"exito"`

	// RutaBinario path absoluto al binario compilado (vacío si falló).
	RutaBinario string `json:"ruta_binario,omitempty"`

	// RutaFuente path absoluto al fuente.go.
	RutaFuente string `json:"ruta_fuente"`

	// Log salida completa de go build (stdout + stderr combinados).
	Log string `json:"log,omitempty"`

	// Duracion de la compilación.
	Duracion time.Duration `json:"duracion,omitempty"`
}

// ResultadoCreacion es el resultado del flujo completo (Gestor.Crear).
type ResultadoCreacion struct {
	// Especificacion final de la herramienta creada.
	Especificacion SpecHerramienta `json:"especificacion"`

	// Deteccion información del Detector (vacía si se forzó spec).
	Deteccion *ResultadoDeteccion `json:"deteccion,omitempty"`

	// Generacion información del Generador.
	Generacion *ResultadoGeneracion `json:"generacion,omitempty"`

	// Compilacion información del Compilador.
	Compilacion *ResultadoCompilacion `json:"compilacion"`

	// CargaExitosa indica si el Cargador pudo invocar el binario y obtener
	// la info correctamente (operacion="info").
	CargaExitosa bool `json:"carga_exitosa"`

	// Registrada indica si la herramienta quedó registrada en el catálogo.
	Registrada bool `json:"registrada"`

	// Metadata final persistida.
	Metadata *MetadataHerramienta `json:"metadata,omitempty"`

	// Error de cualquier etapa (vacío si todo OK).
	Error string `json:"error,omitempty"`
}

// ============================================================================
// Protocolo subprocess (Liz ↔ herramienta auto-creada)
// ============================================================================

// SolicitudSubprocess es el JSON que Liz envía por stdin al binario.
type SolicitudSubprocess struct {
	Operacion  string                 `json:"operacion"` // "info" | "validar" | "ejecutar"
	Parametros map[string]interface{} `json:"parametros,omitempty"`
	TimeoutMS  int                    `json:"timeout_ms,omitempty"`
}

// RespuestaSubprocess es el JSON que el binario devuelve por stdout.
type RespuestaSubprocess struct {
	Exito    bool                   `json:"exito"`
	Datos    interface{}            `json:"datos,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DatosInfo es el payload de la operacion="info".
type DatosInfo struct {
	Nombre      string                   `json:"nombre"`
	Descripcion string                   `json:"descripcion"`
	Parametros  []herramientas.Parametro `json:"parametros"`
}

// DatosValidar es el payload de la operacion="validar".
type DatosValidar struct {
	OK      bool   `json:"ok"`
	Detalle string `json:"detalle,omitempty"`
}

// ============================================================================
// Errores tipados
// ============================================================================

// ErrAutoCreacion es la base de los errores del paquete.
type ErrAutoCreacion struct {
	Etapa   string // "deteccion", "generacion", "compilacion", "carga", "registro"
	Causa   string
	Interno error
}

func (e *ErrAutoCreacion) Error() string {
	msg := "auto-creación [" + e.Etapa + "]: " + e.Causa
	if e.Interno != nil {
		msg += " — " + e.Interno.Error()
	}
	return msg
}

func (e *ErrAutoCreacion) Unwrap() error { return e.Interno }
