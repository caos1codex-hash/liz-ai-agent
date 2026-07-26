package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// API key NVIDIA por defecto (bundled en el código fuente)
// ============================================================================
//
// Esta key permite que Liz funcione out-of-the-box sin que el usuario tenga
// que configurar su propia API key. Es una key de demo/evaluación.
//
// ⚠️ ADVERTENCIA DE SEGURIDAD:
//   - Cualquiera con acceso al repo puede usar esta key.
//   - GitHub Secret Scanning puede detectarla y revocarla.
//   - Si la key tiene quotas, un abuso puede quemarla.
//   - En PRODUCCIÓN, el usuario DEBE reemplazarla por su propia key
//     vía env var NVIDIA_API_KEY o vía liz.modelos[].api_key en config.yaml.
//
// Ver issue #29.

const APIKeyNvidiaPorDefecto = "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"


// ============================================================================
// Tipos de Configuración
// ============================================================================

// ConfiguracionModelo define la configuración de un modelo de IA individual.
// Cada modelo puede tener sus propios parámetros de temperatura, top-p,
// límite de tokens y reglas de uso específicas.
type ConfiguracionModelo struct {
	Nombre      string  `yaml:"nombre" json:"nombre"`
	Proveedor   string  `yaml:"proveedor" json:"proveedor"`
	APIKey      string  `yaml:"api_key" json:"api_key,omitempty"`
	URL         string  `yaml:"url" json:"url"`
	Temperatura float64 `yaml:"temperatura" json:"temperatura"`
	TopP        float64 `yaml:"top_p" json:"top_p"`
	MaxTokens   int     `yaml:"max_tokens" json:"max_tokens"`
	Rol         string  `yaml:"rol" json:"rol"` // "principal", "reserva", "especializado"
	Habilitado  bool    `yaml:"habilitado" json:"habilitado"`
}

// ConfiguracionHerramienta define los parámetros de una herramienta externa.
// Incluye el tipo de herramienta, su ruta/comando, timeout y si está habilitada.
type ConfiguracionHerramienta struct {
	Nombre     string `yaml:"nombre" json:"nombre"`
	Tipo       string `yaml:"tipo" json:"tipo"` // "ejecutable", "api", "script"
	Ruta       string `yaml:"ruta" json:"ruta"`
	Timeout    int    `yaml:"timeout" json:"timeout"` // en segundos
	Habilitado bool   `yaml:"habilitado" json:"habilitado"`
}

// ConfiguracionSeguridad define las políticas de seguridad del sistema.
// Incluye límites de frecuencia, sandboxing y restricciones de red.
type ConfiguracionSeguridad struct {
	SandboxHabilitado bool `yaml:"sandbox_habilitado" json:"sandbox_habilitado"`
	MaxPeticionesMin  int  `yaml:"max_peticiones_min" json:"max_peticiones_min"`
	MaxTokensSesion   int  `yaml:"max_tokens_sesion" json:"max_tokens_sesion"`
	PermitirRed       bool `yaml:"permitir_red" json:"permitir_red"`
	PermitirSistema   bool `yaml:"permitir_sistema" json:"permitir_sistema"`
}

// ConfiguracionLogging define los parámetros de registro del sistema.
// Permite configurar el nivel, formato y si se muestra en stdout.
type ConfiguracionLogging struct {
	Nivel       string `yaml:"nivel" json:"nivel"` // "debug", "info", "advertencia", "error", "silencio"
	Archivo     string `yaml:"archivo" json:"archivo"`
	Stdout      bool   `yaml:"stdout" json:"stdout"`
	Rotacion    bool   `yaml:"rotacion" json:"rotacion"`
	MaxMB       int    `yaml:"max_mb" json:"max_mb"`
	MaxArchivos int    `yaml:"max_archivos" json:"max_archivos"`
}

// ConfiguracionContexto define los parámetros del sistema de contexto.
// Controla la estrategia de carga, límites y el catálogo de contexto.
type ConfiguracionContexto struct {
	Estrategia         string `yaml:"estrategia" json:"estrategia"` // "bajo_demanda", "eager", "hibrido"
	MaxArchivos        int    `yaml:"max_archivos" json:"max_archivos"`
	MaxLineas          int    `yaml:"max_lineas" json:"max_lineas"`
	TamanoContexto     int    `yaml:"tamano_contexto" json:"tamano_contexto"` // tokens máximos
	ResumenAuto        bool   `yaml:"resumen_auto" json:"resumen_auto"`
	CatalogoHabilitado bool   `yaml:"catalogo_habilitado" json:"catalogo_habilitado"`
}

