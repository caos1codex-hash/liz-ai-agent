package empaquetador

import (
	"strings"
	"testing"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/grafo"
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
				Ruta: "auth.go",
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

// ═══════════════════════════════════════════════════════
// TESTS NUEVOS — Capa 3 (imports expandidos)
// ═══════════════════════════════════════════════════════

func TestEmpaquetar_Capa3_ExpandeImports(t *testing.T) {
	e := NuevoEmpaquetador()

	// Buscador con un fragmento relevante en auth.go
	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate(user string) bool { ... }",
		Lenguaje:  "go",
	})

	// Grafo: auth.go → jwt.go (auth depende de jwt)
	g := grafo.NuevoGrafo()
	g.AgregarArchivo("auth.go", "go", 50)
	g.AgregarArchivo("jwt.go", "go", 30)
	g.AgregarImport("auth.go", "jwt.go")

	// Callback: jwt.go tiene un fragmento
	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		if ruta == "jwt.go" {
			return []buscador.FragmentoBuscable{{
				ID:        "f2",
				Ruta:      "jwt.go",
				Contenido: "func GenerateToken() string { ... }",
				Lenguaje:  "go",
			}}
		}
		return nil
	}

	mapaRepo := &mapa_repo.MapaRepo{Proyecto: "test", TotalArchivos: 2}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:           "test",
			Query:              "authenticate",
			PresupuestoTokens:  5000,
			ProfundidadImports: 1,
		},
		DatosEmpaquetado{
			MapaRepo:                 mapaRepo,
			Buscador:                 b,
			Grafo:                    g,
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// Verificar que se incluyó el fragmento de jwt.go como "import"
	encontradoImport := false
	for _, f := range resultado.FragmentosIncluidos {
		if f.Tipo == "import" && f.Ruta == "jwt.go" {
			encontradoImport = true
			break
		}
	}
	if !encontradoImport {
		t.Errorf("debería incluir fragmento de jwt.go como 'import', fragmentos: %+v",
			resultado.FragmentosIncluidos)
	}

	// Verificar que se usaron tokens de imports
	if resultado.TokensImports == 0 {
		t.Error("TokensImports debería ser > 0")
	}
}

func TestEmpaquetar_Capa3_ProfundidadCero_NoExpande(t *testing.T) {
	e := NuevoEmpaquetador()

	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate() bool { ... }",
		Lenguaje:  "go",
	})

	g := grafo.NuevoGrafo()
	g.AgregarArchivo("auth.go", "go", 50)
	g.AgregarArchivo("jwt.go", "go", 30)
	g.AgregarImport("auth.go", "jwt.go")

	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		if ruta == "jwt.go" {
			return []buscador.FragmentoBuscable{{ID: "f2", Ruta: "jwt.go", Contenido: "func GenerateToken() string { ... }"}}
		}
		return nil
	}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:           "test",
			Query:              "authenticate",
			PresupuestoTokens:  5000,
			ProfundidadImports: 0, // sin expansión
		},
		DatosEmpaquetado{
			MapaRepo:                 &mapa_repo.MapaRepo{Proyecto: "test"},
			Buscador:                 b,
			Grafo:                    g,
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// No debería haber fragmentos tipo "import"
	for _, f := range resultado.FragmentosIncluidos {
		if f.Tipo == "import" {
			t.Errorf("con ProfundidadImports=0 no debería expandir imports, got %+v", f)
		}
	}
	if resultado.TokensImports != 0 {
		t.Errorf("TokensImports debería ser 0, got %d", resultado.TokensImports)
	}
}

