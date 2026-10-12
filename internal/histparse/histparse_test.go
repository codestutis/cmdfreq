package histparse

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestParseHistoryCommands(t *testing.T) {
	history := strings.Join([]string{
		"ls | grep file.txt",
		"cd repo && git status",
		"cat log | grep error | sort | uniq",
		"printf done |& tee output || echo failed; pwd",
	}, "\n")

	entries, err := ParseHistory(strings.NewReader(history))
	if err != nil {
		t.Fatalf("ParseHistory() error = %v", err)
	}

	want := [][]string{
		{"ls"},
		{"grep", "file.txt"},
		{"cd", "repo"},
		{"git", "status"},
		{"cat", "log"},
		{"grep", "error"},
		{"sort"},
		{"uniq"},
		{"printf", "done"},
		{"tee", "output"},
		{"echo", "failed"},
		{"pwd"},
	}
	assertCommands(t, entries, want)
}

func TestParseHistoryFiltersPathsAndAssignments(t *testing.T) {
	history := strings.Join([]string{
		"./script",
		"../tool",
		"/usr/bin/python app.py",
		`\~/bin/foo`,
		"FOO=bar npm test",
		"FOO=bar",
		"FOO=bar BAR=baz make test && ./script | grep x",
		"NOT-AN-ASSIGNMENT=value arg",
	}, "\n")

	entries, err := ParseHistory(strings.NewReader(history))
	if err != nil {
		t.Fatalf("ParseHistory() error = %v", err)
	}

	want := [][]string{
		{"npm", "test"},
		{"make", "test"},
		{"grep", "x"},
		{"NOT-AN-ASSIGNMENT=value", "arg"},
	}
	assertCommands(t, entries, want)
}

func TestParseHistoryWithCommandsInEnvAssignment(t *testing.T) {
	// Commands run inside command substitutions must be counted too.
	history := strings.Join([]string{
		// substitution used as an argument to another command
		`echo $(date)`,
		// substitution assignment followed by a real command
		`RESULT=$(git status) npm test`,
		// nested substitutions
		`echo $(basename $(pwd))`,
		// several substitutions on one line
		`a=$(foo) b=$(bar)`,
		// separators inside a substitution must not split the outer command
		`VERSION=$(git describe --tags; echo done)`,
		// backticks are equivalent to $(...)
		"STAMP=`date +%s`",
		// arithmetic expansion is not a command substitution
		`x=$((1+2))`,
		// single quotes suppress command substitution
		`echo '$(not-run)'`,
	}, "\n")

	entries, err := ParseHistory(strings.NewReader(history))
	if err != nil {
		t.Fatalf("ParseHistory() error = %v", err)
	}

	want := [][]string{
		{"date"},
		{"echo"},
		{"git", "status"},
		{"npm", "test"},
		{"pwd"},
		{"basename"},
		{"echo"},
		{"foo"},
		{"bar"},
		{"git", "describe", "--tags"},
		{"echo", "done"},
		{"date", "+%s"},
		{"echo", "$(not-run)"},
	}
	assertCommands(t, entries, want)
}

func TestParseHistorySkipsEmptyAndInvalidSegments(t *testing.T) {
	history := strings.Join([]string{
		"",
		"; ;",
		"ls || ; && grep x | | sort;",
		`echo "unterminated`,
		": malformed extended history",
	}, "\n")

	entries, err := ParseHistory(strings.NewReader(history))
	if err != nil {
		t.Fatalf("ParseHistory() error = %v", err)
	}

	assertCommands(t, entries, [][]string{{"ls"}, {"grep", "x"}, {"sort"}})
}

func TestParseHistoryPreservesQuotedAndEscapedSeparators(t *testing.T) {
	history := strings.Join([]string{
		`echo "a|b" 'c;d'`,
		`printf a\|b`,
	}, "\n")

	entries, err := ParseHistory(strings.NewReader(history))
	if err != nil {
		t.Fatalf("ParseHistory() error = %v", err)
	}

	assertCommands(t, entries, [][]string{
		{"echo", "a|b", "c;d"},
		{"printf", "a|b"},
	})
}

func TestParseExtendedAndMultilineHistory(t *testing.T) {
	history := ": 12345678:2;echo \\\n\"hello there\" && git status\n"

	entries, err := ParseHistory(strings.NewReader(history))
	if err != nil {
		t.Fatalf("ParseHistory() error = %v", err)
	}

	assertCommands(t, entries, [][]string{
		{"echo", "hello there"},
		{"git", "status"},
	})
}

func TestParseHistoryWithAliases(t *testing.T) {
	aliases := map[string]string{
		"g":       "git",
		"gco":     "g checkout",
		"ll":      "ls -la",
		"script":  "./script",
		"nothing": "FOO=bar",
	}
	history := "g status && gco main; ll /tmp | script; nothing"

	entries, err := ParseHistoryWithAliases(strings.NewReader(history), aliases)
	if err != nil {
		t.Fatalf("ParseHistoryWithAliases() error = %v", err)
	}

	assertCommands(t, entries, [][]string{
		{"git", "status"},
		{"git", "checkout", "main"},
		{"ls", "-la", "/tmp"},
	})
}

func TestAliasCyclesAndInvalidExpansionsAreSafe(t *testing.T) {
	aliases := map[string]string{
		"a":   "b",
		"b":   "a",
		"bad": "'",
	}

	entries, err := ParseHistoryWithAliases(strings.NewReader("a; bad"), aliases)
	if err != nil {
		t.Fatalf("ParseHistoryWithAliases() error = %v", err)
	}
	assertCommands(t, entries, [][]string{{"a"}, {"bad"}})
}

func TestParseAliases(t *testing.T) {
	output := strings.Join([]string{
		"alias g='git'",
		"ll='ls -la'",
		"alias quoted='echo \"hello world\"'",
		"not-an-alias",
		"alias broken='",
		"alias empty=",
	}, "\n")

	want := map[string]string{
		"g":      "git",
		"ll":     "ls -la",
		"quoted": `echo "hello world"`,
	}
	if got := ParseAliases(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseAliases() = %#v, want %#v", got, want)
	}
}

func TestParseHistoryReturnsReaderError(t *testing.T) {
	wantErr := errors.New("read failed")
	_, err := ParseHistory(errorReader{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ParseHistory() error = %v, want %v", err, wantErr)
	}
}

func TestParseHistoryHandlesFinalContinuation(t *testing.T) {
	entries, err := ParseHistory(strings.NewReader("echo trailing\\"))
	if err != nil {
		t.Fatalf("ParseHistory() error = %v", err)
	}
	assertCommands(t, entries, [][]string{{"echo", "trailing"}})
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

var _ io.Reader = errorReader{}

func assertCommands(t *testing.T, entries []CommandEntry, want [][]string) {
	t.Helper()
	got := make([][]string, len(entries))
	for i, entry := range entries {
		got[i] = entry.Command
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}
