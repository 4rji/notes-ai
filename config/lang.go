package config

import (
	"os"
	"strings"
)

// Lang agrupa todos los textos visibles para el usuario en un idioma.
// Los campos con verbos de formato (%d, %s, %w) se usan con fmt.Printf/Errorf.
type Lang struct {
	// Configuración / setup
	SetupHeader        string
	NoAPIKey           string
	UseAnthropic       string
	UseOpenAI          string
	Optional           string
	CommentModel       string
	CommentSystem      string
	ThenRun            string
	ErrUnknownProvider string

	// Cabecera del chat
	HeaderDir   string
	HeaderFiles string
	ModeRAG     string
	ModeFull    string
	HeaderHelp  string
	NoFilesWarn string
	SkippedWarn string
	EnableEmbed string

	// Menú de comandos
	MenuTitle   string
	MenuIndex   string
	MenuReindex string
	MenuInfo    string
	MenuReload  string
	MenuClear   string
	MenuHelp    string
	MenuExit    string
	MenuHint    string

	// /info
	InfoRAG       string
	InfoEmbed     string
	InfoFull      string
	InfoSkipped   string
	SessionTokens string

	// Tokens por respuesta
	TokensLine3 string
	TokensLine2 string

	// Runtime del chat
	Goodbye        string
	Thinking       string
	Indexing       string
	IndexNeedsKey  string
	IndexUpdated   string
	Reloaded       string
	Reindexed      string
	IndexSynced    string
	HistoryCleared string
	NoNotesFound   string

	// Errores
	ErrReadNotes  string
	ErrReadIndex  string
	ErrIndexing   string
	ErrSaveIndex  string
	ErrReloading  string
	ErrReindexing string
	ErrIndexCmd   string
	ErrGeneric    string
	ErrConfig     string
	ErrCwd        string
	AskUsage      string

	// notes-ai index (CLI)
	IndexNeedsKeyLong string
	AddToZshrc        string
	IndexingNFiles    string
	IndexDone         string
	EmbedTokens       string
	IndexInfoLine     string

	// Uso (--help) y system prompt
	Usage        string
	SystemPrompt string
}

