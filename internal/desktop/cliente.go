package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClienteBackend es el cliente HTTP/SSE que se comunica con el servidor Go
// de Liz (Fases 1-7) sobre localhost. Es el equivalente desktop de los
// archivos web/src/lib/api.ts + sse.ts del frontend React original.
type ClienteBackend struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string // opcional, futuro
	userAgent  string
}

// OpcionesCliente permite configurar el cliente.
type OpcionesCliente struct {
	BaseURL   string        // default http://localhost:3000
	Timeout   time.Duration // default 30s
	UserAgent string
}

// NuevoCliente crea un cliente del backend de Liz.
func NuevoCliente(opt OpcionesCliente) *ClienteBackend {
	if opt.BaseURL == "" {
		opt.BaseURL = "http://localhost:3000"
	}
	if opt.Timeout == 0 {
		opt.Timeout = 30 * time.Second
	}
	if opt.UserAgent == "" {
		opt.UserAgent = "LizDesktop/0.9.0"
	}
	return &ClienteBackend{
		baseURL: strings.TrimRight(opt.BaseURL, "/"),
		httpClient: &http.Client{
			Timeout: opt.Timeout,
		},
		userAgent: opt.UserAgent,
	}
}

// BaseURL devuelve la URL base configurada.
func (c *ClienteBackend) BaseURL() string { return c.baseURL }

// SetBaseURL permite cambiar la URL base (por ejemplo desde settings).
func (c *ClienteBackend) SetBaseURL(u string) {
	c.baseURL = strings.TrimRight(u, "/")
}

// ============================================================================
// Tipos espejo del backend (espejo de internal/pipeline/tipos.go y structs Go)
// ============================================================================

// SolicitudChat espeja pipeline.SolicitudChat.
type SolicitudChat struct {
	Mensaje   string `json:"mensaje"`
	UsuarioID string `json:"usuario_id,omitempty"`
	SesionID  string `json:"sesion_id,omitempty"`
	Proyecto  string `json:"proyecto,omitempty"`
	Stream    bool   `json:"stream,omitempty"`
}

// ChunkSSE espeja pipeline.ChunkStream. Llega como `data: {...}\n\n`.
type ChunkSSE struct {
	Tipo      string          `json:"tipo"`
	Contenido string          `json:"contenido"`
	Datos     json.RawMessage `json:"datos,omitempty"`
	PasoID    int             `json:"paso_id,omitempty"`
	Modelo    string          `json:"modelo,omitempty"`
	// Campos adicionales del chunk "completado":
	SesionID       string `json:"sesion_id,omitempty"`
	Tokens         int    `json:"tokens,omitempty"`
	DuracionMs     int64  `json:"duracion_ms,omitempty"`
	PasosEjecutados int   `json:"pasos_ejecutados,omitempty"`
	Error          string `json:"error,omitempty"`
}

// SesionChat espeja memoria.Sesion.
type SesionChat struct {
	ID         string    `json:"id"`
	UsuarioID  string    `json:"usuario_id"`
	Proyecto   string    `json:"proyecto,omitempty"`
	Titulo     string    `json:"titulo,omitempty"`
	Activa     bool      `json:"activa"`
	CreadaEn   time.Time `json:"creada_en"`
	CerradaEn  *time.Time `json:"cerrada_en,omitempty"`
}

// MensajeChat espeja memoria.Mensaje.
type MensajeChat struct {
	ID              string                 `json:"id"`
	SesionID        string                 `json:"sesion_id"`
	UsuarioID       string                 `json:"usuario_id"`
	Contenido       string                 `json:"contenido"`
	Rol             string                 `json:"rol"`
	Timestamp       time.Time              `json:"timestamp"`
	TokensEstimados int                    `json:"tokens_estimados"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// EstadoPipeline espeja pipeline.EstadoPipeline (respuesta GET /api/v1/chat).
type EstadoPipeline struct {
	MensajesProcesados int64            `json:"mensajes_procesados"`
	PromedioDuracion   string           `json:"promedio_duracion"`
	UltimoUso          string           `json:"ultimo_uso"`
	Categorias         map[string]int   `json:"categorias"`
	ModeloMasUsado     string           `json:"modelo_mas_usado"`
}

// ModeloOrquestador espeja orquestador.InfoModelo.
type ModeloOrquestador struct {
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion,omitempty"`
	TipoTarea   string `json:"tipo_tarea,omitempty"`
}

