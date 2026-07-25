package auto_creacion

import (
        "context"
        "crypto/sha256"
        "encoding/hex"
        "fmt"
        "sync"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/herramientas/registro"
)

// ============================================================================
// Gestor — orquesta el flujo completo de auto-creación
// ============================================================================

// Gestor es el punto de entrada del sistema de auto-creación. Expone operaciones
// de alto nivel para:
//
//   - Detectar qué herramientas faltan (vía Detector + LLM)
//   - Crear una herramienta nueva (flujo completo: detect→generar→compilar→cargar→registrar)
//   - Cargar todas las herramientas auto-creadas al iniciar Liz
//   - Recompilar una herramienta desde su fuente
//   - Eliminar una herramienta (del registro y del catálogo)
//   - Probar una herramienta con parámetros arbitrarios
//
// El Gestor es thread-safe: todas las operaciones toman locks apropiados.
type Gestor struct {
        mu         sync.Mutex
        detector   *Detector
        generador  *Generador
        compilador *Compilador
        registro   *Registro
        catalogo   *registro.Catalogo // catálogo principal donde se registran las nuevas

        // herramientas cargadas en memoria (nombre → wrapper subprocess)
        cargadas map[string]*HerramientaSubproceso

        logFunc func(formato string, args ...interface{})
}

// NuevoGestor crea un Gestor con todos los componentes.
//
// Si llm es nil, el Detector y Generador quedan deshabilitados (pero el
// Gestor sigue pudiendo Cargar herramientas existentes, Recompilarlas
// desde fuente, Eliminarlas, etc. — solo no puede Crear nuevas con LLM).
//
// Si catalogo es nil, las herramientas se compilan y persisten pero no se
// registran automáticamente en el catálogo (útil para tests).
func NuevoGestor(
        llm ClienteLLM,
        dirRaiz string,
        catalogo *registro.Catalogo,
) (*Gestor, error) {
        reg, err := NuevoRegistro(dirRaiz)
        if err != nil {
                return nil, fmt.Errorf("creando registro: %w", err)
        }

        comp := NuevoCompilador()

        g := &Gestor{
                detector:   nil, // se setea abajo si llm != nil
                generador:  nil,
                compilador: comp,
                registro:   reg,
                catalogo:   catalogo,
                cargadas:   make(map[string]*HerramientaSubproceso),
                logFunc:    func(string, ...interface{}) {},
        }

        if llm != nil {
                g.detector = NuevoDetector(llm)
                g.generador = NuevoGenerador(llm)
        }

        return g, nil
}

// ConLog inyecta un logger opcional que se propaga a todos los sub-componentes.
func (g *Gestor) ConLog(fn func(formato string, args ...interface{})) *Gestor {
        if fn != nil {
                g.logFunc = fn
                g.compilador.ConLog(fn)
                g.registro.ConLog(fn)
                if g.detector != nil {
                        g.detector.ConLog(fn)
                }
                if g.generador != nil {
                        g.generador.ConLog(fn)
                }
                g.mu.Lock()
        for _, h := range g.cargadas {
                        h.ConLog(fn)
                }
                g.mu.Unlock()
        }
        return g
}

// LLMDisponible indica si el Gestor puede crear herramientas nuevas (vía LLM).
func (g *Gestor) LLMDisponible() bool {
        return g.detector != nil && g.generador != nil
}

// ============================================================================
// Crear — flujo completo
// ============================================================================

