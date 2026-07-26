package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNuevaConSalida(t *testing.T) {
	var buf bytes.Buffer
	log := NuevaConSalida("test_modulo", &buf)
	if log == nil {
		t.Fatal("NuevaConSalida() retornó nil")
	}
	if log.modulo != "test_modulo" {
		t.Errorf("módulo esperado 'test_modulo', obtuve '%s'", log.modulo)
	}
}

func TestNiveles(t *testing.T) {
	var buf bytes.Buffer
	log := NuevaConSalida("test", &buf)

	log.Debug("mensaje debug")
	log.Info("mensaje info")
	log.Warn("mensaje warn")
	log.Error("mensaje error")

	output := buf.String()

	// Verificar que todos los niveles aparecen
	if !strings.Contains(output, "DEBUG") {
		t.Error("falta nivel DEBUG en output")
	}
	if !strings.Contains(output, "INFO") {
		t.Error("falta nivel INFO en output")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("falta nivel WARN en output")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("falta nivel ERROR en output")
	}
	if !strings.Contains(output, "mensaje debug") {
		t.Error("falta contenido del mensaje debug")
	}
	if !strings.Contains(output, "[test") {
		t.Error("falta nombre del módulo en output")
	}
}

func TestSetNivelMin(t *testing.T) {
	var buf bytes.Buffer
	log := NuevaConSalida("test", &buf)
	log.SetNivelMin(NivelError)

	log.Debug("no debe aparecer")
	log.Info("tampoco")
	log.Warn("este tampoco")
	log.Error("este sí")

	output := buf.String()

	if strings.Contains(output, "no debe aparecer") {
		t.Error("mensaje DEBUG no debió aparecer con nivel mínimo ERROR")
	}
	if strings.Contains(output, "tampoco") {
		t.Error("mensaje INFO no debió aparecer con nivel mínimo ERROR")
	}
	if !strings.Contains(output, "este sí") {
		t.Error("mensaje ERROR debió aparecer")
	}
}

func TestNivelValor(t *testing.T) {
	if nivelValor(NivelDebug) >= nivelValor(NivelInfo) {
		t.Error("DEBUG debe ser menor que INFO")
	}
	if nivelValor(NivelInfo) >= nivelValor(NivelWarn) {
		t.Error("INFO debe ser menor que WARN")
	}
	if nivelValor(NivelWarn) >= nivelValor(NivelError) {
		t.Error("WARN debe ser menor que ERROR")
	}
	if nivelValor(NivelError) >= nivelValor(NivelFatal) {
		t.Error("ERROR debe ser menor que FATAL")
	}
}

func TestFormatoConArgs(t *testing.T) {
	var buf bytes.Buffer
	log := NuevaConSalida("test", &buf)

	log.Info("procesando %d archivos en %s", 42, "/tmp")

	output := buf.String()
	if !strings.Contains(output, "42 archivos en /tmp") {
		t.Errorf("formato con args no funcionó. output: %s", output)
	}
}
