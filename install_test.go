package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	configBegin = "# >>> cmdfreq >>>"
	configEnd   = "# <<< cmdfreq <<<"
)

func TestInstallerConfiguresBashIdempotently(t *testing.T) {
	fixture := newInstallerFixture(t, "bash")
	fixture.binDir = filepath.Join(fixture.home, "custom bin's")
	fixture.configHome = filepath.Join(fixture.home, "config root's")

	bashrc := filepath.Join(fixture.home, ".bashrc")
	bashProfile := filepath.Join(fixture.home, ".bash_profile")
	profile := filepath.Join(fixture.home, ".profile")
	writeTestFile(t, bashrc, "export EXISTING_BASHRC=1\n", 0o644)
	writeTestFile(t, bashProfile, "export EXISTING_PROFILE=1\n", 0o644)
	writeTestFile(t, profile, "leave this profile alone\n", 0o644)

	fixture.run(t)
	firstBashrc := readTestFile(t, bashrc)
	firstBashProfile := readTestFile(t, bashProfile)
	configPath := filepath.Join(fixture.configHome, "cmdfreq", "bashrc")
	firstConfig := readTestFile(t, configPath)

	assertManagedOnce(t, firstBashrc)
	assertManagedOnce(t, firstBashProfile)
	if got := readTestFile(t, profile); got != "leave this profile alone\n" {
		t.Fatalf(".profile was changed despite .bash_profile taking precedence: %q", got)
	}
	if got := readTestFile(t, bashrc+".cmdfreq.bak"); got != "export EXISTING_BASHRC=1\n" {
		t.Fatalf(".bashrc backup = %q", got)
	}
	if got := readTestFile(t, bashProfile+".cmdfreq.bak"); got != "export EXISTING_PROFILE=1\n" {
		t.Fatalf(".bash_profile backup = %q", got)
	}
	if !strings.Contains(firstConfig, "unset HISTCONTROL") ||
		!strings.Contains(firstConfig, "shopt -s histappend") ||
		!strings.Contains(firstConfig, "__cmdfreq_history_sync") {
		t.Fatalf("bash config is missing history integration:\n%s", firstConfig)
	}
	if _, err := os.Stat(filepath.Join(fixture.binDir, "cmdfreq")); err != nil {
		t.Fatalf("installed binary: %v", err)
	}

	fixture.run(t)
	if got := readTestFile(t, bashrc); got != firstBashrc {
		t.Fatal("second install changed .bashrc")
	}
	if got := readTestFile(t, bashProfile); got != firstBashProfile {
		t.Fatal("second install changed .bash_profile")
	}
	if got := readTestFile(t, configPath); got != firstConfig {
		t.Fatal("second install changed generated bash config")
	}
	if got := readTestFile(t, bashrc+".cmdfreq.bak"); got != "export EXISTING_BASHRC=1\n" {
		t.Fatalf("second install overwrote .bashrc backup: %q", got)
	}

	assertBashConfigBehavior(t, bashrc, fixture.home, fixture.binDir)
}

func TestInstallerCreatesMissingBashStartupFiles(t *testing.T) {
	fixture := newInstallerFixture(t, "bash")
	fixture.run(t)

	for _, name := range []string{".bashrc", ".bash_profile"} {
		path := filepath.Join(fixture.home, name)
		assertManagedOnce(t, readTestFile(t, path))
		if _, err := os.Stat(path + ".cmdfreq.bak"); !os.IsNotExist(err) {
			t.Fatalf("backup for newly created %s exists or could not be checked: %v", name, err)
		}
	}
}

