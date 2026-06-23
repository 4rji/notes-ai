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
func prepare(cfg *config.Config, dir string, rebuild bool) (*session, index.Stats, error) {
	docs, err := reader.LoadFiles(dir)
	if err != nil {
		return nil, index.Stats{}, fmt.Errorf(config.Tr().ErrReadNotes, err)
	}

	s := &session{cfg: cfg, dir: dir, fileCount: len(docs)}

	if !cfg.RAGEnabled() {
		s.fallbackCtx, s.included, s.skipped = reader.BuildContext(docs, cfg.MaxContextChars)
		return s, index.Stats{}, nil
	}

	path := filepath.Join(dir, reader.IndexFileName)
	idx, err := index.Load(path)
	if err != nil {
		return nil, index.Stats{}, fmt.Errorf(config.Tr().ErrReadIndex, err)
	}
	if idx == nil || rebuild {
		idx = &index.Index{}
	}

	st, err := idx.Sync(docs, cfg.EmbedModel, embedFunc(cfg))
	if err != nil {
		return nil, st, fmt.Errorf(config.Tr().ErrIndexing, err)
	}
	if st.Changed() || rebuild {
		if err := idx.Save(path); err != nil {
			return nil, st, fmt.Errorf(config.Tr().ErrSaveIndex, err)
		}
	}

	s.idx = idx
	s.fileCount = idx.FileCount()
	return s, st, nil
}

func embedFunc(cfg *config.Config) index.EmbedFunc {
	return func(texts []string) ([][]float32, int, error) { return ai.Embed(cfg, texts) }
}

// buildContext arma el contexto para una pregunta: trozos relevantes (RAG) o
// el contexto completo (fallback). Devuelve también los tokens que costó
// embeber la consulta (0 en modo fallback).
func (s *session) buildContext(question string) (string, int, error) {
	if s.idx == nil {
		return s.fallbackCtx, 0, nil
	}
	embs, qTokens, err := ai.Embed(s.cfg, []string{question})
	if err != nil {
		return "", 0, err
	}
	hits := s.idx.Search(embs[0], s.cfg.TopK)

	var sb strings.Builder
	for _, h := range hits {
		sb.WriteString(fmt.Sprintf("\n=== FROM: ./%s ===\n%s\n", h.File, h.Text))
	}
	return sb.String(), qTokens, nil
}

