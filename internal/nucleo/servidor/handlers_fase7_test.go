package servidor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caos1codex-hash/liz-ai-agent/internal/pipeline"
)

// ============================================================================
// Tests Fase 7 — Handlers del Pipeline de Chat
// ============================================================================

func TestHandlerChatGet_SinPipeline(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	req := httptest.NewRequest("GET", "/api/v1/chat", nil)
	w := httptest.NewRecorder()

	srv.handlerChatGet(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("esperaba 503, got %d", resp.StatusCode)
	}
}

func TestHandlerChatPost_SinPipeline(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	body := `{"mensaje": "hola liz"}`
	req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handlerChatPost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("esperaba 503, got %d", resp.StatusCode)
	}
}

func TestHandlerChatPost_BadRequest(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	srv.pipelineMgr = pipeline.Nuevo(pipeline.NuevasOpciones{})

	// Sin body
	req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handlerChatPost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("esperaba 400, got %d", resp.StatusCode)
	}

	// Mensaje vacío
	body := `{"mensaje": ""}`
	req = httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	srv.handlerChatPost(w, req)

	resp = w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("esperaba 400, got %d", resp.StatusCode)
	}
}

func TestHandlerChatPost_ConPipeline(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	srv.pipelineMgr = pipeline.Nuevo(pipeline.NuevasOpciones{})

	body := `{"mensaje": "hola liz, ¿qué puedes hacer?"}`
	req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handlerChatPost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("esperaba 200, got %d", resp.StatusCode)
	}

	var apiResp RespuestaAPI
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("error decodificando respuesta: %v", err)
	}
	if !apiResp.Exito {
		t.Errorf("esperaba éxito, got error: %s", apiResp.Error)
	}
}

func TestHandlerChatPost_SSE(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	srv.pipelineMgr = pipeline.Nuevo(pipeline.NuevasOpciones{})

	body := `{"mensaje": "hola liz", "stream": true}`
	req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handlerChatPost(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("esperaba 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("esperaba text/event-stream, got %s", contentType)
	}
}

func TestHandlerChatSesiones_SinMemoria(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	req := httptest.NewRequest("GET", "/api/v1/chat/sesiones", nil)
	w := httptest.NewRecorder()

	srv.handlerChatSesiones(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("esperaba 503, got %d", resp.StatusCode)
	}
}

func TestHandlerChatMetricas_SinPipeline(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	req := httptest.NewRequest("GET", "/api/v1/chat/metricas", nil)
	w := httptest.NewRecorder()

	srv.handlerChatMetricas(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("esperaba 503, got %d", resp.StatusCode)
	}
}

func TestHandlerChatMetricas_ConPipeline(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	srv.pipelineMgr = pipeline.Nuevo(pipeline.NuevasOpciones{})

	// Procesar un mensaje para tener métricas
	_, _ = srv.pipelineMgr.Procesar(nil, &pipeline.SolicitudChat{Mensaje: "test"})

	req := httptest.NewRequest("GET", "/api/v1/chat/metricas", nil)
	w := httptest.NewRecorder()

	srv.handlerChatMetricas(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("esperaba 200, got %d", resp.StatusCode)
	}

	var apiResp RespuestaAPI
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		t.Fatalf("error decodificando: %v", err)
	}
	if !apiResp.Exito {
		t.Errorf("esperaba éxito: %s", apiResp.Error)
	}
}
