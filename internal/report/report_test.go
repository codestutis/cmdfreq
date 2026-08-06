package report

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	data := Data{
		Period:         "Aug 2025–Aug 2026",
		TotalCommands:  1024,
		UniqueCommands: 56,
		Commands: []Command{
			{Name: "npm", Count: 52},
			{Name: "clear", Count: 295},
			{Name: "git", Count: 204},
			{Name: "cd", Count: 75},
			{Name: "ls", Count: 75},
		},
	}

	output, err := Render(data)
	if previewPath := os.Getenv("CMDFREQ_REPORT_PREVIEW"); previewPath != "" {
		if err := os.WriteFile(previewPath, output, 0o644); err != nil {
			t.Fatalf("write report preview: %v", err)
		}
		t.Logf("report preview: %s", previewPath)
	}
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	html := string(output)
	for _, want := range []string{
		"<!doctype html>",
		"Aug 2025–Aug 2026",
		"1,024",
		">clear<",
		">28.8%<",
		">04</span>",
		">ls</span>",
		">05</span>",
		">npm</span>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("Render() output does not contain %q", want)
		}
	}
}

func TestRenderEscapesCommandNames(t *testing.T) {
	output, err := Render(Data{
		TotalCommands:  1,
		UniqueCommands: 1,
		Commands:       []Command{{Name: "<script>alert(1)</script>", Count: 1}},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	html := string(output)
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatal("Render() did not escape the command name")
	}
	if !strings.Contains(html, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatal("Render() output does not contain the escaped command name")
	}
}

func TestGenerateWritesHTML(t *testing.T) {
	var output bytes.Buffer
	err := Generate(&output, Data{
		TotalCommands:  2,
		UniqueCommands: 1,
		Commands:       []Command{{Name: "git", Count: 2}},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if output.Len() == 0 {
		t.Fatal("Generate() wrote no output")
	}
}

func TestRenderRejectsInvalidData(t *testing.T) {
	tests := []struct {
		name string
		data Data
	}{
		{name: "empty", data: Data{}},
		{
			name: "missing commands",
			data: Data{TotalCommands: 1, UniqueCommands: 1},
		},
		{
			name: "empty command name",
			data: Data{TotalCommands: 1, UniqueCommands: 1, Commands: []Command{{Count: 1}}},
		},
		{
			name: "non-positive count",
			data: Data{TotalCommands: 1, UniqueCommands: 1, Commands: []Command{{Name: "git"}}},
		},
		{
			name: "count exceeds total",
			data: Data{TotalCommands: 1, UniqueCommands: 1, Commands: []Command{{Name: "git", Count: 2}}},
		},
		{
			name: "too few unique commands",
			data: Data{
				TotalCommands:  2,
				UniqueCommands: 1,
				Commands: []Command{
					{Name: "git", Count: 1},
					{Name: "go", Count: 1},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Render(test.data); err == nil {
				t.Fatal("Render() error = nil, want an error")
			}
		})
	}
}
