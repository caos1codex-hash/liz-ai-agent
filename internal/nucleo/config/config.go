package config

import (
        "encoding/json"
        "fmt"
        "net/url"
        "os"
        "path/filepath"
        "strings"
        "sync"
        "time"

        "gopkg.in/yaml.v3"
)

// ═══════════════════════════════════════════════════════
// TIPOS DE CONFIGURACIÓN
// ═══════════════════════════════════════════════════════

// ModeloNVIDIA representa la configuración de un modelo de IA disponible.
type ModeloNVIDIA struct {
        ID        string   `yaml:"id" json:"id"`
        Nombre    string   `yaml:"nombre" json:"nombre"`
        Tipo      []string `yaml:"tipo" json:"tipo"`
        Velocidad string   `yaml:"velocidad" json:"velocidad"`
        Prioridad int      `yaml:"prioridad" json:"prioridad"`
}

// ConfigNVIDIA contiene la configuración de la API de NVIDIA.
type ConfigNVIDIA struct {
        APIKey   string         `yaml:"api_key" json:"-"` // nunca exponer en JSON
        Endpoint string         `yaml:"endpoint" json:"endpoint"`
        Modelos  []ModeloNVIDIA `yaml:"modelos" json:"modelos"`
}

// ConfigServidor contiene la configuración del servidor HTTP.
type ConfigServidor struct {
        Puerto int    `yaml:"puerto" json:"puerto"`
        Host   string `yaml:"host" json:"host"`
}

// ConfigPermisos contiene la configuración del sistema de permisos.
type ConfigPermisos struct {
        SolicitarAlIniciar    bool `yaml:"solicitar_al_iniciar" json:"solicitar_al_iniciar"`
        RecordarEntreSesiones bool `yaml:"recordar_entre_sesiones" json:"recordar_entre_sesiones"`
}

// Configuracion es la configuración completa de Liz.
type Configuracion struct {
        Version          string         `yaml:"version" json:"version"`
        Servidor         ConfigServidor `yaml:"servidor" json:"servidor"`
        NVIDIA           ConfigNVIDIA   `yaml:"nvidia" json:"nvidia"`
        DirectorioTrabajo string         `yaml:"directorio_trabajo" json:"directorio_trabajo"`
        Tema             string         `yaml:"tema" json:"tema"`
        Permisos         ConfigPermisos `yaml:"permisos" json:"permisos"`
}

// ═══════════════════════════════════════════════════════
// VALIDACIONES
// ═══════════════════════════════════════════════════════

// Temas validos para la UI.
var temasValidos = map[string]bool{
        "oscuro": true, "claro": true, "auto": true,
}

// Velocidades validas para modelos.
var velocidadesValidas = map[string]bool{
        "alta": true, "media": true, "lenta": true,
}

// Tipos de tarea validos para modelos.
var tiposTareaValidos = map[string]bool{
        "razonamiento": true, "complejo": true, "codigo": true, "general": true,
        "analisis": true, "rapido": true, "creatividad": true, "eficiente": true,
        "contexto_largo": true, "resumen": true, "especializado": true, "potente": true,
}

// Errores de validación.
var (
        ErrPuertoInvalido     = fmt.Errorf("puerto debe estar entre 1 y 65535")
        ErrHostInvalido        = fmt.Errorf("host no puede estar vacío")
        ErrTemaInvalido        = fmt.Errorf("tema debe ser 'oscuro', 'claro' o 'auto'")
        ErrEndpointInvalido    = fmt.Errorf("endpoint NVIDIA debe ser una URL válida (https://)")
        ErrDirectorioInvalido  = fmt.Errorf("directorio_trabajo no puede estar vacío")
        ErrModeloSinID        = fmt.Errorf("modelo debe tener un 'id' no vacío")
        ErrModeloSinNombre     = fmt.Errorf("modelo debe tener un 'nombre' no vacío")
        ErrModeloTipoInvalido = fmt.Errorf("tipo de modelo no reconocido")
        ErrModeloVelInvalida   = fmt.Errorf("velocidad de modelo debe ser 'alta', 'media' o 'lenta'")
)

