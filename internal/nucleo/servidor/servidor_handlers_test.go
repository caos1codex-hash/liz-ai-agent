package servidor

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/auto_creacion"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/registro"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/memoria"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/orquestador"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
	"github.com/caos1codex-hash/liz-ai-agent/internal/pipeline"
)

// Aliases para señales (compat cross-platform; en Windows SIGTERM no es soportado
// pero los tests en Linux/macOS sí).
var (
	sigTERM       = syscall.SIGTERM
	syscallKill   = syscall.Kill
	syscallGetpid = syscall.Getpid
)

// ============================================================================
// Helpers compartidos
// ============================================================================

// setupServidorConMemoria crea un servidor con gestor de memoria inyectado en un dir temporal.
func setupServidorConMemoria(t *testing.T) (*Servidor, *memoria.Gestor) {
	t.Helper()
	srv, _, _ := setupTestServidor(t)

	dirMem := filepath.Join(t.TempDir(), "memoria_root")
	g, err := memoria.NuevoGestor(dirMem)
	if err != nil {
		t.Fatalf("NuevoGestor: %v", err)
	}
	srv = srv.ConMemoria(g)
	return srv, g
}

// setupServidorConOrquestador crea un servidor con orquestador apuntando a un mock NVIDIA.
func setupServidorConOrquestador(t *testing.T, mockSrv *httptest.Server) *Servidor {
	t.Helper()

	tmpDir := t.TempDir()
	log := logger.NuevaConSalida("test", io.Discard)

	// Crear gestor de config directamente con struct (evita problemas de parsing YAML)
	gestorCfg := config.NuevoGestorConConfig(&config.Configuracion{
		Puerto:  8080,
		Host:    "localhost",
		Nombre:  "Liz",
		Version: "0.4.0",
		Modelos: []config.ConfiguracionModelo{
			{
				Nombre:     "test-model",
				Proveedor:  "nvidia",
				APIKey:     "test-key-12345",
				URL:        mockSrv.URL,
				Habilitado: true,
			},
		},
	})

	gestorPer, err := permisos.Inicializar(tmpDir)
	if err != nil {
		t.Fatalf("Inicializar permisos: %v", err)
	}

	srv := Nuevo(gestorCfg, gestorPer, log)

	o, err := orquestador.NuevoOrquestador(gestorCfg)
	if err != nil {
		t.Fatalf("NuevoOrquestador: %v", err)
	}
	srv = srv.ConOrquestador(o)
	return srv
}

// =====================================================================
// Tests — Setters (ConMemoria, ConOrquestador, ConPipeline, ConAutoGestor)
// =====================================================================

func TestConMemoria_InyectaGestor(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	if srv.gestorMem != nil {
		t.Fatal("gestorMem debería ser nil inicialmente")
	}
	dirBase := t.TempDir()
	g, err := memoria.NuevoGestor(dirBase)
	if err != nil {
		t.Fatalf("NuevoGestor: %v", err)
	}
	ret := srv.ConMemoria(g)
	if ret != srv {
		t.Error("ConMemoria debería retornar el mismo servidor (fluent)")
	}
	if srv.gestorMem == nil {
		t.Error("gestorMem debería estar inyectado después de ConMemoria")
	}
}

func TestConOrquestador_Inyecta(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	if srv.orquestador != nil {
		t.Fatal("orquestador debería ser nil inicialmente")
	}
	// Usar NuevoGestorConConfig para evitar problemas de YAML
	cfg := config.NuevoGestorConConfig(&config.Configuracion{
		Puerto:  8080,
		Host:    "localhost",
		Nombre:  "Liz",
		Version: "0.4.0",
		Modelos: []config.ConfiguracionModelo{
			{
				Nombre:     "m",
				Proveedor:  "nvidia",
				APIKey:     "k",
				URL:        "http://x",
				Habilitado: true,
			},
		},
	})
	o, err := orquestador.NuevoOrquestador(cfg)
	if err != nil {
		t.Fatalf("NuevoOrquestador: %v", err)
	}
	ret := srv.ConOrquestador(o)
	if ret != srv {
		t.Error("ConOrquestador debería retornar el mismo servidor (fluent)")
	}
	if srv.orquestador == nil {
		t.Error("orquestador debería estar inyectado")
	}
}

func TestConPipeline_Inyecta(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	if srv.pipelineMgr != nil {
		t.Fatal("pipelineMgr debería ser nil inicialmente")
	}
	p := pipeline.Nuevo(pipeline.NuevasOpciones{})
	ret := srv.ConPipeline(p)
	if ret != srv {
		t.Error("ConPipeline debería retornar el mismo servidor (fluent)")
	}
	if srv.pipelineMgr == nil {
		t.Error("pipelineMgr debería estar inyectado")
	}
}

func TestConAutoGestor_Inyecta(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	if srv.autoGestor != nil {
		t.Fatal("autoGestor debería ser nil inicialmente")
	}
	tmpDir := t.TempDir()
	g, err := auto_creacion.NuevoGestor(nil, tmpDir, nil)
	if err != nil {
		t.Fatalf("NuevoGestor: %v", err)
	}
	ret := srv.ConAutoGestor(g)
	if ret != srv {
		t.Error("ConAutoGestor debería retornar el mismo servidor (fluent)")
	}
	if srv.autoGestor == nil {
		t.Error("autoGestor debería estar inyectado")
	}
}

