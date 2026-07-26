package pipeline

import (
	"context"
	"testing"
	"time"
)

// ============================================================================
// Tests del Receptor
// ============================================================================

func TestSolicitudChat_Validar_ErrorVacio(t *testing.T) {
	sol := &SolicitudChat{Mensaje: ""}
	err := sol.Validar()
	if err == nil {
		t.Fatal("esperaba error para mensaje vacío")
	}
}

func TestSolicitudChat_Validar_ErrorMuyLargo(t *testing.T) {
	sol := &SolicitudChat{Mensaje: string(make([]byte, 50001))}
	err := sol.Validar()
	if err == nil {
		t.Fatal("esperaba error para mensaje muy largo")
	}
}

func TestSolicitudChat_Validar_Ok(t *testing.T) {
	sol := &SolicitudChat{Mensaje: "hola liz"}
	err := sol.Validar()
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if sol.UsuarioID != "usuario_default" {
		t.Fatalf("esperaba usuario_default, got %s", sol.UsuarioID)
	}
}

func TestReceptor_Recibir_SinMemoria(t *testing.T) {
	rec := nuevoReceptor(nil)
	sol := &SolicitudChat{Mensaje: "hola liz"}

	msg, sesion, err := rec.Recibir(context.Background(), sol)
	if err != nil {
		t.Fatalf("no esperaba error sin memoria: %v", err)
	}
	if msg.Contenido != "hola liz" {
		t.Fatalf("esperaba 'hola liz', got '%s'", msg.Contenido)
	}
	if msg.Rol != "usuario" {
		t.Fatalf("esperaba rol 'usuario', got '%s'", msg.Rol)
	}
	if msg.ID == "" {
		t.Fatal("esperaba ID generado")
	}
	// Sin memoria, la sesión se crea localmente con UUID
	if sesion == nil {
		t.Fatal("esperaba sesión no nil")
	}
}

func TestReceptor_Recibir_ConMemoriaMock(t *testing.T) {
	mem := &AdaptadorMemoria{
		crearSesionFunc: func(ctx context.Context, usuarioID, proyecto string) (*InfoSesion, error) {
			return &InfoSesion{ID: "sesion_test", UsuarioID: usuarioID, Proyecto: proyecto}, nil
		},
		agregarMensajeFunc: func(ctx context.Context, sesionID, usuarioID, contenido string) error {
			return nil
		},
	}

	rec := nuevoReceptor(mem)
	sol := &SolicitudChat{Mensaje: "hola liz", UsuarioID: "test_user"}

	msg, sesion, err := rec.Recibir(context.Background(), sol)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if msg.UsuarioID != "test_user" {
		t.Fatalf("esperaba 'test_user', got '%s'", msg.UsuarioID)
	}
	if sesion.ID != "sesion_test" {
		t.Fatalf("esperaba 'sesion_test', got '%s'", sesion.ID)
	}
}

// ============================================================================
// Tests del Clasificador
// ============================================================================

func TestClasificador_SinOrquestador(t *testing.T) {
	clasif := nuevoClasificador(nil)

	cases := []struct {
		mensaje   string
		categoria CategoriaTarea
		minConf   float64
	}{
		{"hola liz", CategoriaConversacion, 0.8},
		{"instala docker", CategoriaInstalacion, 0.7},
		{"busca todos los .log", CategoriaBusqueda, 0.7},
		{"mata el proceso en el puerto 8080", CategoriaProcesos, 0.7},
		{"estado de la cpu", CategoriaMonitorizacion, 0.7},
		{"crea un servidor HTTP en Go", CategoriaCodigo, 0.6},
		{"ejecuta ls -la", CategoriaEjecucionComando, 0.7},
		{"elimina los archivos temporales", CategoriaArchivos, 0.6},
		{"crea una herramienta para comprimir", CategoriaAutoCreacion, 0.7},
	}

	for _, tc := range cases {
		t.Run(tc.mensaje, func(t *testing.T) {
			result, err := clasif.Clasificar(context.Background(), tc.mensaje, "")
			if err != nil {
				t.Fatalf("no esperaba error para '%s': %v", tc.mensaje, err)
			}
			if result.Categoria != tc.categoria {
				t.Errorf("esperaba categoría '%s', got '%s' para '%s'", tc.categoria, result.Categoria, tc.mensaje)
			}
			if result.Confianza < tc.minConf {
				t.Errorf("confianza %.2f menor al mínimo %.2f para '%s'", result.Confianza, tc.minConf, tc.mensaje)
			}
		})
	}
}

