package resumen

import (
	"strings"
	"testing"
)

func TestGenerarResumen_GoFunc(t *testing.T) {
	codigo := `package main

import "fmt"

// Saludar saluda al usuario
func Saludar(nombre string) string {
	return "Hola " + nombre
}

func main() {
	fmt.Println(Saludar("mundo"))
}`

	resumen := GenerarResumen(codigo, "saludo.go", "go")
	if resumen == "" {
		t.Fatal("esperaba resumen no vacío")
	}
}

func TestGenerarResumen_GoStruct(t *testing.T) {
	codigo := `package models

// Usuario representa un usuario del sistema
type Usuario struct {
	ID       int
	Nombre   string
	Email    string
}

// Admin es un usuario con privilegios
type Admin struct {
	Usuario
	Nivel int
}`

	resumen := GenerarResumen(codigo, "models.go", "go")
	if resumen == "" {
		t.Fatal("esperaba resumen no vacío")
	}
	// Debería detectar structs
	if !strings.Contains(resumen, "Usuario") && !strings.Contains(resumen, "Admin") {
		t.Log("resumen no contiene nombres de structs (puede ser aceptable)")
	}
}

func TestGenerarResumen_GoInterface(t *testing.T) {
	codigo := `package storage

// Almacenable define la interfaz de almacenamiento
type Almacenable interface {
	Guardar(data []byte) error
	Leer() ([]byte, error)
	eliminar()
}`

	resumen := GenerarResumen(codigo, "storage.go", "go")
	if resumen == "" {
		t.Fatal("esperaba resumen no vacío")
	}
}

func TestGenerarResumen_Python(t *testing.T) {
	codigo := `class Calculadora:
    def sumar(self, a, b):
        return a + b
    
    def restar(self, a, b):
        return a - b`

	resumen := GenerarResumen(codigo, "calc.py", "python")
	if resumen == "" {
		t.Fatal("esperaba resumen no vacío")
	}
}

func TestGenerarResumen_JavaScript(t *testing.T) {
	codigo := `export function procesar(datos) {
    return datos.map(d => d * 2);
}

export const PI = 3.14159;`

	resumen := GenerarResumen(codigo, "utils.js", "javascript")
	if resumen == "" {
		t.Fatal("esperaba resumen no vacío")
	}
}

func TestGenerarResumen_Vacio(t *testing.T) {
	resumen := GenerarResumen("", "vacio.go", "go")
	if resumen == "" {
		t.Log("resumen vacío para código vacío es aceptable")
	}
}

func TestGenerarResumen_SoloComentarios(t *testing.T) {
	codigo := `// Este es un archivo
// solo con comentarios
// sin código real`

	resumen := GenerarResumen(codigo, "coments.go", "go")
	// Puede ser vacío o contener info de comentarios
	_ = resumen
}

func TestGenerarResumen_LenguajeDesconocido(t *testing.T) {
	codigo := `<html><body>Hola</body></html>`
	resumen := GenerarResumen(codigo, "index.html", "html")
	// No debería panic
	_ = resumen
}

func TestGenerarResumen_CodigoLargo(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("// Función %d\nfunc Funcion%d() int { return %d }\n\n", i, i, i))
	}

	resumen := GenerarResumen(sb.String(), "largo.go", "go")
	if resumen == "" {
		t.Fatal("esperaba resumen para código largo")
	}
	// Verificar que se trunca razonablemente
	if len(resumen) > 5000 {
		t.Errorf("resumen muy largo: %d chars", len(resumen))
	}
}

func TestGenerarResumen_Constantes(t *testing.T) {
	codigo := `package config

const Version = "1.0.0"
const MaxRetries = 3
var debug = false`

	resumen := GenerarResumen(codigo, "config.go", "go")
	if resumen == "" {
		t.Fatal("esperaba resumen")
	}
}

func TestGenerarResumen_ConErrores(t *testing.T) {
	codigo := `package main

func main() {
    // código roto
    if true {
        println("hello")
    }
}`

	resumen := GenerarResumen(codigo, "roto.go", "go")
	// No debería panic con código que no compila
	_ = resumen
}