var langEN = Lang{
	SetupHeader:        "Setup",
	NoAPIKey:           "No API key configured.",
	UseAnthropic:       "To use Anthropic (recommended), add to your ~/.zshrc:",
	UseOpenAI:          "To use OpenAI, add to your ~/.zshrc:",
	Optional:           "Optional:",
	CommentModel:       "# specific model",
	CommentSystem:      "# system prompt",
	ThenRun:            "Then run:  source ~/.zshrc",
	ErrUnknownProvider: "unknown provider: %q (use 'anthropic' or 'openai')",

	HeaderDir:   "  │  Dir: %s\n",
	HeaderFiles: "  │  %d files  │  mode: %s\n",
	ModeRAG:     "RAG (embeddings)",
	ModeFull:    "full context",
	HeaderHelp:  "  │  /help for commands  │  Ctrl+C to quit",
	NoFilesWarn: "  Note: no .md, .txt or .json files found here.",
	SkippedWarn: "  Note: %d file(s) skipped due to context limit.\n",
	EnableEmbed: "  Set OPENAI_API_KEY to enable embeddings search (no limit).",

	MenuTitle:   "  Commands:",
	MenuIndex:   "    /index     → index / update notes (RAG)",
	MenuReindex: "    /reindex   → rebuild the index from scratch",
	MenuInfo:    "    /info      → index status / session tokens",
	MenuReload:  "    /reload    → reload and sync changes",
	MenuClear:   "    /clear     → clear conversation history",
	MenuHelp:    "    /help      → show this menu",
	MenuExit:    "    /exit      → quit the program",
	MenuHint:    "  Or just type your question and press Enter.",

	InfoRAG:       "\n  RAG mode  │  %d files indexed, %d chunks\n",
	InfoEmbed:     "  Embedding model: %s  │  top-K: %d\n\n",
	InfoFull:      "\n  Full-context mode  │  %d files loaded\n",
	InfoSkipped:   "\n  %d skipped (context limit):\n",
	SessionTokens: "  Session tokens: %s sent · %s received · %s in searches (%d questions)\n\n",

	TokensLine3: "  ┄ tokens: %s sent · %s received · %s in search\n\n",
	TokensLine2: "  ┄ tokens: %s sent · %s received\n\n",

	Goodbye:        "  Bye.",
	Thinking:       "  thinking...",
	Indexing:       "  indexing...",
	IndexNeedsKey:  "  Indexing requires OPENAI_API_KEY (embeddings). See /help.",
	IndexUpdated:   "  Index updated: %d files, %d chunks (%s embedding tokens).\n\n",
	Reloaded:       "  Reloaded: %d files.\n\n",
	Reindexed:      "  Index rebuilt: %d files (%s embedding tokens).\n\n",
	IndexSynced:    "  Index synced on startup: %d new, %d updated (%s embedding tokens).\n\n",
	HistoryCleared: "  Conversation history cleared.",
	NoNotesFound:   "  No notes (.md, .txt, .json) found in this directory.",

	ErrReadNotes:  "error reading notes: %w",
	ErrReadIndex:  "error reading index: %w",
	ErrIndexing:   "error indexing: %w",
	ErrSaveIndex:  "error saving index: %w",
	ErrReloading:  "  Error reloading: %v\n\n",
	ErrReindexing: "  Error reindexing: %v\n\n",
	ErrIndexCmd:   "  Error indexing: %v\n\n",
	ErrGeneric:    "  Error: %v\n\n",
	ErrConfig:     "Configuration error: %v\n",
	ErrCwd:        "Could not get current directory: %v\n",
	AskUsage:      "Usage: notes-ai ask \"your question here\"",

	IndexNeedsKeyLong: "  Indexing requires an embeddings API key (OpenAI).",
	AddToZshrc:        "  Add to your ~/.zshrc:",
	IndexingNFiles:    "  Indexing %d files...\n",
	IndexDone:         "  Done. New: %d  Updated: %d  Removed: %d  Unchanged: %d\n",
	EmbedTokens:       "  Embedding tokens consumed: %s\n",
	IndexInfoLine:     "  Index: %s (%d files, %d chunks)\n",

	Usage: `
  notes-ai — Ask questions about your notes with AI

  Usage:
    notes-ai                        interactive chat mode
    notes-ai ask "your question"    one-off question, then exit
    notes-ai index                  build / update the index
    notes-ai index --rebuild        rebuild the index from scratch
    notes-ai --provider openai      use OpenAI instead of Anthropic
    notes-ai --model claude-opus-4-8  specific model

  Environment variables:
    ANTHROPIC_API_KEY     your Anthropic API key (answers)
    OPENAI_API_KEY        your OpenAI API key (answers and/or embeddings)
    NOTES_AI_PROVIDER     anthropic (default) | openai
    NOTES_AI_MODEL        answer model name
    NOTES_AI_EMBED_MODEL  embedding model (default: text-embedding-3-small)
    NOTES_AI_TOP_K        relevant chunks per question (default: 8)
    NOTES_AI_MAX_CHARS    context limit in fallback mode
    NOTES_AI_SYSTEM       custom system prompt
    NOTES_AI_LANG         interface language: en | es

  Smart search (RAG):
    If OPENAI_API_KEY is set, notes are indexed with embeddings and each
    question only sends the relevant chunks (fast and cheap).
    Without that key, the full context is used with a character limit.

  Supported files:  .md  .txt  .json
  Read recursively from the current directory.`,

	SystemPrompt: `You are an intelligent assistant with access to the user's notes.
The notes may contain text, task lists, diagrams (Mermaid or other formats), projects, documents, and any kind of information.
Reply in the same language the user asks in.
Use the notes as the main context for your answers. If the answer is not in the notes, say so clearly.
Be concise and helpful.`,
}

