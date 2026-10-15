package parser

import (
	"regexp"
	"strconv"
	"strings"
)

// ReflectionLabel describes the title's month and sequence, not an exact period.
type ReflectionLabel struct {
	Month    string
	Sequence int32
}

var reflectionHeading = regexp.MustCompile(`^# ([0-9]{4})(0[1-9]|1[0-2])-([1-9][0-9]*)$`)

// ExtractReflectionLabel reads a YYYYMM-N H1 on the first nonblank line.
// Unrecognized titles remain undated; filenames and body dates are not fallbacks.
func ExtractReflectionLabel(content []byte) *ReflectionLabel {
	text := strings.TrimPrefix(string(content), "\ufeff")
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Keep leading indentation: an indented code block is not a title.
		match := reflectionHeading.FindStringSubmatch(strings.TrimRight(line, " \t\r"))
		if match == nil || match[1] == "0000" {
			return nil
		}
		sequence, err := strconv.ParseInt(match[3], 10, 32)
		if err != nil {
			return nil
		}
		return &ReflectionLabel{Month: match[1] + "-" + match[2], Sequence: int32(sequence)}
	}
	return nil
}
