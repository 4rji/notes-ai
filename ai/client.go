package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/4rji/notes-ai/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Usage reporta los tokens reales consumidos en una llamada.
type Usage struct {
	Input  int
	Output int
}

var httpClient = &http.Client{Timeout: 120 * time.Second}

func Ask(cfg *config.Config, notesContext string, history []Message, question string) (string, Usage, error) {
	switch cfg.Provider {
	case "anthropic":
		return askAnthropic(cfg, notesContext, history, question)
	case "openai":
		return askOpenAI(cfg, notesContext, history, question)
	default:
		return "", Usage{}, fmt.Errorf("provider desconocido: %s", cfg.Provider)
	}
}

// ── Embeddings (OpenAI) ──────────────────────────────────────────────────────

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

const embedBatchSize = 64

// Embed convierte textos en vectores usando el endpoint de embeddings de OpenAI.
// Procesa en lotes y devuelve también el total de tokens consumidos.
func Embed(cfg *config.Config, texts []string) ([][]float32, int, error) {
	out := make([][]float32, 0, len(texts))
	totalTokens := 0
	for i := 0; i < len(texts); i += embedBatchSize {
		end := i + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		embs, tokens, err := embedBatch(cfg, texts[i:end])
		if err != nil {
			return nil, totalTokens, err
		}
		out = append(out, embs...)
		totalTokens += tokens
	}
	return out, totalTokens, nil
}

func embedBatch(cfg *config.Config, texts []string) ([][]float32, int, error) {
	body, _ := json.Marshal(embedRequest{Model: cfg.EmbedModel, Input: texts})

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.EmbedKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("error de red (embeddings): %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result embedResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, 0, fmt.Errorf("respuesta inválida (embeddings): %w", err)
	}
	if result.Error != nil {
		return nil, 0, fmt.Errorf("embeddings API error: %s", result.Error.Message)
	}

	out := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	return out, result.Usage.TotalTokens, nil
}

// ── Anthropic ────────────────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func askAnthropic(cfg *config.Config, notesContext string, history []Message, question string) (string, Usage, error) {
	systemPrompt := cfg.SystemPrompt
	if notesContext != "" {
		systemPrompt += "\n\n## NOTAS DEL USUARIO\n" + notesContext
	}

	messages := append(history, Message{Role: "user", Content: question})

	body, _ := json.Marshal(anthropicRequest{
		Model:     cfg.Model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  messages,
	})

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("error de red: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result anthropicResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", Usage{}, fmt.Errorf("respuesta inválida: %w", err)
	}
	if result.Error != nil {
		return "", Usage{}, fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Content) == 0 {
		return "", Usage{}, fmt.Errorf("respuesta vacía del API")
	}
	usage := Usage{Input: result.Usage.InputTokens, Output: result.Usage.OutputTokens}
	return result.Content[0].Text, usage, nil
}

// ── OpenAI ───────────────────────────────────────────────────────────────────

type openAIRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type openAIResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func askOpenAI(cfg *config.Config, notesContext string, history []Message, question string) (string, Usage, error) {
	systemContent := cfg.SystemPrompt
	if notesContext != "" {
		systemContent += "\n\n## NOTAS DEL USUARIO\n" + notesContext
	}

	messages := []Message{{Role: "system", Content: systemContent}}
	messages = append(messages, history...)
	messages = append(messages, Message{Role: "user", Content: question})

	body, _ := json.Marshal(openAIRequest{
		Model:    cfg.Model,
		Messages: messages,
	})

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("error de red: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result openAIResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", Usage{}, fmt.Errorf("respuesta inválida: %w", err)
	}
	if result.Error != nil {
		return "", Usage{}, fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("respuesta vacía del API")
	}
	usage := Usage{Input: result.Usage.PromptTokens, Output: result.Usage.CompletionTokens}
	return result.Choices[0].Message.Content, usage, nil
}
