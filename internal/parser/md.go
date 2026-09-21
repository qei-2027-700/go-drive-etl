package parser

import (
	"io"
	"strings"
	"unicode/utf8"
)

const defaultMaxChunkChars = 4000

// MarkdownParser splits Markdown at headings. A section exceeding the maximum
// size is split into fixed-size character chunks as a fallback.
type MarkdownParser struct{}

var _ Parser = MarkdownParser{}

func (MarkdownParser) Parse(r io.Reader) ([]string, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	sections := splitMarkdownSections(string(content))
	chunks := make([]string, 0, len(sections))
	for _, section := range sections {
		chunks = append(chunks, splitLongChunk(section, defaultMaxChunkChars)...)
	}
	return chunks, nil
}

func splitMarkdownSections(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	sections := make([]string, 0)
	current := make([]string, 0)

	for _, line := range lines {
		if isHeading(line) && len(current) > 0 {
			if section := strings.TrimSpace(strings.Join(current, "\n")); section != "" {
				sections = append(sections, section)
			}
			current = current[:0]
		}
		current = append(current, line)
	}

	if section := strings.TrimSpace(strings.Join(current, "\n")); section != "" {
		sections = append(sections, section)
	}
	return sections
}

func isHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}

	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	return level < len(line) && line[level] == ' '
}

func splitLongChunk(content string, maxChars int) []string {
	if utf8.RuneCountInString(content) <= maxChars {
		return []string{content}
	}

	runes := []rune(content)
	chunks := make([]string, 0, (len(runes)+maxChars-1)/maxChars)
	for start := 0; start < len(runes); start += maxChars {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}
