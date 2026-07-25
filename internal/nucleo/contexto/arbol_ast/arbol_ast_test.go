package arbol_ast

import (
	"strings"
	"testing"
)

func TestParsearGo_FuncionSimple(t *testing.T) {
	parser := NuevoParser()
	contenido := `package auth

// Saludar retorna un saludo personalizado.
func Saludar(nombre string) string {
	return "hola " + nombre
}
`
	ast, err := parser.ParsearContenido("auth.go", contenido)
	if err != nil {
		t.Fatalf("ParsearContenido: %v", err)
	}
	if ast.Paquete != "auth" {
		t.Errorf("Paquete = %q, esperado 'auth'", ast.Paquete)
	}

	// Debe tener 1 símbolo tipo "funcion"
	var fn *Simbolo
	for i, s := range ast.Simbolos {
		if s.Tipo == "funcion" {
			fn = &ast.Simbolos[i]
			break
		}
	}
	if fn == nil {
		t.Fatal("debería haber 1 función")
	}
	if fn.Nombre != "Saludar" {
		t.Errorf("Nombre = %q, esperado 'Saludar'", fn.Nombre)
	}
	if !fn.Exportado {
		t.Error("Saludar debería estar exportado")
	}
	if fn.Docstring == "" {
		t.Error("Docstring debería estar poblado")
	}
	if !strings.Contains(fn.Docstring, "Saludar retorna un saludo") {
		t.Errorf("Docstring inesperado: %q", fn.Docstring)
	}
	if !strings.Contains(fn.Firma, "func Saludar(nombre string) string") {
		t.Errorf("Firma inesperada: %q", fn.Firma)
	}
}

func TestParsearGo_MetodoConReceiver(t *testing.T) {
	parser := NuevoParser()
	contenido := `package server

type Server struct {
	addr string
}

func (s *Server) Handle(ctx context.Context, req *Request) (*Response, error) {
	return nil, nil
}
`
	ast, _ := parser.ParsearContenido("server.go", contenido)

	var metodo *Simbolo
	for i, s := range ast.Simbolos {
		if s.Tipo == "metodo" {
			metodo = &ast.Simbolos[i]
			break
		}
	}
	if metodo == nil {
		t.Fatal("debería haber 1 método")
	}
	if metodo.Nombre != "Handle" {
		t.Errorf("Nombre = %q, esperado 'Handle'", metodo.Nombre)
	}
	if metodo.Receiver != "*Server" {
		t.Errorf("Receiver = %q, esperado '*Server'", metodo.Receiver)
	}
	if !strings.Contains(metodo.Firma, "func (s *Server) Handle") {
		t.Errorf("Firma incorrecta: %q", metodo.Firma)
	}
	if !strings.Contains(metodo.Firma, "context.Context") {
		t.Errorf("Firma debería tener context.Context: %q", metodo.Firma)
	}
}

func TestParsearGo_Struct(t *testing.T) {
	parser := NuevoParser()
	contenido := `package models

// Usuario representa un usuario del sistema.
type Usuario struct {
	Nombre string
	Edad   int
}
`
	ast, _ := parser.ParsearContenido("models.go", contenido)

	var estructura *Simbolo
	for i, s := range ast.Simbolos {
		if s.Tipo == "estructura" {
			estructura = &ast.Simbolos[i]
			break
		}
	}
	if estructura == nil {
		t.Fatal("debería haber 1 estructura")
	}
	if estructura.Nombre != "Usuario" {
		t.Errorf("Nombre = %q", estructura.Nombre)
	}
	if estructura.Docstring == "" {
		t.Error("Docstring debería estar poblado")
	}
}

func TestParsearGo_Interface(t *testing.T) {
	parser := NuevoParser()
	contenido := `package storage

type Storage interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
}
`
	ast, _ := parser.ParsearContenido("storage.go", contenido)

	var iface *Simbolo
	for i, s := range ast.Simbolos {
		if s.Tipo == "interface" {
			iface = &ast.Simbolos[i]
			break
		}
	}
	if iface == nil {
		t.Fatal("debería haber 1 interface")
	}
	if iface.Nombre != "Storage" {
		t.Errorf("Nombre = %q", iface.Nombre)
	}
}

