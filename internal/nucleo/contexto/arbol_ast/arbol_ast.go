// Package arbol_ast proporciona parsing de código a nivel de AST real.
//
// Reemplaza los fragmentadores regex (que son frágiles ante strings,
// comentarios y llaves anidadas) con parsing AST nativo:
//
//   - Go: usa go/parser + go/ast de la stdlib (sin CGO, sin dependencias)
//   - Otros lenguajes: tree-sitter cuando CGO esté disponible,
//     o regex mejorado como fallback
//
// Para cada archivo se extraen "símbolos" (funciones, tipos, métodos, vars)
// con información rica: nombre, firma, docstring, receiver, parámetros,
// retornos, líneas de inicio/fin y símbolos importados.
package arbol_ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ═══════════════════════════════════════════════════════
// TIPOS
// ═══════════════════════════════════════════════════════

// Simbolo representa una unidad de código de nivel superior.
// Es la "vista enriquecida" de un fragmento: incluye firma parseada,
// docstring, receiver, etc.
type Simbolo struct {
	Nombre     string   `json:"nombre"`     // nombre del símbolo (e.g. "Handle", "Server")
	Tipo       string   `json:"tipo"`       // "funcion", "metodo", "estructura", "tipo", "constante", "variable", "import"
	Firma      string   `json:"firma"`      // firma completa en una línea
	Docstring  string   `json:"docstring"`  // comentario de documentación
	Receiver   string   `json:"receiver"`   // para métodos: "(s *Server)" o ""
	Parametros []string `json:"parametros"` // lista de parámetros tipados
	Retornos   []string `json:"retornos"`   // lista de tipos de retorno
	LineaIni   int      `json:"linea_ini"`  // 1-indexed
	LineaFin   int      `json:"linea_fin"`  // inclusive
	Exportado  bool     `json:"exportado"`  // primera letra mayúscula (Go)
	Contenido  string   `json:"contenido"`  // cuerpo completo del símbolo
	Importados []string `json:"importados"` // paquetes importados usados
}

// ArchivoAST es el resultado de parsear un archivo completo.
type ArchivoAST struct {
	Ruta        string    `json:"ruta"`
	Lenguaje    string    `json:"lenguaje"`
	Paquete     string    `json:"paquete"` // e.g. "auth"
	Imports     []string  `json:"imports"` // rutas importadas
	Simbolos    []Simbolo `json:"simbolos"`
	TotalLineas int       `json:"total_lineas"`
	TieneError  bool      `json:"tiene_error"`
	Error       string    `json:"error,omitempty"`
}

// Parser es el parser AST unificado.
// Delega al parser apropiado según el lenguaje.
type Parser struct{}

// NuevoParser crea un nuevo parser AST.
func NuevoParser() *Parser {
	return &Parser{}
}

// ═══════════════════════════════════════════════════════
// PUNTO DE ENTRADA
// ═══════════════════════════════════════════════════════

// Parsear analiza un archivo y retorna su AST enriquecido.
// Selecciona el parser apropiado según la extensión.
func (p *Parser) Parsear(rutaRelativa, rutaAbsoluta string) (*ArchivoAST, error) {
	ext := strings.ToLower(filepath.Ext(rutaRelativa))

	switch ext {
	case ".go":
		return p.parsearGo(rutaRelativa, rutaAbsoluta)
	default:
		// Para otros lenguajes, retornar AST vacío (los fragmentadores
		// regex existentes siguen funcionando).
		return &ArchivoAST{
			Ruta:     rutaRelativa,
			Lenguaje: detectarLenguaje(ext),
		}, nil
	}
}

// ParsearContenido parsea un string de código Go directamente.
// Útil para tests.
func (p *Parser) ParsearContenido(ruta, contenido string) (*ArchivoAST, error) {
	ext := strings.ToLower(filepath.Ext(ruta))
	if ext != ".go" {
		return &ArchivoAST{Ruta: ruta, Lenguaje: detectarLenguaje(ext)}, nil
	}
	return p.parsearGoContenido(ruta, contenido)
}

// ═══════════════════════════════════════════════════════
// GO PARSER (usando stdlib go/parser + go/ast)
// ═══════════════════════════════════════════════════════

// parsearGo parsea un archivo Go usando go/parser (stdlib, sin CGO).
func (p *Parser) parsearGo(rutaRelativa, rutaAbsoluta string) (*ArchivoAST, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rutaAbsoluta, nil, parser.ParseComments)
	if err != nil {
		return &ArchivoAST{
			Ruta:       rutaRelativa,
			Lenguaje:   "go",
			TieneError: true,
			Error:      err.Error(),
		}, nil // no retornar error: el archivo puede tener errores de sintaxis
	}

	return p.extraerDeGoAST(rutaRelativa, fset, file), nil
}

