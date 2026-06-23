package index

import (
	"strings"
	"testing"

	"github.com/4rji/notes-ai/reader"
)

func TestChunkText(t *testing.T) {
	// Texto corto → un solo chunk.
	short := "Hola mundo."
	if got := chunkText(short); len(got) != 1 || got[0] != short {
		t.Fatalf("texto corto: got %v", got)
	}

	// Texto vacío → sin chunks.
	if got := chunkText("   "); got != nil {
		t.Fatalf("texto vacío: got %v", got)
	}

	// Texto largo → múltiples chunks, ninguno descomunal.
	long := strings.Repeat("Párrafo de prueba con contenido.\n\n", 200)
	chunks := chunkText(long)
	if len(chunks) < 2 {
		t.Fatalf("texto largo debería partirse, got %d chunks", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > chunkTarget*2 {
			t.Fatalf("chunk %d demasiado grande: %d chars", i, len(c))
		}
	}
}

func TestCosine(t *testing.T) {
	a := []float32{1, 0, 0}
	if s := cosine(a, a); s < 0.999 {
		t.Fatalf("cosine de vector consigo mismo debería ser ~1, got %v", s)
	}
	b := []float32{0, 1, 0}
	if s := cosine(a, b); s != 0 {
		t.Fatalf("cosine de ortogonales debería ser 0, got %v", s)
	}
	// Dimensiones distintas → 0 sin pánico.
	if s := cosine(a, []float32{1, 2}); s != 0 {
		t.Fatalf("dimensiones distintas deberían dar 0, got %v", s)
	}
}

func TestSyncIncremental(t *testing.T) {
	// embed falso: vector determinista por longitud del texto; reporta tokens.
	fake := func(texts []string) ([][]float32, int, error) {
		out := make([][]float32, len(texts))
		tokens := 0
		for i, tx := range texts {
			out[i] = []float32{float32(len(tx)), 1, 0}
			tokens += len(tx)
		}
		return out, tokens, nil
	}

	idx := &Index{}
	docs := []reader.FileDoc{
		{Path: "a.md", Content: "Contenido A", Hash: "h1"},
		{Path: "b.md", Content: "Contenido B", Hash: "h2"},
	}

	st, err := idx.Sync(docs, "test-model", fake)
	if err != nil {
		t.Fatal(err)
	}
	if st.Added != 2 || st.Unchanged != 0 {
		t.Fatalf("primer sync: %+v", st)
	}

	// Segundo sync sin cambios → todo Unchanged, sin re-embeber.
	st, err = idx.Sync(docs, "test-model", fake)
	if err != nil {
		t.Fatal(err)
	}
	if st.Unchanged != 2 || st.Added != 0 || st.Updated != 0 {
		t.Fatalf("sync sin cambios: %+v", st)
	}

	// Modificar a.md y eliminar b.md.
	docs2 := []reader.FileDoc{
		{Path: "a.md", Content: "Contenido A modificado", Hash: "h1b"},
	}
	st, err = idx.Sync(docs2, "test-model", fake)
	if err != nil {
		t.Fatal(err)
	}
	if st.Updated != 1 || st.Removed != 1 {
		t.Fatalf("sync con cambios: %+v", st)
	}
	if idx.FileCount() != 1 {
		t.Fatalf("debería quedar 1 archivo, got %d", idx.FileCount())
	}
}