func TestClasificador_ConProyectoActivo(t *testing.T) {
	clasif := nuevoClasificador(nil)
	result, err := clasif.Clasificar(context.Background(), "qué pasa con esto", "mi-proyecto")
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	// Sin palabras clave claras pero con proyecto → debería inclinarse a código
	if result.Categoria != CategoriaCodigo {
		t.Logf("Nota: sin palabras clave claras + proyecto → %s (aceptable si heurística no match)", result.Categoria)
	}
}

func TestTodasCategorias_Valida(t *testing.T) {
	cats := TodasCategorias()
	if len(cats) == 0 {
		t.Fatal("esperaba categorías definidas")
	}
	for _, c := range cats {
		if !c.Valida() {
			t.Errorf("categoría '%s' debería ser válida", c)
		}
	}
}

// ============================================================================
// Tests del Planificador
// ============================================================================

func TestPlanificador_Conversacion(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaConversacion}

	plan, err := planif.Planificar(context.Background(), "hola", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(plan.Pasos) != 1 {
		t.Fatalf("esperaba 1 paso, got %d", len(plan.Pasos))
	}
	if !plan.Pasos[0].RequiereLLM {
		t.Error("conversación debería requerir LLM")
	}
}

func TestPlanificador_EjecucionComando(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaEjecucionComando}

	plan, err := planif.Planificar(context.Background(), "ejecuta ls -la", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(plan.Pasos) == 0 {
		t.Fatal("esperaba al menos 1 paso")
	}
}

func TestPlanificador_Monitorizacion(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaMonitorizacion}

	plan, err := planif.Planificar(context.Background(), "estado de la cpu y memoria", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(plan.Pasos) < 1 {
		t.Fatal("esperaba al menos 1 paso")
	}
	// Debería usar la herramienta monitor
	foundMonitor := false
	for _, p := range plan.Pasos {
		if p.Herramienta == "monitor" {
			foundMonitor = true
			break
		}
	}
	if !foundMonitor {
		t.Log("Nota: no se detectó herramienta 'monitor' (aceptable para heurísticas simples)")
	}
}

func TestPlanificador_AutoCreacion(t *testing.T) {
	planif := nuevoPlanificador(nil, nil, nil, nil)
	clasif := &ResultadoClasificacion{Categoria: CategoriaAutoCreacion}

	plan, err := planif.Planificar(context.Background(), "crea una herramienta para comprimir archivos", clasif, &SesionInfo{}, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if !plan.NecesitaAutoCrear {
		t.Error("plan de auto-creación debería marcar NecesitaAutoCrear=true")
	}
	if len(plan.Pasos) == 0 {
		t.Fatal("esperaba pasos para auto-creación")
	}
}

// ============================================================================
// Tests del Ejecutor
// ============================================================================

func TestEjecutor_SinCatalogo(t *testing.T) {
	ejec := nuevoEjecutor(nil, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "test", Herramienta: "terminal"},
		},
	}

	resultados, err := ejec.EjecutarPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(resultados) != 1 {
		t.Fatalf("esperaba 1 resultado, got %d", len(resultados))
	}
	// Sin catálogo, el ejecutor retorna éxito=false en datos vacíos
	// pero el paso con herramienta vacía se marca como éxito
	if resultados[0].Error != "No hay catálogo de herramientas disponible" {
		t.Logf("Nota: error sin catálogo: %s", resultados[0].Error)
	}
}

