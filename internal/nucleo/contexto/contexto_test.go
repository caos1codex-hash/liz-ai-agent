package contexto

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Setup Helper
// ============================================================================

// crearProyectoTest crea un directorio temporal con estructura de proyecto
// y retorna su ruta absoluta.
func crearProyectoTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Estructura:
	//   main.go
	//   README.md
	//   src/
	//     auth.go
	//     db.go
	//   tests/
	//     auth_test.go
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.MkdirAll(filepath.Join(dir, "tests"), 0755)

	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "fmt"

func main() {
	fmt.Println("hola")
}

func saludar() string {
	return "hola"
}
`), 0644)

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Proyecto Test\n\nDocumentación de prueba.\n"), 0644)

	os.WriteFile(filepath.Join(dir, "src", "auth.go"), []byte(`package src

type Usuario struct {
	Nombre string
}

func Autenticar(u Usuario) bool {
	return u.Nombre != ""
}
`), 0644)

	os.WriteFile(filepath.Join(dir, "src", "db.go"), []byte(`package src

import "database/sql"

func Conectar() (*sql.DB, error) {
	return nil, nil
}
`), 0644)

	os.WriteFile(filepath.Join(dir, "tests", "auth_test.go"), []byte(`package tests

import "testing"

func TestAutenticar(t *testing.T) {
	if true != true {
		t.Fail()
	}
}
`), 0644)

	return dir
}

// crearCoordinadorTest crea un coordinador con un directorio temporal base.
func crearCoordinadorTest(t *testing.T) *Coordinador {
	t.Helper()
	dirBase := t.TempDir()
	c, err := NuevoCoordinador(dirBase)
	if err != nil {
		t.Fatalf("NuevoCoordinador: %v", err)
	}
	return c
}

// ============================================================================
// Tests de Indexación
// ============================================================================

func TestIndexarProyecto_Basico(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, err := c.IndexarProyecto(proyectoDir)
	if err != nil {
		t.Fatalf("IndexarProyecto: %v", err)
	}

	if estado.Nombre == "" {
		t.Error("Nombre no debería estar vacío")
	}
	if estado.TotalArchivos == 0 {
		t.Error("TotalArchivos debería ser > 0")
	}
	if estado.TotalFragmentos == 0 {
		t.Error("TotalFragmentos debería ser > 0")
	}
	if !estado.MapaGenerado {
		t.Error("MapaGenerado debería ser true")
	}
	if !estado.IndiceGenerado {
		t.Error("IndiceGenerado debería ser true")
	}
}

func TestIndexarProyecto_PersistenciaEstado(t *testing.T) {
	// Después de indexar, el estado debe estar persistido en disco
	dirBase := t.TempDir()
	proyectoDir := crearProyectoTest(t)

	c1, _ := NuevoCoordinador(dirBase)
	c1.IndexarProyecto(proyectoDir)
	nombreProyecto := filepath.Base(proyectoDir)

	// Verificar archivo de estado
	rutaEstado := filepath.Join(dirBase, nombreProyecto, ".liz", "estado.json")
	if _, err := os.Stat(rutaEstado); err != nil {
		t.Fatalf("estado.json debería existir: %v", err)
	}

	// Crear nuevo coordinador (simular reinicio)
	c2, err := NuevoCoordinador(dirBase)
	if err != nil {
		t.Fatalf("NuevoCoordinador: %v", err)
	}

	proyectos := c2.ListarProyectos()
	if len(proyectos) == 0 {
		t.Error("debería cargar el proyecto desde disco")
	}

	encontrado := false
	for _, p := range proyectos {
		if p.Nombre == nombreProyecto {
			encontrado = true
			if p.TotalArchivos == 0 {
				t.Error("TotalArchivos debería estar cargado")
			}
		}
	}
	if !encontrado {
		t.Error("proyecto no encontrado después de reinicio")
	}
}

func TestIndexarProyecto_ReindexacionIncremental(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	// Primera indexación
	estado1, _ := c.IndexarProyecto(proyectoDir)
	fragmentos1 := estado1.TotalFragmentos

	// Modificar un archivo existente
	os.WriteFile(filepath.Join(proyectoDir, "main.go"), []byte(`package main

import "fmt"

func main() {
	fmt.Println("hola modificado")
}

func saludar() string {
	return "hola"
}

