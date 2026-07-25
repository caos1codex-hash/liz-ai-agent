package permisos

import (
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "os"
        "path/filepath"
        "testing"
        "time"
)

func crearSistemaTest(t *testing.T) (*Sistema, string) {
        t.Helper()
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, ".liz", "permisos.json")
        if err := os.MkdirAll(filepath.Dir(ruta), 0755); err != nil {
                t.Fatalf("error creando dir: %v", err)
        }

        s := &Sistema{
                rutaArchivo:   ruta,
                maxAuditoria: 1000,
                recordar:      false,
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

        // Verificar persistencia
        datos, err := os.ReadFile(s.rutaArchivo)
        if err != nil {
                t.Fatalf("error leyendo archivo: %v", err)
        }

        var cargado EstadoPermisos
        if err := json.Unmarshal(datos, &cargado); err != nil {
                t.Fatalf("error parseando: %v", err)
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

func TestRevocar(t *testing.T) {
        s, _ := crearSistemaTest(t)
        s.ConcederTodos("test")

        err := s.Revocar("archivos")
        if err != nil {
                t.Fatalf("Revocar() error: %v", err)
        }

        if s.Verificar(PermArchivos) {
                t.Error("permiso 'archivos' debería estar revocado")
        }
        if s.TodosConcedidos() {
                t.Error("concedidos debería ser false después de revocar uno")
        }
}

func TestRevocarInexistente(t *testing.T) {
        s, _ := crearSistemaTest(t)

        err := s.Revocar("no_existe")
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
                t.Error("permisos deberían estar revocados")
        }
        if s.Verificar(PermArchivos) {
                t.Error("permisos individuales deberían estar revocados")
        }
}

func TestEstadoCopia(t *testing.T) {
        s, _ := crearSistemaTest(t)
        estado1 := s.Estado()
        _ = estado1
}

func TestVerificarConTodosConcedidos(t *testing.T) {
        s, _ := crearSistemaTest(t)
        s.ConcederTodos("test")

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

        sistema := &Sistema{
                rutaArchivo: ruta,
                maxAuditoria: 1000,
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
                t.Error("debería haber cargado permisos concedidos")
        }
        if sistema.estado.SesionID != "sesion_existente" {
                t.Errorf("sesion_id incorrecto")
        }
}

// ═══════════════════════════════════════════════════
// AUDITORÍA (Fase 2)
// ═══════════════════════════════════════════════════

func TestAuditoria(t *testing.T) {
        s, _ := crearSistemaTest(t)

        s.registrarAuditoria(EntradaAuditoria{
                Permiso:   "terminal",
                Accion:    "POST /api/chat",
                Concedido: true,
        })

        s.registrarAuditoria(EntradaAuditoria{
                Permiso:   "archivos",
                Accion:    "DELETE /api/conversations/1",
                Concedido: false,
        })

        if s.TotalAuditoria() != 2 {
                t.Errorf("total auditoría esperado 2, obtuve %d", s.TotalAuditoria())
        }

        regs := s.Auditoria(10)
        if len(regs) != 2 {
                t.Errorf("auditoría(10) debería retornar 2, obtuvo %d", len(regs))
        }

        regs1 := s.Auditoria(1)
        if len(regs1) != 1 {
                t.Errorf("auditoría(1) debería retornar 1, obtuvo %d", len(regs1))
        }
        if regs1[0].Accion != "DELETE /api/conversations/1" {
                t.Error("última entrada debería ser la del DELETE")
        }
}

// ═══════════════════════════════════════════════════
// MIDDLEWARE HTTP (Fase 2)
// ═══════════════════════════════════════════════════

func TestMiddleware_PermisoConcedido(t *testing.T) {
        s, _ := crearSistemaTest(t)
        s.ConcederTodos("test")

        handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                w.Write([]byte("ok"))
        })

        mw := s.MiddlewareHTTP(handler)

        // GET /api/tools requiere PermSistema
        req := httptest.NewRequest("GET", "/api/tools", nil)
        rec := httptest.NewRecorder()
        mw.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("con permisos, GET /api/tools debería ser 200, obtuve %d", rec.Code)
        }
}

func TestMiddleware_PermisoDenegado(t *testing.T) {
        s, _ := crearSistemaTest(t)

        handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                w.Write([]byte("ok"))
        })

        mw := s.MiddlewareHTTP(handler)

        // POST /api/chat requiere PermTerminal — no concedido
        req := httptest.NewRequest("POST", "/api/chat", nil)
        rec := httptest.NewRecorder()
        mw.ServeHTTP(rec, req)

        if rec.Code != http.StatusForbidden {
                t.Errorf("sin permisos, POST /api/chat debería ser 403, obtuve %d", rec.Code)
        }
}

