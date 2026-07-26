package integradas

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTerminal_NombreYDescripcion(t *testing.T) {
	tl := NewTerminal()
	if tl.Nombre() != "terminal" {
		t.Errorf("Nombre = %q", tl.Nombre())
	}
	if tl.Descripcion() == "" {
		t.Error("Descripcion vacía")
	}
	if len(tl.Parametros()) < 5 {
		t.Errorf("muy pocos parámetros: %d", len(tl.Parametros()))
	}
	if err := tl.Validar(); err != nil {
		t.Errorf("Validar falló: %v", err)
	}
}

func TestTerminal_Echo(t *testing.T) {
	tl := NewTerminal()
	res, err := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando": "echo",
		"args":    []string{"hola", "mundo"},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !res.Exito {
		t.Errorf("no exito: %+v", res)
	}
	datos := res.Datos.(ResultadoTerminal)
	if datos.CodigoSalida != 0 {
		t.Errorf("CodigoSalida = %d", datos.CodigoSalida)
	}
	if strings.TrimSpace(datos.Stdout) != "hola mundo" {
		t.Errorf("Stdout = %q, esperaba 'hola mundo'", datos.Stdout)
	}
}

func TestTerminal_ComandoInexistente(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando": "comando_que_no_existe_seguramente_2025",
	})
	if res.Exito {
		t.Error("debería fallar")
	}
	// Cuando el binario no existe, no hay Datos pero sí Error
	if res.Error == "" {
		t.Error("Error vacío para comando inexistente")
	}
}

func TestTerminal_Timeout(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando":          "sleep",
		"args":             []string{"10"},
		"timeout_segundos": 1,
	})
	if res.Exito {
		t.Error("debería fallar por timeout")
	}
	datos := res.Datos.(ResultadoTerminal)
	if !datos.Timeout {
		t.Error("debería marcar Timeout=true")
	}
	if datos.DuracionMs < 900 || datos.DuracionMs > 1500 {
		t.Errorf("DuracionMs = %.2f, esperaba ~1000", datos.DuracionMs)
	}
}

func TestTerminal_PeligrosoBloqueado(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando": "rm",
		"args":    []string{"-rf", "/"},
		// sin peligroso_confirma
	})
	if res.Exito {
		t.Error("debería bloquearse sin confirma")
	}
	if !strings.Contains(res.Error, "peligroso") {
		t.Errorf("Error = %q", res.Error)
	}
}

func TestTerminal_PeligrosoConfirmado(t *testing.T) {
	// Probamos con un comando peligroso simulado — no ejecutamos rm real,
	// solo verificamos que el flag permite pasar la verificación.
	tl := NewTerminal()
	// Usamos echo para no causar daño
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando":            "echo",
		"args":               []string{"shutdown"}, // contiene "shutdown" en args
		"peligroso_confirma": true,
	})
	// echo shutdown no contiene el patrón "shutdown" exacto como comando,
	// pero sí en comandoCompleto. Vamos a verificar:
	datos, ok := res.Datos.(ResultadoTerminal)
	if !ok {
		t.Fatalf("Datos no es ResultadoTerminal: %+v", res.Datos)
	}
	// "echo shutdown" no contiene el patrón peligroso exacto "shutdown" porque
	// es solo una substring — sí lo contiene!
	if datos.Peligroso && !res.Exito {
		t.Errorf("si es peligroso y se confirmó, debería ejecutarse: %+v", res)
	}
}

func TestTerminal_Directorio(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando":    "pwd",
		"directorio": "/tmp",
	})
	if !res.Exito {
		t.Fatalf("pwd falló: %+v", res)
	}
	datos := res.Datos.(ResultadoTerminal)
	if strings.TrimSpace(datos.Stdout) != "/tmp" {
		t.Errorf("Stdout = %q, esperaba /tmp", datos.Stdout)
	}
	if datos.Directorio != "/tmp" {
		t.Errorf("Directorio = %q", datos.Directorio)
	}
}

func TestTerminal_EnvVars(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando": "sh",
		"args":    []string{"-c", "echo $MI_VAR"},
		"env":     map[string]interface{}{"MI_VAR": "valor123"},
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoTerminal)
	if strings.TrimSpace(datos.Stdout) != "valor123" {
		t.Errorf("Stdout = %q, esperaba 'valor123'", datos.Stdout)
	}
}

