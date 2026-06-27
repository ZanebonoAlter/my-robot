package airouter

import (
	"testing"

	"syntopica-backend/internal/models"
)

func TestStripThinkTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no think tags",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "simple think tags",
			input: "<think\n>\nLet me think about this\n</think\n>\nThe answer is 42",
			want:  "The answer is 42",
		},
		{
			name:  "think tags with multiline content",
			input: "<think\n>\nStep 1: analyze\nStep 2: reason\nStep 3: conclude\n</think\n>\nFinal answer",
			want:  "Final answer",
		},
		{
			name:  "only think content no trailing text",
			input: "<think\n>\nJust thinking\n</think\n>",
			want:  "",
		},
		{
			name:  "think tags with extra whitespace in tags",
			input: "<think  >\nreasoning\n</think  >\nresult",
			want:  "result",
		},
		{
			name:  "multiple think blocks",
			input: "<think\n>first</think\n>middle<think\n>second</think\n>end",
			want:  "middleend",
		},
		{
			name:  "empty think tags",
			input: "<think\n></think\n>content",
			want:  "content",
		},
		{
			name:  "think with code blocks inside",
			input: "<think\n>\n```go\nfmt.Println(\"hi\")\n```\n</think\n>\nHere is the answer",
			want:  "Here is the answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripThinkTags(tt.input)
			if got != tt.want {
				t.Errorf("stripThinkTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildPayload_EnableThinking(t *testing.T) {
	// EnableThinking=true → payload 必须含 chat_template_kwargs.enable_thinking=true
	t.Run("enable_thinking true propagates chat_template_kwargs", func(t *testing.T) {
		provider := models.AIProvider{Model: "qwythos", EnableThinking: true}
		req := ChatRequest{
			Messages: []Message{{Role: "user", Content: "hi"}},
		}
		payload := buildPayload(provider, req)
		kwargs, ok := payload["chat_template_kwargs"].(map[string]any)
		if !ok {
			t.Fatalf("expected chat_template_kwargs map, got %T", payload["chat_template_kwargs"])
		}
		if kwargs["enable_thinking"] != true {
			t.Fatalf("expected enable_thinking=true, got %v", kwargs["enable_thinking"])
		}
	})

	// EnableThinking=false → payload 必须不含 chat_template_kwargs
	t.Run("enable_thinking false omits chat_template_kwargs", func(t *testing.T) {
		provider := models.AIProvider{Model: "qwythos", EnableThinking: false}
		req := ChatRequest{
			Messages: []Message{{Role: "user", Content: "hi"}},
		}
		payload := buildPayload(provider, req)
		if _, ok := payload["chat_template_kwargs"]; ok {
			t.Fatalf("expected no chat_template_kwargs when EnableThinking=false, got %v", payload["chat_template_kwargs"])
		}
	})

	// 既有行为不回归：model/messages/temperature/max_tokens 仍在
	t.Run("preserves base payload fields", func(t *testing.T) {
		temp := 0.3
		maxTok := 2000
		provider := models.AIProvider{Model: "qwythos"}
		req := ChatRequest{
			Messages:    []Message{{Role: "user", Content: "hi"}},
			Temperature: &temp,
			MaxTokens:   &maxTok,
		}
		payload := buildPayload(provider, req)
		if payload["model"] != "qwythos" {
			t.Fatalf("expected model=qwythos, got %v", payload["model"])
		}
		if payload["temperature"] != 0.3 {
			t.Fatalf("expected temperature=0.3, got %v", payload["temperature"])
		}
		if payload["max_tokens"] != 2000 {
			t.Fatalf("expected max_tokens=2000, got %v", payload["max_tokens"])
		}
	})
}
