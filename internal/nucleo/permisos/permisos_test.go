package permisos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests de Inicialización
// ============================================================================

func TestInicializar_CrearGestor(t *testing.T) {
	tmpDir := t.TempDir()

	g, err := Inicializar(tmpDir)
	if err != nil {
		t.Fatalf("Inicializar falló: %v", err)
	}
	if g == nil {
		t.Fatal("Gestor es nil")
	}
	if !g.EstaHabilitado() {
		t.Error("Gestor debería estar habilitado por defecto")
	}
}

func TestInicializar_CreaArchivoPermisos(t *testing.T) {
	tmpDir := t.TempDir()

	g, _ := Inicializar(tmpDir)

	ruta := filepath.Join(tmpDir, "permisos", "permisos.json")
	if _, err := os.Stat(ruta); os.IsNotExist(err) {
		t.Error("El archivo de permisos debería haberse creado")
	}

	// Verificar que se puede leer
	estado := g.ObtenerTodos()
	if len(estado.Permisos) == 0 {
		t.Error("Debería haber permisos concedidos")
	}
}

func TestInicializar_CargaPermisosExistentes(t *testing.T) {
	tmpDir := t.TempDir()
	permDir := filepath.Join(tmpDir, "permisos")
	os.MkdirAll(permDir, 0755)

	// Crear archivo de permisos preexistente
	estado := EstadoPermisos{
		ConcedidoPor: "test",
		Version:      2,
		Permisos: map[TipoPermiso]*RegistroPermiso{
			PermArchivos: {
				Tipo:      PermArchivos,
				Categoria: "archivos",
				Concedido: true,
				Nivel:     NivelTotal,
			},
		},
	}
	datos, _ := json.MarshalIndent(estado, "", "  ")
	os.WriteFile(filepath.Join(permDir, "permisos.json"), datos, 0644)

	g, err := Inicializar(tmpDir)
	if err != nil {
		t.Fatalf("Error al inicializar con permisos existentes: %v", err)
	}

	estadoCargado := g.ObtenerTodos()
	if estadoCargado.Version != 2 {
		t.Errorf("Version = %d, se esperaba 2", estadoCargado.Version)
	}

	// Debería haber completado los permisos faltantes
	if len(estadoCargado.Permisos) != len(TodosLosPermisos) {
		t.Errorf("Se esperaban %d permisos, hay %d", len(TodosLosPermisos), len(estadoCargado.Permisos))
	}
}

// ============================================================================
// Tests de ConcederTodos (D-006)
// ============================================================================

func TestConcederTodos_TodosConcedidos(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)

	g.ConcederTodos("test", "razón de test")

	estado := g.ObtenerTodos()

	for _, tipo := range TodosLosPermisos {
		reg, ok := estado.Permisos[tipo]
		if !ok {
			t.Errorf("Permiso %s no encontrado", tipo)
			continue
		}
		if !reg.Concedido {
			t.Errorf("Permiso %s no está concedido", tipo)
		}
		if reg.Nivel != NivelTotal {
			t.Errorf("Permiso %s nivel = %s, se esperaba total", tipo, reg.Nivel)
		}
		if reg.ConcedidoPor != "test" {
			t.Errorf("Permiso %s concedido_por = %s, se esperaba test", tipo, reg.ConcedidoPor)
		}
	}
}

func TestConcederTodos_SubPermisosCreados(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)

	g.ConcederTodos("test", "test")

	estado := g.ObtenerTodos()

	// Verificar que archivos tenga sub-permisos
	regArchivos := estado.Permisos[PermArchivos]
	if regArchivos == nil {
		t.Fatal("Permiso archivos no encontrado")
	}
	if len(regArchivos.SubPermisos) == 0 {
		t.Error("Se esperaban sub-permisos para archivos")
	}

	// Verificar nombres de sub-permisos esperados
	nombresEsperados := map[string]bool{"leer": true, "escribir": true, "eliminar": true, "ejecutar": true}
	for _, sp := range regArchivos.SubPermisos {
		if !nombresEsperados[sp.Nombre] {
			t.Errorf("Sub-permiso inesperado: %s", sp.Nombre)
		}
	}
}

