// Package integradas contiene las 7 herramientas que vienen incluidas con Liz.
//
// Las herramientas integradas son la base sobre la que Liz opera el sistema.
// Cada una implementa herramientas.Herramienta y se registra en el catálogo
// al iniciar.
//
// Las 7 herramientas (ver docs/ARQUITECTURA.md sección 6 y roadmap Fase 5):
//   - terminal            — ejecución de comandos shell
//   - navegador_archivos  — navegación de directorios
//   - buscador            — búsqueda de archivos por patrón/contenido
//   - editor              — lectura/escritura/modificación de archivos
//   - procesos            — gestión de procesos (ps, kill)
//   - monitor             — métricas de sistema (CPU, RAM, disco, red)
//   - instalador          — instalación de software (apt, snap, pip, npm)
package integradas

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Terminal — Ejecución de Comandos Shell
// ============================================================================

// DefaultTimeoutComando es el timeout por defecto para comandos (30s).
// Los comandos que excedan este timeout son cancelados.
const DefaultTimeoutComando = 30 * time.Second

// MaxOutputComando es el tamaño máximo de stdout/stderr capturado (1MB).
// Si un comando produce más salida, se trunca con un mensaje al final.
const MaxOutputComando = 1 * 1024 * 1024

// ComandosPeligrososEs el conjunto de patrones que requieren confirmación
// explícita vía el parámetro "peligroso_confirma=true".
// Liz tiene permisos totales (D-006), pero esto protege contra accidents.
var ComandosPeligrosos = []string{
	"rm -rf /",
	"rm -rf /*",
	"mkfs",
	"dd if=/dev/zero of=/dev/sda",
	":(){:|:&};:", // fork bomb
	"chmod -R 000",
	"shutdown",
	"halt",
	"reboot",
	"init 0",
	"init 6",
}

// Terminal ejecuta comandos en la shell del sistema.
//
// Características:
//   - Timeout configurable (default 30s, max 5min)
//   - Captura stdout + stderr combinados (o separados)
//   - Working directory configurable
//   - Variables de entorno extra
//   - Detección de comandos peligrosos (requiere flag confirmación)
//   - Respeto de ctx.Done() (cancellation limpia vía SIGKILL)
//   - Limitación de output (1MB) para evitar OOM
//
// NO soporta shell pipes (`|`) ni redirecciones (`>`) implícitamente —
// para eso se debe usar shell=true (que envuelve en `sh -c "..."`).
type Terminal struct {
	timeoutDefault time.Duration
}

// NewTerminal crea una instancia con el timeout por defecto.
func NewTerminal() *Terminal {
	return &Terminal{timeoutDefault: DefaultTimeoutComando}
}

// Compile-time check de interfaz.
var _ herramientas.Herramienta = (*Terminal)(nil)

func (t *Terminal) Nombre() string { return "terminal" }

func (t *Terminal) Descripcion() string {
	return "Ejecuta comandos en la terminal del sistema. Soporta timeout, " +
		"working directory, variables de entorno, y captura stdout/stderr. " +
		"Detección de comandos peligrosos."
}

