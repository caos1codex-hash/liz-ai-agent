package orquestador

import (
        "context"
        "encoding/json"
        "fmt"
        "net/http"
        "net/http/httptest"
        "strings"
        "testing"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
)

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// crearGestorModelos crea un gestor de configuración en memoria con N modelos.
func crearGestorModelos(t *testing.T, modelos []config.ConfiguracionModelo) *config.Gestor {
        t.Helper()
        cfg := &config.Configuracion{
                Puerto:   3000,
                Host:     "localhost",
                Nombre:   "test",
                Version:  "test",
                Modelos:  modelos,
        }
        return config.NuevoGestorConConfig(cfg)
}

// modeloTest helper para crear ConfiguracionModelo rápidamente.
func modeloTest(nombre string, habilitado bool) config.ConfiguracionModelo {
        return config.ConfiguracionModelo{
                Nombre:     nombre,
                Proveedor:  "nvidia",
                APIKey:     "test-key",
                URL:        "http://test",
                Habilitado: habilitado,
        }
}

// servidorMockNVIDIA crea un servidor HTTP que simula la API de NVIDIA.
// handler es la función que maneja /chat/completions.
func servidorMockNVIDIA(t *testing.T, handler http.HandlerFunc) *httptest.Server {
        t.Helper()
        return httptest.NewServer(handler)
}

// orquestadorConMock crea un orquestador que apunta al servidor mock.
func orquestadorConMock(t *testing.T, endpoint string, modelos []config.ConfiguracionModelo) *Orquestador {
        t.Helper()
        g := crearGestorModelos(t, modelos)
        o, err := NuevoOrquestador(g)
        if err != nil {
                t.Fatalf("error creando orquestador: %v", err)
        }
        // Sobreescribir cliente con URL del mock
        o.cliente = NuevoClienteNVIDIA(endpoint, "test-key")
        return o
}

// ═══════════════════════════════════════════════════════
// TESTS CLIENTE NVIDIA
// ═══════════════════════════════════════════════════════

func TestClienteNVIDIA_ChatCompletion_Exitoso(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                if r.URL.Path != "/chat/completions" {
                        t.Errorf("path inesperado: %s", r.URL.Path)
                }
                // Verificar auth header
                auth := r.Header.Get("Authorization")
                if auth != "Bearer test-key" {
                        t.Errorf("auth incorrecto: %s", auth)
                }

                // Parsear request
                var req solicitudChatOpenAI
                if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                        t.Errorf("error parseando body: %v", err)
                }
                if req.Model != "test-model" {
                        t.Errorf("modelo incorrecto: %s", req.Model)
                }

                // Responder
                resp := `{
                        "id": "chatcmpl-123",
                        "model": "test-model",
                        "choices": [{
                                "index": 0,
                                "message": {"role": "assistant", "content": "Hola desde el modelo"},
                                "finish_reason": "stop"
                        }],
                        "usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
                }`
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(resp))
        })
        defer srv.Close()

        cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
        resp, apiErr, err := cliente.ChatCompletion(solicitudChatOpenAI{
                Model: "test-model",
                Messages: []mensajeOpenAI{{Role: "user", Content: "Hola"}},
        })

        if err != nil {
                t.Fatalf("error inesperado: %v", err)
        }
        if apiErr != nil {
                t.Fatalf("API error inesperado: %v", apiErr)
        }
        if resp.Choices[0].Message.Content != "Hola desde el modelo" {
                t.Errorf("contenido incorrecto: %s", resp.Choices[0].Message.Content)
        }
        if resp.Usage.TotalTokens != 15 {
                t.Errorf("tokens totales incorrectos: %d", resp.Usage.TotalTokens)
        }
}

func TestClienteNVIDIA_ChatCompletion_Error429_Reintentable(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusTooManyRequests)
                w.Write([]byte(`{"error": {"message": "rate limit", "type": "rate_limit_error", "code": "429"}}`))
        })
        defer srv.Close()

        cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
        _, apiErr, err := cliente.ChatCompletion(solicitudChatOpenAI{Model: "test"})

        if err != nil {
                t.Fatalf("error de red inesperado: %v", err)
        }
        if apiErr == nil {
                t.Fatal("debería retornar ErrorAPI")
        }
        if apiErr.Status != 429 {
                t.Errorf("status incorrecto: %d", apiErr.Status)
        }
        if !apiErr.Reintentable {
                t.Error("429 debería ser reinterrable")
        }
}

