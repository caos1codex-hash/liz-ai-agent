package grafo

import (
        "fmt"
        "testing"
)

// ═══════════════════════════════════════════════════════
// P4.3: BENCHMARK TESTS — PageRank
// ═══════════════════════════════════════════════════════

// construirGrafoGrande crea un grafo realista con n archivos y aristas de
// dependencias típicas de un proyecto Go mediano.
func construirGrafoGrande(n int) *Grafo {
        g := NuevoGrafo()

        // Generar nombres de archivos realistas
        paquetes := []string{
                "cmd/server",
                "internal/auth",
                "internal/auth/jwt",
                "internal/auth/password",
                "internal/database",
                "internal/database/postgres",
                "internal/database/migrations",
                "internal/cache",
                "internal/cache/redis",
                "internal/middleware",
                "internal/middleware/logger",
                "internal/middleware/cors",
                "internal/middleware/ratelimit",
                "internal/router",
                "internal/router/routes",
                "internal/users",
                "internal/users/service",
                "internal/users/repository",
                "internal/users/model",
                "internal/payments",
                "internal/payments/stripe",
                "internal/payments/repository",
                "internal/payments/model",
                "internal/email",
                "internal/email/sender",
                "internal/email/template",
                "internal/storage",
                "internal/storage/s3",
                "internal/config",
                "internal/config/loader",
                "internal/server",
                "internal/server/http",
                "internal/server/grpc",
                "internal/websocket",
                "internal/websocket/hub",
                "internal/messaging",
                "internal/messaging/processor",
                "internal/jobs",
                "internal/jobs/queue",
                "internal/jobs/worker",
                "internal/sessions",
                "internal/sessions/cleanup",
                "internal/health",
                "internal/health/checker",
                "internal/metrics",
                "internal/metrics/collector",
                "internal/templates",
                "internal/templates/renderer",
                "internal/static",
                "internal/static/server",
                "internal/utils",
                "internal/utils/uuid",
                "internal/utils/slug",
                "internal/crypto",
                "internal/crypto/hash",
                "internal/crypto/aes",
                "internal/search",
                "internal/search/index",
                "internal/search/service",
                "internal/worker",
                "internal/worker/background",
                "internal/resilience",
                "internal/resilience/circuit",
                "internal/tracing",
                "internal/tracing/span",
                "pkg/logger",
                "pkg/errors",
                "pkg/validator",
                "pkg/response",
                "pkg/pagination",
                "pkg/contextx",
        }

        // Agregar archivos (usar índice para garantizar unicidad de ruta)
        for i := 0; i < n; i++ {
                pkg := paquetes[i%len(paquetes)]
                ruta := fmt.Sprintf("%s/file_%04d.go", pkg, i)
                g.AgregarArchivo(ruta, "go", 50+i*10)
        }

        // Generar aristas de dependencia realistas (patrón hub-and-spoke + capa)
        // Archivos de bajo índice tienden a ser "core" (más importados)
        rutas := make([]string, 0, n)
        todosNodos := g.ObtenerTodos()
        for _, nodo := range todosNodos {
                rutas = append(rutas, nodo.Ruta)
        }

        for i, origen := range rutas {
                // Cada archivo importa 2-5 otros archivos
                numImports := 2 + (i%4)
                for j := 0; j < numImports; j++ {
                        // Importar archivos de "capa inferior" (índices menores = más core)
                        destIdx := j * (i + 1) % n
                        if destIdx != i {
                                g.AgregarImport(origen, rutas[destIdx])
                        }
                }
                // Archivos de rutas y handlers importan auth y database
                if i < n/4 {
                        g.AgregarImport(origen, rutas[n/4%n])
                        g.AgregarImport(origen, rutas[(n/4+1)%n])
                }
        }

        return g
}

// BenchmarkPageRank_CalcularImportancia mide el rendimiento del cálculo de
// PageRank sobre un grafo grande (200 nodos, ~400 aristas).
func BenchmarkPageRank_CalcularImportancia(b *testing.B) {
        g := construirGrafoGrande(200)

        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                g.CalcularImportancia(50, 0.85)
        }
}

// BenchmarkPageRank_CalcularImportancia_Grande mide PageRank con un grafo
// más grande (500 nodos, ~1000 aristas).
func BenchmarkPageRank_CalcularImportancia_Grande(b *testing.B) {
        g := construirGrafoGrande(500)

        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                g.CalcularImportancia(50, 0.85)
        }
}

// BenchmarkGrafo_AgregarArchivo mide el rendimiento de agregar archivos al grafo.
func BenchmarkGrafo_AgregarArchivo(b *testing.B) {
        g := NuevoGrafo()
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                g.AgregarArchivo(
                        fmt.Sprintf("internal/pkg/file_%d.go", i),
                        "go",
                        100,
                )
        }
}

// BenchmarkGrafo_AgregarImport mide el rendimiento de agregar aristas al grafo.
func BenchmarkGrafo_AgregarImport(b *testing.B) {
        g := NuevoGrafo()
        // Pre-agregar nodos
        for i := 0; i < 200; i++ {
                g.AgregarArchivo(fmt.Sprintf("file_%d.go", i), "go", 100)
        }
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                origen := fmt.Sprintf("file_%d.go", i%200)
                destino := fmt.Sprintf("file_%d.go", (i+1)%200)
                g.AgregarImport(origen, destino)
        }
}

// BenchmarkGrafo_TopN mide el rendimiento de consultar los N archivos más
// importantes después de calcular PageRank.
func BenchmarkGrafo_TopN(b *testing.B) {
        g := construirGrafoGrande(200)
        g.CalcularImportancia(50, 0.85)

        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                _ = g.TopN(10)
        }
}

// BenchmarkGrafo_Estadisticas mide el rendimiento de calcular estadísticas del grafo.
func BenchmarkGrafo_Estadisticas(b *testing.B) {
        g := construirGrafoGrande(200)
        g.CalcularImportancia(50, 0.85)

        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                _ = g.Estadisticas()
        }
}
