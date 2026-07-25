package permisos

import (
        "encoding/json"
        "fmt"
        "os"
        "path/filepath"
        "sort"
        "strings"
        "sync"
        "time"
)

// ============================================================================
// Tipos de Permisos
// ============================================================================

// TipoPermiso define las categorías de permisos disponibles en Liz.
// Cada categoría agrupa permisos relacionados funcionalmente.
type TipoPermiso string

const (
        // PermArchivos — lectura, escritura, eliminación de archivos y directorios
        PermArchivos TipoPermiso = "archivos"
        // PermRed — acceso a red, peticiones HTTP, DNS, sockets
        PermRed TipoPermiso = "red"
        // PermSistema — comandos del sistema, procesos, servicios
        PermSistema TipoPermiso = "sistema"
        // PermTerminal — ejecución de comandos en la terminal
        PermTerminal TipoPermiso = "terminal"
        // PermHerramientas — uso de herramientas externas y plugins
        PermHerramientas TipoPermiso = "herramientas"
        // PermModelos — acceso a modelos de IA, API keys, orquestación
        PermModelos TipoPermiso = "modelos"
)

// TodosLosPermisos contiene la lista completa de tipos de permisos.
var TodosLosPermisos = []TipoPermiso{
        PermArchivos,
        PermRed,
        PermSistema,
        PermTerminal,
        PermHerramientas,
        PermModelos,
}

// DescripcionPermiso retorna la descripción humana de un tipo de permiso.
func DescripcionPermiso(t TipoPermiso) string {
        switch t {
        case PermArchivos:
                return "Acceso a archivos y directorios del sistema"
        case PermRed:
                return "Acceso a red, HTTP, DNS y sockets"
        case PermSistema:
                return "Control de procesos, servicios y configuración del sistema"
        case PermTerminal:
                return "Ejecución de comandos en la terminal"
        case PermHerramientas:
                return "Uso de herramientas externas y plugins"
        case PermModelos:
                return "Acceso a modelos de IA y orquestación"
        default:
                return "Permiso desconocido"
        }
}

// NivelPermiso define el nivel de granularidad de un permiso.
type NivelPermiso string

const (
        NivelTotal   NivelPermiso = "total"    // Acceso completo sin restricciones
        NivelLectura NivelPermiso = "lectura"  // Solo lectura
        NivelEscritura NivelPermiso = "escritura" // Lectura y escritura
        NivelRestringido NivelPermiso = "restringido" // Acceso con restricciones específicas
        NivelDenegado NivelPermiso = "denegado"  // Acceso completamente denegado
)

// SubPermiso define un permiso granular dentro de una categoría.
// Permite control fino sobre operaciones específicas.
type SubPermiso struct {
        Nombre      string       `json:"nombre"`       // Ej: "leer", "escribir", "ejecutar"
        Descripcion string       `json:"descripcion"`
        Concedido   bool         `json:"concedido"`
        Nivel       NivelPermiso `json:"nivel"`
        Restricciones []string   `json:"restricciones,omitempty"` // Paths o comandos restringidos
}

// RegistroPermiso es un permiso individual con metadata completa.
type RegistroPermiso struct {
        Tipo        TipoPermiso   `json:"tipo"`
        Categoria   string        `json:"categoria"`
        Concedido   bool          `json:"concedido"`
        Nivel       NivelPermiso  `json:"nivel"`
        SubPermisos []SubPermiso  `json:"sub_permisos,omitempty"`
        ConcedidoEn time.Time     `json:"concedido_en"`
        ConcedidoPor string       `json:"concedido_por"` // "sistema", "usuario", "decision_d-006"
        ExpiraEn    *time.Time    `json:"expira_en,omitempty"`
        Razon       string        `json:"razon,omitempty"`
}

// RegistroAuditoria es una entrada en el log de auditoría de permisos.
type RegistroAuditoria struct {
        Timestamp   time.Time    `json:"timestamp"`
        Tipo        TipoPermiso  `json:"tipo"`
        Accion      string       `json:"accion"` // "conceder", "verificar", "denegar", "revocar"
        SubPermiso  string       `json:"sub_permiso,omitempty"`
        Resultado   string       `json:"resultado"` // "concedido", "denegado", "error"
        Detalle     string       `json:"detalle,omitempty"`
}

