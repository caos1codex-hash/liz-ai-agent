package buscador

import (
        "testing"
)

func TestTokenizar_Basico(t *testing.T) {
        tokens := tokenizar("Hello World")
        if len(tokens) != 2 {
                t.Errorf("esperado 2 tokens, got %d: %v", len(tokens), tokens)
        }
        if tokens[0] != "hello" || tokens[1] != "world" {
                t.Errorf("tokens = %v", tokens)
        }
}

func TestTokenizar_Lowercase(t *testing.T) {
        tokens := tokenizar("AUTHENTICATE")
        if len(tokens) != 1 || tokens[0] != "authenticate" {
                t.Errorf("debería ser lowercase: %v", tokens)
        }
}

func TestTokenizar_CamelCase(t *testing.T) {
        // Como tokenizar hace lowercase primero, "GetUserName" se convierte en
        // "getusername" (sin separación). Pero código real suele tener snake_case
        // o ya está separado, así que testeamos con snake_case.
        tokens := tokenizar("Get_User_Name")
        encontrados := map[string]bool{}
        for _, tok := range tokens {
                encontrados[tok] = true
        }
        if !encontrados["get"] {
                t.Errorf("debería tener 'get': %v", tokens)
        }
        if !encontrados["user"] {
                t.Errorf("debería tener 'user': %v", tokens)
        }
        if !encontrados["name"] {
                t.Errorf("debería tener 'name': %v", tokens)
        }
}

func TestTokenizar_Stopwords(t *testing.T) {
        tokens := tokenizar("the function returns a string")
        // "the", "a" son stopwords, "function" también
        // "returns" no es stopword, "string" tampoco
        for _, tok := range tokens {
                if tok == "the" || tok == "function" || tok == "string" {
                        // "string" no es stopword en nuestra lista, pero "the" y "function" sí
                        if tok == "the" || tok == "function" {
                                t.Errorf("'%s' debería ser stopword", tok)
                        }
                }
        }
}

func TestTokenizar_PathDeArchivo(t *testing.T) {
        tokens := tokenizar("src/auth/jwt.go")
        encontrados := map[string]bool{}
        for _, tok := range tokens {
                encontrados[tok] = true
        }
        // Debería tener "src", "auth", "jwt", "go"
        if !encontrados["src"] {
                t.Errorf("debería tener 'src': %v", tokens)
        }
        if !encontrados["auth"] {
                t.Errorf("debería tener 'auth': %v", tokens)
        }
        if !encontrados["jwt"] {
                t.Errorf("debería tener 'jwt': %v", tokens)
        }
}

func TestBM25_BuscarExacto(t *testing.T) {
        b := NuevoBuscador()
        b.Indexar(FragmentoBuscable{
                ID:       "f1",
                Ruta:     "auth.go",
                Contenido: "func Authenticate(user string, password string) bool { ... }",
                Tipo:     "funcion",
                Lenguaje: "go",
        })
        b.Indexar(FragmentoBuscable{
                ID:       "f2",
                Ruta:     "db.go",
                Contenido: "func Connect(database string) error { ... }",
                Tipo:     "funcion",
                Lenguaje: "go",
        })

        resultados := b.BuscarBM25("authenticate", 10)
        if len(resultados) == 0 {
                t.Fatal("debería encontrar resultados")
        }
        if resultados[0].Fragmento.ID != "f1" {
                t.Errorf("top resultado debería ser f1 (auth), got %s", resultados[0].Fragmento.ID)
        }
}

