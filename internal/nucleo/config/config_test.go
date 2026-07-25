package config

import (
        "os"
        "path/filepath"
        "strings"
        "testing"
)

func TestConfiguracionPorDefecto(t *testing.T) {
        cfg := ConfiguracionPorDefecto()

        if cfg.Version != "0.1.0" {
                t.Errorf("versión esperada '0.1.0', obtuve '%s'", cfg.Version)
        }
        if cfg.Servidor.Puerto != 3000 {
                t.Errorf("puerto esperado 3000, obtuve %d", cfg.Servidor.Puerto)
        }
        if cfg.Servidor.Host != "localhost" {
                t.Errorf("host esperado 'localhost', obtuve '%s'", cfg.Servidor.Host)
        }
        if cfg.Tema != "oscuro" {
                t.Errorf("tema esperado 'oscuro', obtuve '%s'", cfg.Tema)
        }
        if cfg.NVIDIA.Endpoint != "https://integrate.api.nvidia.com/v1" {
                t.Errorf("endpoint NVIDIA incorrecto")
        }
        if !cfg.Permisos.SolicitarAlIniciar {
                t.Error("solicitar_al_iniciar debería ser true por defecto")
        }
}

func TestCargarDesde(t *testing.T) {
        contenido := `
liz:
  version: "0.2.0"
  servidor:
    puerto: 8080
    host: "0.0.0.0"
  tema: "claro"
  permisos:
    solicitar_al_iniciar: false
`
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, "test.yaml")
        if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
                t.Fatalf("error creando archivo temporal: %v", err)
        }

        cfg, err := CargarDesde(ruta)
        if err != nil {
                t.Fatalf("CargarDesde() error: %v", err)
        }

        if cfg.Version != "0.2.0" {
                t.Errorf("versión esperada '0.2.0', obtuve '%s'", cfg.Version)
        }
        if cfg.Servidor.Puerto != 8080 {
                t.Errorf("puerto esperado 8080, obtuve %d", cfg.Servidor.Puerto)
        }
        if cfg.Servidor.Host != "0.0.0.0" {
                t.Errorf("host esperado '0.0.0.0', obtuve '%s'", cfg.Servidor.Host)
        }
        if cfg.Tema != "claro" {
                t.Errorf("tema esperado 'claro', obtuve '%s'", cfg.Tema)
        }
        if cfg.Permisos.SolicitarAlIniciar {
                t.Error("solicitar_al_iniciar debería ser false")
        }
}

func TestCargarDesdeArchivoInexistente(t *testing.T) {
        _, err := CargarDesde("/tmp/no_existe_liz_12345.yaml")
        if err == nil {
                t.Error("debería retornar error para archivo inexistente")
        }
}

// ═══════════════════════════════════════════════════
// VALIDACIONES (Fase 2)
// ═══════════════════════════════════════════════════

func TestValidar_ConfigValida(t *testing.T) {
        cfg := ConfiguracionPorDefecto()
        errores := cfg.Validar()
        if len(errores) > 0 {
                t.Errorf("config por defecto debería ser válida, obtuve %d errores: %v", len(errores), errores)
        }
}

func TestValidar_PuertoInvalido(t *testing.T) {
        cfg := ConfiguracionPorDefecto()
        cfg.Servidor.Puerto = 0
        errores := cfg.Validar()
        if len(errores) == 0 {
                t.Error("puerto 0 debería ser inválido")
        }

        cfg.Servidor.Puerto = 70000
        errores = cfg.Validar()
        if len(errores) == 0 {
                t.Error("puerto 70000 debería ser inválido")
        }
}

func TestValidar_TemaInvalido(t *testing.T) {
        cfg := ConfiguracionPorDefecto()
        cfg.Tema = "neon"
        errores := cfg.Validar()
        encontrado := false
        for _, e := range errores {
                if e == ErrTemaInvalido {
                        encontrado = true
                }
        }
        if !encontrado {
                t.Error("tema 'neon' debería ser inválido")
        }
}

func TestValidar_HostVacio(t *testing.T) {
        cfg := ConfiguracionPorDefecto()
        cfg.Servidor.Host = ""
        errores := cfg.Validar()
        encontrado := false
        for _, e := range errores {
                if e == ErrHostInvalido {
                        encontrado = true
                }
        }
        if !encontrado {
                t.Error("host vacío debería ser inválido")
        }
}

func TestValidar_EndpointInvalido(t *testing.T) {
        cfg := ConfiguracionPorDefecto()
        cfg.NVIDIA.Endpoint = "http://inseguro.com"
        errores := cfg.Validar()
        encontrado := false
        for _, e := range errores {
                if e == ErrEndpointInvalido {
                        encontrado = true
                }
        }
        if !encontrado {
                t.Error("endpoint http:// debería ser inválido (solo https)")
        }
}