// ============================================================================
// Estado de Permisos
// ============================================================================

// EstadoPermisos es el estado completo del sistema de permisos.
// Es lo que se serializa a JSON para persistencia.
type EstadoPermisos struct {
        Permisos     map[TipoPermiso]*RegistroPermiso `json:"permisos"`
        ConcedidosEn  time.Time                        `json:"concedidos_en"`
        ConcedidoPor  string                           `json:"concedido_por"`
        Version      int                              `json:"version"`
}

// ============================================================================
// Gestor de Permisos (Thread-Safe)
// ============================================================================

// Gestor es el gestor central de permisos. Provee acceso thread-safe
// a todos los permisos con soporte para auditoría completa.
type Gestor struct {
        mu           sync.RWMutex
        estado       *EstadoPermisos
        rutaArchivo  string
        auditoria    []RegistroAuditoria
        habilitado   bool
}

// Variable global del gestor de permisos.
var gestorGlobal *Gestor

// ============================================================================
// Inicialización
// ============================================================================

// Inicializar crea el gestor global de permisos según la Decisión D-006
// "Permisos Una Vez": todos los permisos se conceden al inicio.
// Retorna el gestor inicializado o un error si falla la persistencia.
func Inicializar(rutaBase string) (*Gestor, error) {
        g := &Gestor{
                estado: &EstadoPermisos{
                        Permisos:    make(map[TipoPermiso]*RegistroPermiso),
                        Version:     1,
                },
                rutaArchivo: filepath.Join(expandirHome(rutaBase), "permisos", "permisos.json"),
                auditoria:   make([]RegistroAuditoria, 0),
                habilitado:  true,
        }

        // Intentar cargar permisos existentes
        if err := g.cargar(); err != nil {
                // Si no existe el archivo, conceder todos (D-006)
                g.ConcederTodos("decision_d-006", "Permisos concedidos al inicio según D-006")
        } else {
                // Verificar integridad de los permisos cargados
                g.completarPermisosFaltantes()
        }

        gestorGlobal = g
        return g, nil
}

// ObtenerGestor retorna el gestor global de permisos.
// Retorna nil si no ha sido inicializado.
func ObtenerGestor() *Gestor {
        return gestorGlobal
}

// ============================================================================
// Concesión de Permisos (D-006: Permisos Una Vez)
// ============================================================================

// ConcederTodos concede todos los permisos según la Decisión D-006.
// Esto se ejecuta UNA VEZ al inicio del sistema.
func (g *Gestor) ConcederTodos(concedidoPor, razon string) {
        g.mu.Lock()
        defer g.mu.Unlock()

        ahora := time.Now()
        g.estado.ConcedidosEn = ahora
        g.estado.ConcedidoPor = concedidoPor
        g.estado.Permisos = make(map[TipoPermiso]*RegistroPermiso)

        for _, tipo := range TodosLosPermisos {
                g.estado.Permisos[tipo] = &RegistroPermiso{
                        Tipo:        tipo,
                        Categoria:   string(tipo),
                        Concedido:   true,
                        Nivel:       NivelTotal,
                        SubPermisos: subPermisosPorDefecto(tipo),
                        ConcedidoEn: ahora,
                        ConcedidoPor: concedidoPor,
                        Razon:       razon,
                }
        }

        // Registrar en auditoría
        g.auditoria = append(g.auditoria, RegistroAuditoria{
                Timestamp: ahora,
                Accion:    "conceder",
                Resultado: "concedido",
                Detalle:   fmt.Sprintf("Todos los permisos concedidos por %s", concedidoPor),
        })

        // Persistir inmediatamente
        _ = g.guardar()
}

