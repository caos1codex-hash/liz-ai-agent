package buscador

import (
        "fmt"
        "testing"
)

// ═══════════════════════════════════════════════════════
// P4.3: BENCHMARK TESTS
// ═══════════════════════════════════════════════════════

// fragmentosGoCode genera 100 fragmentos de código Go realistas para benchmarks.
func fragmentosGoCode(n int) []FragmentoBuscable {
        funciones := []string{
                "AuthenticateUser(ctx context.Context, username, password string) (Token, error)",
                "ConnectPostgres(dsn string) (*sql.DB, error)",
                "SetCache(ctx context.Context, key string, value []byte, ttl time.Duration) error",
                "GetCache(ctx context.Context, key string) ([]byte, error)",
                "LoggerMiddleware(next http.Handler) http.Handler",
                "SetupRouter() *mux.Router",
                "GenerateJWT(userID string, expiresAt time.Time) (string, error)",
                "ValidateJWT(tokenString string) (*Claims, error)",
                "HashPassword(password string) (string, error)",
                "CheckPassword(hashedPassword, password string) bool",
                "CreateUser(ctx context.Context, req CreateUserRequest) (*User, error)",
                "UpdateUser(ctx context.Context, userID string, req UpdateUserRequest) (*User, error)",
                "DeleteUser(ctx context.Context, userID string) error",
                "GetUserByID(ctx context.Context, userID string) (*User, error)",
                "ListUsers(ctx context.Context, filter UserFilter) ([]User, error)",
                "SendEmail(to, subject, body string) error",
                "ProcessPayment(ctx context.Context, req PaymentRequest) (*PaymentResult, error)",
                "RefundPayment(ctx context.Context, paymentID string) error",
                "UploadFile(ctx context.Context, file io.Reader, filename string) (string, error)",
                "DownloadFile(ctx context.Context, fileID string) (io.ReadCloser, error)",
                "ParseConfig(configPath string) (*Config, error)",
                "ValidateConfig(cfg *Config) error",
                "StartServer(addr string) error",
                "StopServer(ctx context.Context) error",
                "HandleWebSocket(conn *websocket.Conn)",
                "ProcessMessage(msg Message) (Response, error)",
                "EnqueueJob(job Job) (jobID string, error)",
                "DequeueJob(queue string) (*Job, error)",
                "RetryFailedJobs(maxRetries int) error",
                "CleanupExpiredSessions(ttl time.Duration) error",
                "RateLimitMiddleware(limit int, window time.Duration) func(http.Handler) http.Handler",
                "CORSHandler(allowedOrigins []string) func(http.Handler) http.Handler",
                "HealthCheck(ctx context.Context) (Status, error)",
                "MetricsCollector() *prometheus.Registry",
                "NewTemplateRenderer(dir string) (*Renderer, error)",
                "RenderTemplate(w io.Writer, name string, data interface{}) error",
                "ServeStaticFiles(root http.FileSystem) http.Handler",
                "NewRedisClient(addr string) (*redis.Client, error)",
                "CacheSet(ctx context.Context, key string, val interface{}, ttl time.Duration) error",
                "CacheGet(ctx context.Context, key string) (interface{}, error)",
                "CacheDel(ctx context.Context, keys ...string) error",
                "NewS3Client(region, bucket string) (*S3Client, error)",
                "PutObject(ctx context.Context, key string, data []byte) error",
                "GetObject(ctx context.Context, key string) ([]byte, error)",
                "DeleteObject(ctx context.Context, key string) error",
                "ListObjects(ctx context.Context, prefix string) ([]string, error)",
                "NewGRPCServer(addr string) (*grpc.Server, error)",
                "RegisterService(svc ServiceRegistrar)",
                "DialGRPC(addr string) (*grpc.ClientConn, error)",
                "LoadFixtures(db *sql.DB, fixturesPath string) error",
                "RunMigrations(db *sql.DB, migrationsPath string) error",
                "SeedDatabase(db *sql.DB) error",
                "TruncateTables(db *sql.DB, tables ...string) error",
                "NewUUID() string",
                "GenerateSlug(text string) string",
                "SanitizeInput(input string) string",
                "FormatPhoneNumber(raw string) string",
                "ParsePhoneNumber(formatted string) (country, number string, error)",
                "NewHTTPClient(timeout time.Duration) *http.Client",
                "RetryHTTPRequest(fn func() (*http.Response, error), attempts int) (*http.Response, error)",
                "DecodeJSON(body io.Reader, v interface{}) error",
                "EncodeJSON(w io.Writer, v interface{}) error",
                "CompressGzip(data []byte) ([]byte, error)",
                "DecompressGzip(data []byte) ([]byte, error)",
                "HashSHA256(data []byte) []byte",
                "VerifySignature(data, signature []byte, pubKey PublicKey) bool",
                "EncryptAES(key []byte, plaintext []byte) ([]byte, error)",
                "DecryptAES(key []byte, ciphertext []byte) ([]byte, error)",
                "NewPaginator(page, perPage int) *Paginator",
                "Paginate(items []Item, page, perPage int) ([]Item, PaginationMeta)",
                "SearchIndex(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error)",
                "IndexDocument(ctx context.Context, doc Document) error",
                "DeleteFromIndex(ctx context.Context, docID string) error",
                "NewBackgroundWorker(queueSize int) *Worker",
                "SubmitTask(w *Worker, task Task) error",
                "WaitAll(w *Worker, timeout time.Duration) error",
                "NewCircuitBreaker(failureThreshold int, cooldown time.Duration) *CircuitBreaker",
                "Execute(cb *CircuitBreaker, fn func() error) error",
                "NewRateLimiter(rate int, burst int) *limiter.Limiter",
                "Allow(rl *limiter.Limiter, key string) bool",
                "NewTracer(serviceName string) trace.Tracer",
                "StartSpan(tracer trace.Tracer, name string) trace.Span",
                "RecordError(span trace.Span, err error)",
        }

        rutas := []string{
                "internal/auth/handler.go",
                "internal/database/postgres.go",
                "internal/cache/redis.go",
                "internal/middleware/logger.go",
                "internal/router/routes.go",
                "internal/auth/jwt.go",
                "internal/auth/password.go",
                "internal/users/service.go",
                "internal/email/sender.go",
                "internal/payments/stripe.go",
                "internal/storage/s3.go",
                "internal/config/loader.go",
                "internal/server/http.go",
                "internal/server/grpc.go",
                "internal/websocket/hub.go",
                "internal/messaging/processor.go",
                "internal/jobs/queue.go",
                "internal/sessions/cleanup.go",
                "internal/middleware/ratelimit.go",
                "internal/middleware/cors.go",
                "internal/health/checker.go",
                "internal/metrics/collector.go",
                "internal/templates/renderer.go",
                "internal/static/server.go",
                "internal/redis/client.go",
                "internal/cache/operations.go",
                "internal/storage/s3_ops.go",
                "internal/grpc/server.go",
                "internal/grpc/client.go",
                "internal/database/fixtures.go",
                "internal/database/migrations.go",
                "internal/database/seeds.go",
                "internal/utils/uuid.go",
                "internal/utils/slug.go",
                "internal/utils/sanitize.go",
                "internal/utils/phone.go",
                "internal/http/client.go",
                "internal/http/retry.go",
                "internal/http/json.go",
                "internal/compress/gzip.go",
                "internal/crypto/hash.go",
                "internal/crypto/signature.go",
                "internal/crypto/aes.go",
                "internal/pagination/paginator.go",
                "internal/search/index.go",
                "internal/search/service.go",
                "internal/worker/background.go",
                "internal/worker/tasks.go",
                "internal/resilience/circuit.go",
                "internal/ratelimit/limiter.go",
                "internal/tracing/tracer.go",
                "internal/tracing/span.go",
                "internal/tracing/errors.go",
        }

        tipos := []string{"funcion", "metodo", "handler", "middleware", "helper"}
        lenguajes := []string{"go", "go", "go", "go", "go"}

        fragments := make([]FragmentoBuscable, n)
        for i := 0; i < n; i++ {
                fi := i % len(funciones)
                ri := i % len(rutas)
                ti := i % len(tipos)

                // Agregar ruido realista al contenido
                signature := funciones[fi]
                // Incluir palabras clave en snake_case en comentarios para que BM25
                // pueda tokenizarlas correctamente (camelCase se concatena)
                descripcion := fmt.Sprintf("%s %s handler service", rutas[ri], funciones[fi])
                cont := fmt.Sprintf(`package main

import (
        "context"
        "fmt"
)

// %s implements the core logic for %s.
// Keywords: %s
// This function handles validation, error checking, and response formatting.
func %s {
        if ctx == nil {
                return nil, fmt.Errorf("context cannot be nil")
        }
        result, err := processInternal(ctx)
        if err != nil {
                return nil, fmt.Errorf("processing failed: %%w", err)
        }
        return result, nil
}

func processInternal(ctx context.Context) (interface{}, error) {
        select {
        case <-ctx.Done():
                return nil, ctx.Err()
        default:
                return struct{}{}, nil
        }
}`, rutas[ri], rutas[ri], descripcion, signature)

                fragments[i] = FragmentoBuscable{
                        ID:        fmt.Sprintf("frag-%04d", i),
                        Ruta:      rutas[ri],
                        Contenido: cont,
                        Tipo:      tipos[ti],
                        Lenguaje:  lenguajes[ti],
                }
        }
        return fragments
}

