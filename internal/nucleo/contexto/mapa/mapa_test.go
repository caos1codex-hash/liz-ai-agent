package mapa

import (
	"os"
	"path/filepath"
	"testing"
)

func crearProyectoTest(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Crear estructura de proyecto Go de prueba
	dirs := []string{
		"cmd/app",
		"internal/core",
		"internal/utils",
		"configs",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Crear archivos Go
	archivos := map[string]string{
		"cmd/app/main.go": `package main

import "fmt"

func main() {
	fmt.Println("hola mundo")
}
`,
		"internal/core/core.go": `package core

// Procesador procesa datos
type Procesador struct {
	Nombre string
}

// Ejecutar ejecuta el procesador
func (p *Procesador) Ejecutar() error {
	return nil
}

func NuevaFuncion() string {
	return "nueva"
}
`,
		"internal/utils/helpers.go": `package utils

// Ayudante es una función helper
func Ayudante(x int) int {
	return x * 2
}
`,
		"configs/config.yaml": `servidor:
  puerto: 8080
  host: localhost
`,
		"README.md": `# Mi Proyecto

Este es un proyecto de prueba.
`,
		"go.mod": `module mi-proyecto

go 1.21
`,
	}

	for ruta, contenido := range archivos {
		if err := os.WriteFile(filepath.Join(tmpDir, ruta), []byte(contenido), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return tmpDir
}

func TestGenerar_Basico(t *testing.T) {
	tmpDir := crearProyectoTest(t)
	gen := NuevoGenerador()

	mapaProy, err := gen.Generar(tmpDir)
	if err != nil {
		t.Fatalf("Generar() error: %v", err)
	}

	if mapaProy.Version != "1.0" {
		t.Errorf("versión esperada '1.0', obtuve '%s'", mapaProy.Version)
	}

	// Debería tener al menos los archivos Go, YAML, markdown y go.mod
	if mapaProy.TotalArchivos < 5 {
		t.Errorf("se esperaban al menos 5 archivos, obtuve %d", mapaProy.TotalArchivos)
	}

	if mapaProy.TotalLineas < 10 {
		t.Errorf("se esperaban al menos 10 líneas, obtuve %d", mapaProy.TotalLineas)
	}

	// Verificar que no incluye .git ni otros ignorados
	for _, e := range mapaProy.Entradas {
		for _, ignorado := range []string{".git", "node_modules", "vendor"} {
			if filepath.Base(e.Ruta) == ignorado {
				t.Errorf("entrada %s no debería estar en el mapa", e.Ruta)
			}
		}
	}
}

func TestGenerar_DetectaLenguaje(t *testing.T) {
	tmpDir := crearProyectoTest(t)
	gen := NuevoGenerador()

	mapaProy, err := gen.Generar(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Buscar archivos por lenguaje
	lenguajes := make(map[string]bool)
	for _, e := range mapaProy.Entradas {
		if e.Lenguaje != "" {
			lenguajes[e.Lenguaje] = true
		}
	}

	if !lenguajes["go"] {
		t.Error("debería detectar archivos Go")
	}
	if !lenguajes["yaml"] {
		t.Error("debería detectar archivos YAML")
	}
	if !lenguajes["markdown"] {
		t.Error("debería detectar archivos Markdown")
	}
}

func TestGenerar_ArchivosConResumen(t *testing.T) {
	tmpDir := crearProyectoTest(t)
	gen := NuevoGenerador()

	mapaProy, err := gen.Generar(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Verificar que el mapa tiene el diccionario de archivos
	if len(mapaProy.Archivos) == 0 {
		t.Error("el mapa debería tener entradas en el diccionario 'archivos'")
	}

	// Cada archivo debería tener un resumen no vacío
	for ruta, resumen := range mapaProy.Archivos {
		if resumen == "" {
			t.Errorf("archivo %s tiene resumen vacío", ruta)
		}
	}
}

func TestGenerar_Estructura(t *testing.T) {
	tmpDir := crearProyectoTest(t)
	gen := NuevoGenerador()

	mapaProy, err := gen.Generar(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if mapaProy.Estructura == "" {
		t.Error("la estructura no debería estar vacía")
	}

	if mapaProy.Resumen == "" {
		t.Error("el resumen no debería estar vacío")
	}
}

func TestGenerar_DirectorioInexistente(t *testing.T) {
	gen := NuevoGenerador()
	_, err := gen.Generar("/tmp/no_existe_123456789")
	if err == nil {
		t.Error("debería retornar error para directorio inexistente")
	}
}

func TestGenerar_ArchivoComoRuta(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "archivo.txt")
	os.WriteFile(tmpFile, []byte("hola"), 0644)

	gen := NuevoGenerador()
	_, err := gen.Generar(tmpFile)
	if err == nil {
		t.Error("debería retornar error cuando la ruta es un archivo, no un directorio")
	}
}

func TestGenerar_ConProfundidad(t *testing.T) {
	tmpDir := crearProyectoTest(t)

	// Solo raíz (profundidad 1)
	gen := NuevoGenerador(OpcionesMapa{ProfundidadMax: 1})
	mapaProy, err := gen.Generar(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Con profundidad 1, debería tener solo archivos en la raíz
	for _, e := range mapaProy.Entradas {
		profundidad := len(filepath.SplitList(e.Ruta))
		if profundidad > 1 {
			t.Errorf("con profundidad 1, %s está a profundidad %d", e.Ruta, profundidad)
		}
	}
}

func TestGuardarY_Cargar(t *testing.T) {
	tmpDir := crearProyectoTest(t)
	gen := NuevoGenerador()

	mapaProy, err := gen.Generar(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Guardar
	rutaGuardar := filepath.Join(t.TempDir(), "mapa.json")
	if err := gen.Guardar(mapaProy, rutaGuardar); err != nil {
		t.Fatalf("Guardar() error: %v", err)
	}

	// Cargar
	cargado, err := Cargar(rutaGuardar)
	if err != nil {
		t.Fatalf("Cargar() error: %v", err)
	}

	if cargado.Version != mapaProy.Version {
		t.Errorf("versión no coincide después de cargar")
	}
	if cargado.TotalArchivos != mapaProy.TotalArchivos {
		t.Errorf("total archivos no coincide: %d vs %d",
			cargado.TotalArchivos, mapaProy.TotalArchivos)
	}
	if cargado.Proyecto != mapaProy.Proyecto {
		t.Errorf("proyecto no coincide: '%s' vs '%s'",
			cargado.Proyecto, mapaProy.Proyecto)
	}
}

func TestDetectarLenguaje(t *testing.T) {
	tests := []struct {
		ruta     string
		esperado string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"index.ts", "typescript"},
		{"style.css", "css"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"README.md", "markdown"},
		{"index.html", "html"},
		{"server.rs", "rust"},
		{"Makefile", "makefile"},
		{"Dockerfile", "docker"},
		{"go.mod", "go-module"},
		{"app.unknown", ""},
	}

	for _, tt := range tests {
		resultado := detectarLenguaje(tt.ruta)
		if resultado != tt.esperado {
			t.Errorf("detectarLenguaje(%q) = %q, esperado %q",
				tt.ruta, resultado, tt.esperado)
		}
	}
}

func TestResumirGo(t *testing.T) {
	codigo := `package main

import "fmt"

type MiTipo struct {
	Nombre string
}

func Funcion1() {}
func (m *MiTipo) Metodo1() {}

var VariableGlobal = 42
`
	resultado := resumirGo(codigo, 14)
	if resultado == "" {
		t.Error("resumirGo no debería retornar vacío")
	}
}
