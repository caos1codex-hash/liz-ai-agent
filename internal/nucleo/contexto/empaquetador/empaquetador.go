// Package empaquetador ensambla el contexto óptimo para un LLM dado un
// intent del usuario y un presupuesto de tokens.
//
// Estrategia en capas (inspirada en Claude Code):
//
//   1. SIEMPRE incluir el Repository Map compacto (~30% del presupuesto)
//      - Vista de firmas de símbolos, ordenada por importancia PageRank
//      - Permite al modelo "ver todo el proyecto" sin saturar el contexto
//
//   2. Top-K fragmentos relevantes por hybrid search (~50% del presupuesto)
//      - Búsqueda BM25 sobre la query del usuario
//      - Incluye el código completo de los fragmentos más relevantes
//
//   3. Imports directos de esos fragmentos (~15% del presupuesto)
//      - Si un fragmento usa auth.Jwt, incluir también el fragmento de jwt.go
//      - Una profundidad de expansión (configurable, default 1)
//      - IMPLEMENTADO: usa Grafo.Vecinos + callback ObtenerFragmentosPorRuta
//
//   4. Archivos recientemente editados (~5% del presupuesto, opcional)
//      - Bias por localidad (Copilot-style)
//      - Solo si hay presupuesto restante
//      - IMPLEMENTADO: usa callback ObtenerFragmentosPorRuta para incluir
//        fragmentos reales de los archivos recientes
//
// El packer retorna:
//   - Un string listo para incluir en el prompt del modelo
//   - Metadata sobre qué se incluyó (para logging/debugging)
//   - Cantidad de tokens usados
package empaquetador