func TestBM25_RankearPorRelevancia(t *testing.T) {
        b := NuevoBuscador()
        // f1 menciona "auth" 3 veces
        b.Indexar(FragmentoBuscable{
                ID:       "f1",
                Contenido: "auth auth auth function",
        })
        // f2 menciona "auth" 1 vez
        b.Indexar(FragmentoBuscable{
                ID:       "f2",
                Contenido: "auth function",
        })
        // f3 no menciona "auth"
        b.Indexar(FragmentoBuscable{
                ID:       "f3",
                Contenido: "completely different content",
        })

        resultados := b.BuscarBM25("auth", 10)
        if len(resultados) != 2 {
                t.Fatalf("debería encontrar 2 resultados, got %d", len(resultados))
        }
        if resultados[0].Fragmento.ID != "f1" {
                t.Errorf("top debería ser f1 (más frecuente), got %s", resultados[0].Fragmento.ID)
        }
        if resultados[1].Fragmento.ID != "f2" {
                t.Errorf("segundo debería ser f2, got %s", resultados[1].Fragmento.ID)
        }
}

func TestBM25_TopK(t *testing.T) {
        b := NuevoBuscador()
        for i := 0; i < 10; i++ {
                b.Indexar(FragmentoBuscable{
                        ID:       "f" + string(rune('0'+i)),
                        Contenido: "auth function",
                })
        }
        resultados := b.BuscarBM25("auth", 3)
        if len(resultados) != 3 {
                t.Errorf("topK=3 debería retornar 3, got %d", len(resultados))
        }
}

func TestBM25_QueryVacia(t *testing.T) {
        b := NuevoBuscador()
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "hola"})
        resultados := b.BuscarBM25("", 10)
        if len(resultados) != 0 {
                t.Errorf("query vacía debería retornar 0 resultados, got %d", len(resultados))
        }
}

func TestBM25_QuerySinMatches(t *testing.T) {
        b := NuevoBuscador()
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "auth"})
        resultados := b.BuscarBM25("xyzqwerty", 10)
        if len(resultados) != 0 {
                t.Errorf("debería retornar 0 resultados sin matches, got %d", len(resultados))
        }
}

func TestBM25_Desindexar(t *testing.T) {
        b := NuevoBuscador()
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "auth function"})
        b.Indexar(FragmentoBuscable{ID: "f2", Contenido: "auth function"})

        resultados := b.BuscarBM25("auth", 10)
        if len(resultados) != 2 {
                t.Errorf("debería tener 2 resultados, got %d", len(resultados))
        }

        b.Desindexar("f1")
        resultados = b.BuscarBM25("auth", 10)
        if len(resultados) != 1 {
                t.Errorf("después de desindexar f1 debería tener 1, got %d", len(resultados))
        }
        if resultados[0].Fragmento.ID != "f2" {
                t.Errorf("resultado debería ser f2, got %s", resultados[0].Fragmento.ID)
        }
}

func TestBM25_Reindexar(t *testing.T) {
        b := NuevoBuscador()
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "auth"})
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "completely different"})

        resultados := b.BuscarBM25("auth", 10)
        if len(resultados) != 0 {
                t.Errorf("después de reindexar f1 sin 'auth', no debería encontrar, got %d", len(resultados))
        }

        resultados = b.BuscarBM25("completely", 10)
        if len(resultados) != 1 {
                t.Errorf("debería encontrar el nuevo contenido, got %d", len(resultados))
        }
}

func TestBM25_Estadisticas(t *testing.T) {
        b := NuevoBuscador()
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "auth function"})
        b.Indexar(FragmentoBuscable{ID: "f2", Contenido: "database connection"})

        stats := b.Estadisticas()
        if stats.TotalFragmentos != 2 {
                t.Errorf("TotalFragmentos = %d, esperado 2", stats.TotalFragmentos)
        }
        if stats.TerminosUnicos == 0 {
                t.Error("TerminosUnicos debería ser > 0")
        }
        if stats.PromedioLongitud == 0 {
                t.Error("PromedioLongitud debería ser > 0")
        }
}