// =====================================================================
// Tests — Helper functions
// =====================================================================

func TestRequiereMemoria_TruePath(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)
	w := httptest.NewRecorder()
	if !srv.requiereMemoria(w) {
		t.Error("requiereMemoria debería retornar true con gestorMem inyectado")
	}
}

func TestRequiereOrquestador_TruePath(t *testing.T) {
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"x"}`))
	}))
	defer mockSrv.Close()
	srv := setupServidorConOrquestador(t, mockSrv)

	w := httptest.NewRecorder()
	if !srv.requiereOrquestador(w) {
		t.Error("requiereOrquestador debería retornar true con orquestador inyectado")
	}
}

func TestRequiereAutoGestor_TruePath(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	tmpDir := t.TempDir()
	g, _ := auto_creacion.NuevoGestor(nil, tmpDir, nil)
	srv = srv.ConAutoGestor(g)

	w := httptest.NewRecorder()
	if !srv.requiereAutoGestor(w) {
		t.Error("requiereAutoGestor debería retornar true con autoGestor inyectado")
	}
}

func TestParseIntSafe(t *testing.T) {
	cases := []struct {
		input string
		want  int
		err   bool
	}{
		{"42", 42, false},
		{"0", 0, false},
		{"-5", -5, false},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := parseIntSafe(c.input)
		if c.err && err == nil {
			t.Errorf("parseIntSafe(%q): esperaba error, got nil", c.input)
		}
		if !c.err && err != nil {
			t.Errorf("parseIntSafe(%q): error inesperado: %v", c.input, err)
		}
		if !c.err && got != c.want {
			t.Errorf("parseIntSafe(%q) = %d, esperaba %d", c.input, got, c.want)
		}
	}
}

func TestAutoGestorExpuesto(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	if srv.AutoGestorExpuesto() != nil {
		t.Error("AutoGestorExpuesto debería ser nil sin inyectar")
	}
	tmpDir := t.TempDir()
	g, _ := auto_creacion.NuevoGestor(nil, tmpDir, nil)
	srv = srv.ConAutoGestor(g)
	if srv.AutoGestorExpuesto() == nil {
		t.Error("AutoGestorExpuesto debería ser no-nil tras ConAutoGestor")
	}
}

func TestCatalogoSnapshotExpuesto(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	if srv.CatalogoSnapshotExpuesto() != nil {
		t.Error("CatalogoSnapshotExpuesto debería ser nil sin catálogo")
	}
	cat := registro.NuevoCatalogo()
	srv = srv.ConCatalogo(cat)
	if snap := srv.CatalogoSnapshotExpuesto(); snap == nil {
		t.Error("CatalogoSnapshotExpuesto debería ser no-nil con catálogo")
	}
}

// =====================================================================
// Tests — Middleware Recuperacion Panic
// =====================================================================

func TestMiddlewareRecuperacionPanic_Recupera(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	// Registrar un handler que paniquea
	srv.router.HandleFunc("/api/v1/_test_panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}).Methods("GET")

	req := httptest.NewRequest("GET", "/api/v1/_test_panic", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, esperaba 500 tras panic", rec.Code)
	}
}

// =====================================================================
// Tests — HandlerConfigPut casos adicionales
// =====================================================================

func TestHandlerConfigPut_MultipleCampos(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	body := `{"campos": {"puerto": "9091", "nombre": "LizMod"}}`
	req := httptest.NewRequest("PUT", "/api/v1/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

// =====================================================================
// Tests — Health (different state)
// =====================================================================

func TestHandlerHealth_PermisosHabilitado(t *testing.T) {
	srv, _, gestorPer := setupTestServidor(t)

	_ = gestorPer.Conceder(permisos.PermTerminal, permisos.NivelTotal, "test", "test")

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200", rec.Code)
	}

	resp := parseRespuesta(t, rec.Body.Bytes())
	datos := resp["datos"].(map[string]interface{})
	perms := datos["permisos"].(map[string]interface{})
	if perms["habilitado"] != true {
		t.Error("permisos.habilitado debería ser true tras Conceder")
	}
}

// =====================================================================
// Tests — Contexto Fase 3.5 (simbolos, grafo, importancia, buscar-hibrido, mapa-repo, empaquetar)
// =====================================================================

// indexarProyectoParaTest helper que indexa el proyecto de test y retorna el nombre.
func indexarProyectoParaTest(t *testing.T, srv *Servidor) string {
	t.Helper()
	proyectoDir := crearProyectoDirTest(t)
	body := fmt.Sprintf(`{"ruta": %q}`, proyectoDir)
	req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("indexar: Status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	return resp["datos"].(map[string]interface{})["nombre"].(string)
}

func TestHandlerContextoSimbolos_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/simbolos?ruta=main.go", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	if resp["exito"] != true {
		t.Error("exito debería ser true")
	}
}

func TestHandlerContextoSimbolos_SinParametroRuta(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/x/simbolos", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin ruta", rec.Code)
	}
}

func TestHandlerContextoSimbolos_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/simbolos?ruta=main.go", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoGrafo_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/grafo", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	datos, ok := resp["datos"].(map[string]interface{})
	if !ok {
		t.Fatalf("datos no es map: %T", resp["datos"])
	}
	if datos["nodos"] == nil {
		t.Error("datos.nodos no debería ser nil")
	}
	if datos["estadisticas"] == nil {
		t.Error("datos.estadisticas no debería ser nil")
	}
}

func TestHandlerContextoGrafo_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/grafo", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoImportancia_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/importancia", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerContextoImportancia_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/importancia", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoBuscarHibrido_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	body := `{"query": "auth", "top_k": 5}`
	req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos/"+nombre+"/buscar-hibrido", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerContextoBuscarHibrido_SinQuery(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	body := `{"top_k": 5}`
	req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos/"+nombre+"/buscar-hibrido", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin query", rec.Code)
	}
}

func TestHandlerContextoBuscarHibrido_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	body := `{"query": "x"}`
	req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos/no-existe/buscar-hibrido", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoMapaRepo_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/mapa-repo", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerContextoMapaRepo_FormatoTexto(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/mapa-repo?formato=texto&max_tokens=500", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %s, esperaba text/plain", ct)
	}
}

func TestHandlerContextoMapaRepo_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/mapa-repo", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoEmpaquetar_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	body := `{"query": "auth", "presupuesto_tokens": 4000, "profundidad_imports": 1}`
	req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos/"+nombre+"/empaquetar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerContextoEmpaquetar_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	body := `{"query": "x"}`
	req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos/no-existe/empaquetar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoFragmento_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	// Primero obtener fragmentos por ruta para tener un ID
	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/fragmentos?ruta=main.go", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("fragmentos: Status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	frags, ok := resp["datos"].([]interface{})
	if !ok || len(frags) == 0 {
		t.Skip("No hay fragmentos para main.go")
	}
	primero := frags[0].(map[string]interface{})
	id, _ := primero["id"].(string)
	if id == "" {
		t.Skip("Fragmento sin ID")
	}

	// Ahora obtener ese fragmento por ID
	req = httptest.NewRequest("GET", "/api/v1/contexto/proyectos/"+nombre+"/fragmentos/"+id, nil)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("fragmento por ID: Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerContextoFragmento_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/fragmentos/abc", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoEliminar_SinNombre(t *testing.T) {
	// Invocar el handler directamente con request sin {nombre} en vars.
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("DELETE", "/api/v1/contexto/proyectos/", nil)
	rec := httptest.NewRecorder()
	srv.handlerContextoEliminar(rec, req)
	// Debe responder 400 porque nombre == ""
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin nombre", rec.Code)
	}
}

// =====================================================================
// Tests — Tracker handlers
// =====================================================================

func TestHandlerTrackerRegistrarEdicion_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	body := fmt.Sprintf(`{"proyecto": %q, "ruta": "main.go"}`, nombre)
	req := httptest.NewRequest("POST", "/api/v1/contexto/tracker/edicion", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerTrackerRegistrarEdicion_SinProyecto(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	body := `{"ruta": "main.go"}`
	req := httptest.NewRequest("POST", "/api/v1/contexto/tracker/edicion", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin proyecto", rec.Code)
	}
}

func TestHandlerTrackerRegistrarEdicion_BodyInvalido(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("POST", "/api/v1/contexto/tracker/edicion", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 con body inválido", rec.Code)
	}
}

func TestHandlerTrackerRecientes_OK(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)
	nombre := indexarProyectoParaTest(t, srv)

	// Registrar una edición primero
	body := fmt.Sprintf(`{"proyecto": %q, "ruta": "main.go"}`, nombre)
	req := httptest.NewRequest("POST", "/api/v1/contexto/tracker/edicion", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	// Listar recientes
	req = httptest.NewRequest("GET", "/api/v1/contexto/tracker/recientes?proyecto="+nombre, nil)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerTrackerRecientes_SinProyecto(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/tracker/recientes", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin proyecto", rec.Code)
	}
}

// =====================================================================
// Tests — Orquestador handlers (con instancia real)
// =====================================================================

func TestHandlerOrquestadorEstado_ConInyeccion(t *testing.T) {
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"x"}`))
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	req := httptest.NewRequest("GET", "/api/v1/orquestador", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	datos, ok := resp["datos"].(map[string]interface{})
	if !ok {
		t.Fatalf("datos no es map: %T", resp["datos"])
	}
	if datos["disponible"] != true {
		t.Error("disponible debería ser true")
	}
}

