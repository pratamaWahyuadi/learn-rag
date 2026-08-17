package ports

import "context"

// Embedder generates dense text embeddings in batches (implemented by
// Cloudflare Workers AI BGE-M3). Each returned slice has one embedding per
// input text.
type Embedder interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}