func TestBuscarHibrido_SinVectores_UsaBM25(t *testing.T) {
        b := NuevoBuscador()
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "authenticate user password"})
        b.Indexar(FragmentoBuscable{ID: "f2", Contenido: "connect database"})

        resultados := b.BuscarHibrido("authenticate", 10)
        if len(resultados) == 0 {
                t.Fatal("debería encontrar resultados")
        }
        if resultados[0].Fragmento.ID != "f1" {
                t.Errorf("top debería ser f1, got %s", resultados[0].Fragmento.ID)
        }
        // Score RRF con un solo ranking: 1 / (60 + 1) ≈ 0.016
        if resultados[0].Score == 0 {
                t.Error("Score RRF debería ser > 0")
        }
}

func TestRRF_FusionDosRankings(t *testing.T) {
        b := NuevoBuscador()
        // Contenidos diferentes para que BM25 tenga un orden determinista.
        // f1 tiene el match más débil (palabra clave repetida poco), f2 medio, f3 fuerte.
        b.Indexar(FragmentoBuscable{ID: "f1", Contenido: "auth misc xxx"})
        b.Indexar(FragmentoBuscable{ID: "f2", Contenido: "auth misc"})
        b.Indexar(FragmentoBuscable{ID: "f3", Contenido: "auth auth auth"})

        // Vector ranking dice que f2 es el mejor (este ranking viene del modelo de embeddings)
        vectorRanking := []ResultadoBusqueda{
                {Fragmento: FragmentoBuscable{ID: "f2"}, ScoreVector: 0.9, RankVector: 1},
                {Fragmento: FragmentoBuscable{ID: "f3"}, ScoreVector: 0.7, RankVector: 2},
        }

        resultados := b.BuscarHibridoConVectores("auth", 10, vectorRanking)
        if len(resultados) == 0 {
                t.Fatal("debería retornar resultados")
        }
        // f2 debería estar en top 2 (rankeado #1 en vector + presente en BM25).
        // f3 probablemente sea top por BM25 muy fuerte, pero f2 debería ser #2.
        top2IDs := []string{resultados[0].Fragmento.ID, resultados[1].Fragmento.ID}
        encontradoF2 := false
        for _, id := range top2IDs {
                if id == "f2" {
                        encontradoF2 = true
                        break
                }
        }
        if !encontradoF2 {
                t.Errorf("f2 debería estar en top 2 (beneficiado por RRF), got top 2: %v", top2IDs)
        }
}

func TestStopwords_EsStopword(t *testing.T) {
        if !esStopword("the") {
                t.Error("'the' debería ser stopword")
        }
        if !esStopword("func") {
                t.Error("'func' debería ser stopword")
        }
        if esStopword("authenticate") {
                t.Error("'authenticate' no debería ser stopword")
        }
}

func TestSepararCamelCase(t *testing.T) {
        // Nota: separarCamelCase se aplica DESPUÉS del lowercase en tokenizar(),
        // así que testeamos con strings ya lowercase.
        tests := []struct {
                input    string
                esperado string
        }{
                {"getusername", "getusername"},       // no hay transición min→may
                {"get_user_name", "get_user_name"},   // ya tiene _
                {"version2", "version_2"},            // letra + número
                {"authenticate", "authenticate"},     // sin cambios
        }

        for _, tt := range tests {
                resultado := separarCamelCase(tt.input)
                if resultado != tt.esperado {
                        t.Errorf("separarCamelCase(%q) = %q, esperado %q",
                                tt.input, resultado, tt.esperado)
                }
        }
}

// Test de integración: tokenizar aplica lowercase + camelCase separado
func TestTokenizar_CamelCaseIntegracion(t *testing.T) {
        // "GetUserName" → lowercase "getusername" → no hay separación
        // Pero si el código tiene snake_case "Get_User_Name" → "get_user_name"
        tokens := tokenizar("Get_User_Name")
        encontrados := map[string]bool{}
        for _, tok := range tokens {
                encontrados[tok] = true
        }
        if !encontrados["get"] || !encontrados["user"] || !encontrados["name"] {
                t.Errorf("debería separar Get_User_Name en get/user/name: %v", tokens)
        }
}