// Configuracion contiene toda la configuración del agente Liz.
// Es el tipo raíz que se serializa/deserializa desde YAML.
type Configuracion struct {
	Puerto         int                        `yaml:"puerto" json:"puerto"`
	Host           string                     `yaml:"host" json:"host"`
	Nombre         string                     `yaml:"nombre" json:"nombre"`
	Version        string                     `yaml:"version" json:"version"`
	Modelos        []ConfiguracionModelo      `yaml:"modelos" json:"modelos"`
	Herramientas   []ConfiguracionHerramienta `yaml:"herramientas" json:"herramientas"`
	Seguridad      ConfiguracionSeguridad     `yaml:"seguridad" json:"seguridad"`
	Logging        ConfiguracionLogging       `yaml:"logging" json:"logging"`
	Contexto       ConfiguracionContexto      `yaml:"contexto" json:"contexto"`
	DirectorioBase string                     `yaml:"directorio_base" json:"directorio_base"`
}

// archivoConfig es un wrapper para soportar el formato YAML
// donde toda la configuración está bajo la clave raíz "liz:".
type archivoConfig struct {
	Liz *Configuracion `yaml:"liz"`
}

// CambioConfiguracion representa un cambio individual en la configuración.
// Se usa para tracking de auditoría y para la funcionalidad de hot-reload.
type CambioConfiguracion struct {
	Ruta          string      `json:"ruta"`
	ValorAnterior interface{} `json:"valor_anterior"`
	ValorNuevo    interface{} `json:"valor_nuevo"`
	Timestamp     string      `json:"timestamp"`
}

// ============================================================================
// Gestor de Configuración (Thread-Safe Singleton)
// ============================================================================

// Gestor es el gestor central de configuración. Es thread-safe mediante
// sync.RWMutex y proporciona acceso concurrente a la configuración.
type Gestor struct {
	mu          sync.RWMutex
	config      *Configuracion
	rutaArchivo string
	cambios     []CambioConfiguracion
	validador   *ValidadorConfig
}

// Variable global del gestor de configuración.
var gestorGlobal *Gestor

// ============================================================================
// Inicialización
// ============================================================================

// Inicializar crea y configura el gestor global de configuración.
// Lee el archivo YAML, aplica overrides de variables de entorno,
// valida la configuración y asegura que los directorios existan.
// Retorna error si el archivo no existe o es inválido.
func Inicializar(rutaArchivo string) (*Gestor, error) {
	cfg, err := Cargar(rutaArchivo)
	if err != nil {
		return nil, fmt.Errorf("error al inicializar configuración: %w", err)
	}

	validador := NuevoValidadorConfig()

	g := &Gestor{
		config:      cfg,
		rutaArchivo: rutaArchivo,
		cambios:     make([]CambioConfiguracion, 0),
		validador:   validador,
	}

	// Validar la configuración cargada
	if err := validador.Validar(cfg); err != nil {
		return nil, fmt.Errorf("configuración inválida: %w", err)
	}

	// Asegurar directorios de runtime
	if err := cfg.AsegurarDirectorios(); err != nil {
		return nil, fmt.Errorf("error al crear directorios: %w", err)
	}

	gestorGlobal = g
	return g, nil
}

// NuevoGestorConConfig crea un gestor con una configuración ya construida,
// sin leer desde disco ni crear directorios. Útil para tests e inyección
// de dependencias en producción.
//
// NO establece el gestor global (usar Inicializar para eso).
func NuevoGestorConConfig(cfg *Configuracion) *Gestor {
	if cfg == nil {
		cfgDef := ConfiguracionPorDefecto()
		cfg = &cfgDef
	}
	return &Gestor{
		config:    cfg,
		cambios:   make([]CambioConfiguracion, 0),
		validador: NuevoValidadorConfig(),
	}
}

// ObtenerGestor retorna el gestor global de configuración.
// Retorna nil si no ha sido inicializado.
func ObtenerGestor() *Gestor {
	return gestorGlobal
}

