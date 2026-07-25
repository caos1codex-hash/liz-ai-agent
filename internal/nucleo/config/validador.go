package config

import (
        "fmt"
        "net"
        "reflect"
        "regexp"
        "strconv"
        "strings"
)

// ============================================================================
// Tipos de Validación
// ============================================================================

// TipoValidacion define los tipos de validación soportados para cada campo.
type TipoValidacion int

const (
        ValidacionRequerido TipoValidacion = iota
        ValidacionRangoInt
        ValidacionRangoFloat
        ValidacionUnoDe
        ValidacionFormato
        ValidacionPuerto
        ValidacionURL
        ValidacionPath
)

// ReglaValidacion define una regla individual de validación para un campo.
// Cada campo puede tener múltiples reglas que se evalúan en orden.
type ReglaValidacion struct {
        Campo    string         // Campo del struct (PascalCase)
        Ruta     string         // Ruta en dot-notation (para API)
        Tipo     TipoValidacion // Tipo de validación
        Requerido bool          // Si es true, el campo no puede estar vacío
        Mensaje  string         // Mensaje de error personalizado

        // Parámetros según el tipo
        MinInt    int     // Para RangoInt
        MaxInt    int     // Para RangoInt
        MinFloat  float64 // Para RangoFloat
        MaxFloat  float64 // Para RangoFloat
        Valores   []string // Para UnoDe
        Patron    string  // Para Formato (regex)
}

// EsquemaConfig describe el esquema completo de configuración para
// documentación y auto-generación de APIs.
type EsquemaConfig struct {
        Campos []EsquemaCampo `json:"campos"`
}

// EsquemaCampo describe un campo individual del esquema.
type EsquemaCampo struct {
        Ruta      string   `json:"ruta"`
        Tipo      string   `json:"tipo"`
        Requerido bool     `json:"requerido"`
        Defecto   string   `json:"defecto"`
        Descripcion string `json:"descripcion"`
        Opciones  []string `json:"opciones,omitempty"` // Para campos UnoDe
        Rango     *RangoEsquema `json:"rango,omitempty"`
}

// RangoEsquema describe un rango válido para un campo numérico.
type RangoEsquema struct {
        Min string `json:"min"`
        Max string `json:"max"`
}

// ErrorValidacion representa un error de validación individual.
// Contiene el campo, la ruta y el mensaje descriptivo.
type ErrorValidacion struct {
        Campo   string `json:"campo"`
        Ruta    string `json:"ruta"`
        Mensaje string `json:"mensaje"`
}

func (e ErrorValidacion) Error() string {
        return fmt.Sprintf("[%s] %s", e.Ruta, e.Mensaje)
}

// ErroresValidacion es una colección de errores de validación.
type ErroresValidacion []ErrorValidacion

func (errs ErroresValidacion) Error() string {
        if len(errs) == 0 {
                return ""
        }
        mensajes := make([]string, len(errs))
        for i, e := range errs {
                mensajes[i] = e.Error()
        }
        return strings.Join(mensajes, "; ")
}

// ============================================================================
// Validador de Configuración
// ============================================================================

// ValidadorConfig es el validador central de la configuración.
// Contiene todas las reglas y ejecuta la validación en cascada.
type ValidadorConfig struct {
        reglas []ReglaValidacion
}

// NuevoValidadorConfig crea un validador con todas las reglas predefinidas
// para la estructura de configuración de Liz.
func NuevoValidadorConfig() *ValidadorConfig {
        v := &ValidadorConfig{}
        v.definirReglas()
        return v
}

