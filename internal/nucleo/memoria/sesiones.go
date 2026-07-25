package memoria

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// RolMensaje identifica quién emitió un mensaje.
type RolMensaje string

const (
	RolUsuario    RolMensaje = "usuario"
	RolAsistente  RolMensaje = "asistente"
	RolSistema    RolMensaje = "sistema"
	RolHerramienta RolMensaje = "herramienta"
)

// Mensaje es un turno de conversación.
type Mensaje struct {
	ID        string     `json:"id"`         // uuid
	Rol       RolMensaje `json:"rol"`
	Contenido string     `json:"contenido"`
	Timestamp string     `json:"timestamp"` // RFC3339
	TokenEstim int      `json:"token_estim"` // estimación ~4 chars/token
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // tool calls, etc.
}

// Sesion es una conversación entre usuario y Liz.
type Sesion struct {
	ID              string    `json:"id"`               // uuid
	UsuarioID       string    `json:"usuario_id"`       // identificador del usuario
	Titulo          string    `json:"titulo,omitempty"` // opcional, autogenerado
	Inicio          string    `json:"inicio"`           // RFC3339
	Fin             string    `json:"fin,omitempty"`    // RFC3339, vacío si sigue activa
	Activa          bool      `json:"activa"`
	Mensajes        []Mensaje `json:"mensajes"`
	Resumen         string    `json:"resumen,omitempty"` // generado al cerrar
	Proyecto        string    `json:"proyecto,omitempty"` // si la sesión trabaja sobre un proyecto
	TokensTotales   int       `json:"tokens_totales"`
}

// EstadisticasSesion resume una sesión.
type EstadisticasSesion struct {
	TotalMensajes   int `json:"total_mensajes"`
	MensajesUsuario int `json:"mensajes_usuario"`
	MensajesAsistente int `json:"mensajes_asistente"`
	TokensTotales   int `json:"tokens_totales"`
	DuracionSegundos int `json:"duracion_segundos"`
}

// ═══════════════════════════════════════════════════════
// GESTOR DE SESIONES
// ═══════════════════════════════════════════════════════

// GestorSesiones gestiona la persistencia y consulta de sesiones.
type GestorSesiones struct {
	mu         sync.RWMutex
	dirSesiones string // ~/.liz/memoria/sesiones/
	logFunc    func(string, ...interface{})

	// Cache en memoria: sesión activa por usuario
	activas map[string]*Sesion // usuarioID → sesión activa
}

// NuevoGestorSesiones crea un nuevo gestor de sesiones.
func NuevoGestorSesiones(dirBase string) (*GestorSesiones, error) {
	dir := filepath.Join(dirBase, "memoria", "sesiones")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de sesiones: %w", err)
	}
	g := &GestorSesiones{
		dirSesiones: dir,
		activas:     make(map[string]*Sesion),
		logFunc:     func(string, ...interface{}) {},
	}
	g.cargarSesionesActivas()
	return g, nil
}

// ConLog asigna función de log.
func (g *GestorSesiones) ConLog(fn func(string, ...interface{})) *GestorSesiones {
	if fn != nil {
		g.logFunc = fn
	}
	return g
}

// ═══════════════════════════════════════════════════════
// OPERACIONES DE SESIÓN
// ═══════════════════════════════════════════════════════

// NuevaSesion crea y persiste una nueva sesión para un usuario.
// Si ya existe una sesión activa, la cierra primero.
func (g *GestorSesiones) NuevaSesion(usuarioID, proyecto string) (*Sesion, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Cerrar sesión activa previa si existe
	if sesion, existe := g.activas[usuarioID]; existe {
		g.cerrarSesionInterno(sesion)
	}

	sesion := &Sesion{
		ID:        generarUUID(),
		UsuarioID: usuarioID,
		Inicio:    time.Now().UTC().Format(time.RFC3339),
		Activa:    true,
		Proyecto:  proyecto,
		Mensajes:  []Mensaje{},
	}

	g.activas[usuarioID] = sesion

	if err := g.guardarSesion(sesion); err != nil {
		delete(g.activas, usuarioID)
		return nil, err
	}

	g.logFunc("nueva sesión: %s (usuario: %s, proyecto: %s)",
		sesion.ID, usuarioID, proyecto)
	return sesion, nil
}

