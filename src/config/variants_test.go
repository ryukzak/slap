package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryukzak/slap/src/storage"
)

var variantTasks = map[storage.TaskID]bool{"lab1": true, "lab2": true}

func TestParseVariants(t *testing.T) {
	csv := "id;lab1;lab2\n" +
		"100;7;\n" +
		" 200 ; 3 ; B-2 \n" +
		"300;5\n" +
		"\n"
	got, err := parseVariants(strings.NewReader(csv), variantTasks)
	if err != nil {
		t.Fatalf("parseVariants: %v", err)
	}

	tests := []struct {
		student storage.UserID
		task    storage.TaskID
		want    string
	}{
		{"100", "lab1", "7"},
		{"100", "lab2", ""},
		{"200", "lab1", "3"},
		{"200", "lab2", "B-2"},
		{"300", "lab1", "5"},
		{"300", "lab2", ""},
		{"999", "lab1", ""},
	}
	cfg := &Config{Variants: got}
	for _, tt := range tests {
		if v := cfg.GetVariant(tt.student, tt.task); v != tt.want {
			t.Errorf("GetVariant(%s, %s) = %q, want %q", tt.student, tt.task, v, tt.want)
		}
	}
}

func TestParseVariantsErrors(t *testing.T) {
	tests := []struct {
		name string
		csv  string
		want string
	}{
		{"empty file", "", "empty"},
		{"header without tasks", "id\n100\n", "header"},
		{"unknown task column", "id;lab9\n100;1\n", "non-existent task: lab9"},
		{"duplicate student", "id;lab1\n100;1\n100;2\n", "more than once"},
		{"too many cells", "id;lab1\n100;1;2\n", "more cells"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVariants(strings.NewReader(tt.csv), variantTasks)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadConfigVariantsFileRelativeToConfig(t *testing.T) {
	path := writeTempConfig(t, `
tasks:
  - id: "lab1"
    title: "Lab 1"
variants_file: vars.csv
`)
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "vars.csv"), []byte("id;lab1\n100;7\n"), 0o600); err != nil {
		t.Fatalf("write variants: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if v := cfg.GetVariant("100", "lab1"); v != "7" {
		t.Errorf("GetVariant = %q, want %q", v, "7")
	}
}

func TestLoadConfigVariantsFileMissing(t *testing.T) {
	path := writeTempConfig(t, `
tasks:
  - id: "lab1"
    title: "Lab 1"
variants_file: missing.csv
`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected an error for a missing variants file")
	}
}

func TestGetVariantWithoutVariantsFile(t *testing.T) {
	if v := (&Config{}).GetVariant("100", "lab1"); v != "" {
		t.Errorf("GetVariant = %q, want empty", v)
	}
}