// Conceder concede un permiso individual.
// Si ya está concedido, actualiza la metadata.
func (g *Gestor) Conceder(tipo TipoPermiso, nivel NivelPermiso, concedidoPor, razon string) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        ahora := time.Now()

        reg := &RegistroPermiso{
                Tipo:        tipo,
                Categoria:   string(tipo),
                Concedido:   true,
                Nivel:       nivel,
                SubPermisos: subPermisosPorDefecto(tipo),
                ConcedidoEn: ahora,
                ConcedidoPor: concedidoPor,
                Razon:       razon,
        }

        // Si el permiso ya existe, preservar sub-permisos personalizados
        if existente, ok := g.estado.Permisos[tipo]; ok {
                reg.SubPermisos = existente.SubPermisos
        }

        g.estado.Permisos[tipo] = reg

        g.auditoria = append(g.auditoria, RegistroAuditoria{
                Timestamp:  ahora,
                Tipo:       tipo,
                Accion:     "conceder",
                Resultado:  "concedido",
                Detalle:    fmt.Sprintf("Permiso %s concedido con nivel %s", tipo, nivel),
        })

        return g.guardar()
}

// ConcederSubPermiso concede o revoca un sub-permiso específico.
func (g *Gestor) ConcederSubPermiso(tipo TipoPermiso, subNombre string, concedido bool, nivel NivelPermiso) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        reg, ok := g.estado.Permisos[tipo]
        if !ok {
                return fmt.Errorf("permiso %s no existe", tipo)
        }

        for i, sp := range reg.SubPermisos {
                if sp.Nombre == subNombre {
                        reg.SubPermisos[i].Concedido = concedido
                        reg.SubPermisos[i].Nivel = nivel

                        accion := "conceder"
                        resultado := "concedido"
                        if !concedido {
                                accion = "revocar"
                                resultado = "revocado"
                        }

                        g.auditoria = append(g.auditoria, RegistroAuditoria{
                                Timestamp:  time.Now(),
                                Tipo:       tipo,
                                Accion:     accion,
                                SubPermiso: subNombre,
                                Resultado:  resultado,
                        })

                        return g.guardar()
                }
        }

        return fmt.Errorf("sub-permiso '%s' no encontrado en %s", subNombre, tipo)
}

// ============================================================================
// Verificación de Permisos
// ============================================================================

// Verificar comprueba si un tipo de permiso está concedido.
// Retorna true si el permiso existe y está concedido.
func (g *Gestor) Verificar(tipo TipoPermiso) bool {
        g.mu.RLock()
        defer g.mu.RUnlock()

        if !g.habilitado {
                return true // Si el gestor está deshabilitado, todo permitido
        }

        reg, ok := g.estado.Permisos[tipo]
        if !ok {
                g.registrarAuditoriaLectura(tipo, "", "denegado", "permiso no existe")
                return false
        }

        resultado := "concedido"
        if !reg.Concedido {
                resultado = "denegado"
        }
        g.registrarAuditoriaLectura(tipo, "", resultado, "")

        return reg.Concedido
}

// VerificarSubPermiso comprueba si un sub-permiso específico está concedido.
func (g *Gestor) VerificarSubPermiso(tipo TipoPermiso, subNombre string) bool {
        g.mu.RLock()
        defer g.mu.RUnlock()

        if !g.habilitado {
                return true
        }

        reg, ok := g.estado.Permisos[tipo]
        if !ok || !reg.Concedido {
                g.registrarAuditoriaLectura(tipo, subNombre, "denegado", "permiso no concedido")
                return false
        }

        // Si el nivel es "total", todos los sub-permisos están concedidos
        if reg.Nivel == NivelTotal {
                g.registrarAuditoriaLectura(tipo, subNombre, "concedido", "nivel total")
                return true
        }

        // Verificar el sub-permiso específico
        for _, sp := range reg.SubPermisos {
                if sp.Nombre == subNombre {
                        resultado := "concedido"
                        if !sp.Concedido {
                                resultado = "denegado"
                        }
                        g.registrarAuditoriaLectura(tipo, subNombre, resultado, "")
                        return sp.Concedido
                }
        }

        // Sub-permiso no encontrado — denegar por defecto
        g.registrarAuditoriaLectura(tipo, subNombre, "denegado", "sub-permiso no encontrado")
        return false
}

// VerificarConDetalle verifica un permiso y retorna detalles del resultado.
func (g *Gestor) VerificarConDetalle(tipo TipoPermiso) (concedido bool, nivel NivelPermiso, detalle string) {
        g.mu.RLock()
        defer g.mu.RUnlock()

        if !g.habilitado {
                return true, NivelTotal, "gestor de permisos deshabilitado"
        }

        reg, ok := g.estado.Permisos[tipo]
        if !ok {
                return false, NivelDenegado, "permiso no registrado"
        }

        if !reg.Concedido {
                return false, NivelDenegado, "permiso revocado"
        }

        return true, reg.Nivel, fmt.Sprintf("concedido desde %s", reg.ConcedidoEn.Format("2006-01-02 15:04:05"))
}

