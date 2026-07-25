package buscador

import (
        "errors"
        "fmt"
        "math"
        "sync"
        "testing"
)

// ═══════════════════════════════════════════════════════
// PROVIDER MOCK
// ═══════════════════════════════════════════════════════

// mockProvider es un EmbeddingsProvider de prueba que genera vectores
// deterministas a partir del texto (sin llamar a ninguna API).
type mockProvider struct {
        dimensiones int
        failOnCall  bool
}

func (m *mockProvider) GenerarEmbeddings(textos []string) ([][]float32, error) {
        if m.failOnCall {
                return nil, errors.New("mock error")
        }
        resultado := make([][]float32, len(textos))
        for i, texto := range textos {
                vec := make([]float32, m.dimensiones)
                // Generar vector determinista basado en el hash del texto
                // (cada caracter contribuye a una dimensión)
                for j, c := range texto {
                        dim := j % m.dimensiones
                        vec[dim] += float32(c) / 100.0
                }
                // Normalizar a norma 1 (para que similitud coseno sea estable)
                norm := 0.0
                for _, v := range vec {
                        norm += float64(v) * float64(v)
                }
                if norm > 0 {
                        sqrtNorm := math.Sqrt(norm)
                        for i := range vec {
                                vec[i] = float32(float64(vec[i]) / sqrtNorm)
                        }
                }
                resultado[i] = vec
        }
        return resultado, nil
}

func (m *mockProvider) Dimensiones() int { return m.dimensiones }

// ═══════════════════════════════════════════════════════
// TESTS BUSCADOR EMBEDDINGS
// ═══════════════════════════════════════════════════════

func TestBuscadorEmbeddings_SinProvider_SeComportaComoBuscadorBM25(t *testing.T) {
        be := NuevoBuscadorEmbeddings(nil)

        if be.TieneProvider() {
                t.Error("sin provider, TieneProvider debería ser false")
        }

        be.Indexar(FragmentoBuscable{ID: "f1", Contenido: "hola mundo"})
        be.Indexar(FragmentoBuscable{ID: "f2", Contenido: "hola liz"})

        // Búsqueda BM25 debería funcionar
        resultados := be.BuscarBM25("hola", 10)
        if len(resultados) != 2 {
                t.Errorf("debería encontrar 2, got %d", len(resultados))
        }
}

func TestBuscadorEmbeddings_IndexarConEmbeddings_Exitoso(t *testing.T) {
        provider := &mockProvider{dimensiones: 8}
        be := NuevoBuscadorEmbeddings(provider)

        err := be.IndexarConEmbeddings(FragmentoBuscable{
                ID:        "f1",
                Contenido: "funcion de autenticacion",
        })
        if err != nil {
                t.Fatalf("error: %v", err)
        }

        if be.TotalEmbeddings() != 1 {
                t.Errorf("debería tener 1 embedding, got %d", be.TotalEmbeddings())
        }

        // También debería estar en BM25
        if be.Total() != 1 {
                t.Errorf("debería tener 1 en BM25, got %d", be.Total())
        }
}

func TestBuscadorEmbeddings_IndexarConEmbeddings_SinProvider_Error(t *testing.T) {
        be := NuevoBuscadorEmbeddings(nil)

        err := be.IndexarConEmbeddings(FragmentoBuscable{ID: "f1", Contenido: "test"})
        if err == nil {
                t.Error("sin provider debería retornar error")
        }
        if err != ErrProviderNoConfigurado {
                t.Errorf("debería ser ErrProviderNoConfigurado, got %v", err)
        }

        // Pero debería seguir indexado en BM25
        if be.Total() != 1 {
                t.Errorf("debería estar en BM25 incluso si falla embeddings, got %d", be.Total())
        }
}

