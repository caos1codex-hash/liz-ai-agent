package pipeline

import (
        "context"
        "fmt"
        "strings"
        "time"
)

// Receptor se encarga de recibir, validar y almacenar mensajes del usuario.
// Gestiona la integración con el sistema de memoria para sesiones y mensajes.
type Receptor struct {
        memoria memoriaGestor
}

// nuevoReceptor crea un Receptor con el gestor de memoria inyectado.
func nuevoReceptor(mem memoriaGestor) *Receptor {
        return &Receptor{memoria: mem}
}

// Recibir valida la solicitud, crea/retoma una sesión y almacena el mensaje.
// Retorna el MensajeChat creado listo para el pipeline.
func (r *Receptor) Recibir(ctx context.Context, sol *SolicitudChat) (*MensajeChat, *SesionInfo, error) {
        if err := sol.Validar(); err != nil {
                return nil, nil, fmt.Errorf("solicitud inválida: %w", err)
        }

        sesion, err := r.obtenerOCrearSesion(ctx, sol.UsuarioID, sol.SesionID, sol.Proyecto)
        if err != nil {
                return nil, nil, fmt.Errorf("error gestionando sesión: %w", err)
        }

        mensaje := &MensajeChat{
                ID:              generarUUID(),
                SesionID:        sesion.ID,
                UsuarioID:       sol.UsuarioID,
                Contenido:       sol.Mensaje,
                Rol:             "usuario",
                Timestamp:       time.Now(),
                TokensEstimados: estimarTokens(sol.Mensaje),
                Metadata:        make(map[string]interface{}),
        }

        // Almacenar mensaje en memoria si está disponible
        if r.memoria != nil {
                if err := r.memoria.AgregarMensaje(ctx, sesion.ID, sol.UsuarioID, sol.Mensaje); err != nil {
                        // Log pero no fallar — el pipeline continúa
                        mensaje.Metadata["advertencia_memoria"] = err.Error()
                }
        }

        return mensaje, sesion, nil
}

// obtenerOCrearSesion retoma una sesión existente o crea una nueva.
func (r *Receptor) obtenerOCrearSesion(ctx context.Context, usuarioID, sesionID, proyecto string) (*SesionInfo, error) {
        if r.memoria == nil {
                return &SesionInfo{
                        ID:        sesionID,
                        UsuarioID: usuarioID,
                        Proyecto:  proyecto,
                }, nil
        }

        if sesionID != "" {
                sesion, err := r.memoria.ObtenerSesion(ctx, sesionID, usuarioID)
                if err == nil && sesion != nil {
                        return &SesionInfo{
                                ID:        sesion.ID,
                                UsuarioID: usuarioID,
                                Proyecto:  proyecto,
                                Titulo:    sesion.Titulo,
                        }, nil
                }
        }

        sesion, err := r.memoria.CrearSesion(ctx, usuarioID, proyecto)
        if err != nil {
                // Fallback: crear sesión local sin persistencia
                return &SesionInfo{
                        ID:        generarUUID(),
                        UsuarioID: usuarioID,
                        Proyecto:  proyecto,
                }, nil
        }

        return &SesionInfo{
                ID:        sesion.ID,
                UsuarioID: usuarioID,
                Proyecto:  proyecto,
                Titulo:    sesion.Titulo,
        }, nil
}

// SesionInfo contiene información de la sesión para el pipeline.
type SesionInfo struct {
        ID        string `json:"id"`
        UsuarioID string `json:"usuario_id"`
        Proyecto  string `json:"proyecto,omitempty"`
        Titulo    string `json:"titulo,omitempty"`
}

// generarUUID crea un identificador único simple.
func generarUUID() string {
        return fmt.Sprintf("%d", time.Now().UnixNano())
}

// truncarTexto trunca un texto al máximo de caracteres indicado.
func truncarTexto(texto string, max int) string {
        if len(texto) <= max {
                return texto
        }
        return texto[:max] + "... (truncado)"
}

// tienePalabrasClave verifica si el texto contiene alguna de las palabras clave.
func tienePalabrasClave(texto string, palabras []string) bool {
        textoLower := strings.ToLower(texto)
        for _, p := range palabras {
                if strings.Contains(textoLower, strings.ToLower(p)) {
                        return true
                }
        }
        return false
}

// extraerComando intenta extraer un comando shell del texto del usuario.
func extraerComando(texto string) string {
        texto = strings.TrimSpace(texto)
        // Si está entre backticks o comillas, extraerlo
        if strings.HasPrefix(texto, "`") && strings.HasSuffix(texto, "`") {
                return strings.Trim(texto, "`")
        }
        if strings.HasPrefix(texto, "'") && strings.HasSuffix(texto, "'") {
                return strings.Trim(texto, "'")
        }
        if strings.HasPrefix(texto, "\"") && strings.HasSuffix(texto, "\"") {
                return strings.Trim(texto, "\"")
        }
        // Si empieza con verbos de terminal comunes
        verbos := []string{"ls", "cd", "rm", "cp", "mv", "cat", "grep", "find", "chmod",
                "chown", "mkdir", "touch", "echo", "wget", "curl", "apt", "snap", "dnf",
                "pacman", "pip", "npm", "cargo", "go ", "docker", "systemctl", "kill",
                "ps ", "top", "htop", "df", "du", "free", "uname", "whoami", "pwd",
                "git ", "make ", "gcc", "g++", "python", "node", "bash"}
        for _, v := range verbos {
                if strings.HasPrefix(textoLower(texto), v) {
                        return texto
                }
        }
        return ""
}

func textoLower(s string) string {
        return strings.ToLower(s)
}
