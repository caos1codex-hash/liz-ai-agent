package integradas

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
)

// ============================================================================
// Instalador — Instalación de Software
// ============================================================================

// GestoresSoportados lista los gestores de paquetes que Instalador soporta.
var GestoresSoportados = []string{
	"apt",       // Debian/Ubuntu
	"apt-get",   // Debian/Ubuntu (legacy)
	"snap",      // Universal Linux
	"dnf",       // Fedora/RHEL
	"yum",       // RHEL/CentOS (legacy)
	"pacman",    // Arch Linux
	"zypper",    // openSUSE
	"apk",       // Alpine
	"brew",      // macOS/Homebrew
	"pip",       // Python
	"pip3",      // Python 3
	"npm",       // Node.js
	"yarn",      // Node.js alternativo
	"cargo",     // Rust
	"gem",       // Ruby
	"composer",  // PHP
	"go",        // Go install
}

// Instalador instala y desinstala paquetes usando gestores del sistema.
// Detecta automáticamente qué gestores están disponibles.
type Instalador struct{}

// NewInstalador crea una instancia.
func NewInstalador() *Instalador { return &Instalador{} }

var _ herramientas.Herramienta = (*Instalador)(nil)

func (i *Instalador) Nombre() string { return "instalador" }

func (i *Instalador) Descripcion() string {
	return "Instala, desinstala y actualiza paquetes usando gestores " +
		"del sistema (apt, snap, dnf, pacman, brew, pip, npm, cargo, gem, " +
		"composer, go install). Detecta automáticamente qué gestores " +
		"están disponibles. Soporta sudo opcional."
}

func (i *Instalador) Parametros() []herramientas.Parametro {
	return []herramientas.Parametro{
		{
			Nombre:      "operacion",
			Tipo:        "string",
			Requerido:   true,
			Opciones:    []string{"instalar", "desinstalar", "actualizar", "buscar", "info", "gestores", "actualizar_todo"},
			Descripcion: "Operación a realizar.",
		},
		{
			Nombre:      "paquetes",
			Tipo:        "array",
			Items:       "string",
			Descripcion: "Lista de paquetes a instalar/desinstalar. Requerido para 'instalar', 'desinstalar'.",
		},
		{
			Nombre:      "gestor",
			Tipo:        "string",
			Descripcion: "Gestor específico a usar. Si vacío, autodetecta.",
			Opciones:    GestoresSoportados,
		},
		{
			Nombre:      "buscar",
			Tipo:        "string",
			Descripcion: "Término de búsqueda para 'buscar'.",
		},
		{
			Nombre:      "sudo",
			Tipo:        "bool",
			Default:     true,
			Descripcion: "Si true, ejecuta vía sudo (necesario para apt, dnf, etc.).",
		},
		{
			Nombre:      "args_extra",
			Tipo:        "array",
			Items:       "string",
			Descripcion: "Argumentos extra para el gestor (ej: ['--no-install-recommends'] para apt).",
		},
		{
			Nombre:      "timeout_segundos",
			Tipo:        "int",
			Default:     300,
			Min:         float64Ptr(10),
			Max:         float64Ptr(3600),
			Descripcion: "Timeout en segundos. Instalaciones pueden ser lentas.",
		},
		{
			Nombre:      "solo_verificar",
			Tipo:        "bool",
			Default:     false,
			Descripcion: "Si true, solo verifica disponibilidad sin ejecutar (dry-run).",
		},
	}
}

// ResultadoInstalador es el Datos de Instalador.
type ResultadoInstalador struct {
	Operacion   string             `json:"operacion"`
	Gestor      string             `json:"gestor"`
	Gestores    []GestorDisponible `json:"gestores,omitempty"`
	Paquetes    []string           `json:"paquetes,omitempty"`
	Exitoso     bool               `json:"exitoso"`
	Salida      string             `json:"salida,omitempty"`
	Error       string             `json:"error,omitempty"`
	CodigoSalida int               `json:"codigo_salida,omitempty"`
	DryRun      bool               `json:"dry_run,omitempty"`
	ComandoEjecutado string        `json:"comando_ejecutado,omitempty"`
}

