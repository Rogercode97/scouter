package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/MichaelAyles/goformer"
)

type Embedder interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

type SemanticEngine struct {
	model *goformer.Model
}

func NewSemanticEngine() (*SemanticEngine, error) {
	se := &SemanticEngine{}
	err := se.Init(context.Background(), "")
	return se, err
}

func (s *SemanticEngine) Init(ctx context.Context, modelPath string) error {
	if modelPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		modelPath = filepath.Join(homeDir, ".scouter", "models", "bge-small-en-v1.5")
	}

	if err := s.ensureModelDownloaded(ctx, modelPath); err != nil {
		return fmt.Errorf("failed to ensure model is downloaded: %w", err)
	}

	model, err := goformer.Load(modelPath)
	if err != nil {
		return fmt.Errorf("failed to load model from %s: %w", modelPath, err)
	}
	s.model = model
	return nil
}

func (s *SemanticEngine) ensureModelDownloaded(ctx context.Context, modelPath string) error {
	/* #nosec G101 */
	files := map[string]string{
		"model.safetensors": "https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/main/model.safetensors",
		"config.json":       "https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/main/config.json",
		"tokenizer.json":    "https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/main/tokenizer.json",
	}

	var needsDownload bool
	for file := range files {
		path := filepath.Join(modelPath, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			needsDownload = true
			break
		}
	}

	if !needsDownload {
		return nil
	}

	fmt.Printf("Model weights not found. Scouter is downloading the neural brain to %s...\n", modelPath)

	if err := os.MkdirAll(modelPath, 0755); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	for file, url := range files {
		path := filepath.Join(modelPath, file)
		if _, err := os.Stat(path); err == nil {
			continue // skip already downloaded
		}
		fmt.Printf("Downloading %s...\n", file)
		if err := downloadFile(ctx, url, path); err != nil {
			return fmt.Errorf("failed to download %s: %w", file, err)
		}
	}

	fmt.Println("Neural brain downloaded successfully!")
	return nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (s *SemanticEngine) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if s.model == nil {
		return nil, fmt.Errorf("SemanticEngine not initialized")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	embedding, err := s.model.Embed(text)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	return embedding, nil
}
