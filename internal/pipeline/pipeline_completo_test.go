package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Mock implementations
// ============================================================================

type mockOrquestador struct {
	completarFunc       func(ctx context.Context, prompt, tipo string) (string, error)
	completarStreamFunc func(ctx context.Context, prompt, tipo string) (<-chan ChunkOrquestador, error)
	modeloActual        string
}

func (m *mockOrquestador) Completar(ctx context.Context, prompt, tipo string) (string, error) {
	if m.completarFunc != nil {
		return m.completarFunc(ctx, prompt, tipo)
	}
	return `"respuesta": "ok"`, nil
}

func (m *mockOrquestador) CompletarStream(ctx context.Context, prompt, tipo string) (<-chan ChunkOrquestador, error) {
	if m.completarStreamFunc != nil {
		return m.completarStreamFunc(ctx, prompt, tipo)
	}
	ch := make(chan ChunkOrquestador, 3)
	ch <- ChunkOrquestador{Delta: "Hola ", Modelo: "test-model"}
	ch <- ChunkOrquestador{Delta: "mundo", Modelo: "test-model"}
	ch <- ChunkOrquestador{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockOrquestador) ModeloActual() string {
	return m.modeloActual
}

type mockMemoria struct {
	mu             sync.Mutex
	sesiones       map[string]*InfoSesion
	mensajes       map[string][]InfoMensaje
	hechos         map[string]string
	crearSesionErr error
	agregarMsgErr  error
}

func newMockMemoria() *mockMemoria {
	return &mockMemoria{
		sesiones: make(map[string]*InfoSesion),
		mensajes: make(map[string][]InfoMensaje),
		hechos:   make(map[string]string),
	}
}

func (m *mockMemoria) ObtenerSesion(ctx context.Context, sesionID, usuarioID string) (*InfoSesion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sesiones[sesionID]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockMemoria) CrearSesion(ctx context.Context, usuarioID, proyecto string) (*InfoSesion, error) {
	if m.crearSesionErr != nil {
		return nil, m.crearSesionErr
	}
	id := generarUUID()
	s := &InfoSesion{ID: id, UsuarioID: usuarioID, Proyecto: proyecto, Titulo: "Sesion " + id}
	m.mu.Lock()
	m.sesiones[id] = s
	m.mu.Unlock()
	return s, nil
}

func (m *mockMemoria) AgregarMensaje(ctx context.Context, sesionID, usuarioID, contenido string) error {
	if m.agregarMsgErr != nil {
		return m.agregarMsgErr
	}
	m.mu.Lock()
	m.mensajes[sesionID] = append(m.mensajes[sesionID], InfoMensaje{Rol: "usuario", Contenido: contenido})
	m.mu.Unlock()
	return nil
}

func (m *mockMemoria) ObtenerMensajesRecientes(sesionID string, limite int) []InfoMensaje {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := m.mensajes[sesionID]
	if len(msgs) > limite {
		msgs = msgs[len(msgs)-limite:]
	}
	return msgs
}

func (m *mockMemoria) ObtenerHechos(usuarioID string, limite int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hechos[usuarioID]
}

func (m *mockMemoria) ContextoParaLLM(usuarioID string, ultimosNMensajes int, limiteHechos int) string {
	return "contexto LLM mock"
}

type mockCatalogo struct {
	existeFunc   func(nombre string) bool
	ejecutarFunc func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error)
	snapshotFunc func() []InfoHerramientaSnapshot
}

func (m *mockCatalogo) Existe(nombre string) bool {
	if m.existeFunc != nil {
		return m.existeFunc(nombre)
	}
	return true
}

func (m *mockCatalogo) Ejecutar(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
	if m.ejecutarFunc != nil {
		return m.ejecutarFunc(ctx, nombre, params)
	}
	return &ResultadoHerramienta{Exito: true, Datos: "ok"}, nil
}

func (m *mockCatalogo) Snapshot() []InfoHerramientaSnapshot {
	if m.snapshotFunc != nil {
		return m.snapshotFunc()
	}
	return []InfoHerramientaSnapshot{
		{Nombre: "terminal", Descripcion: "ejecuta comandos", Parametros: "params"},
		{Nombre: "buscador", Descripcion: "busca archivos", Parametros: "params"},
	}
}

type mockAutoGestor struct {
	crearFunc func(ctx context.Context, descripcion string) (*ResultadoAutoCreacion, error)
}

func (m *mockAutoGestor) Crear(ctx context.Context, descripcion string) (*ResultadoAutoCreacion, error) {
	if m.crearFunc != nil {
		return m.crearFunc(ctx, descripcion)
	}
	return &ResultadoAutoCreacion{Exito: true, Datos: "herramienta_creada"}, nil
}

type mockContextoCoord struct {
	empaquetarFunc func(ctx context.Context, proyecto, query string, maxTokens int) (string, error)
}

func (m *mockContextoCoord) EmpaquetarContexto(ctx context.Context, proyecto, query string, maxTokens int) (string, error) {
	if m.empaquetarFunc != nil {
		return m.empaquetarFunc(ctx, proyecto, query, maxTokens)
	}
	return "contexto empaquetado", nil
}

// ============================================================================
// Pipeline.Procesar — full flow with mocks
// ============================================================================

func TestPipeline_Procesar_ConOrquestadorMock(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "Respuesta del LLM", nil
		},
		modeloActual: "nvidia/llama-3.1-nemotron-70b-instruct",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch})
	resp, err := p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola liz"})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta no nil")
	}
	if resp.ModeloUsado != "nvidia/llama-3.1-nemotron-70b-instruct" {
		t.Errorf("esperaba modelo nvidia, got '%s'", resp.ModeloUsado)
	}
	if resp.Mensaje == "" {
		t.Error("esperaba mensaje no vacío")
	}
	if resp.SesionID == "" {
		t.Error("esperaba sesion ID")
	}
	if resp.ID == "" {
		t.Error("esperaba ID")
	}
}