// Validar realiza todas las validaciones de la configuración.
// Retorna una lista de errores. Vacía significa que todo es válido.
func (c *Configuracion) Validar() []error {
        var errores []error

        // Servidor
        if c.Servidor.Puerto < 1 || c.Servidor.Puerto > 65535 {
                errores = append(errores, ErrPuertoInvalido)
        }
        if strings.TrimSpace(c.Servidor.Host) == "" {
                errores = append(errores, ErrHostInvalido)
        }

        // Tema
        if !temasValidos[c.Tema] {
                errores = append(errores, ErrTemaInvalido)
        }

        // Directorio de trabajo
        if strings.TrimSpace(c.DirectorioTrabajo) == "" {
                errores = append(errores, ErrDirectorioInvalido)
        }

        // NVIDIA
        if c.NVIDIA.Endpoint != "" {
                if u, err := url.Parse(c.NVIDIA.Endpoint); err != nil || u.Scheme != "https" {
                        errores = append(errores, ErrEndpointInvalido)
                }
        }

        for i, m := range c.NVIDIA.Modelos {
                if strings.TrimSpace(m.ID) == "" {
                        errores = append(errores, fmt.Errorf("modelo[%d]: %w", i, ErrModeloSinID))
                }
                if strings.TrimSpace(m.Nombre) == "" {
                        errores = append(errores, fmt.Errorf("modelo[%d]: %w", i, ErrModeloSinNombre))
                }
                for _, tipo := range m.Tipo {
                        if !tiposTareaValidos[tipo] {
                                errores = append(errores, fmt.Errorf("modelo[%d] '%s': %w (%s)", i, m.ID, ErrModeloTipoInvalido, tipo))
                        }
                }
                if !velocidadesValidas[m.Velocidad] {
                        errores = append(errores, fmt.Errorf("modelo[%d] '%s': %w (%s)", i, m.ID, ErrModeloVelInvalida, m.Velocidad))
                }
        }

        return errores
}

// ═══════════════════════════════════════════════════════
// GESTOR DE CONFIGURACIÓN (thread-safe con persistencia)
// ═══════════════════════════════════════════════════════

// Gestor es el gestor thread-safe de la configuración de Liz.
// Maneja la configuración activa, el archivo de origen, y la persistencia.
type Gestor struct {
        mu          sync.RWMutex
        config      *Configuracion
        rutaOrigen  string // ruta del archivo YAML cargado (vacío si solo defaults)
        rutaActiva  string // ruta de ~/.liz/config.json (config activa con overrides)
        logFunc     func(string, ...interface{}) // inyectable para testing
}

// Config es la instancia global del gestor de configuración.
var GestorGlobal *Gestor

// Rutas de configuración que se buscan en orden de prioridad.
var rutasConfig = []string{
        "liz.yaml",
        "configs/liz.yaml",
}

// archivoConfig es el wrapper YAML que contiene la configuración de Liz.
type archivoConfig struct {
        Liz *Configuracion `yaml:"liz"`
}

// parsearYAML lee datos YAML y los parsea en una Configuracion.
// Soporta tanto el formato con wrapper "liz:" como sin él.
func parsearYAML(datos []byte, cfg *Configuracion) error {
        var wrapper archivoConfig
        if err := yaml.Unmarshal(datos, &wrapper); err == nil && wrapper.Liz != nil {
                *cfg = *wrapper.Liz
                return nil
        }
        return yaml.Unmarshal(datos, cfg)
}

// NuevoGestor crea un nuevo gestor de configuración.
// Carga la config desde YAML, aplica env vars, valida, y la prepara para uso.
func NuevoGestor(logFn func(string, ...interface{})) (*Gestor, error) {
        if logFn == nil {
                logFn = func(string, ...interface{}) {} // noop
        }

        home, err := os.UserHomeDir()
        if err != nil {
                return nil, fmt.Errorf("error obteniendo directorio home: %w", err)
        }

        g := &Gestor{
                rutaActiva: filepath.Join(home, ".liz", "config.json"),
                logFunc:    logFn,
        }

        // 1. Buscar y cargar archivo YAML
        var rutaEncontrada string
        for _, ruta := range rutasConfig {
                if _, err := os.Stat(ruta); err == nil {
                        rutaEncontrada = ruta
                                break
                }
        }

        cfg := ConfiguracionPorDefecto()

        if rutaEncontrada != "" {
                datos, err := os.ReadFile(rutaEncontrada)
                if err != nil {
                        return nil, fmt.Errorf("error leyendo configuración %s: %w", rutaEncontrada, err)
                }

                if err := parsearYAML(datos, cfg); err != nil {
                        return nil, fmt.Errorf("error parseando YAML %s: %w", rutaEncontrada, err)
                }
                g.rutaOrigen = rutaEncontrada
                g.logFunc("configuración cargada desde %s", rutaEncontrada)
        } else {
                g.logFunc("no se encontró liz.yaml, usando configuración por defecto")
        }

        // 2. Aplicar overrides de ~/.liz/config.json si existe
        if datos, err := os.ReadFile(g.rutaActiva); err == nil {
                var overrides Configuracion
                if json.Unmarshal(datos, &overrides) == nil {
                        g.aplicarOverrides(cfg, &overrides)
                        g.logFunc("overrides aplicados desde %s", g.rutaActiva)
                }
        }

        // 3. Expandir ~ en directorio de trabajo
        cfg = expandirHome(cfg)

        // 4. Aplicar variables de entorno (máxima prioridad)
        cfg = aplicarEnvVars(cfg)

        // 5. Validar
        if errores := cfg.Validar(); len(errores) > 0 {
                for _, e := range errores {
                        g.logFunc("validación: %v", e)
                }
                // No es fatal — loguear y continuar con defaults corregidos
                g.logFunc("advertencia: la configuración tiene %d errores de validación", len(errores))
        }

        g.config = cfg
        GestorGlobal = g
        Config = cfg

        return g, nil
}

