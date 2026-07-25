package contexto

import (
        "encoding/json"
        "fmt"
        "os"
        "path/filepath"
        "sort"
        "strings"
        "sync"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/fragmentos"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/indice"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/mapa"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/resumen"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// EstadoProyecto representa el estado de indexación de un proyecto.
type EstadoProyecto struct {
        Nombre              string `json:"nombre"`
        Ruta                string `json:"ruta"`
        TotalArchivos       int    `json:"total_archivos"`
        TotalFragmentos     int    `json:"total_fragmentos"`
        UltimaIndexacion    string `json:"ultima_indexacion"`
        MapaGenerado        bool   `json:"mapa_generado"`
        IndiceGenerado      bool   `json:"indice_generado"`
}

// SolicitudIndexar es el body para solicitar la indexación de un proyecto.
type SolicitudIndexar struct {
        Ruta string `json:"ruta"`
}

// SolicitudFragmento es el body para solicitar un fragmento específico.
type SolicitudFragmento struct {
        Ruta     string `json:"ruta"`
        LineaIni int    `json:"linea_ini,omitempty"`
        LineaFin int    `json:"linea_fin,omitempty"`
}

// Coordinador es el punto de entrada del sistema de contexto.
// Coordina mapa, fragmentos, índice y resúmenes.
type Coordinador struct {
        mu           sync.RWMutex
        dirBase      string            // ~/.liz/contexto/proyectos/
        proyectos    map[string]*ProyectoContexto // nombre → proyecto
        logFunc      func(string, ...interface{})
}

// ProyectoContexto agrupa los componentes de contexto para un proyecto.
type ProyectoContexto struct {
        Nombre     string
        Ruta       string // ruta absoluta al proyecto
        GenMapa    *mapa.Generador
        Almacen    *fragmentos.Almacen
        Indice     *indice.GestorIndice
        GenResumen *resumen.Generador
}

// NuevoCoordinador crea un nuevo coordinador de contexto.
func NuevoCoordinador(dirBase string) (*Coordinador, error) {
        if err := os.MkdirAll(dirBase, 0755); err != nil {
                return nil, fmt.Errorf("error creando directorio base: %w", err)
        }

        c := &Coordinador{
                dirBase:   dirBase,
                proyectos: make(map[string]*ProyectoContexto),
                logFunc:   func(string, ...interface{}) {},
        }

        // Cargar proyectos existentes
        c.cargarProyectos()

        return c, nil
}

// ConLog asigna función de log.
func (c *Coordinador) ConLog(fn func(string, ...interface{})) *Coordinador {
        if fn != nil {
                c.logFunc = fn
        }
        return c
}

// ═══════════════════════════════════════════════════════
// GESTIÓN DE PROYECTOS
// ═══════════════════════════════════════════════════════

// IndexarProyecto indexa completamente un proyecto: genera mapa, fragmenta, indexa.
func (c *Coordinador) IndexarProyecto(rutaProyecto string) (*EstadoProyecto, error) {
        c.mu.Lock()
        defer c.mu.Unlock()

        rutaAbs, err := filepath.Abs(rutaProyecto)
        if err != nil {
                return nil, fmt.Errorf("error resolviendo ruta: %w", err)
        }

        nombre := filepath.Base(rutaAbs)

        // Crear o obtener proyecto
        proy, err := c.obtenerOCrearProyecto(nombre, rutaAbs)
        if err != nil {
                return nil, err
        }

        c.logFunc("indexando proyecto: %s (%s)", nombre, rutaAbs)

        // 1. Generar mapa
        mapaProy, err := proy.GenMapa.Generar(rutaAbs)
        if err != nil {
                return nil, fmt.Errorf("error generando mapa: %w", err)
        }

        // Guardar mapa en ~/.liz/contexto/proyectos/<nombre>/.liz/mapa.json
        rutaMapa := filepath.Join(c.dirBase, nombre, ".liz", "mapa.json")
        if err := proy.GenMapa.Guardar(mapaProy, rutaMapa); err != nil {
                c.logFunc("advertencia: error guardando mapa: %v", err)
        }

        // 2. Detectar archivos modificados ANTES de reconstruir el índice.
        // (Bug histórico: si Reconstruir se ejecuta primero, actualiza los hashes
        // al valor actual del disco, y ArchivosModificados siempre retorna vacío.)
        archivosModificados, _ := proy.Indice.ArchivosModificados(rutaAbs)

        // 3. Reconstruir índice (refresca hashes, mtimes y tamaños)
        if err := proy.Indice.Reconstruir(rutaAbs); err != nil {
                c.logFunc("advertencia: error reconstruyendo índice: %v", err)
        }

        // 4. Fragmentar archivos modificados (refresh incremental)
        for _, archRuta := range archivosModificados {
                rutaCompleta := filepath.Join(rutaAbs, archRuta)

                // Eliminar fragmentos viejos de este archivo antes de re-fragmentar
                _, _ = proy.Almacen.EliminarPorRuta(archRuta)

                ids, err := proy.Almacen.AgregarDesdeArchivo(archRuta, rutaCompleta)
                if err != nil {
                        c.logFunc("advertencia: error fragmentando %s: %v", archRuta, err)
                        continue
                }
                // Asignar fragmentos al índice
                if len(ids) > 0 {
                        proy.Indice.AsignarFragmentos(archRuta, ids)
                }
        }

        // 5. Primera indexación: si no había archivos modificados (índice vacío)
        //    hacer fragmentación completa.
        if len(archivosModificados) == 0 {
                err = filepath.WalkDir(rutaAbs, func(ruta string, d os.DirEntry, walkErr error) error {
                        if walkErr != nil || d.IsDir() {
                                return nil
                        }

                        relativa, _ := filepath.Rel(rutaAbs, ruta)

                        // Coherencia con mapa: respetar exclusiones de OpcionesMapa.
                        // Los archivos ocultos normales (.gitignore, .env.example) SÍ se
                        // fragmentan; solo se excluyen los directorios ocultos (.git/).
                        if d.IsDir() {
                                return nil
                        }
                        // Directorios padre ocultos (e.g. ".git/HEAD") se excluyen
                        if strings.HasPrefix(relativa, ".git/") || strings.HasPrefix(relativa, ".svn/") ||
                                strings.HasPrefix(relativa, ".hg/") || relativa == ".git" {
                                return nil
                        }

                        // Verificar si ya tiene fragmentos
                        if len(proy.Indice.ObtenerFragmentosIDs(relativa)) > 0 {
                                return nil
                        }

                        ids, err := proy.Almacen.AgregarDesdeArchivo(relativa, ruta)
                        if err != nil {
                                return nil
                        }
                        if len(ids) > 0 {
                                proy.Indice.AsignarFragmentos(relativa, ids)
                        }
                        return nil
                })
                if err != nil {
                        c.logFunc("advertencia: error fragmentando: %v", err)
                }
        }

        // 6. Actualizar índice global
        indiceGlobal := proy.Indice.Obtener()

        estado := &EstadoProyecto{
                Nombre:           nombre,
                Ruta:             rutaAbs,
                TotalArchivos:    indiceGlobal.TotalArchivos,
                TotalFragmentos:  indiceGlobal.TotalFragmentos,
                UltimaIndexacion: time.Now().UTC().Format(time.RFC3339),
                MapaGenerado:     true,
                IndiceGenerado:   true,
        }

        // Guardar estado del proyecto
        c.guardarEstadoProyecto(nombre, estado)

        c.logFunc("proyecto indexado: %s (%d archivos, %d fragmentos)",
                nombre, estado.TotalArchivos, estado.TotalFragmentos)

        return estado, nil
}

// ═══════════════════════════════════════════════════════
// CONSULTAS
// ═══════════════════════════════════════════════════════

// ObtenerMapa retorna el mapa de un proyecto.
func (c *Coordinador) ObtenerMapa(nombreProyecto string) (*mapa.MapaProyecto, error) {
        c.mu.RLock()
        defer c.mu.RUnlock()

        ruta := filepath.Join(c.dirBase, nombreProyecto, ".liz", "mapa.json")
        return mapa.Cargar(ruta)
}

// ObtenerIndice retorna el índice de un proyecto.
func (c *Coordinador) ObtenerIndice(nombreProyecto string) (*indice.IndiceProyecto, error) {
        c.mu.RLock()
        defer c.mu.RUnlock()

        proy, existe := c.proyectos[nombreProyecto]
        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }
        return proy.Indice.Obtener(), nil
}