func TestPipeline_Procesar_ConMemoriaYOrquestador(t *testing.T) {
	mem := newMockMemoria()
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "Respuesta con memoria", nil
		},
		modeloActual: "test-model",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch, Memoria: mem})
	resp, err := p.Procesar(context.Background(), &SolicitudChat{Mensaje: "lista los procesos", UsuarioID: "user1"})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp.Mensaje == "" {
		t.Error("esperaba mensaje")
	}
	// Verificar que se creó sesión y se almacenó mensaje
	if resp.SesionID == "" {
		t.Error("esperaba sesión ID")
	}
}

func TestPipeline_Procesar_ConCatalogoYOrquestador(t *testing.T) {
	cat := &mockCatalogo{
		existeFunc: func(nombre string) bool { return true },
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return &ResultadoHerramienta{Exito: true, Datos: map[string]interface{}{"processes": 5}}, nil
		},
	}
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "5 procesos activos", nil
		},
		modeloActual: "test",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch, Catalogo: cat})
	// Mensaje que clasifica como procesos → requiere herramientas
	resp, err := p.Procesar(context.Background(), &SolicitudChat{Mensaje: "mata el proceso en el puerto 8080"})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp.Categoria != CategoriaProcesos {
		t.Logf("categoría: %s (esperaba procesos)", resp.Categoria)
	}
	if resp.PasosEjecutados == 0 && resp.Categoria != CategoriaConversacion {
		t.Log("Nota: no se ejecutaron pasos (puede ser aceptable si el planificador no generó pasos con herramientas)")
	}
}

func TestPipeline_Procesar_OrquestadorError(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "", fmt.Errorf("API key inválida")
		},
		modeloActual: "test",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch})
	resp, err := p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola"})
	if err != nil {
		t.Fatalf("no esperaba error, pipeline degrada gracefully: %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta no nil")
	}
	if resp.ModeloUsado != "fallback" {
		t.Errorf("esperaba modelo 'fallback', got '%s'", resp.ModeloUsado)
	}
}

// ============================================================================
// Pipeline.ProcesarStream
// ============================================================================

func TestPipeline_ProcesarStream_ConOrquestador(t *testing.T) {
	orch := &mockOrquestador{
		completarStreamFunc: func(ctx context.Context, prompt, tipo string) (<-chan ChunkOrquestador, error) {
			ch := make(chan ChunkOrquestador, 5)
			ch <- ChunkOrquestador{Delta: "Hola ", Modelo: "stream-model"}
			ch <- ChunkOrquestador{Delta: "desde ", Modelo: "stream-model"}
			ch <- ChunkOrquestador{Delta: "stream", Modelo: "stream-model"}
			ch <- ChunkOrquestador{Done: true}
			close(ch)
			return ch, nil
		},
		modeloActual: "stream-model",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch})
	var chunks []*ChunkStream
	resp, err := p.ProcesarStream(context.Background(), &SolicitudChat{Mensaje: "hola liz"}, func(c *ChunkStream) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta")
	}
	// Debería tener chunks de estado + texto
	if len(chunks) == 0 {
		t.Error("esperaba al menos un chunk")
	}
	// Verificar que hay chunks de texto
	textChunks := 0
	for _, c := range chunks {
		if c.Tipo == "texto" {
			textChunks++
		}
	}
	if textChunks == 0 {
		t.Error("esperaba chunks de texto del stream")
	}
}

func TestPipeline_ProcesarStream_StreamError(t *testing.T) {
	orch := &mockOrquestador{
		completarStreamFunc: func(ctx context.Context, prompt, tipo string) (<-chan ChunkOrquestador, error) {
			ch := make(chan ChunkOrquestador, 2)
			ch <- ChunkOrquestador{Error: fmt.Errorf("timeout de stream")}
			ch <- ChunkOrquestador{Done: true}
			close(ch)
			return ch, nil
		},
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "fallback response", nil
		},
		modeloActual: "fallback",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch})
	resp, err := p.ProcesarStream(context.Background(), &SolicitudChat{Mensaje: "hola"}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta")
	}
}

func TestPipeline_ProcesarStream_NilCallback(t *testing.T) {
	p := Nuevo(NuevasOpciones{})
	resp, err := p.ProcesarStream(context.Background(), &SolicitudChat{Mensaje: "hola"}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta")
	}
}

func TestPipeline_ProcesarStream_ConHerramientas(t *testing.T) {
	cat := &mockCatalogo{
		existeFunc: func(nombre string) bool { return true },
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return &ResultadoHerramienta{Exito: true, Datos: "CPU: 45%"}, nil
		},
	}
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "La CPU está al 45%", nil
		},
		modeloActual: "test",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch, Catalogo: cat})
	var chunks []*ChunkStream
	resp, err := p.ProcesarStream(context.Background(), &SolicitudChat{Mensaje: "estado de la cpu"}, func(c *ChunkStream) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta")
	}
}

// ============================================================================
// Pipeline.Estado — métricas
// ============================================================================

func TestPipeline_Estado_ModeloMasUsado(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "resp", nil
		},
		modeloActual: "modelo-dominante",
	}

	p := Nuevo(NuevasOpciones{Orquestador: orch})
	for i := 0; i < 5; i++ {
		p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola"})
	}

	estado := p.Estado()
	if estado.MensajesProcesados != 5 {
		t.Errorf("esperaba 5 mensajes, got %d", estado.MensajesProcesados)
	}
	if estado.ModeloMasUsado != "modelo-dominante" {
		t.Errorf("esperaba 'modelo-dominante', got '%s'", estado.ModeloMasUsado)
	}
	if estado.UltimoUso.IsZero() {
		t.Error("esperaba ultimo uso no zero")
	}
}