// definirReglas establece todas las reglas de validación para cada campo
// de la configuración. Este es el corazón del sistema de validación.
func (v *ValidadorConfig) definirReglas() {
        v.reglas = []ReglaValidacion{
                // --- Campos raíz ---
                {
                        Campo: "Puerto", Ruta: "puerto", Tipo: ValidacionPuerto,
                        Requerido: true, Mensaje: "puerto debe estar entre 1 y 65535",
                },
                {
                        Campo: "Host", Ruta: "host", Tipo: ValidacionFormato,
                        Requerido: true, Patron: `^[a-zA-Z0-9.\-]+$`,
                        Mensaje: "host debe ser una dirección IP o hostname válido",
                },
                {
                        Campo: "Nombre", Ruta: "nombre", Tipo: ValidacionRequerido,
                        Requerido: true, Mensaje: "nombre del agente es requerido",
                },
                {
                        Campo: "Version", Ruta: "version", Tipo: ValidacionFormato,
                        Patron: `^\d+\.\d+\.\d+.*$`,
                        Mensaje: "versión debe seguir formato semver (ej: 1.0.0)",
                },
                {
                        Campo: "DirectorioBase", Ruta: "directorio_base", Tipo: ValidacionPath,
                        Requerido: true, Mensaje: "directorio_base es requerido",
                },

                // --- Seguridad ---
                {
                        Campo: "MaxPeticionesMin", Ruta: "seguridad.max_peticiones_min",
                        Tipo: ValidacionRangoInt, MinInt: 1, MaxInt: 1000,
                        Mensaje: "max_peticiones_min debe estar entre 1 y 1000",
                },
                {
                        Campo: "MaxTokensSesion", Ruta: "seguridad.max_tokens_sesion",
                        Tipo: ValidacionRangoInt, MinInt: 1000, MaxInt: 10000000,
                        Mensaje: "max_tokens_sesion debe estar entre 1,000 y 10,000,000",
                },

                // --- Logging ---
                {
                        Campo: "Nivel", Ruta: "logging.nivel", Tipo: ValidacionUnoDe,
                        Valores: []string{"debug", "info", "advertencia", "error", "silencio"},
                        Mensaje: "nivel debe ser: debug, info, advertencia, error o silencio",
                },
                {
                        Campo: "MaxMB", Ruta: "logging.max_mb", Tipo: ValidacionRangoInt,
                        MinInt: 1, MaxInt: 1000,
                        Mensaje: "max_mb debe estar entre 1 y 1000",
                },
                {
                        Campo: "MaxArchivos", Ruta: "logging.max_archivos", Tipo: ValidacionRangoInt,
                        MinInt: 1, MaxInt: 100,
                        Mensaje: "max_archivos debe estar entre 1 y 100",
                },

                // --- Contexto ---
                {
                        Campo: "Estrategia", Ruta: "contexto.estrategia", Tipo: ValidacionUnoDe,
                        Valores: []string{"bajo_demanda", "eager", "hibrido"},
                        Mensaje: "estrategia debe ser: bajo_demanda, eager o hibrido",
                },
                {
                        Campo: "MaxArchivos", Ruta: "contexto.max_archivos", Tipo: ValidacionRangoInt,
                        MinInt: 1, MaxInt: 1000,
                        Mensaje: "contexto.max_archivos debe estar entre 1 y 1000",
                },
                {
                        Campo: "MaxLineas", Ruta: "contexto.max_lineas", Tipo: ValidacionRangoInt,
                        MinInt: 100, MaxInt: 100000,
                        Mensaje: "contexto.max_lineas debe estar entre 100 y 100,000",
                },
                {
                        Campo: "TamanoContexto", Ruta: "contexto.tamano_contexto", Tipo: ValidacionRangoInt,
                        MinInt: 1000, MaxInt: 1000000,
                        Mensaje: "contexto.tamano_contexto debe estar entre 1,000 y 1,000,000",
                },
        }
}

// ============================================================================
// Validación Pública
// ============================================================================

// Validar ejecuta todas las reglas de validación contra la configuración.
// Retorna un ErroresValidacion con todos los errores encontrados, o nil si
// la configuración es válida.
func (v *ValidadorConfig) Validar(cfg *Configuracion) error {
        var errores ErroresValidacion

        for _, regla := range v.reglas {
                if err := v.validarRegla(cfg, regla); err != nil {
                        errores = append(errores, *err)
                }
        }

        // Validación de modelos (regla especial)
        if err := v.validarModelos(cfg); err != nil {
                errores = append(errores, *err)
        }

        if len(errores) == 0 {
                return nil
        }
        return errores
}

// ValidarCampo valida un valor individual contra la regla correspondiente.
// Se usa para validar cambios antes de aplicarlos.
func (v *ValidadorConfig) ValidarCampo(ruta, valor string) error {
        for _, regla := range v.reglas {
                if regla.Ruta == ruta {
                        // IMPORTANTE: capturar el puntero en una variable local antes de
                        // retornarlo como interfaz error. Si retornamos directamente
                        // validarValor(...) cuando retorna nil, el tipo subyacente
                        // (*ErrorValidacion)(nil) NO es igual a nil como interfaz, lo que
                        // rompe el `if err != nil` del llamador.
                        if err := v.validarValor(valor, regla); err != nil {
                                return err
                        }
                        return nil
                }
        }
        // Campo sin regla explícita — permitir
        return nil
}