// GestorDisponible describe un gestor detectado en el sistema.
type GestorDisponible struct {
	Nombre    string `json:"nombre"`
	Ruta      string `json:"ruta"`
	Disponible bool  `json:"disponible"`
	Version   string `json:"version,omitempty"`
}

func (i *Instalador) Validar() error { return nil }

func (i *Instalador) Ejecutar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	operacion, err := herramientas.ObtenerString(params, i.paramByName("operacion"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}

	switch operacion {
	case "gestores":
		return i.opGestores()
	case "instalar":
		return i.opInstalar(ctx, params, false)
	case "desinstalar":
		return i.opDesinstalar(ctx, params)
	case "actualizar":
		return i.opInstalar(ctx, params, true)
	case "actualizar_todo":
		return i.opActualizarTodo(ctx, params)
	case "buscar":
		return i.opBuscar(ctx, params)
	case "info":
		return i.opInfo(ctx, params)
	default:
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("operación '%s' no soportada", operacion)}, nil
	}
}

// opGestores lista todos los gestores soportados y su disponibilidad.
func (i *Instalador) opGestores() (herramientas.Resultado, error) {
	gestores := make([]GestorDisponible, 0, len(GestoresSoportados))
	for _, g := range GestoresSoportados {
		ruta, err := exec.LookPath(g)
		disponible := err == nil
		gd := GestorDisponible{
			Nombre:     g,
			Disponible: disponible,
		}
		if disponible {
			gd.Ruta = ruta
			gd.Version = obtenerVersionGestor(g)
		}
		gestores = append(gestores, gd)
	}

	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoInstalador{
			Operacion: "gestores",
			Gestores:  gestores,
		},
		Metadata: herramientas.NuevaMetadata(0),
	}, nil
}

// obtenerVersionGestor intenta obtener la versión de un gestor.
func obtenerVersionGestor(gestor string) string {
	var cmd *exec.Cmd
	switch gestor {
	case "apt", "apt-get":
		cmd = exec.Command(gestor, "--version")
	case "pip", "pip3":
		cmd = exec.Command(gestor, "--version")
	case "npm":
		cmd = exec.Command(gestor, "--version")
	case "cargo":
		cmd = exec.Command(gestor, "--version")
	case "go":
		cmd = exec.Command(gestor, "version")
	default:
		cmd = exec.Command(gestor, "--version")
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Solo primera línea
	salida := strings.TrimSpace(string(out))
	if idx := strings.Index(salida, "\n"); idx > 0 {
		salida = salida[:idx]
	}
	// Limitar a 100 chars
	if len(salida) > 100 {
		salida = salida[:100] + "..."
	}
	return salida
}

// opInstalar instala paquetes usando el gestor especificado o autodetecta.
func (i *Instalador) opInstalar(ctx context.Context, params map[string]interface{},
	actualizar bool) (herramientas.Resultado, error) {

	paquetes, err := herramientas.ObtenerArrayString(params, i.paramByName("paquetes"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}
	if len(paquetes) == 0 {
		return herramientas.Resultado{Exito: false,
			Error: "parámetro 'paquetes' requerido (no vacío)"}, nil
	}

	gestor, _ := herramientas.ObtenerString(params, i.paramByName("gestor"))
	if gestor == "" {
		gestor = autodetectarGestor(paquetes)
		if gestor == "" {
			return herramientas.Resultado{Exito: false,
				Error: "no se pudo autodetectar gestor. Especifique 'gestor'."}, nil
		}
	}

	// Verificar que el gestor esté disponible
	if _, err := exec.LookPath(gestor); err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("gestor '%s' no disponible en el sistema", gestor)}, nil
	}

	sudo, _ := herramientas.ObtenerBool(params, i.paramByName("sudo"))
	dryRun, _ := herramientas.ObtenerBool(params, i.paramByName("solo_verificar"))
	argsExtra, _ := herramientas.ObtenerArrayString(params, i.paramByName("args_extra"))

	// Construir comando
	args := construirArgsInstalacion(gestor, paquetes, actualizar, argsExtra)
	if args == nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("gestor '%s' no soportado para instalar", gestor)}, nil
	}

	comandoFinal := gestor
	comandoArgs := args
	if precisaSudo(gestor) && sudo {
		comandoFinal = "sudo"
		comandoArgs = append([]string{"-n", gestor}, args...)
	}

	comandoStr := comandoFinal + " " + strings.Join(comandoArgs, " ")

	if dryRun {
		return herramientas.Resultado{
			Exito: true,
			Datos: ResultadoInstalador{
				Operacion:        "instalar",
				Gestor:           gestor,
				Paquetes:         paquetes,
				Exitoso:          true,
				DryRun:           true,
				ComandoEjecutado: comandoStr,
			},
			Metadata: herramientas.NuevaMetadata(0),
		}, nil
	}

	return ejecutarComandoInstalador(ctx, comandoFinal, comandoArgs, gestor, paquetes, comandoStr, params, i)
}

