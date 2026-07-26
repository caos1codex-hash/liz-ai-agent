package grafo

import (
	"testing"
)

func TestGrafo_AgregarYObtener(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("main.go", "go", 100)
	g.AgregarArchivo("auth.go", "go", 50)

	nodo, ok := g.Obtener("main.go")
	if !ok {
		t.Fatal("debería existir main.go")
	}
	if nodo.Lenguaje != "go" {
		t.Errorf("Lenguaje = %q", nodo.Lenguaje)
	}
	if nodo.Lineas != 100 {
		t.Errorf("Lineas = %d", nodo.Lineas)
	}
}

func TestGrafo_AgregarImport(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("main.go", "go", 100)
	g.AgregarArchivo("auth.go", "go", 50)

	g.AgregarImport("main.go", "auth.go")

	// main.go ahora tiene 1 import
	nodo, _ := g.Obtener("main.go")
	if nodo.Imports != 1 {
		t.Errorf("main.go.Imports = %d, esperado 1", nodo.Imports)
	}

	// auth.go tiene 1 importador
	nodo, _ = g.Obtener("auth.go")
	if nodo.Importadores != 1 {
		t.Errorf("auth.go.Importadores = %d, esperado 1", nodo.Importadores)
	}

	// No self-loops
	g.AgregarImport("main.go", "main.go")
	nodo, _ = g.Obtener("main.go")
	if nodo.Imports != 1 {
		t.Errorf("no debería aumentar imports por self-loop: %d", nodo.Imports)
	}

	// No duplicados
	g.AgregarImport("main.go", "auth.go")
	nodo, _ = g.Obtener("main.go")
	if nodo.Imports != 1 {
		t.Errorf("no debería duplicar arista: %d", nodo.Imports)
	}
}

func TestPageRank_ArchivoMasImportadoSube(t *testing.T) {
	// Estructura:
	//   a.go → b.go
	//   c.go → b.go
	//   d.go → b.go
	//   b.go es el más importado → mayor PageRank
	g := NuevoGrafo()
	g.AgregarArchivo("a.go", "go", 10)
	g.AgregarArchivo("b.go", "go", 10)
	g.AgregarArchivo("c.go", "go", 10)
	g.AgregarArchivo("d.go", "go", 10)

	g.AgregarImport("a.go", "b.go")
	g.AgregarImport("c.go", "b.go")
	g.AgregarImport("d.go", "b.go")

	g.CalcularImportancia(50, 0.85)

	nodoB, _ := g.Obtener("b.go")
	nodoA, _ := g.Obtener("a.go")

	if nodoB.Importancia <= nodoA.Importancia {
		t.Errorf("b.go debería ser más importante que a.go: %.3f vs %.3f",
			nodoB.Importancia, nodoA.Importancia)
	}
	// b.go debería tener score 1.0 (el máximo)
	if nodoB.Importancia != 1.0 {
		t.Errorf("b.go debería tener score 1.0, got %.3f", nodoB.Importancia)
	}
}

func TestPageRank_ArchivoAisladoBajaScore(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("hub.go", "go", 10)
	g.AgregarArchivo("a.go", "go", 10)
	g.AgregarArchivo("b.go", "go", 10)
	g.AgregarArchivo("aislado.go", "go", 10)

	g.AgregarImport("a.go", "hub.go")
	g.AgregarImport("b.go", "hub.go")

	g.CalcularImportancia(50, 0.85)

	nodoHub, _ := g.Obtener("hub.go")
	nodoAislado, _ := g.Obtener("aislado.go")

	if nodoAislado.Importancia >= nodoHub.Importancia {
		t.Errorf("aislado.go debería ser menos importante que hub.go: %.3f vs %.3f",
			nodoAislado.Importancia, nodoHub.Importancia)
	}
}

func TestPageRank_TopN(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("a.go", "go", 10)
	g.AgregarArchivo("b.go", "go", 10)
	g.AgregarArchivo("c.go", "go", 10)
	g.AgregarArchivo("d.go", "go", 10)

	// b.go es importado por a, c, d → más importante
	// c.go es importado por a → segundo
	g.AgregarImport("a.go", "b.go")
	g.AgregarImport("c.go", "b.go")
	g.AgregarImport("d.go", "b.go")
	g.AgregarImport("a.go", "c.go")

	g.CalcularImportancia(50, 0.85)

	top := g.TopN(2)
	if len(top) != 2 {
		t.Fatalf("TopN(2) = %d, esperado 2", len(top))
	}
	if top[0].Ruta != "b.go" {
		t.Errorf("top[0] debería ser b.go, got %s", top[0].Ruta)
	}
	if top[1].Ruta != "c.go" {
		t.Errorf("top[1] debería ser c.go, got %s", top[1].Ruta)
	}
}

