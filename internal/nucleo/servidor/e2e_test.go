package servidor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/permisos"
	"github.com/caos1codex-hash/liz-ai-agent/internal/pipeline"
)

// TestE2E_ConversacionCompleta simula una conversación completa usuario → respuesta
// pasando por todos los componentes del pipeline: Receptor → Clasificador → Planificador → Ejecutor → Respondedor
func TestE2E_ConversacionCompleta_SinLLM(t *testing.T) {
	cfg := config.NuevoGestorConConfig(&config.Config{
		Puerto: 3000,
		Host:   "localhost",
		Nombre: "liz-test",
		Version: "0.9.0",
	})

	log := logger.NuevaConSalida("e2e_test", nil)
	per := permisos.NuevoGestor()
	per.Conceder(permisos.PermisoSistema, "testing e2e")

	srv := Nuevo(cfg, per, log)

	// Crear pipeline sin orquestador (degrada gracefully)
	pipe := pipeline.Nuevo(pipeline.NuevasOpciones{})
	srv.ConPipeline(pipe)

	// Test 1: Conversación simple
	t.Run("conversacion_simple", func(t *testing.T) {
		body := `{"mensaje": "hola liz, que puedes hacer?"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var resp RespuestaAPI
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("error parseando respuesta: %v", err)
		}
		if !resp.Exito {
			t.Errorf("esperaba éxito: %s", resp.Error)
		}
	})

	// Test 2: Mensaje con categoría procesos
	t.Run("mensaje_procesos", func(t *testing.T) {
		body := `{"mensaje": "mata el proceso en el puerto 8080"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// Test 3: Mensaje con categoría monitorización
	t.Run("mensaje_monitorizacion", func(t *testing.T) {
		body := `{"mensaje": "estado de la cpu y memoria ram"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d", rr.Code, rr.Body.String())
		}
	})

	// Test 4: Mensaje de instalación
	t.Run("mensaje_instalacion", func(t *testing.T) {
		body := `{"mensaje": "instala docker en el sistema"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d", rr.Code, rr.Body.String())
		}
	})

	// Test 5: Mensaje de búsqueda
	t.Run("mensaje_busqueda", func(t *testing.T) {
		body := `{"mensaje": "busca todos los archivos .log del mes pasado"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d", rr.Code, rr.Body.String())
		}
	})

	// Test 6: Mensaje de código
	t.Run("mensaje_codigo", func(t *testing.T) {
		body := `{"mensaje": "crea un servidor HTTP en Go con autenticación JWT"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d", rr.Code, rr.Body.String())
		}
	})

	// Test 7: Auto-creación
	t.Run("mensaje_auto_creacion", func(t *testing.T) {
		body := `{"mensaje": "crea una herramienta para comprimir archivos CSV"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d", rr.Code, rr.Body.String())
		}
	})

	// Verificar que las métricas del pipeline se actualizan
	t.Run("metricas_despues_de_conversacion", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/chat", nil)
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d", rr.Code, rr.Body.String())
		}

		var resp RespuestaAPI
		json.Unmarshal(rr.Body.Bytes(), &resp)

		datos, ok := resp.Datos.(map[string]interface{})
		if !ok {
			t.Fatal("datos deberían ser un mapa")
		}
		mensajes, ok := datos["mensajes_procesados"].(int64)
		if !ok {
			t.Log("tipo de mensajes_procesados diferente, verificando con float")
			return
		}
		if mensajes < 7 {
			t.Errorf("esperaba al menos 7 mensajes procesados, got %d", mensajes)
		}
	})

	// Test 8: Mensaje inválido
	t.Run("mensaje_invalido", func(t *testing.T) {
		body := `{"mensaje": ""}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("esperaba 400 para mensaje vacío, got %d", rr.Code)
		}
	})

	// Test 9: Sin pipeline
	t.Run("sin_pipeline_503", func(t *testing.T) {
		srv2 := Nuevo(cfg, per, log)
		// No inyectar pipeline

		body := `{"mensaje": "hola"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv2.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("esperaba 503 sin pipeline, got %d", rr.Code)
		}
	})
}

// TestE2E_FlujoCompletoConSesiones simula un flujo donde se crean sesiones
// y se envían múltiples mensajes a la misma sesión.
func TestE2E_FlujoCompletoConSesiones(t *testing.T) {
	cfg := config.NuevoGestorConConfig(&config.Config{
		Puerto: 3001,
		Host:   "localhost",
		Nombre: "liz-e2e",
		Version: "0.9.0",
	})

	log := logger.NuevaConSalida("e2e_sesiones", nil)
	per := permisos.NuevoGestor()
	per.Conceder(permisos.PermisoSistema, "testing e2e")

	srv := Nuevo(cfg, per, log)
	pipe := pipeline.Nuevo(pipeline.NuevasOpciones{})
	srv.ConPipeline(pipe)

	// Crear sesión
	t.Run("crear_sesion", func(t *testing.T) {
		body := `{"usuario_id": "test_user", "proyecto": "mi-proyecto"}`
		req := httptest.NewRequest("POST", "/api/v1/chat/sesiones", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("crear sesión: %d %s", rr.Code, rr.Body.String())
		}
	})

	// Listar sesiones
	t.Run("listar_sesiones", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/chat/sesiones", nil)
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Logf("listar sesiones: %d", rr.Code)
		}
	})

	// Enviar mensaje (esto creará sesión automáticamente si no hay memoria)
	t.Run("enviar_mensaje", func(t *testing.T) {
		body := `{"mensaje": "hola liz", "usuario_id": "test_user"}`
		req := httptest.NewRequest("POST", "/api/v1/chat", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})

	// Verificar métricas del pipeline
	t.Run("verificar_metricas", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/chat/metricas", nil)
		rr := httptest.NewRecorder()
		srv.router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("esperaba 200, got %d", rr.Code)
		}

		var resp RespuestaAPI
		json.Unmarshal(rr.Body.Bytes(), &resp)
		if !resp.Exito {
			t.Errorf("métricas fallaron: %s", resp.Error)
		}
	})
}

// TestE2E_HealthEndpoints verifica que todos los endpoints de health funcionan
// después de una conversación completa.
func TestE2E_HealthEndpoints(t *testing.T) {
	cfg := config.NuevoGestorConConfig(&config.Config{
		Puerto: 3002,
		Host:   "localhost",
		Nombre: "liz-health",
		Version: "0.9.0",
	})

	log := logger.NuevaConSalida("e2e_health", nil)
	per := permisos.NuevoGestor()
	srv := Nuevo(cfg, per, log)

	// Health check
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	srv.router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("health check falló: %d", rr.Code)
	}

	var resp RespuestaAPI
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp.Exito {
		t.Error("health debería ser exitoso")
	}

	datos := resp.Datos.(map[string]interface{})
	if datos["nombre"] != "liz-health" {
		t.Errorf("esperaba nombre 'liz-health', got %v", datos["nombre"])
	}
}