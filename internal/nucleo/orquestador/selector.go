package orquestador

import (
        "fmt"
        "sort"
        "sync"
        "time"

        "github.com/caos1codex-hash/liz-ai-agent/internal/nucleo/config"
)

// ═══════════════════════════════════════════════════════
// SELECCIÓN DE MODELO
// ═══════════════════════════════════════════════════════

// SeleccionarModelo elige el mejor modelo para una tarea.
//
// Estrategia (en orden):
//   1. Si la solicitud especifica Modelo, usarlo (si está habilitado)
//   2. Filtrar modelos habilitados
//   3. Si la tarea es conocida, priorizar modelos con esa especialidad
//   4. Si no, usar el modelo con mejor tasa de éxito histórica
//   5. Si no hay métricas, usar el primer modelo habilitado (orden de config)
//
// Retorna el modelo seleccionado y la lista de fallback (otros modelos
// habilitados, en orden de prioridad).
func (o *Orquestador) SeleccionarModelo(tarea TipoTarea, modeloEspecifico string) (config.ConfiguracionModelo, []config.ConfiguracionModelo, error) {
        o.mu.RLock()
        defer o.mu.RUnlock()

        habilitados := make([]config.ConfiguracionModelo, 0)
        for _, m := range o.modelos {
                if m.Habilitado {
                        habilitados = append(habilitados, m)
                }
        }
        if len(habilitados) == 0 {
                return config.ConfiguracionModelo{}, nil, fmt.Errorf("no hay modelos habilitados")
        }

        // Si se especifica modelo, validarlo
        if modeloEspecifico != "" {
                for _, m := range habilitados {
                        if m.Nombre == modeloEspecifico {
                                fallback := filterOut(habilitados, m.Nombre)
                                return m, fallback, nil
                        }
                }
                return config.ConfiguracionModelo{}, nil, fmt.Errorf("modelo %s no está habilitado", modeloEspecifico)
        }

        // Si la tarea es conocida, priorizar modelos especializados
        if tarea != "" {
                especializados := filterByEspecialidad(habilitados, string(tarea))
                if len(especializados) > 0 {
                        // Dentro de los especializados, ordenar por métricas (tasa de éxito desc, latencia asc)
                        sortedByMetrics := o.sortByMetrics(especializados)
                        primero := sortedByMetrics[0]
                        fallback := filterOut(habilitados, primero.Nombre)
                        return primero, fallback, nil
                }
        }

        // Default: ordenar todos los habilitados por métricas
        sortedByMetrics := o.sortByMetrics(habilitados)
        primero := sortedByMetrics[0]
        fallback := filterOut(habilitados, primero.Nombre)
        return primero, fallback, nil
}

// filterOut retorna una lista sin el modelo con el nombre dado.
func filterOut(modelos []config.ConfiguracionModelo, nombre string) []config.ConfiguracionModelo {
        resultado := make([]config.ConfiguracionModelo, 0, len(modelos))
        for _, m := range modelos {
                if m.Nombre != nombre {
                        resultado = append(resultado, m)
                }
        }
        return resultado
}

// filterByEspecialidad retorna los modelos cuyo campo "Rol" contiene la especialidad.
// (Asumimos que el YAML de config usa "rol" para listar especialidades; en una
// futura iteración podemos agregar un campo "especialidades []string" dedicado.)
//
// Heurística: mapea tipos de tarea (en español) a substrings típicos en nombres
// de modelo (en inglés). Ej: TareaCodigo → "code", TareaRazonamiento → "reason",
// TareaContextoLargo → "long" o "128k", etc.
func filterByEspecialidad(modelos []config.ConfiguracionModelo, especialidad string) []config.ConfiguracionModelo {
        // Mapeo de tareas a substrings típicos en nombres de modelo
        substrings := subsstringsParaEspecialidad(especialidad)
        if len(substrings) == 0 {
                return nil
        }

        resultado := make([]config.ConfiguracionModelo, 0)
        for _, m := range modelos {
                nombreLower := toLower(m.Nombre)
                for _, sub := range substrings {
                        if containsSubstring(nombreLower, sub) {
                                resultado = append(resultado, m)
                                break
                        }
                }
        }
        return resultado
}