// ObtenerFragmento retorna un fragmento por ID de un proyecto.
func (c *Coordinador) ObtenerFragmento(nombreProyecto, fragmentoID string) (*fragmentos.Fragmento, error) {
        c.mu.RLock()
        defer c.mu.RUnlock()

        proy, existe := c.proyectos[nombreProyecto]
        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }
        return proy.Almacen.Obtener(fragmentoID)
}

// ObtenerFragmentosPorRuta retorna todos los fragmentos de un archivo.
func (c *Coordinador) ObtenerFragmentosPorRuta(nombreProyecto, rutaArchivo string) ([]fragmentos.Fragmento, error) {
        c.mu.RLock()
        defer c.mu.RUnlock()

        proy, existe := c.proyectos[nombreProyecto]
        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }
        return proy.Almacen.ObtenerPorRuta(rutaArchivo)
}

// BuscarEnIndice busca en el índice de un proyecto.
func (c *Coordinador) BuscarEnIndice(nombreProyecto, patron string) []indice.EntradaIndice {
        c.mu.RLock()
        defer c.mu.RUnlock()

        proy, existe := c.proyectos[nombreProyecto]
        if !existe {
                return nil
        }
        return proy.Indice.Buscar(patron)
}

// ListarProyectos retorna todos los proyectos indexados.
func (c *Coordinador) ListarProyectos() []EstadoProyecto {
        c.mu.RLock()
        defer c.mu.RUnlock()

        var resultado []EstadoProyecto
        for nombre := range c.proyectos {
                if estado := c.cargarEstadoProyecto(nombre); estado != nil {
                        resultado = append(resultado, *estado)
                }
        }

        sort.Slice(resultado, func(i, j int) bool {
                return resultado[i].Nombre < resultado[j].Nombre
        })

        return resultado
}