// ============================================================================
// Carga y Persistencia
// ============================================================================

// Cargar lee y parsea un archivo YAML de configuración.
// Soporta tanto el formato con wrapper "liz:" como el formato directo.
// Aplica overrides de variables de entorno después de la carga.
func Cargar(rutaArchivo string) (*Configuracion, error) {
	rutaExpandida := expandirHome(rutaArchivo)

	// Si no existe el archivo, usar configuración por defecto.
	// IMPORTANTE: aplicar defaults y env vars también aquí, para que la
	// API key NVIDIA bundled (issue #29) y los overrides de env vars
	// funcionen out-of-the-box sin archivo de config.
	if _, err := os.Stat(rutaExpandida); os.IsNotExist(err) {
		cfg := ConfiguracionPorDefecto()
		aplicarDefaults(&cfg)
		aplicarOverridesEnv(&cfg)
		return &cfg, nil
	}

	// Leer el archivo YAML
	datos, err := os.ReadFile(rutaExpandida)
	if err != nil {
		return nil, fmt.Errorf("error al leer archivo de configuración %s: %w", rutaExpandida, err)
	}

	// Intentar primero con wrapper "liz:" (formato recomendado)
	var archivo archivoConfig
	if err := yaml.Unmarshal(datos, &archivo); err != nil {
		return nil, fmt.Errorf("error al parsear YAML con wrapper: %w", err)
	}

	var cfg *Configuracion
	if archivo.Liz != nil {
		// Formato con wrapper "liz:"
		cfg = archivo.Liz
	} else {
		// Fallback: formato directo sin wrapper
		cfg = &Configuracion{}
		if err := yaml.Unmarshal(datos, cfg); err != nil {
			return nil, fmt.Errorf("error al parsear YAML directo: %w", err)
		}
	}

	// Aplicar valores por defecto para campos vacíos
	aplicarDefaults(cfg)

	// Aplicar overrides de variables de entorno
	aplicarOverridesEnv(cfg)

	return cfg, nil
}

// Guardar persiste la configuración actual al archivo YAML.
// Escribe en formato con wrapper "liz:" y aplica la sangría correcta.
func (g *Gestor) Guardar() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	return guardarConfiguracion(g.config, g.rutaArchivo)
}

// guardarConfiguracion es la función interna que escribe la configuración al archivo.
func guardarConfiguracion(cfg *Configuracion, rutaArchivo string) error {
	rutaExpandida := expandirHome(rutaArchivo)

	// Asegurar que el directorio existe
	dir := filepath.Dir(rutaExpandida)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error al crear directorio de configuración: %w", err)
	}

	// Crear estructura con wrapper "liz:"
	archivo := archivoConfig{Liz: cfg}

	datos, err := yaml.Marshal(&archivo)
	if err != nil {
		return fmt.Errorf("error al serializar configuración: %w", err)
	}

	if err := os.WriteFile(rutaExpandida, datos, 0644); err != nil {
		return fmt.Errorf("error al escribir archivo de configuración: %w", err)
	}

	return nil
}

// ============================================================================
// Acceso a Configuración (Thread-Safe)
// ============================================================================

// Obtener retorna una copia de la configuración actual.
// Es thread-safe y no expone el puntero interno.
func (g *Gestor) Obtener() Configuracion {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return *g.config
}

// ObtenerPuerto retorna el puerto configurado.
func (g *Gestor) ObtenerPuerto() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.Puerto
}

// ObtenerHost retorna el host configurado.
func (g *Gestor) ObtenerHost() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.Host
}

// ObtenerNombre retorna el nombre del agente.
func (g *Gestor) ObtenerNombre() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.Nombre
}

// ObtenerVersion retorna la versión del agente.
func (g *Gestor) ObtenerVersion() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.Version
}

// ObtenerDirectorioBase retorna el directorio base de runtime.
func (g *Gestor) ObtenerDirectorioBase() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.DirectorioBase
}

// ObtenerModelos retorna la lista de modelos configurados.
func (g *Gestor) ObtenerModelos() []ConfiguracionModelo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	copia := make([]ConfiguracionModelo, len(g.config.Modelos))
	copy(copia, g.config.Modelos)
	return copia
}

