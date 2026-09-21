package parser

import "io"

type Parser interface {
	Parse(r io.Reader) ([]string, error)
}
