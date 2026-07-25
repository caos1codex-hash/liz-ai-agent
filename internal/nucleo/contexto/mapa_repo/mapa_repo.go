// Package mapa_repo implementa el "Repository Map" estilo Aider: una vista
// compacta del proyecto que muestra solo firmas de símbolos (no código
// completo), ordenada por importancia y limitada por presupuesto de tokens.
//
// La idea central: el modelo puede ver TODO el proyecto en < 500 tokens,
// y luego pedir fragmentos específicos cuando necesita el código completo.
//
// Formato de ejemplo (500 tokens de presupuesto):
//
//   src/auth/jwt.go:
//     func GenerateToken(userID string, claims map[string]interface{}) (string, error)
//     func ValidateToken(token string) (*Claims, error)
//     type Claims struct { UserID string; Exp int64 }
//
//   src/auth/oauth.go:
//     func HandleOAuthCallback(w http.ResponseWriter, r *http.Request)
//
//   src/db/postgres.go:
//     func Connect(dsn string) (*sql.DB, error)
//     type Conn struct { db *sql.DB }
package mapa_repo

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/arbol_ast"
	"github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/contexto/grafo"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// EntradaMapaRepo es la representación compacta de un archivo en el mapa.
type EntradaMapaRepo struct {
	Ruta         string  `json:"ruta"`
	Lenguaje     string  `json:"lenguaje"`
	Lineas       int     `json:"lineas"`
	Importancia  float64 `json:"importancia"` // PageRank score [0.0, 1.0]
	Simbolos     []SimboloCompacto `json:"simbolos"`
	TokensAprox  int     `json:"tokens_aprox"`
}

// SimboloCompacto es un símbolo representado solo por su firma.
type SimboloCompacto struct {
	Nombre    string `json:"nombre"`
	Tipo      string `json:"tipo"`       // "funcion", "metodo", "estructura", etc.
	Firma     string `json:"firma"`      // firma completa en una línea
	Exportado bool   `json:"exportado"`
}

// MapaRepo es el repository map completo.
type MapaRepo struct {
	Proyecto       string             `json:"proyecto"`
	TotalArchivos  int                `json:"total_archivos"`
	ArchivosIncluidos int              `json:"archivos_incluidos"` // cuántos se incluyeron
	TokensAprox    int                `json:"tokens_aprox"`
	PresupuestoTokens int              `json:"presupuesto_tokens"`
	Entradas       []EntradaMapaRepo  `json:"entradas"`
	Truncado       bool               `json:"truncado"` // true si no caben todos
}

// Generador genera repository maps a partir de AST + grafo.
type Generador struct {
	parser *arbol_ast.Parser
}

// NuevoGenerador crea un nuevo generador de repository maps.
func NuevoGenerador() *Generador {
	return &Generador{
		parser: arbol_ast.NuevoParser(),
	}
}

// ═══════════════════════════════════════════════════════
// GENERACIÓN
// ═══════════════════════════════════════════════════════

// ArchivoParaMapa es la información mínima que Generador necesita sobre
// cada archivo para construir el mapa.
type ArchivoParaMapa struct {
	Ruta         string  // ruta relativa
	RutaAbsoluta string  // ruta absoluta al archivo
	Lenguaje     string
	Lineas       int
	Importancia  float64 // PageRank score
}

// Generar construye el repository map respetando el presupuesto de tokens.
//
// Algoritmo:
//   1. Ordenar archivos por importancia (PageRank score descendente)
//   2. Para cada archivo, generar su vista compacta (firmas solo)
//   3. Estimar tokens (4 chars ≈ 1 token)
//   4. Agregar archivos hasta alcanzar el presupuesto
//   5. Si no caben todos, marcar truncado=true
func (g *Generador) Generar(proyecto string, archivos []ArchivoParaMapa, presupuestoTokens int) *MapaRepo {
	mapa := &MapaRepo{
		Proyecto:          proyecto,
		TotalArchivos:     len(archivos),
		PresupuestoTokens: presupuestoTokens,
		Truncado:          false,
	}

	// Ordenar por importancia descendente
	sort.SliceStable(archivos, func(i, j int) bool {
		if archivos[i].Importancia != archivos[j].Importancia {
			return archivos[i].Importancia > archivos[j].Importancia
		}
		return archivos[i].Ruta < archivos[j].Ruta
	})

	tokensUsados := 0
	for _, arch := range archivos {
		// Parsear archivo a AST
		ast, err := g.parser.Parsear(arch.Ruta, arch.RutaAbsoluta)
		if err != nil || ast == nil {
			continue
		}

		// Convertir símbolos a versión compacta
		entrada := EntradaMapaRepo{
			Ruta:        arch.Ruta,
			Lenguaje:    arch.Lenguaje,
			Lineas:      arch.Lineas,
			Importancia: arch.Importancia,
		}

		for _, s := range ast.Simbolos {
			// Solo incluir símbolos "interesantes" (no imports)
			if s.Tipo == "import" {
				continue
			}
			entrada.Simbolos = append(entrada.Simbolos, SimboloCompacto{
				Nombre:    s.Nombre,
				Tipo:      s.Tipo,
				Firma:     s.Firma,
				Exportado: s.Exportado,
			})
		}

		// Estimar tokens de esta entrada
		entrada.TokensAprox = estimarTokensEntrada(entrada)

		// Si agregar esta entrada excede el presupuesto, marcar truncado y parar
		if tokensUsados+entrada.TokensAprox > presupuestoTokens {
			mapa.Truncado = true
			break
		}

		mapa.Entradas = append(mapa.Entradas, entrada)
		tokensUsados += entrada.TokensAprox
		mapa.ArchivosIncluidos++
	}

	mapa.TokensAprox = tokensUsados
	return mapa
}