func TestBuscadorEmbeddings_IndexarConEmbeddings_ProviderFalla_Graceful(t *testing.T) {
        provider := &mockProvider{dimensiones: 4, failOnCall: true}
        be := NuevoBuscadorEmbeddings(provider)

        err := be.IndexarConEmbeddings(FragmentoBuscable{ID: "f1", Contenido: "test"})
        if err == nil {
                t.Error("debería propagar el error del provider")
        }

        // BM25 debería seguir funcionando
        if be.Total() != 1 {
                t.Errorf("BM25 debería tener el fragmento, got %d", be.Total())
        }
        // Pero embeddings vacío
        if be.TotalEmbeddings() != 0 {
                t.Errorf("no debería tener embeddings, got %d", be.TotalEmbeddings())
        }
}

func TestBuscadorEmbeddings_IndexarBatchConEmbeddings_Eficiencia(t *testing.T) {
        provider := &mockProvider{dimensiones: 4}
        be := NuevoBuscadorEmbeddings(provider)

        frags := []FragmentoBuscable{
                {ID: "f1", Contenido: "auth"},
                {ID: "f2", Contenido: "database"},
                {ID: "f3", Contenido: "logger"},
        }
        n, err := be.IndexarBatchConEmbeddings(frags)
        if err != nil {
                t.Fatalf("error: %v", err)
        }
        if n != 3 {
                t.Errorf("debería indexar 3 embeddings, got %d", n)
        }
        if be.TotalEmbeddings() != 3 {
                t.Errorf("TotalEmbeddings debería ser 3, got %d", be.TotalEmbeddings())
        }
        if be.Total() != 3 {
                t.Errorf("Total BM25 debería ser 3, got %d", be.Total())
        }
}

func TestBuscadorEmbeddings_IndexarBatch_Vacio(t *testing.T) {
        provider := &mockProvider{dimensiones: 4}
        be := NuevoBuscadorEmbeddings(provider)

        n, err := be.IndexarBatchConEmbeddings(nil)
        if err != nil {
                t.Errorf("no debería error con slice vacío: %v", err)
        }
        if n != 0 {
                t.Errorf("debería retornar 0, got %d", n)
        }
}

func TestBuscadorEmbeddings_Desindexar_EliminaDeAmbosIndices(t *testing.T) {
        provider := &mockProvider{dimensiones: 4}
        be := NuevoBuscadorEmbeddings(provider)

        _ = be.IndexarConEmbeddings(FragmentoBuscable{ID: "f1", Contenido: "test"})
        be.Desindexar("f1")

        if be.Total() != 0 {
                t.Errorf("BM25 debería estar vacío, got %d", be.Total())
        }
        if be.TotalEmbeddings() != 0 {
                t.Errorf("embeddings debería estar vacío, got %d", be.TotalEmbeddings())
        }
}

func TestBuscadorEmbeddings_BuscarVector_Exitoso(t *testing.T) {
        provider := &mockProvider{dimensiones: 8}
        be := NuevoBuscadorEmbeddings(provider)

        _ = be.IndexarConEmbeddings(FragmentoBuscable{ID: "f1", Contenido: "authenticate user"})
        _ = be.IndexarConEmbeddings(FragmentoBuscable{ID: "f2", Contenido: "database connection"})

        // Buscar algo similar a f1
        resultados, err := be.BuscarVector("authenticate", 10)
        if err != nil {
                t.Fatalf("error: %v", err)
        }
        if len(resultados) == 0 {
                t.Fatal("debería retornar resultados")
        }
        // El score del primer resultado debería ser > 0
        if resultados[0].ScoreVector <= 0 {
                t.Error("score debería ser > 0")
        }
}

func TestBuscadorEmbeddings_BuscarVector_SinEmbeddings_RetornaVacio(t *testing.T) {
        provider := &mockProvider{dimensiones: 4}
        be := NuevoBuscadorEmbeddings(provider)
        // No indexar nada

        resultados, err := be.BuscarVector("test", 10)
        if err != nil {
                t.Fatalf("no debería error: %v", err)
        }
        if len(resultados) != 0 {
                t.Errorf("debería retornar 0, got %d", len(resultados))
        }
}

