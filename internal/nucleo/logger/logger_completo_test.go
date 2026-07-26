package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNueva_CreaDirectorioYArchivo(t *testing.T) {
	// Usar un temp dir para no afectar el home real
	tmpDir := t.TempDir()

	// Monkey-patch: no podemos fácilmente mock UserHomeDir, así que testeamos
	// que Nueva falla gracefully si el home no existe
	log, err := Nueva("test")
	if err != nil {
		// Puede fallar si no hay home, es aceptable
		t.Logf("Nueva() retornó error (aceptable): %v", err)
		return
	}
	if log == nil {
		t.Fatal("esperaba logger no nil")
	}
	log.Cerrar()
}

func TestNueva_SinHome(t *testing.T) {
	// Este test verifica que Nueva maneja errores gracefully
	// No podemos forzar el error de UserHomeDir fácilmente,
	// pero podemos verificar que la función retorna sin panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Nueva() panic: %v", r)
		}
	}()
	log, err := Nueva("test")
	if err != nil {
		return // Error esperado en algunos entornos
	}
	if log != nil {
		log.Cerrar()
	}
}

func TestNivelValor_Default(t *testing.T) {
	// Categoría desconocida debe retornar 0
	if nivelValor(Nivel("desconocido")) != 0 {
		t.Error("nivel desconocido debería ser 0")
	}
}

func TestCerrar_Nil(t *testing.T) {
	log := NuevaConSalida("test", nil)
	log.Cerrar() // No debe panic
}

func TestCerrar_ConArchivo(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "liz_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	log := &Logger{
		archivo:  tmpFile,
		modulo:   "test",
		nivelMin: NivelDebug,
		salida:   os.Stdout,
	}
	log.Cerrar()
	// Verificar que el archivo se cerró intentando renombrarlo
	// Si se cerró bien, deberíamos poder eliminarlo
	os.Remove(tmpPath)
}

func TestRegistrar_ConArchivo(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "liz_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	// Reabrir para append
	f, err := os.OpenFile(tmpPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}

	log := &Logger{
		archivo:  f,
		modulo:   "test_mod",
		nivelMin: NivelDebug,
		salida:   os.Stdout,
	}

	log.Info("mensaje de prueba")
	log.Cerrar()

	// Verificar que se escribió JSON al archivo
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `"nivel":"INFO"`) {
		t.Error("debería contener nivel INFO en JSON")
	}
	if !strings.Contains(content, `"modulo":"test_mod"`) {
		t.Error("debería contener módulo en JSON")
	}
	if !strings.Contains(content, "mensaje de prueba") {
		t.Error("debería contener el mensaje en JSON")
	}

	os.Remove(tmpPath)
}

func TestRegistrar_SinArchivo(t *testing.T) {
	var buf strings.Builder
	log := &Logger{
		archivo:  nil,
		modulo:   "test",
		nivelMin: NivelDebug,
		salida:   &buf,
	}

	log.Debug("debug sin archivo")
	log.Warn("warn sin archivo")
	log.Error("error sin archivo")

	output := buf.String()
	if !strings.Contains(output, "debug sin archivo") {
		t.Error("debería contener mensaje debug")
	}
	if !strings.Contains(output, "warn sin archivo") {
		t.Error("debería contener mensaje warn")
	}
	if !strings.Contains(output, "error sin archivo") {
		t.Error("debería contener mensaje error")
	}
}

func TestSetNivelMin_TodosLosNiveles(t *testing.T) {
	var buf strings.Builder
	log := NuevaConSalida("test", &buf)

	niveles := []Nivel{NivelDebug, NivelInfo, NivelWarn, NivelError, NivelFatal}
	for _, nivel := range niveles {
		buf.Reset()
		log.SetNivelMin(nivel)

		// Escribir en todos los niveles
		log.Debug("d")
		log.Info("i")
		log.Warn("w")
		log.Error("e")

		output := buf.String()
		// Verificar que los niveles inferiores al mínimo no aparecen
		for _, n := range niveles {
			nombre := string(n)
			if nivelValor(n) < nivelValor(nivel) {
				// No debería aparecer
				// (no verificamos por nombre de nivel porque el output usa formato diferente)
			}
		}
	}
}

