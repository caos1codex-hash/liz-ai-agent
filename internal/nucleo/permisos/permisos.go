package permisos

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// TipoPermiso representa los diferentes permisos que Liz puede solicitar.
type TipoPermiso string

const (
	PermArchivos  TipoPermiso = "archivos"
	PermTerminal  TipoPermiso = "terminal"
	PermProcesos  TipoPermiso = "procesos"
	PermRed       TipoPermiso = "red"
	PermPaquetes  TipoPermiso = "paquetes"
	PermSistema   TipoPermiso = "sistema"
)

// Permiso individual con su estado.
type Permiso struct {
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion"`
	Concedido   bool      `json:"concedido"`
	FechaHora   time.Time `json:"fecha_hora,omitempty"`
}

// EstadoPermisos es el estado completo de permisos del usuario.
type EstadoPermisos struct {
	Version         string             `json:"version"`
	Concedidos      bool               `json:"concedidos"`
	Permisos        map[string]Permiso `json:"permisos"`
	FechaConcesion  time.Time          `json:"fecha_concesion,omitempty"`
	SesionID        string             `json:"sesion_id,omitempty"`
	RecordarSesion  bool               `json:"recordar_sesion"`
}

// EntradaAuditoria es un registro de una acción que requirió permisos.
type EntradaAuditoria struct {
	Permiso   string    `json:"permiso"`
	Accion    string    `json:"accion"`
	Concedido bool      `json:"concedido"`
	Timestamp time.Time `json:"timestamp"`
	Detalle   string    `json:"detalle,omitempty"`
}

// RequisitoPermiso mapea qué permiso necesita cada ruta.
type RequisitoPermiso struct {
	Ruta    string
	Metodo  string
	Permiso TipoPermiso
}

// Sistema gestiona el sistema de permisos de Liz.
// Implementa el principio de "Permisos Una Vez" (D-006).
type Sistema struct {
	mu          sync.RWMutex
	estado      *EstadoPermisos
	rutaArchivo string
	auditoria   []EntradaAuditoria
	maxAuditoria int
	recordar    bool // recordar_entre_sesiones
}

// permisosDefecto define los permisos que Liz solicita al iniciar.
var permisosDefecto = []Permiso{
	{
		Nombre:      string(PermArchivos),
		Descripcion: "Acceso completo al sistema de archivos (leer, escribir, eliminar)",
	},
	{
		Nombre:      string(PermTerminal),
		Descripcion: "Ejecución de comandos en la terminal del sistema",
	},
	{
		Nombre:      string(PermProcesos),
		Descripcion: "Gestión de procesos (listar, iniciar, detener, matar)",
	},
	{
		Nombre:      string(PermRed),
		Descripcion: "Acceso a red (requests HTTP, descargas, conexiones)",
	},
	{
		Nombre:      string(PermPaquetes),
		Descripcion: "Instalación de paquetes del sistema operativo",
	},
	{
		Nombre:      string(PermSistema),
		Descripcion: "Acceso a /proc y /sys para monitoreo del sistema",
	},
}

// ═══════════════════════════════════════════════════════
// CONSTRUCTOR
// ═══════════════════════════════════════════════════════

// NuevoSistema crea e inicializa el sistema de permisos.
// Carga permisos existentes desde ~/.liz/permisos.json si existen.
func NuevoSistema() (*Sistema, error) {
	return NuevoSistemaConRecordar(false)
}

