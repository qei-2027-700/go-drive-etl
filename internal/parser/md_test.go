package parser

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMarkdownParserParse_SplitsAtHeadings(t *testing.T) {
	input := "# Overview\nSummary.\n\n## Steps\nDo this.\n\n## Notes\nBe careful.\n"

	chunks, err := (MarkdownParser{}).Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []string{
		"# Overview\nSummary.",
		"## Steps\nDo this.",
		"## Notes\nBe careful.",
	}
	if len(chunks) != len(want) {
		t.Fatalf("len(chunks) = %d, want %d: %#v", len(chunks), len(want), chunks)
	}
	for i := range want {
		if chunks[i] != want[i] {
			t.Errorf("chunks[%d] = %q, want %q", i, chunks[i], want[i])
		}
	}
}

func TestMarkdownParserParse_SplitsLongSectionsWithoutBreakingRunes(t *testing.T) {
	input := "# Long\n" + strings.Repeat("あ", defaultMaxChunkChars)

	chunks, err := (MarkdownParser{}).Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}
	for i, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Errorf("chunks[%d] is not valid UTF-8", i)
		}
		if utf8.RuneCountInString(chunk) > defaultMaxChunkChars {
			t.Errorf("chunks[%d] exceeds limit: %d runes", i, utf8.RuneCountInString(chunk))
		}
	}
	if chunks[0] != "# Long\n"+strings.Repeat("あ", defaultMaxChunkChars-7) {
		t.Errorf("first chunk did not preserve the heading and content")
	}
}

func TestMarkdownParserParse_EmptyContentReturnsNoChunks(t *testing.T) {
	chunks, err := (MarkdownParser{}).Parse(strings.NewReader(" \n\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("chunks = %#v, want no chunks", chunks)
	}
}
