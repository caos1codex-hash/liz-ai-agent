package herramientas

import (
	"testing"
)

func TestTiposHerramienta(t *testing.T) {
	cats := []TipoHerramienta{
		TipoSistema, TipoArchivo, TipoBusqueda, TipoCodigo,
		TipoRed, TipoMonitor, TipoInstalacion,
	}
	for _, c := range cats {
		s := c.String()
		if s == "" {
			t.Errorf("TipoHerramienta(%d).String() vacío", c)
		}
	}
}

func TestTipoHerramienta_Desconocido(t *testing.T) {
	unknown := TipoHerramienta(99)
	s := unknown.String()
	if s != "desconocido" {
		t.Errorf("tipo desconocido debería ser 'desconocido', got '%s'", s)
	}
}

func TestResultado_Estructura(t *testing.T) {
	r := Resultado{
		Exito:    true,
		Datos:    map[string]interface{}{"key": "value"},
		Error:    "",
		Metadata: map[string]interface{}{"duracion_ms": 100},
	}

	if !r.Exito {
		t.Error("debería ser exitoso")
	}
	if r.Metadata == nil {
		t.Error("metadata no debería ser nil")
	}
}

func TestResultado_Serializacion(t *testing.T) {
	r := Resultado{
		Exito: true,
		Datos: "test data",
	}

	// Verificar que los campos son accesibles
	_ = r.Exito
	_ = r.Datos
	_ = r.Error
	_ = r.Metadata
}

func TestParametroInfo(t *testing.T) {
	p := ParametroInfo{
		Nombre:     "comando",
		Tipo:       "string",
		Requerido:  true,
		Descripcion: "Comando a ejecutar",
	}

	if p.Nombre != "comando" {
		t.Error("nombre incorrecto")
	}
	if !p.Requerido {
		t.Error("debería ser requerido")
	}
}

func TestInfoHerramienta(t *testing.T) {
	info := InfoHerramienta{
		Nombre:     "terminal",
		Descripcion: "Ejecuta comandos",
		Categoria:   TipoSistema,
		Parametros: []ParametroInfo{
			{Nombre: "comando", Tipo: "string", Requerido: true},
		},
	}

	if info.Nombre != "terminal" {
		t.Error("nombre incorrecto")
	}
	if len(info.Parametros) != 1 {
		t.Error("debería tener 1 parámetro")
	}
	if info.Categoria != TipoSistema {
		t.Error("categoría incorrecta")
	}
}