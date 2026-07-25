package memoria

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

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// Hecho es una tripleta (sujeto, predicado, objeto) extraída del diálogo.
// Inspirado en Mem0: representación semántica de información del usuario.
//
// Ejemplos:
//   {Sujeto: "usuario", Predicado: "prefiere_lenguaje", Objeto: "Go"}
//   {Sujeto: "proyecto_liz", Predicado: "usar_api", Objeto: "NVIDIA"}
//   {Sujeto: "usuario", Predicado: "sistema_operativo", Objeto: "Arch Linux"}
type Hecho struct {
        ID           string  `json:"id"`            // uuid
        Sujeto       string  `json:"sujeto"`        // de quién/qué habla el hecho
        Predicado    string  `json:"predicado"`     // qué relación
        Objeto       string  `json:"objeto"`        // valor
        Confianza    float64 `json:"confianza"`     // [0.0, 1.0]
        Timestamp    string  `json:"timestamp"`     // RFC3339
        SesionOrigen string  `json:"sesion_origen"` // en qué sesión se extrajo
        Obsoleto     bool    `json:"obsoleto"`      // true si fue reemplazado
        Notas        string  `json:"notas,omitempty"`
}

// TipoHecho clasifica el tipo de información (estilo Mem0 taxonomy).
type TipoHecho string

const (
        HechoPreferencia TipoHecho = "preferencia"  // "usuario prefiere Go"
        HechoProyecto    TipoHecho = "proyecto"     // "proyecto X usa API Y"
        HechoContexto    TipoHecho = "contexto"     // "usuario trabaja en empresa Z"
        HechoTecnico     TipoHecho = "tecnico"      // "configuración de PostgreSQL"
        HechoTemporal    TipoHecho = "temporal"     // "usuario tiene deadline el viernes"
)

// EstadisticasHechos resume el almacén de hechos de un usuario.
type EstadisticasHechos struct {
        TotalHechos     int            `json:"total_hechos"`
        HechosActivos   int            `json:"hechos_activos"`
        HechosObsoletos int            `json:"hechos_obsoletos"`
        PorTipo         map[string]int `json:"por_tipo"`
}

// ═══════════════════════════════════════════════════════
// GESTOR DE HECHOS
// ═══════════════════════════════════════════════════════

// GestorHechos gestiona los hechos extraídos del diálogo de cada usuario.
// Persistencia: ~/.liz/memoria/hechos/<usuario>.json
type GestorHechos struct {
        mu       sync.RWMutex
        dirHechos string // ~/.liz/memoria/hechos/
        logFunc  func(string, ...interface{})

        // Cache en memoria: usuarioID → lista de hechos
        cache map[string]*AlmacenHechos
}

// AlmacenHechos es la estructura persistida por usuario.
type AlmacenHechos struct {
        UsuarioID string  `json:"usuario_id"`
        Hechos    []Hecho `json:"hechos"`
        Actualizado string `json:"actualizado"` // RFC3339
}

// NuevoGestorHechos crea un nuevo gestor de hechos.
func NuevoGestorHechos(dirBase string) (*GestorHechos, error) {
        dir := filepath.Join(dirBase, "memoria", "hechos")
        if err := os.MkdirAll(dir, 0755); err != nil {
                return nil, fmt.Errorf("error creando directorio de hechos: %w", err)
        }
        g := &GestorHechos{
                dirHechos: dir,
                cache:     make(map[string]*AlmacenHechos),
                logFunc:   func(string, ...interface{}) {},
        }
        return g, nil
}

// ConLog asigna función de log.
func (g *GestorHechos) ConLog(fn func(string, ...interface{})) *GestorHechos {
        if fn != nil {
                g.logFunc = fn
        }
        return g
}

// ═══════════════════════════════════════════════════════
// OPERACIONES DE HECHOS
// ═══════════════════════════════════════════════════════