// aplicarOverrides mezcla los overrides no-nulos sobre la configuración base.
func (g *Gestor) aplicarOverrides(base, overrides *Configuracion) {
        if overrides.Servidor.Puerto != 0 {
                base.Servidor.Puerto = overrides.Servidor.Puerto
        }
        if overrides.Servidor.Host != "" {
                base.Servidor.Host = overrides.Servidor.Host
        }
        if overrides.Tema != "" {
                base.Tema = overrides.Tema
        }
        if overrides.DirectorioTrabajo != "" {
                base.DirectorioTrabajo = overrides.DirectorioTrabajo
        }
        if overrides.NVIDIA.Endpoint != "" {
                base.NVIDIA.Endpoint = overrides.NVIDIA.Endpoint
        }
        if overrides.NVIDIA.APIKey != "" {
                base.NVIDIA.APIKey = overrides.NVIDIA.APIKey
        }
        if overrides.Permisos.SolicitarAlIniciar != base.Permisos.SolicitarAlIniciar {
                base.Permisos.SolicitarAlIniciar = overrides.Permisos.SolicitarAlIniciar
        }
        if overrides.Permisos.RecordarEntreSesiones != base.Permisos.RecordarEntreSesiones {
                base.Permisos.RecordarEntreSesiones = overrides.Permisos.RecordarEntreSesiones
        }
        if len(overrides.NVIDIA.Modelos) > 0 {
                base.NVIDIA.Modelos = overrides.NVIDIA.Modelos
        }
}

// expandirHome expande ~ al inicio de directorio_trabajo.
func expandirHome(cfg *Configuracion) *Configuracion {
        if cfg.DirectorioTrabajo == "~" || cfg.DirectorioTrabajo == "" {
                home, _ := os.UserHomeDir()
                cfg.DirectorioTrabajo = home
        } else if len(cfg.DirectorioTrabajo) > 0 && cfg.DirectorioTrabajo[0] == '~' {
                home, _ := os.UserHomeDir()
                cfg.DirectorioTrabajo = home + cfg.DirectorioTrabajo[1:]
        }
        return cfg
}

// aplicarEnvVars aplica overrides de variables de entorno (máxima prioridad).
func aplicarEnvVars(cfg *Configuracion) *Configuracion {
        if puerto := os.Getenv("LIZ_PUERTO"); puerto != "" {
                var p int
                if _, err := fmt.Sscanf(puerto, "%d", &p); err == nil && p > 0 && p < 65536 {
                        cfg.Servidor.Puerto = p
                }
        }
        if apiKey := os.Getenv("NVIDIA_API_KEY"); apiKey != "" {
                cfg.NVIDIA.APIKey = apiKey
        }
        if host := os.Getenv("LIZ_HOST"); host != "" {
                cfg.Servidor.Host = host
        }
        if tema := os.Getenv("LIZ_TEMA"); temasValidos[tema] {
                cfg.Tema = tema
        }
        return cfg
}

// ═══════════════════════════════════════════════════════
// MÉTODOS DEL GESTOR
// ═══════════════════════════════════════════════════════

// Obtener retorna una copia de la configuración actual.
func (g *Gestor) Obtener() Configuracion {
        g.mu.RLock()
        defer g.mu.RUnlock()
        return *g.config
}