func TestClienteNVIDIA_ChatCompletion_Error401_NoReintentable(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusUnauthorized)
                w.Write([]byte(`{"error": {"message": "invalid api key", "type": "invalid_request_error"}}`))
        })
        defer srv.Close()

        cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
        _, apiErr, _ := cliente.ChatCompletion(solicitudChatOpenAI{Model: "test"})

        if apiErr == nil {
                t.Fatal("debería retornar ErrorAPI")
        }
        if apiErr.Reintentable {
                t.Error("401 NO debería ser reinterrable")
        }
}

func TestClienteNVIDIA_ChatCompletion_Error500_Reintentable(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusInternalServerError)
                w.Write([]byte(`{"error": {"message": "internal"}}`))
        })
        defer srv.Close()

        cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
        _, apiErr, _ := cliente.ChatCompletion(solicitudChatOpenAI{Model: "test"})

        if apiErr == nil {
                t.Fatal("debería retornar ErrorAPI")
        }
        if !apiErr.Reintentable {
                t.Error("500 debería ser reinterrable")
        }
}

// ═══════════════════════════════════════════════════════
// TESTS ORQUESTADOR — Selección
// ═══════════════════════════════════════════════════════

func TestSeleccionarModelo_SinEspecificar_UsaPrimerHabilitado(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {})
        defer srv.Close()

        modelos := []config.ConfiguracionModelo{
                modeloTest("m1", true),
                modeloTest("m2", true),
                modeloTest("m3", false), // deshabilitado
        }
        o := orquestadorConMock(t, srv.URL, modelos)

        elegido, fallback, err := o.SeleccionarModelo("", "")
        if err != nil {
                t.Fatalf("error: %v", err)
        }
        if elegido.Nombre != "m1" {
                t.Errorf("debería elegir m1, got %s", elegido.Nombre)
        }
        if len(fallback) != 1 {
                t.Errorf("fallback debería tener 1 modelo (m2), got %d", len(fallback))
        }
        if len(fallback) > 0 && fallback[0].Nombre != "m2" {
                t.Errorf("fallback debería ser m2, got %s", fallback[0].Nombre)
        }
}

func TestSeleccionarModelo_Especifico_LoUsa(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {})
        defer srv.Close()

        modelos := []config.ConfiguracionModelo{
                modeloTest("m1", true),
                modeloTest("m2", true),
        }
        o := orquestadorConMock(t, srv.URL, modelos)

        elegido, fallback, err := o.SeleccionarModelo("", "m2")
        if err != nil {
                t.Fatalf("error: %v", err)
        }
        if elegido.Nombre != "m2" {
                t.Errorf("debería elegir m2, got %s", elegido.Nombre)
        }
        if len(fallback) != 1 || fallback[0].Nombre != "m1" {
                t.Errorf("fallback debería ser [m1], got %v", fallback)
        }
}

func TestSeleccionarModelo_EspecificoDeshabilitado_Error(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {})
        defer srv.Close()

        modelos := []config.ConfiguracionModelo{
                modeloTest("m1", true),
                modeloTest("m2", false),
        }
        o := orquestadorConMock(t, srv.URL, modelos)

        _, _, err := o.SeleccionarModelo("", "m2")
        if err == nil {
                t.Error("debería retornar error para modelo deshabilitado")
        }
}

func TestSeleccionarModelo_SinModelosHabilitados_Error(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {})
        defer srv.Close()

        modelos := []config.ConfiguracionModelo{
                modeloTest("m1", false),
        }
        o := orquestadorConMock(t, srv.URL, modelos)

        _, _, err := o.SeleccionarModelo("", "")
        if err == nil {
                t.Error("debería retornar error cuando no hay modelos habilitados")
        }
}