// Crear ejecuta el flujo completo de auto-creación:
//
//  1. Si sol.ForzarSpec != nil → usar esa spec; si no, si sol.ForzarNombre != ""
//     → construir spec mínima con ese nombre; si no, llamar al Detector.
//  2. Llamar al Generador con la spec (o usar GenerarDesdePlantilla si no hay LLM).
//  3. Llamar al Compilador con el fuente.
//  4. Crear HerramientaSubproceso y cargar info (op="info").
//  5. Persistir metadata en el Registro.
//  6. Registrar en el catálogo principal (si está configurado).
//
// Si cualquier etapa falla, retorna ResultadoCreacion con el error detallado
// y las etapas previas completadas (para diagnóstico).
func (g *Gestor) Crear(ctx context.Context, sol SolicitudCreacion) (*ResultadoCreacion, error) {
        g.mu.Lock()
        defer g.mu.Unlock()

        resultado := &ResultadoCreacion{}

        // ── ETAPA 1: Determinar la spec ──────────────────────────────────
        var spec SpecHerramienta
        if sol.ForzarSpec != nil {
                spec = *sol.ForzarSpec
                if err := normalizarSpec(&spec); err != nil {
                        resultado.Error = "spec inválida: " + err.Error()
                        return resultado, &ErrAutoCreacion{Etapa: "spec", Causa: err.Error()}
                }
        } else if sol.ForzarNombre != "" {
                // Construir spec mínima
                spec = SpecHerramienta{
                        Nombre:      sol.ForzarNombre,
                        Descripcion: "Herramienta auto-creada: " + sol.ForzarNombre,
                        Categoria:   "otro",
                }
                if err := normalizarSpec(&spec); err != nil {
                        resultado.Error = "nombre inválido: " + err.Error()
                        return resultado, &ErrAutoCreacion{Etapa: "spec", Causa: err.Error()}
                }
        } else {
                // Usar Detector
                if !g.LLMDisponible() {
                        resultado.Error = "LLM no disponible y no se proporcionó ForzarSpec/ForzarNombre"
                        return resultado, &ErrAutoCreacion{Etapa: "deteccion",
                                Causa: "LLM no configurado"}
                }

                deteccion, err := g.detector.Detectar(ctx, sol.Descripcion, sol.CatalogoActual)
                if err != nil {
                        resultado.Error = err.Error()
                        return resultado, err
                }
                resultado.Deteccion = deteccion

                if len(deteccion.Faltantes) == 0 {
                        resultado.Error = "el detector no identificó herramientas faltantes"
                        return resultado, &ErrAutoCreacion{Etapa: "deteccion",
                                Causa: "no se requieren herramientas nuevas"}
                }

                // Tomar la primera herramienta faltante (el caller puede invocar Crear
                // de nuevo para las siguientes)
                spec = deteccion.Faltantes[0]
        }

        resultado.Especificacion = spec
        g.logFunc("creando herramienta: %s (%s)", spec.Nombre, spec.Descripcion)

        // Verificar que no exista ya
        if g.registro.Existe(spec.Nombre) {
                resultado.Error = fmt.Sprintf("la herramienta '%s' ya existe", spec.Nombre)
                return resultado, &ErrAutoCreacion{Etapa: "spec",
                        Causa: "herramienta ya existe: " + spec.Nombre}
        }

        // ── ETAPA 2: Generar fuente ──────────────────────────────────────
        var genResult *ResultadoGeneracion
        if g.LLMDisponible() {
                gr, err := g.generador.Generar(ctx, spec)
                if err != nil {
                        // Fallback a stub si el LLM falló en generación
                        g.logFunc("WARN: generación LLM falló, usando stub: %v", err)
                        gr, err = GenerarDesdePlantilla(spec)
                        if err != nil {
                                resultado.Error = err.Error()
                                return resultado, err
                        }
                }
                genResult = gr
        } else {
                // Sin LLM: usar stub
                gr, err := GenerarDesdePlantilla(spec)
                if err != nil {
                        resultado.Error = err.Error()
                        return resultado, err
                }
                genResult = gr
        }
        resultado.Generacion = genResult

        // ── ETAPA 3: Compilar ────────────────────────────────────────────
        dirHerr := g.registro.RutaDirectorio(spec.Nombre)
        compResult, err := g.compilador.Compilar(ctx, dirHerr, genResult.FuenteGo)
        if err != nil {
                resultado.Compilacion = compResult
                resultado.Error = err.Error()
                return resultado, err
        }
        resultado.Compilacion = compResult

        // ── ETAPA 4: Cargar (crear wrapper subprocess) ───────────────────
        herr := NuevaHerramientaSubproceso(compResult.RutaBinario, spec.Nombre).
                ConLog(g.logFunc)

        // Validación explícita (verifica que el binario existe, responde op="info" y op="validar")
        if err := herr.Validar(); err != nil {
                resultado.CargaExitosa = false
                resultado.Error = "carga falló: " + err.Error()
                return resultado, &ErrAutoCreacion{Etapa: "carga", Causa: err.Error()}
        }
        resultado.CargaExitosa = true

        // ── ETAPA 5: Persistir metadata ──────────────────────────────────
        fuenteHash := sha256Hex(genResult.FuenteGo)
        meta := &MetadataHerramienta{
                SpecHerramienta: spec,
                CreadoEn:        time.Now(),
                ActualizadoEn:   time.Now(),
                ModeloGenerador: genResult.ModeloUsado,
                VersionContador: 1,
                Compila:         true,
                FuenteHash:      fuenteHash,
        }
        if resultado.Deteccion != nil {
                meta.ModeloDetector = resultado.Deteccion.ModeloUsado
        }

        if err := g.registro.Guardar(meta); err != nil {
                resultado.Error = "persistencia falló: " + err.Error()
                return resultado, &ErrAutoCreacion{Etapa: "registro",
                        Causa: err.Error(), Interno: err}
        }
        resultado.Metadata = meta

        // ── ETAPA 6: Registrar en catálogo principal ─────────────────────
        if g.catalogo != nil {
                if err := g.catalogo.Registrar(herr); err != nil {
                        resultado.Error = "registro en catálogo falló: " + err.Error()
                        return resultado, &ErrAutoCreacion{Etapa: "registro",
                                Causa: "catalogo", Interno: err}
                }
                g.cargadas[spec.Nombre] = herr
                resultado.Registrada = true
        }

        g.logFunc("herramienta creada exitosamente: %s", spec.Nombre)
        return resultado, nil
}

