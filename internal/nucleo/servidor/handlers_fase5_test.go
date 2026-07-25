package servidor

import (
        "bytes"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "testing"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/integradas"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/registro"
)

// setupServidorConCatalogo crea un servidor con catálogo de herramientas
// conteniendo las 7 herramientas integradas.
func setupServidorConCatalogo(t *testing.T) *Servidor {
        t.Helper()
        srv, _, _ := setupTestServidor(t)

        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)
        return srv
}

// registrarHerramientasIntegradas helper que registra las 7 herramientas
// en un catálogo.
func registrarHerramientasIntegradas(t *testing.T, cat *registro.Catalogo) {
        t.Helper()
        if err := cat.Registrar(integradas.NewTerminal()); err != nil {
                t.Fatalf("registrar Terminal: %v", err)
        }
        if err := cat.Registrar(integradas.NewNavegadorArchivos()); err != nil {
                t.Fatalf("registrar NavegadorArchivos: %v", err)
        }
        if err := cat.Registrar(integradas.NewBuscador()); err != nil {
                t.Fatalf("registrar Buscador: %v", err)
        }
        if err := cat.Registrar(integradas.NewEditor()); err != nil {
                t.Fatalf("registrar Editor: %v", err)
        }
        if err := cat.Registrar(integradas.NewProcesos()); err != nil {
                t.Fatalf("registrar Procesos: %v", err)
        }
        if err := cat.Registrar(integradas.NewMonitor()); err != nil {
                t.Fatalf("registrar Monitor: %v", err)
        }
        if err := cat.Registrar(integradas.NewInstalador()); err != nil {
                t.Fatalf("registrar Instalador: %v", err)
        }
}

// ============================================================================
// Tests de /api/v1/herramientas
// ============================================================================

func TestHerramientasSinCatalogo_Devuelve503(t *testing.T) {
        srv, _, _ := setupTestServidor(t) // sin catálogo

        req := httptest.NewRequest("GET", "/api/v1/herramientas", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusServiceUnavailable {
                t.Errorf("Status = %d, esperaba 503", rec.Code)
        }
}

func TestHerramientasListar_Vacia(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        srv = srv.ConCatalogo(cat)

        req := httptest.NewRequest("GET", "/api/v1/herramientas", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d, esperaba 200", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Errorf("exito = %v, esperaba true", resp["exito"])
        }
        datos, ok := resp["datos"].([]interface{})
        if !ok {
                t.Fatalf("datos no es array: %T", resp["datos"])
        }
        if len(datos) != 0 {
                t.Errorf("len(datos) = %d, esperaba 0", len(datos))
        }
}

func TestHerramientasListar_ConIntegradas(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        req := httptest.NewRequest("GET", "/api/v1/herramientas", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        datos, ok := resp["datos"].([]interface{})
        if !ok {
                t.Fatalf("datos no es array: %T", resp["datos"])
        }
        if len(datos) != 7 {
                t.Errorf("len(datos) = %d, esperaba 7 (7 herramientas integradas)", len(datos))
        }
}

func TestHerramientasListar_ToolsEndpointCompatibilidad(t *testing.T) {
        // /api/v1/tools debe dar el mismo resultado que /api/v1/herramientas
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        req := httptest.NewRequest("GET", "/api/v1/tools", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        datos, _ := resp["datos"].([]interface{})
        if len(datos) != 7 {
                t.Errorf("len(datos) = %d, esperaba 7", len(datos))
        }
}

// ============================================================================
// Tests de /api/v1/herramientas/{nombre}
// ============================================================================

func TestHerramientasInfo_Existe(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        req := httptest.NewRequest("GET", "/api/v1/herramientas/terminal", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d, esperaba 200", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatalf("datos no es objeto: %T", resp["datos"])
        }
        if datos["nombre"] != "terminal" {
                t.Errorf("nombre = %v, esperaba terminal", datos["nombre"])
        }
        if datos["descripcion"] == nil || datos["descripcion"] == "" {
                t.Errorf("descripcion vacía")
        }
}

func TestHerramientasInfo_NoExiste(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        req := httptest.NewRequest("GET", "/api/v1/herramientas/inexistente_2025", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotFound {
                t.Errorf("Status = %d, esperaba 404", rec.Code)
        }
}

// ============================================================================
// Tests de /api/v1/herramientas/ejecutar
// ============================================================================

func TestHerramientasEjecutar_TerminalEcho(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre: "terminal",
                Parametros: map[string]interface{}{
                        "comando": "echo",
                        "args":    []interface{}{"hola", "test"},
                },
        })

        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d, body: %s", rec.Code, rec.Body.String())
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Errorf("exito = %v, esperaba true. Body: %s", resp["exito"], rec.Body.String())
        }
}

func TestHerramientasEjecutar_NavegadorListar(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre: "navegador_archivos",
                Parametros: map[string]interface{}{
                        "operacion": "listar",
                        "ruta":      "/tmp",
                },
        })

        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Errorf("exito = %v. Body: %s", resp["exito"], rec.Body.String())
        }
}

