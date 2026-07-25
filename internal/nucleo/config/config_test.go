package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Tests de Carga de Configuración
// ============================================================================

func TestConfiguracionPorDefecto_ValoresEsperados(t *testing.T) {
	cfg := ConfiguracionPorDefecto()

	if cfg.Puerto != 8080 {
		t.Errorf("Puerto por defecto = %d, se esperaba 8080", cfg.Puerto)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host por defecto = %s, se esperaba 0.0.0.0", cfg.Host)
	}
	if cfg.Nombre != "Liz" {
		t.Errorf("Nombre por defecto = %s, se esperaba Liz", cfg.Nombre)
	}
	if cfg.Version != "0.1.0" {
		t.Errorf("Version por defecto = %s, se esperaba 0.1.0", cfg.Version)
	}
	if cfg.DirectorioBase != "~/.liz" {
		t.Errorf("DirectorioBase por defecto = %s, se esperaba ~/.liz", cfg.DirectorioBase)
	}
}

func TestConfiguracionPorDefecto_Modelos(t *testing.T) {
	cfg := ConfiguracionPorDefecto()

	if len(cfg.Modelos) == 0 {
		t.Fatal("Se esperaba al menos un modelo por defecto")
	}

	// Verificar que al menos uno está habilitado
	habilitado := false
	for _, m := range cfg.Modelos {
		if m.Habilitado {
			habilitado = true
		}
		if m.Nombre == "" {
			t.Error("Modelo sin nombre")
		}
		if m.Proveedor == "" {
			t.Errorf("Modelo %s sin proveedor", m.Nombre)
		}
		if m.Temperatura < 0 || m.Temperatura > 2.0 {
			t.Errorf("Modelo %s: temperatura fuera de rango: %f", m.Nombre, m.Temperatura)
		}
		if m.TopP < 0 || m.TopP > 1.0 {
			t.Errorf("Modelo %s: top_p fuera de rango: %f", m.Nombre, m.TopP)
		}
	}

	if !habilitado {
		t.Error("Se esperaba al menos un modelo habilitado por defecto")
	}
}

func TestConfiguracionPorDefecto_Seguridad(t *testing.T) {
	cfg := ConfiguracionPorDefecto()

	if cfg.Seguridad.MaxPeticionesMin != 60 {
		t.Errorf("MaxPeticionesMin = %d, se esperaba 60", cfg.Seguridad.MaxPeticionesMin)
	}
	if cfg.Seguridad.MaxTokensSesion != 100000 {
		t.Errorf("MaxTokensSesion = %d, se esperaba 100000", cfg.Seguridad.MaxTokensSesion)
	}
}

func TestConfiguracionPorDefecto_Logging(t *testing.T) {
	cfg := ConfiguracionPorDefecto()

	if cfg.Logging.Nivel != "info" {
		t.Errorf("Logging.Nivel = %s, se esperaba info", cfg.Logging.Nivel)
	}
	if cfg.Logging.MaxMB != 50 {
		t.Errorf("Logging.MaxMB = %d, se esperaba 50", cfg.Logging.MaxMB)
	}
	if cfg.Logging.MaxArchivos != 5 {
		t.Errorf("Logging.MaxArchivos = %d, se esperaba 5", cfg.Logging.MaxArchivos)
	}
}

func TestConfiguracionPorDefecto_Contexto(t *testing.T) {
	cfg := ConfiguracionPorDefecto()

	if cfg.Contexto.Estrategia != "bajo_demanda" {
		t.Errorf("Contexto.Estrategia = %s, se esperaba bajo_demanda", cfg.Contexto.Estrategia)
	}
	if cfg.Contexto.CatalogoHabilitado != true {
		t.Error("Se esperaba CatalogoHabilitado = true")
	}
}

// ============================================================================
// Tests de Carga desde Archivo
// ============================================================================

func TestCargar_SinArchivo(t *testing.T) {
	cfg, err := Cargar("/tmp/archivo_que_no_existe_liz.yaml")
	if err != nil {
		t.Fatalf("Cargar sin archivo debería retornar defaults, no error: %v", err)
	}
	if cfg.Puerto != 8080 {
		t.Errorf("Puerto = %d, se esperaba 8080 (default)", cfg.Puerto)
	}
}

