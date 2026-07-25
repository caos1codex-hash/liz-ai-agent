package buscador

import (
        "fmt"
        "strings"
        "testing"
)

func TestNuevoRerankerLLM_SinLLM(t *testing.T) {
        r := NuevoRerankerLLM(nil)
        if r.TieneLLM() {
                t.Error("debería ser false sin llmFn")
        }
        if r.maxTopK != 10 {
                t.Errorf("default maxTopK debería ser 10, got %d", r.maxTopK)
        }
}

func TestRerank_SinLLM_Passthrough(t *testing.T) {
        r := NuevoRerankerLLM(nil)
        frag1 := crearFragmento("id1", "a.go", "func A() {}")
        frag2 := crearFragmento("id2", "b.go", "func B() {}")
        resultados := []ResultadoBusqueda{
                {Fragmento: frag1, Score: 0.8},
                {Fragmento: frag2, Score: 0.5},
        }

        rerankeados := r.Rerank("test query", resultados)
        if len(rerankeados) != 2 {
                t.Fatalf("esperados 2 resultados, got %d", len(rerankeados))
        }
        if rerankeados[0].Fragmento.ID != "id1" {
                t.Error("sin LLM, debería ser passthrough")
        }
}

func TestRerank_ConLLM_Reordena(t *testing.T) {
        // Simular LLM que retorna IDs en orden inverso
        llmFn := func(prompt string) (string, error) {
                // Verificar que el prompt contiene la query
                if !strings.Contains(prompt, "test query") {
                        t.Error("prompt debería contener la query")
                }
                return "id2\nid1", nil
        }

        r := NuevoRerankerLLM(llmFn)
        frag1 := crearFragmento("id1", "a.go", "func A() {}")
        frag2 := crearFragmento("id2", "b.go", "func B() {}")
        resultados := []ResultadoBusqueda{
                {Fragmento: frag1, Score: 0.8},
                {Fragmento: frag2, Score: 0.5},
        }

        rerankeados := r.Rerank("test query", resultados)
        if len(rerankeados) != 2 {
                t.Fatalf("esperados 2 resultados, got %d", len(rerankeados))
        }
        if rerankeados[0].Fragmento.ID != "id2" {
                t.Errorf("más relevante debería ser id2, got %s", rerankeados[0].Fragmento.ID)
        }
        if rerankeados[1].Fragmento.ID != "id1" {
                t.Errorf("segundo debería ser id1, got %s", rerankeados[1].Fragmento.ID)
        }
}

func TestRerank_LLMFalla_Gracioful(t *testing.T) {
        llmFn := func(prompt string) (string, error) {
                return "", fmt.Errorf("LLM no disponible")
        }

        r := NuevoRerankerLLM(llmFn)
        frag := crearFragmento("id1", "a.go", "func A() {}")
        resultados := []ResultadoBusqueda{{Fragmento: frag, Score: 0.8}}

        rerankeados := r.Rerank("test", resultados)
        if len(rerankeados) != 1 || rerankeados[0].Fragmento.ID != "id1" {
                t.Error("si LLM falla, debería retornar resultados originales")
        }
}

func TestRerank_LLMRetornaIDsInvalidos_Gracioful(t *testing.T) {
        llmFn := func(prompt string) (string, error) {
                return "no_existe\notro_invalido", nil
        }

        r := NuevoRerankerLLM(llmFn)
        frag := crearFragmento("id1", "a.go", "func A() {}")
        resultados := []ResultadoBusqueda{{Fragmento: frag, Score: 0.8}}

        rerankeados := r.Rerank("test", resultados)
        if len(rerankeados) != 1 {
                t.Errorf("con IDs inválidos, debería mantener originales, got %d", len(rerankeados))
        }
}

func TestRerank_MaxTopK(t *testing.T) {
        llmFn := func(prompt string) (string, error) {
                return "id3\nid2\nid1", nil
        }

        r := NuevoRerankerLLM(llmFn).ConMaxTopK(2)

        frag1 := crearFragmento("id1", "a.go", "func A() {}")
        frag2 := crearFragmento("id2", "b.go", "func B() {}")
        frag3 := crearFragmento("id3", "c.go", "func C() {}")
        resultados := []ResultadoBusqueda{
                {Fragmento: frag1, Score: 0.9},
                {Fragmento: frag2, Score: 0.7},
                {Fragmento: frag3, Score: 0.5},
        }

        rerankeados := r.Rerank("test", resultados)
        // Solo los primeros 2 se envían al LLM, id3 va al final
        if len(rerankeados) != 3 {
                t.Fatalf("esperados 3 resultados, got %d", len(rerankeados))
        }
}

func TestParsearRespuestaRerank_FormatoSimple(t *testing.T) {
        idToIndex := map[string]int{"abc": 0, "def": 1, "ghi": 2}
        respuesta := "abc\ndef\nghi"
        ids := parsearRespuestaRerank(respuesta, idToIndex)
        if len(ids) != 3 || ids[0] != "abc" || ids[2] != "ghi" {
                t.Errorf("formato simple incorrecto: %v", ids)
        }
}

func TestParsearRespuestaRerank_FormatoNumerado(t *testing.T) {
        idToIndex := map[string]int{"abc": 0, "def": 1}
        respuesta := "1) abc\n2) def"
        ids := parsearRespuestaRerank(respuesta, idToIndex)
        if len(ids) != 2 || ids[0] != "abc" || ids[1] != "def" {
                t.Errorf("formato numerado incorrecto: %v", ids)
        }
}

func TestTruncarContenido(t *testing.T) {
        corto := "hola"
        if truncarContenido(corto, 10) != corto {
                t.Error("contenido corto no debería truncarse")
        }
        largo := strings.Repeat("x", 200)
        truncado := truncarContenido(largo, 100)
        if len(truncado) != 103 { // 100 + "..."
                t.Errorf("esperados 103 chars, got %d", len(truncado))
        }
        if !strings.HasSuffix(truncado, "...") {
                t.Error("truncado debería terminar con '...'")
        }
}

// Helper para crear fragmentos de test
func crearFragmento(id, ruta, contenido string) FragmentoBuscable {
        return FragmentoBuscable{
                ID:        id,
                Ruta:      ruta,
                Contenido: contenido,
                Tipo:      "funcion",
                Lenguaje:  "go",
        }
}
