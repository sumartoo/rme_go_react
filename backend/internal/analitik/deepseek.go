package analitik

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultDeepSeekURL is DeepSeek's OpenAI-compatible API.
const DefaultDeepSeekURL = "https://api.deepseek.com"

// DeepSeek is the Assistant backed by the DeepSeek chat completions API
// (OpenAI-compatible, called over plain HTTP: there is no official Go SDK).
type DeepSeek struct {
	apiKey  string
	baseURL string
	model   string
	timeout time.Duration
	http    *http.Client
	retry   time.Duration // wait before the one retry of a 429/5xx
}

// NewDeepSeek returns a DeepSeek assistant. baseURL may be empty for the
// public API.
func NewDeepSeek(apiKey, baseURL, model string, timeout time.Duration) *DeepSeek {
	if baseURL == "" {
		baseURL = DefaultDeepSeekURL
	}
	return &DeepSeek{
		apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), model: model, timeout: timeout,
		http: &http.Client{}, retry: 2 * time.Second,
	}
}

type dsFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON text
}

type dsToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function dsFunctionCall `json:"function"`
}

// dsMessage is a chat message. Fields the API returns but must not be sent
// back (such as reasoning_content) are left out on purpose.
type dsMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content"`
	ToolCalls  []dsToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type dsFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type dsTool struct {
	Type     string     `json:"type"`
	Function dsFunction `json:"function"`
}

type dsRequest struct {
	Model      string      `json:"model"`
	Messages   []dsMessage `json:"messages"`
	Tools      []dsTool    `json:"tools,omitempty"`
	ToolChoice string      `json:"tool_choice,omitempty"`
	// ResponseFormat {"type": "json_object"} asks for a JSON answer.
	ResponseFormat *dsResponseFormat `json:"response_format,omitempty"`
	MaxTokens      int               `json:"max_tokens"`
	Stream         bool              `json:"stream"`
}

type dsResponseFormat struct {
	Type string `json:"type"`
}

type dsResponse struct {
	Choices []struct {
		Message      dsMessage `json:"message"`
		FinishReason string    `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func dsTools() []dsTool {
	out := make([]dsTool, len(toolDefs))
	for i, t := range toolDefs {
		out[i] = dsTool{Type: "function", Function: dsFunction{Name: t.Name, Description: t.Description, Parameters: t.Params}}
	}
	return out
}

// complete sends one chat completion request, retrying once on 429 or 5xx.
func (d *DeepSeek) complete(ctx context.Context, req dsRequest) (dsMessage, string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return dsMessage{}, "", err
	}
	for attempt := 0; ; attempt++ {
		msg, finish, retryable, err := d.send(ctx, body)
		if err == nil || !retryable || attempt == 1 {
			return msg, finish, err
		}
		select {
		case <-ctx.Done():
			return dsMessage{}, "", ctx.Err()
		case <-time.After(d.retry):
		}
	}
}

func (d *DeepSeek) send(ctx context.Context, body []byte) (dsMessage, string, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return dsMessage{}, "", false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+d.apiKey)
	resp, err := d.http.Do(httpReq)
	if err != nil {
		return dsMessage{}, "", ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return dsMessage{}, "", true, err
	}
	var out dsResponse
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK {
		detail := strings.TrimSpace(string(raw))
		if out.Error != nil && out.Error.Message != "" {
			detail = out.Error.Message
		}
		if len(detail) > 300 {
			detail = detail[:300]
		}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return dsMessage{}, "", retryable, fmt.Errorf("deepseek: HTTP %d: %s", resp.StatusCode, detail)
	}
	if len(out.Choices) == 0 {
		return dsMessage{}, "", false, fmt.Errorf("deepseek: empty response")
	}
	c := out.Choices[0]
	if c.FinishReason == "content_filter" {
		return dsMessage{}, "", false, ErrRefused
	}
	return c.Message, c.FinishReason, false, nil
}

// Ringkasan writes an executive summary of data (JSON).
func (d *DeepSeek) Ringkasan(ctx context.Context, konteks, data string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	msg, _, err := d.complete(ctx, dsRequest{
		Model: d.model, MaxTokens: 4000,
		Messages: []dsMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: konteks + "\n\n" + ringkasanPrompt + "\n\n<data>\n" + data + "\n</data>"},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(msg.Content), nil
}

// Chat answers the last user message, calling the tools as needed.
func (d *DeepSeek) Chat(ctx context.Context, konteks string, history []ChatMessage, run ToolFunc) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	messages := make([]dsMessage, 0, len(history)+1+2*MaxToolRounds)
	messages = append(messages, dsMessage{Role: "system", Content: systemPrompt})
	for i, m := range history {
		content := m.Content
		if i == 0 {
			content = konteks + "\n\n" + content
		}
		role := "user"
		if m.Role == "assistant" {
			role = "assistant"
		}
		messages = append(messages, dsMessage{Role: role, Content: content})
	}
	tools := dsTools()
	for round := 0; ; round++ {
		choice := "auto"
		if round >= MaxToolRounds {
			choice = "none" // tools stay declared; only further calls are forbidden
		}
		msg, _, err := d.complete(ctx, dsRequest{Model: d.model, MaxTokens: 4000, Messages: messages, Tools: tools, ToolChoice: choice})
		if err != nil {
			return "", err
		}
		if len(msg.ToolCalls) == 0 || round >= MaxToolRounds {
			return strings.TrimSpace(msg.Content), nil
		}
		messages = append(messages, dsMessage{Role: "assistant", Content: msg.Content, ToolCalls: msg.ToolCalls})
		for _, call := range msg.ToolCalls {
			out, err := run(ctx, call.Function.Name, json.RawMessage(call.Function.Arguments))
			if err != nil {
				out = "Error: " + err.Error()
			}
			messages = append(messages, dsMessage{Role: "tool", ToolCallID: call.ID, Content: out})
		}
	}
}

const saranICDPrompt = `Anda membantu dokter di Indonesia mencari kode ICD-10 (WHO) untuk deskripsi klinis.
Balas hanya dengan JSON: {"kandidat": [{"kode": "J06.9", "alasan": "..."}]}.
- Maksimal 5 kandidat, paling sesuai lebih dulu.
- Kode ICD-10 yang benar-benar ada; boleh kategori tiga karakter (mis. J06) atau subkategori (mis. J06.9).
- "alasan" satu kalimat singkat dalam Bahasa Indonesia.
- Ini saran untuk ditinjau dokter, bukan diagnosis. Bila deskripsi tidak cukup, kembalikan kandidat yang paling umum atau daftar kosong.`

// SaranICD proposes ICD-10 codes for a clinical description.
func (d *DeepSeek) SaranICD(ctx context.Context, deskripsi string) ([]ICDSaran, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	msg, _, err := d.complete(ctx, dsRequest{
		Model: d.model, MaxTokens: 800,
		ResponseFormat: &dsResponseFormat{Type: "json_object"},
		Messages: []dsMessage{
			{Role: "system", Content: saranICDPrompt},
			{Role: "user", Content: "Deskripsi klinis: " + deskripsi},
		},
	})
	if err != nil {
		return nil, err
	}
	var out struct {
		Kandidat []ICDSaran `json:"kandidat"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(msg.Content)), &out); err != nil {
		return nil, fmt.Errorf("deepseek: ICD answer is not JSON: %w", err)
	}
	return out.Kandidat[:min(5, len(out.Kandidat))], nil
}