// ============================================================================
// Cargar al iniciar Liz
// ============================================================================

// CargarTodas escanea el registro y carga todas las herramientas auto-creadas
// en el catálogo. Se llama al iniciar Liz.
//
// Si una herramienta no compila o falla al cargar, se registra el error en
// su metadata pero NO se aborta el proceso completo — las demás se cargan.
//
// Retorna el número de herramientas cargadas exitosamente y la lista de errores.
func (g *Gestor) CargarTodas() (int, []error) {
        g.mu.Lock()
        defer g.mu.Unlock()

        metas, err := g.registro.Listar()
        if err != nil {
                return 0, []error{fmt.Errorf("listando registro: %w", err)}
        }

        cargadas := 0
        errs := []error{}

        for _, meta := range metas {
                // Verificar que el binario existe
                if !g.registro.BinarioExiste(meta.Nombre) {
                        errs = append(errs, fmt.Errorf("%s: binario no encontrado, saltando", meta.Nombre))
                        continue
                }

                rutaBin := g.registro.RutaBinario(meta.Nombre)
                herr := NuevaHerramientaSubproceso(rutaBin, meta.Nombre).ConLog(g.logFunc)

                // Verificar que responde
                if err := herr.Validar(); err != nil {
                        errs = append(errs, fmt.Errorf("%s: validar falló: %w", meta.Nombre, err))
                        // Marcar como no compila en metadata
                        meta.Compila = false
                        meta.UltimoError = err.Error()
                        _ = g.registro.Guardar(meta)
                        continue
                }

                // Registrar en catálogo
                if g.catalogo != nil {
                        if err := g.catalogo.Registrar(herr); err != nil {
                                errs = append(errs, fmt.Errorf("%s: registrar en catálogo: %w", meta.Nombre, err))
                                continue
                        }
                }
                g.cargadas[meta.Nombre] = herr
                cargadas++
                g.logFunc("herramienta cargada: %s", meta.Nombre)
        }

        g.logFunc("carga inicial: %d OK, %d errores", cargadas, len(errs))
        return cargadas, errs
}