// opDesinstalar desinstala paquetes.
func (i *Instalador) opDesinstalar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	paquetes, err := herramientas.ObtenerArrayString(params, i.paramByName("paquetes"))
	if err != nil {
		return herramientas.Resultado{Exito: false, Error: err.Error()}, nil
	}
	if len(paquetes) == 0 {
		return herramientas.Resultado{Exito: false,
			Error: "parámetro 'paquetes' requerido"}, nil
	}

	gestor, _ := herramientas.ObtenerString(params, i.paramByName("gestor"))
	if gestor == "" {
		gestor = autodetectarGestor(paquetes)
	}
	if gestor == "" {
		return herramientas.Resultado{Exito: false,
			Error: "no se pudo autodetectar gestor"}, nil
	}

	if _, err := exec.LookPath(gestor); err != nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("gestor '%s' no disponible", gestor)}, nil
	}

	sudo, _ := herramientas.ObtenerBool(params, i.paramByName("solo_verificar"))
	dryRun, _ := herramientas.ObtenerBool(params, i.paramByName("solo_verificar"))
	argsExtra, _ := herramientas.ObtenerArrayString(params, i.paramByName("args_extra"))

	args := construirArgsDesinstalacion(gestor, paquetes, argsExtra)
	if args == nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("gestor '%s' no soportado para desinstalar", gestor)}, nil
	}

	comandoFinal := gestor
	comandoArgs := args
	if precisaSudo(gestor) && sudo {
		comandoFinal = "sudo"
		comandoArgs = append([]string{"-n", gestor}, args...)
	}
	comandoStr := comandoFinal + " " + strings.Join(comandoArgs, " ")

	if dryRun {
		return herramientas.Resultado{
			Exito: true,
			Datos: ResultadoInstalador{
				Operacion:        "desinstalar",
				Gestor:           gestor,
				Paquetes:         paquetes,
				Exitoso:          true,
				DryRun:           true,
				ComandoEjecutado: comandoStr,
			},
			Metadata: herramientas.NuevaMetadata(0),
		}, nil
	}

	return ejecutarComandoInstalador(ctx, comandoFinal, comandoArgs, gestor, paquetes, comandoStr, params, i)
}

// opActualizarTodo actualiza todos los paquetes del sistema.
func (i *Instalador) opActualizarTodo(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	gestor, _ := herramientas.ObtenerString(params, i.paramByName("gestor"))
	if gestor == "" {
		// Autodetectar el gestor principal del sistema
		for _, g := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk", "brew"} {
			if _, err := exec.LookPath(g); err == nil {
				gestor = g
				break
			}
		}
	}
	if gestor == "" {
		return herramientas.Resultado{Exito: false,
			Error: "no se pudo autodetectar gestor de paquetes del sistema"}, nil
	}

	args := construirArgsActualizarTodo(gestor)
	if args == nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("gestor '%s' no soporta actualización masiva", gestor)}, nil
	}

	sudo, _ := herramientas.ObtenerBool(params, i.paramByName("sudo"))
	comandoFinal := gestor
	comandoArgs := args
	if precisaSudo(gestor) && sudo {
		comandoFinal = "sudo"
		comandoArgs = append([]string{"-n", gestor}, args...)
	}
	comandoStr := comandoFinal + " " + strings.Join(comandoArgs, " ")

	return ejecutarComandoInstalador(ctx, comandoFinal, comandoArgs, gestor, nil, comandoStr, params, i)
}