// subsstringsParaEspecialidad retorna substrings típicos en nombres de modelo
// para cada tipo de tarea.
func subsstringsParaEspecialidad(esp string) []string {
        switch toLower(esp) {
        case "codigo":
                return []string{"code", "codellama", "deepseek-coder", "starcoder"}
        case "razonamiento":
                return []string{"reason", "nemotron", "gpt-4", "llama-3.1-405b"}
        case "contexto_largo":
                return []string{"long", "128k", "1m", "2m"}
        case "resumen":
                return []string{"phi", "gemma", "small"}
        case "analisis":
                return []string{"mixtral", "nemotron"}
        case "creatividad":
                return []string{"nemotron", "gemma", "llama-3.1-405b"}
        case "general":
                return []string{"llama", "qwen", "mistral"}
        default:
                return []string{toLower(esp)}
        }
}

// sortByMetrics ordena modelos por tasa de éxito descendente y latencia ascendente.
func (o *Orquestador) sortByMetrics(modelos []config.ConfiguracionModelo) []config.ConfiguracionModelo {
        resultado := make([]config.ConfiguracionModelo, len(modelos))
        copy(resultado, modelos)

        sort.SliceStable(resultado, func(i, j int) bool {
                mi := o.metricas[resultado[i].Nombre]
                mj := o.metricas[resultado[j].Nombre]
                if mi == nil {
                        mi = &MetricasModelo{}
                }
                if mj == nil {
                        mj = &MetricasModelo{}
                }

                // Primero: tasa de éxito (si hay datos)
                tasaI := 0.5 // neutral si no hay datos
                tasaJ := 0.5
                if mi.TotalSolicitudes > 0 {
                        tasaI = float64(mi.Exitos) / float64(mi.TotalSolicitudes)
                }
                if mj.TotalSolicitudes > 0 {
                        tasaJ = float64(mj.Exitos) / float64(mj.TotalSolicitudes)
                }
                if tasaI != tasaJ {
                        return tasaI > tasaJ
                }

                // Segundo: latencia promedio (menor es mejor)
                return mi.LatenciaPromedio < mj.LatenciaPromedio
        })

        return resultado
}

// ═══════════════════════════════════════════════════════
// COMPLETAR (con fallback)
// ═══════════════════════════════════════════════════════

// Completar envía una solicitud de chat completion con selección inteligente
// y fallback automático.
//
// Si el primer modelo falla con un error reinterrable (429, 5xx), intenta
// el siguiente modelo en la cadena de fallback.
//
// Máximo de intentos: 1 (modelo principal) + len(fallback).
func (o *Orquestador) Completar(req SolicitudChat) (*RespuestaChat, error) {
        inicio := time.Now()

        modeloPrincipal, fallback, err := o.SeleccionarModelo(req.Tarea, req.Modelo)
        if err != nil {
                return nil, fmt.Errorf("error seleccionando modelo: %w", err)
        }

        // Cadena de intentos: principal + fallback
        cadena := append([]config.ConfiguracionModelo{modeloPrincipal}, fallback...)

        var ultimoError error
        intentos := 0

        for _, modelo := range cadena {
                intentos++
                o.logFunc("intentando modelo: %s (intento %d/%d)", modelo.Nombre, intentos, len(cadena))

                resp, apiErr, err := o.probarModelo(modelo, req)
                if err == nil && apiErr == nil {
                        // Éxito
                        latencia := time.Since(inicio)
                        o.registrarExito(modelo.Nombre, latencia, resp.TokensTotal)

                        resp.ModeloUsado = modelo.Nombre
                        resp.Latencia = latencia
                        resp.Intentos = intentos
                        return resp, nil
                }

                // Error
                razon := "red"
                reintentable := true
                if apiErr != nil {
                        razon = fmt.Sprintf("api_%d", apiErr.Status)
                        reintentable = apiErr.Reintentable
                }
                o.registrarFallo(modelo.Nombre, razon)

                ultimoError = err
                if err != nil {
                        ultimoError = err
                } else {
                        ultimoError = apiErr
                }

                // Si el error no es reinterrable, no seguir intentando
                if !reintentable {
                        o.logFunc("modelo %s falló con error no reinterrable: %v", modelo.Nombre, ultimoError)
                        break
                }

                o.logFunc("modelo %s falló (%v), intentando siguiente fallback", modelo.Nombre, ultimoError)
        }

        return &RespuestaChat{
                Error:    fmt.Sprintf("todos los modelos fallaron. Último error: %v", ultimoError),
                Intentos: intentos,
                Latencia: time.Since(inicio),
        }, fmt.Errorf("todos los modelos fallaron: %w", ultimoError)
}