func TestHerramientasEjecutar_MonitorCompleto(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre:     "monitor",
                Parametros: map[string]interface{}{"operacion": "completo"},
        })

        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Errorf("exito = %v. Body: %s", resp["exito"], rec.Body.String())
        }
}

func TestHerramientasEjecutar_SinNombre(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Parametros: map[string]interface{}{},
        })

        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, esperaba 400", rec.Code)
        }
}

func TestHerramientasEjecutar_HerramientaNoExiste(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre:     "inexistente_xyz",
                Parametros: map[string]interface{}{},
        })

        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusNotFound {
                t.Errorf("Status = %d, esperaba 404", rec.Code)
        }
}

func TestHerramientasEjecutar_BodyInvalido(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar",
                bytes.NewReader([]byte("{invalid json}")))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusBadRequest {
                t.Errorf("Status = %d, esperaba 400", rec.Code)
        }
}

func TestHerramientasEjecutar_ResultadoFalla(t *testing.T) {
        // Ejecutar una herramienta que retorna Exito=false (sin peligroso_confirma)
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre: "terminal",
                Parametros: map[string]interface{}{
                        "comando": "rm",
                        "args":    []interface{}{"-rf", "/"},
                        // sin peligroso_confirma
                },
        })

        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        // HTTP 200 pero exito=false (la herramienta decidió no ejecutar)
        if rec.Code != http.StatusOK {
                t.Errorf("Status = %d, esperaba 200", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != false {
                t.Errorf("exito = %v, esperaba false (comando peligroso bloqueado)", resp["exito"])
        }
}

// ============================================================================
// Tests de /api/v1/herramientas/metricas
// ============================================================================

func TestHerramientasMetricas_Vacia(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        srv = srv.ConCatalogo(cat)

        req := httptest.NewRequest("GET", "/api/v1/herramientas/metricas", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        datos, ok := resp["datos"].(map[string]interface{})
        if !ok {
                t.Fatalf("datos no es objeto: %T", resp["datos"])
        }
        if datos["total_ejecuciones"].(float64) != 0 {
                t.Errorf("total_ejecuciones = %v, esperaba 0", datos["total_ejecuciones"])
        }
}

func TestHerramientasMetricas_TrasEjecucion(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        // Ejecutar 3 veces
        for i := 0; i < 3; i++ {
                body, _ := json.Marshal(BodyEjecutarHerramienta{
                        Nombre:     "monitor",
                        Parametros: map[string]interface{}{"operacion": "uptime"},
                })
                req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
                rec := httptest.NewRecorder()
                srv.router.ServeHTTP(rec, req)
        }

        // Verificar métricas
        req := httptest.NewRequest("GET", "/api/v1/herramientas/metricas", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        resp := parseRespuesta(t, rec.Body.Bytes())
        datos := resp["datos"].(map[string]interface{})
        if datos["total_ejecuciones"].(float64) != 3 {
                t.Errorf("total_ejecuciones = %v, esperaba 3", datos["total_ejecuciones"])
        }
}

func TestHerramientasMetricas_Una(t *testing.T) {
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        // Ejecutar una vez
        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre:     "monitor",
                Parametros: map[string]interface{}{"operacion": "uptime"},
        })
        req1 := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec1 := httptest.NewRecorder()
        srv.router.ServeHTTP(rec1, req1)

        // Métricas de monitor
        req := httptest.NewRequest("GET", "/api/v1/herramientas/metricas/monitor", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Fatalf("Status = %d", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        datos := resp["datos"].(map[string]interface{})
        if datos["nombre"] != "monitor" {
                t.Errorf("nombre = %v", datos["nombre"])
        }
        if datos["ejecuciones"].(float64) != 1 {
                t.Errorf("ejecuciones = %v, esperaba 1", datos["ejecuciones"])
        }
}

// ============================================================================
// Test de integración completa
// ============================================================================

func TestHerramientas_IntegracionCompleta(t *testing.T) {
        // Test end-to-end: listar → info → ejecutar → métricas
        srv, _, _ := setupTestServidor(t)
        cat := registro.NuevoCatalogo()
        registrarHerramientasIntegradas(t, cat)
        srv = srv.ConCatalogo(cat)

        // 1. Listar
        req := httptest.NewRequest("GET", "/api/v1/herramientas", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        if rec.Code != http.StatusOK {
                t.Fatalf("listar: Status = %d", rec.Code)
        }

        // 2. Info
        req = httptest.NewRequest("GET", "/api/v1/herramientas/editor", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        if rec.Code != http.StatusOK {
                t.Fatalf("info: Status = %d", rec.Code)
        }

        // 3. Ejecutar (leer /etc/hostname — siempre existe en Linux)
        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre: "editor",
                Parametros: map[string]interface{}{
                        "operacion": "leer",
                        "ruta":      "/etc/hostname",
                },
        })
        req = httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        if rec.Code != http.StatusOK {
                t.Fatalf("ejecutar: Status = %d", rec.Code)
        }
        resp := parseRespuesta(t, rec.Body.Bytes())
        if resp["exito"] != true {
                t.Errorf("ejecutar editor falló: %s", rec.Body.String())
        }

        // 4. Métricas
        req = httptest.NewRequest("GET", "/api/v1/herramientas/metricas", nil)
        rec = httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)
        if rec.Code != http.StatusOK {
                t.Fatalf("metricas: Status = %d", rec.Code)
        }
        resp = parseRespuesta(t, rec.Body.Bytes())
        datos := resp["datos"].(map[string]interface{})
        if datos["total_ejecuciones"].(float64) != 1 {
                t.Errorf("total_ejecuciones = %v, esperaba 1", datos["total_ejecuciones"])
        }
}

// TestHerramientasSinCatalogo_Ejecutar_Devuelve503 verifica 503 sin catálogo.
func TestHerramientasSinCatalogo_Ejecutar_Devuelve503(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        body, _ := json.Marshal(BodyEjecutarHerramienta{
                Nombre:     "terminal",
                Parametros: map[string]interface{}{},
        })
        req := httptest.NewRequest("POST", "/api/v1/herramientas/ejecutar", bytes.NewReader(body))
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusServiceUnavailable {
                t.Errorf("Status = %d, esperaba 503", rec.Code)
        }
}

// TestHerramientasSinCatalogo_Metricas_Devuelve503
func TestHerramientasSinCatalogo_Metricas_Devuelve503(t *testing.T) {
        srv, _, _ := setupTestServidor(t)

        req := httptest.NewRequest("GET", "/api/v1/herramientas/metricas", nil)
        rec := httptest.NewRecorder()
        srv.router.ServeHTTP(rec, req)

        if rec.Code != http.StatusServiceUnavailable {
                t.Errorf("Status = %d, esperaba 503", rec.Code)
        }
}
