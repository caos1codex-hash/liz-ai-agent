package auto_creacion

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/registro"
)

// ============================================================================
// Helper: ¿está `go` disponible?
// ============================================================================

func goDisponible() bool {
	// Buscar el go instalado en el entorno de test
	if home, err := os.UserHomeDir(); err == nil {
		local := filepath.Join(home, "go-local", "go", "bin", "go")
		if _, err := os.Stat(local); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("go"); err == nil {
		return true
	}
	return false
}

// skipSinGo se llama al inicio de tests que requieren `go` para compilar.
func skipSinGo(t *testing.T) {
	t.Helper()
	if !goDisponible() {
		t.Skip("go no disponible en PATH — saltando test de compilación")
	}
}

// ============================================================================
// Tests del Compilador (requieren go)
// ============================================================================

func TestCompilador_CompilarStub_OK(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	comp := NuevoCompilador().ConLog(func(f string, a ...interface{}) { t.Logf(f, a...) })

	spec := SpecHerramienta{
		Nombre:      "stub_compilar_test",
		Descripcion: "Herramienta de prueba para compilación",
		Categoria:   "test",
	}
	gen, err := GenerarDesdePlantilla(spec)
	if err != nil {
		t.Fatalf("GenerarDesdePlantilla: %v", err)
	}

	res, err := comp.Compilar(context.Background(), tmp, gen.FuenteGo)
	if err != nil {
		t.Fatalf("Compilar: %v — log: %s", err, res.Log)
	}
	if !res.Exito {
		t.Fatalf("compilación falló: %s", res.Log)
	}
	if res.RutaBinario == "" {
		t.Error("RutaBinario vacía")
	}
	if _, err := os.Stat(res.RutaBinario); err != nil {
		t.Errorf("binario no existe en %s: %v", res.RutaBinario, err)
	}
	if res.Duracion <= 0 {
		t.Errorf("duración inválida: %v", res.Duracion)
	}

	// Verificar fuente escrito
	if _, err := os.Stat(res.RutaFuente); err != nil {
		t.Errorf("fuente no existe en %s", res.RutaFuente)
	}
}

func TestCompilador_Compilar_FuenteInvalido(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	comp := NuevoCompilador()

	// Fuente sin package main → validación falla antes de invocar go
	_, err := comp.Compilar(context.Background(), tmp, "esto no es Go")
	if err == nil {
		t.Fatal("esperaba error")
	}
}

func TestCompilador_Compilar_FuenteQueNoCompila(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	comp := NuevoCompilador().ConLog(func(f string, a ...interface{}) { t.Logf(f, a...) })

	// Fuente válido sintácticamente pero con error de compilación
	fuente := "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Printlnx(\"hola\") }"
	res, err := comp.Compilar(context.Background(), tmp, fuente)
	if err == nil {
		t.Fatal("esperaba error de compilación")
	}
	if res.Exito {
		t.Error("esperaba res.Exito=false")
	}
	if res.Log == "" {
		t.Error("esperaba log de error")
	}
	if !strings.Contains(res.Log, "Printlnx") && !strings.Contains(res.Log, "undefined") {
		t.Errorf("log no menciona el error: %s", res.Log)
	}
}

func TestCompilador_GoDisponible(t *testing.T) {
	skipSinGo(t)
	comp := NuevoCompilador()
	ver, err := comp.GoDisponible()
	if err != nil {
		t.Fatalf("GoDisponible: %v", err)
	}
	if !strings.Contains(ver, "go version") {
		t.Errorf("versión inesperada: %s", ver)
	}
	t.Logf("go: %s", ver)
}

// ============================================================================
// Tests del Cargador (subprocess wrapper) — requiere go para compilar stub
// ============================================================================

func TestHerramientaSubproceso_FlujoCompleto(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	comp := NuevoCompilador()

	spec := SpecHerramienta{
		Nombre:      "stub_subproceso_test",
		Descripcion: "Herramienta para test del cargador",
		Categoria:   "test",
		Parametros: []herramientas.Parametro{
			{Nombre: "msg", Tipo: "string", Requerido: true, Descripcion: "mensaje"},
		},
	}
	gen, err := GenerarDesdePlantilla(spec)
	if err != nil {
		t.Fatalf("GenerarDesdePlantilla: %v", err)
	}

	res, err := comp.Compilar(context.Background(), tmp, gen.FuenteGo)
	if err != nil {
		t.Fatalf("Compilar: %v", err)
	}

	// Crear wrapper
	h := NuevaHerramientaSubproceso(res.RutaBinario, spec.Nombre).
		ConLog(func(f string, a ...interface{}) { t.Logf(f, a...) })

	// Validar
	if err := h.Validar(); err != nil {
		t.Fatalf("Validar: %v", err)
	}

	// Nombre (forzado)
	if h.Nombre() != "stub_subproceso_test" {
		t.Errorf("Nombre: %q", h.Nombre())
	}

	// Descripcion (cargada vía op="info")
	desc := h.Descripcion()
	if !strings.Contains(desc, "Herramienta para test") {
		t.Errorf("Descripcion: %q", desc)
	}

	// Parametros
	params := h.Parametros()
	if len(params) != 1 {
		t.Fatalf("esperaba 1 parametro, got %d", len(params))
	}
	if params[0].Nombre != "msg" {
		t.Errorf("parametro[0].Nombre: %q", params[0].Nombre)
	}

	// Ejecutar (debe retornar error porque es un stub)
	r, err := h.Ejecutar(context.Background(), map[string]interface{}{
		"msg": "hola",
	})
	if err != nil {
		t.Fatalf("Ejecutar err: %v", err)
	}
	if r.Exito {
		t.Error("stub debe retornar exito=false en ejecutar")
	}
	if !strings.Contains(r.Error, "stub") {
		t.Errorf("error inesperado: %q", r.Error)
	}

	// Estadísticas
	est := h.Estadisticas()
	if est.VecesEjecutada != 1 {
		t.Errorf("VecesEjecutada: %d", est.VecesEjecutada)
	}
	if est.VecesExitosas != 0 {
		t.Errorf("VecesExitosas: %d (esperaba 0 porque el stub falla)", est.VecesExitosas)
	}
}

func TestHerramientaSubproceso_BinarioInexistente(t *testing.T) {
	h := NuevaHerramientaSubproceso("/no/existe/binario", "test_inexistente")

	err := h.Validar()
	if err == nil {
		t.Fatal("esperaba error")
	}
}

// ============================================================================
// Tests del Registro
// ============================================================================

func TestRegistro_FlujoCompleto(t *testing.T) {
	tmp := t.TempDir()
	reg, err := NuevoRegistro(tmp)
	if err != nil {
		t.Fatalf("NuevoRegistro: %v", err)
	}

	// Existe debe ser false
	if reg.Existe("test_tool") {
		t.Error("Existe debe ser false para tool nueva")
	}

	// Guardar
	meta := &MetadataHerramienta{
		SpecHerramienta: SpecHerramienta{
			Nombre:      "test_tool",
			Descripcion: "Tool de test",
			Categoria:   "test",
		},
		CreadoEn:        time.Now(),
		ActualizadoEn:   time.Now(),
		VersionContador: 1,
		Compila:         true,
	}
	if err := reg.Guardar(meta); err != nil {
		t.Fatalf("Guardar: %v", err)
	}

	// Existe debe ser true ahora
	if !reg.Existe("test_tool") {
		t.Error("Existe debe ser true tras Guardar")
	}

	// Obtener
	got, err := reg.Obtener("test_tool")
	if err != nil {
		t.Fatalf("Obtener: %v", err)
	}
	if got.Nombre != "test_tool" {
		t.Errorf("Nombre: %q", got.Nombre)
	}
	if got.VersionContador != 1 {
		t.Errorf("VersionContador: %d", got.VersionContador)
	}

	// Listar
	lista, err := reg.Listar()
	if err != nil {
		t.Fatalf("Listar: %v", err)
	}
	if len(lista) != 1 {
		t.Errorf("len(Listar): %d", len(lista))
	}

	// Guardar fuente
	if err := reg.GuardarFuente("test_tool", "package main\nfunc main(){}\n"); err != nil {
		t.Fatalf("GuardarFuente: %v", err)
	}

	// Leer fuente
	fuente, err := reg.LeerFuente("test_tool")
	if err != nil {
		t.Fatalf("LeerFuente: %v", err)
	}
	if !strings.Contains(fuente, "package main") {
		t.Errorf("fuente leído mal: %q", fuente)
	}

	// Incrementar estadísticas
	if err := reg.IncrementarEstadisticas("test_tool", true, ""); err != nil {
		t.Fatalf("IncrementarEstadisticas: %v", err)
	}
	got2, _ := reg.Obtener("test_tool")
	if got2.VecesEjecutada != 1 || got2.VecesExitosas != 1 {
		t.Errorf("stats mal: ejec=%d exit=%d", got2.VecesEjecutada, got2.VecesExitosas)
	}

	// Eliminar
	if err := reg.Eliminar("test_tool"); err != nil {
		t.Fatalf("Eliminar: %v", err)
	}
	if reg.Existe("test_tool") {
		t.Error("aún existe tras Eliminar")
	}
}

func TestRegistro_Obtener_NoCreada(t *testing.T) {
	tmp := t.TempDir()
	reg, _ := NuevoRegistro(tmp)

	_, err := reg.Obtener("no_existe")
	if err == nil {
		t.Fatal("esperaba error")
	}
}

func TestRegistro_Listar_Vacio(t *testing.T) {
	tmp := t.TempDir()
	reg, _ := NuevoRegistro(tmp)

	lista, err := reg.Listar()
	if err != nil {
		t.Fatalf("Listar: %v", err)
	}
	if len(lista) != 0 {
		t.Errorf("esperaba lista vacía, got %d", len(lista))
	}
}

// ============================================================================
// Tests del Gestor — flujo completo sin LLM (stub fallback)
// ============================================================================

func TestGestor_Crear_ConForzarNombre_SinLLM(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	cat := registro.NuevoCatalogo()

	g, err := NuevoGestor(nil, tmp, cat)
	if err != nil {
		t.Fatalf("NuevoGestor: %v", err)
	}
	g.ConLog(func(f string, a ...interface{}) { t.Logf("[gestor] "+f, a...) })

	if g.LLMDisponible() {
		t.Error("LLM no debe estar disponible con nil")
	}

	sol := SolicitudCreacion{
		ForzarNombre: "test_gestor_tool",
	}
	res, err := g.Crear(context.Background(), sol)
	if err != nil {
		t.Fatalf("Crear: %v — res.Error: %s", err, res.Error)
	}
	if !res.Registrada {
		t.Fatalf("no registrada — error: %s", res.Error)
	}
	if res.Especificacion.Nombre != "test_gestor_tool" {
		t.Errorf("Nombre: %q", res.Especificacion.Nombre)
	}
	if !res.Compilacion.Exito {
		t.Errorf("compilación falló: %s", res.Compilacion.Log)
	}
	if !res.CargaExitosa {
		t.Errorf("carga falló: %s", res.Error)
	}
	if res.Metadata == nil {
		t.Fatal("metadata nil")
	}
	if res.Metadata.VersionContador != 1 {
		t.Errorf("VersionContador: %d", res.Metadata.VersionContador)
	}

	// Verificar que está en el catálogo
	if !cat.Existe("test_gestor_tool") {
		t.Error("no está en el catálogo tras Crear")
	}

	// Verificar que está en el registro
	if !g.Registro().Existe("test_gestor_tool") {
		t.Error("no está en el registro tras Crear")
	}
}

func TestGestor_Crear_Duplicado(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	cat := registro.NuevoCatalogo()
	g, _ := NuevoGestor(nil, tmp, cat)

	// Primera creación
	sol := SolicitudCreacion{ForzarNombre: "test_duplicado"}
	_, err := g.Crear(context.Background(), sol)
	if err != nil {
		t.Fatalf("primera creación: %v", err)
	}

	// Segunda creación con mismo nombre → debe fallar
	_, err = g.Crear(context.Background(), sol)
	if err == nil {
		t.Fatal("esperaba error por duplicado")
	}
}

func TestGestor_CargarTodas_Persistencia(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	cat1 := registro.NuevoCatalogo()
	g1, _ := NuevoGestor(nil, tmp, cat1)
	g1.ConLog(func(f string, a ...interface{}) { t.Logf("[g1] "+f, a...) })

	// Crear 2 herramientas
	for _, nombre := range []string{"tool_persist_1", "tool_persist_2"} {
		_, err := g1.Crear(context.Background(), SolicitudCreacion{ForzarNombre: nombre})
		if err != nil {
			t.Fatalf("Crear %s: %v", nombre, err)
		}
	}
	if cat1.Tamaño() != 2 {
		t.Errorf("catálogo 1 tamaño: %d (esperaba 2)", cat1.Tamaño())
	}

	// Crear un NUEVO gestor apuntando al mismo directorio (simula reinicio de Liz)
	cat2 := registro.NuevoCatalogo()
	g2, _ := NuevoGestor(nil, tmp, cat2)
	g2.ConLog(func(f string, a ...interface{}) { t.Logf("[g2] "+f, a...) })

	cargadas, errs := g2.CargarTodas()
	if cargadas != 2 {
		t.Errorf("CargarTodas: %d (esperaba 2)", cargadas)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			t.Logf("err carga: %v", e)
		}
		t.Errorf("errs: %d", len(errs))
	}
	if cat2.Tamaño() != 2 {
		t.Errorf("catálogo 2 tamaño: %d (esperaba 2)", cat2.Tamaño())
	}
}