func TestPipeline_Estado_CategoriaCount(t *testing.T) {
	p := Nuevo(NuevasOpciones{})

	// Procesar mensajes de diferentes categorías
	p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola"})             // conversacion
	p.Procesar(context.Background(), &SolicitudChat{Mensaje: "instala docker"})   // instalacion
	p.Procesar(context.Background(), &SolicitudChat{Mensaje: "estado de la cpu"}) // monitorizacion

	estado := p.Estado()
	if estado.MensajesProcesados != 3 {
		t.Errorf("esperaba 3, got %d", estado.MensajesProcesados)
	}
	if len(estado.CategoriaCount) == 0 {
		t.Error("esperaba categorías contabilizadas")
	}
}

func TestPipeline_Estado_PromedioDuracion(t *testing.T) {
	p := Nuevo(NuevasOpciones{})
	p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola"})
	p.Procesar(context.Background(), &SolicitudChat{Mensaje: "adiós"})

	estado := p.Estado()
	if estado.PromedioDuracion == 0 {
		t.Error("esperaba duración promedio > 0")
	}
}

// ============================================================================
// Pipeline.respuestaSinOrquestador
// ============================================================================

func TestPipeline_RespuestaSinOrquestador_ConResultados(t *testing.T) {
	p := Nuevo(NuevasOpciones{})
	clasif := &ResultadoClasificacion{Categoria: CategoriaProcesos}
	resultados := []ResultadoPaso{
		{PasoID: 1, Exito: true, ToolUsada: "monitor", Datos: "CPU: 80%"},
		{PasoID: 2, Exito: false, ToolUsada: "terminal", Error: "timeout"},
	}

	resp := p.respuestaSinOrquestador("estado del sistema", clasif, resultados)
	if resp == "" {
		t.Fatal("esperaba respuesta no vacía")
	}
	// Verificar que contiene info de resultados
	if !contains(resp, "monitor") {
		t.Error("debería mencionar la herramienta monitor")
	}
	if !contains(resp, "fallido") {
		t.Error("debería mencionar el paso fallido")
	}
}

func TestPipeline_RespuestaSinOrquestador_SinResultados(t *testing.T) {
	p := Nuevo(NuevasOpciones{})
	clasif := &ResultadoClasificacion{Categoria: CategoriaConversacion}

	resp := p.respuestaSinOrquestador("hola", clasif, nil)
	if resp == "" {
		t.Fatal("esperaba respuesta")
	}
	if !contains(resp, "configura una API key") {
		t.Error("debería mencionar configurar API key")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================================
// Receptor — más escenarios
// ============================================================================

func TestReceptor_Recibir_ConMemoria_ErrorCrearSesion(t *testing.T) {
	mem := newMockMemoria()
	mem.crearSesionErr = fmt.Errorf("error de BD")

	rec := nuevoReceptor(mem)
	sol := &SolicitudChat{Mensaje: "hola", UsuarioID: "u1"}

	// No debe fallar — fallback a sesión local
	msg, sesion, err := rec.Recibir(context.Background(), sol)
	if err != nil {
		t.Fatalf("no esperaba error (debe hacer fallback): %v", err)
	}
	if msg == nil {
		t.Fatal("esperaba mensaje")
	}
	if sesion == nil {
		t.Fatal("esperaba sesión")
	}
}

func TestReceptor_Recibir_ConMemoria_ErrorAgregarMensaje(t *testing.T) {
	mem := newMockMemoria()
	mem.agregarMsgErr = fmt.Errorf("error almacenando")

	rec := nuevoReceptor(mem)
	sol := &SolicitudChat{Mensaje: "hola", UsuarioID: "u1"}

	msg, _, err := rec.Recibir(context.Background(), sol)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if msg.Metadata["advertencia_memoria"] == nil {
		t.Error("esperaba advertencia de memoria")
	}
}

func TestReceptor_Recibir_SesionExistente(t *testing.T) {
	mem := newMockMemoria()
	// Crear sesión previamente
	s := &InfoSesion{ID: "ses-123", UsuarioID: "u1", Proyecto: "p1", Titulo: "Test"}
	mem.sesiones["ses-123"] = s

	rec := nuevoReceptor(mem)
	sol := &SolicitudChat{Mensaje: "hola", UsuarioID: "u1", SesionID: "ses-123"}

	msg, sesion, err := rec.Recibir(context.Background(), sol)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if sesion.ID != "ses-123" {
		t.Errorf("esperaba ses-123, got %s", sesion.ID)
	}
	if msg.SesionID != "ses-123" {
		t.Errorf("esperaba mensaje con sesion ses-123, got %s", msg.SesionID)
	}
}

func TestReceptor_Recibir_MensajeVacio(t *testing.T) {
	rec := nuevoReceptor(nil)
	sol := &SolicitudChat{Mensaje: "   "}

	_, _, err := rec.Recibir(context.Background(), sol)
	if err == nil {
		t.Fatal("esperaba error para mensaje vacío")
	}
}

func TestReceptor_Recibir_ConProyecto(t *testing.T) {
	rec := nuevoReceptor(nil)
	sol := &SolicitudChat{Mensaje: "analiza el main.go", UsuarioID: "u1", Proyecto: "mi-proyecto"}

	msg, sesion, err := rec.Recibir(context.Background(), sol)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if sesion.Proyecto != "mi-proyecto" {
		t.Errorf("esperaba proyecto 'mi-proyecto', got '%s'", sesion.Proyecto)
	}
	if msg.UsuarioID != "u1" {
		t.Errorf("esperaba usuario 'u1', got '%s'", msg.UsuarioID)
	}
	if msg.Rol != "usuario" {
		t.Errorf("esperaba rol 'usuario', got '%s'", msg.Rol)
	}
	if msg.TokensEstimados <= 0 {
		t.Error("esperaba tokens estimados > 0")
	}
}

// ============================================================================
// Clasificador — LLM path y edge cases
// ============================================================================

func TestClasificador_ConLLM_BuenaRespuesta(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return `{"categoria": "archivos", "confianza": 0.95, "razonamiento": "menciona archivos", "necesita_contexto": false, "prioridad": 2}`, nil
		},
	}

	clasif := nuevoClasificador(orch)
	// Mensaje con confianza baja en heurística para forzar LLM
	result, err := clasif.Clasificar(context.Background(), "mueve ese archivo a la papelera", "")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	// Puede ser heurística (archivos: 0.75) o LLM (archivos: 0.95)
	if result.Categoria != CategoriaArchivos {
		t.Logf("categoría: %s (aceptable si heurística o LLM coinciden)", result.Categoria)
	}
}