func TestParsearGo_Imports(t *testing.T) {
	parser := NuevoParser()
	contenido := `package main

import (
	"fmt"
	"context"
	"os"
)

func main() {
	fmt.Println("hola")
}
`
	ast, _ := parser.ParsearContenido("main.go", contenido)

	if len(ast.Imports) != 3 {
		t.Errorf("debería tener 3 imports, got %d", len(ast.Imports))
	}
	// Debe haber 3 símbolos tipo "import"
	importCount := 0
	for _, s := range ast.Simbolos {
		if s.Tipo == "import" {
			importCount++
		}
	}
	if importCount != 3 {
		t.Errorf("debería tener 3 símbolos import, got %d", importCount)
	}
}

func TestParsearGo_ConstYVar(t *testing.T) {
	parser := NuevoParser()
	contenido := `package config

const Version = "1.0.0"

var DefaultPort = 8080
`
	ast, _ := parser.ParsearContenido("config.go", contenido)

	hayConst := false
	hayVar := false
	for _, s := range ast.Simbolos {
		if s.Tipo == "constante" && s.Nombre == "Version" {
			hayConst = true
		}
		if s.Tipo == "variable" && s.Nombre == "DefaultPort" {
			hayVar = true
		}
	}
	if !hayConst {
		t.Error("debería detectar la constante Version")
	}
	if !hayVar {
		t.Error("debería detectar la variable DefaultPort")
	}
}

func TestParsearGo_FuncionMultilinea(t *testing.T) {
	parser := NuevoParser()
	contenido := `package api

func FuncionLarga(
	a int,
	b string,
	c float64,
) (int, string, error) {
	return 0, "", nil
}
`
	ast, _ := parser.ParsearContenido("api.go", contenido)

	var fn *Simbolo
	for i, s := range ast.Simbolos {
		if s.Tipo == "funcion" {
			fn = &ast.Simbolos[i]
			break
		}
	}
	if fn == nil {
		t.Fatal("debería haber 1 función")
	}
	// La firma debe contener todos los parámetros
	if !strings.Contains(fn.Firma, "a int") {
		t.Errorf("firma debería tener 'a int': %q", fn.Firma)
	}
	if !strings.Contains(fn.Firma, "b string") {
		t.Errorf("firma debería tener 'b string': %q", fn.Firma)
	}
	if !strings.Contains(fn.Firma, "c float64") {
		t.Errorf("firma debería tener 'c float64': %q", fn.Firma)
	}
	// Y los 3 retornos
	if !strings.Contains(fn.Firma, "int") || !strings.Contains(fn.Firma, "string") || !strings.Contains(fn.Firma, "error") {
		t.Errorf("firma debería tener los retornos: %q", fn.Firma)
	}
}

func TestParsearGo_NoExportado(t *testing.T) {
	parser := NuevoParser()
	contenido := `package main

func helper() {}

type private struct{}
`
	ast, _ := parser.ParsearContenido("main.go", contenido)

	for _, s := range ast.Simbolos {
		if s.Exportado {
			t.Errorf("símbolo %q no debería estar exportado", s.Nombre)
		}
	}
}

func TestParsearGo_SintaxisInvalida(t *testing.T) {
	parser := NuevoParser()
	contenido := `package main
func {{{ invalid
`
	ast, _ := parser.ParsearContenido("bad.go", contenido)
	if !ast.TieneError {
		t.Error("TieneError debería ser true con sintaxis inválida")
	}
	if ast.Error == "" {
		t.Error("Error debería estar poblado")
	}
}

func TestParsearGo_LineasCorrectas(t *testing.T) {
	parser := NuevoParser()
	contenido := `package main

import "fmt"

func main() {
	fmt.Println("hola")
}

func otra() {}
`
	ast, _ := parser.ParsearContenido("main.go", contenido)

	for _, s := range ast.Simbolos {
		if s.LineaIni < 1 {
			t.Errorf("%s: LineaIni debería ser >= 1, got %d", s.Nombre, s.LineaIni)
		}
		if s.LineaFin < s.LineaIni {
			t.Errorf("%s: LineaFin (%d) debería ser >= LineaIni (%d)",
				s.Nombre, s.LineaFin, s.LineaIni)
		}
	}
}

func TestParsear_LenguajeNoGo(t *testing.T) {
	parser := NuevoParser()
	ast, err := parser.ParsearContenido("script.py", "print('hola')")
	if err != nil {
		t.Fatalf("ParsearContenido: %v", err)
	}
	if ast.Lenguaje != "python" {
		t.Errorf("Lenguaje = %q, esperado 'python'", ast.Lenguaje)
	}
	if len(ast.Simbolos) != 0 {
		t.Errorf("no debería tener símbolos para python, got %d", len(ast.Simbolos))
	}
}
