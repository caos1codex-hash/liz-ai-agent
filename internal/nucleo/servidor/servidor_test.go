package servidor

import (
        "encoding/json"
        "io"
        "net/http"
        "net/http/httptest"
        "os"
        "path/filepath"
        "strings"
        "testing"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
)

// ============================================================================
// Setup Helper
// ============================================================================

func setupTestServidor(t *testing.T) (*Servidor, *config.Gestor, *permisos.Gestor) {
        t.Helper()

        tmpDir := t.TempDir()
        rutaCfg := filepath.Join(tmpDir, "config.yaml")
        os.WriteFile(rutaCfg, []byte("liz:\n  puerto: 8080\n  nombre: Liz\n  version: 0.2.0\n"), 0644)

        log := logger.NuevaConSalida("test", io.Discard)
        gestorCfg, err := config.Inicializar(rutaCfg)
        if err != nil {
                t.Fatalf("Error al inicializar config: %v", err)
        }

        gestorPer, err := permisos.Inicializar(tmpDir)
        if err != nil {
                t.Fatalf("Error al inicializar permisos: %v", err)
        }

        srv := Nuevo(gestorCfg, gestorPer, log)
        return srv, gestorCfg, gestorPer
}

func parseRespuesta(t *testing.T, body []byte) map[string]interface{} {
        t.Helper()
        var result map[string]interface{}
        if err := json.Unmarshal(body, &result); err != nil {
                t.Fatalf("Error al parsear JSON: %v (body: %s)", err, string(body))
        }
        return result
}

// ============================================================================
// Tests de Health
// ============================================================================

func TestHandlerHealth(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/health", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d", rec.Code, http.StatusOK)
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatal("datos no es un mapa")
        }
        if datos["estado"] != "operativo" {
                t.Errorf("estado = %v, se esperaba operativo", datos["estado"])
        }
        if datos["version"] != "0.2.0" {
                t.Errorf("version = %v, se esperada 0.2.0", datos["version"])
        }
}

// ============================================================================
// Tests de Config — GET
// ============================================================================

func TestHandlerConfigGet(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/config", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d", rec.Code, http.StatusOK)
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatal("datos no es un mapa")
        }
        if datos["puerto"] != float64(8080) {
                t.Errorf("puerto = %v, se esperaba 8080", datos["puerto"])
        }
        if datos["nombre"] != "Liz" {
                t.Errorf("nombre = %v, se esperaba Liz", datos["nombre"])
        }
}

func TestHandlerConfigGet_SanitizeAPIKeys(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/config", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        resp := parseRespuesta(t, rec.Body.Bytes())
        datos := resp["datos"].(map[string]interface{})

        modelos, ok := datos["modelos"].([]interface{})
        if !ok || len(modelos) == 0 {
                t.Skip("No hay modelos para verificar sanitización")
        }

        // Los modelos del config por defecto no tienen API key, pero si la tuvieran
        // deberían aparecer como "***"
        bodyStr := rec.Body.String()
        if strings.Contains(bodyStr, "sk-") || strings.Contains(bodyStr, "nvapi-") {
                t.Error("La respuesta no debería contener API keys reales")
        }
}

// ============================================================================
// Tests de Config — PUT
// ============================================================================

func TestHandlerConfigPut_CambioSimple(t *testing.T) {
        srv, gestorCfg, _ := setupTestServidor(t)

        body := `{"campos": {"puerto": "9090"}}`
        req := httptest.NewRequest("PUT", "/api/v1/config", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        if gestorCfg.ObtenerPuerto() != 9090 {
                t.Errorf("Puerto = %d, se esperaba 9090 después del cambio", gestorCfg.ObtenerPuerto())
        }
}

func TestHandlerConfigPut_ValidacionFalla(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        body := `{"campos": {"puerto": "99999"}}`
        req := httptest.NewRequest("PUT", "/api/v1/config", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba %d para puerto inválido", rec.Code, http.StatusBadRequest)
        }
}

func TestHandlerConfigPut_SinCampos(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        body := `{"campos": {}}`
        req := httptest.NewRequest("PUT", "/api/v1/config", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba %d para campos vacíos", rec.Code, http.StatusBadRequest)
        }
}

func TestHandlerConfigPut_JSONInvalido(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        body := `no es json`
        req := httptest.NewRequest("PUT", "/api/v1/config", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba %d para JSON inválido", rec.Code, http.StatusBadRequest)
        }
}

// ============================================================================
// Tests de Config — Esquema
// ============================================================================

func TestHandlerConfigEsquema(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/config/esquema", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperada %d", rec.Code, http.StatusOK)
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        esquema, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatal("datos no contiene esquema")
        }

        campos, ok := esquema["campos"].([]interface{})
        if !ok || len(campos) == 0 {
                t.Error("El esquema debería tener campos")
        }
}