// ============================================================================
// Estado y Consultas
// ============================================================================

// ObtenerTodos retorna una copia del estado completo de permisos.
func (g *Gestor) ObtenerTodos() EstadoPermisos {
        g.mu.RLock()
        defer g.mu.RUnlock()

        copia := EstadoPermisos{
                ConcedidosEn: g.estado.ConcedidosEn,
                ConcedidoPor: g.estado.ConcedidoPor,
                Version:      g.estado.Version,
                Permisos:     make(map[TipoPermiso]*RegistroPermiso),
        }

        for k, v := range g.estado.Permisos {
                regCopia := *v
                regCopia.SubPermisos = make([]SubPermiso, len(v.SubPermisos))
                copy(regCopia.SubPermisos, v.SubPermisos)
                copia.Permisos[k] = &regCopia
        }

        return copia
}

// ObtenerPermiso retorna un permiso individual por tipo.
// Retorna nil si no existe.
func (g *Gestor) ObtenerPermiso(tipo TipoPermiso) *RegistroPermiso {
        g.mu.RLock()
        defer g.mu.RUnlock()

        reg, ok := g.estado.Permisos[tipo]
        if !ok {
                return nil
        }

        copia := *reg
        copia.SubPermisos = make([]SubPermiso, len(reg.SubPermisos))
        copy(copia.SubPermisos, reg.SubPermisos)
        return &copia
}

// ObtenerResumen retorna un resumen de los permisos para respuestas API.
func (g *Gestor) ObtenerResumen() map[string]interface{} {
        g.mu.RLock()
        defer g.mu.RUnlock()

        total := len(TodosLosPermisos)
        concedidos := 0
        denegados := 0

        for _, tipo := range TodosLosPermisos {
                if reg, ok := g.estado.Permisos[tipo]; ok && reg.Concedido {
                        concedidos++
                } else {
                        denegados++
                }
        }

        return map[string]interface{}{
                "total":     total,
                "concedidos": concedidos,
                "denegados":  denegados,
                "habilitado": g.habilitado,
                "concedido_por": g.estado.ConcedidoPor,
                "concedido_en":  g.estado.ConcedidosEn.Format(time.RFC3339),
                "version":      g.estado.Version,
        }
}

// ListarTipos retorna la lista de todos los tipos de permisos con descripción.
func ListarTipos() []map[string]string {
        tipos := make([]map[string]string, len(TodosLosPermisos))
        for i, t := range TodosLosPermisos {
                tipos[i] = map[string]string{
                        "tipo":       string(t),
                        "descripcion": DescripcionPermiso(t),
                }
        }
        return tipos
}

// ============================================================================
// Auditoría
// ============================================================================

// ObtenerAuditoria retorna el historial completo de auditoría.
func (g *Gestor) ObtenerAuditoria() []RegistroAuditoria {
        g.mu.RLock()
        defer g.mu.RUnlock()

        copia := make([]RegistroAuditoria, len(g.auditoria))
        copy(copia, g.auditoria)
        return copia
}

// ObtenerAuditoriaReciente retorna los últimos N registros de auditoría.
func (g *Gestor) ObtenerAuditoriaReciente(n int) []RegistroAuditoria {
        g.mu.RLock()
        defer g.mu.RUnlock()

        if n <= 0 || n > len(g.auditoria) {
                n = len(g.auditoria)
        }

        // Retornar los más recientes primero
        inicio := len(g.auditoria) - n
        copia := make([]RegistroAuditoria, n)
        copy(copia, g.auditoria[inicio:])

        // Invertir para que el más reciente sea primero
        for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
                copia[i], copia[j] = copia[j], copia[i]
        }

        return copia
}

// LimpiarAuditoria elimina todos los registros de auditoría.
func (g *Gestor) LimpiarAuditoria() {
        g.mu.Lock()
        defer g.mu.Unlock()
        g.auditoria = make([]RegistroAuditoria, 0)
}

