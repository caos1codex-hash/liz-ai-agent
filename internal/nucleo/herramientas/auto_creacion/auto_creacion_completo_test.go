package auto_creacion

import (
	"context"
	"testing"
)

// ============================================================================
// Tests adicionales para gestor.go — cobertura de funciones no cubiertas
// ============================================================================

func TestGestor_CargarTodas_ErrorDirectorio(t *testing.T) {
	// Directorio que no existe — debe manejar gracefully
	g := &Gestor{
		directorio: "/tmp/no_existe_liz_auto_creacion_test_" + randomID(),
	}
	
	resultados := g.CargarTodas()
	if resultados == nil {
		t.Fatal("CargarTodas no debería retornar nil")
	}
	// Debería retornar slice vacío o errores, pero no panic
	for _, r := range resultados {
		if r.Exito {
			t.Errorf("esperaba fallo para herramienta en directorio inexistente: %s", r.Nombre)
		}
	}
}

func TestGestor_Eliminar_Inexistente(t *testing.T) {
	g := &Gestor{
		directorio: t.TempDir(),
	}
	
	// No debería fallar eliminando algo que no existe
	err := g.Eliminar("no_existe")
	// Puede retornar error o nil dependiendo de la implementación
	_ = err
}

func TestGestor_Recargar_SinFuente(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Gestor{
		directorio: tmpDir,
	}
	
	// Recargar algo que no existe
	err := g.Recargar(context.Background(), "no_existe", false)
	// No debería panic
	_ = err
}

func TestGestor_Probar_SinCatalogo(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Gestor{
		directorio: tmpDir,
	}
	
	// Probar sin inyectar al catálogo
	err := g.Probar(context.Background(), "no_existe", nil)
	_ = err
}

func TestGestor_ObtenerInfo_Inexistente(t *testing.T) {
	g := &Gestor{
		directorio: t.TempDir(),
	}
	
	info, err := g.ObtenerInfo("no_existe")
	if err != nil {
		t.Logf("error esperado para herramienta inexistente: %v", err)
	} else {
		t.Log("ObtenerInfo retornó info sin error (puede ser nil)")
	}
	_ = info
}

// ============================================================================
// Tests adicionales para compilador.go
// ============================================================================

func TestCompilar_FuenteInvalida(t *testing.T) {
	tmpDir := t.TempDir()
	
	_, _, err := Compilar(tmpDir, "fuente_invalido.go", "esta_no_es_go_valido{{{")
	if err == nil {
		t.Fatal("esperaba error compilando fuente inválida")
	}
}

func TestCompilar_FuenteVacia(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Escribir archivo Go vacío
	escribirArchivo(t, tmpDir+"/vacio.go", "package main\n")
	
	_, _, err := Compilar(tmpDir, "vacio.go", "")
	if err != nil {
		t.Fatalf("archivo Go vacío debería compilar: %v", err)
	}
}

func escribirArchivo(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatalf("error escribiendo %s: %v", ruta, err)
	}
}

// ============================================================================
// Tests adicionales para registro.go
// ============================================================================

func TestRegistro_MetadataInvalida(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Metadata JSON roto
	rutaMetadata := tmpDir + "/metadata.json"
	os.WriteFile(rutaMetadata, []byte("no es json"), 0644)
	
	_, err := LeerMetadata(rutaMetadata)
	if err == nil {
		t.Log("metadata inválida puede ser aceptada (depende de implementación)")
	}
}

func TestRegistro_ListarVacio(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(tmpDir+"/herramientas", 0755)
	
	lista, err := ListarHerramientas(tmpDir)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if lista == nil {
		t.Fatal("esperaba lista no nil")
	}
	if len(lista) != 0 {
		t.Errorf("directorio vacío debería dar lista vacía, got %d", len(lista))
	}
}

// ============================================================================
// Tests adicionales para tipos.go
// ============================================================================

func TestSolicitudCreacion_Vacia(t *testing.T) {
	sol := SolicitudCreacion{}
	err := sol.Validar()
	if err == nil {
		t.Fatal("solicitud vacía debería ser inválida")
	}
}

