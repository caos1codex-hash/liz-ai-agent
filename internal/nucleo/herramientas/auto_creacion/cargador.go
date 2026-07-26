package auto_creacion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Cargador — wrapper que expone un binario subprocess como Herramienta
// ============================================================================

// HerramientaSubproceso implementa herramientas.Herramienta delegando cada
// llamada a un binario externo vía JSON sobre stdin/stdout (ver doc.go).
//
// Es thread-safe: múltiples goroutines pueden llamar a Ejecutar concurrentemente.
// La información (nombre, descripción, parámetros) se cachea tras la primera
// invocación a operacion="info".
//
// Compile-time check de interfaz:
var _ herramientas.Herramienta = (*HerramientaSubproceso)(nil)

// HerramientaSubproceso es la implementación de Herramienta que envuelve un
// binario subprocess.
type HerramientaSubproceso struct {
	// rutaBinario es la ruta absoluta al binario compilado.
	rutaBinario string

	// nombre forzado (si el caller lo conoce de antemano, evita una llamada info).
	nombreForzado string

	// cache de info (se llena en la primera llamada a ensureInfo).
	infoCache *DatosInfo
	infoMu    sync.RWMutex

	// timeout por llamada (0 = sin timeout, solo respeta ctx).
	timeoutPorLlamada time.Duration

	// logFunc opcional.
	logFunc func(formato string, args ...interface{})

	// contador de ejecuciones (para metadata persistente, thread-safe).
	contadorMu     sync.Mutex
	vecesEjecutada int
	vecesExitosas  int
	ultimoError    string
	ultimoUso      time.Time
}

// NuevaHerramientaSubproceso crea un wrapper para el binario dado.
//
// Si nombreForzado != "", se usa como nombre sin llamar al binario. Esto es
// útil para registrar la herramienta inmediatamente sin esperar al primer
// info (y para que Validar() funcione).
func NuevaHerramientaSubproceso(rutaBinario string, nombreForzado string) *HerramientaSubproceso {
	return &HerramientaSubproceso{
		rutaBinario:       rutaBinario,
		nombreForzado:     nombreForzado,
		timeoutPorLlamada: 30 * time.Second, // default 30s
		logFunc:           func(string, ...interface{}) {},
	}
}

// ConTimeout setea el timeout por llamada (default 30s).
func (h *HerramientaSubproceso) ConTimeout(d time.Duration) *HerramientaSubproceso {
	h.timeoutPorLlamada = d
	return h
}

// ConLog inyecta un logger opcional.
func (h *HerramientaSubproceso) ConLog(fn func(formato string, args ...interface{})) *HerramientaSubproceso {
	if fn != nil {
		h.logFunc = fn
	}
	return h
}

// Estadisticas retorna los contadores de uso de la herramienta (thread-safe).
type Estadisticas struct {
	VecesEjecutada int       `json:"veces_ejecutada"`
	VecesExitosas  int       `json:"veces_exitosas"`
	UltimoError    string    `json:"ultimo_error,omitempty"`
	UltimoUso      time.Time `json:"ultimo_uso,omitempty"`
}

// Estadisticas retorna los contadores actuales.
func (h *HerramientaSubproceso) Estadisticas() Estadisticas {
	h.contadorMu.Lock()
	defer h.contadorMu.Unlock()
	return Estadisticas{
		VecesEjecutada: h.vecesEjecutada,
		VecesExitosas:  h.vecesExitosas,
		UltimoError:    h.ultimoError,
		UltimoUso:      h.ultimoUso,
	}
}

// ============================================================================
// Implementación de Herramienta
// ============================================================================