// NuevoSistemaConRecordar crea el sistema con la opción de recordar entre sesiones.
func NuevoSistemaConRecordar(recordar bool) (*Sistema, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo directorio home: %w", err)
	}

	ruta := filepath.Join(home, ".liz", "permisos.json")

	s := &Sistema{
		rutaArchivo:   ruta,
		maxAuditoria:  1000,
		recordar:      recordar,
		estado: &EstadoPermisos{
			Version:  "0.1.0",
			Permisos: make(map[string]Permiso),
		},
	}

	// Inicializar permisos por defecto
	for _, p := range permisosDefecto {
		s.estado.Permisos[p.Nombre] = Permiso{
			Nombre:      p.Nombre,
			Descripcion: p.Descripcion,
			Concedido:   false,
		}
	}

	// Cargar permisos existentes si el archivo ya existe
	if datos, err := os.ReadFile(ruta); err == nil {
		var existente EstadoPermisos
		if json.Unmarshal(datos, &existente) == nil {
			for nombre, perm := range existente.Permisos {
				s.estado.Permisos[nombre] = perm
			}
			s.estado.Concedidos = existente.Concedidos
			s.estado.FechaConcesion = existente.FechaConcesion
			s.estado.SesionID = existente.SesionID
			s.estado.RecordarSesion = existente.RecordarSesion
		}
	}

	// Si NO se debe recordar entre sesiones y hay permisos concedidos de una sesión previa
	if !recordar && s.estado.Concedidos {
		// Limpiar la sesión anterior pero no el archivo
		s.estado.Concedidos = false
		s.estado.FechaConcesion = time.Time{}
		s.estado.SesionID = ""
		for nombre, p := range s.estado.Permisos {
			p.Concedido = false
			p.FechaHora = time.Time{}
			s.estado.Permisos[nombre] = p
		}
		s.guardar()
	}

	return s, nil
}

// ═══════════════════════════════════════════════════════
// CONSULTAS
// ═══════════════════════════════════════════════════════

// PermisosPendientes retorna los permisos que aún no han sido concedidos.
func (s *Sistema) PermisosPendientes() []Permiso {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pendientes []Permiso
	for _, p := range s.estado.Permisos {
		if !p.Concedido {
			pendientes = append(pendientes, p)
		}
	}
	return pendientes
}

// TodosConcedidos retorna true si todos los permisos están concedidos.
func (s *Sistema) TodosConcedidos() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.estado.Concedidos
}

// Verificar retorna true si un permiso específico está concedido.
func (s *Sistema) Verificar(permiso TipoPermiso) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.estado.Concedidos {
		return true
	}

	if p, existe := s.estado.Permisos[string(permiso)]; existe {
		return p.Concedido
	}

	return false
}

// Estado retorna una copia del estado actual de permisos.
func (s *Sistema) Estado() EstadoPermisos {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copia := EstadoPermisos{
		Version:         s.estado.Version,
		Concedidos:      s.estado.Concedidos,
		FechaConcesion:  s.estado.FechaConcesion,
		SesionID:        s.estado.SesionID,
		RecordarSesion:  s.estado.RecordarSesion,
		Permisos:        make(map[string]Permiso),
	}
	for k, v := range s.estado.Permisos {
		copia.Permisos[k] = v
	}
	return copia
}

// Auditoria retorna las últimas N entradas de auditoría.
func (s *Sistema) Auditoria(limit int) []EntradaAuditoria {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.auditoria) {
		limit = len(s.auditoria)
	}

	resultado := make([]EntradaAuditoria, limit)
	total := len(s.auditoria)
	for i := 0; i < limit; i++ {
		resultado[i] = s.auditoria[total-limit+i]
	}
	return resultado
}

// TotalAuditoria retorna el número total de entradas.
func (s *Sistema) TotalAuditoria() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.auditoria)
}

// ═══════════════════════════════════════════════════════
// MUTACIONES
// ═══════════════════════════════════════════════════════

// ConcederTodos concede todos los permisos de una sola vez.
func (s *Sistema) ConcederTodos(sesionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	s.estado.Concedidos = true
	s.estado.FechaConcesion = now
	s.estado.SesionID = sesionID

	for nombre, p := range s.estado.Permisos {
		p.Concedido = true
		p.FechaHora = now
		s.estado.Permisos[nombre] = p
	}

	return s.guardar()
}

// Conceder concede un permiso individual por nombre.
func (s *Sistema) Conceder(nombre string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, existe := s.estado.Permisos[nombre]; existe {
		p.Concedido = true
		p.FechaHora = time.Now().UTC()
		s.estado.Permisos[nombre] = p

		todos := true
		for _, pp := range s.estado.Permisos {
			if !pp.Concedido {
				todos = false
				break
			}
		}
		if todos {
			s.estado.Concedidos = true
			s.estado.FechaConcesion = time.Now().UTC()
		}

		return s.guardar()
	}

	return fmt.Errorf("permiso %s no existe", nombre)
}

