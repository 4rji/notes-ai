package chat

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"

	"github.com/4rji/notes-ai/ai"
	"github.com/4rji/notes-ai/config"
	"github.com/4rji/notes-ai/index"
	"github.com/4rji/notes-ai/reader"
)

const maxHistory = 20 // turnos máximos en el historial de conversación

// session agrupa el estado para responder preguntas, ya sea con RAG
// (índice de embeddings) o en modo fallback (contexto completo).
type session struct {
	cfg *config.Config
	dir string

	idx       *index.Index // nil → modo fallback
	fileCount int

	// modo fallback
	fallbackCtx string
	included    []string
	skipped     []string
}

// prepare construye la sesión: si hay credenciales de embeddings, sincroniza el
// índice; si no, carga las notas como contexto completo (con límite).
func prepare(cfg *config.Config, dir string, rebuild bool) (*session, error) {
	docs, err := reader.LoadFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("error leyendo notas: %w", err)
	}

	s := &session{cfg: cfg, dir: dir, fileCount: len(docs)}

	if !cfg.RAGEnabled() {
		s.fallbackCtx, s.included, s.skipped = reader.BuildContext(docs, cfg.MaxContextChars)
		return s, nil
	}

	path := filepath.Join(dir, reader.IndexFileName)
	idx, err := index.Load(path)
	if err != nil {
		return nil, fmt.Errorf("error leyendo índice: %w", err)
	}
	if idx == nil || rebuild {
		idx = &index.Index{}
	}

	st, err := idx.Sync(docs, cfg.EmbedModel, embedFunc(cfg))
	if err != nil {
		return nil, fmt.Errorf("error indexando: %w", err)
	}
	if st.Changed() || rebuild {
		if err := idx.Save(path); err != nil {
			return nil, fmt.Errorf("error guardando índice: %w", err)
		}
	}

	s.idx = idx
	s.fileCount = idx.FileCount()
	return s, nil
}

func embedFunc(cfg *config.Config) index.EmbedFunc {
	return func(texts []string) ([][]float32, error) { return ai.Embed(cfg, texts) }
}

// buildContext arma el contexto para una pregunta: trozos relevantes (RAG) o
// el contexto completo (fallback).
func (s *session) buildContext(question string) (string, error) {
	if s.idx == nil {
		return s.fallbackCtx, nil
	}
	embs, err := ai.Embed(s.cfg, []string{question})
	if err != nil {
		return "", err
	}
	hits := s.idx.Search(embs[0], s.cfg.TopK)

	var sb strings.Builder
	for _, h := range hits {
		sb.WriteString(fmt.Sprintf("\n=== FROM: ./%s ===\n%s\n", h.File, h.Text))
	}
	return sb.String(), nil
}

