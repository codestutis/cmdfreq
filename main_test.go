package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadAliases(t *testing.T) {
	shell := filepath.Join(t.TempDir(), "test-shell")
	contents := "#!/bin/sh\nprintf \"alias g='git'\\nll='ls -la'\\n\"\n"
	if err := os.WriteFile(shell, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := loadAliases(shell)
	if err != nil {
		t.Fatalf("loadAliases() error = %v", err)
	}
	want := map[string]string{"g": "git", "ll": "ls -la"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadAliases() = %#v, want %#v", got, want)
	}
}

func TestLoadAliasesErrors(t *testing.T) {
	if _, err := loadAliases(""); err == nil || !strings.Contains(err.Error(), "SHELL is not set") {
		t.Fatalf("loadAliases(\"\") error = %v", err)
	}

	if _, err := loadAliases(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("loadAliases() succeeded with a missing shell")
	}
}

func TestPrintSummaryAllowsNoCommands(t *testing.T) {
	printSummary(nil, defaultTopN)
}