// parsearGoContenido parsea un string de código Go.
func (p *Parser) parsearGoContenido(ruta, contenido string) (*ArchivoAST, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, ruta, contenido, parser.ParseComments)
	if err != nil {
		return &ArchivoAST{
			Ruta:       ruta,
			Lenguaje:   "go",
			TieneError: true,
			Error:      err.Error(),
		}, nil
	}
	return p.extraerDeGoAST(ruta, fset, file), nil
}

// extraerDeGoAST convierte un *ast.File al ArchivoAST de Liz.
func (p *Parser) extraerDeGoAST(ruta string, fset *token.FileSet, file *ast.File) *ArchivoAST {
	result := &ArchivoAST{
		Ruta:     ruta,
		Lenguaje: "go",
		Paquete:  file.Name.Name,
	}

	// Imports
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		result.Imports = append(result.Imports, path)
		// Agregar como símbolo tipo "import"
		simbolo := Simbolo{
			Nombre:     filepath.Base(path),
			Tipo:       "import",
			Firma:      fmt.Sprintf(`import "%s"`, path),
			Importados: []string{path},
			Exportado:  false,
		}
		if imp.Name != nil {
			simbolo.Nombre = imp.Name.Name
			simbolo.Firma = fmt.Sprintf(`%s "%s"`, imp.Name.Name, path)
		}
		pos := fset.Position(imp.Pos())
		endPos := fset.Position(imp.End())
		simbolo.LineaIni = pos.Line
		simbolo.LineaFin = endPos.Line
		result.Simbolos = append(result.Simbolos, simbolo)
	}

	// Declaraciones de nivel superior
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			result.Simbolos = append(result.Simbolos, p.extraerFuncion(fset, d))
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					result.Simbolos = append(result.Simbolos, p.extraerTipo(fset, d, s))
				case *ast.ValueSpec:
					result.Simbolos = append(result.Simbolos, p.extraerVariable(fset, d, s))
				}
			}
		}
	}

	// Total de líneas
	if file != nil && file.End().IsValid() {
		result.TotalLineas = fset.Position(file.End()).Line
	}

	return result
}

// extraerFuncion extrae un *ast.FuncDecl (función o método) como Simbolo.
func (p *Parser) extraerFuncion(fset *token.FileSet, d *ast.FuncDecl) Simbolo {
	simbolo := Simbolo{
		Nombre:    d.Name.Name,
		Exportado: isExported(d.Name.Name),
	}

	// Tipo: funcion vs metodo
	if d.Recv != nil && len(d.Recv.List) > 0 {
		simbolo.Tipo = "metodo"
		simbolo.Receiver = expresionString(fset, d.Recv.List[0].Type)
	} else {
		simbolo.Tipo = "funcion"
	}

	// Parámetros
	if d.Type.Params != nil {
		for _, param := range d.Type.Params.List {
			tipo := expresionString(fset, param.Type)
			if len(param.Names) == 0 {
				simbolo.Parametros = append(simbolo.Parametros, tipo)
			} else {
				for _, name := range param.Names {
					simbolo.Parametros = append(simbolo.Parametros,
						fmt.Sprintf("%s %s", name.Name, tipo))
				}
			}
		}
	}

	// Retornos
	if d.Type.Results != nil {
		for _, result := range d.Type.Results.List {
			tipo := expresionString(fset, result.Type)
			if len(result.Names) == 0 {
				simbolo.Retornos = append(simbolo.Retornos, tipo)
			} else {
				for _, name := range result.Names {
					simbolo.Retornos = append(simbolo.Retornos,
						fmt.Sprintf("%s %s", name.Name, tipo))
				}
			}
		}
	}

	// Firma completa
	simbolo.Firma = p.construirFirmaFuncion(d, fset)

	// Docstring
	if d.Doc != nil {
		simbolo.Docstring = strings.TrimSpace(d.Doc.Text())
	}

	// Líneas
	pos := fset.Position(d.Pos())
	endPos := fset.Position(d.End())
	simbolo.LineaIni = pos.Line
	simbolo.LineaFin = endPos.Line

	return simbolo
}

