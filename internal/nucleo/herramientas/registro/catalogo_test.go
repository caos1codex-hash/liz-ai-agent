package registro

import (
        "context"
        "errors"
        "fmt"
        "sync"
        "testing"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Herramientas de prueba
// ============================================================================

type herrMock struct {
        nombre      string
        descripcion string
        valido      bool
        delay       time.Duration
        res         herramientas.Resultado
        err         error
}

var _ herramientas.Herramienta = (*herrMock)(nil)

func (h *herrMock) Nombre() string      { return h.nombre }
func (h *herrMock) Descripcion() string { return h.descripcion }
func (h *herrMock) Parametros() []herramientas.Parametro {
        return []herramientas.Parametro{{Nombre: "input", Tipo: "string", Requerido: true}}
}
func (h *herrMock) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
        if h.delay > 0 {
                select {
                case <-time.After(h.delay):
                case <-ctx.Done():
                        return herramientas.Resultado{Exito: false, Error: "cancelado"}, ctx.Err()
                }
        }
        return h.res, h.err
}
func (h *herrMock) Validar() error {
        if !h.valido {
                return errors.New("herramienta inválida (mock)")
        }
        return nil
}

// ============================================================================
// Tests del Catálogo
// ============================================================================

func TestCatalogo_RegistrarYObtener(t *testing.T) {
        c := NuevoCatalogo()
        h := &herrMock{nombre: "test1", valido: true}

        if err := c.Registrar(h); err != nil {
                t.Fatalf("Registrar falló: %v", err)
        }
        if c.Tamaño() != 1 {
                t.Errorf("Tamaño = %d, esperaba 1", c.Tamaño())
        }

        obt, ok := c.Obtener("test1")
        if !ok {
                t.Fatal("herramienta no encontrada después de registrar")
        }
        if obt.Nombre() != "test1" {
                t.Errorf("Nombre obtenido = %q", obt.Nombre())
        }
}

func TestCatalogo_RegistrarDuplicado(t *testing.T) {
        c := NuevoCatalogo()
        h1 := &herrMock{nombre: "dup", valido: true}
        h2 := &herrMock{nombre: "dup", valido: true, descripcion: "v2"}

        if err := c.Registrar(h1); err != nil {
                t.Fatalf("primer Registrar falló: %v", err)
        }
        // Segundo registro con mismo nombre: se reemplaza (no error)
        if err := c.Registrar(h2); err != nil {
                t.Fatalf("segundo Registrar debería reemplazar: %v", err)
        }
        obt, _ := c.Obtener("dup")
        if obt.Descripcion() != "v2" {
                t.Errorf("Descripción = %q, esperaba v2 (reemplazo)", obt.Descripcion())
        }
}

func TestCatalogo_RegistrarNombreInvalido(t *testing.T) {
        c := NuevoCatalogo()
        casos := []string{"", "a", "CON_MAYUS", "con-guion", "con$pecial"}
        for _, nombre := range casos {
                h := &herrMock{nombre: nombre, valido: true}
                if err := c.Registrar(h); err == nil {
                        t.Errorf("nombre %q debería fallar", nombre)
                }
        }
}

func TestCatalogo_RegistrarValidarFalla(t *testing.T) {
        c := NuevoCatalogo()
        h := &herrMock{nombre: "falsa", valido: false}
        err := c.Registrar(h)
        if err == nil {
                t.Fatal("esperaba error porque Validar() falla")
        }
        var errInv *herramientas.ErrHerramientaInvalida
        if !errors.As(err, &errInv) {
                t.Errorf("esperaba ErrHerramientaInvalida, obtuve %T: %v", err, err)
        }
}

func TestCatalogo_RegistrarNil(t *testing.T) {
        c := NuevoCatalogo()
        if err := c.Registrar(nil); err == nil {
                t.Error("Registrar(nil) debería fallar")
        }
}

func TestCatalogo_Eliminar(t *testing.T) {
        c := NuevoCatalogo()
        c.Registrar(&herrMock{nombre: "temp", valido: true})

        if !c.Eliminar("temp") {
                t.Error("Eliminar debería retornar true para herramienta existente")
        }
        if c.Existe("temp") {
                t.Error("Existe debería retornar false después de Eliminar")
        }
        if c.Eliminar("temp") {
                t.Error("Eliminar debería retornar false para herramienta inexistente")
        }
}

func TestCatalogo_ListarYOrdenar(t *testing.T) {
        c := NuevoCatalogo()
        c.Registrar(&herrMock{nombre: "zeta", valido: true})
        c.Registrar(&herrMock{nombre: "alfa", valido: true})
        c.Registrar(&herrMock{nombre: "beta", valido: true})

        lista := c.Listar()
        if len(lista) != 3 {
                t.Fatalf("len(lista) = %d, esperaba 3", len(lista))
        }
        // Verificar orden alfabético
        esperado := []string{"alfa", "beta", "zeta"}
        for i, h := range lista {
                if h.Nombre() != esperado[i] {
                        t.Errorf("posición %d: %q, esperaba %q", i, h.Nombre(), esperado[i])
                }
        }

        nombres := c.Nombres()
        for i, n := range nombres {
                if n != esperado[i] {
                        t.Errorf("Nombres[%d] = %q", i, n)
                }
        }
}

