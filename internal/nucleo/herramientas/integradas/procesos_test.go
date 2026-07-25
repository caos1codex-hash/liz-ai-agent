package integradas

import (
        "context"
        "os"
        "os/exec"
        "runtime"
        "strings"
        "testing"
        "time"
)

func TestProcesos_Basico(t *testing.T) {
        p := NewProcesos()
        if p.Nombre() != "procesos" {
                t.Errorf("Nombre = %q", p.Nombre())
        }
        if err := p.Validar(); err != nil {
                t.Errorf("Validar: %v", err)
        }
}

func TestProcesos_Listar(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
                "limite":    10,
        })
        if !res.Exito {
                t.Fatalf("listar falló: %+v", res)
        }
        datos := res.Datos.(ResultadoProcesos)
        if datos.Total == 0 {
                t.Error("esperaba al menos 1 proceso")
        }
        // Debe incluir PID 1 (init/systemd)
        encontrado := false
        for _, proc := range datos.Procesos {
                if proc.PID == 1 {
                        encontrado = true
                        break
                }
        }
        if !encontrado {
                t.Log("PID 1 no encontrado (puede no ser Linux estándar)")
        }
}

func TestProcesos_ListarFiltroNombre(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        // Buscar procesos con nombre que contenga "go" o "test" o "liz"
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
                "nombre":    "go",
                "limite":    50,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoProcesos)
        for _, proc := range datos.Procesos {
                if !strings.Contains(strings.ToLower(proc.Nombre), "go") {
                        t.Errorf("filtro nombre no respetado: %s", proc.Nombre)
                }
        }
}

func TestProcesos_InfoSelf(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        pidSelf := os.Getpid()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "info",
                "pid":       pidSelf,
        })
        if !res.Exito {
                t.Fatalf("info falló para PID %d: %+v", pidSelf, res)
        }
        datos := res.Datos.(ResultadoProcesos)
        if datos.Proceso == nil {
                t.Fatal("Proceso nil")
        }
        if datos.Proceso.PID != pidSelf {
                t.Errorf("PID = %d, esperaba %d", datos.Proceso.PID, pidSelf)
        }
        if datos.Proceso.Nombre == "" {
                t.Error("Nombre vacío")
        }
}

func TestProcesos_InfoNoExiste(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        // PID muy alto seguramente no existe
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "info",
                "pid":       99999999,
        })
        if res.Exito {
                t.Error("debería fallar para PID inexistente")
        }
}

func TestProcesos_MatarSelf(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        pidSelf := os.Getpid()

        // Matar con SIGCONT (no mata, solo continúa si estaba parado)
        // — pero debería "enviarse" exitosamente
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "matar",
                "pid":       pidSelf,
                "senal":     "SIGCONT",
        })
        if !res.Exito {
                t.Fatalf("matar SIGCONT falló: %+v", res)
        }
        datos := res.Datos.(ResultadoProcesos)
        if !datos.Enviada {
                t.Error("Enviada debería ser true")
        }
        if datos.Senal != "SIGCONT" {
                t.Errorf("Senal = %q", datos.Senal)
        }
}

func TestProcesos_MatarPIDInvalido(t *testing.T) {
        p := NewProcesos()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "matar",
                "pid":       -1,
        })
        if res.Exito {
                t.Error("debería fallar para PID inválido")
        }
}

func TestProcesos_MatarSenalInvalida(t *testing.T) {
        p := NewProcesos()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "matar",
                "pid":       1,
                "senal":     "SIGINVALIDA",
        })
        if res.Exito {
                t.Error("debería fallar para señal inválida")
        }
}

func TestProcesos_MatarProcesoInexistente(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "matar",
                "pid":       99999999,
                "senal":     "SIGTERM",
        })
        if res.Exito {
                t.Error("debería fallar para PID inexistente")
        }
}

func TestProcesos_Arbol(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        pidSelf := os.Getpid()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "arbol",
                "pid":       pidSelf,
        })
        if !res.Exito {
                t.Fatalf("arbol falló: %+v", res)
        }
        datos := res.Datos.(ResultadoProcesos)
        // Esperamos al menos el proceso actual
        if datos.Total == 0 {
                t.Log("árbol vacío (puede ser normal si el proceso no tiene hijos)")
        }
}

func TestProcesos_OperacionInvalida(t *testing.T) {
        p := NewProcesos()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "invalida",
        })
        if res.Exito {
                t.Error("debería fallar para operacion inválida")
        }
}

func TestProcesos_Limite(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        p := NewProcesos()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "listar",
                "limite":    5,
        })
        if !res.Exito {
                t.Fatalf("falló: %+v", res)
        }
        datos := res.Datos.(ResultadoProcesos)
        if datos.Total > 5 {
                t.Errorf("Total = %d, esperaba <= 5", datos.Total)
        }
        if !datos.Truncado {
                t.Log("esperaba Truncado=true (puede haber pocos procesos)")
        }
}

func TestParsearSenal(t *testing.T) {
        casos := map[string]bool{
                "SIGTERM": true,
                "SIGKILL": true,
                "SIGINT":  true,
                "SIGHUP":  true,
                "SIGSTOP": true,
                "SIGCONT": true,
                "SIGINVALIDA": false,
                "":            false,
        }
        for nombre, valida := range casos {
                sig := parsearSenal(nombre)
                if valida && sig == 0 {
                        t.Errorf("parsearSenal(%q) = 0, esperaba != 0", nombre)
                }
                if !valida && sig != 0 {
                        t.Errorf("parsearSenal(%q) = %d, esperaba 0", nombre, sig)
                }
        }
}

func TestProcesos_SubprocessKill(t *testing.T) {
        if runtime.GOOS != "linux" {
                t.Skip("solo Linux")
        }
        // Lanzar un sleep 60 que matamos
        cmd := exec.Command("sleep", "60")
        if err := cmd.Start(); err != nil {
                t.Skipf("no se pudo iniciar sleep: %v", err)
        }
        defer cmd.Wait()
        pid := cmd.Process.Pid
        time.Sleep(100 * time.Millisecond) // esperar que arranque

        p := NewProcesos()
        res, _ := p.Ejecutar(context.Background(), map[string]interface{}{
                "operacion": "matar",
                "pid":       pid,
                "senal":     "SIGTERM",
        })
        if !res.Exito {
                t.Errorf("matar falló: %+v", res)
        }
        // Esperar a que el proceso termine
        done := make(chan error, 1)
        go func() { done <- cmd.Wait() }()
        select {
        case <-done:
                // OK
        case <-time.After(2 * time.Second):
                t.Error("el proceso no terminó tras SIGTERM")
        }
}