func TestSeleccionarModelo_TareaCodigo_PrefiereCodeLlama(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {})
        defer srv.Close()

        modelos := []config.ConfiguracionModelo{
                modeloTest("llama-3.1-70b", true),
                modeloTest("codellama-70b", true), // debería elegirse para "codigo"
        }
        o := orquestadorConMock(t, srv.URL, modelos)

        elegido, _, err := o.SeleccionarModelo(TareaCodigo, "")
        if err != nil {
                t.Fatalf("error: %v", err)
        }
        // Sin métricas previas, debería preferir el que tiene "code" en el nombre
        if elegido.Nombre != "codellama-70b" {
                t.Errorf("para tarea=codigo debería preferir codellama, got %s", elegido.Nombre)
        }
}

// ═══════════════════════════════════════════════════════
// TESTS ORQUESTADOR — Completar con fallback
// ═══════════════════════════════════════════════════════

func TestCompletar_Exitoso_AlPrimerIntento(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(`{
                        "model": "m1",
                        "choices": [{"message": {"content": "respuesta"}, "finish_reason": "stop"}],
                        "usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
                }`))
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
        })

        resp, err := o.Completar(SolicitudChat{
                Mensajes: []MensajeChat{{Rol: "user", Contenido: "Hola"}},
        })
        if err != nil {
                t.Fatalf("error: %v", err)
        }
        if resp.Contenido != "respuesta" {
                t.Errorf("contenido incorrecto: %s", resp.Contenido)
        }
        if resp.Intentos != 1 {
                t.Errorf("debería tener 1 intento, got %d", resp.Intentos)
        }
        if resp.ModeloUsado != "m1" {
                t.Errorf("modelo usado incorrecto: %s", resp.ModeloUsado)
        }
        if resp.TokensTotal != 8 {
                t.Errorf("tokens totales incorrectos: %d", resp.TokensTotal)
        }
}

func TestCompletar_FallaPrimerModelo_UsaFallback(t *testing.T) {
        intentos := 0
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                intentos++
                var req solicitudChatOpenAI
                _ = json.NewDecoder(r.Body).Decode(&req)

                if req.Model == "m1" {
                        // Fallar con 500 (reintentable)
                        w.WriteHeader(http.StatusInternalServerError)
                        w.Write([]byte(`{"error": {"message": "internal"}}`))
                        return
                }
                // m2: éxito
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(`{
                        "model": "m2",
                        "choices": [{"message": {"content": "ok from m2"}, "finish_reason": "stop"}],
                        "usage": {"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8}
                }`))
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
                modeloTest("m2", true),
        })

        resp, err := o.Completar(SolicitudChat{
                Mensajes: []MensajeChat{{Rol: "user", Contenido: "Hola"}},
        })
        if err != nil {
                t.Fatalf("no debería retornar error: %v", err)
        }
        if resp.ModeloUsado != "m2" {
                t.Errorf("debería usar m2 tras fallback, got %s", resp.ModeloUsado)
        }
        if resp.Intentos != 2 {
                t.Errorf("debería tener 2 intentos, got %d", resp.Intentos)
        }
        if resp.Contenido != "ok from m2" {
                t.Errorf("contenido incorrecto: %s", resp.Contenido)
        }
}

func TestCompletar_TodosFalla_RetornaError(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusInternalServerError)
                w.Write([]byte(`{"error": {"message": "all fail"}}`))
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
                modeloTest("m2", true),
        })

        _, err := o.Completar(SolicitudChat{
                Mensajes: []MensajeChat{{Rol: "user", Contenido: "Hola"}},
        })
        if err == nil {
                t.Error("debería retornar error cuando todos los modelos fallan")
        }
}

func TestCompletar_ErrorNoReintentable_NoUsaFallback(t *testing.T) {
        intentos := 0
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                intentos++
                w.WriteHeader(http.StatusUnauthorized)
                w.Write([]byte(`{"error": {"message": "invalid key"}}`))
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
                modeloTest("m2", true),
        })

        _, err := o.Completar(SolicitudChat{
                Mensajes: []MensajeChat{{Rol: "user", Contenido: "Hola"}},
        })
        if err == nil {
                t.Error("debería retornar error")
        }
        if intentos != 1 {
                t.Errorf("error 401 no debería reintentar, got %d intentos", intentos)
        }
}

