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

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/arbol_ast"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/empaquetador"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/fragmentos"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/grafo"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/indice"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/mapa"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/mapa_repo"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/resumen"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/tracker"
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
// Coordina mapa, fragmentos, índice, resúmenes, grafo, buscador,
// repository map y empaquetador (Fase 3.5: sistema world-class).
type Coordinador struct {
        mu        sync.RWMutex
        dirBase   string                   // ~/.liz/contexto/proyectos/
        proyectos map[string]*ProyectoContexto // nombre → proyecto
        logFunc   func(string, ...interface{})
        providerEmbeddings buscador.EmbeddingsProvider // opcional, para BM25+vector
}

// IBuscador define la interfaz de búsqueda usada por el coordinador.
// Tanto *buscador.Buscador como *buscador.BuscadorEmbeddings la implementan.
type IBuscador interface {
        Indexar(f buscador.FragmentoBuscable)
        Desindexar(id string)
        BuscarBM25(query string, topK int) []buscador.ResultadoBusqueda
        BuscarHibrido(query string, topK int) []buscador.ResultadoBusqueda
        Total() int
        Estadisticas() buscador.EstadisticasBuscador
}

// ProyectoContexto agrupa los componentes de contexto para un proyecto.
// En Fase 3.5 se añaden: Grafo, Buscador, GenMapaRepo, Empaquetador.
type ProyectoContexto struct {
        Nombre       string
        Ruta         string // ruta absoluta al proyecto
        Modulo       string // módulo Go del proyecto (ej. "github.com/foo/bar")
        GenMapa      *mapa.Generador
        Almacen      *fragmentos.Almacen
        Indice       *indice.GestorIndice
        GenResumen   *resumen.Generador
        Tracker      *tracker.TrackerEdiciones
        // Nuevos componentes Fase 3.5:
        Parser       *arbol_ast.Parser
        Grafo        *grafo.Grafo
        Buscador     IBuscador
        GenMapaRepo  *mapa_repo.Generador
        Empaquetador *empaquetador.Empaquetador
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

// ConProviderEmbeddings configura un provider de embeddings para búsqueda híbrida.
// Debe llamarse antes de indexar proyectos.
func (c *Coordinador) ConProviderEmbeddings(p buscador.EmbeddingsProvider) *Coordinador {
        c.providerEmbeddings = p
        return c
}

// TieneBusquedaHibrida retorna true si hay provider de embeddings configurado.
func (c *Coordinador) TieneBusquedaHibrida() bool {
        return c.providerEmbeddings != nil
}

// RegistrarEdicion registra una edición de archivo en el tracker del proyecto.
func (c *Coordinador) RegistrarEdicion(nombreProyecto, ruta string) {
        c.mu.RLock()
        proy, ok := c.proyectos[nombreProyecto]
        c.mu.RUnlock()
        if !ok || proy.Tracker == nil {
                return
        }
        proy.Tracker.RegistrarEdicion(ruta)
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

        // 7. Construir grafo de dependencias y calcular PageRank (Fase 3.5)
        c.construirGrafo(proy, rutaAbs, indiceGlobal)

        // 8. Indexar fragmentos en el buscador BM25 (Fase 3.5)
        c.indexarBuscador(proy)

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

        c.logFunc("proyecto indexado: %s (%d archivos, %d fragmentos, grafo: %d nodos, buscador: %d fragmentos)",
                nombre, estado.TotalArchivos, estado.TotalFragmentos,
                proy.Grafo.TotalArchivos(), proy.Buscador.Total())

        return estado, nil
}

// construirGrafo construye el grafo de dependencias del proyecto.
// Para cada archivo .go, parsea los imports y resuelve cuáles son internos
// al proyecto (mismo módulo Go). Luego calcula PageRank.
func (c *Coordinador) construirGrafo(proy *ProyectoContexto, rutaAbs string, ind *indice.IndiceProyecto) {
        // Limpiar grafo existente
        proy.Grafo = grafo.NuevoGrafo()

        // Agregar archivos al grafo
        for ruta, entrada := range ind.Archivos {
                proy.Grafo.AgregarArchivo(ruta, entrada.Lenguaje, entrada.Lineas)
        }

        // Para archivos Go, parsear imports y resolver dependencias
        for ruta, entrada := range ind.Archivos {
                if entrada.Lenguaje != "go" {
                        continue
                }
                rutaCompleta := filepath.Join(rutaAbs, ruta)
                ast, err := proy.Parser.Parsear(ruta, rutaCompleta)
                if err != nil || ast == nil {
                        continue
                }

                // Para cada import, resolver si es interno al proyecto
                for _, imp := range ast.Imports {
                        if proy.Modulo == "" {
                                continue
                        }
                        // Si el import empieza con el módulo del proyecto, es interno
                        if !strings.HasPrefix(imp, proy.Modulo) {
                                continue
                        }
                        // Resolver el import a una ruta de directorio dentro del proyecto
                        rutaDir := strings.TrimPrefix(imp, proy.Modulo)
                        rutaDir = strings.TrimPrefix(rutaDir, "/")
                        if rutaDir == "" {
                                continue
                        }
                        // Encontrar todos los archivos indexados en ese directorio
                        for otraRuta := range ind.Archivos {
                                if strings.HasPrefix(otraRuta, rutaDir+"/") {
                                        proy.Grafo.AgregarImport(ruta, otraRuta)
                                }
                        }
                }
        }

        // Calcular PageRank
        proy.Grafo.CalcularImportancia(50, 0.85)
        c.logFunc("grafo construido: %d archivos, %d aristas",
                proy.Grafo.TotalArchivos(), proy.Grafo.TotalAristas())
}

// indexarBuscador indexa todos los fragmentos en el buscador BM25 (o híbrido si hay provider).
// Si el buscador ya tenía datos, se limpia y se re-indexa desde cero.
func (c *Coordinador) indexarBuscador(proy *ProyectoContexto) {
        // Crear buscador: híbrido si hay provider, BM25 puro si no
        if c.providerEmbeddings != nil {
                proy.Buscador = buscador.NuevoBuscadorEmbeddings(c.providerEmbeddings)
        } else {
                proy.Buscador = buscador.NuevoBuscador()
        }

        // Listar todos los fragmentos del almacén (metadata solo)
        frags, err := proy.Almacen.Listar()
        if err != nil {
                c.logFunc("advertencia: error listando fragmentos para buscador: %v", err)
                return
        }

        for _, f := range frags {
                // Obtener fragmento completo (con contenido) por ID
                fragCompleto, err := proy.Almacen.Obtener(f.ID)
                if err != nil || fragCompleto == nil {
                        continue
                }
                proy.Buscador.Indexar(buscador.FragmentoBuscable{
                        ID:        fragCompleto.ID,
                        Ruta:      fragCompleto.Ruta,
                        Contenido: fragCompleto.Contenido,
                        Tipo:      fragCompleto.Tipo,
                        Lenguaje:  fragCompleto.Lenguaje,
                })
        }
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
// Si el resumen ya está en cache/disco, lo retorna sin regenerar.
// Para forzar regeneración, usar ForzarResumen.
func (c *Coordinador) ObtenerResumen(nombreProyecto, rutaRelativa, rutaAbsoluta string) (*resumen.ResumenArchivo, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }

        // Intentar cargar de cache/disco primero
        if cached, _ := proy.GenResumen.Cargar(rutaRelativa); cached != nil {
                return cached, nil
        }

        // Generar nuevo
        r, err := proy.GenResumen.Generar(rutaRelativa, rutaAbsoluta)
        if err != nil {
                return nil, err
        }

        // Persistir para futuras consultas
        _ = proy.GenResumen.Guardar(r)
        return r, nil
}

// ForzarResumen regenera el resumen sin usar cache.
func (c *Coordinador) ForzarResumen(nombreProyecto, rutaRelativa, rutaAbsoluta string) (*resumen.ResumenArchivo, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }

        r, err := proy.GenResumen.Generar(rutaRelativa, rutaAbsoluta)
        if err != nil {
                return nil, err
        }
        _ = proy.GenResumen.Guardar(r)
        return r, nil
}