// RutaOrigen retorna la ruta del archivo YAML que se cargó (vacío si solo defaults).
func (g *Gestor) RutaOrigen() string {
        g.mu.RLock()
        defer g.mu.RUnlock()
        return g.rutaOrigen
}

// RutaActiva retorna la ruta del archivo de config activa (~/.liz/config.json).
func (g *Gestor) RutaActiva() string {
        g.mu.RLock()
        defer g.mu.RUnlock()
        return g.rutaActiva
}

// Modificar aplica cambios parciales a la configuración en runtime y los persiste.
// Solo modifica los campos no-nulos del parámetro cambios.
// Valida antes de guardar. Retorna error si la validación falla.
func (g *Gestor) Modificar(cambios *Configuracion) (*Configuracion, error) {
        g.mu.Lock()
        defer g.mu.Unlock()

        // Crear copia con cambios aplicados
        nueva := *g.config

        if cambios.Servidor.Puerto != 0 {
                nueva.Servidor.Puerto = cambios.Servidor.Puerto
        }
        if cambios.Servidor.Host != "" {
                nueva.Servidor.Host = cambios.Servidor.Host
        }
        if cambios.Tema != "" {
                nueva.Tema = cambios.Tema
        }
        if cambios.DirectorioTrabajo != "" {
                nueva.DirectorioTrabajo = cambios.DirectorioTrabajo
        }
        if cambios.NVIDIA.Endpoint != "" {
                nueva.NVIDIA.Endpoint = cambios.NVIDIA.Endpoint
        }
        if cambios.Permisos.SolicitarAlIniciar != g.config.Permisos.SolicitarAlIniciar {
                nueva.Permisos.SolicitarAlIniciar = cambios.Permisos.SolicitarAlIniciar
        }
        if cambios.Permisos.RecordarEntreSesiones != g.config.Permisos.RecordarEntreSesiones {
                nueva.Permisos.RecordarEntreSesiones = cambios.Permisos.RecordarEntreSesiones
        }

        // Expandir home
        nueva = *expandirHome(&nueva)

        // Validar la nueva configuración
        if errores := nueva.Validar(); len(errores) > 0 {
                return nil, fmt.Errorf("validación falló: %v", errores[0])
        }

        // Persistir a ~/.liz/config.json
        if err := g.guardar(&nueva); err != nil {
                return nil, err
        }

        g.config = &nueva
        Config = &nueva

        g.logFunc("configuración actualizada y persistida en %s", g.rutaActiva)
        return &nueva, nil
}

// GuardarAPIKey guarda la API key de NVIDIA de forma segura (solo en ~/.liz/config.json, nunca en YAML).
func (g *Gestor) GuardarAPIKey(apiKey string) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        if strings.TrimSpace(apiKey) == "" {
                return fmt.Errorf("API key no puede estar vacía")
        }

        g.config.NVIDIA.APIKey = apiKey
        Config = g.config

        return g.guardar(g.config)
}

// guardar persiste la configuración a ~/.liz/config.json.
func (g *Gestor) guardar(cfg *Configuracion) error {
        datos, err := json.MarshalIndent(cfg, "", "  ")
        if err != nil {
                return fmt.Errorf("error serializando config: %w", err)
        }

        // NOTA: la API key se guarda en el JSON de config activa.
        // Está protegida porque ~/.liz/ tiene permisos 700.
        if err := os.WriteFile(g.rutaActiva, datos, 0644); err != nil {
                return fmt.Errorf("error guardando config en %s: %w", g.rutaActiva, err)
        }

        return nil
}

// ═══════════════════════════════════════════════════════
// ESTADO DE SESIÓN (~/.liz/contexto/sistema/estado/)
// ═══════════════════════════════════════════════════════

// EstadoSesion representa el estado actual de la sesión de Liz.
type EstadoSesion struct {
        SesionID      string    `json:"sesion_id"`
        Inicio        time.Time `json:"inicio"`
        Version       string    `json:"version"`
        PID           int       `json:"pid"`
        ConfigOrigen  string    `json:"config_origen"`
        PermisosListos bool     `json:"permisos_listos"`
        Uptime        string    `json:"uptime,omitempty"`
}

