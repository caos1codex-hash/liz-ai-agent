package memoria

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: crea un directorio temporal y un gestor
func nuevoGestorTest(t *testing.T) (*Gestor, string) {
	t.Helper()
	dir := t.TempDir()
	g, err := NuevoGestor(dir)
	if err != nil {
		t.Fatalf("error creando gestor: %v", err)
	}
	return g, dir
}

// ═══════════════════════════════════════════════════════
// SESIONES
// ═══════════════════════════════════════════════════════

func TestNuevaSesion_CreaSesionActiva(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	s, err := g.NuevaSesion("user-1", "proyecto-x")
	if err != nil {
		t.Fatalf("error creando sesión: %v", err)
	}
	if s.ID == "" {
		t.Error("ID no debería estar vacío")
	}
	if !s.Activa {
		t.Error("debería ser activa")
	}
	if s.UsuarioID != "user-1" {
		t.Errorf("UsuarioID debería ser 'user-1', got %s", s.UsuarioID)
	}
	if s.Proyecto != "proyecto-x" {
		t.Errorf("Proyecto debería ser 'proyecto-x', got %s", s.Proyecto)
	}
	if len(s.Mensajes) != 0 {
		t.Errorf("Mensajes debería estar vacío, got %d", len(s.Mensajes))
	}
}

func TestSesionActiva_RetornaSesionCacheada(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	s, _ := g.NuevaSesion("user-1", "")
	if s == nil {
		t.Fatal("sesión no debería ser nil")
	}

	activa := g.Sesiones().SesionActiva("user-1")
	if activa == nil {
		t.Fatal("SesionActiva no debería retornar nil")
	}
	if activa.ID != s.ID {
		t.Errorf("ID debería coincidir: %s vs %s", s.ID, activa.ID)
	}
}

func TestSesionActiva_UsuarioSinSesion_RetornaNil(t *testing.T) {
	g, _ := nuevoGestorTest(t)
	if act := g.Sesiones().SesionActiva("user-fantasma"); act != nil {
		t.Error("debería ser nil para usuario sin sesión")
	}
}

func TestAgregarMensaje_AgregaAPersistencia(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.NuevaSesion("user-1", "")
	msg, err := g.AgregarMensaje("user-1", RolUsuario, "Hola Liz")
	if err != nil {
		t.Fatalf("error agregando mensaje: %v", err)
	}
	if msg.ID == "" {
		t.Error("ID del mensaje no debería estar vacío")
	}
	if msg.Rol != RolUsuario {
		t.Errorf("Rol debería ser 'usuario', got %s", msg.Rol)
	}
	if msg.Contenido != "Hola Liz" {
		t.Errorf("Contenido incorrecto: %s", msg.Contenido)
	}
	if msg.TokenEstim == 0 {
		t.Error("TokenEstim debería ser > 0 para contenido no vacío")
	}

	// Verificar que está en la sesión
	activa := g.Sesiones().SesionActiva("user-1")
	if len(activa.Mensajes) != 1 {
		t.Errorf("debería tener 1 mensaje, got %d", len(activa.Mensajes))
	}
}

func TestAgregarMensaje_SinSesionActiva_RetornaError(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, err := g.AgregarMensaje("user-fantasma", RolUsuario, "Hola")
	if err == nil {
		t.Error("debería retornar error si no hay sesión activa")
	}
}

func TestCerrarSesion_MarcaComoInactiva(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	s, _ := g.NuevaSesion("user-1", "")
	_, _ = g.AgregarMensaje("user-1", RolUsuario, "Hola")
	_, _ = g.AgregarMensaje("user-1", RolAsistente, "Hola, ¿cómo estás?")

	if err := g.CerrarSesion("user-1"); err != nil {
		t.Fatalf("error cerrando sesión: %v", err)
	}

	// Ya no debería haber sesión activa
	if act := g.Sesiones().SesionActiva("user-1"); act != nil {
		t.Error("después de cerrar, SesionActiva debería ser nil")
	}

	// Pero la sesión debería seguir existiendo en disco (cargable por ID)
	sesionCargada, err := g.Sesiones().ObtenerSesion(s.ID)
	if err != nil {
		t.Fatalf("error cargando sesión cerrada: %v", err)
	}
	if sesionCargada.Activa {
		t.Error("sesión cargada debería tener Activa=false")
	}
	if sesionCargada.Fin == "" {
		t.Error("sesión cerrada debería tener Fin no vacío")
	}
	if len(sesionCargada.Mensajes) != 2 {
		t.Errorf("debería tener 2 mensajes persistidos, got %d", len(sesionCargada.Mensajes))
	}
	// Debería tener título autogenerado
	if sesionCargada.Titulo == "" {
		t.Error("debería tener título autogenerado")
	}
}

