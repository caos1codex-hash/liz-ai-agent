package herramientas

import (
	"context"
	"errors"
	"testing"
)

// herrPrueba es una implementación mínima para testear la interfaz.
type herrPrueba struct {
	nombre string
}

var _ Herramienta = (*herrPrueba)(nil)

func (h *herrPrueba) Nombre() string          { return h.nombre }
func (h *herrPrueba) Descripcion() string     { return "herramienta de prueba" }
func (h *herrPrueba) Parametros() []Parametro { return nil }
func (h *herrPrueba) Ejecutar(ctx context.Context, p map[string]interface{}) (Resultado, error) {
	return Resultado{Exito: true, Datos: "ok"}, nil
}
func (h *herrPrueba) Validar() error { return nil }

// TestNombreVálido verifica que la interfaz se satisface y el método Nombre funciona.
func TestInterfaceSatisfecha(t *testing.T) {
	h := &herrPrueba{nombre: "prueba"}
	if h.Nombre() != "prueba" {
		t.Errorf("Nombre() = %q, esperaba %q", h.Nombre(), "prueba")
	}
	if h.Descripcion() != "herramienta de prueba" {
		t.Errorf("Descripcion() inesperada")
	}
	if err := h.Validar(); err != nil {
		t.Errorf("Validar() retornó error: %v", err)
	}
	res, err := h.Ejecutar(context.Background(), nil)
	if err != nil || !res.Exito {
		t.Errorf("Ejecutar falló: %v / %+v", err, res)
	}
}

// TestObtenerString_Requerido verifica el error de parámetro requerido.
func TestObtenerString_Requerido(t *testing.T) {
	p := Parametro{Nombre: "ruta", Requerido: true}
	_, err := ObtenerString(map[string]interface{}{}, p)
	var errReq *ErrParametroRequerido
	if !errors.As(err, &errReq) {
		t.Errorf("esperaba ErrParametroRequerido, obtuve %v", err)
	}
	if errReq.Nombre != "ruta" {
		t.Errorf("Nombre del error = %q", errReq.Nombre)
	}
}

// TestObtenerString_Default aplica el default cuando el parámetro falta.
func TestObtenerString_Default(t *testing.T) {
	p := Parametro{Nombre: "formato", Default: "json"}
	val, err := ObtenerString(map[string]interface{}{}, p)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if val != "json" {
		t.Errorf("val = %q, esperaba json", val)
	}
}

// TestObtenerString_Opciones valida la lista de opciones.
func TestObtenerString_Opciones(t *testing.T) {
	p := Parametro{Nombre: "modo", Opciones: []string{"rapido", "seguro", "debug"}}
	// Valor válido
	val, err := ObtenerString(map[string]interface{}{"modo": "rapido"}, p)
	if err != nil || val != "rapido" {
		t.Errorf("caso válido falló: %v / %q", err, val)
	}
	// Valor inválido
	_, err = ObtenerString(map[string]interface{}{"modo": "peligroso"}, p)
	var errOp *ErrOpcionInvalida
	if !errors.As(err, &errOp) {
		t.Errorf("esperaba ErrOpcionInvalida, obtuve %v", err)
	}
}

// TestObtenerString_Coercion intenta coercion desde float64 (caso JSON).
func TestObtenerString_Coercion(t *testing.T) {
	p := Parametro{Nombre: "puerto"}
	val, err := ObtenerString(map[string]interface{}{"puerto": float64(8080)}, p)
	if err != nil {
		t.Fatalf("coercion falló: %v", err)
	}
	if val != "8080" {
		t.Errorf("val = %q, esperaba 8080", val)
	}
}

// TestObtenerString_Longitud valida Min/Max como longitud de string.
func TestObtenerString_Longitud(t *testing.T) {
	min, max := 2.0, 5.0
	p := Parametro{Nombre: "codigo", Min: &min, Max: &max}
	// Muy corto
	_, err := ObtenerString(map[string]interface{}{"codigo": "x"}, p)
	var errRango *ErrValorFueraDeRango
	if !errors.As(err, &errRango) {
		t.Errorf("esperaba ErrValorFueraDeRango para corto, obtuve %v", err)
	}
	// Muy largo
	_, err = ObtenerString(map[string]interface{}{"codigo": "abcdef"}, p)
	if !errors.As(err, &errRango) {
		t.Errorf("esperaba ErrValorFueraDeRango para largo, obtuve %v", err)
	}
	// OK
	val, err := ObtenerString(map[string]interface{}{"codigo": "ok"}, p)
	if err != nil || val != "ok" {
		t.Errorf("caso válido falló: %v / %q", err, val)
	}
}

// TestObtenerInt_AceptaVariosTipos verifica coercion desde JSON/strings.
func TestObtenerInt_AceptaVariosTipos(t *testing.T) {
	p := Parametro{Nombre: "n"}
	casos := []struct {
		input    interface{}
		esperado int
	}{
		{int(42), 42},
		{int64(42), 42},
		{float64(42), 42},
		{"42", 42},
	}
	for _, c := range casos {
		val, err := ObtenerInt(map[string]interface{}{"n": c.input}, p)
		if err != nil {
			t.Errorf("input %v (%T): error %v", c.input, c.input, err)
			continue
		}
		if val != c.esperado {
			t.Errorf("input %v: val=%d, esperaba %d", c.input, val, c.esperado)
		}
	}
}