func TestInstallerUsesDefaultPaths(t *testing.T) {
	fixture := newInstallerFixture(t, "bash")
	fixture.binDir = ""
	fixture.configHome = ""
	fixture.run(t)

	if _, err := os.Stat(filepath.Join(fixture.home, ".local", "bin", "cmdfreq")); err != nil {
		t.Fatalf("binary was not installed in the default directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fixture.home, ".config", "cmdfreq", "bashrc")); err != nil {
		t.Fatalf("shell config was not installed in the default directory: %v", err)
	}
}

func TestInstallerSelectsExistingBashLoginFile(t *testing.T) {
	fixture := newInstallerFixture(t, "bash")
	bashLogin := filepath.Join(fixture.home, ".bash_login")
	profile := filepath.Join(fixture.home, ".profile")
	writeTestFile(t, bashLogin, "existing bash login\n", 0o644)
	writeTestFile(t, profile, "existing profile\n", 0o644)

	fixture.run(t)

	assertManagedOnce(t, readTestFile(t, bashLogin))
	if got := readTestFile(t, profile); got != "existing profile\n" {
		t.Fatalf(".profile was changed despite .bash_login taking precedence: %q", got)
	}
	if _, err := os.Stat(filepath.Join(fixture.home, ".bash_profile")); !os.IsNotExist(err) {
		t.Fatalf(".bash_profile was unexpectedly created: %v", err)
	}
}

func TestInstallerConfiguresZshWithZDOTDIR(t *testing.T) {
	fixture := newInstallerFixture(t, "zsh")
	fixture.zdotdir = filepath.Join(fixture.home, "zsh config")
	fixture.run(t)

	rcPath := filepath.Join(fixture.zdotdir, ".zshrc")
	firstRC := readTestFile(t, rcPath)
	configPath := filepath.Join(fixture.configHome, "cmdfreq", "zshrc")
	firstConfig := readTestFile(t, configPath)
	assertManagedOnce(t, firstRC)
	for _, want := range []string{
		"export HISTFILE=\"${HISTFILE:-$HOME/.zsh_history}\"",
		"setopt INC_APPEND_HISTORY",
		"setopt SHARE_HISTORY",
	} {
		if !strings.Contains(firstConfig, want) {
			t.Fatalf("zsh config does not contain %q:\n%s", want, firstConfig)
		}
	}
	if _, err := os.Stat(rcPath + ".cmdfreq.bak"); !os.IsNotExist(err) {
		t.Fatalf("backup for newly created .zshrc exists or could not be checked: %v", err)
	}

	fixture.run(t)
	if got := readTestFile(t, rcPath); got != firstRC {
		t.Fatal("second install changed .zshrc")
	}
	if got := readTestFile(t, configPath); got != firstConfig {
		t.Fatal("second install changed generated zsh config")
	}

	if zsh, err := exec.LookPath("zsh"); err == nil {
		cmd := exec.Command(zsh, "-n", rcPath, configPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("zsh config syntax error: %v\n%s", err, output)
		}
	}
}

func TestInstallerRejectsInvalidEnvironmentBeforeDownload(t *testing.T) {
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path := os.Getenv("PATH")
	tests := []struct {
		name    string
		env     []string
		wantErr string
	}{
		{
			name:    "missing HOME",
			env:     []string{"PATH=" + path, "SHELL=/bin/bash"},
			wantErr: "HOME is required",
		},
		{
			name:    "missing SHELL",
			env:     []string{"PATH=" + path, "HOME=" + t.TempDir(), "SHELL="},
			wantErr: "SHELL is required",
		},
		{
			name:    "unsupported shell",
			env:     []string{"PATH=" + path, "HOME=" + t.TempDir(), "SHELL=/usr/bin/fish"},
			wantErr: "unsupported shell: fish",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("sh", filepath.Join(repoDir, "install.sh"))
			cmd.Dir = repoDir
			cmd.Env = tt.env
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("installer succeeded; output:\n%s", output)
			}
			if !strings.Contains(string(output), tt.wantErr) {
				t.Fatalf("installer output = %q, want error containing %q", output, tt.wantErr)
			}
		})
	}
}

func assertBashConfigBehavior(t *testing.T, startupPath, home, binDir string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}

	script := "\n" +
		"set -u\n" +
		"PATH=/usr/bin:/bin\n" +
		"unset HISTFILE\n" +
		"HISTSIZE=1\n" +
		"HISTFILESIZE=2\n" +
		"HISTCONTROL=ignoreboth\n" +
		"PROMPT_COMMAND=existing_command\n" +
		". \"$1\"\n" +
		". \"$1\"\n" +
		"shopt -q histappend\n" +
		"printf '%s\\n' \"$PATH\" \"$HISTFILE\" \"$HISTSIZE\" \"$HISTFILESIZE\" \"${HISTCONTROL-unset}\" \"$PROMPT_COMMAND\"\n"
	cmd := exec.Command(bash, "--noprofile", "--norc", "-c", script, "bash", startupPath)
	cmd.Env = append(cleanEnvironment("HOME", "PATH", "HISTFILE", "HISTSIZE", "HISTFILESIZE", "PROMPT_COMMAND"), "HOME="+home)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("source generated bash config: %v\n%s", err, output)
	}
	want := []string{
		binDir + ":/usr/bin:/bin",
		filepath.Join(home, ".bash_history"),
		"10000",
		"10000",
		"unset",
		"__cmdfreq_history_sync;existing_command",
	}
	if got := strings.Split(strings.TrimSpace(string(output)), "\n"); !equalStrings(got, want) {
		t.Fatalf("bash config values = %#v, want %#v", got, want)
	}

	arrayScript := "\n" +
		"HISTFILE=/custom/history\n" +
		"HISTSIZE=20000\n" +
		"HISTFILESIZE=30000\n" +
		"PROMPT_COMMAND=(existing_command)\n" +
		". \"$1\"\n" +
		". \"$1\"\n" +
		"printf '%s\\n' \"${#PROMPT_COMMAND[@]}\" \"${PROMPT_COMMAND[0]}\" \"${PROMPT_COMMAND[1]}\" \"$HISTFILE\" \"$HISTSIZE\" \"$HISTFILESIZE\"\n"
	cmd = exec.Command(bash, "--noprofile", "--norc", "-c", arrayScript, "bash", startupPath)
	cmd.Env = append(cleanEnvironment("HOME"), "HOME="+home)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("source bash config with PROMPT_COMMAND array: %v\n%s", err, output)
	}
	if got, want := strings.Split(strings.TrimSpace(string(output)), "\n"), []string{"2", "existing_command", "__cmdfreq_history_sync", "/custom/history", "20000", "30000"}; !equalStrings(got, want) {
		t.Fatalf("PROMPT_COMMAND array = %#v, want %#v", got, want)
	}
}