func TestCatalogo_EjecutarExitosa(t *testing.T) {
        c := NuevoCatalogo()
        c.Registrar(&herrMock{
                nombre: "ok",
                valido: true,
                res:    herramientas.Resultado{Exito: true, Datos: "hola"},
        })

        res, err := c.Ejecutar(context.Background(), "ok", nil)
        if err != nil {
                t.Fatalf("Ejecutar falló: %v", err)
        }
        if !res.Exito || res.Datos != "hola" {
                t.Errorf("Resultado inesperado: %+v", res)
        }
        // Verificar que se añadió metadata
        if res.Metadata["herramienta"] != "ok" {
                t.Errorf("Metadata[herramienta] = %v", res.Metadata["herramienta"])
        }
        if res.Metadata["duracion_ms"] == nil {
                t.Errorf("Metadata[duracion_ms] no presente")
        }
}

func TestCatalogo_EjecutarNoEncontrada(t *testing.T) {
        c := NuevoCatalogo()
        _, err := c.Ejecutar(context.Background(), "inexistente", nil)
        var errNF *ErrHerramientaNoEncontrada
        if !errors.As(err, &errNF) {
                t.Errorf("esperaba ErrHerramientaNoEncontrada, obtuve %T: %v", err, err)
        }
}

func TestCatalogo_EjecutarRespetaContext(t *testing.T) {
        c := NuevoCatalogo()
        c.Registrar(&herrMock{
                nombre: "lenta",
                valido: true,
                delay:  500 * time.Millisecond,
                res:    herramientas.Resultado{Exito: true},
        })

        ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
        defer cancel()

        _, err := c.Ejecutar(ctx, "lenta", nil)
        if err == nil {
                t.Error("esperaba error de contexto cancelado")
        }
}

func TestCatalogo_Snapshot(t *testing.T) {
        c := NuevoCatalogo()
        c.Registrar(&herrMock{nombre: "aa", valido: true, descripcion: "alpha"})
        c.Registrar(&herrMock{nombre: "bb", valido: true, descripcion: "beta"})

        snap := c.Snapshot()
        if len(snap) != 2 {
                t.Fatalf("len(Snapshot) = %d, esperaba 2", len(snap))
        }
        if snap[0].Nombre != "aa" || snap[1].Nombre != "bb" {
                t.Errorf("orden Snapshot incorrecto: %+v", snap)
        }
        if snap[0].Descripcion != "alpha" {
                t.Errorf("descripcion snap[0] = %q", snap[0].Descripcion)
        }
        if len(snap[0].Parametros) != 1 || snap[0].Parametros[0].Nombre != "input" {
                t.Errorf("Parametros no reflejados en Snapshot")
        }
}

// ============================================================================
// Tests de Concurrencia
// ============================================================================

func TestCatalogo_ConcurrenciaRegistro(t *testing.T) {
        c := NuevoCatalogo()
        var wg sync.WaitGroup
        // 10 goroutines registrando herramientas distintas
        for i := 0; i < 10; i++ {
                wg.Add(1)
                go func(n int) {
                        defer wg.Done()
                        nombre := fmt.Sprintf("herr_%02d", n)
                        c.Registrar(&herrMock{nombre: nombre, valido: true})
                }(i)
        }
        wg.Wait()
        if c.Tamaño() != 10 {
                t.Errorf("Tamaño = %d, esperaba 10", c.Tamaño())
        }
}

func TestCatalogo_ConcurrenciaEjecucion(t *testing.T) {
        c := NuevoCatalogo()
        c.Registrar(&herrMock{
                nombre: "concurrente",
                valido: true,
                res:    herramientas.Resultado{Exito: true},
        })

        var wg sync.WaitGroup
        for i := 0; i < 50; i++ {
                wg.Add(1)
                go func() {
                        defer wg.Done()
                        res, _ := c.Ejecutar(context.Background(), "concurrente", nil)
                        if !res.Exito {
                                t.Errorf("ejecución concurrente falló")
                        }
                }()
        }
        wg.Wait()

        // Verificar que las métricas contabilizaron 50 ejecuciones
        m := c.Metricas().Obtener("concurrente")
        if m.Ejecuciones != 50 {
                t.Errorf("Métricas.Ejecuciones = %d, esperaba 50", m.Ejecuciones)
        }
}

// ============================================================================
// Tests de Métricas
// ============================================================================