func despedirse() string {
	return "chau"
}
`), 0644)

	// Re-indexar
	estado2, _ := c.IndexarProyecto(proyectoDir)

	// Debe seguir teniendo archivos pero los fragmentos pueden cambiar
	if estado2.TotalArchivos != estado1.TotalArchivos {
		t.Errorf("TotalArchivos debería ser igual (%d vs %d)", estado1.TotalArchivos, estado2.TotalArchivos)
	}
	// No debe perder fragmentos (al menos la misma cantidad)
	if estado2.TotalFragmentos < fragmentos1 {
		t.Errorf("TotalFragmentos no debería disminuir: %d < %d", estado2.TotalFragmentos, fragmentos1)
	}
}

// ============================================================================
// Tests de Consultas
// ============================================================================

func TestObtenerMapa(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	mapa, err := c.ObtenerMapa(estado.Nombre)
	if err != nil {
		t.Fatalf("ObtenerMapa: %v", err)
	}
	if mapa == nil {
		t.Fatal("mapa no debería ser nil")
	}
	if mapa.Proyecto == "" {
		t.Error("mapa.Proyecto no debería estar vacío")
	}
}

func TestObtenerIndice(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	indice, err := c.ObtenerIndice(estado.Nombre)
	if err != nil {
		t.Fatalf("ObtenerIndice: %v", err)
	}
	if indice.TotalArchivos == 0 {
		t.Error("indice.TotalArchivos debería ser > 0")
	}
}

func TestObtenerArbol(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	arbol, err := c.ObtenerArbol(estado.Nombre)
	if err != nil {
		t.Fatalf("ObtenerArbol: %v", err)
	}
	if arbol == nil {
		t.Fatal("arbol no debería ser nil")
	}
	if !arbol.EsDir {
		t.Error("la raíz del árbol debería ser directorio")
	}
	if arbol.TotalArchivos == 0 {
		t.Error("TotalArchivos del árbol debería ser > 0")
	}
}

func TestBuscarEnIndice(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	// Buscar "auth"
	resultados := c.BuscarEnIndice(estado.Nombre, "auth")
	if len(resultados) == 0 {
		t.Error("debería encontrar archivos con 'auth'")
	}

	// Buscar algo que no existe
	resultados = c.BuscarEnIndice(estado.Nombre, "noexisteenelfichero")
	if len(resultados) != 0 {
		t.Errorf("no debería encontrar nada, got %d resultados", len(resultados))
	}
}

func TestObtenerFragmento(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)
	indice, _ := c.ObtenerIndice(estado.Nombre)

	// Tomar el primer archivo con fragmentos
	var primerID string
	for _, entrada := range indice.Archivos {
		if len(entrada.FragmentoIDs) > 0 {
			primerID = entrada.FragmentoIDs[0]
			break
		}
	}
	if primerID == "" {
		t.Skip("no hay fragmentos para probar")
	}

	frag, err := c.ObtenerFragmento(estado.Nombre, primerID)
	if err != nil {
		t.Fatalf("ObtenerFragmento: %v", err)
	}
	if frag == nil {
		t.Fatal("fragmento no debería ser nil")
	}
	if frag.ID != primerID {
		t.Error("ID no coincide")
	}
}

func TestObtenerFragmentosPorRuta(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	frags, err := c.ObtenerFragmentosPorRuta(estado.Nombre, "main.go")
	if err != nil {
		t.Fatalf("ObtenerFragmentosPorRuta: %v", err)
	}
	if len(frags) == 0 {
		t.Error("debería tener fragmentos para main.go")
	}
}

func TestObtenerResumen_ConPersistencia(t *testing.T) {
	dirBase := t.TempDir()
	proyectoDir := crearProyectoTest(t)

	c, _ := NuevoCoordinador(dirBase)
	estado, _ := c.IndexarProyecto(proyectoDir)

	rutaAbs := filepath.Join(proyectoDir, "main.go")
	r1, err := c.ObtenerResumen(estado.Nombre, "main.go", rutaAbs)
	if err != nil {
		t.Fatalf("ObtenerResumen (1ra llamada): %v", err)
	}
	if r1 == nil {
		t.Fatal("resumen no debería ser nil")
	}
	if r1.Lineas == 0 {
		t.Error("r1.Lineas debería ser > 0")
	}

	// Verificar que se persistió a disco
	rutaResumen := filepath.Join(dirBase, estado.Nombre, ".liz", "resumenes", "main.go.json")
	if _, err := os.Stat(rutaResumen); err != nil {
		t.Errorf("resumen debería estar persistido en disco: %v", err)
	}

	// Segunda llamada debe usar cache (sin regenerar)
	r2, err := c.ObtenerResumen(estado.Nombre, "main.go", rutaAbs)
	if err != nil {
		t.Fatalf("ObtenerResumen (2da llamada): %v", err)
	}
	if r2.Ruta != r1.Ruta {
		t.Error("r2 debería ser igual a r1 (cache)")
	}
}

func TestForzarResumen_Regenera(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)
	rutaAbs := filepath.Join(proyectoDir, "main.go")

	// Generar y cachear
	c.ObtenerResumen(estado.Nombre, "main.go", rutaAbs)

	// Forzar regeneración
	r, err := c.ForzarResumen(estado.Nombre, "main.go", rutaAbs)
	if err != nil {
		t.Fatalf("ForzarResumen: %v", err)
	}
	if r == nil {
		t.Fatal("resumen no debería ser nil")
	}
}

// ============================================================================
// Tests de Eliminación
// ============================================================================

func TestEliminarProyecto(t *testing.T) {
	dirBase := t.TempDir()
	proyectoDir := crearProyectoTest(t)

	c, _ := NuevoCoordinador(dirBase)
	estado, _ := c.IndexarProyecto(proyectoDir)
	nombre := estado.Nombre

	// Verificar que existe
	proyectos := c.ListarProyectos()
	if len(proyectos) != 1 {
		t.Fatalf("debería haber 1 proyecto, got %d", len(proyectos))
	}

	// Eliminar
	if err := c.EliminarProyecto(nombre); err != nil {
		t.Fatalf("EliminarProyecto: %v", err)
	}

	// Verificar que ya no está en la lista
	proyectos = c.ListarProyectos()
	if len(proyectos) != 0 {
		t.Errorf("debería haber 0 proyectos, got %d", len(proyectos))
	}

	// Verificar que el directorio fue borrado
	rutaProy := filepath.Join(dirBase, nombre)
	if _, err := os.Stat(rutaProy); !os.IsNotExist(err) {
		t.Error("el directorio del proyecto debería haber sido borrado")
	}

	// Eliminar de nuevo debe dar error
	if err := c.EliminarProyecto(nombre); err == nil {
		t.Error("eliminar proyecto inexistente debería dar error")
	}
}

// ============================================================================
// Tests de Reindexación Selectiva
// ============================================================================

func TestReindexarArchivo_Modificado(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	// Modificar main.go
	os.WriteFile(filepath.Join(proyectoDir, "main.go"), []byte(`package main

