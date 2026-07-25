package memoria

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════
// GESTOR DE MEMORIA UNIFICADO
// ═══════════════════════════════════════════════════════

// Gestor es el punto de entrada unificado del sistema de memoria.
// Coordina sesiones, hechos y (futuro) resúmenes consolidados.
//
// Uso típico:
//
//	g, _ := memoria.NuevoGestor("~/.liz")
//	sesion, _ := g.NuevaSesion("user-1", "liz-ai-agent")
//	g.AgregarMensaje("user-1", memoria.RolUsuario, "Hola")
//	g.AgregarHecho("user-1", "usuario", "prefiere", "Go", 0.9, sesion.ID)
//	contexto := g.ContextoParaLLM("user-1", 10, 20) // últimos 10 mensajes + 20 hechos
type Gestor struct {
	mu      sync.RWMutex
	dirBase string
	sesiones *GestorSesiones
	hechos   *GestorHechos
	logFunc  func(string, ...interface{})
}

// NuevoGestor crea un gestor de memoria unificado.
// dirBase es típicamente ~/.liz
func NuevoGestor(dirBase string) (*Gestor, error) {
	if dirBase == "" {
		return nil, fmt.Errorf("dirBase no puede estar vacío")
	}

	// Crear subdirectorios
	for _, sub := range []string{"memoria", "memoria/sesiones", "memoria/hechos", "memoria/resumenes"} {
		if err := os.MkdirAll(filepath.Join(dirBase, sub), 0755); err != nil {
			return nil, fmt.Errorf("error creando %s: %w", sub, err)
		}
	}

	sesiones, err := NuevoGestorSesiones(dirBase)
	if err != nil {
		return nil, err
	}
	hechos, err := NuevoGestorHechos(dirBase)
	if err != nil {
		return nil, err
	}

	g := &Gestor{
		dirBase:  dirBase,
		sesiones: sesiones,
		hechos:   hechos,
		logFunc:  func(string, ...interface{}) {},
	}

	return g, nil
}

// ConLog asigna función de log a todos los sub-gestores.
func (g *Gestor) ConLog(fn func(string, ...interface{})) *Gestor {
	if fn != nil {
		g.logFunc = fn
		g.sesiones.ConLog(fn)
		g.hechos.ConLog(fn)
	}
	return g
}

// Sesiones retorna el gestor de sesiones (para operaciones granulares).
func (g *Gestor) Sesiones() *GestorSesiones { return g.sesiones }

// Hechos retorna el gestor de hechos (para operaciones granulares).
func (g *Gestor) Hechos() *GestorHechos { return g.hechos }

// ═══════════════════════════════════════════════════════
// MÉTODOS DE CONVENIENCIA
// ═══════════════════════════════════════════════════════

// NuevaSesion crea una nueva sesión para un usuario.
func (g *Gestor) NuevaSesion(usuarioID, proyecto string) (*Sesion, error) {
	return g.sesiones.NuevaSesion(usuarioID, proyecto)
}

// AgregarMensaje agrega un mensaje a la sesión activa del usuario.
func (g *Gestor) AgregarMensaje(usuarioID string, rol RolMensaje, contenido string) (*Mensaje, error) {
	return g.sesiones.AgregarMensaje(usuarioID, rol, contenido)
}

// AgregarHecho extrae y persiste un hecho del usuario.
func (g *Gestor) AgregarHecho(usuarioID, sujeto, predicado, objeto string, confianza float64, sesionOrigen string) (*Hecho, error) {
	return g.hechos.AgregarHecho(usuarioID, sujeto, predicado, objeto, confianza, sesionOrigen)
}

// CerrarSesion cierra la sesión activa del usuario.
func (g *Gestor) CerrarSesion(usuarioID string) error {
	return g.sesiones.CerrarSesion(usuarioID)
}

// ContextoParaLLM construye el contexto de memoria para inyectar en el prompt del LLM.
//
// Incluye:
//   - Memoria semántica: hechos activos del usuario (limite_hechos)
//   - Memoria episódica: últimos N mensajes de la sesión activa (ultimos_n_mensajes)
//
// Útil para alimentar al Context Packer o directamente al prompt del orquestador.
func (g *Gestor) ContextoParaLLM(usuarioID string, ultimosNMensajes int, limiteHechos int) (string, error) {
	var secciones []string

	// Hechos (memoria semántica)
	if hechosCtx, err := g.hechos.FormatoContexto(usuarioID, limiteHechos); err == nil && hechosCtx != "" {
		secciones = append(secciones, hechosCtx)
	}

	// Últimos mensajes (memoria episódica)
	mensajes := g.sesiones.UltimosMensajes(usuarioID, ultimosNMensajes)
	if len(mensajes) > 0 {
		var sb strings.Builder
		sb.WriteString("\n# Contexto de la conversación reciente\n")
		for _, m := range mensajes {
			rol := string(m.Rol)
			sb.WriteString(fmt.Sprintf("[%s] %s\n", rol, m.Contenido))
		}
		secciones = append(secciones, sb.String())
	}

	if len(secciones) == 0 {
		return "", nil
	}

	return strings.Join(secciones, "\n"), nil
}

// EstadisticasMemoria retorna métricas combinadas de sesiones y hechos.
type EstadisticasMemoria struct {
	SesionesActivas int                    `json:"sesiones_activas"`
	HechosActivos   int                    `json:"hechos_activos"`
	HechosObsoletos int                    `json:"hechos_obsoletos"`
}

// Estadisticas retorna métricas generales del sistema de memoria.
func (g *Gestor) Estadisticas(usuarioID string) EstadisticasMemoria {
	stats := EstadisticasMemoria{}

	// Sesiones activas: chequear si hay sesión activa en cache
	if g.sesiones.SesionActiva(usuarioID) != nil {
		stats.SesionesActivas = 1
	}

	// Hechos
	if hStats, err := g.hechos.Estadisticas(usuarioID); err == nil {
		stats.HechosActivos = hStats.HechosActivos
		stats.HechosObsoletos = hStats.HechosObsoletos
	}

	return stats
}