func TestNuevaSesion_CierraSesionPrevia(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	s1, _ := g.NuevaSesion("user-1", "")
	_, _ = g.AgregarMensaje("user-1", RolUsuario, "msg1")

	s2, _ := g.NuevaSesion("user-1", "")
	if s1.ID == s2.ID {
		t.Error("debería crear una sesión nueva con ID diferente")
	}

	// Sesión vieja debería estar cerrada
	s1Cargada, _ := g.Sesiones().ObtenerSesion(s1.ID)
	if s1Cargada.Activa {
		t.Error("sesión vieja debería estar cerrada")
	}
}

func TestUltimosMensajes_RetornaN(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.NuevaSesion("user-1", "")
	for i := 0; i < 5; i++ {
		_, _ = g.AgregarMensaje("user-1", RolUsuario, "msg")
	}

	ultimos := g.Sesiones().UltimosMensajes("user-1", 3)
	if len(ultimos) != 3 {
		t.Errorf("debería retornar 3, got %d", len(ultimos))
	}
}

func TestEstadisticasSesion(t *testing.T) {
	s := &Sesion{
		Inicio: "2026-01-01T10:00:00Z",
		Fin:    "2026-01-01T10:10:00Z",
		Mensajes: []Mensaje{
			{Rol: RolUsuario, TokenEstim: 50},
			{Rol: RolAsistente, TokenEstim: 100},
			{Rol: RolUsuario, TokenEstim: 30},
		},
		TokensTotales: 180,
	}
	stats := s.Estadisticas()
	if stats.TotalMensajes != 3 {
		t.Errorf("TotalMensajes debería ser 3, got %d", stats.TotalMensajes)
	}
	if stats.MensajesUsuario != 2 {
		t.Errorf("MensajesUsuario debería ser 2, got %d", stats.MensajesUsuario)
	}
	if stats.MensajesAsistente != 1 {
		t.Errorf("MensajesAsistente debería ser 1, got %d", stats.MensajesAsistente)
	}
	if stats.TokensTotales != 180 {
		t.Errorf("TokensTotales debería ser 180, got %d", stats.TokensTotales)
	}
	if stats.DuracionSegundos != 600 {
		t.Errorf("DuracionSegundos debería ser 600, got %d", stats.DuracionSegundos)
	}
}

// ═══════════════════════════════════════════════════════
// HECHOS
// ═══════════════════════════════════════════════════════

func TestAgregarHecho_Persiste(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	h, err := g.AgregarHecho("user-1", "usuario", "prefiere_lenguaje", "Go", 0.9, "sesion-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if h.ID == "" {
		t.Error("ID no debería estar vacío")
	}
	if h.Confianza != 0.9 {
		t.Errorf("Confianza debería ser 0.9, got %f", h.Confianza)
	}

	hechos, _ := g.Hechos().HechosActivos("user-1")
	if len(hechos) != 1 {
		t.Errorf("debería tener 1 hecho, got %d", len(hechos))
	}
}

func TestAgregarHecho_ConfianzaFueraDeRango_SeClampea(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	h, _ := g.AgregarHecho("user-1", "u", "p", "o", 5.0, "")
	if h.Confianza != 1.0 {
		t.Errorf("debería clampear a 1.0, got %f", h.Confianza)
	}

	h2, _ := g.AgregarHecho("user-1", "u", "p2", "o2", -0.5, "")
	if h2.Confianza != 0 {
		t.Errorf("debería clampear a 0, got %f", h2.Confianza)
	}
}

func TestAgregarHecho_CamposVacios_RetornaError(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	if _, err := g.AgregarHecho("user-1", "", "p", "o", 0.5, ""); err == nil {
		t.Error("debería fallar con sujeto vacío")
	}
	if _, err := g.AgregarHecho("user-1", "s", "", "o", 0.5, ""); err == nil {
		t.Error("debería fallar con predicado vacío")
	}
	if _, err := g.AgregarHecho("user-1", "s", "p", "", 0.5, ""); err == nil {
		t.Error("debería fallar con objeto vacío")
	}
}

