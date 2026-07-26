package pipeline

import (
	"context"
)

// AdaptadorOrquestador adapta el orquestador real del proyecto a la interfaz del pipeline.
type AdaptadorOrquestador struct {
	completarFunc       func(ctx context.Context, prompt string, tipoTarea string) (string, error)
	completarStreamFunc func(ctx context.Context, prompt string, tipoTarea string) (<-chan ChunkOrquestador, error)
	modeloActualFunc    func() string
}

// Completar implementa OrquestadorCliente.
func (a *AdaptadorOrquestador) Completar(ctx context.Context, prompt string, tipoTarea string) (string, error) {
	if a.completarFunc != nil {
		return a.completarFunc(ctx, prompt, tipoTarea)
	}
	return "", nil
}

// CompletarStream implementa OrquestadorCliente.
func (a *AdaptadorOrquestador) CompletarStream(ctx context.Context, prompt string, tipoTarea string) (<-chan ChunkOrquestador, error) {
	if a.completarStreamFunc != nil {
		return a.completarStreamFunc(ctx, prompt, tipoTarea)
	}
	ch := make(chan ChunkOrquestador, 1)
	close(ch)
	return ch, nil
}

// ModeloActual implementa OrquestadorCliente.
func (a *AdaptadorOrquestador) ModeloActual() string {
	if a.modeloActualFunc != nil {
		return a.modeloActualFunc()
	}
	return "desconocido"
}

// AdaptadorCatalogo adapta el catálogo real a la interfaz del pipeline.
type AdaptadorCatalogo struct {
	existeFunc   func(nombre string) bool
	ejecutarFunc func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error)
	snapshotFunc func() []InfoHerramientaSnapshot
}

// Existe implementa CatalogoCliente.
func (a *AdaptadorCatalogo) Existe(nombre string) bool {
	if a.existeFunc != nil {
		return a.existeFunc(nombre)
	}
	return false
}

// Ejecutar implementa CatalogoCliente.
func (a *AdaptadorCatalogo) Ejecutar(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
	if a.ejecutarFunc != nil {
		return a.ejecutarFunc(ctx, nombre, params)
	}
	return nil, nil
}

// Snapshot implementa CatalogoCliente.
func (a *AdaptadorCatalogo) Snapshot() []InfoHerramientaSnapshot {
	if a.snapshotFunc != nil {
		return a.snapshotFunc()
	}
	return nil
}

// AdaptadorMemoria adapta el gestor de memoria real a la interfaz del pipeline.
type AdaptadorMemoria struct {
	obtenerSesionFunc            func(ctx context.Context, sesionID, usuarioID string) (*InfoSesion, error)
	crearSesionFunc              func(ctx context.Context, usuarioID, proyecto string) (*InfoSesion, error)
	agregarMensajeFunc           func(ctx context.Context, sesionID, usuarioID, contenido string) error
	obtenerMensajesRecientesFunc func(sesionID string, limite int) []InfoMensaje
	obtenerHechosFunc            func(usuarioID string, limite int) string
	contextoParaLLMFunc          func(usuarioID string, ultimosNMensajes int, limiteHechos int) string
}

// ObtenerSesion implementa MemoriaGestor.
func (a *AdaptadorMemoria) ObtenerSesion(ctx context.Context, sesionID, usuarioID string) (*InfoSesion, error) {
	if a.obtenerSesionFunc != nil {
		return a.obtenerSesionFunc(ctx, sesionID, usuarioID)
	}
	return nil, nil
}

// CrearSesion implementa MemoriaGestor.
func (a *AdaptadorMemoria) CrearSesion(ctx context.Context, usuarioID, proyecto string) (*InfoSesion, error) {
	if a.crearSesionFunc != nil {
		return a.crearSesionFunc(ctx, usuarioID, proyecto)
	}
	return &InfoSesion{ID: generarUUID(), UsuarioID: usuarioID, Proyecto: proyecto}, nil
}

// AgregarMensaje implementa MemoriaGestor.
func (a *AdaptadorMemoria) AgregarMensaje(ctx context.Context, sesionID, usuarioID, contenido string) error {
	if a.agregarMensajeFunc != nil {
		return a.agregarMensajeFunc(ctx, sesionID, usuarioID, contenido)
	}
	return nil
}

// ObtenerMensajesRecientes implementa MemoriaGestor.
func (a *AdaptadorMemoria) ObtenerMensajesRecientes(sesionID string, limite int) []InfoMensaje {
	if a.obtenerMensajesRecientesFunc != nil {
		return a.obtenerMensajesRecientesFunc(sesionID, limite)
	}
	return nil
}

// ObtenerHechos implementa MemoriaGestor.
func (a *AdaptadorMemoria) ObtenerHechos(usuarioID string, limite int) string {
	if a.obtenerHechosFunc != nil {
		return a.obtenerHechosFunc(usuarioID, limite)
	}
	return ""
}

// ContextoParaLLM implementa MemoriaGestor.
func (a *AdaptadorMemoria) ContextoParaLLM(usuarioID string, ultimosNMensajes int, limiteHechos int) string {
	if a.contextoParaLLMFunc != nil {
		return a.contextoParaLLMFunc(usuarioID, ultimosNMensajes, limiteHechos)
	}
	return ""
}

// AdaptadorAutoCreacion adapta el gestor de auto-creación real.
type AdaptadorAutoCreacion struct {
	crearFunc func(ctx context.Context, descripcion string) (*ResultadoAutoCreacion, error)
}

// Crear implementa AutoCreacionGestor.
func (a *AdaptadorAutoCreacion) Crear(ctx context.Context, descripcion string) (*ResultadoAutoCreacion, error) {
	if a.crearFunc != nil {
		return a.crearFunc(ctx, descripcion)
	}
	return &ResultadoAutoCreacion{Exito: false, Error: "auto-creación no disponible"}, nil
}

// AdaptadorContexto adapta el coordinador de contexto real.
type AdaptadorContexto struct {
	empaquetarFunc func(ctx context.Context, proyecto, query string, maxTokens int) (string, error)
}

// EmpaquetarContexto implementa ContextoCoordinador.
func (a *AdaptadorContexto) EmpaquetarContexto(ctx context.Context, proyecto, query string, maxTokens int) (string, error) {
	if a.empaquetarFunc != nil {
		return a.empaquetarFunc(ctx, proyecto, query, maxTokens)
	}
	return "", nil
}
