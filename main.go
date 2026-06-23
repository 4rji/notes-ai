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
	config.InitLang()
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
		fmt.Fprintf(os.Stderr, config.Tr().ErrConfig, err)
		os.Exit(1)
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, config.Tr().ErrCwd, err)
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
			fmt.Fprintln(os.Stderr, config.Tr().AskUsage)
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
	fmt.Println(config.Tr().Usage)
}