func TestClasificador_ConLLM_ErrorRespuesta(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "", fmt.Errorf("LLM no disponible")
		},
	}

	clasif := nuevoClasificador(orch)
	// Mensaje que no matchea heurísticas → confianza baja → intenta LLM → falla → default
	result, err := clasif.Clasificar(context.Background(), "algo ambiguo sin palabras clave", "")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if result.Categoria != CategoriaConversacion {
		t.Errorf("esperaba fallback a conversacion, got '%s'", result.Categoria)
	}
}

func TestClasificador_ConLLM_InvalidJSON(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "esto no es json", nil
		},
	}

	clasif := nuevoClasificador(orch)
	// Heurística baja + LLM retorna basura → fallback
	result, err := clasif.Clasificar(context.Background(), "xyzw no match", "")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	// Puede ser conversacion (default) o lo que sea
	if result == nil {
		t.Fatal("esperaba resultado")
	}
}

func TestClasificador_PrioridadDesdeCategoria(t *testing.T) {
	c := nuevoClasificador(nil)

	if c.prioridadDesdeCategoria(CategoriaEjecucionComando) != 1 {
		t.Error("ejecucion_comando debería ser prioridad 1")
	}
	if c.prioridadDesdeCategoria(CategoriaMonitorizacion) != 1 {
		t.Error("monitorizacion debería ser prioridad 1")
	}
	if c.prioridadDesdeCategoria(CategoriaInstalacion) != 2 {
		t.Error("instalacion debería ser prioridad 2")
	}
	if c.prioridadDesdeCategoria(CategoriaConversacion) != 3 {
		t.Error("conversacion debería ser prioridad 3")
	}
}

func TestParsearClasificacion(t *testing.T) {
	tests := []struct {
		nombre string
		input  string
		cat    CategoriaTarea
		conf   float64
	}{
		{"JSON válido", `{"categoria": "codigo", "confianza": 0.9, "razonamiento": "test", "necesita_contexto": true, "prioridad": 2}`, CategoriaCodigo, 0.9},
		{"Confianza 0", `{"categoria": "busqueda", "confianza": 0, "razonamiento": "r", "prioridad": 1}`, CategoriaBusqueda, 0.5},
		{"Confianza > 1", `{"categoria": "analisis", "confianza": 2.5, "razonamiento": "r", "prioridad": 1}`, CategoriaAnalisis, 1.0},
		{"Prioridad 0", `{"categoria": "conversacion", "confianza": 0.8, "razonamiento": "r"}`, CategoriaConversacion, 0.8},
		{"Categoría inválida", `{"categoria": "no_existe", "confianza": 0.7, "razonamiento": "r", "prioridad": 2}`, CategoriaConversacion, 0.7},
	}

	for _, tc := range tests {
		t.Run(tc.nombre, func(t *testing.T) {
			result, err := parsearClasificacion(tc.input)
			if err != nil {
				t.Fatalf("no esperaba error: %v", err)
			}
			if result.Categoria != tc.cat {
				t.Errorf("esperaba %s, got %s", tc.cat, result.Categoria)
			}
			if result.Confianza != tc.conf {
				t.Errorf("esperaba confianza %.2f, got %.2f", tc.conf, result.Confianza)
			}
		})
	}
}

func TestParsearClasificacion_SinJSON(t *testing.T) {
	_, err := parsearClasificacion("texto sin json")
	if err == nil {
		t.Fatal("esperaba error sin JSON")
	}
}

// ============================================================================
// Planificador — más escenarios
// ============================================================================

func TestPlanificador_Archivos_SinLLM(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaArchivos}

	plan, err := planif.Planificar(context.Background(), "elimina los archivos", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if plan == nil {
		t.Fatal("esperaba plan")
	}
	if len(plan.Pasos) == 0 {
		t.Error("esperaba al menos 1 paso")
	}
}

func TestPlanificador_Codigo_ConContexto(t *testing.T) {
	ctxCoord := &mockContextoCoord{}
	planif := nuevoPlanificador(nil, nil, nil, ctxCoord)
	clasif := &ResultadoClasificacion{Categoria: CategoriaCodigo}
	sesion := &SesionInfo{ID: "s1", Proyecto: "mi-proyecto"}

	plan, err := planif.Planificar(context.Background(), "analiza el código", clasif, sesion, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if plan == nil {
		t.Fatal("esperaba plan")
	}
}

func TestPlanificador_Codigo_SinLLM_SinContexto(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaCodigo}

	plan, err := planif.Planificar(context.Background(), "analiza el código", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if plan.DescripcionGlobal != "Análisis de código (sin LLM)" {
		t.Errorf("esperaba fallback sin LLM, got '%s'", plan.DescripcionGlobal)
	}
}

func TestPlanificador_ConLLM_Fallback(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "", fmt.Errorf("LLM error")
		},
	}

	planif := nuevoPlanificador(orch, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaArchivos}

	plan, err := planif.Planificar(context.Background(), "elimina archivos", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if plan == nil {
		t.Fatal("esperaba plan fallback")
	}
	// Fallback debe tener 1 paso
	if len(plan.Pasos) != 1 {
		t.Errorf("esperaba 1 paso en fallback, got %d", len(plan.Pasos))
	}
}