func TestEmpaquetar_Capa3_Profundidad2_ExpandeTransitivamente(t *testing.T) {
	e := NuevoEmpaquetador()

	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "main.go",
		Contenido: "func main() { auth.Authenticate() }",
		Lenguaje:  "go",
	})

	// main → auth → jwt → crypto
	g := grafo.NuevoGrafo()
	g.AgregarArchivo("main.go", "go", 50)
	g.AgregarArchivo("auth.go", "go", 50)
	g.AgregarArchivo("jwt.go", "go", 30)
	g.AgregarArchivo("crypto.go", "go", 20)
	g.AgregarImport("main.go", "auth.go")
	g.AgregarImport("auth.go", "jwt.go")
	g.AgregarImport("jwt.go", "crypto.go")

	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		switch ruta {
		case "auth.go":
			return []buscador.FragmentoBuscable{{ID: "a1", Ruta: "auth.go", Contenido: "func Authenticate() bool { ... }", Lenguaje: "go"}}
		case "jwt.go":
			return []buscador.FragmentoBuscable{{ID: "j1", Ruta: "jwt.go", Contenido: "func GenerateToken() string { ... }", Lenguaje: "go"}}
		case "crypto.go":
			return []buscador.FragmentoBuscable{{ID: "c1", Ruta: "crypto.go", Contenido: "func Hash(s string) []byte { ... }", Lenguaje: "go"}}
		}
		return nil
	}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:           "test",
			Query:              "main",
			PresupuestoTokens:  5000,
			ProfundidadImports: 2, // 2 niveles: auth (1) + jwt (2)
		},
		DatosEmpaquetado{
			MapaRepo:                 &mapa_repo.MapaRepo{Proyecto: "test"},
			Buscador:                 b,
			Grafo:                    g,
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// Debería incluir auth.go (nivel 1) y jwt.go (nivel 2), pero NO crypto.go (nivel 3)
	rutasIncluidas := make(map[string]bool)
	for _, f := range resultado.FragmentosIncluidos {
		rutasIncluidas[f.Ruta] = true
	}
	if !rutasIncluidas["auth.go"] {
		t.Error("debería incluir auth.go (nivel 1)")
	}
	if !rutasIncluidas["jwt.go"] {
		t.Error("debería incluir jwt.go (nivel 2)")
	}
	if rutasIncluidas["crypto.go"] {
		t.Error("NO debería incluir crypto.go (nivel 3, fuera de profundidad=2)")
	}
}

func TestEmpaquetar_Capa3_SinGrafo_NoExpande(t *testing.T) {
	e := NuevoEmpaquetador()

	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate() bool { ... }",
	})

	// Sin grafo (nil)
	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		return []buscador.FragmentoBuscable{{ID: "x", Ruta: "x.go", Contenido: "..."}}
	}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:           "test",
			Query:              "auth",
			PresupuestoTokens:  5000,
			ProfundidadImports: 1,
		},
		DatosEmpaquetado{
			MapaRepo:                 &mapa_repo.MapaRepo{Proyecto: "test"},
			Buscador:                 b,
			Grafo:                    nil, // sin grafo
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// Sin grafo, no debería expandir imports
	for _, f := range resultado.FragmentosIncluidos {
		if f.Tipo == "import" {
			t.Errorf("sin grafo no debería expandir imports, got %+v", f)
		}
	}
	if resultado.TokensImports != 0 {
		t.Errorf("TokensImports debería ser 0 sin grafo, got %d", resultado.TokensImports)
	}
}

// ═══════════════════════════════════════════════════════
// TESTS NUEVOS — Capa 4 (archivos recientes)
// ═══════════════════════════════════════════════════════