// FormatoTexto retorna el mapa en formato texto plano (para incluir en prompts).
// Formato:
//   <ruta_del_archivo>
//     firma1
//     firma2
//
//   <otro_archivo>
//     firma3
func (m *MapaRepo) FormatoTexto() string {
	var b strings.Builder

	for _, entrada := range m.Entradas {
		b.WriteString(entrada.Ruta)
		b.WriteString(":\n")
		for _, s := range entrada.Simbolos {
			b.WriteString("  ")
			b.WriteString(s.Firma)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if m.Truncado {
		b.WriteString(fmt.Sprintf("// ... %d archivos más omitidos (presupuesto: %d tokens)\n",
			m.TotalArchivos-m.ArchivosIncluidos, m.PresupuestoTokens))
	}

	return b.String()
}

// FormatoMarkdown retorna el mapa en formato Markdown (para UI/web).
func (m *MapaRepo) FormatoMarkdown() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Repository Map: %s\n\n", m.Proyecto))
	b.WriteString(fmt.Sprintf("**%d archivos** (mostrando %d) · %d tokens aprox\n\n",
		m.TotalArchivos, m.ArchivosIncluidos, m.TokensAprox))

	for _, entrada := range m.Entradas {
		b.WriteString(fmt.Sprintf("## `%s`\n", entrada.Ruta))
		b.WriteString(fmt.Sprintf("_%s · %d líneas · importancia: %.2f_\n\n",
			entrada.Lenguaje, entrada.Lineas, entrada.Importancia))

		for _, s := range entrada.Simbolos {
			icono := iconoTipo(s.Tipo)
			b.WriteString(fmt.Sprintf("- %s `%s`\n", icono, s.Firma))
		}
		b.WriteString("\n")
	}

	if m.Truncado {
		b.WriteString(fmt.Sprintf("> ⚠️ %d archivos omitidos por presupuesto de tokens\n",
			m.TotalArchivos-m.ArchivosIncluidos))
	}

	return b.String()
}

// iconoTipo retorna un emoji/icono para cada tipo de símbolo.
func iconoTipo(tipo string) string {
	switch tipo {
	case "funcion":
		return "ƒ"
	case "metodo":
		return "↳"
	case "estructura":
		return "S"
	case "interface":
		return "I"
	case "tipo":
		return "T"
	case "constante":
		return "C"
	case "variable":
		return "V"
	default:
		return "•"
	}
}

// estimarTokensEntrada calcula el costo aproximado en tokens de una entrada.
// Heurística: 4 chars ≈ 1 token (estándar GPT/LLM).
// Incluye: ruta + firmas + overhead de formato.
func estimarTokensEntrada(entrada EntradaMapaRepo) int {
	chars := len(entrada.Ruta) + 2 // "ruta:\n"
	for _, s := range entrada.Simbolos {
		chars += 2 + len(s.Firma) + 1 // "  firma\n"
	}
	chars += 1 // "\n" final
	return chars / 4
}

// EstimarTokensTexto calcula el costo aproximado en tokens de un texto.
// Útil para el Context Packer.
func EstimarTokensTexto(texto string) int {
	return len(texto) / 4
}

// ═══════════════════════════════════════════════════════
// HELPERS DE CONSTRUCCIÓN DESDE EL GRAFO
// ═══════════════════════════════════════════════════════

// ArchivosDesdeGrafo convierte un grafo en una lista de ArchivoParaMapa,
// listo para pasarse a Generar().
func ArchivosDesdeGrafo(g *grafo.Grafo, rutasAbsolutas map[string]string) []ArchivoParaMapa {
	nodos := g.ObtenerTodos()
	resultado := make([]ArchivoParaMapa, 0, len(nodos))

	for _, nodo := range nodos {
		rutaAbs, ok := rutasAbsolutas[nodo.Ruta]
		if !ok {
			continue
		}
		resultado = append(resultado, ArchivoParaMapa{
			Ruta:         nodo.Ruta,
			RutaAbsoluta: rutaAbs,
			Lenguaje:     nodo.Lenguaje,
			Lineas:       nodo.Lineas,
			Importancia:  nodo.Importancia,
		})
	}

	return resultado
}

// ExtensionLenguaje retorna la extensión de archivo típica para un lenguaje.
// Útil para debugging/logging.
func ExtensionLenguaje(lenguaje string) string {
	switch lenguaje {
	case "go":
		return ".go"
	case "python":
		return ".py"
	case "javascript":
		return ".js"
	case "typescript":
		return ".ts"
	case "rust":
		return ".rs"
	case "java":
		return ".java"
	case "c":
		return ".c"
	case "cpp":
		return ".cpp"
	default:
		return ""
	}
}

// NombreBase retorna el nombre del archivo sin la ruta.
func NombreBase(ruta string) string {
	return filepath.Base(ruta)
}