// ═══════════════════════════════════════════════════════
// TESTS MÉTRICAS
// ═══════════════════════════════════════════════════════

func TestMetricas_RegistraExitosYFallos(t *testing.T) {
        intentos := 0
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                intentos++
                if intentos == 1 {
                        // Primera vez: falla
                        w.WriteHeader(http.StatusInternalServerError)
                        w.Write([]byte(`{"error": {"message": "fail"}}`))
                        return
                }
                // Segunda vez: éxito
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(`{"model": "m1", "choices": [{"message": {"content": "ok"}, "finish_reason": "stop"}], "usage": {"total_tokens": 5}}`))
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
                modeloTest("m2", true),
        })

        // Primera solicitud: m1 falla, m2 exito
        _, _ = o.Completar(SolicitudChat{Mensajes: []MensajeChat{{Rol: "user", Contenido: "x"}}})

        metricas := o.Metricas()
        porModelo := make(map[string]MetricasModelo)
        for _, m := range metricas {
                porModelo[m.Modelo] = m
        }

        if porModelo["m1"].Fallos != 1 {
                t.Errorf("m1 debería tener 1 fallo, got %d", porModelo["m1"].Fallos)
        }
        if porModelo["m1"].Exitos != 0 {
                t.Errorf("m1 debería tener 0 éxitos, got %d", porModelo["m1"].Exitos)
        }
        if porModelo["m2"].Exitos != 1 {
                t.Errorf("m2 debería tener 1 éxito, got %d", porModelo["m2"].Exitos)
        }
}

// ═══════════════════════════════════════════════════════
// TESTS STREAMING
// ═══════════════════════════════════════════════════════

func TestCompletarStream_Exitoso(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "text/event-stream")
                flusher, ok := w.(http.Flusher)
                if !ok {
                        t.Fatal("ResponseWriter no soporta Flusher")
                }

                // Enviar 3 chunks SSE
                chunks := []string{
                        `data: {"choices":[{"delta":{"content":"Hola"},"finish_reason":null}]}`,
                        `data: {"choices":[{"delta":{"content":" mundo"},"finish_reason":null}]}`,
                        `data: {"choices":[{"delta":{"content":""},"finish_reason":"stop"}]}`,
                        `data: [DONE]`,
                }
                for _, c := range chunks {
                        fmt.Fprint(w, c+"\n\n")
                        flusher.Flush()
                }
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
        })

        ch, err := o.CompletarStream(context.Background(), SolicitudChat{
                Mensajes: []MensajeChat{{Rol: "user", Contenido: "Hola"}},
        })
        if err != nil {
                t.Fatalf("error: %v", err)
        }

        var contenido strings.Builder
        acabado := false
        for chunk := range ch {
                if chunk.Error != nil {
                        t.Errorf("error inesperado en chunk: %v", chunk.Error)
                }
                contenido.WriteString(chunk.Contenido)
                if chunk.Acabado {
                        acabado = true
                }
        }

        if !acabado {
                t.Error("debería recibir chunk de acabado")
        }
        if contenido.String() != "Hola mundo" {
                t.Errorf("contenido incorrecto: %q", contenido.String())
        }
}