func TestEmpaquetar_Capa4_IncluyeArchivosRecientes(t *testing.T) {
	e := NuevoEmpaquetador()

	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate() bool { ... }",
		Lenguaje:  "go",
	})

	// Archivos recientes: config.go tiene un fragmento
	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		if ruta == "config.go" {
			return []buscador.FragmentoBuscable{{
				ID:        "f2",
				Ruta:      "config.go",
				Contenido: "func LoadConfig() *Config { ... }",
				Lenguaje:  "go",
			}}
		}
		return nil
	}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:          "test",
			Query:             "auth",
			PresupuestoTokens: 5000,
			ArchivosRecientes: []string{"config.go"},
		},
		DatosEmpaquetado{
			MapaRepo:                 &mapa_repo.MapaRepo{Proyecto: "test"},
			Buscador:                 b,
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// Debería incluir config.go como "reciente"
	encontradoReciente := false
	for _, f := range resultado.FragmentosIncluidos {
		if f.Tipo == "reciente" && f.Ruta == "config.go" {
			encontradoReciente = true
			break
		}
	}
	if !encontradoReciente {
		t.Errorf("debería incluir config.go como 'reciente', fragmentos: %+v",
			resultado.FragmentosIncluidos)
	}
	if resultado.TokensRecientes == 0 {
		t.Error("TokensRecientes debería ser > 0")
	}
}

func TestEmpaquetar_Capa4_ArchivoRecenteSinFragmentos_SeMencionaRuta(t *testing.T) {
	e := NuevoEmpaquetador()

	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate() bool { ... }",
	})

	// Callback retorna nil para el archivo reciente
	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		return nil
	}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:          "test",
			Query:             "auth",
			PresupuestoTokens: 5000,
			ArchivosRecientes: []string{"unknown.go"},
		},
		DatosEmpaquetado{
			MapaRepo:                 &mapa_repo.MapaRepo{Proyecto: "test"},
			Buscador:                 b,
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// Debería mencionar la ruta aunque no haya fragmentos
	if !strings.Contains(resultado.Contenido, "unknown.go") {
		t.Error("debería mencionar la ruta unknown.go en el contenido")
	}
}

func TestEmpaquetar_Capa4_ArchivoYaIncluido_NoSeDuplica(t *testing.T) {
	e := NuevoEmpaquetador()

	// auth.go tiene un fragmento en el buscador y también está en archivos recientes
	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate() bool { ... }",
		Lenguaje:  "go",
	})

	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		if ruta == "auth.go" {
			return []buscador.FragmentoBuscable{{
				ID:        "f1", // mismo ID que en el buscador
				Ruta:      "auth.go",
				Contenido: "func Authenticate() bool { ... }",
				Lenguaje:  "go",
			}}
		}
		return nil
	}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:          "test",
			Query:             "auth",
			PresupuestoTokens: 5000,
			ArchivosRecientes: []string{"auth.go"},
		},
		DatosEmpaquetado{
			MapaRepo:                 &mapa_repo.MapaRepo{Proyecto: "test"},
			Buscador:                 b,
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// No debería duplicarse: solo 1 inclusión de f1 como "relevante"
	contadorF1 := 0
	for _, f := range resultado.FragmentosIncluidos {
		if f.ID == "f1" {
			contadorF1++
		}
	}
	if contadorF1 != 1 {
		t.Errorf("f1 debería incluirse solo 1 vez, got %d", contadorF1)
	}
}

func TestEmpaquetar_Capa4_SinArchivosRecientes_SeOmite(t *testing.T) {
	e := NuevoEmpaquetador()

	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate() bool { ... }",
	})

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:          "test",
			Query:             "auth",
			PresupuestoTokens: 5000,
			// sin ArchivosRecientes
		},
		DatosEmpaquetado{
			MapaRepo:                 &mapa_repo.MapaRepo{Proyecto: "test"},
			Buscador:                 b,
			ObtenerFragmentosPorRuta: func(string) []buscador.FragmentoBuscable { return nil },
		},
	)

	if resultado.TokensRecientes != 0 {
		t.Errorf("sin archivos recientes, TokensRecientes debería ser 0, got %d", resultado.TokensRecientes)
	}
	if strings.Contains(resultado.Contenido, "Archivos recientemente editados") {
		t.Error("no debería imprimir la sección de archivos recientes si no hay")
	}
}

