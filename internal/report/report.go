// Package report renders shareable command-frequency reports.
package report

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const maxDisplayedCommands = 10

// Command is one command and the number of times it appeared in shell history.
type Command struct {
	Name  string
	Count int
}

// Data contains the content displayed in a report.
//
// Commands may contain the complete ranking; only the top ten are displayed.
// TotalCommands and UniqueCommands describe the complete history rather than
// only the commands displayed in the report.
type Data struct {
	Period         string
	TotalCommands  int
	UniqueCommands int
	Commands       []Command
}

// Generate validates data and writes a self-contained HTML report to w.
func Generate(w io.Writer, data Data) error {
	if w == nil {
		return errors.New("report: writer is nil")
	}

	view, err := newView(data)
	if err != nil {
		return err
	}

	if err := htmlReportTemplate.Execute(w, view); err != nil {
		return fmt.Errorf("report: render HTML: %w", err)
	}
	return nil
}

// Render validates data and returns a self-contained HTML report.
func Render(data Data) ([]byte, error) {
	var output bytes.Buffer
	if err := Generate(&output, data); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

type commandView struct {
	Rank       int
	RankLabel  string
	Name       string
	Count      int
	Percentage string
	BarWidth   int
	Place      string
}

type reportView struct {
	Period         string
	TotalCommands  string
	UniqueCommands int
	TopCount       int
	Podium         []commandView
	Remaining      []commandView
}

func newView(data Data) (reportView, error) {
	if data.TotalCommands <= 0 {
		return reportView{}, errors.New("report: total commands must be greater than zero")
	}
	if data.UniqueCommands <= 0 {
		return reportView{}, errors.New("report: unique commands must be greater than zero")
	}
	if len(data.Commands) == 0 {
		return reportView{}, errors.New("report: at least one command is required")
	}
	if data.UniqueCommands < len(data.Commands) {
		return reportView{}, errors.New("report: unique commands cannot be less than the supplied command count")
	}

	commands := append([]Command(nil), data.Commands...)
	for i, command := range commands {
		if strings.TrimSpace(command.Name) == "" {
			return reportView{}, fmt.Errorf("report: command %d has an empty name", i+1)
		}
		if command.Count <= 0 {
			return reportView{}, fmt.Errorf("report: command %q count must be greater than zero", command.Name)
		}
		if command.Count > data.TotalCommands {
			return reportView{}, fmt.Errorf("report: command %q count exceeds total commands", command.Name)
		}
	}

	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Count == commands[j].Count {
			return commands[i].Name < commands[j].Name
		}
		return commands[i].Count > commands[j].Count
	})

	if len(commands) > maxDisplayedCommands {
		commands = commands[:maxDisplayedCommands]
	}

	views := make([]commandView, len(commands))
	maxCount := commands[0].Count
	for i, command := range commands {
		percentage := float64(command.Count) / float64(data.TotalCommands) * 100
		views[i] = commandView{
			Rank:       i + 1,
			RankLabel:  fmt.Sprintf("%02d", i+1),
			Name:       command.Name,
			Count:      command.Count,
			Percentage: fmt.Sprintf("%.1f%%", percentage),
			BarWidth:   max(4, command.Count*100/maxCount),
		}
	}

	podiumCount := min(3, len(views))
	podium := append([]commandView(nil), views[:podiumCount]...)
	for i := range podium {
		podium[i].Place = []string{"gold", "silver", "bronze"}[i]
	}
	if len(podium) == 3 {
		podium = []commandView{podium[1], podium[0], podium[2]}
	}

	period := strings.TrimSpace(data.Period)
	if period == "" {
		period = "All recorded history"
	}
	return reportView{
		Period:         period,
		TotalCommands:  formatNumber(data.TotalCommands),
		UniqueCommands: data.UniqueCommands,
		TopCount:       commands[0].Count,
		Podium:         podium,
		Remaining:      views[podiumCount:],
	}, nil
}

func formatNumber(value int) string {
	digits := fmt.Sprintf("%d", value)
	for i := len(digits) - 3; i > 0; i -= 3 {
		digits = digits[:i] + "," + digits[i:]
	}
	return digits
}