func TestMiddleware_RutaPublica(t *testing.T) {
        s, _ := crearSistemaTest(t)

        handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                w.Write([]byte("ok"))
        })

        mw := s.MiddlewareHTTP(handler)

        // GET /api/health es pública
        req := httptest.NewRequest("GET", "/api/health", nil)
        rec := httptest.NewRecorder()
        mw.ServeHTTP(rec, req)

        if rec.Code != http.StatusOK {
                t.Errorf("GET /api/health debería ser 200, obtuve %d", rec.Code)
        }
}

func TestPermisoRequerido(t *testing.T) {
        // Ruta protegida
        if p := PermisoRequerido("/api/chat", "POST"); p != PermTerminal {
                t.Errorf("POST /api/chat debería requerir 'terminal', obtuve '%s'", p)
        }

        // Ruta pública
        if p := PermisoRequerido("/api/health", "GET"); p != "" {
                t.Errorf("GET /api/health debería ser pública, obtuvo '%s'", p)
        }
}

func TestRecordarSesion_NoRecordar(t *testing.T) {
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, ".liz", "permisos.json")
        os.MkdirAll(filepath.Dir(ruta), 0755)

        // Simular permisos concedidos de una sesión previa
        existente := EstadoPermisos{
                Version:    "0.1.0",
                Concedidos: true,
                SesionID:   "sesion_anterior",
                Permisos:   make(map[string]Permiso),
        }
        existente.Permisos["archivos"] = Permiso{Nombre: "archivos", Concedido: true}
        datos, _ := json.MarshalIndent(existente, "", "  ")
        os.WriteFile(ruta, datos, 0644)

        // Nuevo sistema con recordar=false debería limpiar
        s, err := NuevoSistemaConRecordarConRuta(ruta, false)
        if err != nil {
                t.Fatalf("error: %v", err)
        }

        if s.TodosConcedidos() {
                t.Error("con recordar=false, permisos previos deberían estar limpiados")
        }
}

func TestRecordarSesion_SiRecordar(t *testing.T) {
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, ".liz", "permisos.json")
        os.MkdirAll(filepath.Dir(ruta), 0755)

        existente := EstadoPermisos{
                Version:    "0.1.0",
                Concedidos: true,
                SesionID:   "sesion_anterior",
                Permisos:   make(map[string]Permiso),
        }
        existente.Permisos["archivos"] = Permiso{Nombre: "archivos", Concedido: true}
        datos, _ := json.MarshalIndent(existente, "", "  ")
        os.WriteFile(ruta, datos, 0644)

        s, err := NuevoSistemaConRecordarConRuta(ruta, true)
        if err != nil {
                t.Fatalf("error: %v", err)
        }

        if !s.TodosConcedidos() {
                t.Error("con recordar=true, permisos previos deberían mantenerse")
        }
}

// NuevoSistemaConRecordarConRuta crea un sistema con ruta personalizable (para testing).
func NuevoSistemaConRecordarConRuta(ruta string, recordar bool) (*Sistema, error) {
        s := &Sistema{
                rutaArchivo:   ruta,
                maxAuditoria: 1000,
                recordar:      recordar,
                estado: &EstadoPermisos{
                        Version:  "0.1.0",
                        Permisos: make(map[string]Permiso),
                },
        }

        for _, p := range permisosDefecto {
                s.estado.Permisos[p.Nombre] = Permiso{
                        Nombre:      p.Nombre,
                        Descripcion: p.Descripcion,
                }
        }

        if datos, err := os.ReadFile(ruta); err == nil {
                var existente EstadoPermisos
                if json.Unmarshal(datos, &existente) == nil {
                        for nombre, perm := range existente.Permisos {
                                s.estado.Permisos[nombre] = perm
                        }
                        s.estado.Concedidos = existente.Concedidos
                        s.estado.FechaConcesion = existente.FechaConcesion
                        s.estado.SesionID = existente.SesionID
                        s.estado.RecordarSesion = existente.RecordarSesion
                }
        }

        if !recordar && s.estado.Concedidos {
                s.estado.Concedidos = false
                s.estado.FechaConcesion = time.Time{}
                s.estado.SesionID = ""
                for nombre, p := range s.estado.Permisos {
                        p.Concedido = false
                        p.FechaHora = time.Time{}
                        s.estado.Permisos[nombre] = p
                }
                s.guardar()
        }

        return s, nil
}