// ObtenerArbol retorna la estructura jerárquica del índice de un proyecto.
func (c *Coordinador) ObtenerArbol(nombreProyecto string) (*indice.NodoArbol, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }
        return proy.Indice.Arbol(), nil
}

// EliminarProyecto elimina un proyecto del coordinador y borra todos sus
// datos del disco (~/.liz/contexto/proyectos/<nombre>/).
func (c *Coordinador) EliminarProyecto(nombreProyecto string) error {
        c.mu.Lock()
        defer c.mu.Unlock()

        proy, existe := c.proyectos[nombreProyecto]
        if !existe {
                return fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }

        // Borrar del disco
        rutaProyecto := filepath.Join(c.dirBase, nombreProyecto)
        if err := os.RemoveAll(rutaProyecto); err != nil {
                return fmt.Errorf("error eliminando directorio del proyecto: %w", err)
        }

        // Quitar del mapa
        delete(c.proyectos, nombreProyecto)

        c.logFunc("proyecto eliminado: %s (%s)", nombreProyecto, proy.Ruta)
        return nil
}

// ReindexarArchivo re-indexa un único archivo (en lugar de todo el proyecto).
// Útil cuando se sabe que un archivo específico cambió.
func (c *Coordinador) ReindexarArchivo(nombreProyecto, rutaRelativa string) error {
        c.mu.Lock()
        defer c.mu.Unlock()

        proy, existe := c.proyectos[nombreProyecto]
        if !existe {
                return fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }

        rutaAbsoluta := filepath.Join(proy.Ruta, rutaRelativa)

        // Verificar que el archivo existe
        if _, err := os.Stat(rutaAbsoluta); err != nil {
                // Archivo eliminado: limpiar del índice y eliminar fragmentos
                _, _ = proy.Almacen.EliminarPorRuta(rutaRelativa)
                _ = proy.GenResumen.Eliminar(rutaRelativa)
                // Marcar en el índice como eliminado (Reconstruir lo detectará después)
                return nil
        }

        // Eliminar fragmentos y resumen viejos
        _, _ = proy.Almacen.EliminarPorRuta(rutaRelativa)
        _ = proy.GenResumen.Eliminar(rutaRelativa)

        // Re-fragmentar
        ids, err := proy.Almacen.AgregarDesdeArchivo(rutaRelativa, rutaAbsoluta)
        if err != nil {
                return fmt.Errorf("error re-fragmentando %s: %w", rutaRelativa, err)
        }
        if len(ids) > 0 {
                if err := proy.Indice.AsignarFragmentos(rutaRelativa, ids); err != nil {
                        return fmt.Errorf("error asignando fragmentos: %w", err)
                }
        }

        // Regenerar resumen
        r, err := proy.GenResumen.Generar(rutaRelativa, rutaAbsoluta)
        if err == nil {
                _ = proy.GenResumen.Guardar(r)
        }

        c.logFunc("archivo re-indexado: %s/%s (%d fragmentos)", nombreProyecto, rutaRelativa, len(ids))
        return nil
}