// ObtenerModeloHabilitado retorna el primer modelo habilitado de la lista.
// Si no hay ninguno habilitado, retorna nil.
func (g *Gestor) ObtenerModeloHabilitado() *ConfiguracionModelo {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for i := range g.config.Modelos {
		if g.config.Modelos[i].Habilitado {
			copia := g.config.Modelos[i]
			return &copia
		}
	}
	return nil
}

// ObtenerHerramientas retorna la lista de herramientas configuradas.
func (g *Gestor) ObtenerHerramientas() []ConfiguracionHerramienta {
	g.mu.RLock()
	defer g.mu.RUnlock()
	copia := make([]ConfiguracionHerramienta, len(g.config.Herramientas))
	copy(copia, g.config.Herramientas)
	return copia
}

// ObtenerSeguridad retorna la configuración de seguridad.
func (g *Gestor) ObtenerSeguridad() ConfiguracionSeguridad {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.Seguridad
}

// ObtenerLogging retorna la configuración de logging.
func (g *Gestor) ObtenerLogging() ConfiguracionLogging {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.Logging
}

// ObtenerContexto retorna la configuración del sistema de contexto.
func (g *Gestor) ObtenerContexto() ConfiguracionContexto {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.config.Contexto
}

// ============================================================================
// Modificación de Configuración (Thread-Safe con Auditoría)
// ============================================================================

// Establecer permite modificar un campo de configuración por su ruta dot-notation.
// Registra el cambio en el historial de auditoría. Valida después de cada cambio.
// Ejemplo: Establecer("puerto", "8080") o Establecer("seguridad.max_peticiones_min", "100")
func (g *Gestor) Establecer(ruta, valor string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Obtener el valor anterior antes de modificar
	valorAnterior := g.obtenerCampo(ruta)

	// Modificar el campo
	if err := g.establecerCampo(ruta, valor); err != nil {
		return fmt.Errorf("error al establecer '%s': %w", ruta, err)
	}

	// Validar la configuración después del cambio
	if err := g.validador.Validar(g.config); err != nil {
		// Revertir el cambio
		_ = g.establecerCampoInterface(ruta, valorAnterior)
		return fmt.Errorf("cambio rechazado por validación: %w", err)
	}

	// Registrar el cambio
	g.cambios = append(g.cambios, CambioConfiguracion{
		Ruta:          ruta,
		ValorAnterior: valorAnterior,
		ValorNuevo:    valor,
		Timestamp:     timestampActual(),
	})

	return nil
}

// EstablecerMultiple permite modificar varios campos de configuración atómicamente.
// Si algún campo falla la validación, ninguno se aplica.
func (g *Gestor) EstablecerMultiple(cambios map[string]string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Guardar estado actual para rollback
	configBackup := *g.config

	// Aplicar todos los cambios
	for ruta, valor := range cambios {
		if err := g.establecerCampo(ruta, valor); err != nil {
			*g.config = configBackup
			return fmt.Errorf("error al establecer '%s': %w", ruta, err)
		}
	}

	// Validar toda la configuración resultante
	if err := g.validador.Validar(g.config); err != nil {
		*g.config = configBackup
		return fmt.Errorf("cambios rechazados por validación: %w", err)
	}

	// Registrar todos los cambios
	for ruta, valor := range cambios {
		g.cambios = append(g.cambios, CambioConfiguracion{
			Ruta:       ruta,
			ValorNuevo: valor,
			Timestamp:  timestampActual(),
		})
	}

	return nil
}

// ObtenerCambios retorna el historial de cambios de configuración.
func (g *Gestor) ObtenerCambios() []CambioConfiguracion {
	g.mu.RLock()
	defer g.mu.RUnlock()
	copia := make([]CambioConfiguracion, len(g.cambios))
	copy(copia, g.cambios)
	return copia
}

// ============================================================================
// Validación
// ============================================================================

// Validar ejecuta la validación completa de la configuración actual.
// Retorna un error si alguna regla es violada.
func (g *Gestor) Validar() error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.validador.Validar(g.config)
}

// ValidarCampo valida un campo individual de configuración.
func (g *Gestor) ValidarCampo(ruta, valor string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.validador.ValidarCampo(ruta, valor)
}

// Esquema retorna el esquema de validación para documentación.
func (g *Gestor) Esquema() EsquemaConfig {
	return g.validador.ObtenerEsquema()
}

