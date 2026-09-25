package scenariogen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Anthropic is a Generator backed by the Anthropic Messages API. APIKey comes
// from the environment; it is never logged or returned.
type Anthropic struct {
	APIKey   string
	Model    string
	Endpoint string
	Client   *http.Client
}

// AnthropicFromEnv builds a Generator from BEAN_ANTHROPIC_API_KEY /
// BEAN_ANTHROPIC_MODEL / BEAN_ANTHROPIC_ENDPOINT; nil when unconfigured.
func AnthropicFromEnv() Generator {
	key := os.Getenv("BEAN_ANTHROPIC_API_KEY")
	if key == "" {
		return nil
	}
	model := os.Getenv("BEAN_ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	endpoint := os.Getenv("BEAN_ANTHROPIC_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.anthropic.com"
	}
	return &Anthropic{APIKey: key, Model: model, Endpoint: strings.TrimRight(endpoint, "/")}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Anthropic) Generate(ctx context.Context, request Request) (map[string]any, error) {
	prompt := request.Prompt
	if request.Feedback != "" {
		prompt += "\n\nThe previous draft failed validation:\n" + request.Feedback + "\nReturn a corrected spec."
	}
	body, err := json.Marshal(anthropicRequest{
		Model:     a.Model,
		MaxTokens: 4096,
		System:    systemPrompt(),
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return nil, err
	}
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Endpoint+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("content-type", "application/json")
	httpRequest.Header.Set("x-api-key", a.APIKey)
	httpRequest.Header.Set("anthropic-version", "2023-06-01")
	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var decoded anthropicResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("generation response is not JSON: %w", err)
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("generation provider error: %s", decoded.Error.Message)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("generation provider returned %d", response.StatusCode)
	}
	for _, block := range decoded.Content {
		if block.Type != "text" {
			continue
		}
		if spec, err := decodeSpec(block.Text); err == nil {
			return spec, nil
		}
	}
	return nil, fmt.Errorf("generation returned no scenario spec")
}

// decodeSpec extracts the JSON object from a model reply, tolerating markdown
// fences and surrounding prose.
func decodeSpec(text string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	if start := strings.Index(text, "```"); start >= 0 {
		rest := text[start+3:]
		if newline := strings.Index(rest, "\n"); newline >= 0 {
			rest = rest[newline+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			text = rest[:end]
		}
	}
	if start := strings.Index(text, "{"); start >= 0 {
		if end := strings.LastIndex(text, "}"); end > start {
			text = text[start : end+1]
		}
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(text), &spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func systemPrompt() string {
	return `You generate Bean Scenario definitions from natural-language test descriptions.

Return ONLY a JSON object — the Scenario spec body — with this shape:
{"title": "Human title", "description": "what this verifies", "start": "<first node id>", "nodes": [{"id": "...", "type": "...", "next": "..."}]}

Node contract:
` + Vocabulary() + `
Rules:
- node ids are lowercase snake_case, unique.
- The graph is linear by default: each node's next points at the following node id; the last node omits next.
- Use snapshot refs only via wait/assert/extract nodes that observe refs produced by the runtime (for example, wait for ref_visible on a ref like "text=Sign in" is not valid — refs come from browser snapshots; prefer text/url conditions unless the prompt names a concrete element).
- Prefer url_equals/url_contains/text_present conditions over refs for generated scenarios.
- fill nodes use text for literal input or secret for a BEAN_SECRET_* environment secret name.
- api_call nodes reference an Action definition name from the application.
- Keep scenarios minimal: navigate, act, assert.
- Output raw JSON only — no markdown, no commentary.`
}