// construirFirmaFuncion arma la firma completa en una línea.
// Ejemplo: `func (s *Server) Handle(ctx context.Context, req *Request) (*Response, error)`
func (p *Parser) construirFirmaFuncion(d *ast.FuncDecl, fset *token.FileSet) string {
	var b strings.Builder
	b.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		b.WriteString("(")
		if len(d.Recv.List[0].Names) > 0 {
			b.WriteString(d.Recv.List[0].Names[0].Name)
			b.WriteString(" ")
		}
		b.WriteString(expresionString(fset, d.Recv.List[0].Type))
		b.WriteString(") ")
	}
	b.WriteString(d.Name.Name)
	b.WriteString("(")
	if d.Type.Params != nil {
		params := make([]string, 0)
		for _, param := range d.Type.Params.List {
			tipo := expresionString(fset, param.Type)
			if len(param.Names) == 0 {
				params = append(params, tipo)
			} else {
				for _, name := range param.Names {
					params = append(params, fmt.Sprintf("%s %s", name.Name, tipo))
				}
			}
		}
		b.WriteString(strings.Join(params, ", "))
	}
	b.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		b.WriteString(" ")
		retornos := make([]string, 0)
		for _, result := range d.Type.Results.List {
			tipo := expresionString(fset, result.Type)
			if len(result.Names) == 0 {
				retornos = append(retornos, tipo)
			} else {
				for _, name := range result.Names {
					retornos = append(retornos, fmt.Sprintf("%s %s", name.Name, tipo))
				}
			}
		}
		if len(retornos) == 1 {
			b.WriteString(retornos[0])
		} else if len(retornos) > 1 {
			b.WriteString("(")
			b.WriteString(strings.Join(retornos, ", "))
			b.WriteString(")")
		}
	}
	return b.String()
}

// extraerTipo extrae un *ast.TypeSpec (type, struct, interface) como Simbolo.
func (p *Parser) extraerTipo(fset *token.FileSet, genDecl *ast.GenDecl, spec *ast.TypeSpec) Simbolo {
	simbolo := Simbolo{
		Nombre:    spec.Name.Name,
		Tipo:      "tipo",
		Exportado: isExported(spec.Name.Name),
	}

	switch spec.Type.(type) {
	case *ast.StructType:
		simbolo.Tipo = "estructura"
	case *ast.InterfaceType:
		simbolo.Tipo = "interface"
	}

	simbolo.Firma = fmt.Sprintf("type %s %s", spec.Name.Name,
		expresionString(fset, spec.Type))

	if spec.Doc != nil {
		simbolo.Docstring = strings.TrimSpace(spec.Doc.Text())
	} else if genDecl.Doc != nil {
		simbolo.Docstring = strings.TrimSpace(genDecl.Doc.Text())
	}

	pos := fset.Position(spec.Pos())
	endPos := fset.Position(spec.End())
	simbolo.LineaIni = pos.Line
	simbolo.LineaFin = endPos.Line

	return simbolo
}

// extraerVariable extrae un *ast.ValueSpec (var/const) como Simbolo.
func (p *Parser) extraerVariable(fset *token.FileSet, genDecl *ast.GenDecl, spec *ast.ValueSpec) Simbolo {
	tipo := "variable"
	if genDecl.Tok == token.CONST {
		tipo = "constante"
	}

	nombre := ""
	if len(spec.Names) > 0 {
		nombre = spec.Names[0].Name
	}

	simbolo := Simbolo{
		Nombre:    nombre,
		Tipo:      tipo,
		Exportado: isExported(nombre),
	}

	firma := fmt.Sprintf("%s %s", genDecl.Tok.String(), nombre)
	if spec.Type != nil {
		tipoStr := expresionString(fset, spec.Type)
		firma = fmt.Sprintf("%s %s %s", genDecl.Tok.String(), nombre, tipoStr)
	}
	simbolo.Firma = firma

	if spec.Doc != nil {
		simbolo.Docstring = strings.TrimSpace(spec.Doc.Text())
	} else if genDecl.Doc != nil {
		simbolo.Docstring = strings.TrimSpace(genDecl.Doc.Text())
	}

	pos := fset.Position(spec.Pos())
	endPos := fset.Position(spec.End())
	simbolo.LineaIni = pos.Line
	simbolo.LineaFin = endPos.Line

	return simbolo
}

// ═══════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════

// expresionString convierte una ast.Expr a string (la firma del tipo).
func expresionString(fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + expresionString(fset, e.X)
	case *ast.SelectorExpr:
		return expresionString(fset, e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + expresionString(fset, e.Elt)
	case *ast.MapType:
		return "map[" + expresionString(fset, e.Key) + "]" + expresionString(fset, e.Value)
	case *ast.FuncType:
		return "func(...)"
	case *ast.StructType:
		return "struct{...}"
	case *ast.InterfaceType:
		return "interface{...}"
	case *ast.Ellipsis:
		return "..." + expresionString(fset, e.Elt)
	case *ast.ChanType:
		return "chan " + expresionString(fset, e.Value)
	case *ast.BasicLit:
		return e.Value
	default:
		return "..."
	}
}

// isExported retorna true si el nombre empieza con mayúscula.
func isExported(nombre string) bool {
	if nombre == "" {
		return false
	}
	first := nombre[0]
	return first >= 'A' && first <= 'Z'
}

// detectarLenguaje detecta el lenguaje por extensión.
func detectarLenguaje(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".mjs", ".cjs", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc", ".cxx":
		return "cpp"
	case ".sh", ".bash":
		return "shell"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return ""
	}
}