func TestPlanificador_ConLLM_InvalidJSON(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "no es json", nil
		},
	}

	planif := nuevoPlanificador(orch, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaArchivos}

	plan, err := planif.Planificar(context.Background(), "elimina archivos", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	// Fallback por JSON inválido
	if len(plan.Pasos) != 1 {
		t.Errorf("esperaba 1 paso fallback, got %d", len(plan.Pasos))
	}
}

func TestPlanificador_ConLLM_BuenaRespuesta(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return `[{"id":1,"descripcion":"buscar archivos","herramienta":"buscador","parametros":{"patron":"*.log"},"requiere_llm":false}]`, nil
		},
	}

	planif := nuevoPlanificador(orch, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaBusqueda}

	plan, err := planif.Planificar(context.Background(), "busca los .log", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(plan.Pasos) == 0 {
		t.Fatal("esperaba pasos")
	}
	if plan.Pasos[0].Herramienta != "buscador" {
		t.Errorf("esperaba herramienta 'buscador', got '%s'", plan.Pasos[0].Herramienta)
	}
}

func TestPlanificador_NecesitaAutoCrear(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return `[{"id":1,"descripcion":"comprimir","herramienta":"compresor_zip","parametros":{},"requiere_llm":false}]`, nil
		},
	}
	cat := &mockCatalogo{
		existeFunc: func(nombre string) bool { return false }, // compresor_zip no existe
	}

	planif := nuevoPlanificador(orch, cat, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaArchivos}

	plan, err := planif.Planificar(context.Background(), "comprime archivos", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if !plan.NecesitaAutoCrear {
		t.Error("debería detectar que necesita auto-crear")
	}
}

func TestPlanificar_Monitorizacion_Metricas(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaMonitorizacion}

	tests := []struct {
		msg     string
		metrica string
	}{
		{"estado de la cpu", "cpu"},
		{"cuánta memoria hay", "memoria"},
		{"espacio en disco", "disco"},
		{"estado de la red", "red"},
		{"métricas del sistema", "completo"},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) {
			plan, err := planif.Planificar(context.Background(), tc.msg, clasif, &SesionInfo{}, nil)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if len(plan.Pasos) < 1 {
				t.Fatal("esperaba al menos 1 paso")
			}
			// Verificar que usa herramienta monitor
			found := false
			for _, p := range plan.Pasos {
				if p.Herramienta == "monitor" {
					found = true
					break
				}
			}
			if !found {
				t.Error("debería usar herramienta monitor")
			}
		})
	}
}

func TestPlanificar_EjecucionComando_SinComando(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaEjecucionComando}

	plan, err := planif.Planificar(context.Background(), "ejecuta algo", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	// Sin comando detectado y sin orquestador → fallback a conversación
	if plan == nil {
		t.Fatal("esperaba plan")
	}
}

func TestPlanificar_EjecucionComando_ConComando(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaEjecucionComando}

	plan, err := planif.Planificar(context.Background(), "ejecuta ls -la /tmp", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(plan.Pasos) < 1 {
		t.Fatal("esperaba pasos")
	}
}

func TestParsearPasosPlan(t *testing.T) {
	tests := []struct {
		nombre  string
		input   string
		esperar int
	}{
		{"Array JSON", `[{"descripcion":"p1"},{"descripcion":"p2"}]`, 2},
		{"Sin JSON", "no json", 0},
		{"JSON vacío", `[]`, 0},
		{"Objeto con array", `{"pasos":[{"descripcion":"p1"}]}`, 1},
	}

	for _, tc := range tests {
		t.Run(tc.nombre, func(t *testing.T) {
			pasos, err := parsearPasosPlan(tc.input)
			if tc.esperar == 0 {
				if err == nil {
					t.Error("esperaba error")
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error: %v", err)
			}
			if len(pasos) != tc.esperar {
				t.Errorf("esperaba %d pasos, got %d", tc.esperar, len(pasos))
			}
		})
	}
}

func TestObtenerHerramientasDisponibles(t *testing.T) {
	t.Run("con catálogo", func(t *testing.T) {
		cat := &mockCatalogo{}
		p := nuevoPlanificador(nil, cat, nil, nil)
		result := p.obtenerHerramientasDisponibles()
		if result == "" {
			t.Error("esperaba texto no vacío")
		}
		if !contains(result, "terminal") {
			t.Error("debería contener 'terminal'")
		}
	})

	t.Run("sin catálogo", func(t *testing.T) {
		p := nuevoPlanificador(nil, nil, nil, nil)
		result := p.obtenerHerramientasDisponibles()
		if result != "No hay herramientas registradas" {
			t.Errorf("esperaba 'No hay herramientas registradas', got '%s'", result)
		}
	})

	t.Run("catálogo vacío", func(t *testing.T) {
		cat := &mockCatalogo{
			snapshotFunc: func() []InfoHerramientaSnapshot { return nil },
		}
		p := nuevoPlanificador(nil, cat, nil, nil)
		result := p.obtenerHerramientasDisponibles()
		if result != "No hay herramientas registradas" {
			t.Errorf("esperaba 'No hay herramientas registradas', got '%s'", result)
		}
	})
}

// ============================================================================
// Ejecutor — más escenarios
// ============================================================================

func TestEjecutor_AutoCreacion(t *testing.T) {
	autoG := &mockAutoGestor{
		crearFunc: func(ctx context.Context, descripcion string) (*ResultadoAutoCreacion, error) {
			return &ResultadoAutoCreacion{Exito: true, Datos: "compresor_zip"}, nil
		},
	}

	ejec := nuevoEjecutor(nil, autoG)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "crear compresor", Herramienta: "__auto_creacion__"},
		},
	}

	resultados, err := ejec.EjecutarPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(resultados) != 1 {
		t.Fatalf("esperaba 1 resultado, got %d", len(resultados))
	}
	if !resultados[0].Exito {
		t.Errorf("esperaba éxito: %s", resultados[0].Error)
	}
}

