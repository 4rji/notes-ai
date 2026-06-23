package reader

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize = 500 * 1024 // 500KB

// IndexFileName es el archivo donde vive el índice de embeddings.
// Se ignora al leer notas para no indexarse a sí mismo.
const IndexFileName = ".gov-notes-index.json"

var allowedExts = map[string]bool{
	".md":   true,
	".txt":  true,
	".json": true,
}

var ignoreDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".cache":       true,
	"vendor":       true,
	".venv":        true,
	"__pycache__":  true,
}

// FileDoc es una nota individual con metadatos para detectar cambios.
type FileDoc struct {
	Path    string // ruta relativa al directorio base
	Content string
	ModTime int64
	Size    int64
	Hash    string // sha256 truncado del contenido
}

// LoadFiles recorre dir recursivamente y devuelve cada nota como FileDoc.
func LoadFiles(dir string) ([]FileDoc, error) {
	var docs []FileDoc

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // saltar lo que no se pueda leer
		}
		if d.IsDir() {
			if ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == IndexFileName {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !allowedExts[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > maxFileSize {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		sum := sha256.Sum256(content)
		docs = append(docs, FileDoc{
			Path:    relPath,
			Content: string(content),
			ModTime: info.ModTime().Unix(),
			Size:    info.Size(),
			Hash:    hex.EncodeToString(sum[:8]),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

// BuildContext concatena documentos en un solo bloque, respetando un límite
// de caracteres (modo fallback cuando RAG está deshabilitado).
func BuildContext(docs []FileDoc, maxChars int) (context string, included []string, skipped []string) {
	var sb strings.Builder
	for _, d := range docs {
		block := fmt.Sprintf("\n=== FILE: ./%s ===\n%s\n", d.Path, d.Content)
		if maxChars > 0 && sb.Len()+len(block) > maxChars {
			skipped = append(skipped, d.Path)
			continue
		}
		sb.WriteString(block)
		included = append(included, d.Path)
	}
	return sb.String(), included, skipped
}

func FormatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%dKB", bytes/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(bytes)/1024/1024)
}