func TestBuscadorEmbeddings_BuscarVector_SinProvider_Error(t *testing.T) {
        be := NuevoBuscadorEmbeddings(nil)
        _, err := be.BuscarVector("test", 10)
        if err != ErrProviderNoConfigurado {
                t.Errorf("debería ser ErrProviderNoConfigurado, got %v", err)
        }
}

// ═══════════════════════════════════════════════════════
// TESTS BÚSQUEDA HÍBRIDA CON EMBEDDINGS
// ═══════════════════════════════════════════════════════

func TestBuscadorEmbeddings_BuscarHibrido_CombinaBM25yVector(t *testing.T) {
        provider := &mockProvider{dimensiones: 8}
        be := NuevoBuscadorEmbeddings(provider)

        _ = be.IndexarConEmbeddings(FragmentoBuscable{ID: "f1", Contenido: "authenticate user password"})
        _ = be.IndexarConEmbeddings(FragmentoBuscable{ID: "f2", Contenido: "connect database"})
        _ = be.IndexarConEmbeddings(FragmentoBuscable{ID: "f3", Contenido: "authenticate session token"})

        resultados := be.BuscarHibridoConEmbeddings("authenticate", 10)
        if len(resultados) == 0 {
                t.Fatal("debería retornar resultados")
        }

        // Los fragmentos con "authenticate" deberían estar arriba
        top := resultados[0]
        if top.Fragmento.ID != "f1" && top.Fragmento.ID != "f3" {
                t.Errorf("top debería ser f1 o f3 (ambos tienen 'authenticate'), got %s", top.Fragmento.ID)
        }

        // Debería tener RankBM25 Y RankVector asignados (fue benefit de RRF)
        if top.RankBM25 == 0 {
                t.Error("top debería tener RankBM25 asignado")
        }
        if top.RankVector == 0 {
                t.Error("top debería tener RankVector asignado")
        }
}

func TestBuscadorEmbeddings_BuscarHibrido_ProviderFalla_CaeABM25(t *testing.T) {
        // Indexar fragmentos con embeddings válidos
        provider := &mockProvider{dimensiones: 4}
        be := NuevoBuscadorEmbeddings(provider)

        _ = be.IndexarConEmbeddings(FragmentoBuscable{ID: "f1", Contenido: "authenticate user"})

        // Ahora hacer que el provider falle para la query (simular API caída)
        provider.failOnCall = true

        resultados := be.BuscarHibridoConEmbeddings("authenticate", 10)
        // Debería caer a BM25 puro y aún retornar resultados
        if len(resultados) == 0 {
                t.Fatal("debería retornar resultados vía BM25 (fallback graceful)")
        }
}

func TestBuscadorEmbeddings_BuscarHibrido_SinProvider_UsaBM25Puro(t *testing.T) {
        be := NuevoBuscadorEmbeddings(nil)

        be.Indexar(FragmentoBuscable{ID: "f1", Contenido: "authenticate user"})

        resultados := be.BuscarHibridoConEmbeddings("authenticate", 10)
        if len(resultados) == 0 {
                t.Fatal("debería retornar resultados vía BM25")
        }
}

func TestBuscadorEmbeddings_ConProvider_AsignaProvider(t *testing.T) {
        be := NuevoBuscadorEmbeddings(nil)
        if be.TieneProvider() {
                t.Error("inicialmente no debería tener provider")
        }

        provider := &mockProvider{dimensiones: 4}
        be.ConProvider(provider)

        if !be.TieneProvider() {
                t.Error("después de ConProvider debería tener provider")
        }
        if be.dimensiones != 4 {
                t.Errorf("dimensiones debería ser 4, got %d", be.dimensiones)
        }
}

// ═══════════════════════════════════════════════════════
// TESTS HELPERS
// ═══════════════════════════════════════════════════════

