package chunker

import (
	"strconv"
	"strings"
	"testing"
)

// makeText returns a string of n sentence-ending-punctuated sentences.
func makeText(n int) string {
	sentences := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		sentences = append(sentences, "Sentence number "+strconv.Itoa(i)+".")
	}
	return strings.Join(sentences, " ")
}

func splitCount(s string) int {
	return len(splitSentences(s))
}

func TestChunkEmptyTextReturnsEmpty(t *testing.T) {
	chunks := Chunk("")
	if chunks == nil {
		t.Fatal("expected an empty (non-nil) slice, got nil")
	}
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

func TestChunkSentenceCountPerChunk(t *testing.T) {
	// 12 sentences -> non-tail chunks hold 3 sentences each.
	text := makeText(12)
	chunks := Chunk(text)
	if len(chunks) == 0 {
		t.Fatal("expected chunks for 12 sentences")
	}

	for i, c := range chunks {
		count := splitCount(c)
		// Every chunk except the final tail chunk must hold full 3 sentences.
		if i < len(chunks)-1 && count != 3 {
			t.Errorf("chunk[%d] has %d sentences, want 3", i, count)
		}
	}
}

func TestChunkOverlap(t *testing.T) {
	// Chunk size 3, stride 2 => overlap of exactly 1 sentence between
	// consecutive chunks. Verify with a window-aware helper.
	text := makeText(14)
	chunks := Chunk(text)

	for i := 1; i < len(chunks); i++ {
		prev := splitSentences(chunks[i-1])
		cur := splitSentences(chunks[i])
		// The first sentence of the current chunk must equal the last
		// sentence of the previous chunk (1-sentence overlap).
		if cur[0] != prev[len(prev)-1] {
			t.Errorf("chunks[%d-1] and chunks[%d] do not overlap by 1 sentence", i-1, i)
		}
	}
}

func TestChunkIndexOrderPreserved(t *testing.T) {
	text := makeText(20)
	chunks := Chunk(text)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}

	// Every sentence in the original text should appear inside at least one
	// chunk, and document order must be preserved across the chunk sequence.
	// We check that the first sentence of chunk i+1 appears after the first
	// sentence of chunk i in the source.
	firsts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		sents := splitSentences(c)
		if len(sents) > 0 {
			firsts = append(firsts, sents[0])
		}
	}
	for i := 1; i < len(firsts); i++ {
		idxPrev := strings.Index(text, firsts[i-1])
		idxCur := strings.Index(text, firsts[i])
		if idxCur < idxPrev {
			t.Errorf("chunk order not preserved for first sentences %q and %q", firsts[i-1], firsts[i])
		}
	}
}