// BenchmarkBM25_Buscar mide el rendimiento de búsqueda BM25 puro con un índice
// grande (100 fragmentos de código Go realistas).
func BenchmarkBM25_Buscar(b *testing.B) {
        idx := NuevoBuscador()
        frags := fragmentosGoCode(100)
        for _, f := range frags {
                idx.Indexar(f)
        }

        queries := []string{
                "authenticate",
                "postgres database",
                "cache redis",
                "cors middleware",
                "password hash",
                "user service",
                "jwt token",
                "health check",
                "websocket message",
                "background job",
                "circuit breaker",
                "rate limiter",
                "pagination page",
                "search document",
                "tracer span",
        }

        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                query := queries[i%len(queries)]
                resultados := idx.BuscarBM25(query, 10)
                // No assertion here — benchmarks should not fail on empty results
                // as long as the index is valid (some queries legitimately have 0 matches)
                _ = resultados
        }
}

// BenchmarkBuscadorEmbeddings_BuscarHibrido mide el rendimiento de búsqueda
// híbrida (BM25 + embeddings + RRF) con 100 fragmentos de código Go.
func BenchmarkBuscadorEmbeddings_BuscarHibrido(b *testing.B) {
        provider := &mockProvider{dimensiones: 32}
        be := NuevoBuscadorEmbeddings(provider)

        frags := fragmentosGoCode(100)
        for _, f := range frags {
                _ = be.IndexarConEmbeddings(f)
        }

        queries := []string{
                "authenticate",
                "postgres database",
                "cache redis",
                "cors middleware",
                "password hash",
                "user service",
                "jwt token",
                "health check",
                "websocket message",
                "background job",
                "circuit breaker",
                "rate limiter",
                "pagination page",
                "search document",
                "tracer span",
        }

        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                query := queries[i%len(queries)]
                resultados := be.BuscarHibridoConEmbeddings(query, 10)
                _ = resultados
        }
}

// BenchmarkBuscadorEmbeddings_IndexarBatch mide el rendimiento de indexación
// por lotes con generación de embeddings.
func BenchmarkBuscadorEmbeddings_IndexarBatch(b *testing.B) {
        frags := fragmentosGoCode(50)
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                provider := &mockProvider{dimensiones: 32}
                be := NuevoBuscadorEmbeddings(provider)
                _, _ = be.IndexarBatchConEmbeddings(frags)
        }
}

// BenchmarkTokenizar mide el rendimiento de la tokenización individual.
func BenchmarkTokenizar(b *testing.B) {
        texto := "func AuthenticateUser(ctx context.Context, username, password string) (Token, error) { return nil, nil }"
        b.ResetTimer()
        for i := 0; i < b.N; i++ {
                _ = tokenizar(texto)
        }
}