// opBuscar busca paquetes disponibles.
func (i *Instalador) opBuscar(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	buscar, err := herramientas.ObtenerString(params, i.paramByName("buscar"))
	if err != nil || buscar == "" {
		return herramientas.Resultado{Exito: false,
			Error: "parámetro 'buscar' requerido"}, nil
	}

	gestor, _ := herramientas.ObtenerString(params, i.paramByName("gestor"))
	if gestor == "" {
		// Para buscar, usar apt-cache si está disponible
		for _, g := range []string{"apt-cache", "apt", "dnf", "yum", "pacman", "brew"} {
			if _, err := exec.LookPath(g); err == nil {
				gestor = g
				break
			}
		}
	}
	if gestor == "" {
		return herramientas.Resultado{Exito: false,
			Error: "no se pudo autodetectar gestor para buscar"}, nil
	}

	args := construirArgsBuscar(gestor, buscar)
	if args == nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("gestor '%s' no soporta búsqueda", gestor)}, nil
	}

	comandoFinal := gestor
	comandoArgs := args
	sudo, _ := herramientas.ObtenerBool(params, i.paramByName("sudo"))
	if precisaSudo(gestor) && sudo {
		comandoFinal = "sudo"
		comandoArgs = append([]string{"-n", gestor}, args...)
	}
	comandoStr := comandoFinal + " " + strings.Join(comandoArgs, " ")

	return ejecutarComandoInstalador(ctx, comandoFinal, comandoArgs, gestor,
		[]string{buscar}, comandoStr, params, i)
}

// opInfo muestra info de un paquete específico.
func (i *Instalador) opInfo(ctx context.Context, params map[string]interface{}) (herramientas.Resultado, error) {
	paquetes, err := herramientas.ObtenerArrayString(params, i.paramByName("paquetes"))
	if err != nil || len(paquetes) == 0 {
		return herramientas.Resultado{Exito: false,
			Error: "parámetro 'paquetes' requerido (1 paquete para info)"}, nil
	}
	paquete := paquetes[0]

	gestor, _ := herramientas.ObtenerString(params, i.paramByName("gestor"))
	if gestor == "" {
		for _, g := range []string{"apt-cache", "apt", "dnf", "yum", "pacman", "brew"} {
			if _, err := exec.LookPath(g); err == nil {
				gestor = g
				break
			}
		}
	}
	if gestor == "" {
		return herramientas.Resultado{Exito: false,
			Error: "no se pudo autodetectar gestor"}, nil
	}

	args := construirArgsInfo(gestor, paquete)
	if args == nil {
		return herramientas.Resultado{Exito: false,
			Error: fmt.Sprintf("gestor '%s' no soporta info", gestor)}, nil
	}

	comandoFinal := gestor
	comandoArgs := args
	sudo, _ := herramientas.ObtenerBool(params, i.paramByName("sudo"))
	if precisaSudo(gestor) && sudo {
		comandoFinal = "sudo"
		comandoArgs = append([]string{"-n", gestor}, args...)
	}
	comandoStr := comandoFinal + " " + strings.Join(comandoArgs, " ")

	return ejecutarComandoInstalador(ctx, comandoFinal, comandoArgs, gestor,
		[]string{paquete}, comandoStr, params, i)
}