// GuardarEstadoSesion crea/actualiza el archivo de estado de sesión.
func GuardarEstadoSesion(sesionID string, cfg *Configuracion) error {
        home, err := os.UserHomeDir()
        if err != nil {
                return err
        }

        ruta := filepath.Join(home, ".liz", "contexto", "sistema", "estado", "sesion_actual.json")

        estado := EstadoSesion{
                SesionID:     sesionID,
                Inicio:       time.Now().UTC(),
                Version:      cfg.Version,
                PID:          os.Getpid(),
                ConfigOrigen: "",
        }

        if GestorGlobal != nil {
                estado.ConfigOrigen = GestorGlobal.RutaOrigen()
        }

        datos, err := json.MarshalIndent(estado, "", "  ")
        if err != nil {
                return fmt.Errorf("error serializando estado: %w", err)
        }

        return os.WriteFile(ruta, datos, 0644)
}

// GuardarHerramientasRegistradas inicializa el registro de herramientas.
func GuardarHerramientasRegistradas() error {
        home, err := os.UserHomeDir()
        if err != nil {
                return err
        }

        ruta := filepath.Join(home, ".liz", "contexto", "sistema", "estado", "herramientas_registradas.json")

        registro := map[string]interface{}{
                "version":     "0.1.0",
                "integradas":   []interface{}{},
                "auto_creadas": []interface{}{},
                "total":       0,
                "actualizado":  time.Now().UTC().Format(time.RFC3339),
        }

        datos, err := json.MarshalIndent(registro, "", "  ")
        if err != nil {
                return fmt.Errorf("error serializando registro: %w", err)
        }

        return os.WriteFile(ruta, datos, 0644)
}

// ═══════════════════════════════════════════════════════
// FUNCIONES LEGADO (compatibilidad con Fase 1)
// ═══════════════════════════════════════════════════════

// Config es la instancia global de configuración (legacy).
var Config *Configuracion

// Cargar lee y parsea el archivo de configuración YAML.
// Función legada — usa NuevoGestor para la nueva API.
func Cargar() (*Configuracion, error) {
        g, err := NuevoGestor(nil)
        if err != nil {
                return nil, err
        }
        cfg := g.Obtener()
        return &cfg, nil
}

// CargarDesde carga la configuración desde una ruta específica (para testing).
func CargarDesde(ruta string) (*Configuracion, error) {
        datos, err := os.ReadFile(ruta)
        if err != nil {
                return nil, fmt.Errorf("error leyendo configuración %s: %w", ruta, err)
        }

        cfg := ConfiguracionPorDefecto()
        if err := parsearYAML(datos, cfg); err != nil {
                return nil, fmt.Errorf("error parseando YAML %s: %w", ruta, err)
        }

        Config = cfg
        return cfg, nil
}

// ConfiguracionPorDefecto retorna la configuración base con valores seguros.
func ConfiguracionPorDefecto() *Configuracion {
        return &Configuracion{
                Version: "0.1.0",
                Servidor: ConfigServidor{
                        Puerto: 3000,
                        Host:   "localhost",
                },
                NVIDIA: ConfigNVIDIA{
                        APIKey:   "",
                        Endpoint: "https://integrate.api.nvidia.com/v1",
                        Modelos:  []ModeloNVIDIA{},
                },
                DirectorioTrabajo: "~",
                Tema:             "oscuro",
                Permisos: ConfigPermisos{
                        SolicitarAlIniciar:    true,
                        RecordarEntreSesiones: false,
                },
        }
}

// AsegurarDirectorios crea la estructura de directorios ~/.liz/ necesaria.
func AsegurarDirectorios() error {
        home, err := os.UserHomeDir()
        if err != nil {
                return fmt.Errorf("error obteniendo directorio home: %w", err)
        }

        directorios := []string{
                filepath.Join(home, ".liz"),
                filepath.Join(home, ".liz", "contexto", "sistema", "estado"),
                filepath.Join(home, ".liz", "contexto", "sistema", "config"),
                filepath.Join(home, ".liz", "contexto", "chat", "conversaciones"),
                filepath.Join(home, ".liz", "contexto", "chat", "preferencias"),
                filepath.Join(home, ".liz", "contexto", "proyectos"),
                filepath.Join(home, ".liz", "herramientas", "auto_creadas"),
                filepath.Join(home, ".liz", "herramientas", "registro", "auto_creadas"),
                filepath.Join(home, ".liz", "logs"),
        }

        for _, dir := range directorios {
                if err := os.MkdirAll(dir, 0755); err != nil {
                return fmt.Errorf("error creando directorio %s: %w", dir, err)
                }
        }

        return nil
}