func TestSimilitudCoseno_Identicos_Retorna1(t *testing.T) {
        a := []float32{1.0, 0.0, 0.0, 0.0}
        b := []float32{1.0, 0.0, 0.0, 0.0}
        score := similitudCoseno(a, b)
        if score < 0.99 || score > 1.01 {
                t.Errorf("vectores idénticos deberían tener coseno ~1.0, got %f", score)
        }
}

func TestSimilitudCoseno_Ortogonales_Retorna0(t *testing.T) {
        a := []float32{1.0, 0.0}
        b := []float32{0.0, 1.0}
        score := similitudCoseno(a, b)
        if score < -0.01 || score > 0.01 {
                t.Errorf("vectores ortogonales deberían tener coseno ~0, got %f", score)
        }
}

func TestSimilitudCoseno_DimensionesDistintas_Retorna0(t *testing.T) {
        a := []float32{1.0, 0.0}
        b := []float32{1.0, 0.0, 0.0}
        score := similitudCoseno(a, b)
        if score != 0 {
                t.Errorf("dimensiones distintas deberían retornar 0, got %f", score)
        }
}

func TestSimilitudCoseno_Vacio_Retorna0(t *testing.T) {
        score := similitudCoseno(nil, nil)
        if score != 0 {
                t.Errorf("vectores vacíos deberían retornar 0, got %f", score)
        }
}

func TestSqrt_ValoresConocidos(t *testing.T) {
        casos := []struct {
                x, esperado float64
        }{
                {0, 0},
                {1, 1},
                {4, 2},
                {9, 3},
                {16, 4},
                {100, 10},
        }
        for _, c := range casos {
                got := math.Sqrt(c.x)
                // Tolerancia 0.001
                diff := got - c.esperado
                if diff < -0.001 || diff > 0.001 {
                        t.Errorf("math.Sqrt(%f) = %f, esperaba %f", c.x, got, c.esperado)
                }
        }
}

// ═══════════════════════════════════════════════════════
// P4.1: CONCURRENCY TESTS
// ═══════════════════════════════════════════════════════

// TestBuscadorEmbeddings_ConcurrentIndexarBuscar lanza 10 goroutines indexando
// y 10 goroutines buscando simultáneamente para verificar seguridad concurrente.
func TestBuscadorEmbeddings_ConcurrentIndexarBuscar(t *testing.T) {
        provider := &mockProvider{dimensiones: 8}
        be := NuevoBuscadorEmbeddings(provider)

        // Pre-indexar 20 fragmentos
        for i := 0; i < 20; i++ {
                be.IndexarConEmbeddings(FragmentoBuscable{
                        ID:        fmt.Sprintf("pre-%d", i),
                        Contenido: fmt.Sprintf("func Procesar%d(ctx context.Context, req *Request%d) (*Response%d, error) { return nil, nil }", i, i, i),
                })
        }

        var wg sync.WaitGroup
        errors := make(chan error, 20)

        // 10 goroutines indexando
        for i := 0; i < 10; i++ {
                wg.Add(1)
                go func(id int) {
                        defer wg.Done()
                        for j := 0; j < 5; j++ {
                                fragID := fmt.Sprintf("idx-%d-%d", id, j)
                                err := be.IndexarConEmbeddings(FragmentoBuscable{
                                        ID:        fragID,
                                        Contenido: fmt.Sprintf("handler_%d procesa request tipo %d con validación de parámetros", id, j),
                                })
                                if err != nil {
                                        errors <- fmt.Errorf("goroutine idx-%d: %w", id, err)
                                }
                        }
                }(i)
        }

        // 10 goroutines buscando
        for i := 0; i < 10; i++ {
                wg.Add(1)
                go func(id int) {
                        defer wg.Done()
                        for j := 0; j < 5; j++ {
                                query := fmt.Sprintf("procesar request handler_%d", id)
                                resultados := be.BuscarHibridoConEmbeddings(query, 5)
                                // Verificar que los resultados son consistentes
                                for _, r := range resultados {
                                        if r.Fragmento.ID == "" {
                                                errors <- fmt.Errorf("goroutine search-%d: resultado con ID vacío", id)
                                        }
                                }
                        }
                }(i)
        }

        wg.Wait()
        close(errors)

        // Verificar que no hubo errores de concurrencia
        for err := range errors {
                t.Errorf("error de concurrencia: %v", err)
        }

        // Verificar que el estado final es consistente
        total := be.Total()
        totalEmb := be.TotalEmbeddings()
        if total == 0 {
                t.Error("el total de fragmentos debería ser > 0 después de indexación concurrente")
        }
        if totalEmb == 0 {
                t.Error("el total de embeddings debería ser > 0")
        }
}