func TestTerminal_StdoutStderrSeparados(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando":                "sh",
		"args":                   []string{"-c", "echo out; echo err 1>&2"},
		"combinar_stdout_stderr": false,
	})
	if !res.Exito {
		t.Fatalf("falló: %+v", res)
	}
	datos := res.Datos.(ResultadoTerminal)
	if !strings.Contains(datos.Stdout, "out") {
		t.Errorf("Stdout = %q", datos.Stdout)
	}
	if !strings.Contains(datos.Stderr, "err") {
		t.Errorf("Stderr = %q", datos.Stderr)
	}
}

func TestTerminal_CodigoSalidaNoCero(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando": "sh",
		"args":    []string{"-c", "exit 42"},
	})
	if res.Exito {
		t.Error("debería fallar")
	}
	datos := res.Datos.(ResultadoTerminal)
	if datos.CodigoSalida != 42 {
		t.Errorf("CodigoSalida = %d, esperaba 42", datos.CodigoSalida)
	}
}

func TestTerminal_Shell(t *testing.T) {
	tl := NewTerminal()
	// Probar pipes con shell=true
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando": "echo hola mundo | tr a-z A-Z",
		"shell":   true,
	})
	if !res.Exito {
		t.Fatalf("shell falló: %+v", res)
	}
	datos := res.Datos.(ResultadoTerminal)
	if strings.TrimSpace(datos.Stdout) != "HOLA MUNDO" {
		t.Errorf("Stdout = %q, esperaba 'HOLA MUNDO'", datos.Stdout)
	}
}

func TestTerminal_SinComando(t *testing.T) {
	tl := NewTerminal()
	res, err := tl.Ejecutar(context.Background(), map[string]interface{}{})
	// No debe retornar error de Go — debe retornar Resultado con Exito=false
	if err != nil {
		t.Errorf("no esperaba error de Go: %v", err)
	}
	if res.Exito {
		t.Error("debería fallar sin comando")
	}
	if !strings.Contains(res.Error, "comando") {
		t.Errorf("Error = %q", res.Error)
	}
}

func TestTerminal_ContextoCancelado(t *testing.T) {
	tl := NewTerminal()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res, _ := tl.Ejecutar(ctx, map[string]interface{}{
		"comando":          "sleep",
		"args":             []string{"10"},
		"timeout_segundos": 30,
	})
	if res.Exito {
		t.Error("debería fallar por cancelación")
	}
}

func TestEsComandoPeligroso(t *testing.T) {
	casos := []struct {
		comando string
		peligro bool
	}{
		{"rm -rf /", true},
		{"RM -RF /", true},   // case insensitive
		{"rm  -rf  /", true}, // múltiples espacios
		{"mkfs.ext4 /dev/sda", true},
		{"dd if=/dev/zero of=/dev/sda", true},
		{"shutdown -h now", true},
		{"echo hola", false},
		{"ls -la", false},
		{"cat archivo.txt", false},
		{"grep patron archivo", false},
	}
	for _, c := range casos {
		obt := esComandoPeligroso(c.comando)
		if obt != c.peligro {
			t.Errorf("esComandoPeligroso(%q) = %v, esperaba %v",
				c.comando, obt, c.peligro)
		}
	}
}

func TestTerminal_MetadataDuracion(t *testing.T) {
	tl := NewTerminal()
	res, _ := tl.Ejecutar(context.Background(), map[string]interface{}{
		"comando": "true",
	})
	if !res.Exito {
		t.Fatal("true debería exito")
	}
	if res.Metadata == nil {
		t.Fatal("Metadata nil")
	}
	duracion, ok := res.Metadata["duracion_ms"].(float64)
	if !ok {
		t.Fatalf("duracion_ms no es float64: %T", res.Metadata["duracion_ms"])
	}
	if duracion < 0 || duracion > 1000 {
		t.Errorf("duracion_ms = %v, esperaba [0, 1000]", duracion)
	}
}

func TestSanitizarParaLog(t *testing.T) {
	entrada := "hola\x00mundo\n\tnull\x07bell"
	salida := SanitizarParaLog(entrada)
	if strings.Contains(salida, "\x00") || strings.Contains(salida, "\x07") {
		t.Errorf("caracteres no imprimibles no sanitizados: %q", salida)
	}
	if !strings.Contains(salida, "hola") || !strings.Contains(salida, "mundo") {
		t.Errorf("texto válido perdido: %q", salida)
	}
}
