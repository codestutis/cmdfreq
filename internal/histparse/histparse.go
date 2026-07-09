// currently only supports EXTENDED_HISTORY

package histparse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/google/shlex"
)

type CommandEntry struct {
	Command []string
}

// multiline commands end with a \
// next line is a continuation line if it does not start with ": "
func ParseHistory(hist io.Reader) ([]CommandEntry, error) {
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

		entry, err := parseCommandEntry(currLine)
		if err == nil && entry != nil {
			commands = append(commands, *entry)
		}
		currLine = nil
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}

// extended history format
// : <time-stamp>:<duration>;<command>
// parse a single command entry into the CommandEntry struct
func parseCommandEntry(entry []byte) (*CommandEntry, error) {
	s := string(entry)
	if s == "" {
		return nil, nil
	}

	// extended entry
	if entry[0] == ':' {
		idx := strings.Index(s, ";")
		if idx == -1 {
			return nil, fmt.Errorf("invalid command format")
		}

		s = s[idx+1:]
		s = strings.ReplaceAll(s, "\\\n", " ")
	}
	args, err := shlex.Split(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

	return &CommandEntry{
		Command: args,
	}, nil
}