import (
        "fmt"
        "strings"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/buscador"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/grafo"
        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/mapa_repo"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// BuscadorHibrido es la interfaz mínima que necesita el empaquetador.
// Permite desacoplar del tipo concreto *buscador.Buscador.
type BuscadorHibrido interface {
        BuscarHibrido(query string, topK int) []buscador.ResultadoBusqueda
}

// SolicitudEmpaquetado describe qué contexto se necesita.
type SolicitudEmpaquetado struct {
        Proyecto          string   // nombre del proyecto
        Query             string   // intent del usuario / pregunta
        PresupuestoTokens int      // máximo de tokens a usar (default 8000)
        ArchivosRecientes []string // rutas relativas editadas recientemente (locality bias)
        ProfundidadImports int     // cuántos niveles de imports expandir (default 1)
}

// ContextoEmpaquetado es el resultado de empaquetar.
type ContextoEmpaquetado struct {
        Contenido        string               `json:"contenido"`           // string listo para prompt
        TokensUsados     int                  `json:"tokens_usados"`
        PresupuestoTokens int                 `json:"presupuesto_tokens"`
        MapaRepoIncluido bool                 `json:"mapa_repo_incluido"`
        FragmentosIncluidos []FragmentoIncluido `json:"fragmentos_incluidos"`
        TokensMapaRepo   int                  `json:"tokens_mapa_repo"`
        TokensFragmentos int                  `json:"tokens_fragmentos"`
        TokensImports    int                  `json:"tokens_imports"`
        TokensRecientes  int                  `json:"tokens_recientes"`
}

// FragmentoIncluido describe un fragmento que se incluyó en el contexto.
type FragmentoIncluido struct {
        ID       string  `json:"id"`
        Ruta     string  `json:"ruta"`
        Tipo     string  `json:"tipo"`     // "relevante", "import", "reciente"
        Score    float64 `json:"score"`
        LineaIni int     `json:"linea_ini"`
        LineaFin int     `json:"linea_fin"`
}

// Empaquetador ensambla contexto para LLMs.
type Empaquetador struct {
        mapaRepoGen *mapa_repo.Generador
}

// NuevoEmpaquetador crea un nuevo empaquetador.
func NuevoEmpaquetador() *Empaquetador {
        return &Empaquetador{
                mapaRepoGen: mapa_repo.NuevoGenerador(),
        }
}

// ═══════════════════════════════════════════════════════
// EMPAQUETADO
// ═══════════════════════════════════════════════════════

// Porcentajes del presupuesto asignados a cada capa.
const (
        pctMapaRepo   = 30 // 30% para el mapa del repo
        pctFragmentos = 50 // 50% para fragmentos relevantes
        pctImports    = 15 // 15% para imports expandidos
        pctRecientes  = 5  // 5% para archivos recientes
)

// DatosEmpaquetado es la entrada al empaquetador.
// Quien llama debe recolectar esta información del Coordinador.
//
// Callbacks opcionales (si son nil, la capa correspondiente se omite):
//   - ObtenerFragmento: lookup por ID (no usado activamente, futuro)
//   - ObtenerFragmentosPorRuta: lookup por ruta relativa → []FragmentoBuscable
//     (necesario para capas 3 y 4)
type DatosEmpaquetado struct {
        MapaRepo *mapa_repo.MapaRepo
        Buscador BuscadorHibrido
        Grafo    *grafo.Grafo
        // Callbacks opcionales:
        ObtenerFragmento        func(id string) (buscador.FragmentoBuscable, bool)
        ObtenerFragmentosPorRuta func(ruta string) []buscador.FragmentoBuscable
}

// Empaquetar ensambla el contexto óptimo para el LLM.
func (e *Empaquetador) Empaquetar(req SolicitudEmpaquetado, datos DatosEmpaquetado) *ContextoEmpaquetado {
        // Defaults
        if req.PresupuestoTokens <= 0 {
                req.PresupuestoTokens = 8000
        }
        // ProfundidadImports:
        //   0  = no expansion (capa 3 omitida)
        //   >0 = expandir a esa profundidad (1 = imports directos, 2 = transitivo)
        // El coordinador aplica el default de 1 si el usuario no especifica.

        resultado := &ContextoEmpaquetado{
                PresupuestoTokens: req.PresupuestoTokens,
        }

        presupuestoMapa := req.PresupuestoTokens * pctMapaRepo / 100
        presupuestoFragmentos := req.PresupuestoTokens * pctFragmentos / 100
        presupuestoImports := req.PresupuestoTokens * pctImports / 100
        presupuestoRecientes := req.PresupuestoTokens * pctRecientes / 100

        var b strings.Builder

        // ═══════════════════════════════════════════════════════
        // Capa 1: Repository Map
        // ═══════════════════════════════════════════════════════
        if datos.MapaRepo != nil {
                mapTexto := datos.MapaRepo.FormatoTexto()
                tokensMapa := mapa_repo.EstimarTokensTexto(mapTexto)

                // Truncar si excede el presupuesto
                if tokensMapa > presupuestoMapa {
                        mapTexto = truncarATokens(mapTexto, presupuestoMapa)
                        tokensMapa = presupuestoMapa
                }

                b.WriteString("# Repository Map\n\n")
                b.WriteString(mapTexto)
                b.WriteString("\n")

                resultado.MapaRepoIncluido = true
                resultado.TokensMapaRepo = tokensMapa
        }

        // ═══════════════════════════════════════════════════════
        // Capa 2: Fragmentos relevantes (BM25 + RRF)
        // ═══════════════════════════════════════════════════════
        fragmentosYaIncluidos := make(map[string]bool) // ID → true
        archivosIncluidos := make(map[string]bool)     // ruta → true (para capa 3)

        if datos.Buscador != nil && req.Query != "" {
                // Calcular topK basado en presupuesto (asumir ~500 tokens por fragmento)
                topK := presupuestoFragmentos / 500
                if topK < 1 {
                        topK = 1
                }
                if topK > 20 {
                        topK = 20
                }

                resultados := datos.Buscador.BuscarHibrido(req.Query, topK)

                b.WriteString("\n# Fragmentos relevantes\n\n")

                tokensFragmentosUsados := 0
                for _, r := range resultados {
                        fragTexto := fmt.Sprintf("## %s (score: %.3f)\n```%s\n%s\n```\n\n",
                                r.Fragmento.Ruta, r.Score, r.Fragmento.Lenguaje, r.Fragmento.Contenido)
                        tokensFrag := mapa_repo.EstimarTokensTexto(fragTexto)

                        if tokensFragmentosUsados+tokensFrag > presupuestoFragmentos {
                                break
                        }

                        b.WriteString(fragTexto)
                        resultado.FragmentosIncluidos = append(resultado.FragmentosIncluidos, FragmentoIncluido{
                                ID:    r.Fragmento.ID,
                                Ruta:  r.Fragmento.Ruta,
                                Tipo:  "relevante",
                                Score: r.Score,
                        })
                        fragmentosYaIncluidos[r.Fragmento.ID] = true
                        archivosIncluidos[r.Fragmento.Ruta] = true
                        tokensFragmentosUsados += tokensFrag
                }
                resultado.TokensFragmentos = tokensFragmentosUsados
        }

        // ═══════════════════════════════════════════════════════
        // Capa 3: Imports expandidos (N niveles de profundidad)
        // ═══════════════════════════════════════════════════════
        //
        // Implementación real: para cada archivo ya incluido como fragmento
        // relevante, consultar el grafo por sus vecinos (imports directos),
        // y para cada vecino traer sus fragmentos via callback.
        // Se respeta la profundidad configurada (BFS).
        if datos.Grafo != nil && datos.ObtenerFragmentosPorRuta != nil && req.ProfundidadImports > 0 {
                b.WriteString("\n# Dependencias directas\n\n")

                tokensImportsUsados := 0

                // BFS: nivel 0 = archivos incluidos como relevantes
                // nivel 1..N = vecinos (imports) de cada nivel anterior
                nivelActual := make([]string, 0, len(archivosIncluidos))
                for ruta := range archivosIncluidos {
                        nivelActual = append(nivelActual, ruta)
                }

                visitados := make(map[string]bool)
                for _, r := range nivelActual {
                        visitados[r] = true
                }

                for profundidad := 0; profundidad < req.ProfundidadImports; profundidad++ {
                        if tokensImportsUsados >= presupuestoImports {
                                break
                        }
                        if len(nivelActual) == 0 {
                                break
                        }

                        siguienteNivel := make([]string, 0)

                        for _, ruta := range nivelActual {
                                if tokensImportsUsados >= presupuestoImports {
                                        break
                                }

                                vecinos := datos.Grafo.Vecinos(ruta)
                                for _, vecino := range vecinos {
                                        if tokensImportsUsados >= presupuestoImports {
                                                break
                                        }
                                        if visitados[vecino] {
                                                continue
                                        }
                                        visitados[vecino] = true
                                        siguienteNivel = append(siguienteNivel, vecino)

                                        // Traer los fragmentos de este archivo vecino
                                        frags := datos.ObtenerFragmentosPorRuta(vecino)
                                        for _, f := range frags {
                                                if tokensImportsUsados >= presupuestoImports {
                                                        break
                                                }
                                                if fragmentosYaIncluidos[f.ID] {
                                                        continue
                                                }

                                                fragTexto := fmt.Sprintf("## %s (dependencia de %s)\n```%s\n%s\n```\n\n",
                                                        f.Ruta, ruta, f.Lenguaje, f.Contenido)
                                                tokensFrag := mapa_repo.EstimarTokensTexto(fragTexto)

                                                if tokensImportsUsados+tokensFrag > presupuestoImports {
                                                        break
                                                }

                                                b.WriteString(fragTexto)
                                                resultado.FragmentosIncluidos = append(resultado.FragmentosIncluidos, FragmentoIncluido{
                                                        ID:    f.ID,
                                                        Ruta:  f.Ruta,
                                                        Tipo:  "import",
                                                        Score: 0, // los imports no tienen score de búsqueda
                                                })
                                                fragmentosYaIncluidos[f.ID] = true
                                                tokensImportsUsados += tokensFrag
                                        }
                                }
                        }

                        nivelActual = siguienteNivel
                }
                resultado.TokensImports = tokensImportsUsados
        }

        // ═══════════════════════════════════════════════════════
        // Capa 4: Archivos recientes (locality bias)
        // ═══════════════════════════════════════════════════════
        //
        // Implementación real: para cada ruta en ArchivosRecientes,
        // traer sus fragmentos via callback e incluirlos.
        if len(req.ArchivosRecientes) > 0 && datos.ObtenerFragmentosPorRuta != nil {
                b.WriteString("\n# Archivos recientemente editados\n\n")

                tokensRecientesUsados := 0
                for _, ruta := range req.ArchivosRecientes {
                        if tokensRecientesUsados >= presupuestoRecientes {
                                break
                        }

                        frags := datos.ObtenerFragmentosPorRuta(ruta)
                        if len(frags) == 0 {
                                // Si no hay fragmentos, al menos mencionar la ruta
                                entrada := fmt.Sprintf("- `%s` (sin fragmentos indexados)\n", ruta)
                                tokensEntrada := mapa_repo.EstimarTokensTexto(entrada)
                                if tokensRecientesUsados+tokensEntrada > presupuestoRecientes {
                                        break
                                }
                                b.WriteString(entrada)
                                tokensRecientesUsados += tokensEntrada
                                continue
                        }

                        escribioAlguno := false
                        for _, f := range frags {
                                if tokensRecientesUsados >= presupuestoRecientes {
                                        break
                                }
                                if fragmentosYaIncluidos[f.ID] {
                                        continue
                                }

                                fragTexto := fmt.Sprintf("## %s (editado recientemente)\n```%s\n%s\n```\n\n",
                                        f.Ruta, f.Lenguaje, f.Contenido)
                                tokensFrag := mapa_repo.EstimarTokensTexto(fragTexto)

                                if tokensRecientesUsados+tokensFrag > presupuestoRecientes {
                                        break
                                }

                                b.WriteString(fragTexto)
                                resultado.FragmentosIncluidos = append(resultado.FragmentosIncluidos, FragmentoIncluido{
                                        ID:    f.ID,
                                        Ruta:  f.Ruta,
                                        Tipo:  "reciente",
                                        Score: 0,
                                })
                                fragmentosYaIncluidos[f.ID] = true
                                tokensRecientesUsados += tokensFrag
                                escribioAlguno = true
                        }

                        if !escribioAlguno && len(frags) > 0 {
                                // Todos los fragmentos ya estaban incluidos; mencionar la ruta
                                entrada := fmt.Sprintf("- `%s` (ya incluido arriba)\n", ruta)
                                tokensEntrada := mapa_repo.EstimarTokensTexto(entrada)
                                if tokensRecientesUsados+tokensEntrada <= presupuestoRecientes {
                                        b.WriteString(entrada)
                                        tokensRecientesUsados += tokensEntrada
                                }
                        }
                }
                resultado.TokensRecientes = tokensRecientesUsados
        }

        resultado.Contenido = b.String()
        resultado.TokensUsados = mapa_repo.EstimarTokensTexto(resultado.Contenido)

        return resultado
}

// truncarATokens corta un string a un número aproximado de tokens.
// Conserva el final más cercano al límite (sin cortar palabras a mitad).
func truncarATokens(s string, maxTokens int) string {
        maxChars := maxTokens * 4
        if len(s) <= maxChars {
                return s
        }

        // Cortar al último espacio antes del límite
        corte := maxChars
        for corte > 0 && s[corte-1] != ' ' && s[corte-1] != '\n' {
                corte--
        }
        if corte == 0 {
                corte = maxChars
        }
        return s[:corte] + "\n... (truncado)"
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// Resumen retorna una representación compacta del empaquetado (para logs).
func (c *ContextoEmpaquetado) Resumen() string {
        return fmt.Sprintf("ContextoEmpaquetado{tokens: %d/%d, mapa: %v, fragmentos: %d}",
                c.TokensUsados, c.PresupuestoTokens, c.MapaRepoIncluido, len(c.FragmentosIncluidos))
}
