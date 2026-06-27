package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func intPtr(v int) *int { return &v }

func TestGetWaitingPeriod(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want time.Duration
	}{
		{"unset defaults to 24h", Task{}, DefaultWaitingPeriod},
		{"explicit 48h", Task{WaitingPeriodHours: intPtr(48)}, 48 * time.Hour},
		{"zero means no threshold", Task{WaitingPeriodHours: intPtr(0)}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.GetWaitingPeriod(); got != tt.want {
				t.Errorf("GetWaitingPeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig_WaitingPeriodHours(t *testing.T) {
	path := writeTempConfig(t, `
tasks:
  - id: "task1"
    title: "Lab 1"
    waiting_period_hours: 48
  - id: "task2"
    title: "Lab 2"
    waiting_period_hours: 0
  - id: "task3"
    title: "Lab 3"
`)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if got := cfg.GetTask("task1").GetWaitingPeriod(); got != 48*time.Hour {
		t.Errorf("task1 waiting period = %v, want 48h", got)
	}
	if got := cfg.GetTask("task2").GetWaitingPeriod(); got != 0 {
		t.Errorf("task2 waiting period = %v, want 0 (no threshold)", got)
	}
	if got := cfg.GetTask("task3").GetWaitingPeriod(); got != DefaultWaitingPeriod {
		t.Errorf("task3 waiting period = %v, want default %v", got, DefaultWaitingPeriod)
	}
}

func TestLoadConfig_NegativeWaitingPeriodRejected(t *testing.T) {
	path := writeTempConfig(t, `
tasks:
  - id: "task1"
    title: "Lab 1"
    waiting_period_hours: -5
`)
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("expected error for negative waiting_period_hours, got nil")
	}
}