// ═══════════════════════════════════════════════════════
// MÉTODOS FASE 3.5 (sistema world-class)
// ═══════════════════════════════════════════════════════

// ObtenerSimbolos retorna los símbolos parseados por AST de un archivo.
// Solo funciona con Go (otros lenguajes retornan AST vacío).
func (c *Coordinador) ObtenerSimbolos(nombreProyecto, rutaRelativa string) (*arbol_ast.ArchivoAST, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }

        rutaAbsoluta := filepath.Join(proy.Ruta, rutaRelativa)
        return proy.Parser.Parsear(rutaRelativa, rutaAbsoluta)
}

// ObtenerGrafo retorna el grafo de dependencias de un proyecto.
func (c *Coordinador) ObtenerGrafo(nombreProyecto string) (*grafo.Grafo, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }
        return proy.Grafo, nil
}

// ObtenerImportancias retorna un mapa ruta → score PageRank de un proyecto.
func (c *Coordinador) ObtenerImportancias(nombreProyecto string) (map[string]float64, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }
        return proy.Grafo.ImportanciasDe(), nil
}

// BuscarHibrido busca fragmentos por query usando BM25 + RRF.
// Retorna hasta topK resultados ordenados por score descendente.
func (c *Coordinador) BuscarHibrido(nombreProyecto, query string, topK int) ([]buscador.ResultadoBusqueda, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }
        if topK <= 0 {
                topK = 10
        }
        return proy.Buscador.BuscarHibrido(query, topK), nil
}

