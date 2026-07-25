package config

import (
        "fmt"
        "os"
        "path/filepath"

        "gopkg.in/yaml.v3"
)

// ModeloNVIDIA representa la configuración de un modelo de IA disponible.
type ModeloNVIDIA struct {
        ID       string   `yaml:"id"`
        Nombre   string   `yaml:"nombre"`
        Tipo     []string `yaml:"tipo"`
        Velocidad string  `yaml:"velocidad"`
        Prioridad int     `yaml:"prioridad"`
}

// ConfigNVIDIA contiene la configuración de la API de NVIDIA.
type ConfigNVIDIA struct {
        APIKey   string          `yaml:"api_key"`
        Endpoint string          `yaml:"endpoint"`
        Modelos  []ModeloNVIDIA  `yaml:"modelos"`
}

// ConfigServidor contiene la configuración del servidor HTTP.
type ConfigServidor struct {
        Puerto int    `yaml:"puerto"`
        Host   string `yaml:"host"`
}

// ConfigPermisos contiene la configuración del sistema de permisos.
type ConfigPermisos struct {
        SolicitarAlIniciar  bool `yaml:"solicitar_al_iniciar"`
        RecordarEntreSesiones bool `yaml:"recordar_entre_sesiones"`
}

// Configuracion es la configuración completa de Liz leída desde liz.yaml.
type Configuracion struct {
        Version        string          `yaml:"version"`
        Servidor       ConfigServidor  `yaml:"servidor"`
        NVIDIA         ConfigNVIDIA    `yaml:"nvidia"`
        DirectorioTrabajo string       `yaml:"directorio_trabajo"`
        Tema           string          `yaml:"tema"`
        Permisos       ConfigPermisos  `yaml:"permisos"`
}

// Config es la instancia global de configuración.
var Config *Configuracion

// Rutas de configuración que se buscan en orden de prioridad.
var rutasConfig = []string{
        "liz.yaml",
        "configs/liz.yaml",
}

// archivoConfig es el wrapper YAML que contiene la configuración de Liz.
// El archivo liz.yaml usa "liz:" como raíz, esta estructura lo maneja.
type archivoConfig struct {
        Liz *Configuracion `yaml:"liz"`
}

// parsearYAML lee datos YAML y los parsea en una Configuracion.
// Soporta tanto el formato con wrapper "liz:" como sin él.
func parsearYAML(datos []byte, cfg *Configuracion) error {
        // Intentar primero con wrapper "liz:"
        var wrapper archivoConfig
        if err := yaml.Unmarshal(datos, &wrapper); err == nil && wrapper.Liz != nil {
                *cfg = *wrapper.Liz
                return nil
        }
        // Fallback: sin wrapper (formato directo)
        return yaml.Unmarshal(datos, cfg)
}

// Cargar lee y parsea el archivo de configuración YAML.
// Busca en el directorio actual y en configs/.
// Si no encuentra el archivo, retorna la configuración por defecto.
func Cargar() (*Configuracion, error) {
        var rutaEncontrada string

        // Buscar archivo de configuración en las rutas definidas
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
        }

        // Expandir ~ en directorio de trabajo
        if cfg.DirectorioTrabajo == "~" || cfg.DirectorioTrabajo == "" {
                home, err := os.UserHomeDir()
                if err != nil {
                        return nil, fmt.Errorf("error obteniendo directorio home: %w", err)
                }
                cfg.DirectorioTrabajo = home
        } else {
                // Expandir ~ al inicio de la ruta
                if cfg.DirectorioTrabajo[0] == '~' {
                        home, err := os.UserHomeDir()
                        if err != nil {
                                return nil, fmt.Errorf("error obteniendo directorio home: %w", err)
                        }
                        cfg.DirectorioTrabajo = home + cfg.DirectorioTrabajo[1:]
                }
        }

        // Permitir override con variables de entorno
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

        Config = cfg
        return cfg, nil
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
                Tema:           "oscuro",
                Permisos: ConfigPermisos{
                        SolicitarAlIniciar:   true,
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
