package tracker

import (
        "fmt"
        "os"
        "path/filepath"
        "testing"
)

func TestNuevoTracker_Default(t *testing.T) {
        tr := NuevoTracker(0)
        if tr.maxItems != 20 {
                t.Errorf("default maxItems debería ser 20, got %d", tr.maxItems)
        }
        if tr.Total() != 0 {
                t.Errorf("tracker vacío debería tener 0 ediciones, got %d", tr.Total())
        }
}

func TestRegistrarEdicion_Basico(t *testing.T) {
        tr := NuevoTracker(5)
        tr.RegistrarEdicion("archivo1.go")
        tr.RegistrarEdicion("archivo2.go")

        if tr.Total() != 2 {
                t.Errorf("debería tener 2 ediciones, got %d", tr.Total())
        }

        recientes := tr.ObtenerRecientes(10)
        if len(recientes) != 2 {
                t.Fatalf("debería retornar 2 recientes, got %d", len(recientes))
        }
        if recientes[0] != "archivo2.go" {
                t.Errorf("más reciente debería ser archivo2.go, got %s", recientes[0])
        }
        if recientes[1] != "archivo1.go" {
                t.Errorf("segundo más reciente debería ser archivo1.go, got %s", recientes[1])
        }
}

func TestTracker_RingBuffer(t *testing.T) {
        tr := NuevoTracker(3)
        tr.RegistrarEdicion("a.go")
        tr.RegistrarEdicion("b.go")
        tr.RegistrarEdicionar("c.go")
        tr.RegistrarEdicion("d.go") // debe eliminar "a.go"

        if tr.Total() != 3 {
                t.Errorf("ring buffer debería mantener máximo 3, got %d", tr.Total())
        }

        recientes := tr.ObtenerRecientes(10)
        if len(recientes) != 3 {
                t.Fatalf("debería retornar 3 recientes, got %d", len(recientes))
        }
        if recientes[0] != "d.go" {
                t.Errorf("más reciente debería ser d.go, got %s", recientes[0])
        }
        if recientes[2] != "b.go" {
                t.Errorf("más antiguo debería ser b.go, got %s", recientes[2])
        }
}

func TestObtenerRecientes_Limites(t *testing.T) {
        tr := NuevoTracker(10)
        for i := 0; i < 5; i++ {
                tr.RegistrarEdicion(fmt.Sprintf("f%d.go", i))
        }

        recientes := tr.ObtenerRecientes(3)
        if len(recientes) != 3 {
                t.Fatalf("debería retornar 3, got %d", len(recientes))
        }

        recientes0 := tr.ObtenerRecientes(0)
        if len(recientes0) != 0 {
                t.Errorf("n=0 debería retornar vacío, got %d", len(recientes0))
        }

        recientesTodos := tr.ObtenerRecientes(100)
        if len(recientesTodos) != 5 {
                t.Errorf("n>total debería retornar todos, got %d", len(recientesTodos))
        }
}

func TestTracker_Persistencia(t *testing.T) {
        dir := t.TempDir()
        ruta := filepath.Join(dir, "tracker_ediciones.json")

        tr := NuevoTracker(10)
        tr.RegistrarEdicion("x.go")
        tr.RegistrarEdicion("y.go")

        if err := tr.Guardar(ruta); err != nil {
                t.Fatalf("error guardando: %v", err)
        }

        // Verificar archivo existe
        if _, err := os.Stat(ruta); err != nil {
                t.Fatalf("archivo no creado: %v", err)
        }

        // Cargar en nuevo tracker
        tr2 := NuevoTracker(10)
        if err := tr2.Cargar(ruta); err != nil {
                t.Fatalf("error cargando: %v", err)
        }

        if tr2.Total() != 2 {
                t.Errorf("debería tener 2 ediciones cargadas, got %d", tr2.Total())
        }

        recientes := tr2.ObtenerRecientes(10)
        if len(recientes) != 2 || recientes[0] != "y.go" || recientes[1] != "x.go" {
                t.Errorf("recientes cargados incorrectos: %v", recientes)
        }
}

func TestTracker_CargarInexistente(t *testing.T) {
        tr := NuevoTracker(10)
        if err := tr.Cargar("/ruta/inexistente/tracker.json"); err != nil {
                t.Errorf("cargar archivo inexistente debería retornar nil, got: %v", err)
        }
        if tr.Total() != 0 {
                t.Errorf("tracker debería estar vacío, got %d", tr.Total())
        }
}