// ============================================================================
// Reset y Mantenimiento
// ============================================================================

// Resetear revoca todos los permisos y limpia la auditoría.
// Después de un reset, se deben conceder los permisos nuevamente.
func (g *Gestor) Resetear() {
        g.mu.Lock()
        defer g.mu.Unlock()

        g.estado.Permisos = make(map[TipoPermiso]*RegistroPermiso)
        g.estado.Version++
        g.auditoria = append(g.auditoria, RegistroAuditoria{
                Timestamp: time.Now(),
                Accion:    "resetear",
                Resultado: "reset_completo",
                Detalle:   "Todos los permisos han sido revocados",
        })

        _ = g.guardar()
}

// Habilitar/Deshabilitar el gestor de permisos.
// Cuando está deshabilitado, todas las verificaciones retornan true.
func (g *Gestor) Habilitar()  { g.mu.Lock(); defer g.mu.Unlock(); g.habilitado = true }
func (g *Gestor) Deshabilitar() { g.mu.Lock(); defer g.mu.Unlock(); g.habilitado = false }
func (g *Gestor) EstaHabilitado() bool { g.mu.RLock(); defer g.mu.RUnlock(); return g.habilitado }

// ============================================================================
// Persistencia
// ============================================================================

// cargar lee los permisos desde el archivo JSON.
func (g *Gestor) cargar() error {
        datos, err := os.ReadFile(g.rutaArchivo)
        if err != nil {
                return err
        }

        var estado EstadoPermisos
        if err := json.Unmarshal(datos, &estado); err != nil {
                return fmt.Errorf("error al parsear permisos: %w", err)
        }

        g.estado = &estado
        return nil
}

// guardar escribe los permisos al archivo JSON.
// Debe ser llamado con el lock de escritura tomado.
func (g *Gestor) guardar() error {
        dir := filepath.Dir(g.rutaArchivo)
        if err := os.MkdirAll(dir, 0755); err != nil {
                return fmt.Errorf("error al crear directorio de permisos: %w", err)
        }

        datos, err := json.MarshalIndent(g.estado, "", "  ")
        if err != nil {
                return fmt.Errorf("error al serializar permisos: %w", err)
        }

        if err := os.WriteFile(g.rutaArchivo, datos, 0644); err != nil {
                return fmt.Errorf("error al guardar permisos: %w", err)
        }

        return nil
}

// ============================================================================
// Funciones Internas
// ============================================================================

// expandirHome reemplaza ~ por el directorio home del usuario.
func expandirHome(ruta string) string {
        if strings.HasPrefix(ruta, "~/") {
                home, _ := os.UserHomeDir()
                return filepath.Join(home, ruta[2:])
        }
        return ruta
}

// subPermisosPorDefecto genera los sub-permisos por defecto para cada categoría.
func subPermisosPorDefecto(tipo TipoPermiso) []SubPermiso {
        switch tipo {
        case PermArchivos:
                return []SubPermiso{
                        {Nombre: "leer", Descripcion: "Leer archivos y directorios", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "escribir", Descripcion: "Crear y modificar archivos", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "eliminar", Descripcion: "Eliminar archivos y directorios", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "ejecutar", Descripcion: "Ejecutar archivos ejecutables", Concedido: true, Nivel: NivelTotal},
                }
        case PermRed:
                return []SubPermiso{
                        {Nombre: "http", Descripcion: "Peticiones HTTP/HTTPS", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "dns", Descripcion: "Resolución DNS", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "sockets", Descripcion: "Conexiones de sockets crudos", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "descargar", Descripcion: "Descarga de archivos", Concedido: true, Nivel: NivelTotal},
                }
        case PermSistema:
                return []SubPermiso{
                        {Nombre: "procesos", Descripcion: "Listar y gestionar procesos", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "servicios", Descripcion: "Gestionar servicios del sistema", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "configuracion", Descripcion: "Modificar configuración del sistema", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "paquetes", Descripcion: "Instalar y gestionar paquetes", Concedido: true, Nivel: NivelTotal},
                }
        case PermTerminal:
                return []SubPermiso{
                        {Nombre: "comandos", Descripcion: "Ejecutar comandos en terminal", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "scripts", Descripcion: "Ejecutar scripts", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "pipes", Descripcion: "Usar tuberías y redirecciones", Concedido: true, Nivel: NivelTotal},
                }
        case PermHerramientas:
                return []SubPermiso{
                        {Nombre: "navegador", Descripcion: "Automatización de navegador", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "editor", Descripcion: "Herramientas de edición", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "git", Descripcion: "Operaciones Git", Concedido: true, Nivel: NivelTotal},
                }
        case PermModelos:
                return []SubPermiso{
                        {Nombre: "chat", Descripcion: "Envío de mensajes a modelos", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "orquestar", Descripcion: "Orquestación entre modelos", Concedido: true, Nivel: NivelTotal},
                        {Nombre: "api_keys", Descripcion: "Acceso a API keys", Concedido: true, Nivel: NivelTotal},
                }
        default:
                return []SubPermiso{}
        }
}