func TestAgregarHecho_Conflicto_MarcaViejoComoObsoleto(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.AgregarHecho("user-1", "usuario", "prefiere_lenguaje", "Python", 0.7, "sesion-1")
	// Nuevo hecho conflicto: mismo sujeto+predicado, objeto diferente
	h2, _ := g.AgregarHecho("user-1", "usuario", "prefiere_lenguaje", "Go", 0.95, "sesion-2")

	hechos, _ := g.Hechos().HechosActivos("user-1")
	if len(hechos) != 1 {
		t.Errorf("debería tener 1 hecho activo (el nuevo), got %d", len(hechos))
	}
	if hechos[0].ID != h2.ID {
		t.Error("el hecho activo debería ser el nuevo")
	}
	if hechos[0].Objeto != "Go" {
		t.Errorf("Objeto debería ser 'Go', got %s", hechos[0].Objeto)
	}

	// El viejo debería estar marcado obsoleto
	stats, _ := g.Hechos().Estadisticas("user-1")
	if stats.HechosObsoletos != 1 {
		t.Errorf("debería tener 1 hecho obsoleto, got %d", stats.HechosObsoletos)
	}
}

func TestAgregarHecho_Duplicado_ActualizaConfianza(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.AgregarHecho("user-1", "usuario", "prefiere", "Go", 0.7, "sesion-1")
	h2, _ := g.AgregarHecho("user-1", "usuario", "prefiere", "Go", 0.9, "sesion-2")

	hechos, _ := g.Hechos().HechosActivos("user-1")
	if len(hechos) != 1 {
		t.Errorf("no debería duplicar, got %d hechos", len(hechos))
	}
	// Confianza debería ser promedio: (0.7 + 0.9) / 2 = 0.8
	if hechos[0].Confianza < 0.79 || hechos[0].Confianza > 0.81 {
		t.Errorf("confianza debería ser ~0.8, got %f", hechos[0].Confianza)
	}
	if hechos[0].ID != h2.ID {
		t.Error("mismo ID (no creó nuevo)")
	}
}

func TestBuscarHechos_FiltraPorSujetoYPredicado(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.AgregarHecho("user-1", "usuario", "prefiere_lenguaje", "Go", 0.9, "")
	_, _ = g.AgregarHecho("user-1", "usuario", "sistema_operativo", "Arch Linux", 0.8, "")
	_, _ = g.AgregarHecho("user-1", "proyecto_liz", "usar_api", "NVIDIA", 0.95, "")

	// Filtrar por sujeto
	res, _ := g.Hechos().BuscarHechos("user-1", "usuario", "")
	if len(res) != 2 {
		t.Errorf("buscar por sujeto=usuario debería retornar 2, got %d", len(res))
	}

	// Filtrar por sujeto + predicado
	res, _ = g.Hechos().BuscarHechos("user-1", "usuario", "prefiere_lenguaje")
	if len(res) != 1 {
		t.Errorf("buscar específico debería retornar 1, got %d", len(res))
	}
	if res[0].Objeto != "Go" {
		t.Errorf("objeto debería ser 'Go', got %s", res[0].Objeto)
	}
}

func TestEliminarHecho_MarcaObsoleto(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	h, _ := g.AgregarHecho("user-1", "u", "p", "o", 0.5, "")

	if err := g.Hechos().EliminarHecho("user-1", h.ID); err != nil {
		t.Fatalf("error eliminando: %v", err)
	}

	hechos, _ := g.Hechos().HechosActivos("user-1")
	if len(hechos) != 0 {
		t.Errorf("no debería haber hechos activos, got %d", len(hechos))
	}
}