// Nombre retorna el nombre de la herramienta. Si se forzó en el constructor,
// lo retorna directamente; si no, llama al binario (operacion="info").
func (h *HerramientaSubproceso) Nombre() string {
	if h.nombreForzado != "" {
		return h.nombreForzado
	}
	if err := h.ensureInfo(); err != nil {
		// Fallback: usar el basename del binario
		base := filepath.Base(h.rutaBinario)
		if base == "herramienta" || base == "" {
			return "herramienta_desconocida"
		}
		return base
	}
	return h.infoCache.Nombre
}

// Descripcion retorna la descripción de la herramienta.
func (h *HerramientaSubproceso) Descripcion() string {
	if err := h.ensureInfo(); err != nil {
		return "(descripción no disponible — herramienta subprocess en " + h.rutaBinario + ")"
	}
	return h.infoCache.Descripcion
}

// Parametros retorna la lista de parámetros.
func (h *HerramientaSubproceso) Parametros() []herramientas.Parametro {
	if err := h.ensureInfo(); err != nil {
		return nil
	}
	return h.infoCache.Parametros
}

// Validar verifica que el binario existe y responde a operacion="validar".
func (h *HerramientaSubproceso) Validar() error {
	// 1. Verificar que el binario existe y es ejecutable
	info, err := os.Stat(h.rutaBinario)
	if err != nil {
		return fmt.Errorf("binario no encontrado en %s: %w", h.rutaBinario, err)
	}
	if info.IsDir() {
		return fmt.Errorf("la ruta %s es un directorio, no un binario", h.rutaBinario)
	}
	// Verificar permiso de ejecución (en Unix)
	if info.Mode()&0o111 == 0 {
		// Intentar darle permiso
		if err := os.Chmod(h.rutaBinario, 0o755); err != nil {
			return fmt.Errorf("binario no es ejecutable y no se pudo corregir: %w", err)
		}
	}

	// 2. Llamar a operacion="validar" con timeout corto
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.llamarSubprocess(ctx, SolicitudSubprocess{Operacion: "validar"})
	if err != nil {
		return fmt.Errorf("validar falló: %w", err)
	}
	if !resp.Exito {
		return fmt.Errorf("validar reportó fallo: %s", resp.Error)
	}
	return nil
}

// Ejecutar invoca la herramienta con los parámetros dados.
//
// El contexto controla el timeout. Si h.timeoutPorLlamada > 0, se aplica
// como timeout adicional (el menor de ctx.Deadline() y timeoutPorLlamada).
func (h *HerramientaSubproceso) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	h.contadorMu.Lock()
	h.vecesEjecutada++
	h.ultimoUso = time.Now()
	h.contadorMu.Unlock()

	// Aplicar timeout por llamada si está configurado y es menor que el del ctx
	callCtx := ctx
	if h.timeoutPorLlamada > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, h.timeoutPorLlamada)
		defer cancel()
	}

	resp, err := h.llamarSubprocess(callCtx, SolicitudSubprocess{
		Operacion:  "ejecutar",
		Parametros: params,
	})

	// Registrar resultado
	h.contadorMu.Lock()
	if err != nil {
		h.ultimoError = err.Error()
	} else if !resp.Exito {
		h.ultimoError = resp.Error
	} else {
		h.vecesExitosas++
		h.ultimoError = ""
	}
	h.contadorMu.Unlock()

	if err != nil {
		return herramientas.Resultado{
			Exito: false,
			Error: err.Error(),
			Metadata: map[string]interface{}{
				"subprocess": true,
				"binario":    filepath.Base(h.rutaBinario),
			},
		}, err
	}

	return herramientas.Resultado{
		Exito:    resp.Exito,
		Datos:    resp.Datos,
		Error:    resp.Error,
		Metadata: enriquecerMetadata(resp.Metadata, h.rutaBinario),
	}, nil
}

// ============================================================================
// Internos
// ============================================================================

