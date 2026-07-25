package permisos

import (
        "encoding/json"
        "fmt"
        "os"
        "path/filepath"
        "sync"
        "time"
)

// TipoPermiso representa los diferentes permisos que Liz puede solicitar.
type TipoPermiso string

const (
        PermArchivos     TipoPermiso = "archivos"
        PermTerminal     TipoPermiso = "terminal"
        PermProcesos     TipoPermiso = "procesos"
        PermRed          TipoPermiso = "red"
        PermPaquetes     TipoPermiso = "paquetes"
        PermSistema      TipoPermiso = "sistema"
)

// Permiso individual con su estado.
type Permiso struct {
        Nombre    string    `json:"nombre"`
        Descripcion string   `json:"descripcion"`
        Concedido bool      `json:"concedido"`
        FechaHora time.Time `json:"fecha_hora,omitempty"`
}

// EstadoPermisos es el estado completo de permisos del usuario.
type EstadoPermisos struct {
        Version  string             `json:"version"`
        Concedidos bool              `json:"concedidos"` // true si se concedieron todos
        Permisos map[string]Permiso `json:"permisos"`
        FechaConcesion time.Time    `json:"fecha_concesion,omitempty"`
        SesionID string            `json:"sesion_id,omitempty"`
}

// Sistema gestiona el sistema de permisos de Liz.
// Implementa el principio de "Permisos Una Vez" (D-006).
type Sistema struct {
        mu       sync.RWMutex
        estado   *EstadoPermisos
        rutaArchivo string
}

// permisosDefecto define los permisos que Liz solicita al iniciar.
var permisosDefecto = []Permiso{
        {
                Nombre:     string(PermArchivos),
                Descripcion: "Acceso completo al sistema de archivos (leer, escribir, eliminar)",
        },
        {
                Nombre:     string(PermTerminal),
                Descripcion: "Ejecución de comandos en la terminal del sistema",
        },
        {
                Nombre:     string(PermProcesos),
                Descripcion: "Gestión de procesos (listar, iniciar, detener, matar)",
        },
        {
                Nombre:     string(PermRed),
                Descripcion: "Acceso a red (requests HTTP, descargas, conexiones)",
        },
        {
                Nombre:     string(PermPaquetes),
                Descripcion: "Instalación de paquetes del sistema operativo",
        },
        {
                Nombre:     string(PermSistema),
                Descripcion: "Acceso a /proc y /sys para monitoreo del sistema",
        },
}

// NuevoSistema crea e inicializa el sistema de permisos.
// Carga permisos existentes desde ~/.liz/permisos.json si existen.
func NuevoSistema() (*Sistema, error) {
        home, err := os.UserHomeDir()
        if err != nil {
                return nil, fmt.Errorf("error obteniendo directorio home: %w", err)
        }

        ruta := filepath.Join(home, ".liz", "permisos.json")

        s := &Sistema{
                rutaArchivo: ruta,
                estado: &EstadoPermisos{
                        Version:  "0.1.0",
                        Permisos: make(map[string]Permiso),
                },
        }

        // Inicializar permisos por defecto
        for _, p := range permisosDefecto {
                s.estado.Permisos[p.Nombre] = Permiso{
                        Nombre:     p.Nombre,
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
                }
        }

        return s, nil
}

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

// ConcederTodos concede todos los permisos de una sola vez.
// Este es el flujo principal: el usuario concede todo al iniciar.
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

                // Verificar si todos están concedidos ahora
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
                Version:        s.estado.Version,
                Concedidos:     s.estado.Concedidos,
                FechaConcesion: s.estado.FechaConcesion,
                SesionID:      s.estado.SesionID,
                Permisos:      make(map[string]Permiso),
        }
        for k, v := range s.estado.Permisos {
                copia.Permisos[k] = v
        }
        return copia
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
