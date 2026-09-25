// Package histparse parses shell history into individual commands.
package histparse

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"unicode"

	"github.com/google/shlex"
)

type CommandEntry struct {
	Command []string
}

// ParseHistory parses history without expanding aliases.
func ParseHistory(hist io.Reader) ([]CommandEntry, error) {
	return ParseHistoryWithAliases(hist, nil)
}

// ParseHistoryWithAliases parses history and expands command aliases. Each
// command separated by |, |&, &&, ||, or ; is returned as its own entry.
func ParseHistoryWithAliases(hist io.Reader, aliases map[string]string) ([]CommandEntry, error) {
	var commands []CommandEntry

	scanner := bufio.NewScanner(hist)
	var currLine []byte

	for scanner.Scan() {
		line := scanner.Bytes()
		currLine = append(currLine, line...)

		if bytes.HasSuffix(line, []byte(`\`)) {
			currLine = append(currLine, '\n')
			continue
		}

		commands = append(commands, parseCommandEntries(currLine, aliases)...)
		currLine = nil
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// A final continuation without another physical line is still safe to parse.
	if len(currLine) > 0 {
		commands = append(commands, parseCommandEntries(currLine, aliases)...)
	}

	return commands, nil
}

// Extended history format: : <time-stamp>:<duration>;<command>
func parseCommandEntries(entry []byte, aliases map[string]string) []CommandEntry {
	s := string(entry)
	if s == "" {
		return nil
	}

	if entry[0] == ':' {
		idx := strings.IndexByte(s, ';')
		if idx == -1 {
			return nil
		}
		s = s[idx+1:]
	}
	s = strings.ReplaceAll(s, "\\\n", " ")

	var entries []CommandEntry
	for _, segment := range splitCommandSegments(s) {
		args, err := shlex.Split(segment)
		if err != nil {
			continue
		}

		args = commandArgs(args)
		if len(args) == 0 {
			continue
		}

		args = resolveAlias(args, aliases)
		args = commandArgs(args)
		if len(args) == 0 || strings.Contains(args[0], "/") {
			continue
		}

		entries = append(entries, CommandEntry{Command: args})
	}
	return entries
}

func splitCommandSegments(command string) []string {
	var segments []string
	start := 0
	quote := rune(0)
	escaped := false
	runes := []rune(command)

	for i := 0; i < len(runes); i++ {
		char := runes[i]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}

		separatorLength := 0
		switch char {
		case ';':
			separatorLength = 1
		case '|':
			separatorLength = 1
			if i+1 < len(runes) && (runes[i+1] == '|' || runes[i+1] == '&') {
				separatorLength = 2
			}
		case '&':
			if i+1 < len(runes) && runes[i+1] == '&' {
				separatorLength = 2
			}
		}
		if separatorLength == 0 {
			continue
		}

		segments = append(segments, string(runes[start:i]))
		i += separatorLength - 1
		start = i + 1
	}

	segments = append(segments, string(runes[start:]))
	return segments
}

func commandArgs(args []string) []string {
	for len(args) > 0 && isAssignment(args[0]) {
		args = args[1:]
	}
	return args
}

func isAssignment(arg string) bool {
	name, _, ok := strings.Cut(arg, "=")
	if !ok || name == "" {
		return false
	}
	for i, char := range name {
		if char != '_' && !unicode.IsLetter(char) && (i == 0 || !unicode.IsDigit(char)) {
			return false
		}
	}
	return true
}

func resolveAlias(args []string, aliases map[string]string) []string {
	seen := make(map[string]bool)
	for len(args) > 0 {
		expansion, ok := aliases[args[0]]
		if !ok || seen[args[0]] {
			return args
		}
		seen[args[0]] = true

		expanded, err := shlex.Split(expansion)
		if err != nil || len(expanded) == 0 {
			return args
		}
		args = append(expanded, args[1:]...)
		args = commandArgs(args)
	}
	return args
}

// ParseAliases parses the output of the POSIX shell alias builtin.
func ParseAliases(output string) map[string]string {
	aliases := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "alias ")
		name, value, ok := strings.Cut(line, "=")
		if !ok || name == "" {
			continue
		}

		parsed, err := shlex.Split(value)
		if err != nil || len(parsed) != 1 {
			continue
		}
		aliases[name] = parsed[0]
	}
	return aliases
}