// ensureInfo carga la info del binario si aún no se ha hecho (lazy + cached).
func (h *HerramientaSubproceso) ensureInfo() error {
	h.infoMu.RLock()
	if h.infoCache != nil {
		h.infoMu.RUnlock()
		return nil
	}
	h.infoMu.RUnlock()

	h.infoMu.Lock()
	defer h.infoMu.Unlock()

	// Double-check después de adquirir el lock
	if h.infoCache != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.llamarSubprocess(ctx, SolicitudSubprocess{Operacion: "info"})
	if err != nil {
		return err
	}
	if !resp.Exito {
		return fmt.Errorf("info falló: %s", resp.Error)
	}

	// Decode Datos a DatosInfo
	jsonBytes, err := json.Marshal(resp.Datos)
	if err != nil {
		return fmt.Errorf("marshal datos info: %w", err)
	}
	var info DatosInfo
	if err := json.Unmarshal(jsonBytes, &info); err != nil {
		return fmt.Errorf("unmarshal datos info: %w", err)
	}

	h.infoCache = &info
	return nil
}

// llamarSubprocess ejecuta el binario con la solicitud dada y retorna la respuesta.
//
// Protocolo:
//   - Escribe una línea JSON a stdin
//   - Lee una línea JSON de stdout
//   - Timeout vía ctx
//   - Si el binario sale non-zero, retorna error con stderr como mensaje
func (h *HerramientaSubproceso) llamarSubprocess(ctx context.Context, sol SolicitudSubprocess) (*RespuestaSubprocess, error) {
	// Serializar solicitud
	solBytes, err := json.Marshal(sol)
	if err != nil {
		return nil, fmt.Errorf("marshal solicitud: %w", err)
	}

	// Buffer para capturar stdout y stderr por separado
	var stdout, stderr bytes.Buffer

	// #nosec G204 — rutaBinario es controlado por el operador
	cmd := exec.CommandContext(ctx, h.rutaBinario)
	cmd.Stdin = bytes.NewReader(append(solBytes, '\n'))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Cancel propagates as SIGKILL on context expiry (Go 1.20+ uses WaitDelay automatically)
	h.logFunc("invocando subprocess %s op=%s", filepath.Base(h.rutaBinario), sol.Operacion)

	start := time.Now()
	err = cmd.Run()
	duracion := time.Since(start)

	if err != nil {
		// Si fue timeout, distinguir
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout tras %s (op=%s)", duracion, sol.Operacion)
		}
		stderrStr := stderr.String()
		if stderrStr == "" {
			stderrStr = stdout.String()
		}
		return nil, fmt.Errorf("subprocess error: %w (stderr: %s)",
			err, truncar(stderrStr, 500))
	}

	// Parsear stdout como JSON (una línea)
	stdoutStr := stdout.String()
	if stdoutStr == "" {
		return nil, fmt.Errorf("subprocess no produjo output (stderr: %s)",
			truncar(stderr.String(), 200))
	}

	// Tomar solo la última línea no vacía (algunos programas imprimen debug antes)
	lineas := splitNonEmptyLines(stdoutStr)
	if len(lineas) == 0 {
		return nil, fmt.Errorf("subprocess output sin líneas JSON (raw: %s)",
			truncar(stdoutStr, 200))
	}
	jsonLine := lineas[len(lineas)-1]

	var resp RespuestaSubprocess
	if err := json.Unmarshal([]byte(jsonLine), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal respuesta subprocess: %w (línea: %s)",
			err, truncar(jsonLine, 200))
	}

	h.logFunc("subprocess OK op=%s exito=%v (%s)", sol.Operacion, resp.Exito, duracion)
	return &resp, nil
}

// splitNonEmptyLines divide un string en líneas no vacías.
func splitNonEmptyLines(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == '\n' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else if c != '\r' {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// enriquecerMetadata agrega campos estándar a la metadata retornada por la herramienta.
func enriquecerMetadata(meta map[string]interface{}, rutaBinario string) map[string]interface{} {
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["subprocess"] = true
	meta["binario"] = filepath.Base(rutaBinario)
	return meta
}