// probarModelo envía la solicitud a un modelo específico y registra el resultado.
func (o *Orquestador) probarModelo(modelo config.ConfiguracionModelo, req SolicitudChat) (*RespuestaChat, *ErrorAPI, error) {
        // Construir solicitud OpenAI-compatible
        solicitud := solicitudChatOpenAI{
                Model:    modelo.Nombre,
                Messages: convertirMensajes(req.Mensajes),
                Stream:   false, // Completar siempre es no-streaming; usar CompletarStream para streaming
        }
        if req.Temperatura > 0 {
                t := req.Temperatura
                solicitud.Temperature = &t
        } else if modelo.Temperatura > 0 {
                t := modelo.Temperatura
                solicitud.Temperature = &t
        }
        if req.MaxTokens > 0 {
                mt := req.MaxTokens
                solicitud.MaxTokens = &mt
        } else if modelo.MaxTokens > 0 {
                mt := modelo.MaxTokens
                solicitud.MaxTokens = &mt
        }
        if req.TopP > 0 {
                tp := req.TopP
                solicitud.TopP = &tp
        }
        if req.FrecuenciaPenal > 0 {
                fp := req.FrecuenciaPenal
                solicitud.FrequencyPenalty = &fp
        }
        if req.PresenciaPenal > 0 {
                pp := req.PresenciaPenal
                solicitud.PresencePenalty = &pp
        }
        if len(req.Stop) > 0 {
                solicitud.Stop = req.Stop
        }

        resp, apiErr, err := o.cliente.ChatCompletion(solicitud)
        if err != nil {
                return nil, nil, err
        }
        if apiErr != nil {
                return nil, apiErr, nil
        }

        // Convertir respuesta
        resultado := &RespuestaChat{}
        if len(resp.Choices) > 0 {
                resultado.Contenido = resp.Choices[0].Message.Content
                resultado.AcabadoRazon = resp.Choices[0].FinishReason
        }
        resultado.TokensPrompt = resp.Usage.PromptTokens
        resultado.TokensComplet = resp.Usage.CompletionTokens
        resultado.TokensTotal = resp.Usage.TotalTokens
        resultado.ModeloUsado = resp.Model

        return resultado, nil, nil
}

// convertirMensajes convierte []MensajeChat a []mensajeOpenAI.
func convertirMensajes(msgs []MensajeChat) []mensajeOpenAI {
        resultado := make([]mensajeOpenAI, 0, len(msgs))
        for _, m := range msgs {
                resultado = append(resultado, mensajeOpenAI{
                        Role:    m.Rol,
                        Content: m.Contenido,
                        Name:    m.Nombre,
                })
        }
        return resultado
}

// ═══════════════════════════════════════════════════════
// STREAMING (con fallback limitado)
// ═══════════════════════════════════════════════════════

