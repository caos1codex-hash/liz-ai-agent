package mapa_repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func crearArchivoGoTest(t *testing.T, dir, nombre, contenido string) string {
	t.Helper()
	ruta := filepath.Join(dir, nombre)
	os.MkdirAll(filepath.Dir(ruta), 0755)
	os.WriteFile(ruta, []byte(contenido), 0644)
	return ruta
}

func TestGenerar_ArchivoUnico(t *testing.T) {
	dir := t.TempDir()
	contenido := `package auth

func GenerateToken(userID string) (string, error) {
        return "", nil
}

func ValidateToken(token string) (*Claims, error) {
        return nil, nil
}

type Claims struct {
        UserID string
        Exp    int64
}
`
	ruta := crearArchivoGoTest(t, dir, "auth.go", contenido)

	gen := NuevoGenerador()
	archivos := []ArchivoParaMapa{
		{Ruta: "auth.go", RutaAbsoluta: ruta, Lenguaje: "go", Lineas: 10, Importancia: 1.0},
	}

	mapa := gen.Generar("test-proj", archivos, 5000)
	if mapa == nil {
		t.Fatal("mapa no debería ser nil")
	}
	if mapa.TotalArchivos != 1 {
		t.Errorf("TotalArchivos = %d, esperado 1", mapa.TotalArchivos)
	}
	if mapa.ArchivosIncluidos != 1 {
		t.Errorf("ArchivosIncluidos = %d, esperado 1", mapa.ArchivosIncluidos)
	}
	if len(mapa.Entradas) != 1 {
		t.Fatalf("debería tener 1 entrada, got %d", len(mapa.Entradas))
	}

	entrada := mapa.Entradas[0]
	if entrada.Ruta != "auth.go" {
		t.Errorf("Ruta = %s, esperado auth.go", entrada.Ruta)
	}

	// Debería tener 2 funciones + 1 estructura (no imports)
	nombres := make(map[string]bool)
	for _, s := range entrada.Simbolos {
		nombres[s.Nombre] = true
	}
	if !nombres["GenerateToken"] {
		t.Error("debería tener GenerateToken")
	}
	if !nombres["ValidateToken"] {
		t.Error("debería tener ValidateToken")
	}
	if !nombres["Claims"] {
		t.Error("debería tener Claims")
	}
}

func TestGenerar_OrdenadoPorImportancia(t *testing.T) {
	dir := t.TempDir()
	crearArchivoGoTest(t, dir, "a.go", "package a\nfunc A() {}\n")
	crearArchivoGoTest(t, dir, "b.go", "package b\nfunc B() {}\n")

	gen := NuevoGenerador()
	archivos := []ArchivoParaMapa{
		{Ruta: "a.go", RutaAbsoluta: filepath.Join(dir, "a.go"), Lenguaje: "go", Importancia: 0.5},
		{Ruta: "b.go", RutaAbsoluta: filepath.Join(dir, "b.go"), Lenguaje: "go", Importancia: 1.0},
	}

	mapa := gen.Generar("test", archivos, 5000)
	if mapa.Entradas[0].Ruta != "b.go" {
		t.Errorf("top debería ser b.go (más importante), got %s", mapa.Entradas[0].Ruta)
	}
	if mapa.Entradas[1].Ruta != "a.go" {
		t.Errorf("segundo debería ser a.go, got %s", mapa.Entradas[1].Ruta)
	}
}

func TestGenerar_TruncadoPorPresupuesto(t *testing.T) {
	dir := t.TempDir()
	crearArchivoGoTest(t, dir, "a.go", "package a\nfunc AAA() {}\nfunc BBB() {}\nfunc CCC() {}\n")
	crearArchivoGoTest(t, dir, "b.go", "package b\nfunc DDD() {}\nfunc EEE() {}\nfunc FFF() {}\n")

	gen := NuevoGenerador()
	archivos := []ArchivoParaMapa{
		{Ruta: "a.go", RutaAbsoluta: filepath.Join(dir, "a.go"), Lenguaje: "go", Importancia: 1.0},
		{Ruta: "b.go", RutaAbsoluta: filepath.Join(dir, "b.go"), Lenguaje: "go", Importancia: 0.5},
	}

	// Presupuesto muy pequeño (10 tokens): solo cabe ~1 archivo
	mapa := gen.Generar("test", archivos, 10)
	if !mapa.Truncado {
		t.Error("mapa debería estar truncado")
	}
	// Con 10 tokens es probable que ni siquiera 1 archivo quepa
	if mapa.ArchivosIncluidos > 1 {
		t.Errorf("debería incluir 0-1 archivos, got %d", mapa.ArchivosIncluidos)
	}
}