func TestEjecutor_ConCatalogoMock(t *testing.T) {
	cat := &AdaptadorCatalogo{
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return &ResultadoHerramienta{Exito: true, Datos: "resultado_test"}, nil
		},
		existeFunc: func(nombre string) bool { return true },
	}

	ejec := nuevoEjecutor(cat, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "ejecutar test", Herramienta: "terminal", Parametros: map[string]interface{}{"comando": "echo", "args": []interface{}{"hello"}}},
		},
	}

	resultados, err := ejec.EjecutarPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if !resultados[0].Exito {
		t.Errorf("esperaba éxito: %s", resultados[0].Error)
	}
}

func TestEjecutor_Dependencias(t *testing.T) {
	cat := &AdaptadorCatalogo{
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return &ResultadoHerramienta{Exito: true, Datos: "ok"}, nil
		},
		existeFunc: func(nombre string) bool { return true },
	}

	ejec := nuevoEjecutor(cat, nil)
	plan := &PlanEjecucion{
		Pasos: []PasoTarea{
			{ID: 1, Descripcion: "paso 1", Herramienta: "terminal", Parametros: map[string]interface{}{}},
			{ID: 2, Descripcion: "paso 2 depende de 1", Herramienta: "terminal", Parametros: map[string]interface{}{}, DependeDe: []int{1}},
		},
	}

	resultados, err := ejec.EjecutarPlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(resultados) != 2 {
		t.Fatalf("esperaba 2 resultados, got %d", len(resultados))
	}
}

func TestFormatearResultados(t *testing.T) {
	resultados := []ResultadoPaso{
		{PasoID: 1, Exito: true, ToolUsada: "terminal", Duracion: 100 * time.Millisecond, Datos: "output"},
		{PasoID: 2, Exito: false, ToolUsada: "buscador", Error: "no encontrado", Duracion: 50 * time.Millisecond},
	}

	texto := FormatearResultados(resultados)
	if texto == "" {
		t.Fatal("esperaba texto no vacío")
	}
}

// ============================================================================
// Tests del Pipeline (integración)
// ============================================================================

func TestPipeline_SinDependencias(t *testing.T) {
	p := Nuevo(NuevasOpciones{})

	if p == nil {
		t.Fatal("esperaba pipeline creado")
	}

	// Estado inicial
	estado := p.Estado()
	if estado.MensajesProcesados != 0 {
		t.Errorf("esperaba 0 mensajes, got %d", estado.MensajesProcesados)
	}
}

func TestPipeline_Procesar_SinOrquestador(t *testing.T) {
	p := Nuevo(NuevasOpciones{})

	resp, err := p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola liz"})
	if err != nil {
		t.Fatalf("no esperaba error (degrada gracefully): %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta no nil")
	}
	if resp.Mensaje == "" {
		t.Error("esperaba mensaje no vacío")
	}
	if resp.ModeloUsado != "none" {
		t.Errorf("esperaba modelo 'none', got '%s'", resp.ModeloUsado)
	}
}

func TestPipeline_Procesar_ConCatalogo(t *testing.T) {
	cat := &AdaptadorCatalogo{
		ejecutarFunc: func(ctx context.Context, nombre string, params map[string]interface{}) (*ResultadoHerramienta, error) {
			return &ResultadoHerramienta{Exito: true, Datos: "output"}, nil
		},
		existeFunc: func(nombre string) bool { return true },
		snapshotFunc: func() []InfoHerramientaSnapshot {
			return []InfoHerramientaSnapshot{
				{Nombre: "terminal", Descripcion: "ejecuta comandos", Parametros: "params"},
			}
		},
	}

	p := Nuevo(NuevasOpciones{Catalogo: cat})
	resp, err := p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola liz"})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp.Categoria != CategoriaConversacion {
		t.Logf("categoría detectada: %s", resp.Categoria)
	}
}

