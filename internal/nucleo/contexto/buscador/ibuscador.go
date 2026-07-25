package buscador

// IBuscador define la interfaz de búsqueda usada por el coordinador.
// Tanto *Buscador como *BuscadorEmbeddings implementan esta interfaz.
type IBuscador interface {
	Indexar(f FragmentoBuscable)
	Desindexar(id string)
	BuscarBM25(query string, topK int) []ResultadoBusqueda
	BuscarHibrido(query string, topK int) []ResultadoBusqueda
	Total() int
	Estadisticas() EstadisticasBuscador
}

// Compile-time checks.
var _ IBuscador = (*Buscador)(nil)
var _ IBuscador = (*BuscadorEmbeddings)(nil)
