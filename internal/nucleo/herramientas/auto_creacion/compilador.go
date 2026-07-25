package auto_creacion

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// Compilador — compila fuente Go a binario standalone
// ============================================================================

// Compilador toma el fuente Go de una herramienta y lo compila a un binario
// standalone usando `go build`. El binario resultante se comunica con Liz
// vía JSON sobre stdin/stdout (ver doc.go).
//
// Decisiones:
//   - Usa el `go` del PATH (o configurable via Compilador.GoBin).
//   - Compila con GOFLAGS=-mod=mod para tolerar ausencia de go.mod.
//   - Timeout configurable (default 60s). Compilaciones grandes pueden tardar.
//   - Captura stdout+stderr combinados para diagnóstico.
//   - El fuente se escribe en un directorio temporal del tool (no /tmp) para
//     que el binario quede persistente junto a la metadata.
type Compilador struct {
	// GoBin es la ruta al binario `go`. Si vacío, usa "go" del PATH.
	GoBin string

	// Timeout máximo de compilación. Default 60s.
	Timeout time.Duration

	// LogFunc opcional.
	LogFunc func(formato string, args ...interface{})
}

// NuevoCompilador crea un Compilador con defaults sensatos.
func NuevoCompilador() *Compilador {
	return &Compilador{
		GoBin:   "",
		Timeout: 60 * time.Second,
		LogFunc: func(string, ...interface{}) {},
	}
}

// ConGoBin setea la ruta al binario go.
func (c *Compilador) ConGoBin(path string) *Compilador {
	c.GoBin = path
	return c
}

// ConTimeout setea el timeout de compilación.
func (c *Compilador) ConTimeout(d time.Duration) *Compilador {
	c.Timeout = d
	return c
}

// ConLog inyecta un logger opcional.
func (c *Compilador) ConLog(fn func(formato string, args ...interface{})) *Compilador {
	if fn != nil {
		c.LogFunc = fn
	}
	return c
}

// goExecutable retorna la ruta al binario go a usar.
func (c *Compilador) goExecutable() string {
	if c.GoBin != "" {
		return c.GoBin
	}
	// Intentar local instalado por el entorno (~/go-local/go/bin/go)
	if home, err := os.UserHomeDir(); err == nil {
		local := filepath.Join(home, "go-local", "go", "bin", "go")
		if _, err := os.Stat(local); err == nil {
			return local
		}
	}
	return "go"
}

// Compilar escribe el fuente a disco y ejecuta `go build` para producir el
// binario.
//
// Args:
//   - dirHerramienta: directorio donde vivirán fuente+binario+metadata. Se crea si no existe.
//   - fuenteGo: código Go completo (package main).
//
// Retorna ResultadoCompilacion con detalles (ruta binario, logs, duración).
// Si la compilación falla, ResultadoCompilacion.Exito=false y ResultadoCompilacion.Log
// contiene el stderr de go build.
func (c *Compilador) Compilar(ctx context.Context, dirHerramienta string, fuenteGo string) (*ResultadoCompilacion, error) {
	if fuenteGo == "" {
		return nil, &ErrAutoCreacion{Etapa: "compilacion", Causa: "fuente vacío"}
	}
	if err := ValidarFuenteGo(fuenteGo); err != nil {
		return nil, &ErrAutoCreacion{
			Etapa: "compilacion", Causa: "fuente no pasa validación básica", Interno: err,
		}
	}

	// Crear directorio si no existe
	if err := os.MkdirAll(dirHerramienta, 0o755); err != nil {
		return nil, &ErrAutoCreacion{
			Etapa: "compilacion", Causa: "creando directorio", Interno: err,
		}
	}

	rutaFuente := filepath.Join(dirHerramienta, "fuente.go")
	rutaBinario := filepath.Join(dirHerramienta, "herramienta")
	rutaLog := filepath.Join(dirHerramienta, "compilacion.log")

	// Escribir fuente
	if err := os.WriteFile(rutaFuente, []byte(fuenteGo), 0o644); err != nil {
		return nil, &ErrAutoCreacion{
			Etapa: "compilacion", Causa: "escribiendo fuente.go", Interno: err,
		}
	}

	// Compilar
	c.LogFunc("compilando %s → %s", rutaFuente, rutaBinario)
	inicio := time.Now()

	goBin := c.goExecutable()
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	ctxBuild, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 — goBin es controlado por el operador, argumentos son fijos
	cmd := exec.CommandContext(ctxBuild, goBin, "build", "-o", rutaBinario, rutaFuente)
	cmd.Dir = dirHerramienta

	// Capturar stdout y stderr combinados
	output, err := cmd.CombinedOutput()
	duracion := time.Since(inicio)

	// Guardar log siempre (útil para depurar incluso en éxito)
	_ = os.WriteFile(rutaLog, output, 0o644)

	resultado := &ResultadoCompilacion{
		RutaFuente: rutaFuente,
		Duracion:   duracion,
		Log:        string(output),
	}

	if err != nil {
		resultado.Exito = false
		// Si fue timeout, el error lo aclaramos
		if ctxBuild.Err() == context.DeadlineExceeded {
			resultado.Log = fmt.Sprintf("TIMEOUT después de %s\n%s", timeout, resultado.Log)
		}
		c.LogFunc("compilación FALLÓ (%s): %v", duracion, err)
		return resultado, &ErrAutoCreacion{
			Etapa: "compilacion", Causa: "go build falló", Interno: err,
		}
	}

	// Verificar que el binario se creó
	info, err := os.Stat(rutaBinario)
	if err != nil {
		resultado.Exito = false
		c.LogFunc("binario no encontrado tras compilación exitosa: %v", err)
		return resultado, &ErrAutoCreacion{
			Etapa: "compilacion", Causa: "binario no creado", Interno: err,
		}
	}

	// Hacer ejecutable (en caso de que go build no lo haga, ej. en algunos filesystems)
	_ = os.Chmod(rutaBinario, 0o755)

	resultado.Exito = true
	resultado.RutaBinario = rutaBinario
	c.LogFunc("compilación OK (%s, binario: %s, %d bytes)",
		duracion, rutaBinario, info.Size())

	return resultado, nil
}

// LimpiarArtifacts elimina fuente+binario+log de un directorio de herramienta.
// Útil cuando se elimina una herramienta del registro.
func (c *Compilador) LimpiarArtifacts(dirHerramienta string) error {
	artifacts := []string{"fuente.go", "herramienta", "compilacion.log", "metadata.json"}
	var errs []string
	for _, a := range artifacts {
		p := filepath.Join(dirHerramienta, a)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Sprintf("%s: %v", a, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errores limpiando: %s", strings.Join(errs, "; "))
	}
	// Eliminar el directorio si está vacío
	_ = os.Remove(dirHerramienta)
	return nil
}

// GoDisponible verifica que el binario `go` esté disponible y retorna su versión.
// Útil para health checks.
func (c *Compilador) GoDisponible() (string, error) {
	goBin := c.goExecutable()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// #nosec G204 — goBin controlado
	out, err := exec.CommandContext(ctx, goBin, "version").Output()
	if err != nil {
		return "", fmt.Errorf("go no disponible en %s: %w", goBin, err)
	}
	return strings.TrimSpace(string(out)), nil
}