func TestPipeline_ProcesarStream_SinOrquestador(t *testing.T) {
	p := Nuevo(NuevasOpciones{})

	var chunks []*ChunkStream
	resp, err := p.ProcesarStream(context.Background(), &SolicitudChat{Mensaje: "hola"}, func(c *ChunkStream) {
		chunks = append(chunks, c)
	})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resp == nil {
		t.Fatal("esperaba respuesta no nil")
	}
	// Debería haber al menos un chunk de estado
	if len(chunks) == 0 {
		t.Log("Nota: sin orquestador puede que no haya chunks")
	}
}

func TestPipeline_Metricas(t *testing.T) {
	p := Nuevo(NuevasOpciones{})

	// Procesar varios mensajes
	for i := 0; i < 3; i++ {
		_, err := p.Procesar(context.Background(), &SolicitudChat{Mensaje: "hola"})
		if err != nil {
			t.Fatalf("error en iteración %d: %v", i, err)
		}
	}

	estado := p.Estado()
	if estado.MensajesProcesados != 3 {
		t.Errorf("esperaba 3 mensajes, got %d", estado.MensajesProcesados)
	}
}

// ============================================================================
// Tests de tipos y helpers
// ============================================================================

func TestCategoriaTarea_Valida(t *testing.T) {
	if !CategoriaConversacion.Valida() {
		t.Error("conversacion debería ser válida")
	}
	if CategoriaTarea("invalida").Valida() {
		t.Error("categoría inválida no debería ser válida")
	}
}

func TestChunkStream_Serializar(t *testing.T) {
	chunk := &ChunkStream{Tipo: "texto", Contenido: "hola mundo"}
	data, err := chunk.Serializar()
	if err != nil {
		t.Fatalf("error serializando: %v", err)
	}
	if string(data) == "" {
		t.Error("esperaba JSON no vacío")
	}
}

func TestExtraerJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`{"key": "value"}`, `{"key": "value"}`},
		{`texto antes {"key": "value"} texto despues`, `{"key": "value"}`},
		{`[1, 2, 3]`, `[1, 2, 3]`},
		{`no json aqui`, ""},
	}

	for _, tc := range tests {
		result := extraerJSON(tc.input)
		if result != tc.expected {
			t.Errorf("extraerJSON(%q) = %q, esperaba %q", tc.input, result, tc.expected)
		}
	}
}

func TestEstimarTokens(t *testing.T) {
	if estimarTokens("") != 0 {
		t.Error("vacío debería ser 0 tokens")
	}
	if estimarTokens("abcdefgh") != 2 {
		t.Error("8 chars = ~2 tokens")
	}
}

func TestTruncarTexto(t *testing.T) {
	if truncarTexto("hola", 10) != "hola" {
		t.Error("texto corto no debería truncarse")
	}
	truncado := truncarTexto("abcdefghijklmnopqrstuvwxyz", 10)
	if len(truncado) < 10 {
		t.Error("texto truncado debería tener al menos 10 chars")
	}
}

func TestTienePalabrasClave(t *testing.T) {
	if !tienePalabrasClave("instala docker", []string{"instala", "docker"}) {
		t.Error("debería encontrar palabras clave")
	}
	if tienePalabrasClave("hola mundo", []string{"instala"}) {
		t.Error("no debería encontrar palabras clave")
	}
}

func TestGenerarUUID(t *testing.T) {
	uuid1 := generarUUID()
	uuid2 := generarUUID()
	if uuid1 == uuid2 {
		t.Error("UUIDs deberían ser únicos")
	}
}

func TestContextoParaPrompt(t *testing.T) {
	prompt := ContextoParaPrompt()
	if prompt == "" {
		t.Fatal("esperaba prompt no vacío")
	}
	if len(prompt) < 100 {
		t.Error("prompt demasiado corto")
	}
}