func TestHandlerOrquestadorModelos_ConInyeccion(t *testing.T) {
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"x"}`))
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	req := httptest.NewRequest("GET", "/api/v1/orquestador/modelos", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	modelos, ok := resp["datos"].([]interface{})
	if !ok {
		t.Fatalf("datos no es array: %T", resp["datos"])
	}
	if len(modelos) == 0 {
		t.Error("debería haber al menos un modelo")
	}
	// Verificar sanitización: API keys deben ser "***"
	bodyStr := rec.Body.String()
	if strings.Contains(bodyStr, "test-key-12345") {
		t.Error("la API key 'test-key-12345' no debería aparecer en la respuesta")
	}
}

func TestHandlerOrquestadorMetricas_ConInyeccion(t *testing.T) {
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"x"}`))
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	req := httptest.NewRequest("GET", "/api/v1/orquestador/metricas", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerOrquestadorCompletar_SinMensajes(t *testing.T) {
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"x"}`))
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	body := `{"modelo": "test-model"}`
	req := httptest.NewRequest("POST", "/api/v1/orquestador/completar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin mensajes", rec.Code)
	}
}

func TestHandlerOrquestadorCompletar_BodyInvalido(t *testing.T) {
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"x"}`))
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	req := httptest.NewRequest("POST", "/api/v1/orquestador/completar", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 con body inválido", rec.Code)
	}
}