func TestFormatoTexto_Legible(t *testing.T) {
	dir := t.TempDir()
	contenido := `package auth

func GenerateToken(userID string) (string, error) {
        return "", nil
}

type Claims struct {
        UserID string
}
`
	ruta := crearArchivoGoTest(t, dir, "auth.go", contenido)

	gen := NuevoGenerador()
	archivos := []ArchivoParaMapa{
		{Ruta: "auth.go", RutaAbsoluta: ruta, Lenguaje: "go", Importancia: 1.0},
	}

	mapa := gen.Generar("test", archivos, 5000)
	texto := mapa.FormatoTexto()

	if !strings.Contains(texto, "auth.go:") {
		t.Errorf("debería contener la ruta: %s", texto)
	}
	if !strings.Contains(texto, "GenerateToken") {
		t.Errorf("debería contener el símbolo: %s", texto)
	}
	if !strings.Contains(texto, "func GenerateToken(userID string) (string, error)") {
		t.Errorf("debería contener la firma completa: %s", texto)
	}
}

func TestFormatoMarkdown_Legible(t *testing.T) {
	dir := t.TempDir()
	contenido := `package auth
func GenerateToken(userID string) (string, error) { return "", nil }
type Claims struct { UserID string }
`
	ruta := crearArchivoGoTest(t, dir, "auth.go", contenido)

	gen := NuevoGenerador()
	archivos := []ArchivoParaMapa{
		{Ruta: "auth.go", RutaAbsoluta: ruta, Lenguaje: "go", Importancia: 0.8},
	}

	mapa := gen.Generar("test-proj", archivos, 5000)
	md := mapa.FormatoMarkdown()

	if !strings.Contains(md, "# Repository Map") {
		t.Errorf("debería tener título: %s", md)
	}
	if !strings.Contains(md, "test-proj") {
		t.Errorf("debería tener el nombre del proyecto: %s", md)
	}
	if !strings.Contains(md, "`auth.go`") {
		t.Errorf("debería tener la ruta en code: %s", md)
	}
}

func TestEstimarTokensTexto(t *testing.T) {
	texto := "hola mundo este es un texto de prueba"
	tokens := EstimarTokensTexto(texto)
	// 35 chars / 4 = ~8 tokens
	if tokens == 0 {
		t.Error("debería retornar > 0 tokens")
	}
	if tokens != len(texto)/4 {
		t.Errorf("tokens = %d, esperado %d", tokens, len(texto)/4)
	}
}

func TestEstimarTokensEntrada(t *testing.T) {
	entrada := EntradaMapaRepo{
		Ruta: "auth.go",
		Simbolos: []SimboloCompacto{
			{Firma: "func GenerateToken(userID string) (string, error)"},
			{Firma: "type Claims struct { UserID string }"},
		},
	}
	tokens := estimarTokensEntrada(entrada)
	if tokens == 0 {
		t.Error("debería retornar > 0 tokens")
	}
}

func TestIconoTipo(t *testing.T) {
	tests := []struct {
		tipo     string
		esperado string
	}{
		{"funcion", "ƒ"},
		{"metodo", "↳"},
		{"estructura", "S"},
		{"interface", "I"},
		{"tipo", "T"},
		{"constante", "C"},
		{"variable", "V"},
		{"otro", "•"},
	}

	for _, tt := range tests {
		resultado := iconoTipo(tt.tipo)
		if resultado != tt.esperado {
			t.Errorf("iconoTipo(%q) = %q, esperado %q", tt.tipo, resultado, tt.esperado)
		}
	}
}

func TestGenerar_ArchivoInexistente(t *testing.T) {
	gen := NuevoGenerador()
	archivos := []ArchivoParaMapa{
		{Ruta: "no-existe.go", RutaAbsoluta: "/ruta/que/no/existe.go", Lenguaje: "go"},
	}

	// No debe panic; el parser retorna AST con TieneError=true pero sin símbolos
	mapa := gen.Generar("test", archivos, 5000)
	if mapa == nil {
		t.Fatal("mapa no debería ser nil")
	}
	// El archivo se incluye pero sin símbolos
	if len(mapa.Entradas) > 0 && len(mapa.Entradas[0].Simbolos) > 0 {
		t.Errorf("no debería tener símbolos para archivo inexistente, got %d",
			len(mapa.Entradas[0].Simbolos))
	}
}

func TestExtensionLenguaje(t *testing.T) {
	if ExtensionLenguaje("go") != ".go" {
		t.Error("go debería mapear a .go")
	}
	if ExtensionLenguaje("python") != ".py" {
		t.Error("python debería mapear a .py")
	}
	if ExtensionLenguaje("desconocido") != "" {
		t.Error("desconocido debería mapear a ''")
	}
}

func TestNombreBase(t *testing.T) {
	if NombreBase("src/auth/jwt.go") != "jwt.go" {
		t.Error("NombreBase incorrecto")
	}
	if NombreBase("main.go") != "main.go" {
		t.Error("NombreBase incorrecto")
	}
}