// ObtenerResumen genera un resumen detallado de un archivo en un proyecto.
func (c *Coordinador) ObtenerResumen(nombreProyecto, rutaRelativa, rutaAbsoluta string) (*resumen.ResumenArchivo, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }

        return proy.GenResumen.Generar(rutaRelativa, rutaAbsoluta)
}

// ═══════════════════════════════════════════════════════
// PROYECTOS INTERNOS
// ═══════════════════════════════════════════════════════

// obtenerOCrearProyecto obtiene o crea el contexto de un proyecto.
func (c *Coordinador) obtenerOCrearProyecto(nombre, ruta string) (*ProyectoContexto, error) {
        if proy, existe := c.proyectos[nombre]; existe {
                return proy, nil
        }

        // Crear almacén de fragmentos
        almacen, err := fragmentos.NuevoAlmacen(c.dirBase, nombre)
        if err != nil {
                return nil, err
        }

        // Crear gestor de índice
        rutaIndice := filepath.Join(c.dirBase, nombre, ".liz", "indice.json")
        gestorIndice, err := indice.NuevoGestor(rutaIndice)
        if err != nil {
                return nil, err
        }

        proy := &ProyectoContexto{
                Nombre:     nombre,
                Ruta:       ruta,
                GenMapa:    mapa.NuevoGenerador().ConLog(c.logFunc),
                Almacen:    almacen.ConLog(c.logFunc),
                Indice:     gestorIndice.ConLog(c.logFunc),
                GenResumen: resumen.NuevoGenerador().ConLog(c.logFunc),
        }

        c.proyectos[nombre] = proy
        return proy, nil
}

// cargarProyectos carga los proyectos ya indexados desde el filesystem.
func (c *Coordinador) cargarProyectos() {
        entradas, err := os.ReadDir(c.dirBase)
        if err != nil {
                return
        }

        for _, entrada := range entradas {
                if !entrada.IsDir() {
                        continue
                }
                nombre := entrada.Name()

                // Verificar que tiene .liz/indice.json
                rutaIndice := filepath.Join(c.dirBase, nombre, ".liz", "indice.json")
                if _, err := os.Stat(rutaIndice); err != nil {
                        continue
                }

                // Cargar la ruta del proyecto del estado
                estado := c.cargarEstadoProyecto(nombre)
                if estado == nil {
                        continue
                }

                // Crear el contexto sin re-indexar
                almacen, err := fragmentos.NuevoAlmacen(c.dirBase, nombre)
                if err != nil {
                        continue
                }

                gestorIndice, err := indice.NuevoGestor(rutaIndice)
                if err != nil {
                        continue
                }

                c.proyectos[nombre] = &ProyectoContexto{
                        Nombre:     nombre,
                        Ruta:       estado.Ruta,
                        GenMapa:    mapa.NuevoGenerador().ConLog(c.logFunc),
                        Almacen:    almacen.ConLog(c.logFunc),
                        Indice:     gestorIndice.ConLog(c.logFunc),
                        GenResumen: resumen.NuevoGenerador().ConLog(c.logFunc),
                }

                c.logFunc("proyecto cargado desde caché: %s", nombre)
        }
}

// ═══════════════════════════════════════════════════════
// ESTADO DE PROYECTOS
// ═══════════════════════════════════════════════════════

// guardarEstadoProyecto guarda el estado de un proyecto.
func (c *Coordinador) guardarEstadoProyecto(nombre string, estado *EstadoProyecto) error {
        ruta := filepath.Join(c.dirBase, nombre, ".liz", "estado.json")
        if err := os.MkdirAll(filepath.Dir(ruta), 0755); err != nil {
                return err
        }

        datos, err := json.MarshalIndent(estado, "", "  ")
        if err != nil {
                return err
        }
        return os.WriteFile(ruta, datos, 0644)
}

// cargarEstadoProyecto carga el estado de un proyecto.
func (c *Coordinador) cargarEstadoProyecto(nombre string) *EstadoProyecto {
        ruta := filepath.Join(c.dirBase, nombre, ".liz", "estado.json")
        datos, err := os.ReadFile(ruta)
        if err != nil {
                return nil
        }

        var estado EstadoProyecto
        if json.Unmarshal(datos, &estado) != nil {
                return nil
        }
        return &estado
}
