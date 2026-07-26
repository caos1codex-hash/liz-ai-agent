package resumen

import (
	"os"
	"path/filepath"
	"testing"
)

func crearArchivoTest(t *testing.T, nombre, contenido string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), nombre)
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

const codigoGoEjemplo = `package nucleo

import (
	"fmt"
	"os"
	"strings"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/logger"
)

// Configuracion es la configuración principal.
type Configuracion struct {
	Version string
	Puerto  int
}

// Nuevo crea una nueva configuración.
func Nuevo() *Configuracion {
	return &Configuracion{Version: "1.0", Puerto: 3000}
}

// Procesar procesa algo.
func (c *Configuracion) Procesar() error {
	return nil
}

var GlobalConfig = &Configuracion{}

const versionDefault = "0.1.0"
`

func TestGenerar_Go(t *testing.T) {
	ruta := crearArchivoTest(t, "config.go", codigoGoEjemplo)
	gen := NuevoGenerador()

	res, err := gen.Generar("config.go", ruta)
	if err != nil {
		t.Fatalf("Generar() error: %v", err)
	}

	if res.Lenguaje != "go" {
		t.Errorf("lenguaje esperado 'go', obtuve '%s'", res.Lenguaje)
	}
	if res.TipoArchivo != "codigo" {
		t.Errorf("tipo esperado 'codigo', obtuve '%s'", res.TipoArchivo)
	}
	if res.Lineas == 0 {
		t.Error("líneas no debería ser 0")
	}
	if res.Descripcion == "" {
		t.Error("descripción no debería estar vacía")
	}
}

func TestGenerar_ExtraeImports(t *testing.T) {
	ruta := crearArchivoTest(t, "config.go", codigoGoEjemplo)
	gen := NuevoGenerador()

	res, _ := gen.Generar("config.go", ruta)

	// Debería detectar algunos imports
	if len(res.Importados) == 0 {
		t.Error("debería detectar al menos un import")
	}

	// Verificar que fmt está en los imports (se extrae como "fmt" del path)
	encontrado := false
	for _, imp := range res.Importados {
		if imp == "fmt" || imp == "os" || imp == "strings" || imp == "logger" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("no se encontró ningún import esperado en: %v", res.Importados)
	}
}

func TestGenerar_ExtraeExportados(t *testing.T) {
	ruta := crearArchivoTest(t, "config.go", codigoGoEjemplo)
	gen := NuevoGenerador()

	res, _ := gen.Generar("config.go", ruta)

	// Debería detectar: Configuracion, Nuevo, Procesar, GlobalConfig
	if len(res.Exportados) == 0 {
		t.Error("debería detectar al menos un exportado")
	}

	// Buscar Configuracion
	encontrado := false
	for _, exp := range res.Exportados {
		if exp == "Configuracion" || exp == "Nuevo" || exp == "Procesar" {
			encontrado = true
			break
		}
	}
	if !encontrado {
		t.Errorf("no se encontró exportado esperado en: %v", res.Exportados)
	}
}

func TestGenerar_Python(t *testing.T) {
	codigo := `#!/usr/bin/env python3
"""Módulo de utilidades."""

import os
import sys
from pathlib import Path


case class Utilidades:
    def __init__(self):
        pass

    def procesar(self, datos):
        return datos

def helper(x):
    return x * 2


def _privado():
    pass
`
	ruta := crearArchivoTest(t, "utils.py", codigo)
	gen := NuevoGenerador()

	res, err := gen.Generar("utils.py", ruta)
	if err != nil {
		t.Fatalf("Generar() error: %v", err)
	}

	if res.Lenguaje != "python" {
		t.Errorf("lenguaje esperado 'python', obtuve '%s'", res.Lenguaje)
	}

	// _privado NO debería estar en exportados
	for _, exp := range res.Exportados {
		if exp == "_privado" {
			t.Error("_privado no debería estar en exportados")
		}
	}
}

func TestGenerar_ArchivoInexistente(t *testing.T) {
	gen := NuevoGenerador()
	_, err := gen.Generar("no_existe.go", "/tmp/no_existe_12345.go")
	if err == nil {
		t.Error("debería retornar error para archivo inexistente")
	}
}

func TestTipoArchivo(t *testing.T) {
	tests := []struct {
		ruta     string
		esperado string
	}{
		{"main.go", "codigo"},
		{"main_test.go", "test"},
		{"test_utils.py", "test"},
		{"config.yaml", "config"},
		{"data.json", "config"},
		{"README.md", "docs"},
		{"styles.css", "frontend"},
		{"app.html", "frontend"},
		{"deploy.sh", "script"},
		{"Makefile", "script"},
		{"Dockerfile", "script"},
		{"LICENSE", "docs"},
		{"CHANGELOG", "docs"},
		{"script.tsx", "codigo"},
		{"unknown.xyz", "otro"},
	}

	for _, tt := range tests {
		resultado := TipoArchivo(tt.ruta)
		if resultado != tt.esperado {
			t.Errorf("TipoArchivo(%q) = %q, esperado %q", tt.ruta, resultado, tt.esperado)
		}
	}
}

func TestCalcularComplejidad(t *testing.T) {
	tests := []struct {
		res      *ResumenArchivo
		esperado string
	}{
		{
			&ResumenArchivo{Exportados: []string{"A"}, Importados: []string{"fmt"}, Lineas: 10},
			"baja",
		},
		{
			&ResumenArchivo{Exportados: []string{"A", "B", "C", "D", "E"}, Importados: []string{"fmt", "os", "strings", "log", "http"}, Lineas: 50},
			"media",
		},
		{
			&ResumenArchivo{Exportados: []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}, Importados: []string{"fmt", "os", "strings", "log", "http", "net", "encoding", "time", "context", "sync"}, Lineas: 500},
			"alta",
		},
	}

	for _, tt := range tests {
		resultado := calcularComplejidad(tt.res)
		if resultado != tt.esperado {
			t.Errorf("calcularComplejidad() = %q, esperado %q", resultado, tt.esperado)
		}
	}
}

func TestEsExportado(t *testing.T) {
	if !esExportado("Configuracion") {
		t.Error("'Configuracion' debería ser exportado")
	}
	if esExportado("configuracion") {
		t.Error("'configuracion' no debería ser exportado")
	}
	if esExportado("") {
		t.Error("vacío no debería ser exportado")
	}
}

func TestEliminarDuplicados(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "d"}
	resultado := eliminarDuplicados(input)

	if len(resultado) != 4 {
		t.Errorf("esperados 4 únicos, obtuve %d: %v", len(resultado), resultado)
	}

	// Verificar orden relativo
	esperado := []string{"a", "b", "c", "d"}
	for i, v := range esperado {
		if resultado[i] != v {
			t.Errorf("posicion %d: esperado %q, obtuve %q", i, v, resultado[i])
		}
	}
}