// ejecutarComandoInstalador ejecuta el comando y formatea el resultado.
func ejecutarComandoInstalador(ctx context.Context, comando string, args []string,
	gestor string, paquetes []string, comandoStr string,
	params map[string]interface{}, i *Instalador) (herramientas.Resultado, error) {

	// Usar Terminal internamente para respetar permisos peligrosos
	tl := NewTerminal()
	tlParams := map[string]interface{}{
		"comando":            comando,
		"args":               args,
		"timeout_segundos":   300,
	}
	res, _ := tl.Ejecutar(ctx, tlParams)
	if !res.Exito {
		datos := res.Datos.(ResultadoTerminal)
		return herramientas.Resultado{
			Exito: false,
			Datos: ResultadoInstalador{
				Operacion:        "",
				Gestor:           gestor,
				Paquetes:         paquetes,
				Exitoso:          false,
				Salida:           datos.Stdout,
				Error:            datos.Stderr,
				CodigoSalida:     datos.CodigoSalida,
				ComandoEjecutado: comandoStr,
			},
			Error:    res.Error,
			Metadata: herramientas.NuevaMetadata(datos.DuracionMs),
		}, nil
	}

	datos := res.Datos.(ResultadoTerminal)
	return herramientas.Resultado{
		Exito: true,
		Datos: ResultadoInstalador{
			Operacion:        "",
			Gestor:           gestor,
			Paquetes:         paquetes,
			Exitoso:          true,
			Salida:           datos.Stdout,
			CodigoSalida:     datos.CodigoSalida,
			ComandoEjecutado: comandoStr,
		},
		Metadata: herramientas.NuevaMetadata(datos.DuracionMs),
	}, nil
}

// construirArgsInstalacion construye los args para instalar según el gestor.
func construirArgsInstalacion(gestor string, paquetes []string, actualizar bool, extra []string) []string {
	var args []string
	switch gestor {
	case "apt", "apt-get":
		if actualizar {
			args = []string{"install", "--only-upgrade", "-y"}
		} else {
			args = []string{"install", "-y"}
		}
	case "snap":
		args = []string{"install"}
	case "dnf", "yum":
		args = []string{"install", "-y"}
	case "pacman":
		if actualizar {
			args = []string{"-Syu", "--noconfirm"}
		} else {
			args = []string{"-S", "--noconfirm"}
		}
	case "zypper":
		args = []string{"install", "-y"}
	case "apk":
		args = []string{"add"}
	case "brew":
		if actualizar {
			args = []string{"upgrade"}
		} else {
			args = []string{"install"}
		}
	case "pip", "pip3":
		if actualizar {
			args = []string{"install", "--upgrade"}
		} else {
			args = []string{"install"}
		}
	case "npm":
		if actualizar {
			args = []string{"update"}
		} else {
			args = []string{"install"}
		}
	case "yarn":
		if actualizar {
			args = []string{"upgrade"}
		} else {
			args = []string{"add"}
		}
	case "cargo":
		if actualizar {
			args = []string{"update"}
		} else {
			args = []string{"install"}
		}
	case "gem":
		if actualizar {
			args = []string{"update"}
		} else {
			args = []string{"install"}
		}
	case "composer":
		args = []string{"require"}
	case "go":
		// go install pkg@latest o go get pkg
		if actualizar {
			args = []string{"get", "-u"}
		} else {
			args = []string{"install"}
		}
	default:
		return nil
	}
	args = append(args, extra...)
	args = append(args, paquetes...)
	return args
}

// construirArgsDesinstalacion construye args para desinstalar.
func construirArgsDesinstalacion(gestor string, paquetes []string, extra []string) []string {
	var args []string
	switch gestor {
	case "apt", "apt-get":
		args = []string{"remove", "-y", "--purge"}
	case "snap":
		args = []string{"remove"}
	case "dnf", "yum":
		args = []string{"remove", "-y"}
	case "pacman":
		args = []string{"-R", "--noconfirm"}
	case "zypper":
		args = []string{"remove", "-y"}
	case "apk":
		args = []string{"del"}
	case "brew":
		args = []string{"uninstall"}
	case "pip", "pip3":
		args = []string{"uninstall", "-y"}
	case "npm":
		args = []string{"uninstall"}
	case "yarn":
		args = []string{"remove"}
	case "cargo":
		args = []string{"remove"}
	case "gem":
		args = []string{"uninstall"}
	case "composer":
		args = []string{"remove"}
	case "go":
		return nil // go no tiene desinstalar estándar
	default:
		return nil
	}
	args = append(args, extra...)
	args = append(args, paquetes...)
	return args
}

