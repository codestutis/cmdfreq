package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderHTMLSummary(t *testing.T) {
	output, err := renderHTMLSummary([]CommandFreq{
		{Command: "git", Count: 7},
		{Command: "go", Count: 3},
	})
	if err != nil {
		t.Fatalf("renderHTMLSummary() error = %v", err)
	}

	html := string(output)
	for _, want := range []string{"<!doctype html>", ">10<", ">2<", ">git<", ">go<"} {
		if !strings.Contains(html, want) {
			t.Errorf("renderHTMLSummary() output does not contain %q", want)
		}
	}
}

func TestOutputSummaryWritesHTMLFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	if err := outputSummary([]CommandFreq{{Command: "git", Count: 3}}, 20, path); err != nil {
		t.Fatalf("outputSummary() error = %v", err)
	}

	output, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(output), "<!doctype html>") {
		t.Fatal("outputSummary() did not write an HTML report")
	}
}
