package orquestador

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════
// CLIENTE NVIDIA (compatible con OpenAI API)
// ═══════════════════════════════════════════════════════

// ClienteNVIDIA es el cliente HTTP para la API de NVIDIA.
// Usa el formato OpenAI-compatible (mismo JSON schema).
type ClienteNVIDIA struct {
	endpoint   string // https://integrate.api.nvidia.com/v1
	apiKey     string
	httpClient *http.Client
}

// NuevoClienteNVIDIA crea un nuevo cliente NVIDIA.
func NuevoClienteNVIDIA(endpoint, apiKey string) *ClienteNVIDIA {
	return &ClienteNVIDIA{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // 2 minutos para LLM
		},
	}
}

// solicitudChatOpenAI es el body para /chat/completions (formato OpenAI).
type solicitudChatOpenAI struct {
	Model            string          `json:"model"`
	Messages         []mensajeOpenAI `json:"messages"`
	Temperature      *float64        `json:"temperature,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	Stream           bool            `json:"stream,omitempty"`
	Stop             []string        `json:"stop,omitempty"`
}

type mensajeOpenAI struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// respuestaChatOpenAI es la respuesta de /chat/completions.
type respuestaChatOpenAI struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ErrorAPI representa un error devuelto por la API.
type ErrorAPI struct {
	Status       int
	Message      string
	Type         string
	Code         string
	Reintentable bool
}

func (e *ErrorAPI) Error() string {
	return fmt.Sprintf("API error %d: %s (type=%s, code=%s)",
		e.Status, e.Message, e.Type, e.Code)
}

// ═══════════════════════════════════════════════════════
// CHAT COMPLETION (no streaming)
// ═══════════════════════════════════════════════════════

// ChatCompletion envía una solicitud de chat completion al modelo.
// Si stream=true en la solicitud, usar ChatCompletionStream en su lugar.
//
// Retorna:
//   - *respuestaChatOpenAI: respuesta parseada
//   - *ErrorAPI: error de la API (con Status para decidir reintento)
//   - error: error de red/parseo (siempre reinterrable)
func (c *ClienteNVIDIA) ChatCompletion(req solicitudChatOpenAI) (*respuestaChatOpenAI, *ErrorAPI, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error serializando solicitud: %w", err)
	}

	url := c.endpoint + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("error creando request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("error de red: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error leyendo respuesta: %w", err)
	}

	// Errores 4xx/5xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parsearErrorAPI(resp.StatusCode, respBody)
		return nil, apiErr, nil
	}

	// Parsear respuesta exitosa
	var result respuestaChatOpenAI
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, nil, fmt.Errorf("error parseando respuesta: %w (body: %s)", err, string(respBody))
	}

	return &result, nil, nil
}

// parsearErrorAPI construye un ErrorAPI desde el body de error de la API.
// Formato típico OpenAI: {"error": {"message": "...", "type": "...", "code": "..."}}
func parsearErrorAPI(status int, body []byte) *ErrorAPI {
	apiErr := &ErrorAPI{
		Status: status,
	}

	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		apiErr.Message = parsed.Error.Message
		apiErr.Type = parsed.Error.Type
		apiErr.Code = parsed.Error.Code
	} else {
		apiErr.Message = string(body)
	}

	// Determinar si es reinterrable
	// 429 = rate limit → reinterrable
	// 500, 502, 503, 504 = server error → reinterrable
	// 401, 403, 404 = no reinterrables
	switch status {
	case 429, 500, 502, 503, 504:
		apiErr.Reintentable = true
	}

	return apiErr
}

// ═══════════════════════════════════════════════════════
// CHAT COMPLETION STREAMING (SSE)
// ═══════════════════════════════════════════════════════

// ChatCompletionStream envía una solicitud streaming y retorna un canal
// que recibe chunks SSE hasta completarse o fallar.
//
// El llamador debe leer del canal hasta que se cierre.
// Si hay error de red, se envía un ChunkStream con Error != nil y se cierra.
// Si ctx se cancela, el goroutine de lectura termina limpiamente.
func (c *ClienteNVIDIA) ChatCompletionStream(ctx context.Context, req solicitudChatOpenAI) (<-chan ChunkStream, error) {
	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error serializando solicitud: %w", err)
	}

	url := c.endpoint + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creando request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error de red: %w", err)
	}

	// Errores 4xx/5xx: leer body y retornar ErrorAPI
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(resp.Body)
		apiErr := parsearErrorAPI(resp.StatusCode, errBody)
		return nil, apiErr
	}

	ch := make(chan ChunkStream, 32)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Leer SSE stream: líneas "data: {...}\n\n"
		reader := bufio.NewReader(resp.Body)
		for {
			// Verificar cancelación del contexto
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					ch <- ChunkStream{Error: fmt.Errorf("error leyendo stream: %w", err)}
				}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- ChunkStream{Acabado: true}
				return
			}

			// Parsear el chunk SSE (formato OpenAI)
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
				c := ChunkStream{
					Contenido: chunk.Choices[0].Delta.Content,
				}
				if chunk.Choices[0].FinishReason != "" {
					c.Acabado = true
				}
				ch <- c
			}
		}
	}()

	return ch, nil
}

// ═══════════════════════════════════════════════════════
// EMBEDDINGS
// ═══════════════════════════════════════════════════════

// SolicitudEmbeddings es el body para /embeddings.
type SolicitudEmbeddings struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// RespuestaEmbeddings es la respuesta de /embeddings.
type RespuestaEmbeddings struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// Embeddings genera embeddings para los textos dados usando el modelo especificado.
// Modelo por defecto: "nvidia/nv-embed-v1" (1024 dimensiones).
func (c *ClienteNVIDIA) Embeddings(textos []string, modelo string) (*RespuestaEmbeddings, *ErrorAPI, error) {
	if modelo == "" {
		modelo = "nvidia/nv-embed-v1"
	}

	req := SolicitudEmbeddings{
		Model: modelo,
		Input: textos,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error serializando solicitud: %w", err)
	}

	url := c.endpoint + "/embeddings"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("error creando request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("error de red: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error leyendo respuesta: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parsearErrorAPI(resp.StatusCode, respBody)
		return nil, apiErr, nil
	}

	var result RespuestaEmbeddings
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, nil, fmt.Errorf("error parseando respuesta: %w", err)
	}

	return &result, nil, nil
}
