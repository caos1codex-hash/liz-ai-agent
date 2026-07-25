package pipeline

import (
	"context"
)

// OrquestadorCliente define la interfaz que el pipeline necesita del orquestador.
// Permite desacoplar y facilitar testing con mocks.
type OrquestadorCliente interface {
	Completar(ctx context.Context, prompt string, tipoTarea string) (string, error)
	CompletarStream(ctx context.Context, prompt string, tipoTarea string) (<-chan ChunkOrquestador, error)
	ModeloActual() string
}

// ChunkOrquestador representa un fragmento de la respuesta del orquestador.
type ChunkOrquestador struct {
	Delta   string
	Modelo  string
	Error   error
	Done    bool
}

// CatalogoCliente define la interfaz que el pipeline necesita del catálogo de herramientas.
type CatalogoCliente interface {
	Existe(nombre string) bool
	Ejecutar(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error)
	Snapshot() []InfoHerramientaSnapshot
}

// ResultadoHerramienta es el resultado simplificado de una herramienta.
type ResultadoHerramienta struct {
	Exito    bool                   `json:"exito"`
	Datos    interface{}            `json:"datos"`
	Error    string                 `json:"error,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// InfoHerramientaSnapshot es la vista serializable de una herramienta.
type InfoHerramientaSnapshot struct {
	Nombre     string      `json:"nombre"`
	Descripcion string      `json:"descripcion"`
	Parametros interface{} `json:"parametros"`
}

// AutoCreacionGestor define la interfaz para auto-creación de herramientas.
type AutoCreacionGestor interface {
	Crear(ctx context.Context, descripcion string) (*ResultadoAutoCreacion, error)
}

// ResultadoAutoCreacion es el resultado de la auto-creación.
type ResultadoAutoCreacion struct {
	Exito bool        `json:"exito"`
	Datos interface{} `json:"datos"`
	Error string      `json:"error,omitempty"`
}

// ContextoCoordinador define la interfaz para el coordinador de contexto.
type ContextoCoordinador interface {
	EmpaquetarContexto(ctx context.Context, proyecto, query string, maxTokens int) (string, error)
}

// MemoriaGestor define la interfaz para el gestor de memoria.
type MemoriaGestor interface {
	ObtenerSesion(ctx context.Context, sesionID, usuarioID string) (*InfoSesion, error)
	CrearSesion(ctx context.Context, usuarioID, proyecto string) (*InfoSesion, error)
	AgregarMensaje(ctx context.Context, sesionID, usuarioID, contenido string) error
	ObtenerMensajesRecientes(sesionID string, limite int) []InfoMensaje
	ObtenerHechos(usuarioID string, limite int) string
	ContextoParaLLM(usuarioID string, ultimosNMensajes int, limiteHechos int) string
}

// InfoSesion es la información de una sesión.
type InfoSesion struct {
	ID        string `json:"id"`
	UsuarioID string `json:"usuario_id"`
	Proyecto  string `json:"proyecto,omitempty"`
	Titulo    string `json:"titulo,omitempty"`
}

// InfoMensaje es la información de un mensaje.
type InfoMensaje struct {
	Rol       string `json:"rol"`
	Contenido string `json:"contenido"`
}

// jsonParser es una interfaz para parsear JSON (usado en tests).
type jsonParser interface {
	Parsear(data string, v interface{}) error
}