func TestGestor_Eliminar(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	cat := registro.NuevoCatalogo()
	g, _ := NuevoGestor(nil, tmp, cat)

	// Crear
	_, err := g.Crear(context.Background(), SolicitudCreacion{ForzarNombre: "test_eliminar"})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}
	if !cat.Existe("test_eliminar") {
		t.Fatal("no está en catálogo tras crear")
	}

	// Eliminar
	if err := g.Eliminar("test_eliminar"); err != nil {
		t.Fatalf("Eliminar: %v", err)
	}
	if cat.Existe("test_eliminar") {
		t.Error("aún en catálogo tras eliminar")
	}
	if g.Registro().Existe("test_eliminar") {
		t.Error("aún en registro tras eliminar")
	}

	// Eliminar de nuevo → error
	if err := g.Eliminar("test_eliminar"); err == nil {
		t.Error("esperaba error al eliminar inexistente")
	}
}

func TestGestor_Probar(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	cat := registro.NuevoCatalogo()
	g, _ := NuevoGestor(nil, tmp, cat)

	_, err := g.Crear(context.Background(), SolicitudCreacion{ForzarNombre: "test_probar"})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}

	// Probar (stub debe fallar controladamente)
	res, err := g.Probar(context.Background(), "test_probar", map[string]interface{}{
		"msg": "hola",
	})
	if err != nil {
		t.Logf("Probar err (esperado si stub): %v", err)
	}
	if res.Exito {
		t.Error("stub debe retornar exito=false")
	}

	// Verificar estadísticas actualizadas
	meta, _ := g.Obtener("test_probar")
	if meta.VecesEjecutada != 1 {
		t.Errorf("VecesEjecutada: %d", meta.VecesEjecutada)
	}
}

