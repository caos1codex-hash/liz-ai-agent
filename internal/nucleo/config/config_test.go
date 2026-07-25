package config

import (
        "os"
        "path/filepath"
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
        // Crear archivo temporal
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

func TestAsegurarDirectorios(t *testing.T) {
        tmpDir := t.TempDir()

        // Crear estructura esperada
        dirs := []string{
                filepath.Join(tmpDir, ".liz"),
                filepath.Join(tmpDir, ".liz", "logs"),
                filepath.Join(tmpDir, ".liz", "contexto", "sistema", "estado"),
                filepath.Join(tmpDir, ".liz", "herramientas", "auto_creadas"),
        }

        for _, dir := range dirs {
                if err := os.MkdirAll(dir, 0755); err != nil {
                        t.Fatalf("error creando dir temporal: %v", err)
                }
        }

        // Verificar que existen
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
