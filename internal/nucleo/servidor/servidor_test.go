package servidor

import (
        "bytes"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
)

func setupTest(t *testing.T) (*Servidor, *permisos.Sistema) {
        t.Helper()
        var buf bytes.Buffer
        cfg := config.ConfiguracionPorDefecto()
        cfg.Servidor.Puerto = 0

        log := logger.NuevaConSalida("test", &buf)

        sisPermisos, err := permisos.NuevoSistemaConRecordar(false)
        if err != nil {
                t.Fatalf("error creando sistema permisos: %v", err)
        }

        // Config global para legacy mode en tests
        config.Config = cfg

        srv := Nuevo(cfg, log, sisPermisos)
        return srv, sisPermisos
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
        t.Helper()
        var resp map[string]interface{}
        if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
                t.Fatalf("error decodificando respuesta: %v", err)
        }
        return resp
}

func TestHealthEndpoint(t *testing.T) {
        srv, _ := setupTest(t)

        req := httptest.NewRequest("GET", "/api/health", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("status esperado 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        datos := resp["datos"].(map[string]interface{})
        if datos["estado"] != "operativo" {
                t.Errorf("estado esperado 'operativo'")
        }
        if datos["go_version"] == "" {
                t.Error("go_version no debería estar vacío")
        }
}

func TestGetConfig(t *testing.T) {
        srv, _ := setupTest(t)

        req := httptest.NewRequest("GET", "/api/config", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("status esperado 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        datos := resp["datos"].(map[string]interface{})
        if datos["tema"] != "oscuro" {
                t.Errorf("tema esperado 'oscuro'")
        }
}

func TestPermisosGet(t *testing.T) {
        srv, _ := setupTest(t)

        req := httptest.NewRequest("GET", "/api/permisos", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("status esperado 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        datos := resp["datos"].(map[string]interface{})
        if datos["permisos"] == nil {
                t.Error("datos deberían incluir permisos")
        }
}

func TestPermisosPost(t *testing.T) {
        srv, _ := setupTest(t)

        body := `{"conceder_todos": true}`
        req := httptest.NewRequest("POST", "/api/permisos", bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("status esperado 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        datos := resp["datos"].(map[string]interface{})
        if datos["concedidos"] != true {
                t.Error("después de conceder_todos, concedidos debería ser true")
        }
}

// ═══════════════════════════════════════════════════
// FASE 2: DELETE permisos y auditoría
// ═══════════════════════════════════════════════════

func TestPermisosDelete(t *testing.T) {
        srv, _ := setupTest(t)

        // Primero conceder
        srv.permisos.ConcederTodos("test_session")

        // Ahora resetear via DELETE
        req := httptest.NewRequest("DELETE", "/api/permisos", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("DELETE /api/permisos debería ser 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        if resp["exito"] != true {
                t.Error("exito debería ser true después de delete")
        }
}

func TestAuditoriaEndpoint(t *testing.T) {
        srv, _ := setupTest(t)

        req := httptest.NewRequest("GET", "/api/permisos/auditoria", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("auditoría debería ser 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        datos := resp["datos"].(map[string]interface{})
        if datos["total"] == nil {
                t.Error("datos deberían incluir total")
        }
}

func TestChatStub_PermisoDenegado(t *testing.T) {
        srv, _ := setupTest(t)

        // Sin permisos, POST /api/chat debería ser 403 (middleware)
        body := `{"mensaje":"hola"}`
        req := httptest.NewRequest("POST", "/api/chat", bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusForbidden {
                t.Errorf("sin permisos, POST /api/chat debería ser 403, obtuve %d", rec.Code)
        }
}

func TestChatStub_PermisoConcedido(t *testing.T) {
        srv, _ := setupTest(t)
        srv.permisos.ConcederTodos("test_session")

        // Con permisos, debería pasar el middleware y llegar al stub 501
        body := `{"mensaje":"hola"}`
        req := httptest.NewRequest("POST", "/api/chat", bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotImplemented {
                t.Errorf("con permisos, debería llegar al stub 501, obtuve %d", rec.Code)
        }
}

func TestCORSMiddleware(t *testing.T) {
        srv, _ := setupTest(t)

        req := httptest.NewRequest("OPTIONS", "/api/health", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("CORS preflight debería retornar 200, obtuve %d", rec.Code)
        }

        cors := rec.Header().Get("Access-Control-Allow-Origin")
        if cors != "*" {
                t.Errorf("CORS origin esperado '*', obtuve '%s'", cors)
        }
}

func TestPutConfig(t *testing.T) {
        srv, _ := setupTest(t)

        body := `{"tema": "claro"}`
        req := httptest.NewRequest("PUT", "/api/config", bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("status esperado 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        datos := resp["datos"].(map[string]interface{})
        if datos["tema"] != "claro" {
                t.Errorf("tema debería haber cambiado a 'claro'")
        }
}

func TestPutConfigPuertoInvalido(t *testing.T) {
        srv, _ := setupTest(t)

        body := `{"puerto": 99999}`
        req := httptest.NewRequest("PUT", "/api/config", bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("puerto inválido debería retornar 400, obtuve %d", rec.Code)
        }
}

// Health ahora incluye permisos_listos
func TestHealth_PermisosListos(t *testing.T) {
        srv, _ := setupTest(t)

        req := httptest.NewRequest("GET", "/api/health", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        resp := decodeResponse(t, rec)
        datos := resp["datos"].(map[string]interface{})
        if datos["permisos_listos"] == nil {
                t.Error("health debería incluir permisos_listos")
        }
}