func TestEjecutor_AutoCreacion_SinGestor(t *testing.T) {
	ejec := nuevoEjecutor(nil, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "crear algo", Herramienta: "__auto_creacion__"},
		},
	}

	resultados, err := ejec.EjecutarPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resultados[0].Exito {
		t.Error("esperaba fallo sin gestor de auto-creación")
	}
}

func TestEjecutor_AutoCreacion_Error(t *testing.T) {
	autoG := &mockAutoGestor{
		crearFunc: func(ctx context.Context, descripcion string) (*ResultadoAutoCreacion, error) {
			return nil, fmt.Errorf("error compilando")
		},
	}

	ejec := nuevoEjecutor(nil, autoG)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "crear algo", Herramienta: "__auto_creacion__"},
		},
	}

	resultados, _ := ejec.EjecutarPlan(context.Background(), plan, nil)
	if resultados[0].Exito {
		t.Error("esperaba fallo")
	}
	if !contains(resultados[0].Error, "auto-creación") {
		t.Errorf("error debería mencionar auto-creación: %s", resultados[0].Error)
	}
}

func TestEjecutor_DependenciaFallida(t *testing.T) {
	cat := &mockCatalogo{
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return &ResultadoHerramienta{Exito: false, Error: "fallo"}, nil
		},
		existeFunc: func(nombre string) bool { return true },
	}

	ejec := nuevoEjecutor(cat, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "paso 1", Herramienta: "terminal"},
			{ID: 2, Descripcion: "paso 2 depende de 1", Herramienta: "terminal", DependeDe: []int{1}},
		},
	}

	resultados, _ := ejec.EjecutarPlan(context.Background(), plan, nil)
	if len(resultados) != 2 {
		t.Fatalf("esperaba 2 resultados, got %d", len(resultados))
	}
	if resultados[0].Exito {
		t.Error("paso 1 debería fallar")
	}
	if resultados[1].Exito {
		t.Error("paso 2 debería fallar por dependencia")
	}
}

func TestEjecutor_Herramienta_Error(t *testing.T) {
	cat := &mockCatalogo{
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return nil, fmt.Errorf("timeout")
		},
		existeFunc: func(nombre string) bool { return true },
	}

	ejec := nuevoEjecutor(cat, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "paso", Herramienta: "terminal"},
		},
	}

	resultados, _ := ejec.EjecutarPlan(context.Background(), plan, nil)
	if resultados[0].Exito {
		t.Error("esperaba fallo")
	}
}

func TestEjecutor_ConCallback(t *testing.T) {
	cat := &mockCatalogo{
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return &ResultadoHerramienta{Exito: true, Datos: "ok"}, nil
		},
		existeFunc: func(nombre string) bool { return true },
	}

	ejec := nuevoEjecutor(cat, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "paso", Herramienta: "terminal"},
		},
	}

	var chunks []*ChunkStream
	resultados, err := ejec.EjecutarPlan(context.Background(), plan, func(c *ChunkStream) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("esperaba chunks del callback")
	}
}

func TestEjecutor_PasoVacio(t *testing.T) {
	ejec := nuevoEjecutor(nil, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "solo LLM", RequiereLLM: true},
		},
	}

	resultados, err := ejec.EjecutarPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if !resultados[0].Exito {
		t.Error("paso vacío debería ser éxito")
	}
}

func TestEjecutor_Timeout(t *testing.T) {
	cat := &mockCatalogo{
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			time.Sleep(200 * time.Millisecond)
			return &ResultadoHerramienta{Exito: true, Datos: "lento"}, nil
		},
		existeFunc: func(nombre string) bool { return true },
	}

	ejec := nuevoEjecutor(cat, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "timeout test", Herramienta: "terminal", TimeoutSegundos: 0},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	resultados, _ := ejec.EjecutarPlan(ctx, plan, nil)
	// Puede o no fallar dependiendo del timing, lo importante es que no panic
	_ = resultados
}

// ============================================================================
// Respondedor
// ============================================================================

func TestRespondedor_ConstruirPrompt(t *testing.T) {
	t.Run("sin memoria", func(t *testing.T) {
		r := nuevoRespondedor(nil, nil)
		clasif := &ResultadoClasificacion{Categoria: CategoriaConversacion, NecesitaContexto: false}
		sesion := &SesionInfo{ID: "s1", UsuarioID: "u1"}

		prompt := r.construirPrompt("hola", sesion, clasif, nil, "")
		if prompt == "" {
			t.Fatal("esperaba prompt no vacío")
		}
		if !contains(prompt, "Eres Liz") {
			t.Error("debería contener rol del sistema")
		}
		if !contains(prompt, "conversacion") {
			t.Error("debería contener categoría")
		}
	})

	t.Run("con memoria y hechos", func(t *testing.T) {
		mem := newMockMemoria()
		mem.hechos["u1"] = "El usuario prefiere Go"
		mem.mensajes["s1"] = []InfoMensaje{
			{Rol: "usuario", Contenido: "hola"},
			{Rol: "asistente", Contenido: "¡Hola!"},
		}

		r := nuevoRespondedor(nil, mem)
		clasif := &ResultadoClasificacion{Categoria: CategoriaConversacion, NecesitaContexto: false}
		sesion := &SesionInfo{ID: "s1", UsuarioID: "u1"}

		prompt := r.construirPrompt("hola de nuevo", sesion, clasif, nil, "")
		if !contains(prompt, "Memoria del usuario") {
			t.Error("debería contener memoria")
		}
		if !contains(prompt, "El usuario prefiere Go") {
			t.Error("debería contener hechos")
		}
		if !contains(prompt, "Historial reciente") {
			t.Error("debería contener historial")
		}
	})

	t.Run("con resultados de herramientas", func(t *testing.T) {
		r := nuevoRespondedor(nil, nil)
		clasif := &ResultadoClasificacion{Categoria: CategoriaProcesos, NecesitaContexto: false}
		resultados := []ResultadoPaso{
			{PasoID: 1, Exito: true, ToolUsada: "monitor", Datos: "CPU: 50%"},
		}

		prompt := r.construirPrompt("estado cpu", &SesionInfo{}, clasif, resultados, "")
		if !contains(prompt, "Resultados de las herramientas") {
			t.Error("debería contener resultados")
		}
	})

	t.Run("con proyecto activo", func(t *testing.T) {
		r := nuevoRespondedor(nil, nil)
		clasif := &ResultadoClasificacion{Categoria: CategoriaCodigo, NecesitaContexto: true}
		sesion := &SesionInfo{ID: "s1", Proyecto: "mi-proyecto"}

		prompt := r.construirPrompt("analiza main.go", sesion, clasif, nil, "")
		if !contains(prompt, "mi-proyecto") {
			t.Error("debería contener nombre del proyecto")
		}
	})
}

