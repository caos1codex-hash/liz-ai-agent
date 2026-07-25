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

// ═══════════════════════════════════════════════════
// FASE 3: Contexto
// ═══════════════════════════════════════════════════

func setupTestConContexto(t *testing.T) (*Servidor, *permisos.Sistema, *contexto.Coordinador) {
        t.Helper()
        var buf bytes.Buffer
        cfg := config.ConfiguracionPorDefecto()
        cfg.Servidor.Puerto = 0

        log := logger.NuevaConSalida("test", &buf)

        sisPermisos, err := permisos.NuevoSistemaConRecordar(false)
        if err != nil {
                t.Fatalf("error creando sistema permisos: %v", err)
        }

        tmpDir := t.TempDir()
        coord, err := contexto.NuevoCoordinador(filepath.Join(tmpDir, "proyectos"))
        if err != nil {
                t.Fatalf("error creando coordinador: %v", err)
        }

        srv := NuevoConContexto(&config.Gestor{
                rutaActiva: filepath.Join(tmpDir, "config.json"),
                config:     cfg,
                logFunc:    func(string, ...interface{}) {},
        }, log, sisPermisos, coord)
        return srv, sisPermisos, coord
}

func setupTestConCoordinador(t *testing.T) (*Servidor, *contexto.Coordinador) {
        srv, _, coord := setupTestConContexto(t)
        return srv, coord
}

func TestContexto_ListarProyectos(t *testing.T) {
        srv, _ := setupTestConCoordinador(t)

        req := httptest.NewRequest("GET", "/api/contexto/proyectos", nil)
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
        if datos["total"] == nil {
                t.Error("datos deberían incluir total")
        }
}

func TestContexto_Indexar(t *testing.T) {
        srv, coord := setupTestConCoordinador(t)

        // Crear un proyecto de prueba
        tmpDir := t.TempDir()
        os.MkdirAll(filepath.Join(tmpDir, "cmd"), 0755)
        os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() {}"), 0644)
        os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644)

        body := fmt.Sprintf(`{"ruta": "%s"}`, tmpDir)
        req := httptest.NewRequest("POST", "/api/contexto/indexar", bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("status esperado 200, obtuve %d. Body: %s", rec.Code, rec.Body.String())
        }

        resp := decodeResponse(t, rec)
        if resp["exito"] != true {
                t.Errorf("indexación falló: %v", resp["error"])
        }

        datos := resp["datos"].(map[string]interface{})
        if datos["total_archivos"] == nil {
                t.Error("respuesta debería incluir total_archivos")
        }

        // Verificar que se puede obtener el mapa
        nombreProyecto := filepath.Base(tmpDir)
        req2 := httptest.NewRequest("GET", "/api/contexto/mapa/"+nombreProyecto, nil)
        rec2 := httptest.NewRecorder()
        srv.router.ServeHTTP(rec2, req2)

        if rec2.Code != http.StatusOK {
                t.Errorf("mapa debería ser accesible, status: %d. Body: %s", rec2.Code, rec2.Body.String())
        }

        _ = coord
}

func TestContexto_Indexar_SinRuta(t *testing.T) {
        srv, _ := setupTestConCoordinador(t)

        body := `{}`
        req := httptest.NewRequest("POST", "/api/contexto/indexar", bytes.NewBufferString(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("sin ruta debería ser 400, obtuve %d", rec.Code)
        }
}

func TestContexto_Buscar(t *testing.T) {
        srv, coord := setupTestConCoordinador(t)

        // Crear e indexar un proyecto
        tmpDir := t.TempDir()
        os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
        os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644)
        coord.IndexarProyecto(tmpDir)

        nombreProyecto := filepath.Base(tmpDir)
        req := httptest.NewRequest("GET", "/api/contexto/buscar/"+nombreProyecto+"?q=go", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("buscar debería ser 200, obtuve %d", rec.Code)
        }

        resp := decodeResponse(t, rec)
        datos := resp["datos"].(map[string]interface{})
        total := datos["total"].(float64)
        if total == 0 {
                t.Error("búsqueda de 'go' debería encontrar resultados")
        }
}

func TestContexto_MapaNoExistente(t *testing.T) {
        srv, _ := setupTestConCoordinador(t)

        req := httptest.NewRequest("GET", "/api/contexto/mapa/proyecto_no_existente", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotFound {
                t.Errorf("mapa inexistente debería ser 404, obtuve %d", rec.Code)
        }
}

func TestContexto_BuscarSinQuery(t *testing.T) {
        srv, _ := setupTestConCoordinador(t)

        req := httptest.NewRequest("GET", "/api/contexto/buscar/mi_proyecto", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("sin parámetro 'q' debería ser 400, obtuve %d", rec.Code)
        }
}