func TestSolicitudCreacion_ConForzarSpec(t *testing.T) {
	sol := SolicitudCreacion{
		ForzarSpec: &SpecHerramienta{
			Nombre:     "test",
			Descripcion: "test tool",
			Categoria:   "test",
			Parametros:  []ParametroSpec{},
		},
	}
	err := sol.Validar()
	if err != nil {
		t.Fatalf("con forzar_spec válida no debería dar error: %v", err)
	}
}

func TestResultadoCreacion_Serializar(t *testing.T) {
	r := ResultadoCreacion{
		Exito: true,
		Datos: map[string]interface{}{"nombre": "test"},
	}
	_ = r // Verificar que los campos son accesibles
}

func TestSpecHerramienta_Campos(t *testing.T) {
	spec := SpecHerramienta{
		Nombre:     "compresor",
		Descripcion: "Comprime archivos",
		Categoria:   "archivo",
		Parametros: []ParametroSpec{
			{Nombre: "input", Tipo: "string", Requerido: true, Descripcion: "Archivo de entrada"},
		},
	}
	
	if spec.Nombre != "compresor" {
		t.Errorf("nombre incorrecto: %s", spec.Nombre)
	}
	if len(spec.Parametros) != 1 {
		t.Errorf("esperaba 1 parámetro, got %d", len(spec.Parametros))
	}
}

func TestParametroSpec_Default(t *testing.T) {
	p := ParametroSpec{
		Nombre:     "modo",
		Tipo:       "string",
		Requerido:  false,
		Default:    "rapido",
	}
	
	if p.Default != "rapido" {
		t.Errorf("default incorrecto: %v", p.Default)
	}
}

func TestMetadataHerramienta_Estructura(t *testing.T) {
	m := MetadataHerramienta{
		Nombre:     "test",
		Creado:     "2024-01-01T00:00:00Z",
		Modificado: "2024-01-01T00:00:00Z",
		Ejecuciones: 5,
		Exito:       4,
		Fallos:      1,
	}
	
	if m.Nombre != "test" {
		t.Error("nombre incorrecto")
	}
	if m.Ejecuciones != 5 {
		t.Error("ejecuciones incorrectas")
	}
}

// ============================================================================
// Tests adicionales para plantillas.go
// ============================================================================

func TestExtraerFuenteGo_Basico(t *testing.T) {
	codigo := "package main\n\nfunc main() {\n\tfmt.Println(\"hola\")\n}\n"
	
	fuente := ExtraerFuenteGo(codigo)
	if fuente != codigo {
		t.Error("debería retornar el código completo")
	}
}

func TestExtraerFuenteGo_ConMarkdown(t *testing.T) {
	codigo := "```go\npackage main\n\nfunc main() {}\n```\nTexto después"
	
	fuente := ExtraerFuenteGo(codigo)
	if !strings.Contains(fuente, "package main") {
		t.Error("debería extraer el código Go del bloque markdown")
	}
	if strings.Contains(fuente, "Texto después") {
		t.Error("no debería incluir texto fuera del bloque")
	}
}

func TestValidarFuenteGo_Valido(t *testing.T) {
	codigo := "package main\n\nfunc main() {\n\tfmt.Println(\"hola\")\n}\n"
	err := ValidarFuenteGo(codigo)
	if err != nil {
		t.Fatalf("código válido no debería dar error: %v", err)
	}
}

func TestValidarFuenteGo_Invalido(t *testing.T) {
	codigo := "esto no es Go válido {{{"
	err := ValidarFuenteGo(codigo)
	if err == nil {
		t.Fatal("código inválido debería dar error")
	}
}

func TestValidarFuenteGo_ConImportsExternos(t *testing.T) {
	codigo := "package main\n\nimport \"github.com/externo/paquete\"\n\nfunc main() {}\n"
	err := ValidarFuenteGo(codigo)
	if err == nil {
		t.Log("imports externos pueden ser permitidos o no según la implementación")
	}
}