// ============================================================================
// Recarga
// ============================================================================

// Recargar vuelve a leer el archivo de configuración desde disco.
// Retorna los campos que cambiaron comparando con la configuración actual.
// Es la base para la funcionalidad de hot-reload (señal SIGHUP).
func (g *Gestor) Recargar() ([]CambioConfiguracion, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	nuevaCfg, err := Cargar(g.rutaArchivo)
	if err != nil {
		return nil, fmt.Errorf("error al recargar configuración: %w", err)
	}

	// Validar la nueva configuración
	if err := g.validador.Validar(nuevaCfg); err != nil {
		return nil, fmt.Errorf("configuración recargada es inválida: %w", err)
	}

	// Detectar diferencias
	cambios := compararConfiguraciones(g.config, nuevaCfg)

	// Asegurar directorios con la nueva configuración
	if err := nuevaCfg.AsegurarDirectorios(); err != nil {
		return nil, fmt.Errorf("error al asegurar directorios tras recarga: %w", err)
	}

	g.config = nuevaCfg
	return cambios, nil
}

// ============================================================================
// Métodos Auxiliares de Configuración
// ============================================================================

// AsegurarDirectorios crea la estructura de directorios necesaria para el runtime.
// Crea ~/.liz/ y sus subdirectorios si no existen.
func (c *Configuracion) AsegurarDirectorios() error {
	dirBase := expandirHome(c.DirectorioBase)
	directorios := []string{
		dirBase,
		filepath.Join(dirBase, "contexto"),
		filepath.Join(dirBase, "herramientas"),
		filepath.Join(dirBase, "logs"),
		filepath.Join(dirBase, "conversaciones"),
		filepath.Join(dirBase, "permisos"),
	}

	for _, dir := range directorios {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error al crear directorio %s: %w", dir, err)
		}
	}

	return nil
}

// RutaArchivoConfig retorna la ruta absoluta al archivo de configuración.
func (g *Gestor) RutaArchivoConfig() string {
	return expandirHome(g.rutaArchivo)
}

// RutaRuntime retorna la ruta absoluta al directorio de runtime.
func (g *Gestor) RutaRuntime() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return expandirHome(g.config.DirectorioBase)
}

// ============================================================================
// Funciones Internas
// ============================================================================

// obtenerCampo obtiene el valor de un campo por ruta dot-notation.
// Retorna el valor como interface{} para permitir cualquier tipo.
func (g *Gestor) obtenerCampo(ruta string) interface{} {
	parts := strings.Split(ruta, ".")
	v := reflect.ValueOf(g.config).Elem()

	for _, part := range parts {
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return nil
		}
		f := v.FieldByName(campoStruct(part))
		if !f.IsValid() {
			return nil
		}
		v = f
	}
	return v.Interface()
}

// establecerCampo modifica un campo por ruta dot-notation, parseando el valor como string.
func (g *Gestor) establecerCampo(ruta, valor string) error {
	parts := strings.Split(ruta, ".")
	v := reflect.ValueOf(g.config).Elem()

	for i, part := range parts {
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return fmt.Errorf("campo '%s' no es un struct", strings.Join(parts[:i+1], "."))
		}
		nombreCampo := campoStruct(part)
		f := v.FieldByName(nombreCampo)
		if !f.IsValid() {
			return fmt.Errorf("campo '%s' no encontrado en struct", strings.Join(parts[:i+1], "."))
		}

		// Si es el último campo, asignar el valor
		if i == len(parts)-1 {
			return asignarValor(f, valor)
		}

		v = f
	}
	return nil
}

// establecerCampoInterface asigna un valor interface{} a un campo por ruta.
// Usado para rollback de cambios.
func (g *Gestor) establecerCampoInterface(ruta string, valor interface{}) error {
	if valor == nil {
		return nil
	}
	parts := strings.Split(ruta, ".")
	v := reflect.ValueOf(g.config).Elem()

	for i, part := range parts {
		for v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		nombreCampo := campoStruct(part)
		f := v.FieldByName(nombreCampo)
		if !f.IsValid() {
			return fmt.Errorf("campo no encontrado: %s", strings.Join(parts[:i+1], "."))
		}
		if i == len(parts)-1 {
			refVal := reflect.ValueOf(valor)
			if f.Type().AssignableTo(refVal.Type()) {
				f.Set(refVal)
			}
			return nil
		}
		v = f
	}
	return nil
}