// ============================================================================
// Tests de Config — Cambios
// ============================================================================

func TestHandlerConfigCambios(t *testing.T) {
        srv, gestorCfg, _ := setupTestServidor(t)

        // Hacer un cambio primero
        gestorCfg.Establecer("puerto", "5555")

        req := httptest.NewRequest("GET", "/api/v1/config/cambios", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d", rec.Code, http.StatusOK)
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        cambios, ok := resp["datos"].([]interface{})
        if !ok || len(cambios) == 0 {
                t.Error("Debería haber cambios registrados")
        }
}

// ============================================================================
// Tests de Config — Recargar
// ============================================================================

func TestHandlerConfigRecargar(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("POST", "/api/v1/config/recargar", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }
}

// ============================================================================
// Tests de Permisos — GET
// ============================================================================

func TestHandlerPermisosGet(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/permisos", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d", rec.Code, http.StatusOK)
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatal("datos no es un mapa")
        }

        perms, ok := datos["permisos"].([]interface{})
        if !ok || len(perms) == 0 {
                t.Error("Debería haber permisos en la respuesta")
        }
}

// ============================================================================
// Tests de Permisos — POST
// ============================================================================

func TestHandlerPermisosPost_Valido(t *testing.T) {
        srv, _, gestorPer := setupTestServidor(t)

        body := `{"tipo": "archivos", "nivel": "lectura", "razon": "test"}`
        req := httptest.NewRequest("POST", "/api/v1/permisos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
        }

        reg := gestorPer.ObtenerPermiso(permisos.PermArchivos)
        if reg == nil {
                t.Fatal("Permiso no encontrado después de POST")
        }
        if reg.Nivel != permisos.NivelLectura {
                t.Errorf("Nivel = %s, se esperaba lectura", reg.Nivel)
        }
}

func TestHandlerPermisosPost_TipoInvalido(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        body := `{"tipo": "tipo_inventado"}`
        req := httptest.NewRequest("POST", "/api/v1/permisos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba %d para tipo inválido", rec.Code, http.StatusBadRequest)
        }
}

func TestHandlerPermisosPost_SinTipo(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        body := `{"nivel": "total"}`
        req := httptest.NewRequest("POST", "/api/v1/permisos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba %d sin tipo", rec.Code, http.StatusBadRequest)
        }
}

// ============================================================================
// Tests de Permisos — Resumen
// ============================================================================

func TestHandlerPermisosResumen(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/permisos/resumen", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d", rec.Code, http.StatusOK)
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatal("datos no es un mapa")
        }
        if datos["total"] == nil {
                t.Error("resumen debería tener 'total'")
        }
}

// ============================================================================
// Tests de Permisos — Auditoría
// ============================================================================

func TestHandlerPermisosAuditoria(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/permisos/auditoria", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba %d", rec.Code, http.StatusOK)
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        _, ok := resp["datos"].([]interface{})
        if !ok {
                t.Error("datos debería ser un array de auditoría")
        }
}

// ============================================================================
// Tests de Stubs
// ============================================================================

func TestHandlerStub_NotImplemented(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        endpoints := []string{"/api/v1/tools", "/api/v1/orquestador", "/api/v1/modelos",
                "/api/v1/conversations"}

        for _, ep := range endpoints {
                req := httptest.NewRequest("GET", ep, nil)
                rec := httptest.NewRecorder()
                srv.router.ServeHTTP(rec, req)

                if rec.Code != http.StatusNotImplemented {
                        t.Errorf("GET %s: Status = %d, se esperaba %d", ep, rec.Code, http.StatusNotImplemented)
                }

                resp := parseRespuesta(t, rec.Body.Bytes())
                if resp["exito"] != false {
                        t.Errorf("GET %s: exito debería ser false", ep)
                }
        }
}

func TestHandlerStub_Chat_NotImplemented(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        body := `{"mensaje": "hola"}`
        req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotImplemented {
                t.Errorf("Status = %d, se esperaba %d", rec.Code, http.StatusNotImplemented)
        }
}

// ============================================================================
// Tests de CORS
// ============================================================================

func TestMiddlewareCORS_Headers(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/health", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
                t.Error("CORS: Access-Control-Allow-Origin debería ser *")
        }
}

func TestMiddlewareCORS_OPTIONS(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("OPTIONS", "/api/v1/health", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("OPTIONS Status = %d, se esperaba %d", rec.Code, http.StatusOK)
        }
}

// ============================================================================
// Tests de 404
// ============================================================================

func TestRutaNoExistente(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/no-existe", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotFound {
                t.Errorf("Status = %d, se esperaba %d para ruta inexistente", rec.Code, http.StatusNotFound)
        }
}