// TestObtenerInt_Rango valida Min/Max como valor numérico.
func TestObtenerInt_Rango(t *testing.T) {
	min, max := 1.0, 100.0
	p := Parametro{Nombre: "n", Min: &min, Max: &max}
	if _, err := ObtenerInt(map[string]interface{}{"n": 0}, p); err == nil {
		t.Error("0 debería estar fuera de rango")
	}
	if _, err := ObtenerInt(map[string]interface{}{"n": 101}, p); err == nil {
		t.Error("101 debería estar fuera de rango")
	}
	if _, err := ObtenerInt(map[string]interface{}{"n": 50}, p); err != nil {
		t.Errorf("50 debería estar en rango: %v", err)
	}
}

// TestObtenerBool_AceptaStrings verifica "true"/"false"/"1"/"0"/"yes"/"no".
func TestObtenerBool_AceptaStrings(t *testing.T) {
	p := Parametro{Nombre: "flag"}
	casos := map[string]bool{
		"true": true, "True": true, "TRUE": true,
		"1": true, "yes": true, "si": true, "sí": true,
		"false": false, "False": false, "FALSE": false,
		"0": false, "no": false,
	}
	for input, esperado := range casos {
		val, err := ObtenerBool(map[string]interface{}{"flag": input}, p)
		if err != nil {
			t.Errorf("input %q: error %v", input, err)
			continue
		}
		if val != esperado {
			t.Errorf("input %q: val=%v, esperaba %v", input, val, esperado)
		}
	}
}

// TestObtenerArrayString_AceptaMixed verifica []string y []interface{}.
func TestObtenerArrayString_AceptaMixed(t *testing.T) {
	p := Parametro{Nombre: "archivos"}
	// []string
	val, err := ObtenerArrayString(map[string]interface{}{
		"archivos": []string{"a.txt", "b.txt"},
	}, p)
	if err != nil || len(val) != 2 {
		t.Errorf("[]string falló: %v / %v", err, val)
	}
	// []interface{}
	val, err = ObtenerArrayString(map[string]interface{}{
		"archivos": []interface{}{"x.txt", 42, true},
	}, p)
	if err != nil || len(val) != 3 {
		t.Errorf("[]interface{} falló: %v / %v", err, val)
	}
	if val[1] != "42" {
		t.Errorf("coercion int→string falló: %q", val[1])
	}
}

// TestObtenerFloat_AceptaInt verifica coercion desde int.
func TestObtenerFloat_AceptaInt(t *testing.T) {
	p := Parametro{Nombre: "f"}
	val, err := ObtenerFloat(map[string]interface{}{"f": 42}, p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if val != 42.0 {
		t.Errorf("val = %v, esperaba 42.0", val)
	}
}

// TestValidarNombre verifica las reglas de nombres de herramienta.
func TestValidarNombre(t *testing.T) {
	casos := []struct {
		nombre string
		ok     bool
	}{
		{"terminal", true},
		{"buscador_archivos", true},
		{"h2", true},
		{"a", false}, // muy corto
		{"x", false}, // muy corto
		{"CON_MAYUSCULAS", false},
		{"con-guion", false},
		{"con espacios", false},
		{"con$dolar", false},
		{string(make([]byte, 65)), false}, // muy largo (nul bytes, también inválido)
	}
	for _, c := range casos {
		err := ValidarNombre(c.nombre)
		if c.ok && err != nil {
			t.Errorf("nombre %q debería ser válido, error: %v", c.nombre, err)
		}
		if !c.ok && err == nil {
			t.Errorf("nombre %q debería ser inválido", c.nombre)
		}
	}
}

// TestErrores.tipos verifica que los errores implementan la interfaz error.
func TestErroresTipos(t *testing.T) {
	errs := []error{
		&ErrParametroRequerido{Nombre: "x"},
		&ErrTipoParametro{Nombre: "x", Esperado: "int", Obtenido: "string"},
		&ErrValorFueraDeRango{Nombre: "x", Min: "0", Max: "10"},
		&ErrOpcionInvalida{Nombre: "x", Valor: "z", Opciones: []string{"a", "b"}},
		&ErrHerramientaInvalida{Nombre: "x", Causa: "test"},
	}
	for _, e := range errs {
		if e.Error() == "" {
			t.Errorf("error %T tiene mensaje vacío", e)
		}
	}
}

// TestNuevaMetadata verifica la metadata inicial con duración.
func TestNuevaMetadata(t *testing.T) {
	m := NuevaMetadata(42.5)
	if m["duracion_ms"].(float64) != 42.5 {
		t.Errorf("duracion_ms = %v, esperaba 42.5", m["duracion_ms"])
	}
}

// herrInvalida es una herramienta que falla Validar().
type herrInvalida struct{}

var _ Herramienta = (*herrInvalida)(nil)

func (h *herrInvalida) Nombre() string          { return "invalida" }
func (h *herrInvalida) Descripcion() string     { return "falsa" }
func (h *herrInvalida) Parametros() []Parametro { return nil }
func (h *herrInvalida) Ejecutar(ctx context.Context, p map[string]interface{}) (Resultado, error) {
	return Resultado{}, nil
}
func (h *herrInvalida) Validar() error {
	return errors.New("simulación de fallo")
}

// TestHerramientaInvalida verifica que Validar puede fallar.
func TestHerramientaInvalida(t *testing.T) {
	h := &herrInvalida{}
	if err := h.Validar(); err == nil {
		t.Error("Validar debería fallar para herrInvalida")
	}
}