func TestFormatoContexto_FormatoCorrecto(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.AgregarHecho("user-1", "usuario", "prefiere_lenguaje", "Go", 0.95, "")
	_, _ = g.AgregarHecho("user-1", "proyecto_liz", "usar_api", "NVIDIA", 0.90, "")

	ctx, err := g.Hechos().FormatoContexto("user-1", 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(ctx, "Memoria del usuario") {
		t.Error("debería tener header 'Memoria del usuario'")
	}
	if !strings.Contains(ctx, "usuario prefiere_lenguaje: Go") {
		t.Error("debería incluir el primer hecho")
	}
	if !strings.Contains(ctx, "proyecto_liz usar_api: NVIDIA") {
		t.Error("debería incluir el segundo hecho")
	}
	if !strings.Contains(ctx, "0.95") {
		t.Error("debería incluir la confianza")
	}
}

func TestFormatoContexto_SinHechos_RetornaVacio(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	ctx, err := g.Hechos().FormatoContexto("user-fantasma", 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if ctx != "" {
		t.Errorf("debería ser vacío, got %s", ctx)
	}
}

// ═══════════════════════════════════════════════════════
// GESTOR UNIFICADO
// ═══════════════════════════════════════════════════════

func TestGestor_ContextoParaLLM_SinMemoria_Vacio(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	ctx, err := g.ContextoParaLLM("user-fantasma", 10, 20)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if ctx != "" {
		t.Errorf("sin memoria debería retornar '', got %s", ctx)
	}
}

func TestGestor_ContextoParaLLM_ConHechosYMensajes(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.NuevaSesion("user-1", "")
	_, _ = g.AgregarMensaje("user-1", RolUsuario, "Hola")
	_, _ = g.AgregarMensaje("user-1", RolAsistente, "Hola, ¿qué necesitas?")
	_, _ = g.AgregarHecho("user-1", "usuario", "prefiere", "Go", 0.9, "")

	ctx, err := g.ContextoParaLLM("user-1", 10, 20)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if ctx == "" {
		t.Fatal("ctx no debería estar vacío")
	}
	if !strings.Contains(ctx, "Memoria del usuario") {
		t.Error("debería incluir memoria semántica (hechos)")
	}
	if !strings.Contains(ctx, "Contexto de la conversación reciente") {
		t.Error("debería incluir memoria episódica (mensajes)")
	}
	if !strings.Contains(ctx, "Hola") {
		t.Error("debería incluir contenido del mensaje")
	}
	if !strings.Contains(ctx, "usuario prefiere: Go") {
		t.Error("debería incluir el hecho")
	}
}

func TestGestor_Estadisticas(t *testing.T) {
	g, _ := nuevoGestorTest(t)

	_, _ = g.NuevaSesion("user-1", "")
	_, _ = g.AgregarHecho("user-1", "u", "p1", "o1", 0.5, "")
	_, _ = g.AgregarHecho("user-1", "u", "p2", "o2", 0.7, "")

	stats := g.Estadisticas("user-1")
	if stats.SesionesActivas != 1 {
		t.Errorf("SesionesActivas debería ser 1, got %d", stats.SesionesActivas)
	}
	if stats.HechosActivos != 2 {
		t.Errorf("HechosActivos debería ser 2, got %d", stats.HechosActivos)
	}
}

// ═══════════════════════════════════════════════════════
// PERSISTENCIA
// ═══════════════════════════════════════════════════════

func TestPersistencia_SesionSobreviveReload(t *testing.T) {
	dir := t.TempDir()
	g1, _ := NuevoGestor(dir)
	s, _ := g1.NuevaSesion("user-1", "")
	_, _ = g1.AgregarMensaje("user-1", RolUsuario, "Hola persistente")

	// Crear nuevo gestor (simula reinicio)
	g2, _ := NuevoGestor(dir)

	// La sesión activa debería cargarse desde disco
	activa := g2.Sesiones().SesionActiva("user-1")
	if activa == nil {
		t.Fatal("sesión activa debería cargarse desde disco tras reinicio")
	}
	if activa.ID != s.ID {
		t.Errorf("ID debería coincidir: %s vs %s", s.ID, activa.ID)
	}
	if len(activa.Mensajes) != 1 {
		t.Errorf("debería tener 1 mensaje persistido, got %d", len(activa.Mensajes))
	}
	if activa.Mensajes[0].Contenido != "Hola persistente" {
		t.Errorf("mensaje incorrecto: %s", activa.Mensajes[0].Contenido)
	}
}

func TestPersistencia_HechosSobrevivenReload(t *testing.T) {
	dir := t.TempDir()
	g1, _ := NuevoGestor(dir)
	_, _ = g1.AgregarHecho("user-1", "usuario", "prefiere", "Go", 0.9, "")

	// Reload
	g2, _ := NuevoGestor(dir)
	hechos, err := g2.Hechos().HechosActivos("user-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(hechos) != 1 {
		t.Errorf("debería tener 1 hecho persistido, got %d", len(hechos))
	}
	if hechos[0].Objeto != "Go" {
		t.Errorf("objeto incorrecto: %s", hechos[0].Objeto)
	}
}

func TestEstructuraDirectorios_CreadaCorrectamente(t *testing.T) {
	dir := t.TempDir()
	_, _ = NuevoGestor(dir)

	for _, sub := range []string{"memoria", "memoria/sesiones", "memoria/hechos", "memoria/resumenes"} {
		path := filepath.Join(dir, sub)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Errorf("directorio %s debería existir", path)
		}
	}
}

func TestGenerarUUID_Unico(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generarUUID()
		if ids[id] {
			t.Errorf("UUID duplicado: %s", id)
		}
		ids[id] = true
	}
}

func TestEstimarTokens(t *testing.T) {
	casos := []struct {
		texto    string
		esperado int
	}{
		{"", 0},
		{"abc", 0},       // 3 chars / 4 = 0
		{"abcd", 1},      // 4 chars / 4 = 1
		{"abcdefgh", 2},  // 8 chars / 4 = 2
	}
	for _, c := range casos {
		got := estimarTokens(c.texto)
		if got != c.esperado {
			t.Errorf("estimarTokens(%q) = %d, esperaba %d", c.texto, got, c.esperado)
		}
	}
}