// ObtenerMapaRepo genera el repository map compacto (Aider-style).
// Solo incluye firmas de símbolos, ordenadas por importancia PageRank,
// limitado por presupuesto de tokens.
func (c *Coordinador) ObtenerMapaRepo(nombreProyecto string, presupuestoTokens int) (*mapa_repo.MapaRepo, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[nombreProyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", nombreProyecto)
        }

        if presupuestoTokens <= 0 {
                presupuestoTokens = 2000
        }

        // Construir lista de archivos con su ruta absoluta y score de importancia
        indice := proy.Indice.Obtener()
        rutasAbsolutas := make(map[string]string)
        for ruta := range indice.Archivos {
                rutasAbsolutas[ruta] = filepath.Join(proy.Ruta, ruta)
        }

        // Convertir nodos del grafo a ArchivoParaMapa
        archivos := mapa_repo.ArchivosDesdeGrafo(proy.Grafo, rutasAbsolutas)

        return proy.GenMapaRepo.Generar(nombreProyecto, archivos, presupuestoTokens), nil
}

// EmpaquetarContexto ensambla el contexto óptimo para un LLM.
// Es el método principal del sistema world-class: combina mapa repo +
// fragmentos relevantes + imports + archivos recientes en un solo string.
func (c *Coordinador) EmpaquetarContexto(req EmpaquetarSolicitud) (*empaquetador.ContextoEmpaquetado, error) {
        c.mu.RLock()
        proy, existe := c.proyectos[req.Proyecto]
        c.mu.RUnlock()

        if !existe {
                return nil, fmt.Errorf("proyecto %s no indexado", req.Proyecto)
        }

        // Generar mapa repo para el presupuesto adecuado (30% del total)
        presupuestoMapa := req.PresupuestoTokens * 30 / 100
        mapaRepo, err := c.ObtenerMapaRepo(req.Proyecto, presupuestoMapa)
        // Si falla, continuar sin mapa repo
        if err != nil || mapaRepo == nil {
                mapaRepo = &mapa_repo.MapaRepo{Proyecto: req.Proyecto}
        }

        // Callback para obtener fragmentos por ID
        obtenerFragmento := func(id string) (buscador.FragmentoBuscable, bool) {
                frag, err := proy.Almacen.Obtener(id)
                if err != nil || frag == nil {
                        return buscador.FragmentoBuscable{}, false
                }
                return buscador.FragmentoBuscable{
                        ID:        frag.ID,
                        Ruta:      frag.Ruta,
                        Contenido: frag.Contenido,
                        Tipo:      frag.Tipo,
                        Lenguaje:  frag.Lenguaje,
                }, true
        }

        // Callback para obtener fragmentos por ruta relativa (necesario para
        // capas 3 y 4 del empaquetador: imports expandidos y archivos recientes)
        obtenerFragmentosPorRuta := func(ruta string) []buscador.FragmentoBuscable {
                frags, err := proy.Almacen.ObtenerPorRuta(ruta)
                if err != nil {
                        return nil
                }
                resultado := make([]buscador.FragmentoBuscable, 0, len(frags))
                for _, f := range frags {
                        resultado = append(resultado, buscador.FragmentoBuscable{
                                ID:        f.ID,
                                Ruta:      f.Ruta,
                                Contenido: f.Contenido,
                                Tipo:      f.Tipo,
                                Lenguaje:  f.Lenguaje,
                        })
                }
                return resultado
        }

        datos := empaquetador.DatosEmpaquetado{
                MapaRepo:                 mapaRepo,
                Buscador:                 proy.Buscador,
                Grafo:                    proy.Grafo,
                ObtenerFragmento:         obtenerFragmento,
                ObtenerFragmentosPorRuta: obtenerFragmentosPorRuta,
        }

        solicitud := empaquetador.SolicitudEmpaquetado{
                Proyecto:          req.Proyecto,
                Query:             req.Query,
                PresupuestoTokens: req.PresupuestoTokens,
                ArchivosRecientes: req.ArchivosRecientes,
                ProfundidadImports: req.ProfundidadImports,
        }
        // Default: si el usuario no especificó profundidad, usar 1 (imports directos)
        if solicitud.ProfundidadImports == 0 {
                solicitud.ProfundidadImports = 1
        }

        return proy.Empaquetador.Empaquetar(solicitud, datos), nil
}