// Revocar revoca un permiso individual.
func (s *Sistema) Revocar(nombre string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, existe := s.estado.Permisos[nombre]; existe {
		p.Concedido = false
		p.FechaHora = time.Time{}
		s.estado.Permisos[nombre] = p
		s.estado.Concedidos = false

		return s.guardar()
	}

	return fmt.Errorf("permiso %s no existe", nombre)
}

// Resetear revoca todos los permisos.
func (s *Sistema) Resetear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.estado.Concedidos = false
	s.estado.FechaConcesion = time.Time{}
	s.estado.SesionID = ""

	for nombre, p := range s.estado.Permisos {
		p.Concedido = false
		p.FechaHora = time.Time{}
		s.estado.Permisos[nombre] = p
	}

	return s.guardar()
}

// ═══════════════════════════════════════════════════════
// MIDDLEWARE HTTP
// ═══════════════════════════════════════════════════════

// RequisitosPorRuta define qué permiso necesita cada ruta protegida.
// Rutas no listadas aquí son públicas (no requieren permiso).
var RequisitosPorRuta = []RequisitoPermiso{
	{Ruta: "/api/chat", Metodo: "POST", Permiso: PermTerminal},
	{Ruta: "/api/tools", Metodo: "GET", Permiso: PermSistema},
	{Ruta: "/api/conversations", Metodo: "POST", Permiso: PermArchivos},
	{Ruta: "/api/conversations", Metodo: "DELETE", Permiso: PermArchivos},
}

// PermisoRequerido busca qué permiso necesita una ruta+método.
// Retorna "" si la ruta es pública.
func PermisoRequerido(ruta, metodo string) TipoPermiso {
	for _, req := range RequisitosPorRuta {
		if req.Ruta == ruta && req.Metodo == metodo {
			return req.Permiso
		}
	}
	return ""
}

// MiddlewareHTTP retorna un middleware que verifica permisos antes de pasar al handler.
// Rutas públicas (health, config GET, permisos GET/POST) no requieren permiso.
func (s *Sistema) MiddlewareHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		permisoReq := PermisoRequerido(r.URL.Path, r.Method)

		if permisoReq == "" {
			// Ruta pública — pasar directamente
			next.ServeHTTP(w, r)
			return
		}

		// Verificar permiso
		concedido := s.Verificar(permisoReq)

		s.registrarAuditoria(EntradaAuditoria{
			Permiso:   string(permisoReq),
			Accion:    fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			Concedido: concedido,
			Timestamp: time.Now().UTC(),
			Detalle:   r.URL.RequestURI(),
		})

		if !concedido {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"exito":       false,
				"error":       fmt.Sprintf("permiso '%s' no concedido", permisoReq),
				"permiso":     string(permisoReq),
				"conceder_en": "/api/permisos",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ═══════════════════════════════════════════════════════
// AUDITORÍA INTERNA
// ═══════════════════════════════════════════════════════

// registrarAuditoria agrega una entrada al log de auditoría.
// Se ejecuta dentro de un lock externo o internamente.
func (s *Sistema) registrarAuditoria(entrada EntradaAuditoria) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditoria = append(s.auditoria, entrada)

	// Limitar tamaño del buffer de auditoría
	if len(s.auditoria) > s.maxAuditoria {
		s.auditoria = s.auditoria[len(s.auditoria)-s.maxAuditoria:]
	}
}

// ═══════════════════════════════════════════════════════
// PERSISTENCIA
// ═══════════════════════════════════════════════════════

// guardar persiste el estado de permisos a ~/.liz/permisos.json.
func (s *Sistema) guardar() error {
	datos, err := json.MarshalIndent(s.estado, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializando permisos: %w", err)
	}

	if err := os.WriteFile(s.rutaArchivo, datos, 0644); err != nil {
		return fmt.Errorf("error guardando permisos en %s: %w", s.rutaArchivo, err)
	}

	return nil
}
