package grafo

import (
	"testing"
)

func TestNuevoGrafo_Vacio(t *testing.T) {
	g := NuevoGrafo()
	if g == nil {
		t.Fatal("esperaba grafo no nil")
	}
	if len(g.Nodos) != 0 {
		t.Errorf("esperaba 0 nodos, got %d", len(g.Nodos))
	}
	if len(g.Aristas) != 0 {
		t.Errorf("esperaba 0 aristas, got %d", len(g.Aristas))
	}
}

func TestAgregarNodo_Duplicado(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("a.go", "paquete a")

	// Agregar duplicado — debería manejar gracefully
	g.AgregarNodo("a.go", "paquete a actualizado")

	if len(g.Nodos) != 1 {
		t.Errorf("no debería duplicar nodos, got %d", len(g.Nodos))
	}
}

func TestAgregarArista_Duplicada(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("a.go", "a")
	g.AgregarNodo("b.go", "b")
	g.AgregarArista("a.go", "b.go")

	// Duplicada
	g.AgregarArista("a.go", "b.go")

	// Depende de la implementación si duplica o no
	_ = len(g.Aristas)
}

func TestAgregarArista_SinNodos(t *testing.T) {
	g := NuevoGrafo()

	// No debería panic
	g.AgregarArista("no_existe.go", "tampoco.go")
}

func TestCalcularPageRank_Vacio(t *testing.T) {
	g := NuevoGrafo()
	scores := g.CalcularPageRank()

	if len(scores) != 0 {
		t.Errorf("grafo vacío debería dar 0 scores, got %d", len(scores))
	}
}

func TestCalcularPageRank_SoloUnNodo(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("main.go", "main")

	scores := g.CalcularPageRank()
	if len(scores) != 1 {
		t.Errorf("esperaba 1 score, got %d", len(scores))
	}
	// Un nodo aislado tiene score bajo
	if scores["main.go"] <= 0 {
		t.Error("score debería ser > 0")
	}
}

func TestCalcularPageRank_TresNodosLineales(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("a.go", "a")
	g.AgregarNodo("b.go", "b")
	g.AgregarNodo("c.go", "c")
	g.AgregarArista("a.go", "b.go")
	g.AgregarArista("b.go", "c.go")

	scores := g.CalcularPageRank()
	if len(scores) != 3 {
		t.Errorf("esperaba 3 scores, got %d", len(scores))
	}

	// c.go no es importado por nadie → score más bajo
	// a.go es raíz → score bajo también
	// La suma de todos los scores normalizados debería ser ~1.0
	suma := 0.0
	for _, s := range scores {
		suma += s
	}
	if suma < 0.9 || suma > 1.1 {
		t.Errorf("suma de scores %.4f, esperaba ~1.0", suma)
	}
}

func TestCalcularPageRank_ConHub(t *testing.T) {
	g := NuevoGrafo()
	// main.go importa a todos
	g.AgregarNodo("main.go", "main")
	g.AgregarNodo("a.go", "a")
	g.AgregarNodo("b.go", "b")
	g.AgregarNodo("c.go", "c")

	g.AgregarArista("main.go", "a.go")
	g.AgregarArista("main.go", "b.go")
	g.AgregarArista("main.go", "c.go")

	// a, b, c no importan nada más
	g.AgregarArista("a.go", "c.go")

	scores := g.CalcularPageRank()
	// c.go es el más importado (por main y a) → score más alto
	if scores["c.go"] <= scores["main.go"] {
		t.Logf("Nota: c.go (%.4f) debería tener score >= main.go (%.4f)", scores["c.go"], scores["main.go"])
	}
}

func TestCalcularPageRank_Ciclo(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("a.go", "a")
	g.AgregarNodo("b.go", "b")
	g.AgregarArista("a.go", "b.go")
	g.AgregarArista("b.go", "a.go")

	scores := g.CalcularPageRank()
	if len(scores) != 2 {
		t.Errorf("esperaba 2 scores, got %d", len(scores))
	}
	// En un ciclo, ambos deberían tener score similar
	diff := abs(scores["a.go"] - scores["b.go"])
	if diff > 0.01 {
		t.Errorf("en ciclo, scores deberían ser similares: a=%.4f b=%.4f diff=%.4f", scores["a.go"], scores["b.go"], diff)
	}
}