// ProyectoContexto espeja contexto.ProyectoInfo (versión simplificada).
type ProyectoContexto struct {
	Nombre      string `json:"nombre"`
	Ruta         string `json:"ruta,omitempty"`
	Archivos    int    `json:"archivos,omitempty"`
	TamañoBytes int64  `json:"tamano_bytes,omitempty"`
}

// HealthStatus espeja la respuesta de /api/v1/health.
type HealthStatus struct {
	Estado  string `json:"estado"`
	Version string `json:"version,omitempty"`
	Fase    string `json:"fase,omitempty"`
}

// RespuestaAPI envuelve todas las respuestas del backend Go.
type RespuestaAPI struct {
	Exito     bool            `json:"exito"`
	Mensaje   string          `json:"mensaje,omitempty"`
	Datos     json.RawMessage `json:"datos,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
}

// ============================================================================
// Helpers HTTP
// ============================================================================

func (c *ClienteBackend) doGet(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("conexión: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *ClienteBackend) doPost(ctx context.Context, path string, body interface{}, out interface{}) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("conexión: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *ClienteBackend) doDelete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("conexión: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// extraerDatos desempaqueta el campo `datos` de una RespuestaAPI.
func extraerDatos(r RespuestaAPI, dst interface{}) error {
	if !r.Exito {
		if r.Error != "" {
			return fmt.Errorf("API error: %s", r.Error)
		}
		return fmt.Errorf("API error: %s", r.Mensaje)
	}
	if len(r.Datos) == 0 || dst == nil {
		return nil
	}
	return json.Unmarshal(r.Datos, dst)
}

// ============================================================================
// Endpoints (espejo de web/src/lib/endpoints.ts)
// ============================================================================

// Health verifica el estado del backend.
func (c *ClienteBackend) Health(ctx context.Context) (*HealthStatus, error) {
	var r RespuestaAPI
	if err := c.doGet(ctx, "/api/v1/health", &r); err != nil {
		return nil, err
	}
	var h HealthStatus
	if err := extraerDatos(r, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// ListarSesiones devuelve las sesiones del usuario.
func (c *ClienteBackend) ListarSesiones(ctx context.Context, usuarioID string) ([]SesionChat, error) {
	if usuarioID == "" {
		usuarioID = "usuario_default"
	}
	var r RespuestaAPI
	if err := c.doGet(ctx, "/api/v1/chat/sesiones?usuario_id="+usuarioID, &r); err != nil {
		return nil, err
	}
	var s []SesionChat
	if err := extraerDatos(r, &s); err != nil {
		return nil, err
	}
	return s, nil
}

// CrearSesion crea una nueva sesión de chat.
func (c *ClienteBackend) CrearSesion(ctx context.Context, usuarioID, proyecto string) (*SesionChat, error) {
	var r RespuestaAPI
	if err := c.doPost(ctx, "/api/v1/chat/sesiones", map[string]string{
		"usuario_id": usuarioID,
		"proyecto":   proyecto,
	}, &r); err != nil {
		return nil, err
	}
	var s SesionChat
	if err := extraerDatos(r, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ObtenerSesion devuelve el detalle de una sesión con sus mensajes.
func (c *ClienteBackend) ObtenerSesion(ctx context.Context, id string) (*SesionChat, []MensajeChat, error) {
	var r RespuestaAPI
	if err := c.doGet(ctx, "/api/v1/chat/sesiones/"+id, &r); err != nil {
		return nil, nil, err
	}
	// La respuesta puede venir como struct con sesión + mensajes, o como objeto
	// con campos "sesion" y "mensajes". Intentamos ambos formatos.
	var doc struct {
		Sesion   *SesionChat   `json:"sesion"`
		Mensajes []MensajeChat `json:"mensajes"`
	}
	if err := json.Unmarshal(r.Datos, &doc); err == nil && doc.Sesion != nil {
		return doc.Sesion, doc.Mensajes, nil
	}
	// Fallback: solo la sesión
	var s SesionChat
	if err := json.Unmarshal(r.Datos, &s); err != nil {
		return nil, nil, err
	}
	return &s, nil, nil
}

// CerrarSesion cierra (elimina) una sesión.
func (c *ClienteBackend) CerrarSesion(ctx context.Context, id string) error {
	return c.doDelete(ctx, "/api/v1/chat/sesiones/"+id)
}

// EstadoChat devuelve las métricas del pipeline (GET /api/v1/chat).
func (c *ClienteBackend) EstadoChat(ctx context.Context) (*EstadoPipeline, error) {
	var r RespuestaAPI
	if err := c.doGet(ctx, "/api/v1/chat", &r); err != nil {
		return nil, err
	}
	var e EstadoPipeline
	if err := extraerDatos(r, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ListarModelos devuelve los modelos del orquestador NVIDIA.
func (c *ClienteBackend) ListarModelos(ctx context.Context) ([]ModeloOrquestador, error) {
	var r RespuestaAPI
	if err := c.doGet(ctx, "/api/v1/orquestador/modelos", &r); err != nil {
		return nil, err
	}
	var m []ModeloOrquestador
	if err := extraerDatos(r, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// ListarProyectos devuelve los proyectos de contexto indexados.
func (c *ClienteBackend) ListarProyectos(ctx context.Context) ([]ProyectoContexto, error) {
	var r RespuestaAPI
	if err := c.doGet(ctx, "/api/v1/contexto/proyectos", &r); err != nil {
		return nil, err
	}
	// Puede venir como []directo o como {proyectos: [...]}
	var p []ProyectoContexto
	if err := json.Unmarshal(r.Datos, &p); err == nil {
		return p, nil
	}
	var wrap struct {
		Proyectos []ProyectoContexto `json:"proyectos"`
	}
	if err := json.Unmarshal(r.Datos, &wrap); err == nil {
		return wrap.Proyectos, nil
	}
	if err := extraerDatos(r, &p); err != nil {
		return nil, err
	}
	return p, nil
}

// ============================================================================
// SSE Streaming (POST /api/v1/chat con stream=true)
// ============================================================================

// StreamChat envía un mensaje al pipeline y emite chunks SSE por el canal
// retornado. El canal se cierra cuando el stream termina (completado o error).
//
// Cancela el contexto para abortar el stream desde el lado del cliente.
func (c *ClienteBackend) StreamChat(ctx context.Context, sol SolicitudChat) (<-chan ChunkSSE, error) {
	sol.Stream = true
	buf, err := json.Marshal(sol)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/chat", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Para SSE no usamos el httpClient con timeout (cortaría el stream).
	sseClient := &http.Client{Timeout: 0}
	resp, err := sseClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("conexión SSE: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}

	ch := make(chan ChunkSSE, 32)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 4MB max por línea

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue // separador SSE
			}
			// SSE: "data: {...}\n\n"
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(line[5:])
			if len(payload) == 0 {
				continue
			}
			var chunk ChunkSSE
			if err := json.Unmarshal(payload, &chunk); err != nil {
				// Enviar chunk de error parseado pero no romper el stream
				ch <- ChunkSSE{Tipo: "error", Contenido: fmt.Sprintf("parse SSE: %v (raw: %s)", err, string(payload))}
				continue
			}
			ch <- chunk
			if chunk.Tipo == "completado" || chunk.Tipo == "error" {
				return
			}
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			ch <- ChunkSSE{Tipo: "error", Contenido: fmt.Sprintf("stream: %v", err)}
		}
	}()

	return ch, nil
}
