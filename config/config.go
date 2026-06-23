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

func Load() (*Config, error) {
	InitLang()

	provider := os.Getenv("NOTES_AI_PROVIDER")
	if provider == "" {
		provider = "anthropic"
	}

	model := os.Getenv("NOTES_AI_MODEL")
	systemPrompt := os.Getenv("NOTES_AI_SYSTEM")
	if systemPrompt == "" {
		systemPrompt = Tr().SystemPrompt
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
		return nil, fmt.Errorf(Tr().ErrUnknownProvider, provider)
	}

	if apiKey == "" {
		printSetupInstructions(provider)
		os.Exit(1)
	}

	maxChars := defaultMaxChars[provider]
	if v := os.Getenv("NOTES_AI_MAX_CHARS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxChars = n
		}
	}

	// Embeddings: siempre vía OpenAI (Anthropic no tiene endpoint de embeddings).
	// Funciona aunque las respuestas las dé Claude.
	embedKey := os.Getenv("OPENAI_API_KEY")
	if k := os.Getenv("NOTES_AI_EMBED_KEY"); k != "" {
		embedKey = k
	}
	embedModel := os.Getenv("NOTES_AI_EMBED_MODEL")
	if embedModel == "" {
		embedModel = "text-embedding-3-small"
	}
	topK := 8
	if v := os.Getenv("NOTES_AI_TOP_K"); v != "" {
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
	t := Tr()
	fmt.Println()
	fmt.Println("  notes-ai — " + t.SetupHeader)
	fmt.Println("  ──────────────────────────────")
	fmt.Println()
	fmt.Println("  " + t.NoAPIKey)
	fmt.Println()

	if provider == "anthropic" || provider == "" {
		fmt.Println("  " + t.UseAnthropic)
		fmt.Println()
		fmt.Println(`    export ANTHROPIC_API_KEY="sk-ant-..."`)
		fmt.Println(`    export NOTES_AI_PROVIDER="anthropic"`)
	}

	fmt.Println()
	fmt.Println("  " + t.UseOpenAI)
	fmt.Println()
	fmt.Println(`    export OPENAI_API_KEY="sk-..."`)
	fmt.Println(`    export NOTES_AI_PROVIDER="openai"`)

	fmt.Println()
	fmt.Println("  " + t.Optional)
	fmt.Println(`    export NOTES_AI_MODEL="claude-opus-4-8"   ` + t.CommentModel)
	fmt.Println(`    export NOTES_AI_LANG="en"   # en | es`)

	fmt.Println()
	fmt.Println("  " + t.ThenRun)
	fmt.Println()
}