func TestHandlerOrquestadorCompletar_ConMockExitoso(t *testing.T) {
	// Mock que simula respuesta exitosa de NVIDIA
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
                        "id": "chatcmpl-123",
                        "object": "chat.completion",
                        "created": 1677652288,
                        "model": "test-model",
                        "choices": [{
                                "index": 0,
                                "message": {"role": "assistant", "content": "Hola"},
                                "finish_reason": "stop"
                        }],
                        "usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
                }`))
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	body := `{"modelo": "test-model", "mensajes": [{"role": "user", "content": "Hola"}]}`
	req := httptest.NewRequest("POST", "/api/v1/orquestador/completar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerOrquestadorCompletar_Stream(t *testing.T) {
	// Mock SSE que envía chunks
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\": [{\"delta\": {\"content\": \"Hola\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"choices\": [{\"delta\": {},\"finish_reason\":\"stop\"}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	body := `{"modelo": "test-model", "mensajes": [{"role": "user", "content": "Hola"}], "stream": true}`
	req := httptest.NewRequest("POST", "/api/v1/orquestador/completar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %s, esperaba text/event-stream", ct)
	}
}

func TestHandlerOrquestadorCompletar_FallaConError5xx(t *testing.T) {
	// Mock que retorna 503 → error reinterrable → orquestador retorna error
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "service unavailable"}`))
	}))
	defer mockSrv.Close()

	srv := setupServidorConOrquestador(t, mockSrv)

	body := `{"modelo": "test-model", "mensajes": [{"role": "user", "content": "Hola"}]}`
	req := httptest.NewRequest("POST", "/api/v1/orquestador/completar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, esperaba 502 (body: %s)", rec.Code, rec.Body.String())
	}
}

// =====================================================================
// Tests — Memoria handlers (con gestor real)
// =====================================================================

func TestHandlerMemoriaSesiones_SinUsuarioID(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/memoria/sesiones", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin usuario_id", rec.Code)
	}
}

func TestHandlerMemoriaSesiones_Vacia(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/memoria/sesiones?usuario_id=user1", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerMemoriaSesiones_TrasCrear(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	_, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}

	// Solo activas
	req := httptest.NewRequest("GET", "/api/v1/memoria/sesiones?usuario_id=user1&solo_activas=true", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerMemoriaNuevaSesion_OK(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"usuario_id": "user1", "proyecto": "test"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Status = %d, esperaba 201 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	if resp["exito"] != true {
		t.Error("exito debería ser true")
	}
}

func TestHandlerMemoriaNuevaSesion_SinUsuarioID(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"proyecto": "test"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin usuario_id", rec.Code)
	}
}

func TestHandlerMemoriaNuevaSesion_BodyInvalido(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400", rec.Code)
	}
}

func TestHandlerMemoriaObtenerSesion_OK(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	sesion, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/v1/memoria/sesiones/"+sesion.ID, nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerMemoriaObtenerSesion_NoExiste(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/memoria/sesiones/sesion-inexistente", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerMemoriaCerrarSesion_OK(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	sesion, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/"+sesion.ID+"/cerrar?usuario_id=user1", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerMemoriaCerrarSesion_SinParams(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/abc/cerrar", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin id ni usuario_id", rec.Code)
	}
}

func TestHandlerMemoriaCerrarSesion_UsuarioDistinto(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	sesion, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/"+sesion.ID+"/cerrar?usuario_id=otro", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Status = %d, esperaba 403 (sesión no pertenece al usuario)", rec.Code)
	}
}

