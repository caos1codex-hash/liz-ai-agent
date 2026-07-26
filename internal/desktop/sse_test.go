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

// TestClienteBackend_StreamChat verifica que el cliente SSE:
//  1. Envíe POST con stream=true y Accept: text/event-stream
//  2. Parse correctamente los chunks `data: {...}\n\n`
//  3. Acumule tipos: estado, herramienta, texto, completado
//  4. Cierre el canal al recibir "completado"
func TestClienteBackend_StreamChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, quiero POST", r.Method)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Accept = %q, quiero text/event-stream", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"tipo\":\"estado\",\"contenido\":\"Iniciando pipeline...\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"tipo\":\"herramienta\",\"contenido\":\"buscador\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"tipo\":\"texto\",\"contenido\":\"Encontré 5 archivos\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"tipo\":\"texto\",\"contenido\":\". Eliminando...\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"tipo\":\"completado\",\"sesion_id\":\"s-99\",\"modelo\":\"mixtral\",\"tokens\":42,\"duracion_ms\":1500,\"pasos_ejecutados\":2}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	ch, err := c.StreamChat(context.Background(), SolicitudChat{Mensaje: "test"})
	if err != nil {
		t.Fatalf("StreamChat error: %v", err)
	}

	var chunks []ChunkSSE
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 5 {
		t.Fatalf("esperaba 5 chunks, obtuve %d", len(chunks))
	}

	if chunks[0].Tipo != "estado" || chunks[0].Contenido != "Iniciando pipeline..." {
		t.Errorf("chunk[0] inesperado: %+v", chunks[0])
	}
	if chunks[1].Tipo != "herramienta" || chunks[1].Contenido != "buscador" {
		t.Errorf("chunk[1] inesperado: %+v", chunks[1])
	}
	if chunks[2].Tipo != "texto" || chunks[2].Contenido != "Encontré 5 archivos" {
		t.Errorf("chunk[2] inesperado: %+v", chunks[2])
	}
	if chunks[3].Tipo != "texto" || chunks[3].Contenido != ". Eliminando..." {
		t.Errorf("chunk[3] inesperado: %+v", chunks[3])
	}
	if chunks[4].Tipo != "completado" {
		t.Errorf("chunk[4] tipo = %q, quiero completado", chunks[4].Tipo)
	}
	if chunks[4].SesionID != "s-99" {
		t.Errorf("chunk[4].SesionID = %q, quiero s-99", chunks[4].SesionID)
	}
	if chunks[4].Modelo != "mixtral" {
		t.Errorf("chunk[4].Modelo = %q, quiero mixtral", chunks[4].Modelo)
	}
	if chunks[4].Tokens != 42 {
		t.Errorf("chunk[4].Tokens = %d, quiero 42", chunks[4].Tokens)
	}
}

// TestClienteBackend_StreamChatError verifica que el canal se cierre y emita
// un chunk de error cuando el backend envía tipo=error.
func TestClienteBackend_StreamChatError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"tipo\":\"estado\",\"contenido\":\"Iniciando...\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"tipo\":\"error\",\"contenido\":\"API key inválida\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	ch, err := c.StreamChat(context.Background(), SolicitudChat{Mensaje: "test"})
	if err != nil {
		t.Fatalf("StreamChat error: %v", err)
	}

	var chunks []ChunkSSE
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 2 {
		t.Fatalf("esperaba 2 chunks, obtuve %d", len(chunks))
	}
	if chunks[1].Tipo != "error" {
		t.Errorf("chunk[1].Tipo = %q, quiero error", chunks[1].Tipo)
	}
	if chunks[1].Contenido != "API key inválida" {
		t.Errorf("chunk[1].Contenido = %q", chunks[1].Contenido)
	}
}

// TestClienteBackend_StreamChatCancel verifica que cancelar el contexto
// aborte el stream y cierre el canal.
func TestClienteBackend_StreamChatCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		// Enviar un chunk y luego bloquear hasta que el cliente cancele
		fmt.Fprint(w, "data: {\"tipo\":\"estado\",\"contenido\":\"Iniciando...\"}\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	ch, err := c.StreamChat(ctx, SolicitudChat{Mensaje: "test"})
	if err != nil {
		t.Fatalf("StreamChat error: %v", err)
	}

	// Recibir primer chunk, luego cancelar
	chunk := <-ch
	if chunk.Tipo != "estado" {
		t.Errorf("primer chunk tipo = %q, quiero estado", chunk.Tipo)
	}

	cancel()

	// Drenar el canal hasta que se cierre. Puede recibir un chunk de error
	// extra del scanner antes de cerrarse.
	timeout := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // OK: canal cerrado
			}
			// continuar consumiendo
		case <-timeout:
			t.Fatal("timeout esperando cierre del canal")
		}
	}
}

// TestClienteBackend_StreamChatHTTPError verifica que errores HTTP
// (no 200) se reporten como error de retorno.
func TestClienteBackend_StreamChatHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	_, err := c.StreamChat(context.Background(), SolicitudChat{Mensaje: "test"})
	if err == nil {
		t.Fatal("esperaba error, obtuve nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error no contiene 400: %v", err)
	}
}

// TestClienteBackend_StreamChatForzaStream verifica que el cliente fuerce
// Stream=true en la solicitud aunque el caller no lo haya seteado.
func TestClienteBackend_StreamChatForzaStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decodificar body y verificar que stream=true
		var req SolicitudChat
		_ = json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Errorf("Stream = false, debería ser true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"tipo\":\"completado\",\"sesion_id\":\"s-x\"}\n\n")
	}))
	defer srv.Close()

	c := NuevoCliente(OpcionesCliente{BaseURL: srv.URL})
	ch, err := c.StreamChat(context.Background(), SolicitudChat{Mensaje: "hola"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	for range ch {
	}
}
