package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/4rji/gov-notes/chat"
	"github.com/4rji/gov-notes/config"
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
		fmt.Println("gov-notes v" + version)
		return
	}

	// Flags override env vars
	if *provider != "" {
		os.Setenv("GOV_NOTES_PROVIDER", *provider)
	}
	if *model != "" {
		os.Setenv("GOV_NOTES_MODEL", *model)
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
			fmt.Fprintln(os.Stderr, "Uso: gov-notes ask \"tu pregunta aquí\"")
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
  gov-notes — Pregúntale a tus notas con IA

  Uso:
    gov-notes                        modo interactivo (chat)
    gov-notes ask "tu pregunta"      pregunta directa y sale
    gov-notes index                  construye/actualiza el índice
    gov-notes index --rebuild        reconstruye el índice desde cero
    gov-notes --provider openai      usa OpenAI en lugar de Anthropic
    gov-notes --model claude-opus-4-8  modelo específico

  Variables de entorno:
    ANTHROPIC_API_KEY     tu API key de Anthropic (respuestas)
    OPENAI_API_KEY        tu API key de OpenAI (respuestas y/o embeddings)
    GOV_NOTES_PROVIDER    anthropic (default) | openai
    GOV_NOTES_MODEL       nombre del modelo de respuestas
    GOV_NOTES_EMBED_MODEL modelo de embeddings (def: text-embedding-3-small)
    GOV_NOTES_TOP_K       fragmentos relevantes por pregunta (def: 8)
    GOV_NOTES_MAX_CHARS   límite de contexto en modo fallback
    GOV_NOTES_SYSTEM      system prompt personalizado

  Búsqueda inteligente (RAG):
    Si hay OPENAI_API_KEY, las notas se indexan con embeddings y cada
    pregunta solo envía los fragmentos relevantes (rápido y económico).
    Sin esa key, se usa el contexto completo con un límite de caracteres.

  Archivos soportados:  .md  .txt  .json
  Se leen de forma recursiva desde el directorio actual.`)
}
