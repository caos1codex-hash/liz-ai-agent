package servidor

import (
        "encoding/json"
        "fmt"
        "io"
        "net/http"
        "net/http/httptest"
        "os"
        "path/filepath"
        "strings"
        "testing"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto"
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

        // /api/v1/orquestador ya NO es un stub: ahora es un handler real que
        // responde 503 si el orquestador no está inyectado. Se testea aparte.
        endpoints := []string{"/api/v1/tools", "/api/v1/modelos",
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

// TestOrquestador_SinInyectar_Responde503 verifica que los endpoints del
// orquestador respondan 503 cuando no se ha inyectado (Fase 4 opcional).
func TestOrquestador_SinInyectar_Responde503(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        for _, ep := range []string{"/api/v1/orquestador", "/api/v1/orquestador/modelos", "/api/v1/orquestador/metricas"} {
                req := httptest.NewRequest("GET", ep, nil)
                rec := httptest.NewRecorder()
                srv.router.ServeHTTP(rec, req)

                if rec.Code != http.StatusServiceUnavailable {
                        t.Errorf("GET %s: Status = %d, se esperaba 503 (orquestador no inyectado)", ep, rec.Code)
                }
        }
}

// TestMemoria_SinInyectar_Responde503 verifica que los endpoints de memoria
// respondan 503 cuando no se ha inyectado el gestor.
func TestMemoria_SinInyectar_Responde503(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/memoria/sesiones?usuario_id=x", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusServiceUnavailable {
                t.Errorf("GET /api/v1/memoria/sesiones: Status = %d, se esperaba 503", rec.Code)
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
// ============================================================================
// Tests de Endpoints de Contexto (Fase 3)
// ============================================================================

// setupTestServidorConContexto crea un servidor con coordinador de contexto inyectado.
func setupTestServidorConContexto(t *testing.T) (*Servidor, *contexto.Coordinador, string) {
        t.Helper()

        tmpDir := t.TempDir()
        rutaCfg := filepath.Join(tmpDir, "config.yaml")
        os.WriteFile(rutaCfg, []byte("liz:\n  puerto: 8080\n  nombre: Liz\n  version: 0.3.0\n"), 0644)

        log := logger.NuevaConSalida("test", io.Discard)
        gestorCfg, _ := config.Inicializar(rutaCfg)
        gestorPer, _ := permisos.Inicializar(tmpDir)

        // Crear coordinador de contexto
        dirContexto := filepath.Join(tmpDir, "contexto")
        coordinador, err := contexto.NuevoCoordinador(dirContexto)
        if err != nil {
                t.Fatalf("NuevoCoordinador: %v", err)
        }

        srv := Nuevo(gestorCfg, gestorPer, log).ConCoordinador(coordinador)
        return srv, coordinador, tmpDir
}

// crearProyectoDirTest crea un proyecto Go pequeño en un directorio temporal.
func crearProyectoDirTest(t *testing.T) string {
        t.Helper()
        dir := t.TempDir()
        os.MkdirAll(filepath.Join(dir, "src"), 0755)

        os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "fmt"

func main() {
        fmt.Println("hola")
}

func saludar() string {
        return "hola"
}
`), 0644)

        os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644)

        os.WriteFile(filepath.Join(dir, "src", "auth.go"), []byte(`package src

type Usuario struct {
        Nombre string
}

func Autenticar(u Usuario) bool {
        return u.Nombre != ""
}
`), 0644)

        return dir
}

func TestHandlerContexto_SinCoordinador_Da503(t *testing.T) {
        // Servidor sin coordinador debe responder 503 a endpoints de contexto
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusServiceUnavailable {
                t.Errorf("Status = %d, se esperaba 503", rec.Code)
        }
}

func TestHandlerContextoProyectos_Vacio(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)

        req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }

        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }
}

func TestHandlerContextoIndexar_Y_ObtenerMapa(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        // Indexar
        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusCreated {
                t.Fatalf("Status = %d, se esperaba 201 (body: %s)", rec.Code, rec.Body.String())
        }

        // Obtener nombre del proyecto del response
        resp := parseRespuesta(t, rec.Body.Bytes())
        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatalf("datos no es map: %T", resp["datos"])
        }
        nombre, ok := datos["nombre"].(string)
        if !ok || nombre == "" {
                t.Fatalf("nombre no encontrado en response")
        }

        // Listar proyectos debe tener 1 ahora
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp = parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }

        // Obtener mapa
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/mapa", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        if rec.Code != http.StatusOK {
                t.Errorf("mapa: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }
}

func TestHandlerContextoIndexar_BodyInvalido(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)

        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader("not json"))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba 400", rec.Code)
        }
}

func TestHandlerContextoIndexar_SinRuta(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)

        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(`{}`))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba 400 (body: %s)", rec.Code, rec.Body.String())
        }
}

func TestHandlerContextoMapa_ProyectoInexistente(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)

        req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/mapa", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotFound {
                t.Errorf("Status = %d, se esperaba 404", rec.Code)
        }
}

func TestHandlerContextoArbol(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        // Indexar
        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Obtener árbol
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/arbol", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("arbol: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }

        resp = parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Error("exito debería ser true")
        }
}

func TestHandlerContextoFragmentosPorRuta(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        // Indexar
        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Listar fragmentos de main.go
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/fragmentos?ruta=main.go", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("fragmentos: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }
}

func TestHandlerContextoFragmentosPorRuta_SinParametro(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Sin parámetro ruta debe dar 400
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/fragmentos", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, se esperaba 400", rec.Code)
        }
}

func TestHandlerContextoBuscar(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Buscar "auth"
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/buscar?patron=auth", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("buscar: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }
}

func TestHandlerContextoResumen(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Obtener resumen de main.go
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/resumen?ruta=main.go", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("resumen: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }

        resp = parseRespuesta(t, rec.Body.Bytes())
        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatal("datos no es map")
        }
        if datos["lenguaje"] != "go" {
                t.Errorf("lenguaje = %v, se esperaba 'go'", datos["lenguaje"])
        }
}

func TestHandlerContextoEliminar(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Eliminar
        req = httptest.NewRequest("DELETE", "/api/v1/contexto/proyectos/"+nombre, nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("eliminar: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }

        // Verificar que ya no está
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/mapa", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        if rec.Code != http.StatusNotFound {
                t.Errorf("después de eliminar: Status = %d, se esperaba 404", rec.Code)
        }
}

func TestHandlerContextoReindexar_TodoElProyecto(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Reindexar todo
        req = httptest.NewRequest("POST", "/api/v1/contexto/proyectos/"+nombre+"/reindexar", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("reindexar: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }
}

func TestHandlerContextoReindexar_ArchivoUnico(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Reindexar main.go
        req = httptest.NewRequest("POST", "/api/v1/contexto/proyectos/"+nombre+"/reindexar",
                strings.NewReader(`{"ruta":"main.go"}`))
        req.Header.Set("Content-Type", "application/json")
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("reindexar archivo: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }
}

func TestHandlerContextoIndice(t *testing.T) {
        srv, _, _ := setupTestServidorConContexto(t)
        proyectoDir := crearProyectoDirTest(t)

        body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
        req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        resp := parseRespuesta(t, rec.Body.Bytes())
        nombre := resp["datos"].(map[string]interface{})["nombre"].(string)

        // Obtener índice
        req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/indice", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("indice: Status = %d, se esperaba 200 (body: %s)", rec.Code, rec.Body.String())
        }

        resp = parseRespuesta(t, rec.Body.Bytes())
        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatal("datos no es map")
        }
        if datos["total_archivos"] == nil {
                t.Error("total_archivos debería estar presente")
        }
}