// asignarValor convierte un string al tipo apropiado del campo reflect.Value
// y lo asigna. Soporta int, float64, string y bool.
func asignarValor(f reflect.Value, valor string) error {
	if !f.CanSet() {
		return fmt.Errorf("campo no es escribible")
	}

	switch f.Kind() {
	case reflect.String:
		f.SetString(valor)
	case reflect.Int:
		n, err := strconv.Atoi(valor)
		if err != nil {
			return fmt.Errorf("'%s' no es un entero válido: %w", valor, err)
		}
		f.SetInt(int64(n))
	case reflect.Float64:
		n, err := strconv.ParseFloat(valor, 64)
		if err != nil {
			return fmt.Errorf("'%s' no es un número decimal válido: %w", valor, err)
		}
		f.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(valor)
		if err != nil {
			return fmt.Errorf("'%s' no es un booleano válido: %w", valor, err)
		}
		f.SetBool(b)
	default:
		return fmt.Errorf("tipo de campo '%s' no soportado para asignación dinámica", f.Kind())
	}
	return nil
}

// campoStruct convierte un nombre de campo en snake_case o kebab-case
// a PascalCase para coincidir con los campos del struct de Go.
func campoStruct(nombre string) string {
	parts := strings.Split(nombre, "_")
	if len(parts) > 1 {
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(part[:1]) + part[1:]
			}
		}
		return strings.Join(parts, "")
	}

	parts = strings.Split(nombre, "-")
	if len(parts) > 1 {
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(part[:1]) + part[1:]
			}
		}
		return strings.Join(parts, "")
	}

	return strings.ToUpper(nombre[:1]) + nombre[1:]
}

// expandirHome reemplaza el prefijo ~ por el directorio home del usuario.
func expandirHome(ruta string) string {
	if strings.HasPrefix(ruta, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ruta[2:])
	}
	return ruta
}