// ============================================================================
// Recompilar desde fuente
// ============================================================================

// Recargar recompila una herramienta desde su fuente.go, reemplazando el
// binario actual. Útil cuando:
//   - Se editó el fuente manualmente
//   - Se quiere regenerar con un modelo diferente
//   - El binario se perdió pero el fuente está
//
// Si nuevoFuente != "", se usa ese; si no, se lee del disco.
// Si usarLLM=true y hay LLM, se regenera el fuente vía LLM antes de compilar.
func (g *Gestor) Recargar(ctx context.Context, nombre string, nuevoFuente string, usarLLM bool) (*ResultadoCreacion, error) {
        g.mu.Lock()
        defer g.mu.Unlock()

        meta, err := g.registro.Obtener(nombre)
        if err != nil {
                return nil, &ErrAutoCreacion{Etapa: "recarga", Causa: "obteniendo metadata", Interno: err}
        }

        resultado := &ResultadoCreacion{
                Especificacion: meta.SpecHerramienta,
        }

        // Determinar fuente a compilar
        var fuente string
        var modeloUsado string

        if nuevoFuente != "" {
                fuente = nuevoFuente
                modeloUsado = "manual"
        } else if usarLLM && g.LLMDisponible() {
                gen, err := g.generador.Generar(ctx, meta.SpecHerramienta)
                if err != nil {
                        g.logFunc("WARN: regeneración LLM falló, usando fuente existente: %v", err)
                        // Fallback: leer fuente del disco
                        f, err := g.registro.LeerFuente(nombre)
                        if err != nil {
                                resultado.Error = err.Error()
                                return resultado, &ErrAutoCreacion{Etapa: "recarga",
                                        Causa: "no se pudo leer fuente ni regenerar", Interno: err}
                        }
                        fuente = f
                        modeloUsado = "(existente)"
                } else {
                        fuente = gen.FuenteGo
                        modeloUsado = gen.ModeloUsado
                        resultado.Generacion = gen
                }
        } else {
                // Leer fuente del disco
                f, err := g.registro.LeerFuente(nombre)
                if err != nil {
                        resultado.Error = err.Error()
                        return resultado, &ErrAutoCreacion{Etapa: "recarga",
                                Causa: "leyendo fuente", Interno: err}
                }
                fuente = f
                modeloUsado = "(existente)"
        }

        // Compilar
        dirHerr := g.registro.RutaDirectorio(nombre)
        comp, err := g.compilador.Compilar(ctx, dirHerr, fuente)
        if err != nil {
                resultado.Compilacion = comp
                resultado.Error = err.Error()
                // Actualizar metadata para reflejar el fallo
                meta.Compila = false
                meta.UltimoError = err.Error()
                _ = g.registro.Guardar(meta)
                return resultado, &ErrAutoCreacion{Etapa: "recarga",
                        Causa: "compilación", Interno: err}
        }
        resultado.Compilacion = comp

        // Crear nuevo wrapper
        herr := NuevaHerramientaSubproceso(comp.RutaBinario, nombre).ConLog(g.logFunc)
        if err := herr.Validar(); err != nil {
                resultado.Error = "validar falló tras recarga: " + err.Error()
                meta.Compila = false
                meta.UltimoError = err.Error()
                _ = g.registro.Guardar(meta)
                return resultado, &ErrAutoCreacion{Etapa: "recarga",
                        Causa: "validar", Interno: err}
        }
        resultado.CargaExitosa = true

        // Actualizar metadata
        meta.ActualizadoEn = time.Now()
        meta.VersionContador++
        meta.Compila = true
        meta.UltimoError = ""
        meta.ModeloGenerador = modeloUsado
        meta.FuenteHash = sha256Hex(fuente)
        _ = g.registro.Guardar(meta)
        resultado.Metadata = meta

        // Re-registrar en catálogo (reemplaza la anterior)
        if g.catalogo != nil {
                // Eliminar la antigua primero si estaba cargada
                if g.catalogo.Existe(nombre) {
                        g.catalogo.Eliminar(nombre)
                }
                if err := g.catalogo.Registrar(herr); err != nil {
                        resultado.Error = "registrar en catálogo: " + err.Error()
                        return resultado, &ErrAutoCreacion{Etapa: "recarga",
                                Causa: "catalogo", Interno: err}
                }
        }
        g.cargadas[nombre] = herr
        resultado.Registrada = true

        g.logFunc("herramienta recargada: %s v%d", nombre, meta.VersionContador)
        return resultado, nil
}