type installerFixture struct {
	repoDir    string
	root       string
	home       string
	configHome string
	binDir     string
	fakeBin    string
	archive    string
	shell      string
	zdotdir    string
}

func newInstallerFixture(t *testing.T, shell string) *installerFixture {
	t.Helper()
	repoDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	fixture := &installerFixture{
		repoDir:    repoDir,
		root:       root,
		home:       filepath.Join(root, "home"),
		configHome: filepath.Join(root, "config"),
		binDir:     filepath.Join(root, "home", ".local", "bin"),
		fakeBin:    filepath.Join(root, "fake-bin"),
		archive:    filepath.Join(root, "cmdfreq.tar.gz"),
		shell:      shell,
	}
	if err := os.MkdirAll(fixture.fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fixture.home, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestArchive(t, fixture.archive)
	curl := filepath.Join(fixture.fakeBin, "curl")
	curlScript := "#!/bin/sh\n" +
		"out=\n" +
		"while [ \"$#\" -gt 0 ]; do\n" +
		"  if [ \"$1\" = \"-o\" ]; then\n" +
		"    shift\n" +
		"    out=\"$1\"\n" +
		"  fi\n" +
		"  shift\n" +
		"done\n" +
		"cp \"$CMDFREQ_TEST_ARCHIVE\" \"$out\"\n"
	writeTestFile(t, curl, curlScript, 0o755)
	return fixture
}

func (f *installerFixture) run(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("sh", filepath.Join(f.repoDir, "install.sh"))
	cmd.Dir = f.repoDir
	env := cleanEnvironment("HOME", "SHELL", "XDG_CONFIG_HOME", "BIN_DIR", "ZDOTDIR", "PATH")
	env = append(env,
		"HOME="+f.home,
		"SHELL=/bin/"+f.shell,
		"PATH="+f.fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CMDFREQ_TEST_ARCHIVE="+f.archive,
	)
	if f.configHome != "" {
		env = append(env, "XDG_CONFIG_HOME="+f.configHome)
	}
	if f.binDir != "" {
		env = append(env, "BIN_DIR="+f.binDir)
	}
	if f.zdotdir != "" {
		env = append(env, "ZDOTDIR="+f.zdotdir)
	}
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installer failed: %v\n%s", err, output)
	}
	return string(output)
}

func createTestArchive(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	contents := []byte("#!/bin/sh\necho cmdfreq test binary\n")
	header := &tar.Header{Name: "cmdfreq", Mode: 0o755, Size: int64(len(contents))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(contents); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func cleanEnvironment(remove ...string) []string {
	removed := make(map[string]bool, len(remove))
	for _, key := range remove {
		removed[key] = true
	}
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if !removed[key] {
			env = append(env, item)
		}
	}
	return env
}

func assertManagedOnce(t *testing.T, contents string) {
	t.Helper()
	if count := strings.Count(contents, configBegin); count != 1 {
		t.Fatalf("start marker count = %d, want 1:\n%s", count, contents)
	}
	if count := strings.Count(contents, configEnd); count != 1 {
		t.Fatalf("end marker count = %d, want 1:\n%s", count, contents)
	}
}

func writeTestFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func equalStrings(left, right []string) bool {
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}
