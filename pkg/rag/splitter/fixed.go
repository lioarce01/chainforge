package splitter

// FixedSizeSplitter splits text into fixed-size chunks with optional overlap.
// Chunk size and overlap are measured in Unicode code points (runes).
type FixedSizeSplitter struct {
	chunkSize int
	overlap   int
}

// NewFixedSizeSplitter creates a FixedSizeSplitter.
// chunkSize is the maximum chunk length in runes; overlap is how many runes
// the next chunk repeats from the end of the previous chunk.
func NewFixedSizeSplitter(chunkSize, overlap int) *FixedSizeSplitter {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap < 0 {
		overlap = 0
	}
	return &FixedSizeSplitter{chunkSize: chunkSize, overlap: overlap}
}

// Split splits text into fixed-size chunks with overlap.
func (f *FixedSizeSplitter) Split(text string) []string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return nil
	}
	if n <= f.chunkSize {
		return []string{text}
	}

	step := f.chunkSize - f.overlap
	if step <= 0 {
		step = 1
	}

	var chunks []string
	for start := 0; start < n; start += step {
		end := start + f.chunkSize
		if end > n {
			end = n
		}
		chunks = append(chunks, string(runes[start:end]))
		if end == n {
			break
		}
	}
	return chunks
}