func (t *Terminal) Parametros() []herramientas.Parametro {
	return []herramientas.Parametro{
		{
			Nombre:    "comando",
			Tipo:      "string",
			Requerido: true,
			Descripcion: "Comando a ejecutar (sin argumentos si se usa 'args'). " +
				"Si se usa solo, se ejecuta directamente sin shell.",
		},
		{
			Nombre: "args",
			Tipo:   "array",
			Items:  "string",
			Descripcion: "Lista de argumentos para el comando. " +
				"Si se omite y shell=false, el comando se ejecuta tal cual.",
		},
		{
			Nombre:  "shell",
			Tipo:    "bool",
			Default: false,
			Descripcion: "Si true, ejecuta vía 'sh -c comando' (permite pipes, " +
				"redirecciones, &&, etc.). Más lento y menos seguro.",
		},
		{
			Nombre:      "directorio",
			Tipo:        "string",
			Descripcion: "Directorio de trabajo. Default: directorio actual.",
		},
		{
			Nombre:      "timeout_segundos",
			Tipo:        "int",
			Default:     30,
			Min:         float64Ptr(1),
			Max:         float64Ptr(300),
			Descripcion: "Timeout en segundos. Si el comando excede, se cancela.",
		},
		{
			Nombre: "env",
			Tipo:   "object",
			Descripcion: "Variables de entorno extra (key=value). " +
				"Se añaden a las existentes, no las reemplazan.",
		},
		{
			Nombre:  "peligroso_confirma",
			Tipo:    "bool",
			Default: false,
			Descripcion: "Debe ser true para ejecutar comandos en la lista " +
				"de patrones peligrosos (rm -rf /, mkfs, shutdown, etc.).",
		},
		{
			Nombre:  "combinar_stdout_stderr",
			Tipo:    "bool",
			Default: true,
			Descripcion: "Si true, stderr se mezcla con stdout. " +
				"Si false, se capturan por separado.",
		},
	}
}

// ResultadoTerminal es el Datos del Resultado de Terminal.
type ResultadoTerminal struct {
	Comando      string   `json:"comando"`
	Args         []string `json:"args,omitempty"`
	Stdout       string   `json:"stdout"`
	Stderr       string   `json:"stderr,omitempty"`
	CodigoSalida int      `json:"codigo_salida"`
	DuracionMs   float64  `json:"duracion_ms"`
	Timeout      bool     `json:"timeout,omitempty"`
	Truncado     bool     `json:"truncado,omitempty"`
	Peligroso    bool     `json:"peligroso,omitempty"`
	Directorio   string   `json:"directorio,omitempty"`
}

func (t *Terminal) Validar() error {
	// No hay dependencias externas — siempre es válido
	return nil
}

