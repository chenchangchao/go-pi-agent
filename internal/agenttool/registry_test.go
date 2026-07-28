package agenttool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryRegisterAndExecute(t *testing.T) {
	rootDir := t.TempDir()

	filePath := filepath.Join(rootDir, "README.md")
	if err := os.WriteFile(
		filePath,
		[]byte("# Tool Registry\n"),
		0o600,
	); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	readFileTool, err := NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read file tool: %v", err)
	}

	registry := NewRegistry()

	if err := registry.Register(readFileTool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	message, err := registry.ExecuteRaw(
		context.Background(),
		"call_001",
		"read_file",
		map[string]string{
			"path": "README.md",
		},
	)
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}

	if message.ToolCallID != "call_001" {
		t.Fatalf(
			"expected tool call id %q, got %q",
			"call_001",
			message.ToolCallID,
		)
	}

	if message.Name != "read_file" {
		t.Fatalf(
			"expected tool name %q, got %q",
			"read_file",
			message.Name,
		)
	}

	if !strings.Contains(message.Content, "# Tool Registry") {
		t.Fatalf(
			"expected file content in tool result, got:\n%s",
			message.Content,
		)
	}
}

func TestRegistryRejectsDuplicateTool(t *testing.T) {
	rootDir := t.TempDir()

	readFileTool, err := NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read file tool: %v", err)
	}

	registry := NewRegistry()

	if err := registry.Register(readFileTool); err != nil {
		t.Fatalf("register first tool: %v", err)
	}

	err = registry.Register(readFileTool)
	if err == nil {
		t.Fatal("expected duplicate registration to fail")
	}

	if !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryRejectsUnknownTool(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.ExecuteRaw(
		context.Background(),
		"call_002",
		"unknown_tool",
		map[string]string{},
	)
	if err == nil {
		t.Fatal("expected unknown tool to return error")
	}

	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryDefinitionsAreSorted(t *testing.T) {
	rootDir := t.TempDir()

	readFileTool, err := NewReadFileTool(rootDir)
	if err != nil {
		t.Fatalf("create read file tool: %v", err)
	}

	registry := NewRegistry()

	if err := registry.Register(readFileTool); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	definitions := registry.Definitions()

	if len(definitions) != 1 {
		t.Fatalf(
			"expected 1 tool definition, got %d",
			len(definitions),
		)
	}

	if definitions[0].Name != "read_file" {
		t.Fatalf(
			"expected tool definition %q, got %q",
			"read_file",
			definitions[0].Name,
		)
	}
}