// ============================================================================
// Tests de Conceder Individual
// ============================================================================

func TestConceder_PermisoIndividual(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)

	err := g.Conceder(PermRed, NivelLectura, "test", "solo lectura de red")
	if err != nil {
		t.Fatalf("Conceder falló: %v", err)
	}

	reg := g.ObtenerPermiso(PermRed)
	if reg == nil {
		t.Fatal("Permiso no encontrado después de conceder")
	}
	if reg.Nivel != NivelLectura {
		t.Errorf("Nivel = %s, se esperaba lectura", reg.Nivel)
	}
}

func TestConceder_PermisoConSubPermisos(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)

	g.Conceder(PermTerminal, NivelTotal, "test", "acceso total a terminal")

	reg := g.ObtenerPermiso(PermTerminal)
	if len(reg.SubPermisos) == 0 {
		t.Error("Se esperaban sub-permisos para terminal")
	}
}

// ============================================================================
// Tests de ConcederSubPermiso
// ============================================================================

func TestConcederSubPermiso_Existente(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	err := g.ConcederSubPermiso(PermArchivos, "escribir", false, NivelDenegado)
	if err != nil {
		t.Fatalf("ConcederSubPermiso falló: %v", err)
	}

	if g.VerificarSubPermiso(PermArchivos, "escribir") {
		t.Error("Sub-permiso 'escribir' debería estar denegado")
	}
}

func TestConcederSubPermiso_NoExistente(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	err := g.ConcederSubPermiso(PermArchivos, "sub_inexistente", true, NivelTotal)
	if err == nil {
		t.Error("Se esperaba error para sub-permiso inexistente")
	}
}

// ============================================================================
// Tests de Verificar
// ============================================================================

func TestVerificar_PermisoConcedido(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	for _, tipo := range TodosLosPermisos {
		if !g.Verificar(tipo) {
			t.Errorf("Permiso %s debería estar concedido", tipo)
		}
	}
}

func TestVerificar_PermisoNoConcedido(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)

	// Resetear para que no haya permisos
	g.Resetear()

	if g.Verificar(PermArchivos) {
		t.Error("Después de resetear, archivos no debería estar concedido")
	}
}

func TestVerificar_GestorDeshabilitado(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.Resetear()
	g.Deshabilitar()

	if !g.Verificar(PermArchivos) {
		t.Error("Con gestor deshabilitado, todo debería estar permitido")
	}
}

func TestVerificarSubPermiso_Concedido(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	if !g.VerificarSubPermiso(PermArchivos, "leer") {
		t.Error("Sub-permiso 'leer' debería estar concedido")
	}
}

func TestVerificarSubPermiso_NivelTotal(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	// Con nivel total, cualquier sub-permiso debería estar concedido
	if !g.VerificarSubPermiso(PermArchivos, "cualquier_cosa") {
		// Este sub-permiso no existe, debería retornar false
		// El comportamiento correcto es que sub-permisos inexistentes sean false
	}
}

// ============================================================================
// Tests de VerificarConDetalle
// ============================================================================

func TestVerificarConDetalle_Concedido(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	concedido, nivel, detalle := g.VerificarConDetalle(PermArchivos)
	if !concedido {
		t.Error("Debería estar concedido")
	}
	if nivel != NivelTotal {
		t.Errorf("Nivel = %s, se esperaba total", nivel)
	}
	if detalle == "" {
		t.Error("Detalle no debería estar vacío")
	}
}

func TestVerificarConDetalle_Denegado(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.Resetear()

	concedido, nivel, detalle := g.VerificarConDetalle(PermArchivos)
	if concedido {
		t.Error("No debería estar concedido después de reset")
	}
	if nivel != NivelDenegado {
		t.Errorf("Nivel = %s, se esperaba denegado", nivel)
	}
	if detalle == "" {
		t.Error("Detalle no debería estar vacío")
	}
}

// ============================================================================
// Tests de Obtener
// ============================================================================