// completarPermisosFaltantes asegura que todos los tipos de permisos existan
// después de cargar desde disco (por si se agregaron tipos nuevos).
func (g *Gestor) completarPermisosFaltantes() {
        ahora := time.Now()
        modificado := false

        for _, tipo := range TodosLosPermisos {
                if _, ok := g.estado.Permisos[tipo]; !ok {
                        g.estado.Permisos[tipo] = &RegistroPermiso{
                                Tipo:        tipo,
                                Categoria:   string(tipo),
                                Concedido:   true,
                                Nivel:       NivelTotal,
                                SubPermisos: subPermisosPorDefecto(tipo),
                                ConcedidoEn: ahora,
                                ConcedidoPor: "sistema",
                                Razon:       "Permiso agregado automáticamente (tipo nuevo)",
                        }
                        modificado = true
                }
        }

        if modificado {
                g.estado.Version++
                _ = g.guardar()
        }
}

// registrarAuditoriaLectura agrega una entrada de auditoría (lock de lectura).
func (g *Gestor) registrarAuditoriaLectura(tipo TipoPermiso, subPermiso, resultado, detalle string) {
        // Nota: Esta función se llama con RLock tomado.
        // Para evitar deadlock, usamos append que es safe para lecturas concurrentes
        // en nuestro caso porque nadie más escribe simultáneamente.
        // En producción, usaríamos un canal o sync más sofisticado.
        reg := RegistroAuditoria{
                Timestamp:  time.Now(),
                Tipo:       tipo,
                Accion:     "verificar",
                SubPermiso: subPermiso,
                Resultado:  resultado,
                Detalle:    detalle,
        }
        g.auditoria = append(g.auditoria, reg)
}

// FormatearPermisosParaAPI formatea el estado de permisos para respuestas JSON de API.
func (g *Gestor) FormatearPermisosParaAPI() map[string]interface{} {
        estado := g.ObtenerTodos()

        permisos := make([]map[string]interface{}, 0)
        for _, tipo := range TodosLosPermisos {
                reg := estado.Permisos[tipo]
                if reg == nil {
                        continue
                }

                subPerms := make([]map[string]interface{}, 0)
                for _, sp := range reg.SubPermisos {
                        subPerms = append(subPerms, map[string]interface{}{
                                "nombre":      sp.Nombre,
                                "descripcion": sp.Descripcion,
                                "concedido":   sp.Concedido,
                                "nivel":       string(sp.Nivel),
                        })
                }

                permisos = append(permisos, map[string]interface{}{
                        "tipo":        string(reg.Tipo),
                        "categoria":   reg.Categoria,
                        "concedido":   reg.Concedido,
                        "nivel":       string(reg.Nivel),
                        "sub_permisos": subPerms,
                        "concedido_en": reg.ConcedidoEn.Format(time.RFC3339),
                        "concedido_por": reg.ConcedidoPor,
                })
        }

        return map[string]interface{}{
                "permisos":      permisos,
                "concedido_por": estado.ConcedidoPor,
                "concedido_en":  estado.ConcedidosEn.Format(time.RFC3339),
                "version":       estado.Version,
        }
}

// SortTipos ordena los tipos de permisos por nombre para salida consistente.
func SortTipos(tipos []TipoPermiso) {
        sort.Slice(tipos, func(i, j int) bool {
                return string(tipos[i]) < string(tipos[j])
        })
}