// AgregarHecho agrega un hecho al almacén del usuario.
// RESOLUCIÓN DE CONFLICTOS: si ya existe un hecho con el mismo (sujeto, predicado),
// el hecho viejo se marca como Obsoleto=true y el nuevo lo reemplaza.
// (Inspirado en Mem0.)
func (g *GestorHechos) AgregarHecho(usuarioID, sujeto, predicado, objeto string, confianza float64, sesionOrigen string) (*Hecho, error) {
        if sujeto == "" || predicado == "" || objeto == "" {
                return nil, fmt.Errorf("sujeto, predicado y objeto son obligatorios")
        }
        if confianza < 0 {
                confianza = 0
        } else if confianza > 1 {
                confianza = 1
        }

        g.mu.Lock()
        defer g.mu.Unlock()

        almacen := g.obtenerOCrearAlmacen(usuarioID)

        // Resolver conflictos: marcar hechos viejos con mismo (sujeto, predicado) como obsoletos
        for i := range almacen.Hechos {
                h := &almacen.Hechos[i]
                if h.Obsoleto {
                        continue
                }
                if strings.EqualFold(h.Sujeto, sujeto) && strings.EqualFold(h.Predicado, predicado) {
                        if !strings.EqualFold(h.Objeto, objeto) {
                                h.Obsoleto = true
                                h.Notas = fmt.Sprintf("reemplazado por hecho del %s", time.Now().UTC().Format(time.RFC3339))
                                g.logFunc("hecho obsoleto: %s/%s=%s (reemplazado por %s)",
                                        h.Sujeto, h.Predicado, h.Objeto, objeto)
                        } else {
                                // Mismo hecho: actualizar confianza y timestamp, no agregar duplicado
                                h.Confianza = (h.Confianza + confianza) / 2
                                h.Timestamp = time.Now().UTC().Format(time.RFC3339)
                                if err := g.guardar(usuarioID, almacen); err != nil {
                                        return nil, err
                                }
                                return h, nil
                        }
                }
        }

        hecho := Hecho{
                ID:           generarUUID(),
                Sujeto:       sujeto,
                Predicado:    predicado,
                Objeto:       objeto,
                Confianza:    confianza,
                Timestamp:    time.Now().UTC().Format(time.RFC3339),
                SesionOrigen: sesionOrigen,
        }
        almacen.Hechos = append(almacen.Hechos, hecho)

        if err := g.guardar(usuarioID, almacen); err != nil {
                return nil, err
        }

        g.logFunc("hecho agregado: %s/%s=%s (confianza: %.2f)",
                sujeto, predicado, objeto, confianza)
        return &hecho, nil
}

// HechosActivos retorna todos los hechos no obsoletos de un usuario.
// Orden: por confianza descendente.
func (g *GestorHechos) HechosActivos(usuarioID string) ([]Hecho, error) {
        g.mu.RLock()
        defer g.mu.RUnlock()

        almacen, err := g.cargar(usuarioID)
        if err != nil {
                return nil, err
        }

        var resultado []Hecho
        for _, h := range almacen.Hechos {
                if !h.Obsoleto {
                        resultado = append(resultado, h)
                }
        }

        sort.Slice(resultado, func(i, j int) bool {
                return resultado[i].Confianza > resultado[j].Confianza
        })

        return resultado, nil
}

// BuscarHechos retorna hechos que coinciden con sujeto y/o predicado.
// Parámetros vacíos significan "cualquiera".
func (g *GestorHechos) BuscarHechos(usuarioID, sujeto, predicado string) ([]Hecho, error) {
        g.mu.RLock()
        defer g.mu.RUnlock()

        almacen, err := g.cargar(usuarioID)
        if err != nil {
                return nil, err
        }

        var resultado []Hecho
        for _, h := range almacen.Hechos {
                if h.Obsoleto {
                        continue
                }
                if sujeto != "" && !strings.EqualFold(h.Sujeto, sujeto) {
                        continue
                }
                if predicado != "" && !strings.EqualFold(h.Predicado, predicado) {
                        continue
                }
                resultado = append(resultado, h)
        }
        return resultado, nil
}

// EliminarHecho marca un hecho como obsoleto por ID.
func (g *GestorHechos) EliminarHecho(usuarioID, hechoID string) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        almacen, err := g.cargar(usuarioID)
        if err != nil {
                return err
        }

        for i := range almacen.Hechos {
                if almacen.Hechos[i].ID == hechoID {
                        almacen.Hechos[i].Obsoleto = true
                        almacen.Hechos[i].Notas = "eliminado manualmente"
                        return g.guardar(usuarioID, almacen)
                }
        }
        return fmt.Errorf("hecho %s no encontrado", hechoID)
}