func TestCargar_ConWrapperLiz(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
liz:
  puerto: 9090
  host: 127.0.0.1
  nombre: "LizTest"
  version: "2.0.0"
  seguridad:
    max_peticiones_min: 120
  logging:
    nivel: "debug"
`

	if err := os.WriteFile(ruta, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Cargar(ruta)
	if err != nil {
		t.Fatalf("Error al cargar YAML con wrapper liz: %v", err)
	}

	if cfg.Puerto != 9090 {
		t.Errorf("Puerto = %d, se esperaba 9090", cfg.Puerto)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %s, se esperaba 127.0.0.1", cfg.Host)
	}
	if cfg.Nombre != "LizTest" {
		t.Errorf("Nombre = %s, se esperaba LizTest", cfg.Nombre)
	}
	if cfg.Seguridad.MaxPeticionesMin != 120 {
		t.Errorf("MaxPeticionesMin = %d, se esperaba 120", cfg.Seguridad.MaxPeticionesMin)
	}
	if cfg.Logging.Nivel != "debug" {
		t.Errorf("Logging.Nivel = %s, se esperaba debug", cfg.Logging.Nivel)
	}
}

func TestCargar_SinWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
puerto: 3000
host: localhost
nombre: "LizDirecto"
`

	if err := os.WriteFile(ruta, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Cargar(ruta)
	if err != nil {
		t.Fatalf("Error al cargar YAML directo: %v", err)
	}

	if cfg.Puerto != 3000 {
		t.Errorf("Puerto = %d, se esperaba 3000", cfg.Puerto)
	}
	if cfg.Nombre != "LizDirecto" {
		t.Errorf("Nombre = %s, se esperaba LizDirecto", cfg.Nombre)
	}
}

// ============================================================================
// Tests de Override de Variables de Entorno
// ============================================================================

func TestAplicarOverridesEnv_Puerto(t *testing.T) {
	t.Setenv("LIZ_PUERTO", "4444")

	cfg := ConfiguracionPorDefecto()
	aplicarOverridesEnv(&cfg)

	if cfg.Puerto != 4444 {
		t.Errorf("Puerto con override = %d, se esperaba 4444", cfg.Puerto)
	}
}

func TestAplicarOverridesEnv_Host(t *testing.T) {
	t.Setenv("LIZ_HOST", "192.168.1.100")

	cfg := ConfiguracionPorDefecto()
	aplicarOverridesEnv(&cfg)

	if cfg.Host != "192.168.1.100" {
		t.Errorf("Host con override = %s, se esperaba 192.168.1.100", cfg.Host)
	}
}

func TestAplicarOverridesEnv_Nombre(t *testing.T) {
	t.Setenv("LIZ_NOMBRE", "LizProd")

	cfg := ConfiguracionPorDefecto()
	aplicarOverridesEnv(&cfg)

	if cfg.Nombre != "LizProd" {
		t.Errorf("Nombre con override = %s, se esperaba LizProd", cfg.Nombre)
	}
}

func TestAplicarOverridesEnv_NIVEL_LOG(t *testing.T) {
	t.Setenv("LIZ_NIVEL_LOG", "debug")

	cfg := ConfiguracionPorDefecto()
	aplicarOverridesEnv(&cfg)

	if cfg.Logging.Nivel != "debug" {
		t.Errorf("Logging.Nivel con override = %s, se esperaba debug", cfg.Logging.Nivel)
	}
}

// ============================================================================
// Tests del Gestor
// ============================================================================

func TestGestor_Obtener(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `liz:
  puerto: 8080
  host: 0.0.0.0
  nombre: "Liz"
  version: "0.1.0"
`
	os.WriteFile(ruta, []byte(yamlContent), 0644)

	g, err := Inicializar(ruta)
	if err != nil {
		t.Fatalf("Error al inicializar gestor: %v", err)
	}

	cfg := g.Obtener()
	if cfg.Puerto != 8080 {
		t.Errorf("Puerto = %d, se esperaba 8080", cfg.Puerto)
	}
	if cfg.Nombre != "Liz" {
		t.Errorf("Nombre = %s, se esperaba Liz", cfg.Nombre)
	}
}

func TestGestor_ObtenerPuerto(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(ruta, []byte("liz:\n  puerto: 9999\n  nombre: T\n  version: 1.0.0"), 0644)

	g, _ := Inicializar(ruta)

	if g.ObtenerPuerto() != 9999 {
		t.Errorf("ObtenerPuerto() = %d, se esperaba 9999", g.ObtenerPuerto())
	}
}

func TestGestor_Establecer(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(ruta, []byte("liz:\n  puerto: 8080\n  nombre: T\n  version: 1.0.0"), 0644)

	g, _ := Inicializar(ruta)

	// Cambiar puerto
	err := g.Establecer("puerto", "7777")
	if err != nil {
		t.Fatalf("Establecer falló: %v", err)
	}

	if g.ObtenerPuerto() != 7777 {
		t.Errorf("Puerto después de establecer = %d, se esperaba 7777", g.ObtenerPuerto())
	}

	// Verificar que se registró el cambio
	cambios := g.ObtenerCambios()
	if len(cambios) == 0 {
		t.Error("Se esperaba al menos un cambio registrado")
	}
}

func TestGestor_Establecer_Validacion(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(ruta, []byte("liz:\n  puerto: 8080\n  nombre: T\n  version: 1.0.0"), 0644)

	g, _ := Inicializar(ruta)

	// Intentar establecer puerto inválido (fuera de rango)
	err := g.Establecer("puerto", "99999")
	if err == nil {
		t.Error("Se esperaba error al establecer puerto 99999 (fuera de rango 1-65535)")
	}
}

func TestGestor_Establecer_NivelLogInvalido(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(ruta, []byte("liz:\n  puerto: 8080\n  nombre: T\n  version: 1.0.0"), 0644)

	g, _ := Inicializar(ruta)

	err := g.Establecer("logging.nivel", "nivel_inexistente")
	if err == nil {
		t.Error("Se esperaba error al establecer nivel de log inválido")
	}
}

func TestGestor_EstablecerMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(ruta, []byte("liz:\n  puerto: 8080\n  nombre: T\n  version: 1.0.0"), 0644)

	g, _ := Inicializar(ruta)

	err := g.EstablecerMultiple(map[string]string{
		"puerto":         "5555",
		"logging.nivel":  "debug",
	})
	if err != nil {
		t.Fatalf("EstablecerMultiple falló: %v", err)
	}

	if g.ObtenerPuerto() != 5555 {
		t.Errorf("Puerto = %d, se esperaba 5555", g.ObtenerPuerto())
	}
}