// ═══════════════════════════════════════════════════════
// P4.2: ERROR PATH TESTS
// ═══════════════════════════════════════════════════════

// TestBuscadorEmbeddings_ProviderFalla_Gracioso verifica que cuando el provider
// de embeddings falla completamente (errores en indexación y búsqueda), el buscador
// degrada gracefulmente a BM25 puro y sigue retornando resultados correctos.
func TestBuscadorEmbeddings_ProviderFalla_Gracioso(t *testing.T) {
        provider := &mockProvider{dimensiones: 8}
        be := NuevoBuscadorEmbeddings(provider)

        // Indexar fragmentos con embeddings funcionales (snake_case para que
        // el tokenizador BM25 pueda separar las palabras correctamente)
        fragments := []FragmentoBuscable{
                {ID: "f-auth", Contenido: "authenticate_user with username and password to get token jwt auth"},
                {ID: "f-db", Contenido: "connect_postgres database using dsn connection string sql driver"},
                {ID: "f-cache", Contenido: "set_cache with key and value byte ttl duration redis"},
                {ID: "f-logger", Contenido: "logger_middleware handles http request response logging"},
                {ID: "f-router", Contenido: "setup_router configures mux routes handlers endpoints"},
        }
        for _, f := range fragments {
                _ = be.IndexarConEmbeddings(f)
        }

        // Hacer que el provider falle (simular API caída)
        provider.failOnCall = true

        // Intentar indexar un fragmento nuevo — debería fallar pero BM25 debería tenerlo
        err := be.IndexarConEmbeddings(FragmentoBuscable{ID: "f-new", Contenido: "nuevo fragmento para buscar"})
        if err == nil {
                t.Error("debería retornar error cuando el provider falla")
        }
        if be.Total() < 6 {
                t.Errorf("BM25 debería tener al menos 6 fragmentos (5 originales + 1 fallback), got %d", be.Total())
        }

        // Búsqueda híbrida debería caer a BM25 puro
        resultados := be.BuscarHibridoConEmbeddings("authenticate postgres cache", 10)
        if len(resultados) == 0 {
                t.Fatal("la búsqueda debería retornar resultados vía BM25 (fallback graceful)")
        }

        // Verificar que los resultados más relevantes están arriba
        ids := make(map[string]bool)
        for _, r := range resultados {
                ids[r.Fragmento.ID] = true
        }
        if !ids["f-auth"] {
                t.Error("f-auth debería estar en los resultados (menciona 'authenticate')")
        }
        if !ids["f-db"] {
                t.Error("f-db debería estar en los resultados (menciona 'postgres')")
        }
        if !ids["f-cache"] {
                t.Error("f-cache debería estar en los resultados (menciona 'cache')")
        }

        // BuscarVector debería retornar error
        _, err = be.BuscarVector("test", 10)
        if err == nil {
                t.Error("BuscarVector debería fallar cuando el provider falla")
        }
}

func TestErroresTipo(t *testing.T) {
        if ErrProviderNoConfigurado.Error() != "provider de embeddings no configurado" {
                t.Error("mensaje incorrecto")
        }
        if ErrEmbeddingsVacios.Error() != "provider retornó lista de embeddings vacía" {
                t.Error("mensaje incorrecto")
        }
}
