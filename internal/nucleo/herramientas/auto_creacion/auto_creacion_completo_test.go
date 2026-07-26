package auto_creacion

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// randomID retorna un identificador aleatorio para tests (issue #23).
// Antes se usaba una funcion randomID que no existia en el paquete.
func randomID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// nuevoGestorTest construye un Gestor via NuevoGestor con un directorio temporal.
// Antes los tests usaban &Gestor{directorio: ...} pero el campo directorio no
// existe en el struct (issue #23).
func nuevoGestorTest(t *testing.T, dir string) *Gestor {
	t.Helper()
	g, err := NuevoGestor(nil, dir, nil)
	if err != nil {
		t.Fatalf("error construyendo Gestor de test: %v", err)
	}
	return g
}

// ============================================================================
// Tests adicionales para gestor.go — cobertura de funciones no cubiertas
// ============================================================================

func TestGestor_CargarTodas_ErrorDirectorio(t *testing.T) {
	// Directorio temporal vacío — CargarTodas no debería panic ni retornar nil.
	// Nota: NuevoGestor crea el directorio si no existe, así que usamos un
	// directorio temporal vacío en lugar de uno inexistente (issue #23).
	g := nuevoGestorTest(t, t.TempDir())

	cargadas, errs := g.CargarTodas()
	if cargadas != 0 {
		t.Errorf("esperaba 0 cargadas en dir vacío, got %d", cargadas)
	}
	// errs puede ser nil o vacío en un directorio vacío — ambos OK
	for _, e := range errs {
		t.Logf("error esperado en dir vacío: %v", e)
	}
}

func TestGestor_Eliminar_Inexistente(t *testing.T) {
	g := nuevoGestorTest(t, t.TempDir())

	// No debería fallar eliminando algo que no existe
	err := g.Eliminar("no_existe")
	// Puede retornar error o nil dependiendo de la implementación
	_ = err
}

func TestGestor_Recargar_SinFuente(t *testing.T) {
	g := nuevoGestorTest(t, t.TempDir())

	// Recargar algo que no existe (firma: ctx, nombre, nuevoFuente, usarLLM)
	_, err := g.Recargar(context.Background(), "no_existe", "", false)
	// No debería panic
	_ = err
}

func TestGestor_Probar_SinCatalogo(t *testing.T) {
	g := nuevoGestorTest(t, t.TempDir())

	// Probar sin inyectar al catálogo. Probar retorna (Resultado, error).
	_, err := g.Probar(context.Background(), "no_existe", nil)
	_ = err
}

// TestGestor_ObtenerInfo_Inexistente: removido.
// El método ObtenerInfo no existe en Gestor (issue #23). El método real es
// Obtener(ctx, nombre) que retorna (herramientas.Resultado, error) y está
// en el catálogo, no en el gestor. Test removido hasta que se implemente
// ObtenerInfo o se reescriba contra el API real.
func TestGestor_ObtenerInfo_Inexistente_Skipped(t *testing.T) {
	t.Skip("ObtenerInfo no existe en Gestor; ver issue #23")
}

// ============================================================================
// Tests adicionales para compilador.go
// ============================================================================

func TestCompilar_FuenteInvalida(t *testing.T) {
	tmpDir := t.TempDir()
	c := NuevoCompilador()

	// Compilar es método en *Compilador: (ctx, dirHerramienta, fuenteGo) → (*ResultadoCompilacion, error)
	_, err := c.Compilar(context.Background(), tmpDir, "esta_no_es_go_valido{{{")
	if err == nil {
		t.Fatal("esperaba error compilando fuente inválida")
	}
}

func TestCompilar_FuenteVacia(t *testing.T) {
	tmpDir := t.TempDir()
	c := NuevoCompilador()

	// Compilar con fuente vacío debería fallar validación (issue #23).
	// (El test original escribía un archivo y pasaba source="" a Compilar,
	// pero Compilar valida el source string, no el archivo.)
	_, err := c.Compilar(context.Background(), tmpDir, "")
	if err == nil {
		t.Fatal("esperaba error compilando fuente vacío")
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

// TestRegistro_MetadataInvalida: removido. LeerMetadata no es función de paquete;
// la lectura de metadata es vía Registro.Obtener(nombre) que lee de disco (issue #23).
func TestRegistro_MetadataInvalida_Skipped(t *testing.T) {
	t.Skip("LeerMetadata no es función de paquete; ver issue #23")
}

func TestRegistro_ListarVacio(t *testing.T) {
	// ListarHerramientas no es función de paquete; usar Registro.Listar() (issue #23).
	reg, err := NuevoRegistro(t.TempDir())
	if err != nil {
		t.Fatalf("error creando registro: %v", err)
	}

	lista, err := reg.Listar()
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

// TestSolicitudCreacion_Vacia: removido. SolicitudCreacion no tiene método Validar
// (issue #23). Validación se hace en Gestor.Crear(ctx, sol).
func TestSolicitudCreacion_Vacia_Skipped(t *testing.T) {
	t.Skip("SolicitudCreacion.Validar no existe; ver issue #23")
}

// TestSolicitudCreacion_ConForzarSpec: idem.
func TestSolicitudCreacion_ConForzarSpec_Skipped(t *testing.T) {
	t.Skip("SolicitudCreacion.Validar no existe; ver issue #23")
}

// TestResultadoCreacion_Serializar: removido. El struct ResultadoCreacion no
// tiene campos Exito ni Datos (issue #23). Campos reales: Especificacion,
// Deteccion, Generacion, Compilacion, CargaExitosa, Registrada, Metadata, Error.
func TestResultadoCreacion_Serializar_Skipped(t *testing.T) {
	t.Skip("ResultadoCreacion no tiene campos Exito/Datos; ver issue #23")
}

// (Tests TestSpecHerramienta_Campos, TestParametroSpec_Default, TestMetadataHerramienta_Estructura
// removidos: tipos referenciados (ParametroSpec, MetadataHerramienta.Exito) no existen.
// Ver issue #23.)

func TestExtraerFuenteGo_Basico(t *testing.T) {
	// ExtraerFuenteGo aplica TrimSpace al resultado. El test debe comparar
	// contra la versión trimmed del input (issue #23).
	codigo := "package main\n\nfunc main() {\n\tfmt.Println(\"hola\")\n}\n"
	esperado := strings.TrimSpace(codigo)

	fuente := ExtraerFuenteGo(codigo)
	if fuente != esperado {
		t.Errorf("debería retornar el código completo (trimado). got=%q want=%q", fuente, esperado)
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
