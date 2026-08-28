package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	want := Config{RefreshInterval: 30 * time.Second, BotLogin: "coderabbitai", PRLimit: 50}
	if got := Default(); got != want {
		t.Errorf("Default() = %+v, want %+v", got, want)
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error = %v", err)
	}
	if cfg != Default() {
		t.Errorf("Load(nil) = %+v, want defaults %+v", cfg, Default())
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := Load([]string{"-refresh", "1m", "-bot", "mybot", "-limit", "10"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := Config{RefreshInterval: time.Minute, BotLogin: "mybot", PRLimit: 10}
	if cfg != want {
		t.Errorf("Load() = %+v, want %+v", cfg, want)
	}
}

func TestLoad_RefreshBelowMinimum(t *testing.T) {
	if _, err := Load([]string{"-refresh", "1s"}); err == nil {
		t.Fatal("Load() with -refresh 1s: want an error, got nil")
	}
}

func TestLoad_RefreshAtMinimum(t *testing.T) {
	cfg, err := Load([]string{"-refresh", "5s"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RefreshInterval != 5*time.Second {
		t.Errorf("RefreshInterval = %v, want 5s", cfg.RefreshInterval)
	}
}

func TestLoad_InvalidFlag(t *testing.T) {
	if _, err := Load([]string{"-nonexistent"}); err == nil {
		t.Fatal("Load() with an unknown flag: want an error, got nil")
	}
}
