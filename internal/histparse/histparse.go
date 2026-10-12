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

	return parseCommandString(s, aliases)
}

// parseCommandString parses a command line, counting both the command itself
// and any commands run inside shell command substitutions.
func parseCommandString(s string, aliases map[string]string) []CommandEntry {
	outer, substitutions := extractCommandSubstitutions(s)

	var entries []CommandEntry
	for _, substitution := range substitutions {
		entries = append(entries, parseCommandString(substitution, aliases)...)
	}
	return append(entries, parseSegments(outer, aliases)...)
}

func parseSegments(s string, aliases map[string]string) []CommandEntry {
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

// extractCommandSubstitutions removes command substitutions (`$(...)` and
// backticks) from s and returns the remaining text along with the inner
// command strings, in the order they appear. Single-quoted spans and escaped
// characters are left untouched, and `$((...))` arithmetic is not treated as a
// command substitution.
func extractCommandSubstitutions(s string) (string, []string) {
	runes := []rune(s)
	var out strings.Builder
	var substitutions []string
	var quote rune

	for i := 0; i < len(runes); {
		char := runes[i]

		if quote == '\'' {
			out.WriteRune(char)
			i++
			if char == '\'' {
				quote = 0
			}
			continue
		}

		if char == '\\' {
			out.WriteRune(char)
			if i+1 < len(runes) {
				out.WriteRune(runes[i+1])
				i += 2
			} else {
				i++
			}
			continue
		}

		switch char {
		case '\'':
			if quote == 0 {
				quote = '\''
			}
			out.WriteRune(char)
			i++
			continue
		case '"':
			if quote == '"' {
				quote = 0
			} else if quote == 0 {
				quote = '"'
			}
			out.WriteRune(char)
			i++
			continue
		case '$':
			if i+1 < len(runes) && runes[i+1] == '(' {
				end := matchClosingParen(runes, i+1)
				if end == -1 {
					out.WriteRune(char)
					i++
					continue
				}
				if i+2 < len(runes) && runes[i+2] == '(' {
					// Arithmetic expansion; not a command.
					out.WriteString(string(runes[i : end+1]))
				} else {
					substitutions = append(substitutions, string(runes[i+2:end]))
				}
				i = end + 1
				continue
			}
		case '`':
			end := matchClosingBacktick(runes, i)
			if end == -1 {
				out.WriteRune(char)
				i++
				continue
			}
			substitutions = append(substitutions, string(runes[i+1:end]))
			i = end + 1
			continue
		}

		out.WriteRune(char)
		i++
	}

	return out.String(), substitutions
}

// matchClosingParen returns the index of the ')' that closes the '(' at open,
// accounting for nesting, quotes, escapes and backticks. It returns -1 when the
// parenthesis is unbalanced.
func matchClosingParen(runes []rune, open int) int {
	depth := 0
	var quote rune

	for i := open; i < len(runes); i++ {
		char := runes[i]

		if quote == '\'' {
			if char == '\'' {
				quote = 0
			}
			continue
		}
		if quote == '"' {
			if char == '\\' {
				i++
				continue
			}
			if char == '"' {
				quote = 0
			}
			continue
		}

		switch char {
		case '\\':
			i++
		case '\'':
			quote = '\''
		case '"':
			quote = '"'
		case '`':
			end := matchClosingBacktick(runes, i)
			if end == -1 {
				return -1
			}
			i = end
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// matchClosingBacktick returns the index of the backtick that closes the one at
// open, or -1 when it is unbalanced.
func matchClosingBacktick(runes []rune, open int) int {
	for i := open + 1; i < len(runes); i++ {
		if runes[i] == '\\' {
			i++
			continue
		}
		if runes[i] == '`' {
			return i
		}
	}
	return -1
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
