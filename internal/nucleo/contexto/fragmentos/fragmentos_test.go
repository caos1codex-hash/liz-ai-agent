package fragmentos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func crearAlmacenTest(t *testing.T) (*Almacen, string) {
	t.Helper()
	tmpDir := t.TempDir()
	
	almacen, err := NuevoAlmacen(tmpDir, "proyecto_test")
	if err != nil {
		t.Fatalf("error creando almacén: %v", err)
	}
	
	return almacen, tmpDir
}

func TestAgregar_Basico(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	contenido := `package main

func main() {}
`

	id, err := a.Agregar("main.go", contenido, "completo", "go", 1, 3)
	if err != nil {
		t.Fatalf("Agregar() error: %v", err)
	}

	if id == "" {
		t.Error("ID no debería estar vacío")
	}

	// Verificar que se persistió
	frag, err := a.Obtener(id)
	if err != nil {
		t.Fatalf("Obtener() error: %v", err)
	}

	if frag.Ruta != "main.go" {
		t.Errorf("ruta esperada 'main.go', obtuve '%s'", frag.Ruta)
	}
	if frag.Tipo != "completo" {
		t.Errorf("tipo esperado 'completo', obtuve '%s'", frag.Tipo)
	}
	if frag.Contenido != contenido {
		t.Error("contenido no coincide")
	}
	if frag.LineaIni != 1 || frag.LineaFin != 3 {
		t.Errorf("líneas incorrectas: %d-%d", frag.LineaIni, frag.LineaFin)
	}
}

func TestAgregar_NoDuplica(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	contenido := "mismo contenido"

	id1, _ := a.Agregar("file.go", contenido, "completo", "go", 1, 1)
	id2, _ := a.Agregar("file.go", contenido, "completo", "go", 1, 1)

	if id1 != id2 {
		t.Error("mismo contenido debería generar mismo ID")
	}

	if a.Total() != 1 {
		t.Errorf("debería haber solo 1 fragmento, hay %d", a.Total())
	}
}

func TestObtener_NoExistente(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	_, err := a.Obtener("no_existe_id")
	if err == nil {
		t.Error("debería retornar error para ID inexistente")
	}
}

func TestObtenerPorRuta(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	a.Agregar("a.go", "contenido a", "completo", "go", 1, 1)
	a.Agregar("b.go", "contenido b", "completo", "go", 1, 1)
	a.Agregar("a.go", "contenido a2", "funcion", "go", 3, 5)

	frags, err := a.ObtenerPorRuta("a.go")
	if err != nil {
		t.Fatalf("ObtenerPorRuta() error: %v", err)
	}

	if len(frags) != 2 {
		t.Errorf("se esperaban 2 fragmentos para 'a.go', obtuve %d", len(frags))
	}

	// Verificar orden por línea de inicio
	if frags[0].LineaIni > frags[1].LineaIni {
		t.Error("fragmentos deberían estar ordenados por línea de inicio")
	}
}

func TestAgregarArchivoCompleto(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	contenido := "línea1\nlínea2\nlínea3\n"

	id, err := a.AgregarArchivoCompleto("test.go", contenido, "go")
	if err != nil {
		t.Fatalf("AgregarArchivoCompleto() error: %v", err)
	}

	frag, _ := a.Obtener(id)
	if frag.Tipo != "completo" {
		t.Errorf("tipo debería ser 'completo', obtuve '%s'", frag.Tipo)
	}
	if frag.LineaIni != 1 || frag.LineaFin != 3 {
		t.Errorf("líneas incorrectas: %d-%d", frag.LineaIni, frag.LineaFin)
	}
}

func TestAgregarDesdeArchivo(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	// Crear archivo temporal
	ruta := filepath.Join(t.TempDir(), "test.go")
	contenido := `package test

func Hola() string {
	return "hola"
}
`
	os.WriteFile(ruta, []byte(contenido), 0644)

	ids, err := a.AgregarDesdeArchivo("test.go", ruta)
	if err != nil {
		t.Fatalf("AgregarDesdeArchivo() error: %v", err)
	}

	if len(ids) == 0 {
		t.Error("debería haber generado al menos un fragmento")
	}
}

