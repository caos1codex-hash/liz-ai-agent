package logger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// Nivel representa el nivel de severidad del log
type Nivel string

const (
	NivelDebug   Nivel = "DEBUG"
	NivelInfo    Nivel = "INFO"
	NivelWarn    Nivel = "WARN"
	NivelError   Nivel = "ERROR"
	NivelFatal   Nivel = "FATAL"
)

// EntradaLog representa una entrada de log estructurado en JSON
type EntradaLog struct {
	Timestamp string      `json:"timestamp"`
	Nivel    string      `json:"nivel"`
	Modulo   string      `json:"modulo"`
	Mensaje  string      `json:"mensaje"`
	Datos    interface{} `json:"datos,omitempty"`
}

// Logger es el logger estructurado de Liz.
// Escribe en JSON al archivo de log y texto plano a stdout.
type Logger struct {
	mu       sync.Mutex
	archivo  *os.File
	modulo   string
	nivelMin Nivel
	salida   io.Writer // stdout o testing
}

// Nueva crea una nueva instancia de Logger para un módulo dado.
// El archivo de log se crea en ~/.liz/logs/liz.log.
func Nueva(modulo string) (*Logger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo directorio home: %w", err)
	}

	logDir := home + "/.liz/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de logs %s: %w", logDir, err)
	}

	logPath := logDir + "/liz.log"
	archivo, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("error abriendo archivo de log %s: %w", logPath, err)
	}

	return &Logger{
		archivo:  archivo,
		modulo:   modulo,
		nivelMin: NivelDebug,
		salida:   os.Stdout,
	}, nil
}

// NuevaConSalida crea un Logger que escribe a un io.Writer personalizado (para testing).
func NuevaConSalida(modulo string, salida io.Writer) *Logger {
	return &Logger{
		modulo:   modulo,
		nivelMin: NivelDebug,
		salida:   salida,
	}
}

// nivelValor devuelve un valor numérico para comparar niveles.
func nivelValor(n Nivel) int {
	switch n {
	case NivelDebug:
		return 0
	case NivelInfo:
		return 1
	case NivelWarn:
		return 2
	case NivelError:
		return 3
	case NivelFatal:
		return 4
	default:
		return 0
	}
}

// registrar escribe una entrada de log tanto al archivo (JSON) como a stdout (texto).
func (l *Logger) registrar(nivel Nivel, formato string, args ...interface{}) {
	if nivelValor(nivel) < nivelValor(l.nivelMin) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Captura información del caller para contexto adicional
	_, archivo, linea, ok := runtime.Caller(2)
	ubicacion := ""
	if ok {
		ubicacion = fmt.Sprintf("%s:%d", archivo, linea)
	}

	mensaje := fmt.Sprintf(formato, args...)
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Construir entrada estructurada
	entrada := EntradaLog{
		Timestamp: timestamp,
		Nivel:    string(nivel),
		Modulo:   l.modulo,
		Mensaje:  mensaje,
	}

	// Escribir al archivo de log en formato JSON
	if l.archivo != nil {
		lineaJSON := fmt.Sprintf(`{"timestamp":"%s","nivel":"%s","modulo":"%s","mensaje":%q`,
			entrada.Timestamp, entrada.Nivel, entrada.Modulo, entrada.Mensaje)
		if ubicacion != "" {
			lineaJSON += fmt.Sprintf(`,"ubicacion":%q`, ubicacion)
		}
		lineaJSON += "}\n"
		l.archivo.WriteString(lineaJSON)
	}

	// Escribir a stdout en formato legible con colores
	colorReset := "\033[0m"
	color := "\033[37m" // blanco por defecto
	switch nivel {
	case NivelDebug:
		color = "\033[36m" // cyan
	case NivelInfo:
		color = "\033[32m" // verde
	case NivelWarn:
		color = "\033[33m" // amarillo
	case NivelError:
		color = "\033[31m" // rojo
	case NivelFatal:
		color = "\033[35m" // magenta
	}

	lineaSalida := fmt.Sprintf("%s%s [%-5s] [%-15s] %s%s\n",
		color, timestamp, nivel, l.modulo, mensaje, colorReset)
	fmt.Fprint(l.salida, lineaSalida)
}

// Debug loguea a nivel DEBUG.
func (l *Logger) Debug(formato string, args ...interface{}) {
	l.registrar(NivelDebug, formato, args...)
}

// Info loguea a nivel INFO.
func (l *Logger) Info(formato string, args ...interface{}) {
	l.registrar(NivelInfo, formato, args...)
}

// Warn loguea a nivel WARN.
func (l *Logger) Warn(formato string, args ...interface{}) {
	l.registrar(NivelWarn, formato, args...)
}

// Error loguea a nivel ERROR.
func (l *Logger) Error(formato string, args ...interface{}) {
	l.registrar(NivelError, formato, args...)
}

// Fatal loguea a nivel FATAL y termina el programa con código 1.
func (l *Logger) Fatal(formato string, args ...interface{}) {
	l.registrar(NivelFatal, formato, args...)
	os.Exit(1)
}

// Cerrar cierra el archivo de log.
func (l *Logger) Cerrar() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.archivo != nil {
		l.archivo.Close()
	}
}

// SetNivelMin cambia el nivel mínimo de log.
func (l *Logger) SetNivelMin(nivel Nivel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nivelMin = nivel
}