func TestGestor_Guardar(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(ruta, []byte("liz:\n  puerto: 8080\n  nombre: T\n  version: 1.0.0"), 0644)

	g, _ := Inicializar(ruta)

	g.Establecer("puerto", "3333")
	err := g.Guardar()
	if err != nil {
		t.Fatalf("Guardar falló: %v", err)
	}

	// Recargar y verificar
	g2, err := Inicializar(ruta)
	if err != nil {
		t.Fatalf("Error al reinicializar: %v", err)
	}

	if g2.ObtenerPuerto() != 3333 {
		t.Errorf("Puerto después de recarga = %d, se esperaba 3333", g2.ObtenerPuerto())
	}
}

// ============================================================================
// Tests de Expandir Home
// ============================================================================

func TestExpandirHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	resultado := expandirHome("~/liz")
	if resultado != filepath.Join(home, "liz") {
		t.Errorf("expandirHome('~/liz') = %s, se esperaba %s", resultado, filepath.Join(home, "liz"))
	}

	resultado2 := expandirHome("/abs/path")
	if resultado2 != "/abs/path" {
		t.Errorf("expandirHome('/abs/path') = %s, se esperaba /abs/path", resultado2)
	}
}

// ============================================================================
// Tests de CampoStruct
// ============================================================================

func TestCampoStruct_SnakeCase(t *testing.T) {
	resultado := campoStruct("max_tokens")
	if resultado != "MaxTokens" {
		t.Errorf("campoStruct('max_tokens') = %s, se esperaba MaxTokens", resultado)
	}
}

func TestCampoStruct_KebabCase(t *testing.T) {
	resultado := campoStruct("max-tokens")
	if resultado != "MaxTokens" {
		t.Errorf("campoStruct('max-tokens') = %s, se esperaba MaxTokens", resultado)
	}
}

func TestCampoStruct_Simple(t *testing.T) {
	resultado := campoStruct("puerto")
	if resultado != "Puerto" {
		t.Errorf("campoStruct('puerto') = %s, se esperaba Puerto", resultado)
	}
}

// ============================================================================
// Tests de Asegurar Directorios
// ============================================================================

func TestAsegurarDirectorios(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Configuracion{DirectorioBase: tmpDir}

	err := cfg.AsegurarDirectorios()
	if err != nil {
		t.Fatalf("AsegurarDirectorios falló: %v", err)
	}

	dirs := []string{
		tmpDir,
		filepath.Join(tmpDir, "contexto"),
		filepath.Join(tmpDir, "herramientas"),
		filepath.Join(tmpDir, "logs"),
		filepath.Join(tmpDir, "conversaciones"),
		filepath.Join(tmpDir, "permisos"),
	}

	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("Directorio %s no existe: %v", dir, err)
		}
		if !info.IsDir() {
			t.Errorf("%s no es un directorio", dir)
		}
	}
}

// ============================================================================
// Tests de Recarga
// ============================================================================

func TestGestor_Recargar(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")

	// Config inicial
	os.WriteFile(ruta, []byte("liz:\n  puerto: 8080\n  nombre: T\n  version: 1.0.0"), 0644)
	g, _ := Inicializar(ruta)

	// Modificar archivo directamente
	nuevoYAML := "liz:\n  puerto: 6000\n  nombre: T\n  version: 1.0.0\n  logging:\n    nivel: debug"
	os.WriteFile(ruta, []byte(nuevoYAML), 0644)

	cambios, err := g.Recargar()
	if err != nil {
		t.Fatalf("Recargar falló: %v", err)
	}

	if g.ObtenerPuerto() != 6000 {
		t.Errorf("Puerto después de recarga = %d, se esperaba 6000", g.ObtenerPuerto())
	}

	if len(cambios) == 0 {
		t.Error("Se esperaban cambios detectados tras recarga")
	}
}

// ============================================================================
// Tests de Modelo Habilitado
// ============================================================================

func TestGestor_ObtenerModeloHabilitado(t *testing.T) {
	tmpDir := t.TempDir()
	ruta := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(ruta, []byte("liz:\n  puerto: 8080\n  nombre: T\n  version: 1.0.0"), 0644)

	g, _ := Inicializar(ruta)

	modelo := g.ObtenerModeloHabilitado()
	if modelo == nil {
		t.Fatal("Se esperaba al menos un modelo habilitado")
	}
	if modelo.Nombre == "" {
		t.Error("Modelo habilitado sin nombre")
	}
}