package util

import (
	"reflect"
	"testing"
)

func TestExtractTagOps(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []TagOp
	}{
		{"empty", "", nil},
		{"no tags", "plain text with no tags", nil},
		{"single add", "please review #approve", []TagOp{{"approve", false}}},
		{"single remove", "not ready -#approve", []TagOp{{"approve", true}}},
		{"leading tag", "#group-3 topic", []TagOp{{"group-3", false}}},
		{"multiple", "#group-3 topic #approve", []TagOp{{"group-3", false}, {"approve", false}}},
		{"case folded", "#Approve", []TagOp{{"approve", false}}},
		{"underscore and digits", "#group_3a", []TagOp{{"group_3a", false}}},
		{"newline separated", "#a\n-#b", []TagOp{{"a", false}, {"b", true}}},
		{"markdown h1 not a tag", "# Heading\ntext", nil},
		{"markdown h2 no space not a tag", "##Subheading", nil},
		{"hyphen bullet is add not remove", "- #approve", []TagOp{{"approve", false}}},
		{"midword hash not a tag", "C#and stuff", nil},
		{"remove requires adjacency", "well -#approve after dash", []TagOp{{"approve", true}}},
		{"lone hash no name not a tag", "# ", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTagOps(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractTagOps(%q) = %#v, want %#v", tt.content, got, tt.want)
			}
		})
	}
}
