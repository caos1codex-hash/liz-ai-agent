package empaquetador

import (
        "strings"
        "testing"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/mapa_repo"
)

func TestEmpaquetar_SoloMapaRepo(t *testing.T) {
        e := NuevoEmpaquetador()

        // Mapa repo sin fragmentos ni query
        mapaRepo := &mapa_repo.MapaRepo{
                Proyecto:      "test",
                TotalArchivos: 1,
                Entradas: []mapa_repo.EntradaMapaRepo{
                        {
                                Ruta:    "auth.go",
                                Simbolos: []mapa_repo.SimboloCompacto{
                                        {Firma: "func Authenticate(user string) bool"},
                                },
                        },
                },
        }

        resultado := e.Empaquetar(
                SolicitudEmpaquetado{
                        Proyecto:          "test",
                        PresupuestoTokens: 5000,
                },
                DatosEmpaquetado{
                        MapaRepo: mapaRepo,
                },
        )

        if !resultado.MapaRepoIncluido {
                t.Error("MapaRepoIncluido debería ser true")
        }
        if resultado.TokensUsados == 0 {
                t.Error("TokensUsados debería ser > 0")
        }
        if resultado.Contenido == "" {
                t.Error("Contenido no debería estar vacío")
        }
}

func TestEmpaquetar_ConFragmentosRelevantes(t *testing.T) {
        e := NuevoEmpaquetador()

        b := buscador.NuevoBuscador()
        b.Indexar(buscador.FragmentoBuscable{
                ID:        "f1",
                Ruta:      "auth.go",
                Contenido: "func Authenticate(user string, password string) bool { ... }",
                Lenguaje:  "go",
        })
        b.Indexar(buscador.FragmentoBuscable{
                ID:        "f2",
                Ruta:      "db.go",
                Contenido: "func Connect(database string) error { ... }",
                Lenguaje:  "go",
        })

        mapaRepo := &mapa_repo.MapaRepo{
                Proyecto:      "test",
                TotalArchivos: 2,
                Entradas: []mapa_repo.EntradaMapaRepo{
                        {Ruta: "auth.go", Simbolos: []mapa_repo.SimboloCompacto{
                                {Firma: "func Authenticate(user string) bool"},
                        }},
                        {Ruta: "db.go", Simbolos: []mapa_repo.SimboloCompacto{
                                {Firma: "func Connect(database string) error"},
                        }},
                },
        }

        resultado := e.Empaquetar(
                SolicitudEmpaquetado{
                        Proyecto:          "test",
                        Query:             "authenticate user",
                        PresupuestoTokens: 5000,
                },
                DatosEmpaquetado{
                        MapaRepo: mapaRepo,
                        Buscador: b,
                },
        )

        if !resultado.MapaRepoIncluido {
                t.Error("MapaRepo debería estar incluido")
        }
        if len(resultado.FragmentosIncluidos) == 0 {
                t.Error("debería incluir al menos 1 fragmento")
        }
        // El primer fragmento incluido debería ser f1 (auth) por relevancia BM25
        encontradoAuth := false
        for _, f := range resultado.FragmentosIncluidos {
                if f.Ruta == "auth.go" {
                        encontradoAuth = true
                        break
                }
        }
        if !encontradoAuth {
                t.Error("debería incluir fragmento de auth.go")
        }
}

func TestEmpaquetar_QueryVacia_NoIncluyeFragmentos(t *testing.T) {
        e := NuevoEmpaquetador()

        b := buscador.NuevoBuscador()
        b.Indexar(buscador.FragmentoBuscable{ID: "f1", Contenido: "auth"})

        mapaRepo := &mapa_repo.MapaRepo{Proyecto: "test", TotalArchivos: 1}

        resultado := e.Empaquetar(
                SolicitudEmpaquetado{
                        Proyecto:          "test",
                        Query:             "", // sin query
                        PresupuestoTokens: 5000,
                },
                DatosEmpaquetado{
                        MapaRepo: mapaRepo,
                        Buscador: b,
                },
        )

        if len(resultado.FragmentosIncluidos) != 0 {
                t.Errorf("sin query no debería incluir fragmentos, got %d", len(resultado.FragmentosIncluidos))
        }
}

func TestEmpaquetar_RespetoPresupuesto(t *testing.T) {
        e := NuevoEmpaquetador()

        // Crear muchos fragmentos grandes
        b := buscador.NuevoBuscador()
        for i := 0; i < 20; i++ {
                b.Indexar(buscador.FragmentoBuscable{
                        ID:        string(rune('a' + i)),
                        Ruta:      "auth.go",
                        Contenido: "func Authenticate(user string, password string) bool { ... } large content",
                        Lenguaje:  "go",
                })
        }

        mapaRepo := &mapa_repo.MapaRepo{Proyecto: "test", TotalArchivos: 1}

        // Presupuesto muy pequeño
        resultado := e.Empaquetar(
                SolicitudEmpaquetado{
                        Proyecto:          "test",
                        Query:             "auth",
                        PresupuestoTokens: 200, // muy pequeño
                },
                DatosEmpaquetado{
                        MapaRepo: mapaRepo,
                        Buscador: b,
                },
        )

        // No debe exceder significativamente el presupuesto
        if resultado.TokensUsados > 300 { // margen
                t.Errorf("tokens usados (%d) excede presupuesto (%d) por demasiado",
                        resultado.TokensUsados, 200)
        }
}

func TestEmpaquetar_Defaults(t *testing.T) {
        e := NuevoEmpaquetador()

        // Sin especificar presupuesto ni profundidad: defaults aplicados
        resultado := e.Empaquetar(
                SolicitudEmpaquetado{
                        Proyecto: "test",
                        // PresupuestoTokens y ProfundidadImports vacíos
                },
                DatosEmpaquetado{},
        )

        if resultado.PresupuestoTokens != 8000 {
                t.Errorf("default presupuesto debería ser 8000, got %d", resultado.PresupuestoTokens)
        }
}

func TestTruncarATokens(t *testing.T) {
        // String de 1000 chars → 250 tokens aprox
        s := strings.Repeat("hola ", 200) // 1000 chars
        truncado := truncarATokens(s, 50) // 50 tokens = ~200 chars

        if len(truncado) > 250 {
                t.Errorf("truncado debería ser ~200 chars, got %d", len(truncado))
        }
        if !strings.Contains(truncado, "truncado") {
                t.Error("debería indicar que fue truncado")
        }
}

func TestTruncarATokens_YaCabe(t *testing.T) {
        s := "texto corto"
        resultado := truncarATokens(s, 100)
        if resultado != s {
                t.Errorf("texto corto no debería truncarse")
        }
}

func TestResumen(t *testing.T) {
        c := &ContextoEmpaquetado{
                TokensUsados:      500,
                PresupuestoTokens: 1000,
                MapaRepoIncluido:  true,
                FragmentosIncluidos: []FragmentoIncluido{
                        {ID: "f1"}, {ID: "f2"},
                },
        }

        resumen := c.Resumen()
        if resumen == "" {
                t.Error("resumen no debería estar vacío")
        }
}
