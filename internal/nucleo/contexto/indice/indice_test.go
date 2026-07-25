package indice

import (
	"os"
	"path/filepath"
	"testing"
)

func crearProyectoIndice(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Crear estructura de proyecto
	dirs := []string{"pkg/a", "pkg/b"}
	for _, dir := range dirs {
		os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
	}

	// Crear archivos Go
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "pkg/a", "a.go"), []byte("package a\n\nfunc A() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "pkg/b", "b.go"), []byte("package b\n\nfunc B() {}\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test\n"), 0644)

	return tmpDir
}

func TestReconstruir_Basico(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, err := NuevoGestor(rutaIndice)
	if err != nil {
		t.Fatalf("NuevoGestor() error: %v", err)
	}

	err = g.Reconstruir(proyectoDir)
	if err != nil {
		t.Fatalf("Reconstruir() error: %v", err)
	}

	ind := g.Obtener()

	// Debería tener al menos 5 archivos (sin .git ni ignorados)
	if ind.TotalArchivos < 4 {
		t.Errorf("se esperaban al menos 4 archivos, obtuve %d", ind.TotalArchivos)
	}

	if ind.Version != "1.0" {
		t.Errorf("versión incorrecta: %s", ind.Version)
	}

	if ind.Proyecto != filepath.Base(proyectoDir) {
		t.Errorf("nombre de proyecto incorrecto: %s", ind.Proyecto)
	}
}

func TestReconstruir_Incremental(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, _ := NuevoGestor(rutaIndice)
	g.Reconstruir(proyectoDir)

	primerTotal := g.Obtener().TotalArchivos

	// Reconstruir de nuevo sin cambios — no debería cambiar
	g.Reconstruir(proyectoDir)
	segundoTotal := g.Obtener().TotalArchivos

	if primerTotal != segundoTotal {
		t.Errorf("reconstrucción sin cambios no debería alterar el total: %d vs %d",
			primerTotal, segundoTotal)
	}

	// Agregar un archivo nuevo y reconstruir
	os.WriteFile(filepath.Join(proyectoDir, "nuevo.go"), []byte("package nuevo\n"), 0644)
	g.Reconstruir(proyectoDir)

	tercerTotal := g.Obtener().TotalArchivos
	if tercerTotal != primerTotal+1 {
		t.Errorf("después de agregar archivo, total debería ser %d, obtuve %d",
			primerTotal+1, tercerTotal)
	}
}

func TestBuscar(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, _ := NuevoGestor(rutaIndice)
	g.Reconstruir(proyectoDir)

	// Buscar por extensión
	resultados := g.Buscar(".go")
	if len(resultados) == 0 {
		t.Error("debería encontrar archivos .go")
	}

	// Buscar por nombre de archivo
	resultados = g.Buscar("main")
	if len(resultados) == 0 {
		t.Error("debería encontrar 'main.go'")
	}

	// Buscar algo que no existe
	resultados = g.Buscar("xyz_no_existe_123")
	if len(resultados) != 0 {
		t.Errorf("búsqueda sin resultados debería retornar vacío, obtuve %d", len(resultados))
	}
}

func TestObtenerArchivo(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, _ := NuevoGestor(rutaIndice)
	g.Reconstruir(proyectoDir)

	entrada, err := g.ObtenerArchivo("main.go")
	if err != nil {
		t.Fatalf("ObtenerArchivo() error: %v", err)
	}

	if entrada.Lenguaje != "go" {
		t.Errorf("lenguaje esperado 'go', obtuve '%s'", entrada.Lenguaje)
	}

	if entrada.Lineas < 1 {
		t.Error("main.go debería tener al menos 1 línea")
	}
}

func TestObtenerArchivo_NoExistente(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, _ := NuevoGestor(rutaIndice)
	g.Reconstruir(proyectoDir)

	_, err := g.ObtenerArchivo("no_existe.go")
	if err == nil {
		t.Error("debería retornar error para archivo inexistente")
	}
}

func TestObtenerPorLenguaje(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, _ := NuevoGestor(rutaIndice)
	g.Reconstruir(proyectoDir)

	goFiles := g.ObtenerPorLenguaje("go")
	if len(goFiles) == 0 {
		t.Error("debería encontrar archivos Go")
	}

	// Todos deberían ser Go
	for _, f := range goFiles {
		if f.Lenguaje != "go" {
			t.Errorf("archivo %s no es Go: %s", f.Ruta, f.Lenguaje)
		}
	}
}

func TestAsignarFragmentos(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, _ := NuevoGestor(rutaIndice)
	g.Reconstruir(proyectoDir)

	err := g.AsignarFragmentos("main.go", []string{"frag_001", "frag_002"})
	if err != nil {
		t.Fatalf("AsignarFragmentos() error: %v", err)
	}

	entrada, _ := g.ObtenerArchivo("main.go")
	if len(entrada.FragmentoIDs) != 2 {
		t.Errorf("se esperaban 2 fragmentos, obtuve %d", len(entrada.FragmentoIDs))
	}

	ind := g.Obtener()
	if ind.TotalFragmentos != 2 {
		t.Errorf("total de fragmentos debería ser 2, obtuve %d", ind.TotalFragmentos)
	}
}

func TestCargarGestor_Existente(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	// Crear y guardar un índice
	g1, _ := NuevoGestor(rutaIndice)
	g1.Reconstruir(proyectoDir)

	// Cargar el mismo índice
	g2, err := NuevoGestor(rutaIndice)
	if err != nil {
		t.Fatalf("error cargando índice existente: %v", err)
	}

	ind := g2.Obtener()
	if ind.TotalArchivos == 0 {
		t.Error("el índice cargado debería tener archivos")
	}
}

func TestLenguajesContados(t *testing.T) {
	proyectoDir := crearProyectoIndice(t)
	rutaIndice := filepath.Join(t.TempDir(), "indice.json")

	g, _ := NuevoGestor(rutaIndice)
	g.Reconstruir(proyectoDir)

	ind := g.Obtener()
	if len(ind.Lenguajes) == 0 {
		t.Error("debería contar lenguajes")
	}

	// Debería tener 'go' como lenguaje
	if count, ok := ind.Lenguajes["go"]; !ok || count == 0 {
		t.Error("debería contar al menos 1 archivo Go")
	}
}

func TestDetectarLenguajeIndice(t *testing.T) {
	tests := []struct{
		ext      string
		esperado string
	}{
		{".go", "go"},
		{".py", "python"},
		{".ts", "typescript"},
		{".toml", "toml"},
		{".yml", "yaml"},
		{".json", "json"},
		{".md", "markdown"},
		{".txt", "txt"},
		{"", ""},
	}

	for _, tt := range tests {
		resultado := detectarLenguajeIndice(tt.ext)
		if resultado != tt.esperado {
			t.Errorf("detectarLenguajeIndice(%q) = %q, esperado %q",
				tt.ext, resultado, tt.esperado)
		}
	}
}