// EmpaquetarSolicitud es el body para EmpaquetarContexto.
type EmpaquetarSolicitud struct {
        Proyecto           string   `json:"proyecto"`
        Query              string   `json:"query"`
        PresupuestoTokens  int      `json:"presupuesto_tokens"`
        ArchivosRecientes  []string `json:"archivos_recientes"`
        ProfundidadImports int      `json:"profundidad_imports"`
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

        // Crear generador de resúmenes con directorio de persistencia
        dirResumenes := filepath.Join(c.dirBase, nombre, ".liz", "resumenes")

        // Detectar módulo Go del proyecto (para resolver imports internos)
        modulo := detectarModuloGo(ruta)

        proy := &ProyectoContexto{
                Nombre:       nombre,
                Ruta:         ruta,
                Modulo:       modulo,
                GenMapa:      mapa.NuevoGenerador().ConLog(c.logFunc),
                Almacen:      almacen.ConLog(c.logFunc),
                Indice:       gestorIndice.ConLog(c.logFunc),
                GenResumen:   resumen.NuevoGenerador().ConLog(c.logFunc).ConDirResumen(dirResumenes),
                Tracker:      tracker.NuevoTracker(20).ConLog(c.logFunc),
                // Nuevos componentes Fase 3.5:
                Parser:       arbol_ast.NuevoParser(),
                Grafo:        grafo.NuevoGrafo(),
                Buscador:     buscador.NuevoBuscador(),
                GenMapaRepo:  mapa_repo.NuevoGenerador(),
                Empaquetador: empaquetador.NuevoEmpaquetador(),
        }

        c.proyectos[nombre] = proy
        return proy, nil
}

// detectarModuloGo lee go.mod del proyecto y retorna el nombre del módulo.
// Si no es un proyecto Go o no hay go.mod, retorna string vacío.
func detectarModuloGo(rutaProyecto string) string {
        rutaGoMod := filepath.Join(rutaProyecto, "go.mod")
        datos, err := os.ReadFile(rutaGoMod)
        if err != nil {
                return ""
        }
        // Primera línea: "module github.com/foo/bar"
        lineas := strings.Split(string(datos), "\n")
        for _, linea := range lineas {
                linea = strings.TrimSpace(linea)
                if strings.HasPrefix(linea, "module ") {
                        return strings.TrimSpace(strings.TrimPrefix(linea, "module "))
                }
        }
        return ""
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

                dirResumenes := filepath.Join(c.dirBase, nombre, ".liz", "resumenes")

                modulo := detectarModuloGo(estado.Ruta)

                c.proyectos[nombre] = &ProyectoContexto{
                        Nombre:       nombre,
                        Ruta:         estado.Ruta,
                        Modulo:       modulo,
                        GenMapa:      mapa.NuevoGenerador().ConLog(c.logFunc),
                        Almacen:      almacen.ConLog(c.logFunc),
                        Indice:       gestorIndice.ConLog(c.logFunc),
                        GenResumen:   resumen.NuevoGenerador().ConLog(c.logFunc).ConDirResumen(dirResumenes),
                        Tracker:      tracker.NuevoTracker(20).ConLog(c.logFunc),
                        // Nuevos componentes Fase 3.5:
                        Parser:       arbol_ast.NuevoParser(),
                        Grafo:        grafo.NuevoGrafo(),
                        Buscador:     buscador.NuevoBuscador(),
                        GenMapaRepo:  mapa_repo.NuevoGenerador(),
                        Empaquetador: empaquetador.NuevoEmpaquetador(),
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
