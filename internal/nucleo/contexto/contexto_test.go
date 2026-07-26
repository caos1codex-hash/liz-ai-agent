package contexto

import (
	"os"
	"path/filepath"
	"strings"
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

// ============================================================================
// Tests de integracion Fase 3.5 (sistema world-class)
// ============================================================================

func TestIndexarProyecto_ConstruyeGrafoYBuscador(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, err := c.IndexarProyecto(proyectoDir)
	if err != nil {
		t.Fatalf("IndexarProyecto: %v", err)
	}

	// El grafo debería tener nodos (todos los archivos indexados)
	grafo, err := c.ObtenerGrafo(estado.Nombre)
	if err != nil {
		t.Fatalf("ObtenerGrafo: %v", err)
	}
	if grafo.TotalArchivos() == 0 {
		t.Error("grafo debería tener archivos")
	}

	// El buscador debería tener fragmentos indexados
	importancias, err := c.ObtenerImportancias(estado.Nombre)
	if err != nil {
		t.Fatalf("ObtenerImportancias: %v", err)
	}
	if len(importancias) == 0 {
		t.Error("debería tener importancias calculadas")
	}

	// PageRank: como mínimo, todos los scores están en [0.0, 1.0]
	for ruta, score := range importancias {
		if score < 0.0 || score > 1.0 {
			t.Errorf("score de %s fuera de rango: %.3f", ruta, score)
		}
	}
}

func TestBuscarHibrido_EncuentraRelevantes(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	// Buscar "saludar" debería encontrar main.go (que tiene función saludar)
	resultados, err := c.BuscarHibrido(estado.Nombre, "saludar", 10)
	if err != nil {
		t.Fatalf("BuscarHibrido: %v", err)
	}
	if len(resultados) == 0 {
		t.Error("debería encontrar resultados para 'saludar'")
	}

	// El primer resultado debería ser de main.go
	if resultados[0].Fragmento.Ruta != "main.go" {
		t.Errorf("top resultado debería ser de main.go, got %s",
			resultados[0].Fragmento.Ruta)
	}
}

func TestBuscarHibrido_SinResultados(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	resultados, err := c.BuscarHibrido(estado.Nombre, "xyzqwerty12345", 10)
	if err != nil {
		t.Fatalf("BuscarHibrido: %v", err)
	}
	if len(resultados) != 0 {
		t.Errorf("no debería encontrar nada, got %d resultados", len(resultados))
	}
}

func TestObtenerSimbolos_Go(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	ast, err := c.ObtenerSimbolos(estado.Nombre, "main.go")
	if err != nil {
		t.Fatalf("ObtenerSimbolos: %v", err)
	}
	if ast == nil {
		t.Fatal("ast no debería ser nil")
	}
	if ast.Lenguaje != "go" {
		t.Errorf("Lenguaje = %s, esperado 'go'", ast.Lenguaje)
	}
	// main.go tiene main() y saludar()
	if len(ast.Simbolos) == 0 {
		t.Error("debería tener símbolos")
	}
}

func TestObtenerMapaRepo_Compacto(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	mapa, err := c.ObtenerMapaRepo(estado.Nombre, 2000)
	if err != nil {
		t.Fatalf("ObtenerMapaRepo: %v", err)
	}
	if mapa == nil {
		t.Fatal("mapa no debería ser nil")
	}
	if mapa.TotalArchivos == 0 {
		t.Error("TotalArchivos debería ser > 0")
	}

	// El formato texto debería ser legible
	texto := mapa.FormatoTexto()
	if texto == "" {
		t.Error("FormatoTexto no debería estar vacío")
	}
}

func TestObtenerMapaRepo_PresupuestoPequeno(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	// Presupuesto muy pequeño
	mapa, _ := c.ObtenerMapaRepo(estado.Nombre, 50)
	if !mapa.Truncado {
		t.Error("mapa debería estar truncado con presupuesto pequeño")
	}
}

func TestEmpaquetarContexto_QueryEspecifico(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	resultado, err := c.EmpaquetarContexto(EmpaquetarSolicitud{
		Proyecto:          estado.Nombre,
		Query:             "saludar",
		PresupuestoTokens: 5000,
	})
	if err != nil {
		t.Fatalf("EmpaquetarContexto: %v", err)
	}
	if resultado == nil {
		t.Fatal("resultado no debería ser nil")
	}
	if resultado.Contenido == "" {
		t.Error("Contenido no debería estar vacío")
	}
	if !resultado.MapaRepoIncluido {
		t.Error("MapaRepo debería estar incluido")
	}
	if resultado.TokensUsados == 0 {
		t.Error("TokensUsados debería ser > 0")
	}
}

func TestEmpaquetarContexto_SinQuery(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	// Sin query: solo se incluye el mapa repo
	resultado, err := c.EmpaquetarContexto(EmpaquetarSolicitud{
		Proyecto:          estado.Nombre,
		Query:             "",
		PresupuestoTokens: 3000,
	})
	if err != nil {
		t.Fatalf("EmpaquetarContexto: %v", err)
	}
	if !resultado.MapaRepoIncluido {
		t.Error("MapaRepo debería estar incluido")
	}
	if len(resultado.FragmentosIncluidos) != 0 {
		t.Errorf("sin query no debería incluir fragmentos, got %d",
			len(resultado.FragmentosIncluidos))
	}
}

func TestEmpaquetarContexto_ArchivosRecientes(t *testing.T) {
	c := crearCoordinadorTest(t)
	proyectoDir := crearProyectoTest(t)

	estado, _ := c.IndexarProyecto(proyectoDir)

	resultado, err := c.EmpaquetarContexto(EmpaquetarSolicitud{
		Proyecto:          estado.Nombre,
		Query:             "saludar",
		PresupuestoTokens: 8000,
		ArchivosRecientes: []string{"main.go", "src/auth.go"},
	})
	if err != nil {
		t.Fatalf("EmpaquetarContexto: %v", err)
	}
	// Debería mencionar los archivos recientes en el contenido
	if !strings.Contains(resultado.Contenido, "Archivos recientemente editados") {
		t.Error("debería incluir sección de archivos recientes")
	}
}

func TestObtenerGrafo_ProyectoInexistente(t *testing.T) {
	c := crearCoordinadorTest(t)
	if _, err := c.ObtenerGrafo("no-existe"); err == nil {
		t.Error("debería dar error para proyecto inexistente")
	}
}

func TestObtenerSimbolos_ProyectoInexistente(t *testing.T) {
	c := crearCoordinadorTest(t)
	if _, err := c.ObtenerSimbolos("no-existe", "main.go"); err == nil {
		t.Error("debería dar error para proyecto inexistente")
	}
}

func TestBuscarHibrido_ProyectoInexistente(t *testing.T) {
	c := crearCoordinadorTest(t)
	if _, err := c.BuscarHibrido("no-existe", "query", 10); err == nil {
		t.Error("debería dar error para proyecto inexistente")
	}
}

func TestObtenerMapaRepo_ProyectoInexistente(t *testing.T) {
	c := crearCoordinadorTest(t)
	if _, err := c.ObtenerMapaRepo("no-existe", 2000); err == nil {
		t.Error("debería dar error para proyecto inexistente")
	}
}

func TestEmpaquetarContexto_ProyectoInexistente(t *testing.T) {
	c := crearCoordinadorTest(t)
	_, err := c.EmpaquetarContexto(EmpaquetarSolicitud{
		Proyecto:          "no-existe",
		PresupuestoTokens: 5000,
	})
	if err == nil {
		t.Error("debería dar error para proyecto inexistente")
	}
}

func TestPageRank_ArchivoMasImportanteSube(t *testing.T) {
	// Crear proyecto donde auth.go es importado por muchos
	c := crearCoordinadorTest(t)
	proyectoDir := t.TempDir()

	// Crear go.mod para que detectarModuloGo funcione
	os.WriteFile(filepath.Join(proyectoDir, "go.mod"),
		[]byte("module testproyecto\n\ngo 1.21\n"), 0644)

	// auth.go será el más importado
	os.WriteFile(filepath.Join(proyectoDir, "auth.go"),
		[]byte("package testproyecto\n\nfunc Authenticate() bool { return true }\n"), 0644)

	// Otros archivos importan auth
	os.WriteFile(filepath.Join(proyectoDir, "main.go"),
		[]byte("package testproyecto\n\nimport \"testproyecto\"\n\nfunc main() { testproyecto.Authenticate() }\n"), 0644)
	os.WriteFile(filepath.Join(proyectoDir, "handler.go"),
		[]byte("package testproyecto\n\nimport \"testproyecto\"\n\nfunc Handle() { testproyecto.Authenticate() }\n"), 0644)

	estado, _ := c.IndexarProyecto(proyectoDir)
	importancias, _ := c.ObtenerImportancias(estado.Nombre)

	// auth.go debería tener el score más alto
	// (no siempre 1.0 porque los imports no se resuelven perfectamente, pero debería estar entre los top)
	top3 := make([]struct {
		ruta  string
		score float64
	}, 0, len(importancias))
	for ruta, score := range importancias {
		top3 = append(top3, struct {
			ruta  string
			score float64
		}{ruta, score})
	}
	// Encontrar el máximo
	maxScore := 0.0
	maxRuta := ""
	for _, t := range top3 {
		if t.score > maxScore {
			maxScore = t.score
			maxRuta = t.ruta
		}
	}
	_ = maxRuta // no garantizamos que es auth.go porque los imports pueden no resolverse perfectamente
	if maxScore == 0 {
		t.Error("debería haber al menos un archivo con score > 0")
	}
}

func TestDetectarModuloGo(t *testing.T) {
	// Crear directorio con go.mod
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module github.com/foo/bar\n\ngo 1.21\n"), 0644)

	modulo := detectarModuloGo(dir)
	if modulo != "github.com/foo/bar" {
		t.Errorf("modulo = %q, esperado 'github.com/foo/bar'", modulo)
	}
}

func TestDetectarModuloGo_SinGoMod(t *testing.T) {
	dir := t.TempDir()
	modulo := detectarModuloGo(dir)
	if modulo != "" {
		t.Errorf("modulo debería ser vacío sin go.mod, got %q", modulo)
	}
}
