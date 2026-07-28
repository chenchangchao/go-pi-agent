package agenttool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileToolExecute(t *testing.T) {
	rootDir := t.TempDir()

	filePath := filepath.Join(rootDir, "README.md")
	fileContent := "# Demo\n\nHello Agent\n"

	if err := os.WriteFile(filePath, []byte(fileContent), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	tool, err := NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read file tool: %v", err)
	}

	arguments, err := json.Marshal(map[string]string{
		"path": "README.md",
	})
	if err != nil {
		t.Fatalf("marshal arguments: %v", err)
	}

	result, err := tool.Execute(context.Background(), arguments)
	if err != nil {
		t.Fatalf("execute read_file: %v", err)
	}

	expectedFragments := []string{
		"File: README.md",
		"1 | # Demo",
		"3 | Hello Agent",
	}

	for _, expected := range expectedFragments {
		if !strings.Contains(result, expected) {
			t.Fatalf(
				"expected result to contain %q, got:\n%s",
				expected,
				result,
			)
		}
	}
}

func TestReadFileToolRejectsPathOutsideRoot(t *testing.T) {
	rootDir := t.TempDir()

	tool, err := NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read file tool: %v", err)
	}

	arguments, err := json.Marshal(map[string]string{
		"path": "../secret.txt",
	})
	if err != nil {
		t.Fatalf("marshal arguments: %v", err)
	}

	_, err = tool.Execute(context.Background(), arguments)
	if err == nil {
		t.Fatal("expected access outside root to be rejected")
	}

	if !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadFileToolRejectsEmptyPath(t *testing.T) {
	rootDir := t.TempDir()

	tool, err := NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read file tool: %v", err)
	}

	arguments := json.RawMessage(`{"path":""}`)

	_, err = tool.Execute(context.Background(), arguments)
	if err == nil {
		t.Fatal("expected empty path to return an error")
	}
}

func TestReadFileToolTruncatesContent(t *testing.T) {
	rootDir := t.TempDir()

	filePath := filepath.Join(rootDir, "large.txt")
	fileContent := "line one\nline two\nline three\n"

	if err := os.WriteFile(filePath, []byte(fileContent), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	tool, err := NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read file tool: %v", err)
	}

	tool.MaxLines = 2

	arguments := json.RawMessage(`{"path":"large.txt"}`)

	result, err := tool.Execute(context.Background(), arguments)
	if err != nil {
		t.Fatalf("execute read_file: %v", err)
	}

	if !strings.Contains(result, "内容已截断") {
		t.Fatalf("expected truncated marker, got:\n%s", result)
	}

	if strings.Contains(result, "line three") {
		t.Fatalf("expected third line to be omitted, got:\n%s", result)
	}
}