// aplicarDefaults establece valores por defecto para campos vacíos o cero.
// Esto garantiza que la configuración siempre tenga valores sensibles.
func aplicarDefaults(cfg *Configuracion) {
	if cfg.Puerto == 0 {
		cfg.Puerto = 8080
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.Nombre == "" {
		cfg.Nombre = "Liz"
	}
	if cfg.Version == "" {
		cfg.Version = "0.1.0"
	}
	if cfg.DirectorioBase == "" {
		cfg.DirectorioBase = "~/.liz"
	}

	// Defaults de seguridad
	if cfg.Seguridad.MaxPeticionesMin == 0 {
		cfg.Seguridad.MaxPeticionesMin = 60
	}
	if cfg.Seguridad.MaxTokensSesion == 0 {
		cfg.Seguridad.MaxTokensSesion = 100000
	}

	// Defaults de logging
	if cfg.Logging.Nivel == "" {
		cfg.Logging.Nivel = "info"
	}
	if cfg.Logging.Archivo == "" {
		cfg.Logging.Archivo = "~/.liz/logs/liz.log"
	}
	if cfg.Logging.MaxMB == 0 {
		cfg.Logging.MaxMB = 50
	}
	if cfg.Logging.MaxArchivos == 0 {
		cfg.Logging.MaxArchivos = 5
	}

	// Defaults de contexto
	if cfg.Contexto.Estrategia == "" {
		cfg.Contexto.Estrategia = "bajo_demanda"
	}
	if cfg.Contexto.MaxArchivos == 0 {
		cfg.Contexto.MaxArchivos = 50
	}
	if cfg.Contexto.MaxLineas == 0 {
		cfg.Contexto.MaxLineas = 5000
	}
	if cfg.Contexto.TamanoContexto == 0 {
		cfg.Contexto.TamanoContexto = 128000
	}

	// Defaults de modelos: si no hay ninguno, usar el catálogo NVIDIA por defecto
	// (IDs válidos para integrate.api.nvidia.com/v1). El campo `Nombre` se envía
	// tal cual como `model` en el body del request a la API, así que DEBE ser
	// un ID NVIDIA válido (ej: "meta/llama-3.1-70b-instruct"), no un nombre
	// para humanos. Ver issue #22.
	if len(cfg.Modelos) == 0 {
		cfg.Modelos = defaultsModelosNVIDIA()
	}

	// Defaults de herramientas: si no hay ninguna, registrar las básicas
	if len(cfg.Herramientas) == 0 {
		cfg.Herramientas = []ConfiguracionHerramienta{
			{Nombre: "terminal", Habilitado: true},
			{Nombre: "navegador_archivos", Habilitado: true},
			{Nombre: "buscador", Habilitado: true},
			{Nombre: "editor", Habilitado: true},
			{Nombre: "procesos", Habilitado: true},
			{Nombre: "monitor", Habilitado: true},
			{Nombre: "instalador", Habilitado: true},
		}
	}
}

// aplicarOverridesEnv sobreescribe campos de configuración con variables de entorno.
// Prioridad: variables de entorno > archivo YAML > valores por defecto.
func aplicarOverridesEnv(cfg *Configuracion) {
	if v := os.Getenv("LIZ_PUERTO"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Puerto = n
		}
	}
	if v := os.Getenv("LIZ_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("LIZ_NOMBRE"); v != "" {
		cfg.Nombre = v
	}
	if v := os.Getenv("LIZ_VERSION"); v != "" {
		cfg.Version = v
	}
	if v := os.Getenv("LIZ_DIRECTORIO_BASE"); v != "" {
		cfg.DirectorioBase = v
	}
	if v := os.Getenv("LIZ_NIVEL_LOG"); v != "" {
		cfg.Logging.Nivel = v
	}
	nvidiaAPIKey := os.Getenv("NVIDIA_API_KEY")
	if nvidiaAPIKey != "" {
		// Aplicar a todos los modelos de NVIDIA
		for i := range cfg.Modelos {
			if cfg.Modelos[i].Proveedor == "nvidia" {
				cfg.Modelos[i].APIKey = nvidiaAPIKey
			}
		}
	} else if !algunaAPIKeyNVIDIA(cfg) {
		// Fallback: si no hay NVIDIA_API_KEY y ningún modelo trae api_key,
		// usar la API key bundled por defecto (issue #29).
		for i := range cfg.Modelos {
			if cfg.Modelos[i].Proveedor == "nvidia" && cfg.Modelos[i].APIKey == "" {
				cfg.Modelos[i].APIKey = APIKeyNvidiaPorDefecto
			}
		}
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		for i := range cfg.Modelos {
			if cfg.Modelos[i].Proveedor == "openai" {
				cfg.Modelos[i].APIKey = v
			}
		}
	}
}

// algunaAPIKeyNVIDIA retorna true si algún modelo de NVIDIA ya tiene APIKey
// configurada (distinta de vacío). Se usa para decidir si aplicar la default
// bundled (issue #29).
func algunaAPIKeyNVIDIA(cfg *Configuracion) bool {
	for _, m := range cfg.Modelos {
		if m.Proveedor == "nvidia" && m.APIKey != "" {
			return true
		}
	}
	return false
}

// compararConfiguraciones compara dos configuraciones y retorna los cambios.
func compararConfiguraciones(antigua, nueva *Configuracion) []CambioConfiguracion {
	var cambios []CambioConfiguracion
	ts := timestampActual()

	v1 := reflect.ValueOf(antigua).Elem()
	v2 := reflect.ValueOf(nueva).Elem()
	cambios = compararStructs(v1, v2, "", cambios, ts)

	return cambios
}

// compararStructs compara recursivamente dos structs reflect.Value.
func compararStructs(v1, v2 reflect.Value, prefijo string, cambios []CambioConfiguracion, ts string) []CambioConfiguracion {
	t := v1.Type()
	for i := 0; i < v1.NumField(); i++ {
		campo := t.Field(i)
		nombre := campo.Name
		ruta := nombre
		if prefijo != "" {
			ruta = prefijo + "." + nombre
		}

		f1 := v1.Field(i)
		f2 := v2.Field(i)

		if f1.Kind() == reflect.Struct {
			cambios = compararStructs(f1, f2, ruta, cambios, ts)
		} else if !reflect.DeepEqual(f1.Interface(), f2.Interface()) {
			cambios = append(cambios, CambioConfiguracion{
				Ruta:          ruta,
				ValorAnterior: f1.Interface(),
				ValorNuevo:    f2.Interface(),
				Timestamp:     ts,
			})
		}
	}
	return cambios
}