// ============================================================================
// Eliminar
// ============================================================================

// Eliminar quita una herramienta del registro, del catálogo y limpia artifacts.
func (g *Gestor) Eliminar(nombre string) error {
        g.mu.Lock()
        defer g.mu.Unlock()

        if !g.registro.Existe(nombre) {
                return &ErrHerramientaNoCreada{Nombre: nombre}
        }

        // Quitar del catálogo
        if g.catalogo != nil && g.catalogo.Existe(nombre) {
                g.catalogo.Eliminar(nombre)
        }

        // Quitar de cargadas
        delete(g.cargadas, nombre)

        // Limpiar artifacts del disco
        if err := g.registro.Eliminar(nombre); err != nil {
                return fmt.Errorf("eliminando del registro: %w", err)
        }

        g.logFunc("herramienta eliminada: %s", nombre)
        return nil
}

// ============================================================================
// Probar (ejecución de prueba)
// ============================================================================

// Probar ejecuta la herramienta con parámetros de prueba y retorna el resultado.
// Útil para validar que una herramienta recién creada funciona.
func (g *Gestor) Probar(ctx context.Context, nombre string, params map[string]interface{}) (herramientas.Resultado, error) {
        g.mu.Lock()
        herr, ok := g.cargadas[nombre]
        g.mu.Unlock()

        if !ok {
                // Intentar cargar on-demand
                if !g.registro.BinarioExiste(nombre) {
                        return herramientas.Resultado{
                                Exito: false,
                                Error: fmt.Sprintf("herramienta '%s' no existe o binario no encontrado", nombre),
                        }, &ErrHerramientaNoCreada{Nombre: nombre}
                }

                g.mu.Lock()
                herr = NuevaHerramientaSubproceso(g.registro.RutaBinario(nombre), nombre).ConLog(g.logFunc)
                g.cargadas[nombre] = herr
                g.mu.Unlock()
        }

        res, err := herr.Ejecutar(ctx, params)

        // Actualizar estadísticas en metadata persistente
        errStr := ""
        if err != nil {
                errStr = err.Error()
        } else if !res.Exito {
                errStr = res.Error
        }
        _ = g.registro.IncrementarEstadisticas(nombre, err == nil && res.Exito, errStr)

        return res, err
}

// ============================================================================
// Listar y obtener info
// ============================================================================

// Listar retorna la metadata de todas las herramientas auto-creadas.
func (g *Gestor) Listar() ([]*MetadataHerramienta, error) {
        return g.registro.Listar()
}

// Obtener retorna la metadata de una herramienta específica.
func (g *Gestor) Obtener(nombre string) (*MetadataHerramienta, error) {
        return g.registro.Obtener(nombre)
}

// LeerFuente retorna el fuente.go de una herramienta.
func (g *Gestor) LeerFuente(nombre string) (string, error) {
        return g.registro.LeerFuente(nombre)
}

// LeerLogCompilacion retorna el log de la última compilación.
func (g *Gestor) LeerLogCompilacion(nombre string) (string, error) {
        return g.registro.LeerLogCompilacion(nombre)
}

// ============================================================================
// Helpers
// ============================================================================

// sha256Hex calcula el hash SHA-256 de un string y lo retorna como hex.
func sha256Hex(s string) string {
        h := sha256.Sum256([]byte(s))
        return hex.EncodeToString(h[:])
}
