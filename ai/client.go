package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/4rji/gov-notes/config"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var httpClient = &http.Client{Timeout: 120 * time.Second}

func Ask(cfg *config.Config, notesContext string, history []Message, question string) (string, error) {
	switch cfg.Provider {
	case "anthropic":
		return askAnthropic(cfg, notesContext, history, question)
	case "openai":
		return askOpenAI(cfg, notesContext, history, question)
	default:
		return "", fmt.Errorf("provider desconocido: %s", cfg.Provider)
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
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

const embedBatchSize = 64

// Embed convierte textos en vectores usando el endpoint de embeddings de OpenAI.
// Procesa en lotes para no exceder límites por request.
func Embed(cfg *config.Config, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += embedBatchSize {
		end := i + embedBatchSize
		if end > len(texts) {
			end = len(texts)
		}
		embs, err := embedBatch(cfg, texts[i:end])
		if err != nil {
			return nil, err
		}
		out = append(out, embs...)
	}
	return out, nil
}

func embedBatch(cfg *config.Config, texts []string) ([][]float32, error) {
	body, _ := json.Marshal(embedRequest{Model: cfg.EmbedModel, Input: texts})

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.EmbedKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error de red (embeddings): %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result embedResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("respuesta inválida (embeddings): %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("embeddings API error: %s", result.Error.Message)
	}

	out := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	return out, nil
}

// ── Anthropic ────────────────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	System    string            `json:"system"`
	Messages  []Message         `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func askAnthropic(cfg *config.Config, notesContext string, history []Message, question string) (string, error) {
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
		return "", fmt.Errorf("error de red: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result anthropicResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("respuesta inválida: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("respuesta vacía del API")
	}
	return result.Content[0].Text, nil
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
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func askOpenAI(cfg *config.Config, notesContext string, history []Message, question string) (string, error) {
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
		return "", fmt.Errorf("error de red: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result openAIResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("respuesta inválida: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("respuesta vacía del API")
	}
	return result.Choices[0].Message.Content, nil
}