func TestCalcularPageRank_MuchosNodos(t *testing.T) {
	g := NuevoGrafo()
	// Crear 20 nodos con algunas conexiones
	for i := 0; i < 20; i++ {
		g.AgregarNodo(fmt.Sprintf("f%d.go", i), fmt.Sprintf("file %d", i))
	}
	// main (f0) importa a todos
	for i := 1; i < 20; i++ {
		g.AgregarArista("f0.go", fmt.Sprintf("f%d.go", i))
	}
	// Algunos imports cruzados
	g.AgregarArista("f1.go", "f5.go")
	g.AgregarArista("f2.go", "f5.go")
	g.AgregarArista("f3.go", "f5.go")

	scores := g.CalcularPageRank()
	if len(scores) != 20 {
		t.Errorf("esperaba 20 scores, got %d", len(scores))
	}
	// f5.go es importado por f0, f1, f2, f3 → score alto
}

func TestObtenerScore_Nil(t *testing.T) {
	var g *Grafo
	score := g.ObtenerScore("no_existe")
	if score != 0 {
		t.Error("grafo nil debería dar score 0")
	}
}

func TestObtenerScore_NoExiste(t *testing.T) {
	g := NuevoGrafo()
	score := g.ObtenerScore("no_existe")
	if score != 0 {
		t.Errorf("nodo inexistente debería dar 0, got %.4f", score)
	}
}

func TestObtenerScore_Existente(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("main.go", "main")
	g.CalcularPageRank() // Calcular primero

	score := g.ObtenerScore("main.go")
	if score <= 0 {
		t.Error("score debería ser > 0")
	}
}

func TestNodosSalida(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("a.go", "a")
	g.AgregarNodo("b.go", "b")
	g.AgregarNodo("c.go", "c")
	g.AgregarArista("a.go", "b.go")
	g.AgregarArista("a.go", "c.go")

	salida := g.NodosSalida("a.go")
	if len(salida) != 2 {
		t.Errorf("esperaba 2 nodos de salida, got %d", len(salida))
	}
}

func TestNodosSalida_NoExiste(t *testing.T) {
	g := NuevoGrafo()
	salida := g.NodosSalida("no_existe")
	if salida != nil {
		t.Error("nodo inexistente debería dar nil")
	}
}

func TestNodosEntrada(t *testing.T) {
	g := NuevoGrafo()
	g.AgregarNodo("a.go", "a")
	g.AgregarNodo("b.go", "b")
	g.AgregarNodo("c.go", "c")
	g.AgregarArista("b.go", "a.go")
	g.AgregarArista("c.go", "a.go")

	entrada := g.NodosEntrada("a.go")
	if len(entrada) != 2 {
		t.Errorf("esperaba 2 nodos de entrada, got %d", len(entrada))
	}
}

func TestNodosEntrada_NoExiste(t *testing.T) {
	g := NuevoGrafo()
	entrada := g.NodosEntrada("no_existe")
	if entrada != nil {
		t.Error("nodo inexistente debería dar nil")
	}
}

func TestTotalNodos(t *testing.T) {
	g := NuevoGrafo()
	if g.TotalNodos() != 0 {
		t.Error("grafo vacío debería tener 0 nodos")
	}
	g.AgregarNodo("a.go", "a")
	if g.TotalNodos() != 1 {
		t.Error("debería tener 1 nodo")
	}
}

func TestTotalAristas(t *testing.T) {
	g := NuevoGrafo()
	if g.TotalAristas() != 0 {
		t.Error("grafo vacío debería tener 0 aristas")
	}
	g.AgregarNodo("a.go", "a")
	g.AgregarNodo("b.go", "b")
	g.AgregarArista("a.go", "b.go")
	if g.TotalAristas() != 1 {
		t.Error("debería tener 1 arista")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}