// RunInteractive corre el modo chat interactivo.
func RunInteractive(cfg *config.Config, dir string) error {
	s, err := prepare(cfg, dir, false)
	if err != nil {
		return err
	}

	printHeader(s)

	rl, err := readline.New("> ")
	if err != nil {
		return err
	}
	defer rl.Close()

	var history []ai.Message

	for {
		line, err := rl.Readline()
		if err != nil { // EOF o Ctrl+C
			fmt.Println("\n  Hasta luego.")
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch line {
		case "/salir", "/exit", "/quit":
			fmt.Println("  Hasta luego.")
			return nil

		case "/index":
			if !cfg.RAGEnabled() {
				fmt.Println("  El indexado requiere OPENAI_API_KEY (embeddings). Ver /ayuda.")
				fmt.Println()
				continue
			}
			fmt.Print("  indexando...")
			s, err = prepare(cfg, dir, false)
			fmt.Print("\r              \r")
			if err != nil {
				fmt.Printf("  Error indexando: %v\n\n", err)
				continue
			}
			fmt.Printf("  Índice actualizado: %d archivos, %d fragmentos.\n\n",
				s.idx.FileCount(), len(s.idx.Chunks))
			continue

		case "/reload":
			s, err = prepare(cfg, dir, false)
			if err != nil {
				fmt.Printf("  Error recargando: %v\n\n", err)
				continue
			}
			fmt.Printf("  Recargado: %d archivos.\n\n", s.fileCount)
			continue

		case "/reindex":
			s, err = prepare(cfg, dir, true)
			if err != nil {
				fmt.Printf("  Error reindexando: %v\n\n", err)
				continue
			}
			fmt.Printf("  Índice reconstruido: %d archivos.\n\n", s.fileCount)
			continue

		case "/info":
			printInfo(s)
			continue

		case "/limpiar", "/clear":
			history = nil
			fmt.Println("  Historial de conversación borrado.")
			fmt.Println()
			continue

		case "/ayuda", "/help":
			printHelp()
			continue
		}

		fmt.Print("  pensando...")
		ctx, err := s.buildContext(line)
		if err != nil {
			fmt.Print("\r               \r")
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}
		answer, err := ai.Ask(cfg, ctx, history, line)
		fmt.Print("\r               \r")
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		fmt.Println()
		fmt.Println(indent(answer))
		fmt.Println()

		history = append(history,
			ai.Message{Role: "user", Content: line},
			ai.Message{Role: "assistant", Content: answer},
		)
		if len(history) > maxHistory*2 {
			history = history[2:]
		}
	}

	return nil
}

// AskOnce responde una sola pregunta y termina.
func AskOnce(cfg *config.Config, dir string, question string) error {
	s, err := prepare(cfg, dir, false)
	if err != nil {
		return err
	}
	if s.fileCount == 0 {
		fmt.Println("  No se encontraron notas (.md, .txt, .json) en este directorio.")
	}

	ctx, err := s.buildContext(question)
	if err != nil {
		return err
	}
	answer, err := ai.Ask(cfg, ctx, nil, question)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(indent(answer))
	fmt.Println()
	return nil
}

// RunIndex construye o reconstruye el índice y muestra estadísticas.
func RunIndex(cfg *config.Config, dir string, rebuild bool) error {
	if !cfg.RAGEnabled() {
		fmt.Println("  El indexado requiere una API key de embeddings (OpenAI).")
		fmt.Println("  Agrega a tu ~/.zshrc:")
		fmt.Println()
		fmt.Println(`    export OPENAI_API_KEY="sk-..."`)
		fmt.Println()
		return nil
	}

	docs, err := reader.LoadFiles(dir)
	if err != nil {
		return fmt.Errorf("error leyendo notas: %w", err)
	}

	path := filepath.Join(dir, reader.IndexFileName)
	idx, err := index.Load(path)
	if err != nil {
		return fmt.Errorf("error leyendo índice: %w", err)
	}
	if idx == nil || rebuild {
		idx = &index.Index{}
	}

	fmt.Printf("  Indexando %d archivos...\n", len(docs))
	st, err := idx.Sync(docs, cfg.EmbedModel, embedFunc(cfg))
	if err != nil {
		return fmt.Errorf("error indexando: %w", err)
	}
	if err := idx.Save(path); err != nil {
		return fmt.Errorf("error guardando índice: %w", err)
	}

	fmt.Printf("  Listo. Nuevos: %d  Actualizados: %d  Eliminados: %d  Sin cambios: %d\n",
		st.Added, st.Updated, st.Removed, st.Unchanged)
	fmt.Printf("  Índice: %s (%d archivos, %d fragmentos)\n",
		reader.IndexFileName, idx.FileCount(), len(idx.Chunks))
	return nil
}

func printHeader(s *session) {
	provider := strings.Title(s.cfg.Provider)
	mode := "RAG (embeddings)"
	if s.idx == nil {
		mode = "contexto completo"
	}
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────┐")
	fmt.Printf("  │  notes-ai  │  %s %s\n", provider, s.cfg.Model)
	fmt.Printf("  │  Dir: %s\n", s.dir)
	fmt.Printf("  │  %d archivos  │  modo: %s\n", s.fileCount, mode)
	fmt.Println("  │  /ayuda para comandos  │  Ctrl+C para salir")
	fmt.Println("  └─────────────────────────────────────────────────────┘")
	fmt.Println()

	if s.fileCount == 0 {
		fmt.Println("  Aviso: no se encontraron archivos .md, .txt o .json aquí.")
		fmt.Println()
	} else if s.idx == nil && len(s.skipped) > 0 {
		fmt.Printf("  Aviso: %d archivo(s) omitido(s) por límite de contexto.\n", len(s.skipped))
		fmt.Println("  Configura OPENAI_API_KEY para activar búsqueda por embeddings (sin límite).")
		fmt.Println()
	}

	printMenu()
}

// printMenu muestra el menú de comandos disponibles (al arrancar y con /ayuda).
func printMenu() {
	fmt.Println("  Comandos:")
	fmt.Println("    /index     → indexar / actualizar las notas (RAG)")
	fmt.Println("    /reindex   → reconstruir el índice desde cero")
	fmt.Println("    /info      → estado del índice / archivos cargados")
	fmt.Println("    /reload    → recargar y sincronizar cambios")
	fmt.Println("    /limpiar   → borrar historial de conversación")
	fmt.Println("    /ayuda     → mostrar este menú")
	fmt.Println("    /salir     → salir del programa")
	fmt.Println()
	fmt.Println("  O simplemente escribe tu pregunta y presiona Enter.")
	fmt.Println()
}

func printInfo(s *session) {
	if s.idx != nil {
		fmt.Printf("\n  Modo RAG  │  %d archivos indexados, %d fragmentos\n",
			s.idx.FileCount(), len(s.idx.Chunks))
		fmt.Printf("  Modelo de embeddings: %s  │  top-K: %d\n\n", s.cfg.EmbedModel, s.cfg.TopK)
		return
	}
	fmt.Printf("\n  Modo contexto completo  │  %d archivos cargados\n", len(s.included))
	for _, f := range s.included {
		fmt.Printf("    • %s\n", f)
	}
	if len(s.skipped) > 0 {
		fmt.Printf("\n  %d omitidos (límite de contexto):\n", len(s.skipped))
		for _, f := range s.skipped {
			fmt.Printf("    ○ %s\n", f)
		}
	}
	fmt.Println()
}

func printHelp() {
	fmt.Println()
	printMenu()
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "  " + l
		}
	}
	return strings.Join(lines, "\n")
}