func TestCompletarStream_ServidorCaeTarde_RetornaChunksParciales(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                w.Header().Set("Content-Type", "text/event-stream")
                flusher := w.(http.Flusher)
                fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"parcial"},"finish_reason":null}]}`+"\n\n")
                flusher.Flush()
                // Simular caída del servidor sin [DONE]
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
        })

        ch, _ := o.CompletarStream(context.Background(), SolicitudChat{
                Mensajes: []MensajeChat{{Rol: "user", Contenido: "x"}},
        })

        recibioParcial := false
        for chunk := range ch {
                if chunk.Contenido == "parcial" {
                        recibioParcial = true
                }
        }
        if !recibioParcial {
                t.Error("debería haber recibido el chunk parcial antes de la caída")
        }
}

// ═══════════════════════════════════════════════════════
// TESTS EMBEDDINGS
// ═══════════════════════════════════════════════════════

func TestEmbeddings_Exitoso(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                if r.URL.Path != "/embeddings" {
                        t.Errorf("path inesperado: %s", r.URL.Path)
                }
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(`{
                        "data": [
                                {"embedding": [0.1, 0.2, 0.3], "index": 0},
                                {"embedding": [0.4, 0.5, 0.6], "index": 1}
                        ],
                        "model": "nvidia/nv-embed-v1",
                        "usage": {"prompt_tokens": 10, "total_tokens": 10}
                }`))
        })
        defer srv.Close()

        cliente := NuevoClienteNVIDIA(srv.URL, "test-key")
        resp, apiErr, err := cliente.Embeddings([]string{"hola", "mundo"}, "")

        if err != nil {
                t.Fatalf("error inesperado: %v", err)
        }
        if apiErr != nil {
                t.Fatalf("API error: %v", apiErr)
        }
        if len(resp.Data) != 2 {
                t.Errorf("debería tener 2 embeddings, got %d", len(resp.Data))
        }
        if len(resp.Data[0].Embedding) != 3 {
                t.Errorf("dim incorrecta: %d", len(resp.Data[0].Embedding))
        }
}

// ═══════════════════════════════════════════════════════
// TESTS HELPERS
// ═══════════════════════════════════════════════════════

func TestToLower(t *testing.T) {
        if strings.ToLower("CodeLLAMA") != "codellama" {
                t.Error("ToLower incorrecto")
        }
}

func TestContainsSubstring(t *testing.T) {
        if !strings.Contains("codellama-70b", "code") {
                t.Error("debería contener 'code'")
        }
        if strings.Contains("llama-70b", "code") {
                t.Error("NO debería contener 'code'")
        }
        if !strings.Contains("anything", "") {
                t.Error("substring vacío debería retornar true")
        }
}

func TestParsearErrorAPI_FormatoOpenAI(t *testing.T) {
        body := []byte(`{"error": {"message": "test msg", "type": "test_type", "code": "test_code"}}`)
        apiErr := parsearErrorAPI(429, body)

        if apiErr.Status != 429 {
                t.Errorf("status incorrecto: %d", apiErr.Status)
        }
        if apiErr.Message != "test msg" {
                t.Errorf("message incorrecto: %s", apiErr.Message)
        }
        if apiErr.Type != "test_type" {
                t.Errorf("type incorrecto: %s", apiErr.Type)
        }
        if !apiErr.Reintentable {
                t.Error("429 debería ser reinterrable")
        }
}

func TestParsearErrorAPI_BodyInvalido_UsaRawBody(t *testing.T) {
        body := []byte("invalid json")
        apiErr := parsearErrorAPI(500, body)
        if apiErr.Message != "invalid json" {
                t.Errorf("debería usar raw body como message: %s", apiErr.Message)
        }
}

// Asegurar que la latencia se registra correctamente
func TestLatencia_SeRegistra(t *testing.T) {
        srv := servidorMockNVIDIA(t, func(w http.ResponseWriter, r *http.Request) {
                time.Sleep(50 * time.Millisecond)
                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(`{"model": "m1", "choices": [{"message": {"content": "ok"}, "finish_reason": "stop"}], "usage": {"total_tokens": 5}}`))
        })
        defer srv.Close()

        o := orquestadorConMock(t, srv.URL, []config.ConfiguracionModelo{
                modeloTest("m1", true),
        })

        _, _ = o.Completar(SolicitudChat{Mensajes: []MensajeChat{{Rol: "user", Contenido: "x"}}})

        metricas := o.Metricas()
        for _, m := range metricas {
                if m.Modelo == "m1" {
                        if m.LatenciaPromedio < 40*time.Millisecond {
                                t.Errorf("latencia debería ser >= 50ms, got %v", m.LatenciaPromedio)
                        }
                }
        }
}
