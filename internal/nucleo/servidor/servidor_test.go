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
        cfg.Servidor.Puerto = 0 // no importa para tests

        log := logger.NuevaConSalida("test", &buf)

        sisPermisos, err := permisos.NuevoSistema()
        if err != nil {
                t.Fatalf("error creando sistema permisos: %v", err)
        }

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
                t.Errorf("estado esperado 'operativo', obtuve '%v'", datos["estado"])
        }
        if datos["version"] != "0.1.0" {
                t.Errorf("versión esperada '0.1.0', obtuve '%v'", datos["version"])
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
                t.Errorf("tema esperado 'oscuro', obtuve '%v'", datos["tema"])
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

        // Solo verificar que la respuesta tiene estructura válida
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

func TestChatStub(t *testing.T) {
        srv, _ := setupTest(t)

        req := httptest.NewRequest("POST", "/api/chat", bytes.NewBufferString(`{"mensaje":"hola"}`))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()

        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotImplemented {
                t.Errorf("chat stub debería retornar 501, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        if resp["exito"] != false {
                t.Error("chat stub debería tener exito false")
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

        body := `{"puerto": 8080, "tema": "claro"}`
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
                t.Errorf("tema debería haber cambiado a 'claro', obtuve '%v'", datos["tema"])
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