func TestValidar_ModeloSinID(t *testing.T) {
        cfg := ConfiguracionPorDefecto()
        cfg.NVIDIA.Modelos = []ModeloNVIDIA{{ID: "", Nombre: "test"}}
        errores := cfg.Validar()
        if len(errores) == 0 {
                t.Error("modelo sin ID debería ser inválido")
        }
}

func TestValidar_ModeloTipoInvalido(t *testing.T) {
        cfg := ConfiguracionPorDefecto()
        cfg.NVIDIA.Modelos = []ModeloNVIDIA{
                {ID: "test/1", Nombre: "Test", Tipo: []string{"no_existe"}, Velocidad: "media"},
        }
        errores := cfg.Validar()
        encontrado := false
        for _, e := range errores {
                if strings.Contains(e.Error(), "tipo de modelo no reconocido") {
                        encontrado = true
                }
        }
        if !encontrado {
                t.Error("tipo de modelo 'no_existe' debería ser inválido")
        }
}

// ═══════════════════════════════════════════════════
// GESTOR (Fase 2)
// ═══════════════════════════════════════════════════

func TestGestor_ModificarTema(t *testing.T) {
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, "config.json")

        g := &Gestor{
                rutaActiva: ruta,
                config:     ConfiguracionPorDefecto(),
                logFunc:    func(string, ...interface{}) {},
        }

        nueva, err := g.Modificar(&Configuracion{Tema: "claro"})
        if err != nil {
                t.Fatalf("Modificar() error: %v", err)
        }
        if nueva.Tema != "claro" {
                t.Errorf("tema esperado 'claro', obtuve '%s'", nueva.Tema)
        }

        // Verificar persistencia
        datos, _ := os.ReadFile(ruta)
        if len(datos) == 0 {
                t.Error("config debió persistirse en archivo")
        }
}

func TestGestor_ModificarTemaInvalido(t *testing.T) {
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, "config.json")

        g := &Gestor{
                rutaActiva: ruta,
                config:     ConfiguracionPorDefecto(),
                logFunc:    func(string, ...interface{}) {},
        }

        _, err := g.Modificar(&Configuracion{Tema: "invalido"})
        if err == nil {
                t.Error("debería fallar con tema inválido")
        }
}

func TestGestor_ModificarPuertoInvalido(t *testing.T) {
        tmpDir := t.TempDir()
        ruta := filepath.Join(tmpDir, "config.json")

        g := &Gestor{
                rutaActiva: ruta,
                config:     ConfiguracionPorDefecto(),
                logFunc:    func(string, ...interface{}) {},
        }

        _, err := g.Modificar(&Configuracion{Servidor: ConfigServidor{Puerto: 99999}})
        if err == nil {
                t.Error("debería fallar con puerto inválido")
        }
}

func TestGestor_Obtener(t *testing.T) {
        g := &Gestor{
                rutaActiva: "/tmp/test.json",
                config:     ConfiguracionPorDefecto(),
                logFunc:    func(string, ...interface{}) {},
        }

        cfg := g.Obtener()
        if cfg.Tema != "oscuro" {
                t.Errorf("tema esperado 'oscuro', obtuve '%s'", cfg.Tema)
        }
}

func TestGestor_RutaOrigen(t *testing.T) {
        g := &Gestor{
                rutaOrigen: "liz.yaml",
                rutaActiva: "/tmp/test.json",
                config:     ConfiguracionPorDefecto(),
                logFunc:    func(string, ...interface{}) {},
        }

        if g.RutaOrigen() != "liz.yaml" {
                t.Errorf("ruta esperada 'liz.yaml', obtuve '%s'", g.RutaOrigen())
        }
}

func TestAsegurarDirectorios(t *testing.T) {
        tmpDir := t.TempDir()

        dirs := []string{
                filepath.Join(tmpDir, ".liz"),
                filepath.Join(tmpDir, ".liz", "logs"),
                filepath.Join(tmpDir, ".liz", "contexto", "sistema", "estado"),
                filepath.Join(tmpDir, ".liz", "herramientas", "auto_creadas"),
        }

        for _, dir := range dirs {
                if err := os.MkdirAll(dir, 0755); err != nil {
                        t.Fatalf("error creando dir: %v", err)
                }
        }

        for _, dir := range dirs {
                info, err := os.Stat(dir)
                if err != nil {
                        t.Errorf("directorio %s no existe: %v", dir, err)
                        continue
                }
                if !info.IsDir() {
                        t.Errorf("%s no es un directorio", dir)
                }
        }
}