// Estadisticas retorna métricas del almacén de un usuario.
func (g *GestorHechos) Estadisticas(usuarioID string) (EstadisticasHechos, error) {
        g.mu.RLock()
        defer g.mu.RUnlock()

        almacen, err := g.cargar(usuarioID)
        if err != nil {
                return EstadisticasHechos{}, err
        }

        stats := EstadisticasHechos{
                TotalHechos: len(almacen.Hechos),
                PorTipo:     make(map[string]int),
        }
        for _, h := range almacen.Hechos {
                if h.Obsoleto {
                        stats.HechosObsoletos++
                } else {
                        stats.HechosActivos++
                }
        }
        return stats, nil
}

// FormatoContexto retorna los hechos activos como string para inyectar en el prompt del LLM.
// Útil para dar contexto sobre el usuario al inicio de cada nueva sesión.
//
// Formato:
//
//      # Memoria del usuario
//      - usuario prefiere_lenguaje: Go (confianza: 0.95)
//      - proyecto_liz usar_api: NVIDIA (confianza: 0.90)
//      ...
//
// Retorna string vacío si el usuario no tiene hechos (incluyendo si nunca se ha
// almacenado nada para ese usuario — archivo no existe).
func (g *GestorHechos) FormatoContexto(usuarioID string, limite int) (string, error) {
        hechos, err := g.HechosActivos(usuarioID)
        if err != nil {
                // Si el archivo no existe, el usuario no tiene hechos → retornar vacío
                if os.IsNotExist(err) || strings.Contains(err.Error(), "no encontrado") {
                        return "", nil
                }
                return "", err
        }
        if limite > 0 && len(hechos) > limite {
                hechos = hechos[:limite]
        }

        if len(hechos) == 0 {
                return "", nil
        }

        var b strings.Builder
        b.WriteString("# Memoria del usuario\n")
        for _, h := range hechos {
                b.WriteString(fmt.Sprintf("- %s %s: %s (confianza: %.2f)\n",
                        h.Sujeto, h.Predicado, h.Objeto, h.Confianza))
        }
        return b.String(), nil
}

// ═══════════════════════════════════════════════════════
// PERSISTENCIA INTERNA
// ═══════════════════════════════════════════════════════

// obtenerOCrearAlmacen retorna el almacén del usuario (cache o carga desde disco).
// Sin lock: el llamador debe tener el lock.
func (g *GestorHechos) obtenerOCrearAlmacen(usuarioID string) *AlmacenHechos {
        if almacen, existe := g.cache[usuarioID]; existe {
                return almacen
        }
        almacen, err := g.cargar(usuarioID)
        if err != nil {
                // Crear vacío
                almacen = &AlmacenHechos{
                        UsuarioID: usuarioID,
                        Hechos:    []Hecho{},
                }
        }
        g.cache[usuarioID] = almacen
        return almacen
}

// cargar lee el almacén de un usuario desde disco. Sin lock.
func (g *GestorHechos) cargar(usuarioID string) (*AlmacenHechos, error) {
        // Si está en cache, retornar
        if almacen, existe := g.cache[usuarioID]; existe {
                return almacen, nil
        }

        path := filepath.Join(g.dirHechos, usuarioID+".json")
        data, err := os.ReadFile(path)
        if err != nil {
                return nil, fmt.Errorf("almacen de hechos de %s no encontrado: %w", usuarioID, err)
        }
        var almacen AlmacenHechos
        if err := json.Unmarshal(data, &almacen); err != nil {
                return nil, fmt.Errorf("error deserializando hechos: %w", err)
        }
        return &almacen, nil
}

// guardar persiste el almacén de un usuario a disco. Sin lock.
func (g *GestorHechos) guardar(usuarioID string, almacen *AlmacenHechos) error {
        almacen.Actualizado = time.Now().UTC().Format(time.RFC3339)
        almacen.UsuarioID = usuarioID

        path := filepath.Join(g.dirHechos, usuarioID+".json")
        data, err := json.MarshalIndent(almacen, "", "  ")
        if err != nil {
                return fmt.Errorf("error serializando hechos: %w", err)
        }
        tmp := path + ".tmp"
        if err := os.WriteFile(tmp, data, 0644); err != nil {
                return fmt.Errorf("error escribiendo hechos: %w", err)
        }
        return os.Rename(tmp, path)
}