func TestHandlerMemoriaCerrarSesion_SesionNoExiste(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/no-existe/cerrar?usuario_id=user1", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerMemoriaAgregarMensaje_OK(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	_, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}

	body := `{"usuario_id": "user1", "rol": "usuario", "contenido": "Hola"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/abc/mensajes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Status = %d, esperaba 201 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerMemoriaAgregarMensaje_SinUsuarioID(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"rol": "usuario", "contenido": "Hola"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/abc/mensajes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin usuario_id", rec.Code)
	}
}

func TestHandlerMemoriaAgregarMensaje_SinContenido(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"usuario_id": "user1", "rol": "usuario"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/abc/mensajes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin contenido", rec.Code)
	}
}

func TestHandlerMemoriaAgregarMensaje_RolInvalido(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"usuario_id": "user1", "rol": "rol_inventado", "contenido": "Hola"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/abc/mensajes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 con rol inválido", rec.Code)
	}
}

func TestHandlerMemoriaAgregarMensaje_SinSesionActiva(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"usuario_id": "user_sin_sesion", "rol": "usuario", "contenido": "Hola"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/sesiones/abc/mensajes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin sesión activa", rec.Code)
	}
}

func TestHandlerMemoriaHechos_Vacio(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/memoria/hechos?usuario_id=user1", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	if resp["exito"] != true {
		t.Error("exito debería ser true")
	}
}

func TestHandlerMemoriaHechos_SinUsuarioID(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/memoria/hechos", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin usuario_id", rec.Code)
	}
}

func TestHandlerMemoriaAgregarHecho_OK(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"usuario_id": "user1", "sujeto": "usuario", "predicado": "prefiere", "objeto": "Go", "confianza": 0.9}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/hechos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Status = %d, esperaba 201 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerMemoriaAgregarHecho_FaltanCampos(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"usuario_id": "user1", "sujeto": "usuario"}`
	req := httptest.NewRequest("POST", "/api/v1/memoria/hechos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 con campos faltantes", rec.Code)
	}
}

func TestHandlerMemoriaAgregarHecho_BodyInvalido(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("POST", "/api/v1/memoria/hechos", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400", rec.Code)
	}
}