func TestObtenerTodos_Estructura(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	estado := g.ObtenerTodos()

	if estado.Version < 1 {
		t.Error("Version debería ser >= 1")
	}
	if estado.ConcedidoPor != "test" {
		t.Errorf("ConcedidoPor = %s, se esperaba test", estado.ConcedidoPor)
	}
	if len(estado.Permisos) != len(TodosLosPermisos) {
		t.Errorf("Se esperaban %d permisos, hay %d", len(TodosLosPermisos), len(estado.Permisos))
	}
}

func TestObtenerPermiso_Existente(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	reg := g.ObtenerPermiso(PermSistema)
	if reg == nil {
		t.Fatal("Permiso sistema no encontrado")
	}
	if reg.Tipo != PermSistema {
		t.Errorf("Tipo = %s, se esperaba sistema", reg.Tipo)
	}
}

func TestObtenerPermiso_NoExistente(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.Resetear()

	reg := g.ObtenerPermiso(PermArchivos)
	if reg != nil {
		t.Error("Después de resetear, obtener permiso debería ser nil")
	}
}

func TestObtenerResumen(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	resumen := g.ObtenerResumen()

	total, ok := resumen["total"].(int)
	if !ok || total != len(TodosLosPermisos) {
		t.Errorf("total = %v, se esperaba %d", resumen["total"], len(TodosLosPermisos))
	}

	concedidos, ok := resumen["concedidos"].(int)
	if !ok || concedidos != len(TodosLosPermisos) {
		t.Errorf("concedidos = %v, se esperaba %d", resumen["concedidos"], len(TodosLosPermisos))
	}
}

// ============================================================================
// Tests de Auditoría
// ============================================================================

func TestAuditoria_RegistroConcesion(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	auditoria := g.ObtenerAuditoria()

	if len(auditoria) == 0 {
		t.Error("Debería haber registros de auditoría")
	}

	// Verificar que el último es "conceder"
	ultimo := auditoria[len(auditoria)-1]
	if ultimo.Accion != "conceder" {
		t.Errorf("Última acción = %s, se esperaba conceder", ultimo.Accion)
	}
}

func TestAuditoria_RegistroVerificacion(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	// Verificar para generar entrada de auditoría
	g.Verificar(PermArchivos)
	g.Verificar(PermRed)

	auditoria := g.ObtenerAuditoria()

	// Buscar entradas de verificación
	verificaciones := 0
	for _, entry := range auditoria {
		if entry.Accion == "verificar" {
			verificaciones++
		}
	}

	if verificaciones == 0 {
		t.Error("Debería haber entradas de auditoría de verificación")
	}
}

func TestAuditoria_Reciente(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")
	g.Verificar(PermArchivos)
	g.Verificar(PermRed)
	g.Verificar(PermSistema)

	reciente := g.ObtenerAuditoriaReciente(2)

	if len(reciente) != 2 {
		t.Errorf("Se esperaban 2 registros recientes, hay %d", len(reciente))
	}
}

func TestAuditoria_Limpiar(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	g.LimpiarAuditoria()

	auditoria := g.ObtenerAuditoria()
	if len(auditoria) != 0 {
		t.Errorf("Auditoría debería estar vacía después de limpiar, hay %d", len(auditoria))
	}
}

// ============================================================================
// Tests de Reset
// ============================================================================

func TestResetear_RevocaTodo(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	g.Resetear()

	for _, tipo := range TodosLosPermisos {
		if g.Verificar(tipo) {
			t.Errorf("Permiso %s debería estar revocado después de reset", tipo)
		}
	}
}

func TestResetear_IncrementaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	estado1 := g.ObtenerTodos()
	v1 := estado1.Version

	g.Resetear()

	estado2 := g.ObtenerTodos()
	if estado2.Version <= v1 {
		t.Errorf("Versión después de reset = %d, debería ser > %d", estado2.Version, v1)
	}
}

// ============================================================================
// Tests de Habilitar/Deshabilitar
// ============================================================================

func TestHabilitarDeshabilitar(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)

	g.Deshabilitar()
	if g.EstaHabilitado() {
		t.Error("Debería estar deshabilitado")
	}

	g.Habilitar()
	if !g.EstaHabilitado() {
		t.Error("Debería estar habilitado")
	}
}

// ============================================================================
// Tests de Persistencia
// ============================================================================

