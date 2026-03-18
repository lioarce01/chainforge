package splitter

import "strings"

// RecursiveCharacterSplitter splits text by trying a hierarchy of separators:
// paragraphs (\n\n) → lines (\n) → sentences (". ") → words (" ") → characters.
// Each segment that exceeds chunkSize is recursively split with the next separator.
type RecursiveCharacterSplitter struct {
	chunkSize  int
	overlap    int
	separators []string
}

// NewRecursiveCharacterSplitter creates a RecursiveCharacterSplitter.
// It attempts to split on paragraph boundaries first, then progressively finer
// boundaries until chunks fit within chunkSize runes.
func NewRecursiveCharacterSplitter(chunkSize, overlap int) *RecursiveCharacterSplitter {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap < 0 {
		overlap = 0
	}
	return &RecursiveCharacterSplitter{
		chunkSize:  chunkSize,
		overlap:    overlap,
		separators: []string{"\n\n", "\n", ". ", " "},
	}
}

// Split splits text using the separator hierarchy.
func (r *RecursiveCharacterSplitter) Split(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return r.doSplit(text, r.separators)
}

func (r *RecursiveCharacterSplitter) doSplit(text string, seps []string) []string {
	if len([]rune(text)) <= r.chunkSize {
		return []string{text}
	}

	// No more separators: fall back to fixed-size splitting.
	if len(seps) == 0 {
		return NewFixedSizeSplitter(r.chunkSize, r.overlap).Split(text)
	}

	sep := seps[0]
	rest := seps[1:]

	segments := strings.Split(text, sep)

	var out []string
	var buf strings.Builder

	flush := func() {
		chunk := strings.TrimSpace(buf.String())
		buf.Reset()
		if chunk == "" {
			return
		}
		if len([]rune(chunk)) > r.chunkSize {
			// Still too big: recurse with finer separators.
			sub := r.doSplit(chunk, rest)
			out = append(out, sub...)
		} else {
			out = append(out, chunk)
		}
	}

	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}

		var candidate string
		if buf.Len() > 0 {
			candidate = buf.String() + sep + seg
		} else {
			candidate = seg
		}

		if len([]rune(candidate)) > r.chunkSize && buf.Len() > 0 {
			flush()

			// Carry overlap from last chunk if applicable.
			if r.overlap > 0 && len(out) > 0 {
				last := []rune(out[len(out)-1])
				if len(last) > r.overlap {
					buf.WriteString(string(last[len(last)-r.overlap:]))
				} else {
					buf.WriteString(string(last))
				}
			}
		}

		if buf.Len() > 0 {
			buf.WriteString(sep)
		}
		buf.WriteString(seg)
	}

	if buf.Len() > 0 {
		flush()
	}

	return out
}
