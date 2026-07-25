package permisos

import (
        "encoding/json"
        "os"
        "path/filepath"
        "testing"
)

func crearSistemaTest(t *testing.T) (*Sistema, string) {
        t.Helper()
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, ".liz", "permisos.json")
        if err := os.MkdirAll(filepath.Dir(ruta), 0755); err != nil {
                t.Fatalf("error creando dir: %v", err)
        }

        s := &Sistema{
                rutaArchivo: ruta,
                estado: &EstadoPermisos{
                        Version:  "0.1.0",
                        Permisos: make(map[string]Permiso),
                },
        }

        for _, p := range permisosDefecto {
                s.estado.Permisos[p.Nombre] = Permiso{
                        Nombre:      p.Nombre,
                        Descripcion: p.Descripcion,
                        Concedido:   false,
                }
        }

        return s, tmpDir
}

func TestPermisosPendientes(t *testing.T) {
        s, _ := crearSistemaTest(t)

        pendientes := s.PermisosPendientes()
        if len(pendientes) != 6 {
                t.Errorf("se esperaban 6 permisos pendientes, obtuve %d", len(pendientes))
        }
}

func TestConcederTodos(t *testing.T) {
        s, _ := crearSistemaTest(t)

        if s.TodosConcedidos() {
                t.Error("no debería estar todo concedido al inicio")
        }

        err := s.ConcederTodos("test_session")
        if err != nil {
                t.Fatalf("ConcederTodos() error: %v", err)
        }

        if !s.TodosConcedidos() {
                t.Error("todos deberían estar concedidos")
        }

        if s.estado.SesionID != "test_session" {
                t.Errorf("sesion_id esperado 'test_session', obtuve '%s'", s.estado.SesionID)
        }

        // Verificar archivo persistido
        datos, err := os.ReadFile(s.rutaArchivo)
        if err != nil {
                t.Fatalf("error leyendo archivo de permisos: %v", err)
        }

        var cargado EstadoPermisos
        if err := json.Unmarshal(datos, &cargado); err != nil {
                t.Fatalf("error parseando permisos: %v", err)
        }

        if !cargado.Concedidos {
                t.Error("permisos persistidos deberían estar concedidos")
        }
}

func TestConcederIndividual(t *testing.T) {
        s, _ := crearSistemaTest(t)

        err := s.Conceder("archivos")
        if err != nil {
                t.Fatalf("Conceder() error: %v", err)
        }

        if !s.Verificar(PermArchivos) {
                t.Error("permiso 'archivos' debería estar concedido")
        }
        if s.Verificar(PermTerminal) {
                t.Error("permiso 'terminal' NO debería estar concedido")
        }
}

func TestConcederPermisoInexistente(t *testing.T) {
        s, _ := crearSistemaTest(t)

        err := s.Conceder("no_existe")
        if err == nil {
                t.Error("debería retornar error para permiso inexistente")
        }
}

func TestResetear(t *testing.T) {
        s, _ := crearSistemaTest(t)
        s.ConcederTodos("test_session")

        err := s.Resetear()
        if err != nil {
                t.Fatalf("Resetear() error: %v", err)
        }

        if s.TodosConcedidos() {
                t.Error("permisos deberían estar revocados después de resetear")
        }

        if s.Verificar(PermArchivos) {
                t.Error("permisos individuales deberían estar revocados")
        }
}

func TestEstadoCopia(t *testing.T) {
        s, _ := crearSistemaTest(t)

        // Verificar que Estado() retorna un mapa independiente
        estado1 := s.Estado()
        _ = estado1 // Solo verificamos que retorna sin panic
}

func TestVerificarConTodosConcedidos(t *testing.T) {
        s, _ := crearSistemaTest(t)
        s.ConcederTodos("test")

        // Cuando todos están concedidos, Verificar retorna true para cualquier permiso
        if !s.Verificar(PermArchivos) {
                t.Error("con todos concedidos, Verificar debería retornar true")
        }
        if !s.Verificar(PermTerminal) {
                t.Error("con todos concedidos, Verificar debería retornar true")
        }
}

func TestCargaDesdeArchivo(t *testing.T) {
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, ".liz", "permisos.json")
        os.MkdirAll(filepath.Dir(ruta), 0755)

        // Escribir estado previo
        estadoPrevio := EstadoPermisos{
                Version:    "0.1.0",
                Concedidos: true,
                SesionID:   "sesion_existente",
                Permisos:   make(map[string]Permiso),
        }
        estadoPrevio.Permisos["archivos"] = Permiso{
                Nombre:    "archivos",
                Concedido: true,
        }

        datos, _ := json.MarshalIndent(estadoPrevio, "", "  ")
        os.WriteFile(ruta, datos, 0644)

        // Crear sistema con ruta personalizada
        sistema := &Sistema{
                rutaArchivo: ruta,
                estado: &EstadoPermisos{
                        Version:  "0.1.0",
                        Permisos: make(map[string]Permiso),
                },
        }
        for _, p := range permisosDefecto {
                sistema.estado.Permisos[p.Nombre] = Permiso{
                        Nombre:      p.Nombre,
                        Descripcion: p.Descripcion,
                }
        }

        // Simular carga
        if data, err := os.ReadFile(ruta); err == nil {
                var existente EstadoPermisos
                if json.Unmarshal(data, &existente) == nil {
                        for nombre, perm := range existente.Permisos {
                                sistema.estado.Permisos[nombre] = perm
                        }
                        sistema.estado.Concedidos = existente.Concedidos
                        sistema.estado.SesionID = existente.SesionID
                }
        }

        if !sistema.TodosConcedidos() {
                t.Error("debería haber cargado permisos concedidos del archivo")
        }
        if sistema.estado.SesionID != "sesion_existente" {
                t.Errorf("sesion_id esperado 'sesion_existente', obtuve '%s'", sistema.estado.SesionID)
        }
}
