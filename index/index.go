package index

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/4rji/gov-notes/reader"
)

// EmbedFunc convierte textos en vectores. Lo provee la capa ai para evitar
// que este paquete dependa del cliente HTTP.
type EmbedFunc func(texts []string) ([][]float32, error)

// Chunk es un fragmento de una nota con su vector de embedding.
type Chunk struct {
	File      string    `json:"file"`
	Text      string    `json:"text"`
	Embedding []float32 `json:"embedding"`
}

// FileMeta guarda el hash de un archivo para detectar cambios.
type FileMeta struct {
	Hash string `json:"hash"`
}

// Index es el índice completo, serializado a .gov-notes-index.json.
type Index struct {
	EmbedModel string              `json:"embed_model"`
	Files      map[string]FileMeta `json:"files"`
	Chunks     []Chunk             `json:"chunks"`
}

// Stats resume qué cambió en una sincronización.
type Stats struct {
	Added, Updated, Removed, Unchanged int
}

func (s Stats) Changed() bool { return s.Added+s.Updated+s.Removed > 0 }

// Load lee el índice del disco. Devuelve (nil, nil) si no existe.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// Save escribe el índice al disco.
func (idx *Index) Save(path string) error {
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Sync actualiza el índice para que coincida con docs. Solo re-embebe archivos
// nuevos o modificados (comparando hash). Devuelve estadísticas del cambio.
func (idx *Index) Sync(docs []reader.FileDoc, embedModel string, embed EmbedFunc) (Stats, error) {
	var st Stats

	// Si cambió el modelo de embeddings, reconstruir todo (vectores incompatibles).
	if idx.EmbedModel != "" && idx.EmbedModel != embedModel {
		idx.Files = nil
		idx.Chunks = nil
	}
	idx.EmbedModel = embedModel
	if idx.Files == nil {
		idx.Files = map[string]FileMeta{}
	}

	// Agrupar chunks existentes por archivo para reutilizar los sin cambios.
	existingByFile := map[string][]Chunk{}
	for _, c := range idx.Chunks {
		existingByFile[c.File] = append(existingByFile[c.File], c)
	}

	present := map[string]bool{}
	var newChunks []Chunk

	// Recolectar textos a embeber de archivos nuevos/cambiados en un solo lote.
	type changedDoc struct {
		path  string
		texts []string
	}
	var changed []changedDoc
	var allTexts []string

	for _, doc := range docs {
		present[doc.Path] = true
		meta, ok := idx.Files[doc.Path]
		if ok && meta.Hash == doc.Hash {
			newChunks = append(newChunks, existingByFile[doc.Path]...)
			st.Unchanged++
			continue
		}
		texts := chunkText(doc.Content)
		if len(texts) == 0 {
			idx.Files[doc.Path] = FileMeta{Hash: doc.Hash}
			continue
		}
		changed = append(changed, changedDoc{path: doc.Path, texts: texts})
		allTexts = append(allTexts, texts...)
		idx.Files[doc.Path] = FileMeta{Hash: doc.Hash}
		if ok {
			st.Updated++
		} else {
			st.Added++
		}
	}

	if len(allTexts) > 0 {
		embs, err := embed(allTexts)
		if err != nil {
			return st, err
		}
		k := 0
		for _, cd := range changed {
			for _, t := range cd.texts {
				newChunks = append(newChunks, Chunk{File: cd.path, Text: t, Embedding: embs[k]})
				k++
			}
		}
	}

	// Eliminar del índice archivos que ya no existen.
	for f := range idx.Files {
		if !present[f] {
			delete(idx.Files, f)
			st.Removed++
		}
	}

	idx.Chunks = newChunks
	return st, nil
}

// Search devuelve los topK chunks más similares al vector de la consulta.
func (idx *Index) Search(queryEmb []float32, topK int) []Chunk {
	type scored struct {
		c Chunk
		s float64
	}
	arr := make([]scored, 0, len(idx.Chunks))
	for _, c := range idx.Chunks {
		arr = append(arr, scored{c, cosine(queryEmb, c.Embedding)})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].s > arr[j].s })
	if topK > len(arr) {
		topK = len(arr)
	}
	out := make([]Chunk, topK)
	for i := 0; i < topK; i++ {
		out[i] = arr[i].c
	}
	return out
}

// FileCount devuelve cuántos archivos hay indexados.
func (idx *Index) FileCount() int { return len(idx.Files) }

func cosine(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

const (
	chunkTarget  = 1200 // tamaño objetivo de cada chunk (caracteres)
	chunkOverlap = 150  // solapamiento entre chunks para no perder contexto
)

// chunkText parte el contenido en fragmentos de ~chunkTarget caracteres,
// respetando párrafos cuando es posible y con un poco de solapamiento.
func chunkText(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) <= chunkTarget {
		return []string{s}
	}

	paras := strings.Split(s, "\n\n")
	var chunks []string
	var cur strings.Builder

	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			chunks = append(chunks, strings.TrimSpace(cur.String()))
		}
	}

	for _, p := range paras {
		if cur.Len()+len(p)+2 > chunkTarget && cur.Len() > 0 {
			tail := cur.String()
			flush()
			if len(tail) > chunkOverlap {
				tail = tail[len(tail)-chunkOverlap:]
			}
			cur.Reset()
			cur.WriteString(tail)
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
		cur.WriteString("\n\n")
	}
	flush()

	// Partir a la fuerza cualquier chunk gigante (p. ej. JSON sin párrafos).
	var final []string
	for _, c := range chunks {
		for len(c) > chunkTarget*2 {
			final = append(final, c[:chunkTarget])
			c = c[chunkTarget-chunkOverlap:]
		}
		final = append(final, c)
	}
	return final
}