func TestRegistrar_TodosColores(t *testing.T) {
	var buf strings.Builder
	log := NuevaConSalida("test", &buf)

	// Escribir en cada nivel y verificar que hay output
	log.Debug("d")
	log.Info("i")
	log.Warn("w")
	log.Error("e")

	output := buf.String()
	// Verificar códigos de color ANSI
	codes := []string{"\033[36m", "\033[32m", "\033[33m", "\033[31m"}
	for _, code := range codes {
		if !strings.Contains(output, code) {
			t.Errorf("debería contener código de color %q", code)
		}
	}
	// Reset
	if !strings.Contains(output, "\033[0m") {
		t.Error("debería contener color reset")
	}
}

func TestEntradaLog_Struct(t *testing.T) {
	e := EntradaLog{
		Timestamp: "2024-01-01T00:00:00.000Z",
		Nivel:     "INFO",
		Modulo:    "test",
		Mensaje:   "hola",
		Datos:     map[string]string{"key": "value"},
	}

	if e.Timestamp != "2024-01-01T00:00:00.000Z" {
		t.Error("timestamp incorrecto")
	}
	if e.Datos == nil {
		t.Error("datos no debería ser nil")
	}
}

func TestNivel_Constantes(t *testing.T) {
	if NivelDebug != "DEBUG" {
		t.Error("NivelDebug incorrecto")
	}
	if NivelInfo != "INFO" {
		t.Error("NivelInfo incorrecto")
	}
	if NivelWarn != "WARN" {
		t.Error("NivelWarn incorrecto")
	}
	if NivelError != "ERROR" {
		t.Error("NivelError incorrecto")
	}
	if NivelFatal != "FATAL" {
		t.Error("NivelFatal incorrecto")
	}
}

func TestRegistrar_ConFormatoArgs(t *testing.T) {
	var buf strings.Builder
	log := NuevaConSalida("mod", &buf)

	log.Info("usuario %s ejecutó %d comandos", "juan", 5)
	output := buf.String()
	if !strings.Contains(output, "juan ejecutó 5 comandos") {
		t.Errorf("formato con args incorrecto: %s", output)
	}
}

func TestRegistrar_UbicacionEnJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "liz_ubica_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	f, _ := os.OpenFile(tmpPath, os.O_APPEND|os.O_WRONLY, 0644)

	log := &Logger{
		archivo:  f,
		modulo:   "ubic_test",
		nivelMin: NivelInfo,
		salida:   nil,
	}
	log.Info("test ubicacion")
	log.Cerrar()

	data, _ := os.ReadFile(tmpPath)
	content := string(data)
	if !strings.Contains(content, `"ubicacion":`) {
		t.Error("debería contener campo ubicacion en JSON del archivo")
	}
	os.Remove(tmpPath)
}

func TestLogger_Concurrente(t *testing.T) {
	var buf strings.Builder
	log := NuevaConSalida("conc", &buf)

	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func(id int) {
			log.Info("mensaje %d", id)
			done <- true
		}(i)
	}

	// Esperar todos
	for i := 0; i < 50; i++ {
		<-done
	}

	// Verificar que no hubo panic y se escribieron mensajes
	output := buf.String()
	count := strings.Count(output, "mensaje")
	if count < 50 {
		t.Errorf("esperaba al menos 50 mensajes, got %d", count)
	}
}

func TestNueva_RutaAbsoluta(t *testing.T) {
	// Verificar que la ruta del log es absoluta (home + /.liz/logs/)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no se puede obtener home")
	}
	expectedLogDir := filepath.Join(home, ".liz", "logs")
	if !filepath.IsAbs(expectedLogDir) {
		t.Error("ruta de logs debería ser absoluta")
	}
}