var langES = Lang{
	SetupHeader:        "Configuración",
	NoAPIKey:           "No se encontró una API key configurada.",
	UseAnthropic:       "Para usar Anthropic (recomendado), agrega a tu ~/.zshrc:",
	UseOpenAI:          "Para usar OpenAI, agrega a tu ~/.zshrc:",
	Optional:           "Opcionales:",
	CommentModel:       "# modelo específico",
	CommentSystem:      "# system prompt",
	ThenRun:            "Luego ejecuta:  source ~/.zshrc",
	ErrUnknownProvider: "provider desconocido: %q (usa 'anthropic' o 'openai')",

	HeaderDir:   "  │  Dir: %s\n",
	HeaderFiles: "  │  %d archivos  │  modo: %s\n",
	ModeRAG:     "RAG (embeddings)",
	ModeFull:    "contexto completo",
	HeaderHelp:  "  │  /ayuda para comandos  │  Ctrl+C para salir",
	NoFilesWarn: "  Aviso: no se encontraron archivos .md, .txt o .json aquí.",
	SkippedWarn: "  Aviso: %d archivo(s) omitido(s) por límite de contexto.\n",
	EnableEmbed: "  Configura OPENAI_API_KEY para activar búsqueda por embeddings (sin límite).",

	MenuTitle:   "  Comandos:",
	MenuIndex:   "    /index     → indexar / actualizar las notas (RAG)",
	MenuReindex: "    /reindex   → reconstruir el índice desde cero",
	MenuInfo:    "    /info      → estado del índice / tokens de la sesión",
	MenuReload:  "    /reload    → recargar y sincronizar cambios",
	MenuClear:   "    /limpiar   → borrar historial de conversación",
	MenuHelp:    "    /ayuda     → mostrar este menú",
	MenuExit:    "    /salir     → salir del programa",
	MenuHint:    "  O simplemente escribe tu pregunta y presiona Enter.",

	InfoRAG:       "\n  Modo RAG  │  %d archivos indexados, %d fragmentos\n",
	InfoEmbed:     "  Modelo de embeddings: %s  │  top-K: %d\n\n",
	InfoFull:      "\n  Modo contexto completo  │  %d archivos cargados\n",
	InfoSkipped:   "\n  %d omitidos (límite de contexto):\n",
	SessionTokens: "  Tokens de la sesión: %s enviados · %s recibidos · %s en búsquedas (%d preguntas)\n\n",

	TokensLine3: "  ┄ tokens: %s enviados · %s recibidos · %s en búsqueda\n\n",
	TokensLine2: "  ┄ tokens: %s enviados · %s recibidos\n\n",

	Goodbye:        "  Hasta luego.",
	Thinking:       "  pensando...",
	Indexing:       "  indexando...",
	IndexNeedsKey:  "  El indexado requiere OPENAI_API_KEY (embeddings). Ver /ayuda.",
	IndexUpdated:   "  Índice actualizado: %d archivos, %d fragmentos (%s tokens de embeddings).\n\n",
	Reloaded:       "  Recargado: %d archivos.\n\n",
	Reindexed:      "  Índice reconstruido: %d archivos (%s tokens de embeddings).\n\n",
	IndexSynced:    "  Índice sincronizado al inicio: %d nuevos, %d actualizados (%s tokens de embeddings).\n\n",
	HistoryCleared: "  Historial de conversación borrado.",
	NoNotesFound:   "  No se encontraron notas (.md, .txt, .json) en este directorio.",

	ErrReadNotes:  "error leyendo notas: %w",
	ErrReadIndex:  "error leyendo índice: %w",
	ErrIndexing:   "error indexando: %w",
	ErrSaveIndex:  "error guardando índice: %w",
	ErrReloading:  "  Error recargando: %v\n\n",
	ErrReindexing: "  Error reindexando: %v\n\n",
	ErrIndexCmd:   "  Error indexando: %v\n\n",
	ErrGeneric:    "  Error: %v\n\n",
	ErrConfig:     "Error de configuración: %v\n",
	ErrCwd:        "No se pudo obtener el directorio actual: %v\n",
	AskUsage:      "Uso: notes-ai ask \"tu pregunta aquí\"",

	IndexNeedsKeyLong: "  El indexado requiere una API key de embeddings (OpenAI).",
	AddToZshrc:        "  Agrega a tu ~/.zshrc:",
	IndexingNFiles:    "  Indexando %d archivos...\n",
	IndexDone:         "  Listo. Nuevos: %d  Actualizados: %d  Eliminados: %d  Sin cambios: %d\n",
	EmbedTokens:       "  Tokens de embeddings consumidos: %s\n",
	IndexInfoLine:     "  Índice: %s (%d archivos, %d fragmentos)\n",

	Usage: `
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
    NOTES_AI_PROVIDER     anthropic (default) | openai
    NOTES_AI_MODEL        nombre del modelo de respuestas
    NOTES_AI_EMBED_MODEL  modelo de embeddings (def: text-embedding-3-small)
    NOTES_AI_TOP_K        fragmentos relevantes por pregunta (def: 8)
    NOTES_AI_MAX_CHARS    límite de contexto en modo fallback
    NOTES_AI_SYSTEM       system prompt personalizado
    NOTES_AI_LANG         idioma de la interfaz: en | es

  Búsqueda inteligente (RAG):
    Si hay OPENAI_API_KEY, las notas se indexan con embeddings y cada
    pregunta solo envía los fragmentos relevantes (rápido y económico).
    Sin esa key, se usa el contexto completo con un límite de caracteres.

  Archivos soportados:  .md  .txt  .json
  Se leen de forma recursiva desde el directorio actual.`,

	SystemPrompt: `Eres un asistente inteligente que tiene acceso a las notas del usuario.
Las notas pueden contener texto, listas de tareas, diagramas (en formato Mermaid u otros), proyectos, documentos y cualquier tipo de información.
Responde en el mismo idioma en que te pregunten.
Usa las notas como contexto principal para responder. Si la respuesta no está en las notas, dilo claramente.
Sé conciso y útil.`,
}

var active = &langEN

// Tr devuelve el conjunto de textos del idioma activo.
func Tr() *Lang { return active }

// InitLang fija el idioma activo según NOTES_AI_LANG o el locale del sistema.
// Por defecto inglés. Es idempotente y seguro de llamar varias veces.
func InitLang() {
	code := strings.ToLower(strings.TrimSpace(os.Getenv("NOTES_AI_LANG")))
	if code == "" {
		sys := strings.ToLower(os.Getenv("LC_ALL") + os.Getenv("LC_MESSAGES") + os.Getenv("LANG"))
		if strings.HasPrefix(sys, "es") || strings.Contains(sys, "es_") {
			code = "es"
		}
	}
	switch code {
	case "es", "spanish", "español", "espanol":
		active = &langES
	default:
		active = &langEN
	}
}
