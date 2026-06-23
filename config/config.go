package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Provider        string
	Model           string
	APIKey          string
	SystemPrompt    string
	MaxContextChars int

	// Embeddings (RAG). EmbedKey empty → RAG disabled, fallback a contexto completo.
	EmbedKey   string
	EmbedModel string
	TopK       int
}

// RAGEnabled indica si hay credenciales para usar búsqueda por embeddings.
func (c *Config) RAGEnabled() bool { return c.EmbedKey != "" }

const defaultSystemPrompt = `Eres un asistente inteligente que tiene acceso a las notas del usuario.
Las notas pueden contener texto, listas de tareas, diagramas (en formato Mermaid u otros), proyectos, documentos y cualquier tipo de información.
Responde en el mismo idioma en que te pregunten.
Usa las notas como contexto principal para responder. Si la respuesta no está en las notas, dilo claramente.
Sé conciso y útil.`

func Load() (*Config, error) {
	provider := os.Getenv("GOV_NOTES_PROVIDER")
	if provider == "" {
		provider = "anthropic"
	}

	model := os.Getenv("GOV_NOTES_MODEL")
	systemPrompt := os.Getenv("GOV_NOTES_SYSTEM")
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	// Default max context: conservative per provider (~tokens * charsPerToken)
	// Anthropic: 30K tokens → 120K chars; OpenAI free tier: 10K tokens → 40K chars
	defaultMaxChars := map[string]int{
		"anthropic": 120_000,
		"openai":    40_000,
	}

	var apiKey string
	switch provider {
	case "anthropic":
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if model == "" {
			model = "claude-sonnet-4-6"
		}
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
		if model == "" {
			model = "gpt-4o"
		}
	default:
		return nil, fmt.Errorf("provider desconocido: %q (usa 'anthropic' o 'openai')", provider)
	}

	if apiKey == "" {
		printSetupInstructions(provider)
		os.Exit(1)
	}

	maxChars := defaultMaxChars[provider]
	if v := os.Getenv("GOV_NOTES_MAX_CHARS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxChars = n
		}
	}

	// Embeddings: siempre vía OpenAI (Anthropic no tiene endpoint de embeddings).
	// Funciona aunque las respuestas las dé Claude.
	embedKey := os.Getenv("OPENAI_API_KEY")
	if k := os.Getenv("GOV_NOTES_EMBED_KEY"); k != "" {
		embedKey = k
	}
	embedModel := os.Getenv("GOV_NOTES_EMBED_MODEL")
	if embedModel == "" {
		embedModel = "text-embedding-3-small"
	}
	topK := 8
	if v := os.Getenv("GOV_NOTES_TOP_K"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topK = n
		}
	}

	return &Config{
		Provider:        provider,
		Model:           model,
		APIKey:          apiKey,
		SystemPrompt:    systemPrompt,
		MaxContextChars: maxChars,
		EmbedKey:        embedKey,
		EmbedModel:      embedModel,
		TopK:            topK,
	}, nil
}

func printSetupInstructions(provider string) {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════╗")
	fmt.Println("  ║          gov-notes — Configuración               ║")
	fmt.Println("  ╚══════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  No se encontró una API key configurada.")
	fmt.Println()

	if provider == "anthropic" || provider == "" {
		fmt.Println("  Para usar Anthropic (recomendado), agrega a tu ~/.zshrc:")
		fmt.Println()
		fmt.Println(`    export ANTHROPIC_API_KEY="sk-ant-..."`)
		fmt.Println(`    export GOV_NOTES_PROVIDER="anthropic"`)
	}

	fmt.Println()
	fmt.Println("  Para usar OpenAI, agrega a tu ~/.zshrc:")
	fmt.Println()
	fmt.Println(`    export OPENAI_API_KEY="sk-..."`)
	fmt.Println(`    export GOV_NOTES_PROVIDER="openai"`)

	fmt.Println()
	fmt.Println("  Opcionales:")
	fmt.Println(`    export GOV_NOTES_MODEL="claude-opus-4-8"   # modelo específico`)
	fmt.Println(`    export GOV_NOTES_SYSTEM="Eres un asistente..."  # system prompt`)

	fmt.Println()
	fmt.Println("  Luego ejecuta:  source ~/.zshrc")
	fmt.Println()
}