// RunInteractive corre el modo chat interactivo.
func RunInteractive(cfg *config.Config, dir string) error {
	s, st, err := prepare(cfg, dir, false)
	if err != nil {
		return err
	}

	t := config.Tr()

	printHeader(s)
	if st.Changed() {
		fmt.Printf(t.IndexSynced, st.Added, st.Updated, fmtInt(st.Tokens))
	}

	rl, err := readline.New("> ")
	if err != nil {
		return err
	}
	defer rl.Close()

	var history []ai.Message
	var totIn, totOut, totEmbed, turns int // acumulados de la sesión

	for {
		line, err := rl.Readline()
		if err != nil { // EOF o Ctrl+C
			fmt.Println("\n" + t.Goodbye)
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch line {
		case "/salir", "/exit", "/quit":
			fmt.Println(t.Goodbye)
			return nil

		case "/index":
			if !cfg.RAGEnabled() {
				fmt.Println(t.IndexNeedsKey)
				fmt.Println()
				continue
			}
			fmt.Print(t.Indexing)
			s, st, err = prepare(cfg, dir, false)
			fmt.Print("\r              \r")
			if err != nil {
				fmt.Printf(t.ErrIndexCmd, err)
				continue
			}
			fmt.Printf(t.IndexUpdated, s.idx.FileCount(), len(s.idx.Chunks), fmtInt(st.Tokens))
			continue

		case "/reload":
			s, _, err = prepare(cfg, dir, false)
			if err != nil {
				fmt.Printf(t.ErrReloading, err)
				continue
			}
			fmt.Printf(t.Reloaded, s.fileCount)
			continue

		case "/reindex":
			s, st, err = prepare(cfg, dir, true)
			if err != nil {
				fmt.Printf(t.ErrReindexing, err)
				continue
			}
			fmt.Printf(t.Reindexed, s.fileCount, fmtInt(st.Tokens))
			continue

		case "/info":
			printInfo(s)
			fmt.Printf(t.SessionTokens, fmtInt(totIn), fmtInt(totOut), fmtInt(totEmbed), turns)
			continue

		case "/limpiar", "/clear":
			history = nil
			fmt.Println(t.HistoryCleared)
			fmt.Println()
			continue

		case "/ayuda", "/help":
			printHelp()
			continue
		}

		fmt.Print(t.Thinking)
		ctx, qTokens, err := s.buildContext(line)
		if err != nil {
			fmt.Print("\r               \r")
			fmt.Printf(t.ErrGeneric, err)
			continue
		}
		answer, usage, err := ai.Ask(cfg, ctx, history, line)
		fmt.Print("\r               \r")
		if err != nil {
			fmt.Printf(t.ErrGeneric, err)
			continue
		}

		fmt.Println()
		fmt.Println(indent(answer))
		fmt.Println()
		printTokens(usage, qTokens)

		totIn += usage.Input
		totOut += usage.Output
		totEmbed += qTokens
		turns++

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
	s, _, err := prepare(cfg, dir, false)
	if err != nil {
		return err
	}
	if s.fileCount == 0 {
		fmt.Println(config.Tr().NoNotesFound)
	}

	ctx, qTokens, err := s.buildContext(question)
	if err != nil {
		return err
	}
	answer, usage, err := ai.Ask(cfg, ctx, nil, question)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println(indent(answer))
	fmt.Println()
	printTokens(usage, qTokens)
	return nil
}

// RunIndex construye o reconstruye el índice y muestra estadísticas.
func RunIndex(cfg *config.Config, dir string, rebuild bool) error {
	t := config.Tr()
	if !cfg.RAGEnabled() {
		fmt.Println(t.IndexNeedsKeyLong)
		fmt.Println(t.AddToZshrc)
		fmt.Println()
		fmt.Println(`    export OPENAI_API_KEY="sk-..."`)
		fmt.Println()
		return nil
	}

	docs, err := reader.LoadFiles(dir)
	if err != nil {
		return fmt.Errorf(t.ErrReadNotes, err)
	}

	path := filepath.Join(dir, reader.IndexFileName)
	idx, err := index.Load(path)
	if err != nil {
		return fmt.Errorf(t.ErrReadIndex, err)
	}
	if idx == nil || rebuild {
		idx = &index.Index{}
	}

	fmt.Printf(t.IndexingNFiles, len(docs))
	st, err := idx.Sync(docs, cfg.EmbedModel, embedFunc(cfg))
	if err != nil {
		return fmt.Errorf(t.ErrIndexing, err)
	}
	if err := idx.Save(path); err != nil {
		return fmt.Errorf(t.ErrSaveIndex, err)
	}

	fmt.Printf(t.IndexDone, st.Added, st.Updated, st.Removed, st.Unchanged)
	fmt.Printf(t.EmbedTokens, fmtInt(st.Tokens))
	fmt.Printf(t.IndexInfoLine, reader.IndexFileName, idx.FileCount(), len(idx.Chunks))
	return nil
}

func printHeader(s *session) {
	t := config.Tr()
	provider := strings.Title(s.cfg.Provider)
	mode := t.ModeRAG
	if s.idx == nil {
		mode = t.ModeFull
	}
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────┐")
	fmt.Printf("  │  notes-ai  │  %s %s\n", provider, s.cfg.Model)
	fmt.Printf(t.HeaderDir, s.dir)
	fmt.Printf(t.HeaderFiles, s.fileCount, mode)
	fmt.Println(t.HeaderHelp)
	fmt.Println("  └─────────────────────────────────────────────────────┘")
	fmt.Println()

	if s.fileCount == 0 {
		fmt.Println(t.NoFilesWarn)
		fmt.Println()
	} else if s.idx == nil && len(s.skipped) > 0 {
		fmt.Printf(t.SkippedWarn, len(s.skipped))
		fmt.Println(t.EnableEmbed)
		fmt.Println()
	}

	printMenu()
}

// printMenu muestra el menú de comandos disponibles (al arrancar y con /ayuda).
func printMenu() {
	t := config.Tr()
	fmt.Println(t.MenuTitle)
	fmt.Println(t.MenuIndex)
	fmt.Println(t.MenuReindex)
	fmt.Println(t.MenuInfo)
	fmt.Println(t.MenuReload)
	fmt.Println(t.MenuClear)
	fmt.Println(t.MenuHelp)
	fmt.Println(t.MenuExit)
	fmt.Println()
	fmt.Println(t.MenuHint)
	fmt.Println()
}

func printInfo(s *session) {
	t := config.Tr()
	if s.idx != nil {
		fmt.Printf(t.InfoRAG, s.idx.FileCount(), len(s.idx.Chunks))
		fmt.Printf(t.InfoEmbed, s.cfg.EmbedModel, s.cfg.TopK)
		return
	}
	fmt.Printf(t.InfoFull, len(s.included))
	for _, f := range s.included {
		fmt.Printf("    • %s\n", f)
	}
	if len(s.skipped) > 0 {
		fmt.Printf(t.InfoSkipped, len(s.skipped))
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

// printTokens muestra el uso de tokens de una respuesta.
func printTokens(u ai.Usage, queryTokens int) {
	t := config.Tr()
	if queryTokens > 0 {
		fmt.Printf(t.TokensLine3, fmtInt(u.Input), fmtInt(u.Output), fmtInt(queryTokens))
	} else {
		fmt.Printf(t.TokensLine2, fmtInt(u.Input), fmtInt(u.Output))
	}
}

// fmtInt formatea un entero con separador de miles (1234 → "1,234").
func fmtInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
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