func TestGestor_Recargar_DesdeFuenteExistente(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	cat := registro.NuevoCatalogo()
	g, _ := NuevoGestor(nil, tmp, cat)

	_, err := g.Crear(context.Background(), SolicitudCreacion{ForzarNombre: "test_recargar"})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}

	// Recargar sin LLM, sin nuevo fuente (usa el existente)
	res, err := g.Recargar(context.Background(), "test_recargar", "", false)
	if err != nil {
		t.Fatalf("Recargar: %v", err)
	}
	if !res.CargaExitosa {
		t.Errorf("carga falló: %s", res.Error)
	}
	if res.Metadata.VersionContador != 2 {
		t.Errorf("VersionContador: %d (esperaba 2)", res.Metadata.VersionContador)
	}
}

func TestGestor_Recargar_ConNuevoFuente(t *testing.T) {
	skipSinGo(t)

	tmp := t.TempDir()
	cat := registro.NuevoCatalogo()
	g, _ := NuevoGestor(nil, tmp, cat)

	_, err := g.Crear(context.Background(), SolicitudCreacion{ForzarNombre: "test_recargar_nuevo"})
	if err != nil {
		t.Fatalf("Crear: %v", err)
	}

	// Generar un nuevo fuente con stub
	spec := SpecHerramienta{
		Nombre:      "test_recargar_nuevo",
		Descripcion: "Recargada con nuevo fuente",
		Categoria:   "test",
	}
	gen, err := GenerarDesdePlantilla(spec)
	if err != nil {
		t.Fatalf("GenerarDesdePlantilla: %v", err)
	}

	res, err := g.Recargar(context.Background(), "test_recargar_nuevo", gen.FuenteGo, false)
	if err != nil {
		t.Fatalf("Recargar: %v", err)
	}
	if !res.CargaExitosa {
		t.Errorf("carga falló: %s", res.Error)
	}
	if res.Metadata.VersionContador != 2 {
		t.Errorf("VersionContador: %d", res.Metadata.VersionContador)
	}
	if res.Metadata.Descripcion != "Recargada con nuevo fuente" {
		t.Errorf("Descripción no actualizada: %q", res.Metadata.Descripcion)
	}
}
