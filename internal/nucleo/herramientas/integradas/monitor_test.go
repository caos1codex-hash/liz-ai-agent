package integradas

import (
        "context"
        "runtime"
        "strings"
        "testing"
        "time"
)

// duracionFromSegundos helper para test.
func duracionFromSegundos(s int64) time.Duration {
        return time.Duration(s) * time.Second
}

func TestMonitor_Basico(t *testing.T) {
        m := NewMonitor()
        if m.Nombre() != "monitor" {
                t.Errorf("Nombre = %q", m.Nombre())
        }
        if err := m.Validar(); err != nil {
                t.Errorf("Validar: %v", err)
        }
}

func TestMonitor_Completo(t *testing.T) {
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "completo",
        })
        if !res.Exito {
                t.Fatalf("completo falló: %+v", res)
        }
        datos := res.Datos.(ResultadoMonitor)
        if datos.CPU == nil {
                t.Error("CPU nil")
        }
        if datos.Memoria == nil {
                t.Error("Memoria nil")
        }
        if datos.Uptime == nil {
                t.Error("Uptime nil")
        }
}

func TestMonitor_CPU(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "cpu",
        })
        if !res.Exito {
                t.Fatalf("cpu falló: %+v", res)
        }
        datos := res.Datos.(ResultadoMonitor)
        if datos.CPU == nil {
                t.Fatal("CPU nil")
        }
        if datos.CPU.NumCores <= 0 {
                t.Errorf("NumCores = %d, esperaba > 0", datos.CPU.NumCores)
        }
        if datos.CPU.UsoPorcentaje < 0 || datos.CPU.UsoPorcentaje > 100 {
                t.Errorf("UsoPorcentaje = %v, fuera de rango", datos.CPU.UsoPorcentaje)
        }
}

func TestMonitor_Memoria(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "memoria",
        })
        if !res.Exito {
                t.Fatalf("memoria falló: %+v", res)
        }
        datos := res.Datos.(ResultadoMonitor)
        if datos.Memoria == nil {
                t.Fatal("Memoria nil")
        }
        if datos.Memoria.TotalKB <= 0 {
                t.Errorf("TotalKB = %d, esperaba > 0", datos.Memoria.TotalKB)
        }
        if datos.Memoria.UsadaPorcentaje < 0 || datos.Memoria.UsadaPorcentaje > 100 {
                t.Errorf("UsadaPorcentaje = %v, fuera de rango", datos.Memoria.UsadaPorcentaje)
        }
        if datos.Memoria.UsadaKB > datos.Memoria.TotalKB {
                t.Errorf("UsadaKB (%d) > TotalKB (%d)", datos.Memoria.UsadaKB, datos.Memoria.TotalKB)
        }
}

func TestMonitor_Disco(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{
                "operacion":   "disco",
                "ruta_disco":  "/",
        })
        if !res.Exito {
                t.Fatalf("disco falló: %+v", res)
        }
        datos := res.Datos.(ResultadoMonitor)
        if datos.Disco == nil {
                t.Fatal("Disco nil")
        }
        if datos.Disco.TotalBytes <= 0 {
                t.Errorf("TotalBytes = %d, esperaba > 0", datos.Disco.TotalBytes)
        }
        if datos.Disco.UsadoPorcentaje < 0 || datos.Disco.UsadoPorcentaje > 100 {
                t.Errorf("UsadoPorcentaje = %v, fuera de rango", datos.Disco.UsadoPorcentaje)
        }
}

func TestMonitor_Red(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "red",
        })
        if !res.Exito {
                t.Fatalf("red falló: %+v", res)
        }
        datos := res.Datos.(ResultadoMonitor)
        // En Linux debe haber al menos lo (loopback)
        if len(datos.Red) == 0 {
                t.Log("ninguna interfaz encontrada (puede ser normal en contenedores)")
        }
}

func TestMonitor_Uptime(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "uptime",
        })
        if !res.Exito {
                t.Fatalf("uptime falló: %+v", res)
        }
        datos := res.Datos.(ResultadoMonitor)
        if datos.Uptime == nil {
                t.Fatal("Uptime nil")
        }
        if datos.Uptime.Segundos <= 0 {
                t.Errorf("Segundos = %d, esperaba > 0", datos.Uptime.Segundos)
        }
        if datos.Uptime.Humano == "" {
                t.Error("Humano vacío")
        }
        if !strings.Contains(datos.Uptime.Humano, "s") {
                t.Errorf("Humano = %q, debería contener 's' (segundos)", datos.Uptime.Humano)
        }
}

func TestMonitor_OperacionInvalida(t *testing.T) {
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "invalida",
        })
        if res.Exito {
                t.Error("debería fallar para operacion inválida")
        }
}

func TestMonitor_SinOperacion(t *testing.T) {
        m := NewMonitor()
        res, _ := m.Ejecutar(context.Background(), map[string]interface{}{})
        // Sin operacion debería fallar (requerido)
        if res.Exito {
                t.Error("debería fallar sin operacion")
        }
}

func TestHumanizarDuracion(t *testing.T) {
        casos := []struct {
                segundos int64
                contiene []string
        }{
                {45, []string{"45s"}},
                {125, []string{"2m", "5s"}}, // 2m5s
                {3725, []string{"1h", "2m", "5s"}}, // 1h2m5s
                {90061, []string{"1d", "1h", "1m", "1s"}}, // 1d1h1m1s
        }
        for _, c := range casos {
                obt := humanizarDuracion(duracionFromSegundos(c.segundos))
                for _, s := range c.contiene {
                        if !strings.Contains(obt, s) {
                                t.Errorf("humanizarDuracion(%d) = %q, debería contener %q", c.segundos, obt, s)
                        }
                }
        }
}
