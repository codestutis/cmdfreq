package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHistoryFilePath(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		home    string
		homeErr error
		want    string
		wantErr string
	}{
		{
			name: "explicit HISTFILE takes precedence",
			env:  map[string]string{"HISTFILE": "/custom/history", "SHELL": "/bin/fish"},
			want: "/custom/history",
		},
		{
			name: "bash default",
			env:  map[string]string{"SHELL": "/bin/bash"},
			home: "/home/tester",
			want: "/home/tester/.bash_history",
		},
		{
			name: "zsh default",
			env:  map[string]string{"SHELL": "/usr/local/bin/zsh"},
			home: "/Users/tester",
			want: "/Users/tester/.zsh_history",
		},
		{
			name:    "unsupported shell",
			env:     map[string]string{"SHELL": "/usr/bin/fish"},
			wantErr: "HISTFILE is not set and SHELL is not supported",
		},
		{
			name:    "home lookup failure",
			env:     map[string]string{"SHELL": "/bin/bash"},
			homeErr: errors.New("no user record"),
			wantErr: "find home directory: no user record",
		},
		{
			name:    "empty home",
			env:     map[string]string{"SHELL": "/bin/zsh"},
			wantErr: "find home directory: empty path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string { return tt.env[key] }
			userHomeDir := func() (string, error) { return tt.home, tt.homeErr }
			got, err := historyFilePath(getenv, userHomeDir)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("historyFilePath() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("historyFilePath() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("historyFilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenHistoryFile(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(historyPath, []byte("ls\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	getenv := func(key string) string {
		if key == "HISTFILE" {
			return historyPath
		}
		return ""
	}
	file, err := openHistoryFile(getenv, func() (string, error) {
		return "", errors.New("home lookup should not be called")
	})
	if err != nil {
		t.Fatalf("openHistoryFile() error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenHistoryFileReportsMissingFile(t *testing.T) {
	home := t.TempDir()
	getenv := func(key string) string {
		if key == "SHELL" {
			return "/bin/bash"
		}
		return ""
	}
	_, err := openHistoryFile(getenv, func() (string, error) { return home, nil })
	if err == nil || !strings.Contains(err.Error(), filepath.Join(home, ".bash_history")) {
		t.Fatalf("openHistoryFile() error = %v, want missing history path", err)
	}
}

func TestOpenHistoryFileReportsResolutionError(t *testing.T) {
	getenv := func(string) string { return "" }
	_, err := openHistoryFile(getenv, func() (string, error) {
		return "", errors.New("home lookup should not be called")
	})
	if err == nil || !strings.Contains(err.Error(), "HISTFILE is not set and SHELL is not supported") {
		t.Fatalf("openHistoryFile() error = %v, want unsupported shell error", err)
	}
}

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