func TestListar(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	a.Agregar("a.go", "contenido a", "completo", "go", 1, 5)
	a.Agregar("b.py", "contenido b", "completo", "python", 1, 3)

	lista, err := a.Listar()
	if err != nil {
		t.Fatalf("Listar() error: %v", err)
	}

	if len(lista) != 2 {
		t.Errorf("se esperaban 2 fragmentos, obtuve %d", len(lista))
	}

	// La lista no debería incluir el contenido completo
	for _, frag := range lista {
		if frag.Contenido != "" {
			t.Error("la lista no debería incluir contenido completo")
		}
	}
}

func TestTotal(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	if a.Total() != 0 {
		t.Errorf("almacén vacío debería tener 0 fragmentos, tiene %d", a.Total())
	}

	a.Agregar("a.go", "cont1", "completo", "go", 1, 1)
	a.Agregar("b.go", "cont2", "completo", "go", 1, 1)
	a.Agregar("c.go", "cont3", "completo", "go", 1, 1)

	if a.Total() != 3 {
		t.Errorf("se esperaban 3 fragmentos, tiene %d", a.Total())
	}
}

func TestEliminarPorRuta(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	a.Agregar("a.go", "cont a", "completo", "go", 1, 1)
	a.Agregar("b.go", "cont b", "completo", "go", 1, 1)
	a.Agregar("a.go", "cont a2", "funcion", "go", 2, 5)

	eliminados, err := a.EliminarPorRuta("a.go")
	if err != nil {
		t.Fatalf("EliminarPorRuta() error: %v", err)
	}

	if len(eliminados) != 2 {
		t.Errorf("se esperaban 2 eliminados, obtuve %d", len(eliminados))
	}

	if a.Total() != 1 {
		t.Errorf("debería quedar 1 fragmento, hay %d", a.Total())
	}
}

func TestDirectorio(t *testing.T) {
	a, tmpDir := crearAlmacenTest(t)

	esperado := filepath.Join(tmpDir, "proyecto_test", ".liz", "archivos")
	if a.Directorio() != esperado {
		t.Errorf("directorio incorrecto: %s", a.Directorio())
	}
}

func TestGenerarResumen(t *testing.T) {
	tests := []struct{
		contenido string
		tipo     string
	}{
		{"package main\n\nfunc HolaMundo() {}", "funcion"},
		{"# Comentario\nvar x = 1", "var"},
		{"solo texto plano", "completo"},
	}

	for _, tt := range tests {
		resultado := generarResumen(tt.contenido, tt.tipo)
		if resultado == "" {
			t.Errorf("generarResumen(%q, %q) no debería estar vacío", tt.contenido, tt.tipo)
		}
	}
}

func TestDetectarLenguajeExt(t *testing.T) {
	tests := []struct{
		ext      string
		esperado string
	}{
	{".go", "go"},
	{".py", "python"},
	{".ts", "typescript"},
	{".yaml", "yaml"},
	{".json", "json"},
	{".md", "markdown"},
	{".html", "html"},
	{".rs", "rust"},
	{".toml", "toml"},
	{".sh", "shell"},
	{".unknown", ""},
	}

	for _, tt := range tests {
		resultado := detectarLenguajeExt(tt.ext)
		if resultado != tt.esperado {
		t.Errorf("detectarLenguajeExt(%q) = %q, esperado %q",
				tt.ext, resultado, tt.esperado)
		}
	}
}

func TestPersistencia_FormatoJSON(t *testing.T) {
	a, _ := crearAlmacenTest(t)

	id, _ := a.Agregar("test.go", "package test\n", "completo", "go", 1, 1)

	// Verificar que el archivo JSON es válido
	rutaFrag := filepath.Join(a.Directorio(), id+".json")
	datos, err := os.ReadFile(rutaFrag)
	if err != nil {
		t.Fatalf("error leyendo archivo de fragmento: %v", err)
	}

	var frag Fragmento
	if err := json.Unmarshal(datos, &frag); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}

	if frag.ID != id {
		t.Error("ID en archivo no coincide")
	}
	if frag.Timestamp == "" {
		t.Error("timestamp no debería estar vacío")
	}
}