// SesionActiva retorna la sesión activa de un usuario, o nil si no hay.
func (g *GestorSesiones) SesionActiva(usuarioID string) *Sesion {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if s, existe := g.activas[usuarioID]; existe {
		// Retornar copia defensiva
		copia := *s
		return &copia
	}
	return nil
}

// AgregarMensaje agrega un mensaje a la sesión activa de un usuario.
// Si no hay sesión activa, retorna error.
func (g *GestorSesiones) AgregarMensaje(usuarioID string, rol RolMensaje, contenido string) (*Mensaje, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	sesion, existe := g.activas[usuarioID]
	if !existe {
		return nil, fmt.Errorf("no hay sesión activa para el usuario %s", usuarioID)
	}

	msg := Mensaje{
		ID:         generarUUID(),
		Rol:        rol,
		Contenido:  contenido,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		TokenEstim: estimarTokens(contenido),
	}

	sesion.Mensajes = append(sesion.Mensajes, msg)
	sesion.TokensTotales += msg.TokenEstim

	if err := g.guardarSesion(sesion); err != nil {
		// Rollback en memoria
		sesion.Mensajes = sesion.Mensajes[:len(sesion.Mensajes)-1]
		sesion.TokensTotales -= msg.TokenEstim
		return nil, err
	}

	return &msg, nil
}

// CerrarSesion marca la sesión activa de un usuario como cerrada,
// calcula duración y persiste.
func (g *GestorSesiones) CerrarSesion(usuarioID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	sesion, existe := g.activas[usuarioID]
	if !existe {
		return fmt.Errorf("no hay sesión activa para el usuario %s", usuarioID)
	}

	g.cerrarSesionInterno(sesion)
	delete(g.activas, usuarioID)
	return nil
}

// cerrarSesionInterno marca la sesión como cerrada y persiste (sin lock).
func (g *GestorSesiones) cerrarSesionInterno(s *Sesion) {
	s.Activa = false
	s.Fin = time.Now().UTC().Format(time.RFC3339)
	// Autogenerar título si no tiene
	if s.Titulo == "" && len(s.Mensajes) > 0 {
		// Primer mensaje del usuario como título (truncado a 50 chars)
		for _, m := range s.Mensajes {
			if m.Rol == RolUsuario {
				titulo := m.Contenido
				if len(titulo) > 50 {
					titulo = titulo[:50] + "..."
				}
				s.Titulo = titulo
				break
			}
		}
	}
	_ = g.guardarSesion(s)
	g.logFunc("sesión cerrada: %s (%d mensajes, %d tokens)",
		s.ID, len(s.Mensajes), s.TokensTotales)
}

// SetResumen actualiza el resumen de una sesión (llamado tras consolidación LLM).
func (g *GestorSesiones) SetResumen(sesionID, resumen string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Cargar sesión desde disco (puede no estar en cache)
	sesion, err := g.cargarSesion(sesionID)
	if err != nil {
		return err
	}
	sesion.Resumen = resumen
	return g.guardarSesion(sesion)
}

// ═══════════════════════════════════════════════════════
// CONSULTAS
// ═══════════════════════════════════════════════════════

// ObtenerSesion carga una sesión por ID (desde disco).
func (g *GestorSesiones) ObtenerSesion(sesionID string) (*Sesion, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.cargarSesion(sesionID)
}

