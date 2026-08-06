package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/codestutis/cmdfreq/internal/histparse"
	"github.com/codestutis/cmdfreq/internal/report"
)

func mustGetHistoryFile() io.ReadCloser {
	file, err := os.Open(os.Getenv("HISTFILE"))
	if err != nil {
		log.Fatalf("cant find history file")
	}
	return file
}

type CommandFreq struct {
	Command string
	Count   int
}

const maxBarWidth = 36
const defaultTopN = 20

// ANSI helpers
func fg(hex, text string) string { return fmt.Sprintf("\033[38;2;%sm%s\033[0m", hexToANSI(hex), text) }
func bg(hex, text string) string { return fmt.Sprintf("\033[48;2;%sm%s\033[0m", hexToANSI(hex), text) }
func bold(text string) string    { return fmt.Sprintf("\033[1m%s\033[0m", text) }
func dim(text string) string     { return fmt.Sprintf("\033[2m%s\033[0m", text) }

func hexToANSI(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}

func stripANSI(s string) string {
	out := strings.Builder{}
	inEsc := false
	for _, c := range s {
		if c == '\033' {
			inEsc = true
		}
		if !inEsc {
			out.WriteRune(c)
		}
		if inEsc && c == 'm' {
			inEsc = false
		}
	}
	return out.String()
}

func padRight(s string, width int) string {
	visible := stripANSI(s)
	pad := max(width-len([]rune(visible)), 0)
	return s + strings.Repeat(" ", pad)
}

type rankTheme struct {
	barColor   string
	labelColor string
	medal      string
}

var themes = []rankTheme{
	{"#FFD700", "#FFD700", "🥇"},
	{"#C0C0C0", "#A8B2D8", "🥈"},
	{"#CD7F32", "#FF8C42", "🥉"},
}
var defaultTheme = rankTheme{"#6272A4", "#CDD6F4", "  "}

func getTheme(i int) rankTheme {
	if i < len(themes) {
		return themes[i]
	}
	return defaultTheme
}

func renderBar(i int, cmd CommandFreq, maxCount int) string {
	width := int(float64(cmd.Count) / float64(maxCount) * maxBarWidth)
	if width < 1 {
		width = 1
	}
	t := getTheme(i)
	medal := t.medal + " "
	rank := dim(fmt.Sprintf("#%-2d", i+1))
	label := padRight(bold(fg(t.labelColor, cmd.Command)), 16)
	bar := bg(t.barColor, strings.Repeat(" ", width))
	count := dim(fmt.Sprintf(" %d", cmd.Count))
	return fmt.Sprintf("  %s %s %s%s%s", medal, rank, label, bar, count)
}

func printSummary(ranked []CommandFreq, topN int) {
	fmt.Println()
	fmt.Println("  " + bold(fg("#E2E8F0", "cmdfreq")) + "  " + dim("most used commands"))
	fmt.Println("  " + dim(strings.Repeat("─", 58)))
	fmt.Println()

	maxCount := ranked[0].Count
	limit := topN
	if len(ranked) < limit {
		limit = len(ranked)
	}
	for i, cmd := range ranked[:limit] {
		fmt.Println(renderBar(i, cmd, maxCount))
	}

	fmt.Println()
	fmt.Println("  " + dim(fmt.Sprintf("─── %d unique command(s)", len(ranked))))
	fmt.Println()
}

func renderHTMLSummary(ranked []CommandFreq) ([]byte, error) {
	commands := make([]report.Command, len(ranked))
	totalCommands := 0
	for i, command := range ranked {
		commands[i] = report.Command{Name: command.Command, Count: command.Count}
		totalCommands += command.Count
	}

	return report.Render(report.Data{
		TotalCommands:  totalCommands,
		UniqueCommands: len(commands),
		Commands:       commands,
	})
}

func outputSummary(ranked []CommandFreq, topN int, htmlPath string) error {
	if htmlPath == "" {
		printSummary(ranked, topN)
		return nil
	}

	html, err := renderHTMLSummary(ranked)
	if err != nil {
		return err
	}
	if htmlPath == "-" {
		_, err = os.Stdout.Write(html)
		return err
	}
	if err := os.WriteFile(htmlPath, html, 0o644); err != nil {
		return fmt.Errorf("write HTML report: %w", err)
	}
	return nil
}

func main() {
	topN := flag.Int("n", defaultTopN, "number of results to show")
	htmlPath := flag.String("html", "", "write an HTML report to path (use - for stdout)")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: cmdfreq [-n count] [--html path] [<command>]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *topN < 1 {
		log.Fatalf("-n must be greater than 0")
	}

	cmdArgs := flag.Args()
	if len(cmdArgs) > 1 {
		log.Fatalf("too many arguments\nUsage: cmdfreq [-n count] [<command>]")
	}

	histFile := mustGetHistoryFile()
	defer histFile.Close()

	entries, err := histparse.ParseHistory(histFile)
	if err != nil {
		log.Fatal(err)
	}

	if len(cmdArgs) == 0 {
		freq := make(map[string]int)
		for _, entry := range entries {
			if len(entry.Command) > 0 {
				freq[entry.Command[0]]++
			}
		}

		var sortedEntries []CommandFreq
		for cmd, count := range freq {
			sortedEntries = append(sortedEntries, CommandFreq{cmd, count})
		}

		sort.Slice(sortedEntries, func(i, j int) bool {
			return sortedEntries[i].Count > sortedEntries[j].Count
		})
		if err := outputSummary(sortedEntries, *topN, *htmlPath); err != nil {
			log.Fatal(err)
		}
	} else {
		command := cmdArgs[0]
		ok := false
		for _, entry := range entries {
			if len(entry.Command) > 0 && entry.Command[0] == command {
				ok = true
				break
			}
		}
		if !ok {
			log.Fatalf("command not found: %s", command)
		}

		//! this wont work properly if command arguments are used in different orders
		freq := make(map[string]int)
		for _, e := range entries {
			cmd := e.Command
			if len(cmd) < 2 || cmd[0] != command {
				continue
			}
			freq[cmd[1]]++
		}

		if len(freq) == 0 {
			log.Fatalf("no arguments found for: %s", command)
		}

		var sortedEntries []CommandFreq
		for cmd, count := range freq {
			sortedEntries = append(sortedEntries, CommandFreq{cmd, count})
		}

		sort.Slice(sortedEntries, func(i, j int) bool {
			return sortedEntries[i].Count > sortedEntries[j].Count
		})
		if err := outputSummary(sortedEntries, *topN, *htmlPath); err != nil {
			log.Fatal(err)
		}

	}
}