// ============================================================================
// Generación de Esquema
// ============================================================================

// ObtenerEsquema genera el esquema de configuración a partir de las reglas.
// Es útil para documentación y auto-generación de APIs.
func (v *ValidadorConfig) ObtenerEsquema() EsquemaConfig {
        esquema := EsquemaConfig{
                Campos: make([]EsquemaCampo, 0, len(v.reglas)),
        }

        defectos := v.valoresPorDefecto()

        for _, regla := range v.reglas {
                campo := EsquemaCampo{
                        Ruta:        regla.Ruta,
                        Requerido:   regla.Requerido,
                        Descripcion: regla.Mensaje,
                }

                // Tipo
                switch regla.Tipo {
                case ValidacionPuerto, ValidacionRangoInt:
                        campo.Tipo = "integer"
                        if regla.Tipo == ValidacionPuerto {
                                campo.Rango = &RangoEsquema{Min: "1", Max: "65535"}
                        } else {
                                campo.Rango = &RangoEsquema{
                                        Min: strconv.Itoa(regla.MinInt),
                                        Max: strconv.Itoa(regla.MaxInt),
                                }
                        }
                case ValidacionRangoFloat:
                        campo.Tipo = "float"
                        campo.Rango = &RangoEsquema{
                                Min: fmt.Sprintf("%g", regla.MinFloat),
                                Max: fmt.Sprintf("%g", regla.MaxFloat),
                        }
                case ValidacionUnoDe:
                        campo.Tipo = "string"
                        campo.Opciones = regla.Valores
                case ValidacionFormato:
                        campo.Tipo = "string"
                case ValidacionPath:
                        campo.Tipo = "string"
                default:
                        campo.Tipo = "string"
                }

                // Valor por defecto
                if def, ok := defectos[regla.Ruta]; ok {
                        campo.Defecto = def
                }

                esquema.Campos = append(esquema.Campos, campo)
        }

        return esquema
}

// ============================================================================
// Validación Interna
// ============================================================================

// validarRegla ejecuta una regla de validación individual contra la configuración.
func (v *ValidadorConfig) validarRegla(cfg *Configuracion, regla ReglaValidacion) *ErrorValidacion {
        valor := v.obtenerValorCampo(cfg, regla.Campo)

        switch regla.Tipo {
        case ValidacionRequerido:
                if regla.Requerido && valor == "" {
                        return &ErrorValidacion{
                                Campo:   regla.Campo,
                                Ruta:    regla.Ruta,
                                Mensaje: regla.Mensaje,
                        }
                }

        case ValidacionRangoInt:
                n, ok := valor.(int)
                if !ok {
                        return nil // No se puede validar si no es int
                }
                if n < regla.MinInt || n > regla.MaxInt {
                        return &ErrorValidacion{
                                Campo:   regla.Campo,
                                Ruta:    regla.Ruta,
                                Mensaje: regla.Mensaje,
                        }
                }

        case ValidacionRangoFloat:
                f, ok := valor.(float64)
                if !ok {
                        return nil
                }
                if f < regla.MinFloat || f > regla.MaxFloat {
                        return &ErrorValidacion{
                                Campo:   regla.Campo,
                                Ruta:    regla.Ruta,
                                Mensaje: regla.Mensaje,
                        }
                }

        case ValidacionUnoDe:
                s, ok := valor.(string)
                if !ok || s == "" {
                        return nil
                }
                encontrado := false
                for _, valido := range regla.Valores {
                        if s == valido {
                                encontrado = true
                                break
                        }
                }
                if !encontrado {
                        return &ErrorValidacion{
                                Campo:   regla.Campo,
                                Ruta:    regla.Ruta,
                                Mensaje: regla.Mensaje,
                        }
                }

        case ValidacionFormato:
                s, ok := valor.(string)
                if !ok || s == "" {
                        return nil
                }
                if regla.Patron != "" {
                        matched, _ := regexp.MatchString(regla.Patron, s)
                        if !matched {
                                return &ErrorValidacion{
                                        Campo:   regla.Campo,
                                        Ruta:    regla.Ruta,
                                        Mensaje: regla.Mensaje,
                                }
                        }
                }

        case ValidacionPuerto:
                n, ok := valor.(int)
                if !ok {
                        return &ErrorValidacion{
                                Campo:   regla.Campo, Ruta: regla.Ruta,
                                Mensaje: "puerto debe ser un número entero",
                        }
                }
                if n < 1 || n > 65535 {
                        return &ErrorValidacion{
                                Campo:   regla.Campo, Ruta: regla.Ruta,
                                Mensaje: regla.Mensaje,
                        }
                }

        case ValidacionURL:
                s, ok := valor.(string)
                if !ok || s == "" {
                        return nil
                }
                if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
                        return &ErrorValidacion{
                                Campo:   regla.Campo, Ruta: regla.Ruta,
                                Mensaje: "URL debe comenzar con http:// o https://",
                        }
                }

        case ValidacionPath:
                s, ok := valor.(string)
                if !ok || s == "" {
                        return nil
                }
                if regla.Requerido && s == "" {
                        return &ErrorValidacion{
                                Campo:   regla.Campo, Ruta: regla.Ruta,
                                Mensaje: regla.Mensaje,
                        }
                }
        }

        return nil
}