func TestHandlerMemoriaEliminarHecho_OK(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	hecho, err := g.AgregarHecho("user1", "usuario", "prefiere", "Go", 0.9, "")
	if err != nil {
		t.Fatalf("AgregarHecho: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/api/v1/memoria/hechos/"+hecho.ID+"?usuario_id=user1", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerMemoriaEliminarHecho_NoExiste(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("DELETE", "/api/v1/memoria/hechos/inexistente?usuario_id=user1", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerMemoriaEliminarHecho_SinParams(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("DELETE", "/api/v1/memoria/hechos/abc", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin usuario_id", rec.Code)
	}
}

func TestHandlerMemoriaContexto_OK(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	_, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}
	_, _ = g.AgregarMensaje("user1", memoria.RolUsuario, "Hola")
	_, _ = g.AgregarHecho("user1", "usuario", "prefiere", "Go", 0.9, "")

	req := httptest.NewRequest("GET", "/api/v1/memoria/contexto?usuario_id=user1&mensajes=5&hechos=10", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	datos, ok := resp["datos"].(map[string]interface{})
	if !ok {
		t.Fatalf("datos no es map: %T", resp["datos"])
	}
	if datos["mensajes_n"] == nil {
		t.Error("mensajes_n debería estar presente")
	}
	if datos["hechos_n"] == nil {
		t.Error("hechos_n debería estar presente")
	}
}

func TestHandlerMemoriaContexto_SinUsuarioID(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/memoria/contexto", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin usuario_id", rec.Code)
	}
}

// =====================================================================
// Tests — Fase 6 (Auto-creación) handlers
// =====================================================================

// setupServidorConAutoGestor crea servidor + catálogo + autoGestor (sin LLM).
func setupServidorConAutoGestor(t *testing.T) *Servidor {
	t.Helper()
	srv, _, _ := setupTestServidor(t)
	cat := registro.NuevoCatalogo()
	srv = srv.ConCatalogo(cat)
	tmpDir := t.TempDir()
	g, err := auto_creacion.NuevoGestor(nil, tmpDir, cat)
	if err != nil {
		t.Fatalf("NuevoGestor: %v", err)
	}
	srv = srv.ConAutoGestor(g)
	return srv
}

func TestHandlerAutoCrear_SinDescripcionNiForzar(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	body := `{}`
	req := httptest.NewRequest("POST", "/api/v1/herramientas/auto-crear", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin descripción ni forzar", rec.Code)
	}
}

func TestHandlerAutoCrear_BodyInvalido(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("POST", "/api/v1/herramientas/auto-crear", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 con body inválido", rec.Code)
	}
}

func TestHandlerAutoCrear_ConForzarNombre_ConGo(t *testing.T) {
	if !goSDKDisponible() {
		t.Skip("go no disponible en PATH — saltando test de compilación")
	}
	srv := setupServidorConAutoGestor(t)

	body := `{"forzar_nombre": "test_handler_auto_crear"}`
	req := httptest.NewRequest("POST", "/api/v1/herramientas/auto-crear", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	// 201 si todo OK, 422 si falló compilación/carga
	if rec.Code != http.StatusCreated && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("Status = %d, esperaba 201 o 422 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerAutoDetectar_SinDescripcion(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	body := `{}`
	req := httptest.NewRequest("POST", "/api/v1/herramientas/detectar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin descripción", rec.Code)
	}
}

func TestHandlerAutoDetectar_BodyInvalido(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("POST", "/api/v1/herramientas/detectar", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400", rec.Code)
	}
}

func TestHandlerAutoDetectar_SinLLM(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	body := `{"descripcion": "una herramienta que haga X"}`
	req := httptest.NewRequest("POST", "/api/v1/herramientas/detectar", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, esperaba 503 (sin LLM)", rec.Code)
	}
}

func TestHandlerAutoCreadasListar_Vacia(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	// Llamar al handler directamente porque la ruta /api/v1/herramientas/auto-creadas
	// es capturada por /api/v1/herramientas/{nombre} registrado antes en handlers_fase5.go.
	req := httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas", nil)
	rec := httptest.NewRecorder()
	srv.handlerAutoCreadasListar(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200", rec.Code)
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	if resp["exito"] != true {
		t.Error("exito debería ser true")
	}
}

func TestHandlerAutoCreadasInfo_NoExiste(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas/inexistente", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerAutoCreadasEliminar_NoExiste(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("DELETE", "/api/v1/herramientas/auto-creadas/inexistente", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerAutoCreadasProbar_NoExiste(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("POST", "/api/v1/herramientas/auto-creadas/inexistente/probar", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	// Debe responder 200 con exito=false (el gestor.Probar retorna error)
	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerAutoCreadasRecargar_NoExiste(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("POST", "/api/v1/herramientas/auto-creadas/inexistente/recargar", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("Status = %d, esperaba 422 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestHandlerAutoCreadasFuente_NoExiste(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas/inexistente/fuente", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerAutoCreadasLog_NoExiste(t *testing.T) {
	srv := setupServidorConAutoGestor(t)

	req := httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas/inexistente/log", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	// LeerLogCompilacion retorna "", nil si el archivo no existe → handler responde 200
	// con "(sin log de compilación)" como body.
	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (log vacío se trata como OK)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "sin log") {
		t.Errorf("Body debería indicar sin log, got: %s", rec.Body.String())
	}
}

// Test de integración Fase 6: crear herramienta, obtener fuente, log, info, probar, eliminar
func TestHandlerAutoCreadas_FlujoCompleto(t *testing.T) {
	if !goSDKDisponible() {
		t.Skip("go no disponible en PATH — saltando test de flujo completo")
	}
	srv := setupServidorConAutoGestor(t)

	// 1. Crear herramienta
	body := `{"forzar_nombre": "test_flujo_completo"}`
	req := httptest.NewRequest("POST", "/api/v1/herramientas/auto-crear", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("crear: Status = %d, esperaba 201 (body: %s)", rec.Code, rec.Body.String())
	}

	// 2. Listar (debería tener 1)
	// Llamar al handler directamente porque /api/v1/herramientas/auto-creadas
	// es capturada por /api/v1/herramientas/{nombre} registrado primero.
	req = httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas", nil)
	rec = httptest.NewRecorder()
	srv.handlerAutoCreadasListar(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("listar: Status = %d", rec.Code)
	}

	// 3. Info
	req = httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas/test_flujo_completo", nil)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("info: Status = %d (body: %s)", rec.Code, rec.Body.String())
	}

	// 4. Fuente
	req = httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas/test_flujo_completo/fuente", nil)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("fuente: Status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("fuente Content-Type = %s, esperaba text/plain", ct)
	}

	// 5. Log
	req = httptest.NewRequest("GET", "/api/v1/herramientas/auto-creadas/test_flujo_completo/log", nil)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("log: Status = %d", rec.Code)
	}

	// 6. Eliminar
	req = httptest.NewRequest("DELETE", "/api/v1/herramientas/auto-creadas/test_flujo_completo", nil)
	rec = httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("eliminar: Status = %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// =====================================================================
// Tests — Fase 7 chat handlers (con memoria y pipeline)
// =====================================================================

func TestHandlerChatGet_ConPipeline(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	srv.pipelineMgr = pipeline.Nuevo(pipeline.NuevasOpciones{})

	req := httptest.NewRequest("GET", "/api/v1/chat", nil)
	w := httptest.NewRecorder()
	srv.handlerChatGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200", w.Code)
	}
}

func TestHandlerChatSesiones_ConMemoria(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/chat/sesiones?usuario_id=user1", nil)
	w := httptest.NewRecorder()
	srv.handlerChatSesiones(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200", w.Code)
	}
}

func TestHandlerChatCrearSesion_SinMemoria(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	req := httptest.NewRequest("POST", "/api/v1/chat/sesiones", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handlerChatCrearSesion(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, esperaba 503 sin memoria", w.Code)
	}
}

func TestHandlerChatCrearSesion_ConMemoria(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	body := `{"usuario_id": "user1", "proyecto": "test"}`
	req := httptest.NewRequest("POST", "/api/v1/chat/sesiones", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handlerChatCrearSesion(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, esperaba 201 (body: %s)", w.Code, w.Body.String())
	}
}

func TestHandlerChatCrearSesion_BodyInvalido(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("POST", "/api/v1/chat/sesiones", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handlerChatCrearSesion(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 con body inválido", w.Code)
	}
}

func TestHandlerChatSesionDetalle_SinMemoria(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	req := httptest.NewRequest("GET", "/api/v1/chat/sesiones/abc", nil)
	w := httptest.NewRecorder()
	srv.handlerChatSesionDetalle(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, esperaba 503 sin memoria", w.Code)
	}
}

func TestHandlerChatSesionDetalle_NoExiste(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("GET", "/api/v1/chat/sesiones/sesion-inexistente", nil)
	w := httptest.NewRecorder()
	srv.handlerChatSesionDetalle(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", w.Code)
	}
}

func TestHandlerChatSesionDetalle_OK(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	sesion, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}
	_, _ = g.AgregarMensaje("user1", memoria.RolUsuario, "Hola")

	// Usar router.ServeHTTP para que mux.Vars se llene correctamente.
	req := httptest.NewRequest("GET", "/api/v1/chat/sesiones/"+sesion.ID+"?usuario_id=user1", nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", w.Code, w.Body.String())
	}
}

func TestHandlerChatCerrarSesion_SinMemoria(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	req := httptest.NewRequest("DELETE", "/api/v1/chat/sesiones/abc", nil)
	w := httptest.NewRecorder()
	srv.handlerChatCerrarSesion(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, esperaba 503 sin memoria", w.Code)
	}
}

func TestHandlerChatCerrarSesion_NoExiste(t *testing.T) {
	srv, _ := setupServidorConMemoria(t)

	req := httptest.NewRequest("DELETE", "/api/v1/chat/sesiones/inexistente", nil)
	w := httptest.NewRecorder()
	srv.handlerChatCerrarSesion(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", w.Code)
	}
}

func TestHandlerChatCerrarSesion_OK(t *testing.T) {
	srv, g := setupServidorConMemoria(t)
	sesion, err := g.NuevaSesion("user1", "test")
	if err != nil {
		t.Fatalf("NuevaSesion: %v", err)
	}

	// Usar router.ServeHTTP para que mux.Vars se llene correctamente.
	req := httptest.NewRequest("DELETE", "/api/v1/chat/sesiones/"+sesion.ID, nil)
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", w.Code, w.Body.String())
	}
}

// =====================================================================
// Helpers — go SDK availability
// =====================================================================

// goSDKDisponible retorna true si el comando `go` está disponible en PATH o en $HOME/go-sdk/go/bin.
func goSDKDisponible() bool {
	if home, err := os.UserHomeDir(); err == nil {
		local := filepath.Join(home, "go-sdk", "go", "bin", "go")
		if _, err := os.Stat(local); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("go"); err == nil {
		return true
	}
	return false
}

// =====================================================================
// Tests — parsearBody (server method)
// =====================================================================

func TestParsearBody_BodyVacio(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	var dst struct {
		X string `json:"x"`
	}
	// Body nil
	req := httptest.NewRequest("POST", "/x", nil)
	err := srv.parsearBody(req, &dst)
	if err != nil {
		t.Errorf("esperaba nil error con body nil, got %v", err)
	}
}

func TestParsearBody_ContentLengthCero(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	var dst struct {
		X string `json:"x"`
	}
	req := httptest.NewRequest("POST", "/x", strings.NewReader(""))
	req.ContentLength = 0
	err := srv.parsearBody(req, &dst)
	if err != nil {
		t.Errorf("esperaba nil error con content-length 0, got %v", err)
	}
}

func TestParsearBody_JSONValido(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	var dst struct {
		X string `json:"x"`
	}
	req := httptest.NewRequest("POST", "/x", strings.NewReader(`{"x": "hola"}`))
	req.Header.Set("Content-Type", "application/json")
	err := srv.parsearBody(req, &dst)
	if err != nil {
		t.Errorf("esperaba nil error, got %v", err)
	}
	if dst.X != "hola" {
		t.Errorf("X = %q, esperaba 'hola'", dst.X)
	}
}

// =====================================================================
// Tests — Fase 5 handlers adicionales (MetricasUna sin catálogo)
// =====================================================================

func TestHerramientasMetricasUna_SinCatalogo_Devuelve503(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	req := httptest.NewRequest("GET", "/api/v1/herramientas/metricas/terminal", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, esperaba 503 sin catálogo", rec.Code)
	}
}

// =====================================================================
// Tests — SolicitudCompletar struct (JSON marshalling)
// =====================================================================

func TestSolicitudCompletar_JSONRoundtrip(t *testing.T) {
	orig := SolicitudCompletar{
		Modelo:   "test-model",
		Tarea:    "general",
		Mensajes: []orquestador.MensajeChat{{Rol: "user", Contenido: "hola"}},
		Stream:   false,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded SolicitudCompletar
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Modelo != orig.Modelo {
		t.Errorf("Modelo = %q, esperaba %q", decoded.Modelo, orig.Modelo)
	}
}

// =====================================================================
// Tests — Detener servidor sin iniciar (debe fallar gracefully)
// =====================================================================

func TestDetener_SinIniciar(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	// Llamar Detener sin Iniciar — el httpServ está creado pero nunca iniciado.
	// Shutdown debe retornar error o nil, pero no paniquear.
	err := srv.Detener()
	_ = err // Aceptamos cualquiera
}

// =====================================================================
// Tests — Iniciar/Detener servidor ( Smoke test con señal )
// =====================================================================

func TestIniciar_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	rutaCfg := filepath.Join(tmpDir, "config.yaml")
	// Puerto alto poco probable de estar ocupado.
	os.WriteFile(rutaCfg, []byte("liz:\n  puerto: 18099\n  nombre: LizTest\n  version: 0.1.0\n"), 0644)

	log := logger.NuevaConSalida("test", io.Discard)
	gestorCfg, err := config.Inicializar(rutaCfg)
	if err != nil {
		t.Fatalf("Inicializar: %v", err)
	}
	gestorPer, err := permisos.Inicializar(tmpDir)
	if err != nil {
		t.Fatalf("Inicializar permisos: %v", err)
	}

	srv := Nuevo(gestorCfg, gestorPer, log)

	// Iniciar en background
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Iniciar()
	}()

	// Esperar brevemente a que arranque y el handler de señales esté activo.
	time.Sleep(300 * time.Millisecond)

	// Enviar SIGTERM al propio proceso para disparar el shutdown graceful.
	_ = syscallKill(syscallGetpid(), sigTERM)

	// Esperar a que Iniciar retorne (debería tras recibir la señal)
	select {
	case err := <-errChan:
		// OK — Iniciar retornó. Aceptamos nil o error.
		_ = err
	case <-time.After(3 * time.Second):
		t.Fatal("Iniciar no retornó tras SIGTERM")
	}
}

// =====================================================================
// Tests — Permisos Post con nivel vacío (default NivelTotal)
// =====================================================================

func TestHandlerPermisosPost_NivelVacio_UsaTotal(t *testing.T) {
	srv, _, gestorPer := setupTestServidor(t)

	body := `{"tipo": "archivos"}`
	req := httptest.NewRequest("POST", "/api/v1/permisos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	reg := gestorPer.ObtenerPermiso(permisos.PermArchivos)
	if reg == nil {
		t.Fatal("permiso no encontrado")
	}
	if reg.Nivel != permisos.NivelTotal {
		t.Errorf("Nivel = %s, esperaba total (default)", reg.Nivel)
	}
}

func TestHandlerPermisosPost_JSONInvalido(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	body := `not json`
	req := httptest.NewRequest("POST", "/api/v1/permisos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 con JSON inválido", rec.Code)
	}
}

// =====================================================================
// Tests — responderError y responderJSON (cobertura explícita)
// =====================================================================

func TestResponderError_Estructura(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	w := httptest.NewRecorder()
	srv.responderError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400", w.Code)
	}
	resp := parseRespuesta(t, w.Body.Bytes())
	if resp["exito"] != false {
		t.Error("exito debería ser false en error")
	}
	if resp["error"] != "test error" {
		t.Errorf("error = %v, esperaba 'test error'", resp["error"])
	}
	if resp["timestamp"] == nil {
		t.Error("timestamp debería estar presente")
	}
}

func TestResponderJSON_Estructura(t *testing.T) {
	srv, _, _ := setupTestServidor(t)
	w := httptest.NewRecorder()
	srv.responderJSON(w, http.StatusTeapot, map[string]string{"hello": "world"})

	if w.Code != http.StatusTeapot {
		t.Errorf("Status = %d, esperaba 418", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %s, esperaba application/json", ct)
	}
}

// =====================================================================
// Tests — Contexto handlers adicionales (rama de error)
// =====================================================================

func TestHandlerContextoIndice_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/indice", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoArbol_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/arbol", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoMapa_ProyectoNoIndexado_RayaBaja(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/proyecto_no_existe/mapa", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoFragmentosPorRuta_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/fragmentos?ruta=main.go", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoBuscar_SinPatron(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/x/buscar", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin patrón", rec.Code)
	}
}

func TestHandlerContextoResumen_SinRuta(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/x/resumen", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, esperaba 400 sin ruta", rec.Code)
	}
}

func TestHandlerContextoResumen_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("GET", "/api/v1/contexto/proyectos/no-existe/resumen?ruta=main.go", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

func TestHandlerContextoReindexar_ProyectoNoIndexado(t *testing.T) {
	srv, _, _ := setupTestServidorConContexto(t)

	req := httptest.NewRequest("POST", "/api/v1/contexto/proyectos/no-existe/reindexar", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, esperaba 404", rec.Code)
	}
}

// =====================================================================
// Tests — Config Recargar (caso de éxito)
// =====================================================================

func TestHandlerConfigRecargar_MensajeExito(t *testing.T) {
	srv, _, _ := setupTestServidor(t)

	req := httptest.NewRequest("POST", "/api/v1/config/recargar", nil)
	rec := httptest.NewRecorder()
	srv.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, esperaba 200 (body: %s)", rec.Code, rec.Body.String())
	}
	resp := parseRespuesta(t, rec.Body.Bytes())
	if resp["exito"] != true {
		t.Error("exito debería ser true")
	}
	if resp["mensaje"] == nil {
		t.Error("mensaje debería estar presente")
	}
}

// Variable dummy para asegurar uso de imports en tests futuros
var _ = contexto.EmpaquetarSolicitud{}
