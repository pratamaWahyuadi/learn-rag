// Package chunker implements a heuristic flat text chunker that splits content
// into overlapping sentence windows. It is intentionally simple and requires no
// NLP or machine-learning tooling.
package chunker

import (
	"regexp"
	"strings"
)

// sentenceSplitter matches sentence-ending punctuation. The returned sentence
// spans from the previous boundary up to and including this punctuation so that
// sentence boundaries remain detectable in joined chunks.
var sentenceSplitter = regexp.MustCompile(`[.!?]+`)

const (
	// chunkSize is the number of consecutive sentences per chunk (3–4 allowed).
	chunkSize = 3
	// stride is the step between window starts, giving an overlap of 1 sentence
	// (1–2 allowed).
	stride = chunkSize - 1
)

// Chunk splits text into overlapping chunks of ~3 consecutive sentences. Each
// chunk overlaps the previous one by 1 sentence. Empty input yields an empty
// slice rather than an error. Returned chunks are ordered by their index.
func Chunk(text string) []string {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return []string{}
	}

	var chunks []string
	for start := 0; start < len(sentences); start += stride {
		end := start + chunkSize
		if end > len(sentences) {
			end = len(sentences)
		}
		chunks = append(chunks, strings.Join(sentences[start:end], " "))
		if end == len(sentences) {
			break
		}
	}
	return chunks
}

func splitSentences(text string) []string {
	var sentences []string
	start := 0
	for _, idx := range sentenceSplitter.FindAllStringIndex(text, -1) {
		end := idx[1] // index just after the punctuation
		if s := strings.TrimSpace(text[start:end]); s != "" {
			sentences = append(sentences, s)
		}
		start = end
	}
	if rest := strings.TrimSpace(text[start:]); rest != "" {
		sentences = append(sentences, rest)
	}
	return sentences
}