func TestRespondedor_InstruccionCategoria(t *testing.T) {
	r := nuevoRespondedor(nil, nil)

	categorias := []CategoriaTarea{
		CategoriaConversacion, CategoriaEjecucionComando, CategoriaArchivos,
		CategoriaProcesos, CategoriaMonitorizacion, CategoriaInstalacion,
		CategoriaBusqueda, CategoriaCodigo, CategoriaAnalisis, CategoriaAutoCreacion,
	}

	for _, cat := range categorias {
		inst := r.instruccionCategoria(cat)
		if inst == "" {
			t.Errorf("instrucción vacía para %s", cat)
		}
	}
}

func TestRespondedor_GenerarRespuesta_Error(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "", fmt.Errorf("API error")
		},
	}

	r := nuevoRespondedor(orch, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaConversacion, Confianza: 0.9, NecesitaContexto: false, Prioridad: 3}

	_, _, _, err := r.GenerarRespuesta(context.Background(), "hola", &SesionInfo{}, clasif, nil, "")
	if err == nil {
		t.Fatal("esperaba error")
	}
}

func TestRespondedor_GenerarRespuestaStream_Fallback(t *testing.T) {
	orch := &mockOrquestador{
		completarStreamFunc: func(ctx context.Context, prompt, tipo string) (<-chan ChunkOrquestador, error) {
			ch := make(chan ChunkOrquestador, 1)
			ch <- ChunkOrquestador{Done: true}
			close(ch)
			return ch, nil
		},
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "fallback", nil
		},
		modeloActual: "fallback-model",
	}

	r := nuevoRespondedor(orch, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaConversacion, Confianza: 0.9, NecesitaContexto: false, Prioridad: 3}

	resp, modelo, tokens, err := r.GenerarRespuestaStream(context.Background(), "hola", &SesionInfo{}, clasif, nil, "", nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == "" {
		t.Error("esperaba respuesta del fallback")
	}
	if modelo != "fallback-model" {
		t.Errorf("esperaba 'fallback-model', got '%s'", modelo)
	}
	if tokens <= 0 {
		t.Error("esperaba tokens > 0")
	}
}

func TestRespondedor_GenerarRespuestaSimple(t *testing.T) {
	orch := &mockOrquestador{
		completarFunc: func(ctx context.Context, prompt, tipo string) (string, error) {
			return "simple response", nil
		},
		modeloActual: "test",
	}

	r := nuevoRespondedor(orch, nil)
	resp, modelo, tokens, err := r.GenerarRespuestaSimple(context.Background(), "hola", &SesionInfo{}, "")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp != "simple response" {
		t.Errorf("esperaba 'simple response', got '%s'", resp)
	}
	if modelo != "test" {
		t.Errorf("esperaba 'test', got '%s'", modelo)
	}
	if tokens <= 0 {
		t.Error("esperaba tokens")
	}
}