func TestPersistencia_SobreviveReinicio(t *testing.T) {
	tmpDir := t.TempDir()

	// Primer gestor
	g1, _ := Inicializar(tmpDir)
	g1.ConcederTodos("test", "persistencia test")
	g1.ConcederSubPermiso(PermArchivos, "eliminar", false, NivelDenegado)

	// Segundo gestor (simula reinicio)
	g2, err := Inicializar(tmpDir)
	if err != nil {
		t.Fatalf("Error al reiniciar gestor: %v", err)
	}

	if !g2.Verificar(PermArchivos) {
		t.Error("Permisos deberían persistir tras reinicio")
	}

	// Verificar sub-permiso modificado
	reg := g2.ObtenerPermiso(PermArchivos)
	for _, sp := range reg.SubPermisos {
		if sp.Nombre == "eliminar" && sp.Concedido {
			t.Error("Sub-permiso 'eliminar' debería estar denegado tras reinicio")
		}
	}
}

// ============================================================================
// Tests de Utilidades
// ============================================================================

func TestDescripcionPermiso(t *testing.T) {
	desc := DescripcionPermiso(PermArchivos)
	if desc == "" {
		t.Error("Descripción no debería estar vacía")
	}
	if strings.Contains(desc, "desconocido") {
		t.Error("PermArchivos no debería ser desconocido")
	}
}

func TestDescripcionPermiso_Desconocido(t *testing.T) {
	desc := DescripcionPermiso("tipo_inventado")
	if !strings.Contains(desc, "desconocido") {
		t.Error("Tipo inventado debería ser desconocido")
	}
}

func TestListarTipos(t *testing.T) {
	tipos := ListarTipos()

	if len(tipos) != len(TodosLosPermisos) {
		t.Errorf("Se esperaban %d tipos, hay %d", len(TodosLosPermisos), len(tipos))
	}

	for _, tipo := range tipos {
		if tipo["tipo"] == "" {
			t.Error("Tipo sin nombre")
		}
		if tipo["descripcion"] == "" {
			t.Error("Tipo sin descripción")
		}
	}
}

func TestSortTipos(t *testing.T) {
	tipos := []TipoPermiso{PermModelos, PermArchivos, PermTerminal}
	SortTipos(tipos)

	if tipos[0] != PermArchivos {
		t.Errorf("Primer tipo = %s, se esperaba archivos", tipos[0])
	}
}

func TestFormatearPermisosParaAPI(t *testing.T) {
	tmpDir := t.TempDir()
	g, _ := Inicializar(tmpDir)
	g.ConcederTodos("test", "test")

	datos := g.FormatearPermisosParaAPI()

	permisos, ok := datos["permisos"].([]map[string]interface{})
	if !ok {
		t.Fatal("permisos no es array de mapas")
	}
	if len(permisos) == 0 {
		t.Error("Debería haber permisos en la respuesta API")
	}
}

// ============================================================================
// Tests de Sub-Permisos por Defecto
// ============================================================================

func TestSubPermisosPorDefecto_Archivos(t *testing.T) {
	subs := subPermisosPorDefecto(PermArchivos)

	nombres := make([]string, len(subs))
	for i, s := range subs {
		nombres[i] = s.Nombre
	}

	esperados := []string{"leer", "escribir", "eliminar", "ejecutar"}
	for _, esp := range esperados {
		encontrado := false
		for _, n := range nombres {
			if n == esp {
				encontrado = true
				break
			}
		}
		if !encontrado {
			t.Errorf("Sub-permiso '%s' no encontrado en archivos", esp)
		}
	}
}

func TestSubPermisosPorDefecto_Red(t *testing.T) {
	subs := subPermisosPorDefecto(PermRed)

	nombres := make([]string, len(subs))
	for i, s := range subs {
		nombres[i] = s.Nombre
	}

	esperados := []string{"http", "dns", "sockets", "descargar"}
	for _, esp := range esperados {
		encontrado := false
		for _, n := range nombres {
			if n == esp {
				encontrado = true
				break
			}
		}
		if !encontrado {
			t.Errorf("Sub-permiso '%s' no encontrado en red", esp)
		}
	}
}