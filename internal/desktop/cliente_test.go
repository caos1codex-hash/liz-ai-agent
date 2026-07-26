package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestClienteBackend_Health verifica que el cliente haga GET /api/v1/health
// correctamente y parsea la respuesta.
func TestClienteBackend_Health(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("method inesperado: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"exito":true,"datos":{"estado":"ok","version":"0.9.0","fase":"8"}}`)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL, Timeout: 2 * time.Second})
	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error: %v", err)
	}
	if h.Estado != "ok" {
		t.Errorf("estado = %q, quiero %q", h.Estado, "ok")
	}
	if h.Version != "0.9.0" {
		t.Errorf("version = %q, quiero %q", h.Version, "0.9.0")
	}
}

// TestClienteBackend_HealthError verifica el manejo de errores HTTP.
func TestClienteBackend_HealthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL, Timeout: 2 * time.Second})
	_, err := c.Health(context.Background())
	if err == nil {
		t.Fatal("esperaba error, obtuvo nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error no contiene código 500: %v", err)
	}
}

// TestClienteBackend_ListarSesiones verifica el listado de sesiones.
func TestClienteBackend_ListarSesiones(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat/sesiones" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		if r.URL.Query().Get("usuario_id") != "user1" {
			t.Errorf("usuario_id inesperado: %s", r.URL.Query().Get("usuario_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"exito":true,"datos":[{"id":"s1","usuario_id":"user1","titulo":"Conv 1","activa":true,"creada_en":"2026-01-01T00:00:00Z"}]}`)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL, Timeout: 2 * time.Second})
	sesiones, err := c.ListarSesiones(context.Background(), "user1")
	if err != nil {
		t.Fatalf("ListarSesiones error: %v", err)
	}
	if len(sesiones) != 1 {
		t.Fatalf("esperaba 1 sesión, obtuve %d", len(sesiones))
	}
	if sesiones[0].ID != "s1" {
		t.Errorf("ID = %q, quiero %q", sesiones[0].ID, "s1")
	}
	if sesiones[0].Titulo != "Conv 1" {
		t.Errorf("Titulo = %q, quiero %q", sesiones[0].Titulo, "Conv 1")
	}
}

// TestClienteBackend_ListarSesionesDefaultUsuario verifica que use
// "usuario_default" cuando no se especifica usuarioID.
func TestClienteBackend_ListarSesionesDefaultUsuario(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("usuario_id") != "usuario_default" {
			t.Errorf("esperaba usuario_default, obtuve %q", r.URL.Query().Get("usuario_id"))
		}
		fmt.Fprint(w, `{"exito":true,"datos":[]}`)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	if _, err := c.ListarSesiones(context.Background(), ""); err != nil {
		t.Fatalf("error: %v", err)
	}
}

// TestClienteBackend_CrearSesion verifica la creación de sesión.
func TestClienteBackend_CrearSesion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method inesperado: %s", r.Method)
		}
		var req struct {
			UsuarioID string `json:"usuario_id"`
			Proyecto  string `json:"proyecto"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if req.UsuarioID != "user1" {
			t.Errorf("usuario_id = %q", req.UsuarioID)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"exito":true,"datos":{"id":"s2","usuario_id":"user1","proyecto":"repo-x","activa":true,"creada_en":"2026-01-01T00:00:00Z"}}`)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	ses, err := c.CrearSesion(context.Background(), "user1", "repo-x")
	if err != nil {
		t.Fatalf("CrearSesion error: %v", err)
	}
	if ses.ID != "s2" {
		t.Errorf("ID = %q, quiero %q", ses.ID, "s2")
	}
	if ses.Proyecto != "repo-x" {
		t.Errorf("Proyecto = %q, quiero %q", ses.Proyecto, "repo-x")
	}
}

// TestClienteBackend_CerrarSesion verifica el DELETE.
func TestClienteBackend_CerrarSesion(t *testing.T) {
	deleted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method inesperado: %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/sesiones/s3") {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		deleted = true
		fmt.Fprint(w, `{"exito":true}`)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	if err := c.CerrarSesion(context.Background(), "s3"); err != nil {
		t.Fatalf("CerrarSesion error: %v", err)
	}
	if !deleted {
		t.Error("DELETE no fue llamado")
	}
}

// TestClienteBackend_EstadoChat verifica las métricas del pipeline.
func TestClienteBackend_EstadoChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/chat" {
			t.Errorf("path inesperado: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"exito":true,"datos":{"mensajes_procesados":42,"promedio_duracion":"1.5s","ultimo_uso":"2026-01-01T12:00:00Z","categorias":{"codigo":10,"sistema":5},"modelo_mas_usado":"mixtral"}}`)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	est, err := c.EstadoChat(context.Background())
	if err != nil {
		t.Fatalf("EstadoChat error: %v", err)
	}
	if est.MensajesProcesados != 42 {
		t.Errorf("MensajesProcesados = %d, quiero 42", est.MensajesProcesados)
	}
	if est.ModeloMasUsado != "mixtral" {
		t.Errorf("ModeloMasUsado = %q, quiero %q", est.ModeloMasUsado, "mixtral")
	}
	if est.Categorias["codigo"] != 10 {
		t.Errorf("Categorias[codigo] = %d, quiero 10", est.Categorias["codigo"])
	}
}

// TestClienteBackend_ListarProyectos verifica el endpoint de proyectos,
// probando los 3 formatos posibles de respuesta (array directo, wrap, exito+datos).
func TestClienteBackend_ListarProyectos(t *testing.T) {
	casos := []struct {
		nombre string
		body   string
		want   int
	}{
		{
			nombre: "array directo",
			body:   `{"exito":true,"datos":[{"nombre":"repo1","ruta":"/tmp/repo1","archivos":10}]}`,
			want:   1,
		},
		{
			nombre: "wrap en proyectos",
			body:   `{"exito":true,"datos":{"proyectos":[{"nombre":"repo1"},{"nombre":"repo2"}]}}`,
			want:   2,
		},
		{
			nombre: "vacío",
			body:   `{"exito":true,"datos":[]}`,
			want:   0,
		},
	}
	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()

			c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
			ps, err := c.ListarProyectos(context.Background())
			if err != nil {
				t.Fatalf("ListarProyectos error: %v", err)
			}
			if len(ps) != tc.want {
				t.Errorf("len = %d, quiero %d", len(ps), tc.want)
			}
		})
	}
}

// TestClienteBackend_Defaults verifica los valores por defecto del cliente.
func TestClienteBackend_Defaults(t *testing.T) {
	c := NuevoCliente(OpcionesCliente{})
	if c.BaseURL() != "http://localhost:3000" {
		t.Errorf("BaseURL default = %q, quiero http://localhost:3000", c.BaseURL())
	}
}

// TestClienteBackend_SetBaseURL verifica que SetBaseURL funcione.
func TestClienteBackend_SetBaseURL(t *testing.T) {
	c := NuevoCliente(OpcionesCliente{})
	c.SetBaseURL("http://example.com:8080/")
	if c.BaseURL() != "http://example.com:8080" {
		t.Errorf("BaseURL = %q, quiero http://example.com:8080", c.BaseURL())
	}
}