// ═══════════════════════════════════════════════════════
// TESTS DE INTEGRACIÓN
// ═══════════════════════════════════════════════════════

func TestEmpaquetar_Integracion_TodasLasCapas(t *testing.T) {
	e := NuevoEmpaquetador()

	// Buscador con fragmentos relevantes
	b := buscador.NuevoBuscador()
	b.Indexar(buscador.FragmentoBuscable{
		ID:        "f1",
		Ruta:      "auth.go",
		Contenido: "func Authenticate(user string) bool { ... }",
		Lenguaje:  "go",
	})

	// Grafo: auth → jwt
	g := grafo.NuevoGrafo()
	g.AgregarArchivo("auth.go", "go", 50)
	g.AgregarArchivo("jwt.go", "go", 30)
	g.AgregarArchivo("logger.go", "go", 20)
	g.AgregarImport("auth.go", "jwt.go")

	// Callbacks
	obtenerPorRuta := func(ruta string) []buscador.FragmentoBuscable {
		switch ruta {
		case "jwt.go":
			return []buscador.FragmentoBuscable{{ID: "f2", Ruta: "jwt.go", Contenido: "func GenerateToken() string { ... }", Lenguaje: "go"}}
		case "logger.go":
			return []buscador.FragmentoBuscable{{ID: "f3", Ruta: "logger.go", Contenido: "func Log(msg string) { ... }", Lenguaje: "go"}}
		}
		return nil
	}

	mapaRepo := &mapa_repo.MapaRepo{
		Proyecto:      "test",
		TotalArchivos: 3,
		Entradas: []mapa_repo.EntradaMapaRepo{
			{Ruta: "auth.go", Simbolos: []mapa_repo.SimboloCompacto{
				{Firma: "func Authenticate(user string) bool"},
			}},
			{Ruta: "jwt.go", Simbolos: []mapa_repo.SimboloCompacto{
				{Firma: "func GenerateToken() string"},
			}},
			{Ruta: "logger.go", Simbolos: []mapa_repo.SimboloCompacto{
				{Firma: "func Log(msg string)"},
			}},
		},
	}

	resultado := e.Empaquetar(
		SolicitudEmpaquetado{
			Proyecto:           "test",
			Query:              "authenticate user",
			PresupuestoTokens:  8000,
			ProfundidadImports: 1,
			ArchivosRecientes:  []string{"logger.go"},
		},
		DatosEmpaquetado{
			MapaRepo:                 mapaRepo,
			Buscador:                 b,
			Grafo:                    g,
			ObtenerFragmentosPorRuta: obtenerPorRuta,
		},
	)

	// Verificar que se incluyeron las 4 capas
	if !resultado.MapaRepoIncluido {
		t.Error("Capa 1 (mapa repo) debería estar incluida")
	}
	if resultado.TokensMapaRepo == 0 {
		t.Error("Capa 1: TokensMapaRepo debería ser > 0")
	}
	if resultado.TokensFragmentos == 0 {
		t.Error("Capa 2: TokensFragmentos debería ser > 0")
	}
	if resultado.TokensImports == 0 {
		t.Error("Capa 3: TokensImports debería ser > 0 (jwt.go expandido)")
	}
	if resultado.TokensRecientes == 0 {
		t.Error("Capa 4: TokensRecientes debería ser > 0 (logger.go)")
	}

	// Verificar que el contenido tiene las 4 secciones
	if !strings.Contains(resultado.Contenido, "# Repository Map") {
		t.Error("falta sección Repository Map")
	}
	if !strings.Contains(resultado.Contenido, "# Fragmentos relevantes") {
		t.Error("falta sección Fragmentos relevantes")
	}
	if !strings.Contains(resultado.Contenido, "# Dependencias directas") {
		t.Error("falta sección Dependencias directas")
	}
	if !strings.Contains(resultado.Contenido, "# Archivos recientemente editados") {
		t.Error("falta sección Archivos recientemente editados")
	}
}
