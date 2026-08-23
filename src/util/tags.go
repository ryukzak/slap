package util

import (
	"regexp"
	"strings"
)

// tagRe matches a #tag or -#tag token. The token must start at the beginning
// of the string or after whitespace, so it doesn't collide with markdown ATX
// headers (which require a space after "#") or with hyphen bullet markers
// (whose "-" isn't immediately followed by "#").
var tagRe = regexp.MustCompile(`(?:^|\s)(-)?#([A-Za-z0-9][A-Za-z0-9_-]*)`)

// TagOp is a single add/remove tag operation extracted from record content.
type TagOp struct {
	Name   string
	Remove bool
}

// ExtractTagOps scans content for #tag and -#tag tokens, in the order they
// appear. Tag names are lowercased so "#Approve" and "#approve" are the same
// tag.
func ExtractTagOps(content string) []TagOp {
	matches := tagRe.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}
	ops := make([]TagOp, 0, len(matches))
	for _, m := range matches {
		ops = append(ops, TagOp{
			Name:   strings.ToLower(m[2]),
			Remove: m[1] == "-",
		})
	}
	return ops
}