// ListarSesiones lista las sesiones de un usuario, ordenadas por inicio descendente.
// Si soloActivas=true, retorna solo las sesiones activas.
func (g *GestorSesiones) ListarSesiones(usuarioID string, soloActivas bool) ([]Sesion, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	entries, err := os.ReadDir(g.dirSesiones)
	if err != nil {
		return nil, err
	}

	var resultado []Sesion
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(g.dirSesiones, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Sesion
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if s.UsuarioID != usuarioID {
			continue
		}
		if soloActivas && !s.Activa {
			continue
		}
		resultado = append(resultado, s)
	}

	// Ordenar por inicio descendente
	sort.Slice(resultado, func(i, j int) bool {
		return resultado[i].Inicio > resultado[j].Inicio
	})

	return resultado, nil
}

// UltimosMensajes retorna los últimos N mensajes de la sesión activa.
// Implementa "recall memory" estilo Letta.
func (g *GestorSesiones) UltimosMensajes(usuarioID string, n int) []Mensaje {
	g.mu.RLock()
	defer g.mu.RUnlock()

	sesion, existe := g.activas[usuarioID]
	if !existe {
		return nil
	}

	if n <= 0 || n > len(sesion.Mensajes) {
		n = len(sesion.Mensajes)
	}

	start := len(sesion.Mensajes) - n
	copia := make([]Mensaje, n)
	copy(copia, sesion.Mensajes[start:])
	return copia
}

// Estadisticas calcula métricas de una sesión.
func (s *Sesion) Estadisticas() EstadisticasSesion {
	stats := EstadisticasSesion{
		TotalMensajes: len(s.Mensajes),
		TokensTotales: s.TokensTotales,
	}
	for _, m := range s.Mensajes {
		if m.Rol == RolUsuario {
			stats.MensajesUsuario++
		} else if m.Rol == RolAsistente {
			stats.MensajesAsistente++
		}
	}
	if s.Inicio != "" && s.Fin != "" {
		inicio, err1 := time.Parse(time.RFC3339, s.Inicio)
		fin, err2 := time.Parse(time.RFC3339, s.Fin)
		if err1 == nil && err2 == nil {
			stats.DuracionSegundos = int(fin.Sub(inicio).Seconds())
		}
	}
	return stats
}

// ═══════════════════════════════════════════════════════
// PERSISTENCIA
// ═══════════════════════════════════════════════════════

// guardarSesion persiste una sesión a disco (sin lock).
func (g *GestorSesiones) guardarSesion(s *Sesion) error {
	path := filepath.Join(g.dirSesiones, s.ID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando sesión: %w", err)
	}
	// Escritura atómica: escribir a .tmp y renombrar
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("error escribiendo sesión: %w", err)
	}
	return os.Rename(tmp, path)
}

// cargarSesion carga una sesión desde disco (sin lock).
func (g *GestorSesiones) cargarSesion(sesionID string) (*Sesion, error) {
	path := filepath.Join(g.dirSesiones, sesionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("sesión %s no encontrada: %w", sesionID, err)
	}
	var s Sesion
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("error deserializando sesión: %w", err)
	}
	return &s, nil
}

// cargarSesionesActivas escanea disco al iniciar y carga sesiones con Activa=true.
func (g *GestorSesiones) cargarSesionesActivas() {
	entries, err := os.ReadDir(g.dirSesiones)
	if err != nil {
		return
	}
	cargadas := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(g.dirSesiones, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s Sesion
		if json.Unmarshal(data, &s) != nil {
			continue
		}
		if s.Activa {
			g.activas[s.UsuarioID] = &s
			cargadas++
		}
	}
	if cargadas > 0 {
		g.logFunc("sesiones activas cargadas desde disco: %d", cargadas)
	}
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// generarUUID genera un UUID v4 hex string de 32 caracteres (sin guiones).
// No usa crypto fuerte pero sí random criptográfico para evitar colisiones.
func generarUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set version (4) y variant (10xx)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b)
}

// estimarTokens calcula una estimación de tokens (4 chars ≈ 1 token).
func estimarTokens(texto string) int {
	return len(texto) / 4
}