// CompletarStream es como Completar pero retorna un canal de chunks SSE.
//
// Nota: el fallback en streaming es complejo porque ya se han enviado chunks
// al cliente. Esta implementación solo reintenta si el error ocurre ANTES de
// recibir el primer chunk ( handshake). Si falla a mitad del stream, se
// propaga el error al cliente.
func (o *Orquestador) CompletarStream(req SolicitudChat) (<-chan ChunkStream, error) {
        modeloPrincipal, fallback, err := o.SeleccionarModelo(req.Tarea, req.Modelo)
        if err != nil {
                return nil, fmt.Errorf("error seleccionando modelo: %w", err)
        }

        cadena := append([]config.ConfiguracionModelo{modeloPrincipal}, fallback...)

        // Para streaming: intentar solo el primer modelo reinterrable.
        // Si falla antes del primer chunk, intentar el siguiente.
        for i, modelo := range cadena {
                if i >= 3 { // máximo 3 intentos para streaming
                        break
                }

                solicitud := solicitudChatOpenAI{
                        Model:    modelo.Nombre,
                        Messages: convertirMensajes(req.Mensajes),
                        Stream:   true,
                }
                if req.Temperatura > 0 {
                        t := req.Temperatura
                        solicitud.Temperature = &t
                }
                if req.MaxTokens > 0 {
                        mt := req.MaxTokens
                        solicitud.MaxTokens = &mt
                }

                ch, err := o.cliente.ChatCompletionStream(solicitud)
                if err != nil {
                        // Error antes de stream: reintentar
                        apiErr, ok := err.(*ErrorAPI)
                        reintentable := !ok || apiErr.Reintentable
                        o.registrarFallo(modelo.Nombre, "stream_setup")
                        if !reintentable {
                                return nil, err
                        }
                        continue
                }

                // Wrap canal para inyectar ModeloUsado en primer chunk y registrar métricas
                out := make(chan ChunkStream, 32)
                go func() {
                        defer close(out)
                        inicio := time.Now()
                        recibioChunk := false
                        var tokensEstim int

                        for chunk := range ch {
                                if chunk.Contenido != "" {
                                        recibioChunk = true
                                        tokensEstim += len(chunk.Contenido) / 4
                                }
                                chunk.ModeloUsado = modelo.Nombre
                                out <- chunk
                                if chunk.Acabado {
                                        break
                                }
                        }

                        latencia := time.Since(inicio)
                        if recibioChunk {
                                o.registrarExito(modelo.Nombre, latencia, tokensEstim)
                        } else {
                                o.registrarFallo(modelo.Nombre, "stream_vacio")
                        }
                }()

                return out, nil
        }

        return nil, fmt.Errorf("no se pudo establecer stream con ningún modelo")
}

// ═══════════════════════════════════════════════════════
// REGISTRO DE MÉTRICAS
// ═══════════════════════════════════════════════════════

// registrarExito actualiza métricas tras una solicitud exitosa.
func (o *Orquestador) registrarExito(modelo string, latencia time.Duration, tokens int) {
        o.mu.Lock()
        defer o.mu.Unlock()

        m, existe := o.metricas[modelo]
        if !existe {
                m = &MetricasModelo{Modelo: modelo}
                o.metricas[modelo] = m
        }

        m.TotalSolicitudes++
        m.Exitos++
        m.TasaExito = float64(m.Exitos) / float64(m.TotalSolicitudes)
        m.TokensConsumidos += tokens

        // Actualizar latencia promedio (media móvil simple)
        if m.LatenciaPromedio == 0 {
                m.LatenciaPromedio = latencia
        } else {
                m.LatenciaPromedio = (m.LatenciaPromedio + latencia) / 2
        }
        m.UltimoUso = time.Now().UTC().Format(time.RFC3339)
}

// registrarFallo actualiza métricas tras una solicitud fallida.
func (o *Orquestador) registrarFallo(modelo, razon string) {
        o.mu.Lock()
        defer o.mu.Unlock()

        m, existe := o.metricas[modelo]
        if !existe {
                m = &MetricasModelo{Modelo: modelo}
                o.metricas[modelo] = m
        }

        m.TotalSolicitudes++
        m.Fallos++
        m.TasaExito = float64(m.Exitos) / float64(m.TotalSolicitudes)
        m.UltimoUso = time.Now().UTC().Format(time.RFC3339)

        o.logFunc("modelo %s falló (razón: %s) — total: %d, éxitos: %d, fallos: %d, tasa: %.2f",
                modelo, razon, m.TotalSolicitudes, m.Exitos, m.Fallos, m.TasaExito)
}

// ═══════════════════════════════════════════════════════
// HELPERS STRING (sin importar strings para mantener imports mínimos)
// ═══════════════════════════════════════════════════════

// toLower convierte ASCII a minúsculas sin importar strings.
func toLower(s string) string {
        b := []byte(s)
        for i, c := range b {
                if c >= 'A' && c <= 'Z' {
                        b[i] = c + ('a' - 'A')
                }
        }
        return string(b)
}

// containsSubstring verifica si s contiene sub (sin importar strings).
func containsSubstring(s, sub string) bool {
        if len(sub) == 0 {
                return true
        }
        if len(sub) > len(s) {
                return false
        }
        for i := 0; i <= len(s)-len(sub); i++ {
                match := true
                for j := 0; j < len(sub); j++ {
                        if s[i+j] != sub[j] {
                                match = false
                                break
                        }
                }
                if match {
                        return true
                }
        }
        return false
}

//sync.Mutex used to satisfy import
var _ = sync.Mutex{}