// validarValor valida un valor string contra una regla (para cambios via API).
func (v *ValidadorConfig) validarValor(valor string, regla ReglaValidacion) *ErrorValidacion {
        switch regla.Tipo {
        case ValidacionPuerto:
                n, err := strconv.Atoi(valor)
                if err != nil {
                        return &ErrorValidacion{Ruta: regla.Ruta, Mensaje: "puerto debe ser un entero"}
                }
                if n < 1 || n > 65535 {
                        return &ErrorValidacion{Ruta: regla.Ruta, Mensaje: regla.Mensaje}
                }

        case ValidacionRangoInt:
                n, err := strconv.Atoi(valor)
                if err != nil {
                        return &ErrorValidacion{Ruta: regla.Ruta, Mensaje: "valor debe ser un entero"}
                }
                if n < regla.MinInt || n > regla.MaxInt {
                        return &ErrorValidacion{Ruta: regla.Ruta, Mensaje: regla.Mensaje}
                }

        case ValidacionUnoDe:
                encontrado := false
                for _, valido := range regla.Valores {
                        if valor == valido {
                                encontrado = true
                                break
                        }
                }
                if !encontrado {
                        return &ErrorValidacion{Ruta: regla.Ruta, Mensaje: regla.Mensaje}
                }

        case ValidacionFormato:
                if regla.Patron != "" {
                        matched, _ := regexp.MatchString(regla.Patron, valor)
                        if !matched {
                                return &ErrorValidacion{Ruta: regla.Ruta, Mensaje: regla.Mensaje}
                        }
                }
        }

        return nil
}

// validarModelos ejecuta validaciones específicas para la lista de modelos.
func (v *ValidadorConfig) validarModelos(cfg *Configuracion) *ErrorValidacion {
        if len(cfg.Modelos) == 0 {
                return &ErrorValidacion{
                        Ruta:    "modelos",
                        Mensaje: "debe haber al menos un modelo configurado",
                }
        }

        // Verificar que al menos un modelo está habilitado
        habilitado := false
        nombresVistos := make(map[string]bool)
        for _, m := range cfg.Modelos {
                if m.Habilitado {
                        habilitado = true
                }
                if m.Nombre == "" {
                        return &ErrorValidacion{
                                Ruta:    "modelos.nombre",
                                Mensaje: "cada modelo debe tener un nombre",
                        }
                }
                if nombresVistos[m.Nombre] {
                        return &ErrorValidacion{
                                Ruta:    "modelos.nombre",
                                Mensaje: fmt.Sprintf("nombre de modelo duplicado: %s", m.Nombre),
                        }
                }
                nombresVistos[m.Nombre] = true

                // Validar URL del modelo
                if m.URL != "" {
                        if !strings.HasPrefix(m.URL, "http://") && !strings.HasPrefix(m.URL, "https://") {
                                return &ErrorValidacion{
                                        Ruta:    "modelos.url",
                                        Mensaje: fmt.Sprintf("URL inválida para modelo %s", m.Nombre),
                                }
                        }
                }

                // Validar temperatura
                if m.Temperatura < 0 || m.Temperatura > 2.0 {
                        return &ErrorValidacion{
                                Ruta:    "modelos.temperatura",
                                Mensaje: fmt.Sprintf("temperatura inválida para modelo %s (0.0-2.0)", m.Nombre),
                        }
                }

                // Validar top_p
                if m.TopP < 0 || m.TopP > 1.0 {
                        return &ErrorValidacion{
                                Ruta:    "modelos.top_p",
                                Mensaje: fmt.Sprintf("top_p inválido para modelo %s (0.0-1.0)", m.Nombre),
                        }
                }
        }

        if !habilitado {
                return &ErrorValidacion{
                        Ruta:    "modelos",
                        Mensaje: "debe haber al menos un modelo habilitado",
                }
        }

        return nil
}