func TestGrafo_RemoverArchivo(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("a.go", "go", 10)
	g.AgregarArchivo("b.go", "go", 10)
	g.AgregarArchivo("c.go", "go", 10)

	g.AgregarImport("a.go", "b.go")
	g.AgregarImport("c.go", "b.go")

	g.RemoverArchivo("b.go")

	if _, ok := g.Obtener("b.go"); ok {
		t.Error("b.go no debería existir después de remover")
	}
	// a.go y c.go ya no importan a nadie
	nodoA, _ := g.Obtener("a.go")
	if nodoA.Imports != 0 {
		t.Errorf("a.go.Imports = %d, esperado 0", nodoA.Imports)
	}
	nodoC, _ := g.Obtener("c.go")
	if nodoC.Imports != 0 {
		t.Errorf("c.go.Imports = %d, esperado 0", nodoC.Imports)
	}
}

func TestGrafo_VecinosYBacklinks(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("main.go", "go", 10)
	g.AgregarArchivo("auth.go", "go", 10)
	g.AgregarArchivo("db.go", "go", 10)

	g.AgregarImport("main.go", "auth.go")
	g.AgregarImport("main.go", "db.go")

	vecinos := g.Vecinos("main.go")
	if len(vecinos) != 2 {
		t.Errorf("main.go debería tener 2 vecinos, got %d", len(vecinos))
	}

	backlinks := g.Backlinks("auth.go")
	if len(backlinks) != 1 {
		t.Errorf("auth.go debería tener 1 backlink, got %d", len(backlinks))
	}
	if backlinks[0] != "main.go" {
		t.Errorf("backlink[0] = %s, esperado main.go", backlinks[0])
	}
}

func TestGrafo_Estadisticas(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("a.go", "go", 10)
	g.AgregarArchivo("b.go", "go", 10)
	g.AgregarArchivo("c.go", "go", 10)

	g.AgregarImport("a.go", "b.go")
	g.AgregarImport("a.go", "c.go")
	g.AgregarImport("c.go", "b.go")

	g.CalcularImportancia(50, 0.85)

	stats := g.Estadisticas()
	if stats.TotalArchivos != 3 {
		t.Errorf("TotalArchivos = %d, esperado 3", stats.TotalArchivos)
	}
	if stats.TotalAristas != 3 {
		t.Errorf("TotalAristas = %d, esperado 3", stats.TotalAristas)
	}
	if stats.ArchivoTop != "b.go" {
		t.Errorf("ArchivoTop = %s, esperado b.go", stats.ArchivoTop)
	}
}

func TestPageRank_GrafoVacio(t *testing.T) {
	g := NuevoGrafo()
	// No debe panic
	g.CalcularImportancia(50, 0.85)
	if g.TotalArchivos() != 0 {
		t.Error("grafo vacío debería seguir vacío después de PageRank")
	}
}

func TestPageRank_SinAristas(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarArchivo("a.go", "go", 10)
	g.AgregarArchivo("b.go", "go", 10)
	// Sin aristas

	g.CalcularImportancia(50, 0.85)

	// Todos los archivos deberían tener el mismo score
	nodoA, _ := g.Obtener("a.go")
	nodoB, _ := g.Obtener("b.go")
	if nodoA.Importancia != nodoB.Importancia {
		t.Errorf("sin aristas, todos deberían tener el mismo score: %.3f vs %.3f",
			nodoA.Importancia, nodoB.Importancia)
	}
}

func TestResolverImportGo(t *testing.T) {
	modulo := "github.com/caos1codex-hash/liz-ai-agent"

	tests := []struct {
		importPath string
		esperado   string
	}{
		{modulo + "/internal/auth", "internal/auth/"},
		{modulo + "/internal/nucleo/contexto", "internal/nucleo/contexto/"},
		{"github.com/otros/proyecto", ""}, // externo
		{"fmt", ""},                       // stdlib
	}

	for _, tt := range tests {
		resultado := ResolverImportGo(tt.importPath, modulo)
		if resultado != tt.esperado {
			t.Errorf("ResolverImportGo(%q) = %q, esperado %q",
				tt.importPath, resultado, tt.esperado)
		}
	}
}

func TestMatchImportArchivo(t *testing.T) {
	modulo := "github.com/caos1codex-hash/liz-ai-agent"
	importPath := modulo + "/internal/auth"

	if !MatchImportArchivo(importPath, modulo, "internal/auth/jwt.go") {
		t.Error("debería hacer match")
	}
	if !MatchImportArchivo(importPath, modulo, "internal/auth/oauth.go") {
		t.Error("debería hacer match con otro archivo del mismo paquete")
	}
	if MatchImportArchivo(importPath, modulo, "internal/db/postgres.go") {
		t.Error("no debería hacer match con paquete distinto")
	}
}

func TestNormalizarRutaGo(t *testing.T) {
	tests := []struct {
		ruta     string
		esperado string
	}{
		{"internal/auth/jwt.go", "internal/auth"},
		{"main.go", ""},
		{"src/server/handler.go", "src/server"},
	}

	for _, tt := range tests {
		resultado := NormalizarRutaGo(tt.ruta)
		if resultado != tt.esperado {
			t.Errorf("NormalizarRutaGo(%q) = %q, esperado %q",
				tt.ruta, resultado, tt.esperado)
		}
	}
}
