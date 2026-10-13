package main

import (
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaintenanceAliases(t *testing.T) {
	for _, alias := range []string{"--update", "-u", "--version", "-v"} {
		for _, scenario := range []string{"plain", "target", "extra", "history", "both"} {
			t.Run(alias+"/"+scenario, func(t *testing.T) {
				fs := flag.NewFlagSet("test", flag.ContinueOnError)
				fs.SetOutput(io.Discard)
				var updating, showing bool
				fs.BoolVar(&updating, "update", false, "")
				fs.BoolVar(&updating, "u", false, "")
				fs.BoolVar(&showing, "version", false, "")
				fs.BoolVar(&showing, "v", false, "")
				fs.Int("n", 20, "")
				fs.Bool("resolve-aliases", false, "")
				args := []string{alias}
				switch scenario {
				case "target":
					args = append(args, "0.4.0")
				case "extra":
					args = append(args, "0.4.0", "git")
				case "history":
					args = append(args, "-n", "20")
				case "both":
					args = append(args, "--update", "--version")
				}
				if err := fs.Parse(args); err != nil {
					t.Fatal(err)
				}
				target, err := maintenanceMode(fs, updating, showing)
				wantError := scenario == "extra" || scenario == "history" || scenario == "both" || (scenario == "target" && showing)
				if (err != nil) != wantError {
					t.Fatalf("error=%v, want error %v", err, wantError)
				}
				if !wantError && updating {
					want := "latest"
					if scenario == "target" {
						want = "v0.4.0"
					}
					if target != want {
						t.Fatalf("target=%s", target)
					}
				}
			})
		}
	}
}

func TestVersionBinary(t *testing.T) {
	for _, version := range []string{"dev", "1.2.3"} {
		t.Run(version, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "cmdfreq")
			args := []string{"build", "-o", binary}
			if version != "dev" {
				args = append(args, "-ldflags", "-X main.version="+version)
			}
			args = append(args, ".")
			if output, err := exec.Command("go", args...).CombinedOutput(); err != nil {
				t.Fatalf("build: %v\n%s", err, output)
			}
			for _, alias := range []string{"--version", "-v"} {
				cmd := exec.Command(binary, alias)
				cmd.Env = append(cleanEnvironment("HISTFILE", "SHELL"), "HISTFILE="+filepath.Join(t.TempDir(), "missing"), "SHELL=")
				output, err := cmd.CombinedOutput()
				if err != nil || string(output) != "cmdfreq "+version+"\n" {
					t.Fatalf("version: %v %q", err, output)
				}
			}
			for _, alias := range []string{"--update", "-u"} {
				cmd := exec.Command(binary, alias, "invalid")
				cmd.Env = append(os.Environ(), "HISTFILE=/missing", "SHELL=")
				output, err := cmd.CombinedOutput()
				if err == nil || !strings.Contains(string(output), "invalid version") {
					t.Fatalf("update: %v %q", err, output)
				}
			}
		})
	}
}