// obtenerValorCampo extrae el valor de un campo del struct usando reflexión.
func (v *ValidadorConfig) obtenerValorCampo(cfg *Configuracion, campo string) interface{} {
        // Campos anidados (ej: "MaxPeticionesMin" en Seguridad)
        subCampos := map[string]string{
                "MaxPeticionesMin": "Seguridad",
                "MaxTokensSesion":  "Seguridad",
                "SandboxHabilitado": "Seguridad",
                "PermitirRed":      "Seguridad",
                "PermitirSistema":  "Seguridad",
                "Nivel":            "Logging",
                "MaxMB":            "Logging",
                "MaxArchivos":      "Logging",
                "Estrategia":       "Contexto",
                "TamanoContexto":   "Contexto",
        }

        if subStruct, ok := subCampos[campo]; ok {
                var seccion interface{}
                switch subStruct {
                case "Seguridad":
                        seccion = cfg.Seguridad
                case "Logging":
                        seccion = cfg.Logging
                case "Contexto":
                        seccion = cfg.Contexto
                }

                val := reflect.ValueOf(seccion)
                if val.Kind() == reflect.Struct {
                        f := val.FieldByName(campo)
                        if f.IsValid() {
                                return f.Interface()
                        }
                }
                return nil
        }

        // Campos raíz
        val := reflect.ValueOf(cfg).Elem()
        f := val.FieldByName(campo)
        if f.IsValid() {
                return f.Interface()
        }
        return nil
}

// valoresPorDefecto retorna un mapa con los valores por defecto de cada campo.
func (v *ValidadorConfig) valoresPorDefecto() map[string]string {
        cfg := ConfiguracionPorDefecto()
        return map[string]string{
                "puerto":                  strconv.Itoa(cfg.Puerto),
                "host":                    cfg.Host,
                "nombre":                  cfg.Nombre,
                "version":                 cfg.Version,
                "directorio_base":         cfg.DirectorioBase,
                "seguridad.max_peticiones_min":  strconv.Itoa(cfg.Seguridad.MaxPeticionesMin),
                "seguridad.max_tokens_sesion":   strconv.Itoa(cfg.Seguridad.MaxTokensSesion),
                "logging.nivel":           cfg.Logging.Nivel,
                "logging.max_mb":          strconv.Itoa(cfg.Logging.MaxMB),
                "logging.max_archivos":    strconv.Itoa(cfg.Logging.MaxArchivos),
                "contexto.estrategia":     cfg.Contexto.Estrategia,
                "contexto.max_archivos":   strconv.Itoa(cfg.Contexto.MaxArchivos),
                "contexto.max_lineas":     strconv.Itoa(cfg.Contexto.MaxLineas),
                "contexto.tamano_contexto": strconv.Itoa(cfg.Contexto.TamanoContexto),
        }
}

// ValidarHost verifica que un host sea una dirección IP o hostname válido.
func ValidarHost(host string) error {
        // Intentar parsear como IP
        if ip := net.ParseIP(host); ip != nil {
                return nil
        }

        // Validar como hostname
        if len(host) > 253 {
                return fmt.Errorf("hostname demasiado largo (máximo 253 caracteres)")
        }

        hostnameRegex := `^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`
        matched, _ := regexp.MatchString(hostnameRegex, host)
        if !matched {
                return fmt.Errorf("'%s' no es un host válido", host)
        }

        return nil
}

// ValidarPuerto verifica que un puerto esté en el rango válido.
func ValidarPuerto(puerto int) error {
        if puerto < 1 || puerto > 65535 {
                return fmt.Errorf("puerto %d fuera de rango (1-65535)", puerto)
        }
        return nil
}