import "fmt"

func main() {
	fmt.Println("cambiado")
}

func nueva() string {
	return "nueva"
}
`), 0644)

	// Re-indexar solo main.go
	if err := c.ReindexarArchivo(estado.Nombre, "main.go"); err != nil {
		t.Fatalf("ReindexarArchivo: %v", err)
	}

	// Verificar que los fragmentos se actualizaron
	frags, _ := c.ObtenerFragmentosPorRuta(estado.Nombre, "main.go")
	if len(frags) == 0 {
		t.Error("debería tener fragmentos después de reindexar")
	}
}

func TestReindexarArchivo_Eliminado(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	// Eliminar el archivo del disco
	os.Remove(filepath.Join(proyectoDir, "main.go"))

	// Reindexar archivo eliminado
	if err := c.ReindexarArchivo(estado.Nombre, "main.go"); err != nil {
		t.Fatalf("ReindexarArchivo eliminado: %v", err)
	}

	// No debería tener fragmentos
	frags, _ := c.ObtenerFragmentosPorRuta(estado.Nombre, "main.go")
	if len(frags) != 0 {
		t.Errorf("no debería tener fragmentos después de eliminar, got %d", len(frags))
	}
}

// ============================================================================
// Tests de Proyecto Inexistente
// ============================================================================

func TestOperacionesProyectoInexistente(t *testing.T) {
	c := crearCoordinadorTest(t)

	if _, err := c.ObtenerMapa("no-existe"); err == nil {
		t.Error("ObtenerMapa en proyecto inexistente debería fallar")
	}
	if _, err := c.ObtenerIndice("no-existe"); err == nil {
		t.Error("ObtenerIndice en proyecto inexistente debería fallar")
	}
	if _, err := c.ObtenerArbol("no-existe"); err == nil {
		t.Error("ObtenerArbol en proyecto inexistente debería fallar")
	}
	if _, err := c.ObtenerFragmento("no-existe", "x"); err == nil {
		t.Error("ObtenerFragmento en proyecto inexistente debería fallar")
	}
	if _, err := c.ObtenerFragmentosPorRuta("no-existe", "x"); err == nil {
		t.Error("ObtenerFragmentosPorRuta en proyecto inexistente debería fallar")
	}
	if _, err := c.ObtenerResumen("no-existe", "x", "/x"); err == nil {
		t.Error("ObtenerResumen en proyecto inexistente debería fallar")
	}
	if err := c.ReindexarArchivo("no-existe", "x"); err == nil {
		t.Error("ReindexarArchivo en proyecto inexistente debería fallar")
	}
}

// ============================================================================
// Tests de Múltiples Proyectos
// ============================================================================

func TestMultiplesProyectos(t *testing.T) {
	c := crearCoordinadorTest(t)

	dir1 := crearProyectoTest(t)
	dir2 := crearProyectoTest(t)

	c.IndexarProyecto(dir1)
	c.IndexarProyecto(dir2)

	proyectos := c.ListarProyectos()
	if len(proyectos) != 2 {
		t.Errorf("debería haber 2 proyectos, got %d", len(proyectos))
	}
}
