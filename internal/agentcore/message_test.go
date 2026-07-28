package agentcore

import "testing"

func TestUserMessage(t *testing.T) {
	message := UserMessage("读取 README.md")

	if message.Role != RoleUser {
		t.Fatalf("expected role %q, got %q", RoleUser, message.Role)
	}

	if message.Content != "读取 README.md" {
		t.Fatalf(
			"expected content %q, got %q",
			"读取 README.md",
			message.Content,
		)
	}
}

func TestToolResultMessage(t *testing.T) {
	message := ToolResultMessage(
		"call_123",
		"read_file",
		"file content",
	)

	if message.Role != RoleTool {
		t.Fatalf("expected role %q, got %q", RoleTool, message.Role)
	}

	if message.ToolCallID != "call_123" {
		t.Fatalf(
			"expected tool call id %q, got %q",
			"call_123",
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
}