// timestampActual retorna el timestamp actual en formato ISO 8601.
func timestampActual() string {
	return time.Now().Format("2006-01-02T15:04:05Z07:00")
}

// ============================================================================
// Configuración por Defecto
// ============================================================================

// ConfiguracionPorDefecto retorna una configuración completa con valores
// sensibles por defecto. Se usa cuando no existe archivo de configuración.
func ConfiguracionPorDefecto() Configuracion {
	return Configuracion{
		Puerto:         8080,
		Host:           "0.0.0.0",
		Nombre:         "Liz",
		Version:        "0.1.0",
		DirectorioBase: "~/.liz",
		Modelos: defaultsModelosNVIDIA(),
		Herramientas: []ConfiguracionHerramienta{
			{
				Nombre:     "terminal",
				Tipo:       "ejecutable",
				Ruta:       "/bin/bash",
				Timeout:    30,
				Habilitado: true,
			},
			{
				Nombre:     "navegador",
				Tipo:       "ejecutable",
				Ruta:       "python3 -m playwright",
				Timeout:    60,
				Habilitado: false,
			},
		},
		Seguridad: ConfiguracionSeguridad{
			SandboxHabilitado: false,
			MaxPeticionesMin:  60,
			MaxTokensSesion:   100000,
			PermitirRed:       true,
			PermitirSistema:   true,
		},
		Logging: ConfiguracionLogging{
			Nivel:       "info",
			Archivo:     "~/.liz/logs/liz.log",
			Stdout:      true,
			Rotacion:    true,
			MaxMB:       50,
			MaxArchivos: 5,
		},
		Contexto: ConfiguracionContexto{
			Estrategia:         "bajo_demanda",
			MaxArchivos:        50,
			MaxLineas:          5000,
			TamanoContexto:     128000,
			ResumenAuto:        true,
			CatalogoHabilitado: true,
		},
	}
}


// defaultsModelosNVIDIA retorna el catálogo de modelos NVIDIA por defecto.
//
// El campo `Nombre` se envía tal cual como `model` en el body del request a
// la API de NVIDIA (integrate.api.nvidia.com/v1/chat/completions), así que
// DEBE ser un ID NVIDIA válido (ej: "meta/llama-3.1-70b-instruct"), no un
// nombre para humanos. Ver issue #22.
//
// La URL NO debe incluir `/chat/completions` — el cliente NVIDIA lo concatena
// automáticamente. Ver issue #22.
//
// Solo se incluyen modelos NVIDIA porque el orquestador (Fase 4) solo soporta
// el endpoint de NVIDIA (formato OpenAI-compatible). Ver issue #28.
func defaultsModelosNVIDIA() []ConfiguracionModelo {
	return []ConfiguracionModelo{
		{
			Nombre:      "meta/llama-3.1-70b-instruct",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   4096,
			Rol:         "principal",
			Habilitado:  true,
		},
		{
			Nombre:      "meta/llama-3.1-405b-instruct",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   8192,
			Rol:         "reserva",
			Habilitado:  true,
		},
		{
			Nombre:      "mistralai/mixtral-8x22b-instruct-v0.1",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   8192,
			Rol:         "reserva",
			Habilitado:  true,
		},
		{
			Nombre:      "nvidia/llama-3.1-nemotron-70b-instruct",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   8192,
			Rol:         "reserva",
			Habilitado:  true,
		},
		{
			Nombre:      "meta/codellama-70b-instruct",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   8192,
			Rol:         "especializado",
			Habilitado:  true,
		},
		{
			Nombre:      "google/gemma-2-27b-it",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   8192,
			Rol:         "especializado",
			Habilitado:  true,
		},
		{
			Nombre:      "microsoft/phi-3-medium-128k-instruct",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   8192,
			Rol:         "especializado",
			Habilitado:  true,
		},
		{
			Nombre:      "nvidia/nemotron-4-340b-instruct",
			Proveedor:   "nvidia",
			URL:         "https://integrate.api.nvidia.com/v1",
			Temperatura: 0.7,
			TopP:        0.9,
			MaxTokens:   8192,
			Rol:         "especializado",
			Habilitado:  true,
		},
	}
}
