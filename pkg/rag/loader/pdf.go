package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PDFLoader loads text content from a PDF file using heuristic content stream parsing.
// For production use with complex or encrypted PDFs, consider a dedicated PDF library.
type PDFLoader struct {
	Path string
}

// NewPDFLoader creates a PDFLoader for the given file path.
func NewPDFLoader(path string) *PDFLoader {
	return &PDFLoader{Path: path}
}

// Load reads the PDF file and returns extracted text as a Document.
func (l *PDFLoader) Load() ([]Document, error) {
	return LoadPDF(l.Path)
}

// LoadPDF loads a single PDF file and returns its text content as a Document.
// Text is extracted by parsing PDF content stream operators; works well for
// text-based PDFs. Scanned/image-only PDFs return an error.
func LoadPDF(path string) ([]Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loader: read pdf %q: %w", path, err)
	}

	// Validate PDF magic bytes.
	if len(data) < 4 || string(data[:4]) != "%PDF" {
		return nil, fmt.Errorf("loader: %q is not a valid PDF file", path)
	}

	text := extractPDFText(data)
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("loader: no text extracted from pdf %q (may be scanned or image-only)", path)
	}

	return []Document{{
		ID:      filepath.Base(path),
		Content: text,
		Source:  path,
	}}, nil
}

// reTextTj matches text strings in PDF Tj and ' operators: (text)Tj
var reTextTj = regexp.MustCompile(`\(([^)\\]*(?:\\.[^)\\]*)*)\)\s*(?:Tj|')`)

// reTextArray matches text arrays in TJ operators: [(text)...]TJ
var reTextArray = regexp.MustCompile(`\[((?:[^[\]]*\([^)]*\)[^[\]]*)*)\]\s*TJ`)

// reArrayItem extracts individual strings from a TJ array.
var reArrayItem = regexp.MustCompile(`\(([^)\\]*(?:\\.[^)\\]*)*)\)`)

// extractPDFText uses PDF operator parsing to extract readable text from raw PDF bytes.
func extractPDFText(data []byte) string {
	s := string(data)
	var parts []string

	// Extract Tj operator strings.
	for _, m := range reTextTj.FindAllStringSubmatch(s, -1) {
		if text := unescapePDF(m[1]); text != "" {
			parts = append(parts, text)
		}
	}

	// Extract TJ array operator strings.
	for _, m := range reTextArray.FindAllStringSubmatch(s, -1) {
		for _, item := range reArrayItem.FindAllStringSubmatch(m[1], -1) {
			if text := unescapePDF(item[1]); text != "" {
				parts = append(parts, text)
			}
		}
	}

	return strings.Join(parts, " ")
}

// unescapePDF handles common PDF string escape sequences.
func unescapePDF(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\r`, "\r")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\(`, "(")
	s = strings.ReplaceAll(s, `\)`, ")")
	return strings.TrimSpace(s)
}
