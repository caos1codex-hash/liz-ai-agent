package config

import (
	"testing"
)

// ============================================================================
// Tests del Validador
// ============================================================================

func TestValidador_NuevoValidadorConfig(t *testing.T) {
	v := NuevoValidadorConfig()
	if v == nil {
		t.Fatal("NuevoValidadorConfig() retornó nil")
	}
	if len(v.reglas) == 0 {
		t.Error("Se esperaban reglas de validación predefinidas")
	}
}

func TestValidador_Validar_ConfigPorDefecto(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()

	err := v.Validar(&cfg)
	if err != nil {
		t.Errorf("Configuración por defecto debería ser válida, error: %v", err)
	}
}

func TestValidador_Validar_PuertoInvalido(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Puerto = 0

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para puerto 0")
	}
}

func TestValidador_Validar_PuertoFueraDeRango(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Puerto = 99999

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para puerto 99999")
	}
}

func TestValidador_Validar_NivelLogInvalido(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Logging.Nivel = "nivel_inventado"

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para nivel de log inválido")
	}
}

func TestValidador_Validar_EstrategiaInvalida(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Contexto.Estrategia = "estrategia_falsa"

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para estrategia inválida")
	}
}

func TestValidador_Validar_VersionInvalida(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Version = "no-es-semver"

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para versión que no sigue semver")
	}
}

func TestValidador_Validar_SinModelos(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Modelos = nil

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error cuando no hay modelos")
	}
}

func TestValidador_Validar_NingunModeloHabilitado(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()

	for i := range cfg.Modelos {
		cfg.Modelos[i].Habilitado = false
	}

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error cuando ningún modelo está habilitado")
	}
}

func TestValidador_Validar_ModeloTemperaturaInvalida(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Modelos[0].Temperatura = 5.0

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para temperatura 5.0 (fuera de rango 0-2)")
	}
}

func TestValidador_Validar_ModeloTopPInvalido(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Modelos[0].TopP = 3.0

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para top_p 3.0 (fuera de rango 0-1)")
	}
}

func TestValidador_Validar_ModeloURLInvalida(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Modelos[0].URL = "ftp://invalido"

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para URL ftp:// (debe ser http o https)")
	}
}

func TestValidador_Validar_MaxPeticionesMinFueraRango(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Seguridad.MaxPeticionesMin = 0

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para max_peticiones_min = 0")
	}
}

func TestValidador_Validar_MaxTokensSesionFueraRango(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Seguridad.MaxTokensSesion = 500

	err := v.Validar(&cfg)
	if err == nil {
		t.Error("Se esperaba error para max_tokens_sesion = 500 (mínimo 1000)")
	}
}

// ============================================================================
// Tests de ValidarCampo
// ============================================================================

func TestValidador_ValidarCampo_PuertoValido(t *testing.T) {
	v := NuevoValidadorConfig()
	err := v.ValidarCampo("puerto", "8080")
	if err != nil {
		t.Errorf("Puerto 8080 debería ser válido: %v", err)
	}
}

func TestValidador_ValidarCampo_PuertoInvalido(t *testing.T) {
	v := NuevoValidadorConfig()
	err := v.ValidarCampo("puerto", "99999")
	if err == nil {
		t.Error("Puerto 99999 debería ser inválido")
	}
}

func TestValidador_ValidarCampo_NivelLogValido(t *testing.T) {
	v := NuevoValidadorConfig()
	niveles := []string{"debug", "info", "advertencia", "error", "silencio"}

	for _, nivel := range niveles {
		err := v.ValidarCampo("logging.nivel", nivel)
		if err != nil {
			t.Errorf("Nivel '%s' debería ser válido: %v", nivel, err)
		}
	}
}

func TestValidador_ValidarCampo_NivelLogInvalido(t *testing.T) {
	v := NuevoValidadorConfig()
	err := v.ValidarCampo("logging.nivel", "no_existe")
	if err == nil {
		t.Error("Nivel 'no_existe' debería ser inválido")
	}
}

func TestValidador_ValidarCampo_CampoSinRegla(t *testing.T) {
	v := NuevoValidadorConfig()
	err := v.ValidarCampo("campo_inventado", "cualquier_valor")
	if err != nil {
		t.Errorf("Campo sin regla debería ser permitido: %v", err)
	}
}

// ============================================================================
// Tests de Esquema
// ============================================================================

func TestValidador_ObtenerEsquema(t *testing.T) {
	v := NuevoValidadorConfig()
	esquema := v.ObtenerEsquema()

	if len(esquema.Campos) == 0 {
		t.Error("El esquema debería tener campos")
	}

	// Verificar que cada campo tenga los datos necesarios
	for _, campo := range esquema.Campos {
		if campo.Ruta == "" {
			t.Error("Campo sin ruta")
		}
		if campo.Tipo == "" {
			t.Errorf("Campo %s sin tipo", campo.Ruta)
		}
	}
}

// ============================================================================
// Tests de Utilidades
// ============================================================================

func TestValidarHost_IPValida(t *testing.T) {
	err := ValidarHost("127.0.0.1")
	if err != nil {
		t.Errorf("127.0.0.1 debería ser válido: %v", err)
	}

	err = ValidarHost("0.0.0.0")
	if err != nil {
		t.Errorf("0.0.0.0 debería ser válido: %v", err)
	}

	err = ValidarHost("::1")
	if err != nil {
		t.Errorf("::1 debería ser válido: %v", err)
	}
}

func TestValidarHost_HostnameValido(t *testing.T) {
	err := ValidarHost("localhost")
	if err != nil {
		t.Errorf("localhost debería ser válido: %v", err)
	}

	err = ValidarHost("mi-servidor.local")
	if err != nil {
		t.Errorf("mi-servidor.local debería ser válido: %v", err)
	}
}

func TestValidarPuerto_Valido(t *testing.T) {
	puertos := []int{1, 80, 443, 8080, 65535}
	for _, p := range puertos {
		if err := ValidarPuerto(p); err != nil {
			t.Errorf("Puerto %d debería ser válido: %v", p, err)
		}
	}
}

func TestValidarPuerto_Invalido(t *testing.T) {
	puertos := []int{0, -1, 65536, 99999}
	for _, p := range puertos {
		if err := ValidarPuerto(p); err == nil {
			t.Errorf("Puerto %d debería ser inválido", p)
		}
	}
}

// ============================================================================
// Tests de Errores de Validación
// ============================================================================

func TestErroresValidacion_Formato(t *testing.T) {
	v := NuevoValidadorConfig()
	cfg := ConfiguracionPorDefecto()
	cfg.Puerto = 0
	cfg.Logging.Nivel = "no_existe"

	err := v.Validar(&cfg)
	if err == nil {
		t.Fatal("Se esperaban errores de validación")
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error de validación no debería ser string vacío")
	}
}