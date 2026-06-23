package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/4rji/notes-ai/chat"
	"github.com/4rji/notes-ai/config"
)

const version = "1.0.0"

func main() {
	var (
		provider = flag.String("provider", "", "API provider: anthropic (default) o openai")
		model    = flag.String("model", "", "Modelo a usar (ej: claude-opus-4-8, gpt-4o)")
		showVer  = flag.Bool("version", false, "Mostrar versión")
	)
	flag.Usage = usage
	flag.Parse()

	if *showVer {
		fmt.Println("notes-ai v" + version)
		return
	}

	// Flags override env vars
	if *provider != "" {
		os.Setenv("NOTES_AI_PROVIDER", *provider)
	}
	if *model != "" {
		os.Setenv("NOTES_AI_MODEL", *model)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error de configuración: %v\n", err)
		os.Exit(1)
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "No se pudo obtener el directorio actual: %v\n", err)
		os.Exit(1)
	}

	args := flag.Args()

	// Subcommand: index — construye/actualiza el índice de embeddings
	if len(args) > 0 && args[0] == "index" {
		rebuild := false
		for _, a := range args[1:] {
			if a == "--rebuild" {
				rebuild = true
			}
		}
		if err := chat.RunIndex(cfg, dir, rebuild); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Subcommand: ask "pregunta directa"
	if len(args) > 0 && args[0] == "ask" {
		question := strings.Join(args[1:], " ")
		if question == "" {
			fmt.Fprintln(os.Stderr, "Uso: notes-ai ask \"tu pregunta aquí\"")
			os.Exit(1)
		}
		if err := chat.AskOnce(cfg, dir, question); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Modo interactivo por defecto
	if err := chat.RunInteractive(cfg, dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`
  notes-ai — Pregúntale a tus notas con IA

  Uso:
    notes-ai                        modo interactivo (chat)
    notes-ai ask "tu pregunta"      pregunta directa y sale
    notes-ai index                  construye/actualiza el índice
    notes-ai index --rebuild        reconstruye el índice desde cero
    notes-ai --provider openai      usa OpenAI en lugar de Anthropic
    notes-ai --model claude-opus-4-8  modelo específico

  Variables de entorno:
    ANTHROPIC_API_KEY     tu API key de Anthropic (respuestas)
    OPENAI_API_KEY        tu API key de OpenAI (respuestas y/o embeddings)
    NOTES_AI_PROVIDER    anthropic (default) | openai
    NOTES_AI_MODEL       nombre del modelo de respuestas
    NOTES_AI_EMBED_MODEL modelo de embeddings (def: text-embedding-3-small)
    NOTES_AI_TOP_K       fragmentos relevantes por pregunta (def: 8)
    NOTES_AI_MAX_CHARS   límite de contexto en modo fallback
    NOTES_AI_SYSTEM      system prompt personalizado

  Búsqueda inteligente (RAG):
    Si hay OPENAI_API_KEY, las notas se indexan con embeddings y cada
    pregunta solo envía los fragmentos relevantes (rápido y económico).
    Sin esa key, se usa el contexto completo con un límite de caracteres.

  Archivos soportados:  .md  .txt  .json
  Se leen de forma recursiva desde el directorio actual.`)
}