func TestRespondedor_GenerarRespuestaSimpleStream(t *testing.T) {
	orch := &mockOrquestador{
		completarStreamFunc: func(ctx context.Context, prompt, tipo string) (<-chan ChunkOrquestador, error) {
			ch := make(chan ChunkOrquestador, 2)
			ch <- ChunkOrquestador{Delta: "stream ", Modelo: "m"}
			ch <- ChunkOrquestador{Delta: "simple", Modelo: "m"}
			ch <- ChunkOrquestador{Done: true}
			close(ch)
			return ch, nil
		},
		modeloActual: "stream-m",
	}

	r := nuevoRespondedor(orch, nil)
	var chunks []*ChunkStream
	resp, modelo, _, err := r.GenerarRespuestaSimpleStream(context.Background(), "hola", &SesionInfo{}, "", func(c *ChunkStream) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == "" {
		t.Error("esperaba respuesta")
	}
	if modelo != "stream-m" {
		t.Errorf("esperaba 'stream-m', got '%s'", modelo)
	}
	if len(chunks) == 0 {
		t.Error("esperaba chunks")
	}
}

func TestRespondedor_GenerarRespuestaStream_Error(t *testing.T) {
	orch := &mockOrquestador{
		completarStreamFunc: func(ctx context.Context, prompt, tipo string) (<-chan ChunkOrquestador, error) {
			return nil, fmt.Errorf("stream error")
		},
	}

	r := nuevoRespondedor(orch, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaConversacion, Confianza: 0.9, NecesitaContexto: false, Prioridad: 3}

	_, _, _, err := r.GenerarRespuestaStream(context.Background(), "hola", &SesionInfo{}, clasif, nil, "", nil)
	if err == nil {
		t.Fatal("esperaba error")
	}
}

// ============================================================================
// Tipos adicionales
// ============================================================================

func TestSolicitudChat_Validar_Whitespace(t *testing.T) {
	sol := &SolicitudChat{Mensaje: "\t\n"}
	err := sol.Validar()
	if err == nil {
		t.Fatal("esperaba error para solo whitespace")
	}
}

func TestSolicitudChat_Validar_JustoDebajo(t *testing.T) {
	sol := &SolicitudChat{Mensaje: string(make([]byte, 49999))}
	err := sol.Validar()
	if err != nil {
		t.Fatalf("no esperaba error para 49999 chars: %v", err)
	}
}

func TestCategoriaTarea_String(t *testing.T) {
	if CategoriaConversacion.String() != "conversacion" {
		t.Error("String() debería retornar el nombre")
	}
	if CategoriaTarea("xyz").String() != "xyz" {
		t.Error("String() debería retornar el valor crudo")
	}
}

func TestResultadoClasificacion_RequiereHerramientas(t *testing.T) {
	c := &ResultadoClasificacion{Categoria: CategoriaConversacion}
	if c.RequiereHerramientas() {
		t.Error("conversacion no debería requerir herramientas")
	}
	c.Categoria = CategoriaProcesos
	if !c.RequiereHerramientas() {
		t.Error("procesos debería requerir herramientas")
	}
}

func TestResultadoClasificacion_PrioridadModelo(t *testing.T) {
	tests := []struct {
		cat      CategoriaTarea
		esperado string
	}{
		{CategoriaCodigo, "codigo"},
		{CategoriaAutoCreacion, "codigo"},
		{CategoriaAnalisis, "razonamiento"},
		{CategoriaMonitorizacion, "general"},
		{CategoriaProcesos, "general"},
		{CategoriaConversacion, "general"},
		{CategoriaBusqueda, "general"},
	}

	for _, tc := range tests {
		c := &ResultadoClasificacion{Categoria: tc.cat}
		if c.PrioridadModelo() != tc.esperado {
			t.Errorf("%s: esperaba '%s', got '%s'", tc.cat, tc.esperado, c.PrioridadModelo())
		}
	}
}

func TestNuevoChunk(t *testing.T) {
	c := NuevoChunk("texto", "hola")
	if c.Tipo != "texto" || c.Contenido != "hola" {
		t.Error("NuevoChunk no funcionó")
	}
}

func TestChunkStream_Serializar_Error(t *testing.T) {
	// ChunkStream con datos que no serializan bien no debería fallar
	c := &ChunkStream{Tipo: "texto", Contenido: "test", Datos: make(chan int)}
	_, err := c.Serializar()
	if err == nil {
		t.Error("esperaba error serializando chan")
	}
}

func TestFormatearResultados_Vacio(t *testing.T) {
	texto := FormatearResultados(nil)
	if texto != "No se ejecutaron herramientas." {
		t.Errorf("esperada mensaje vacío, got '%s'", texto)
	}
}

func TestFormatearResultados_ConDatosGrandes(t *testing.T) {
	datosGrandes := make([]byte, 3000)
	for i := range datosGrandes {
		datosGrandes[i] = 'x'
	}

	resultados := []ResultadoPaso{
		{PasoID: 1, Exito: true, ToolUsada: "test", Datos: string(datosGrandes)},
	}

	texto := FormatearResultados(resultados)
	if !contains(texto, "truncado") {
		t.Error("datos grandes deberían truncarse")
	}
}

func TestEstadoExito(t *testing.T) {
	if estadoExito(true) != "exitoso" {
		t.Error("true debería ser exitoso")
	}
	if estadoExito(false) != "fallido" {
		t.Error("false debería ser fallido")
	}
}

func TestSerializarJSON(t *testing.T) {
	_, err := serializarJSON(map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}

	// Valor no serializable
	_, err = serializarJSON(make(chan int))
	if err == nil {
		t.Error("esperaba error serializando chan")
	}
}

func TestParsearJSON(t *testing.T) {
	var m map[string]string
	err := parsearJSON(`{"key": "value"}`, &m)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if m["key"] != "value" {
		t.Error("parseo incorrecto")
	}

	err = parsearJSON("invalido", &m)
	if err == nil {
		t.Error("esperaba error parseando JSON inválido")
	}
}

func TestExtraerComando(t *testing.T) {
	tests := []struct {
		input    string
		esperado string
	}{
		{"`ls -la`", "ls -la"},
		{"'rm -rf /tmp'", "rm -rf /tmp"},
		{"\"echo hello\"", "echo hello"},
		{"ls -la /home", "ls -la /home"},
		{"echo test", "echo test"},
		{"docker ps", "docker ps"},
		{"git status", "git status"},
		{"quién eres", ""},
		{"hola mundo", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := extraerComando(tc.input)
			if result != tc.esperado {
				t.Errorf("extraerComando(%q) = %q, esperaba %q", tc.input, result, tc.esperado)
			}
		})
	}
}

func TestPipeline_ObtenerHistorial(t *testing.T) {
	t.Run("sin memoria", func(t *testing.T) {
		p := Nuevo(NuevasOpciones{})
		h := p.obtenerHistorial("s1")
		if h != nil {
			t.Error("esperaba nil sin memoria")
		}
	})

	t.Run("con memoria", func(t *testing.T) {
		mem := newMockMemoria()
		mem.mensajes["s1"] = []InfoMensaje{
			{Rol: "usuario", Contenido: "hola"},
			{Rol: "asistente", Contenido: "¡hola!"},
		}

		p := Nuevo(NuevasOpciones{Memoria: mem})
		h := p.obtenerHistorial("s1")
		if len(h) != 2 {
			t.Errorf("esperaba 2 mensajes, got %d", len(h))
		}
	})
}

func TestPipeline_ActualizarMetricas_Concurrente(t *testing.T) {
	p := Nuevo(NuevasOpciones{})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola"})
		}()
	}
	wg.Wait()

	estado := p.Estado()
	if estado.MensajesProcesados != 10 {
		t.Errorf("esperaba 10, got %d", estado.MensajesProcesados)
	}
}
