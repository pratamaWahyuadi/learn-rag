package ports

import "context"

// LLM is a large language model used for summarization and RAG answering
// (implemented by DeepSeek).
type LLM interface {
	// Summarize produces a summary of text given a system prompt.
	Summarize(ctx context.Context, systemPrompt, text string) (string, error)
	// AnswerQuery answers a user question given a system prompt and user
	// content (e.g. retrieved document context).
	AnswerQuery(ctx context.Context, systemPrompt, userContent string) (string, error)
}