// construirArgsActualizarTodo args para actualizar todos los paquetes.
func construirArgsActualizarTodo(gestor string) []string {
	switch gestor {
	case "apt", "apt-get":
		return []string{"update", "&&", gestor, "upgrade", "-y"}
	case "dnf", "yum":
		return []string{"upgrade", "-y"}
	case "pacman":
		return []string{"-Syu", "--noconfirm"}
	case "zypper":
		return []string{"update", "-y"}
	case "apk":
		return []string{"upgrade"}
	case "brew":
		return []string{"upgrade"}
	default:
		return nil
	}
}

// construirArgsBuscar args para buscar paquetes.
func construirArgsBuscar(gestor string, termino string) []string {
	switch gestor {
	case "apt-cache", "apt":
		return []string{"search", termino}
	case "dnf", "yum":
		return []string{"search", termino}
	case "pacman":
		return []string{"-Ss", termino}
	case "brew":
		return []string{"search", termino}
	case "pip", "pip3":
		return []string{"search", termino}
	case "npm":
		return []string{"search", termino}
	default:
		return nil
	}
}

// construirArgsInfo args para info de paquete.
func construirArgsInfo(gestor string, paquete string) []string {
	switch gestor {
	case "apt-cache", "apt":
		return []string{"show", paquete}
	case "dnf", "yum":
		return []string{"info", paquete}
	case "pacman":
		return []string{"-Si", paquete}
	case "brew":
		return []string{"info", paquete}
	case "npm":
		return []string{"info", paquete}
	default:
		return nil
	}
}

// autodetectarGestor intenta adivinar el gestor apropiado.
// Heurística: si el paquete contiene '/' o '@' es npm/go;
// si empieza con 'python-' o 'py' es pip; si no, gestor del sistema.
func autodetectarGestor(paquetes []string) string {
	// Verificar gestores del sistema primero
	sistemaDisponible := ""
	for _, g := range []string{"apt", "dnf", "pacman", "zypper", "apk", "brew"} {
		if _, err := exec.LookPath(g); err == nil {
			sistemaDisponible = g
			break
		}
	}

	for _, p := range paquetes {
		// Paquetes con @version → npm o pip
		if strings.Contains(p, "@") {
			if _, err := exec.LookPath("npm"); err == nil {
				return "npm"
			}
		}
		// Paquetes Python (python-XXX, py-XXX)
		if strings.HasPrefix(p, "python-") || strings.HasPrefix(p, "py") {
			if _, err := exec.LookPath("pip3"); err == nil {
				return "pip3"
			}
			if _, err := exec.LookPath("pip"); err == nil {
				return "pip"
			}
		}
		// Paquetes Go (github.com/...)
		if strings.Contains(p, "github.com/") || strings.Contains(p, "golang.org/") {
			if _, err := exec.LookPath("go"); err == nil {
				return "go"
			}
		}
	}

	return sistemaDisponible
}

// precisaSudo determina si un gestor requiere sudo para instalar.
func precisaSudo(gestor string) bool {
	switch gestor {
	case "apt", "apt-get", "apt-cache", "dnf", "yum", "pacman", "zypper", "apk", "snap":
		return true
	case "brew", "pip", "pip3", "npm", "yarn", "cargo", "gem", "composer", "go":
		return false
	default:
		return false
	}
}

// paramByName busca un parámetro por nombre.
func (i *Instalador) paramByName(nombre string) herramientas.Parametro {
	for _, p := range i.Parametros() {
		if p.Nombre == nombre {
			return p
		}
	}
	return herramientas.Parametro{Nombre: nombre}
}