// Ejecutar implementa herramientas.Herramienta.Ejecutar.
func (t *Terminal) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	// Extraer parámetros
	comando, err := herramientas.ObtenerString(params, t.paramByName("comando"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	shell, err := herramientas.ObtenerBool(params, t.paramByName("shell"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	directorio, err := herramientas.ObtenerString(params, t.paramByName("directorio"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	timeoutSeg, err := herramientas.ObtenerInt(params, t.paramByName("timeout_segundos"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}
	if timeoutSeg <= 0 {
		timeoutSeg = 30
	}

	args, err := herramientas.ObtenerArrayString(params, t.paramByName("args"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	confirma, err := herramientas.ObtenerBool(params, t.paramByName("peligroso_confirma"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	combinar, err := herramientas.ObtenerBool(params, t.paramByName("combinar_stdout_stderr"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	envExtra := extraerEnv(params)

	// Construir comando completo para detección de peligroso
	comandoCompleto := comando
	if len(args) > 0 {
		comandoCompleto += " " + strings.Join(args, " ")
	}
	peligroso := esComandoPeligroso(comandoCompleto)
	if peligroso && !confirma {
		return herramientas.Resultado{
			Exito: false,
			Error: fmt.Sprintf("comando potencialmente peligroso detectado: %q. "+
				"Requiere 'peligroso_confirma=true' para ejecutar.", comandoCompleto),
			Metadata: herramientas.NuevaMetadata(0),
		}, nil
	}

	// Crear contexto con timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeg)*time.Second)
	defer cancel()

	// Construir Cmd
	var cmd *exec.Cmd
	if shell {
		// sh -c "comando args..."
		fullCmd := comando
		if len(args) > 0 {
			fullCmd += " " + strings.Join(args, " ")
		}
		cmd = exec.CommandContext(ctxTimeout, "sh", "-c", fullCmd)
	} else {
		cmd = exec.CommandContext(ctxTimeout, comando, args...)
	}

	if directorio != "" {
		cmd.Dir = directorio
	}

	// Variables de entorno
	if len(envExtra) > 0 {
		cmd.Env = append(cmd.Env, envExtra...)
	}

	// Capturar output
	var stdout, stderr bytes.Buffer
	if combinar {
		cmd.Stdout = &stdout
		cmd.Stderr = &stdout
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	inicio := time.Now()
	err = cmd.Run()
	duracion := time.Since(inicio)

	// Detectar timeout
	timeout := ctxTimeout.Err() == context.DeadlineExceeded

	// Truncar output si excede límite
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	truncado := false
	if len(stdoutStr) > MaxOutputComando {
		stdoutStr = stdoutStr[:MaxOutputComando] + "\n...[TRUNCADO: output > 1MB]..."
		truncado = true
	}
	if len(stderrStr) > MaxOutputComando {
		stderrStr = stderrStr[:MaxOutputComando] + "\n...[TRUNCADO: output > 1MB]..."
		truncado = true
	}

	// Determinar código de salida
	codigoSalida := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			codigoSalida = exitErr.ExitCode()
		} else if timeout {
			codigoSalida = -1
		} else {
			// Otro error (comando no encontrado, etc.)
			return herramientas.Resultado{
				Exito: false,
				Error: fmt.Sprintf("error al ejecutar comando: %v", err),
				Metadata: herramientas.NuevaMetadata(
					float64(duracion.Microseconds()) / 1000.0),
			}, nil
		}
	}

	// Exito: código de salida 0 (o timeout, que reportamos pero marcamos fallo)
	exito := codigoSalida == 0 && !timeout

	datos := ResultadoTerminal{
		Comando:      comando,
		Args:         args,
		Stdout:       stdoutStr,
		Stderr:       stderrStr,
		CodigoSalida: codigoSalida,
		DuracionMs:   float64(duracion.Microseconds()) / 1000.0,
		Timeout:      timeout,
		Truncado:     truncado,
		Peligroso:    peligroso,
		Directorio:   directorio,
	}

	msg := "comando ejecutado"
	if exito {
		msg = fmt.Sprintf("comando '%s' completado (exit 0)", comando)
	} else if timeout {
		msg = fmt.Sprintf("comando '%s' cancelado por timeout (%ds)", comando, timeoutSeg)
	} else {
		msg = fmt.Sprintf("comando '%s' falló (exit %d)", comando, codigoSalida)
	}

	return herramientas.Resultado{
		Exito: exito,
		Datos: datos,
		Error: func() string {
			if exito {
				return ""
			}
			return msg
		}(),
		Metadata: herramientas.NuevaMetadata(datos.DuracionMs),
	}, nil
}

// paramByName helper para buscar un parámetro por nombre en Parametros().
func (t *Terminal) paramByName(nombre string) herramientas.Parametro {
	for _, p := range t.Parametros() {
		if p.Nombre == nombre {
			return p
		}
	}
	return herramientas.Parametro{Nombre: nombre}
}

// float64Ptr helper para crear punteros a float64 (para Min/Max).
func float64Ptr(v float64) *float64 {
	return &v
}

// esComandoPeligroso verifica si el comando coincide con patrones peligrosos.
// La comparación es case-insensitive y normaliza espacios múltiples.
func esComandoPeligroso(comando string) bool {
	normalizado := strings.ToLower(strings.Join(
		strings.Fields(comando), " "))
	for _, patron := range ComandosPeligrosos {
		patronNorm := strings.ToLower(strings.Join(
			strings.Fields(patron), " "))
		if strings.Contains(normalizado, patronNorm) {
			return true
		}
	}
	return false
}

// extraerEnv obtiene el parámetro "env" como lista de strings "K=V".
func extraerEnv(params map[string]interface{}) []string {
	val, ok := params["env"]
	if !ok || val == nil {
		return nil
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, fmt.Sprintf("%s=%v", k, v))
	}
	return result
}

// SanitizarParaLog remueve caracteres no imprimibles de un output
// para logging seguro.
func SanitizarParaLog(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if unicode.IsPrint(r) || r == '\n' || r == '\t' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('?')
		}
	}
	return sb.String()
}