func TestMetricas_RegistrarYObtener(t *testing.T) {
        m := NuevasMetricas()
        m.RegistrarEjecucion("h1", true, 100*time.Millisecond)
        m.RegistrarEjecucion("h1", true, 200*time.Millisecond)
        m.RegistrarEjecucion("h1", false, 50*time.Millisecond)

        obt := m.Obtener("h1")
        if obt.Ejecuciones != 3 {
                t.Errorf("Ejecuciones = %d, esperaba 3", obt.Ejecuciones)
        }
        if obt.Exitos != 2 || obt.Fallos != 1 {
                t.Errorf("Exitos=%d, Fallos=%d, esperaba 2/1", obt.Exitos, obt.Fallos)
        }
        if obt.TasaExito < 0.66 || obt.TasaExito > 0.67 {
                t.Errorf("TasaExito = %v, esperaba ~0.667", obt.TasaExito)
        }
        // Latencia min: 50ms, max: 200ms, promedio: (100+200+50)/3 = 116.67ms
        if obt.LatenciaMin != 50*time.Millisecond {
                t.Errorf("LatenciaMin = %v, esperaba 50ms", obt.LatenciaMin)
        }
        if obt.LatenciaMax != 200*time.Millisecond {
                t.Errorf("LatenciaMax = %v, esperaba 200ms", obt.LatenciaMax)
        }
        // Promedio: 116.67ms (con truncado a int64ns)
        esperado := time.Duration((100 + 200 + 50) * int(time.Millisecond) / 3)
        if obt.LatenciaPromedio != esperado {
                t.Errorf("LatenciaPromedio = %v, esperaba %v", obt.LatenciaPromedio, esperado)
        }
}

func TestMetricas_NoExistente(t *testing.T) {
        m := NuevasMetricas()
        obt := m.Obtener("no_existe")
        if obt.Nombre != "no_existe" || obt.Ejecuciones != 0 {
                t.Errorf("métrica de herramienta no existente debería ser cero: %+v", obt)
        }
}

func TestMetricas_RegistrarError(t *testing.T) {
        m := NuevasMetricas()
        m.RegistrarError("h1", "fallo de red")
        obt := m.Obtener("h1")
        if obt.UltimoError != "fallo de red" {
                t.Errorf("UltimoError = %q, esperaba 'fallo de red'", obt.UltimoError)
        }
}

func TestMetricas_ListarOrdenado(t *testing.T) {
        m := NuevasMetricas()
        m.RegistrarEjecucion("z", true, 10*time.Millisecond)
        m.RegistrarEjecucion("a", true, 10*time.Millisecond)
        m.RegistrarEjecucion("m", true, 10*time.Millisecond)

        lista := m.Listar()
        if len(lista) != 3 {
                t.Fatalf("len(Listar) = %d", len(lista))
        }
        if lista[0].Nombre != "a" || lista[1].Nombre != "m" || lista[2].Nombre != "z" {
                t.Errorf("Listar no ordenado: %+v", lista)
        }
}

func TestMetricas_Resumen(t *testing.T) {
        m := NuevasMetricas()
        m.RegistrarEjecucion("a", true, 10*time.Millisecond)
        m.RegistrarEjecucion("a", true, 20*time.Millisecond)
        m.RegistrarEjecucion("b", false, 5*time.Millisecond)
        m.RegistrarEjecucion("b", true, 15*time.Millisecond)

        r := m.Resumen()
        if r.TotalHerramientas != 2 {
                t.Errorf("TotalHerramientas = %d, esperaba 2", r.TotalHerramientas)
        }
        if r.TotalEjecuciones != 4 {
                t.Errorf("TotalEjecuciones = %d, esperaba 4", r.TotalEjecuciones)
        }
        if r.TotalExitos != 3 || r.TotalFallos != 1 {
                t.Errorf("Exitos=%d, Fallos=%d, esperaba 3/1", r.TotalExitos, r.TotalFallos)
        }
        if r.TasaExitoGlobal < 0.74 || r.TasaExitoGlobal > 0.76 {
                t.Errorf("TasaExitoGlobal = %v, esperaba ~0.75", r.TasaExitoGlobal)
        }
        if len(r.PorHerramienta) != 2 {
                t.Errorf("PorHerramienta len = %d, esperaba 2", len(r.PorHerramienta))
        }
}

func TestMetricas_Reset(t *testing.T) {
        m := NuevasMetricas()
        m.RegistrarEjecucion("a", true, 10*time.Millisecond)
        m.Reset()
        if r := m.Resumen(); r.TotalEjecuciones != 0 {
                t.Errorf("después de Reset: TotalEjecuciones = %d", r.TotalEjecuciones)
        }
}

func TestMetricas_Concurrencia(t *testing.T) {
        m := NuevasMetricas()
        var wg sync.WaitGroup
        for i := 0; i < 100; i++ {
                wg.Add(1)
                go func(n int) {
                        defer wg.Done()
                        exitoso := n%2 == 0
                        m.RegistrarEjecucion("h", exitoso, time.Duration(n)*time.Millisecond)
                }(i)
        }
        wg.Wait()
        if m.Obtener("h").Ejecuciones != 100 {
                t.Errorf("Ejecuciones = %d, esperaba 100", m.Obtener("h").Ejecuciones)
        }
}
