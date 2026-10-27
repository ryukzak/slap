package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/ryukzak/slap/src/config"
)

func TestHasReportToken(t *testing.T) {
	const token = "0123456789abcdef0123456789abcdef"
	prev := AppConfig
	t.Cleanup(func() { AppConfig = prev })

	tests := []struct {
		name       string
		configured string
		header     string
		want       bool
	}{
		{"matching bearer", token, "Bearer " + token, true},
		{"wrong token", token, "Bearer " + token + "x", false},
		{"missing header", token, "", false},
		{"no bearer prefix", token, token, false},
		{"not configured", "", "Bearer ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AppConfig = &config.Config{ReportToken: tt.configured}
			r := httptest.NewRequest("GET", "/users/csv", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			if got := hasReportToken(r); got != tt.want {
				t.Errorf("hasReportToken() = %v, want %v", got, tt.want)
			}
		})
	}
}
