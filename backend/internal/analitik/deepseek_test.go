package analitik

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDeepSeek answers chat completions from a script, one reply per request,
// and records the requests.
type fakeDeepSeek struct {
	mu      sync.Mutex
	replies []func(w http.ResponseWriter)
	reqs    []dsRequest
	auth    []string
}

func (f *fakeDeepSeek) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/chat/completions" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req dsRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.reqs = append(f.reqs, req)
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	if len(f.replies) == 0 {
		http.Error(w, "no more replies", http.StatusInternalServerError)
		return
	}
	reply := f.replies[0]
	f.replies = f.replies[1:]
	reply(w)
}

func reply(finish string, msg dsMessage) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": msg, "finish_reason": finish}},
		})
	}
}

func status(code int, body string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}
}

func newFakeDeepSeek(t *testing.T, replies ...func(w http.ResponseWriter)) (*DeepSeek, *fakeDeepSeek) {
	t.Helper()
	f := &fakeDeepSeek{replies: replies}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	d := NewDeepSeek("sk-test", srv.URL+"/", "deepseek-chat", 5*time.Second)
	d.retry = time.Millisecond
	return d, f
}

func TestDeepSeekRingkasan(t *testing.T) {
	d, f := newFakeDeepSeek(t, reply("stop", dsMessage{Role: "assistant", Content: "  Ringkasan.  "}))
	got, err := d.Ringkasan(context.Background(), "Tanggal hari ini: 2026-11-25.", `{"x":1}`)
	if err != nil || got != "Ringkasan." {
		t.Fatalf("got %q %v", got, err)
	}
	req := f.reqs[0]
	if f.auth[0] != "Bearer sk-test" || req.Model != "deepseek-chat" || req.Stream || len(req.Tools) != 0 ||
		len(req.Messages) != 2 || req.Messages[0].Role != "system" || !strings.Contains(req.Messages[1].Content, `{"x":1}`) ||
		!strings.Contains(req.Messages[1].Content, "2026-11-25") {
		t.Errorf("request: %s %+v", f.auth[0], req)
	}
}

func TestDeepSeekChatRunsTools(t *testing.T) {
	d, f := newFakeDeepSeek(t,
		reply("tool_calls", dsMessage{Role: "assistant", ToolCalls: []dsToolCall{
			{ID: "call_1", Type: "function", Function: dsFunctionCall{Name: ToolPrediksiObat, Arguments: `{"status":"kritis"}`}},
			{ID: "call_2", Type: "function", Function: dsFunctionCall{Name: "hapus_data", Arguments: `{}`}},
		}}),
		reply("stop", dsMessage{Role: "assistant", Content: "Ada 2 obat kritis."}),
	)
	var ran []string
	run := func(_ context.Context, name string, input json.RawMessage) (string, error) {
		ran = append(ran, name+string(input))
		if name != ToolPrediksiObat {
			return "", errors.New("tool tidak dikenal")
		}
		return `{"rows":2}`, nil
	}
	history := []ChatMessage{{Role: "user", Content: "Halo"}, {Role: "assistant", Content: "Ada yang bisa dibantu?"}, {Role: "user", Content: "Obat kritis?"}}
	got, err := d.Chat(context.Background(), "Cabang: KMJ.", history, run)
	if err != nil || got != "Ada 2 obat kritis." {
		t.Fatalf("got %q %v", got, err)
	}
	if strings.Join(ran, "|") != `prediksi_obat{"status":"kritis"}|hapus_data{}` {
		t.Errorf("tools run: %v", ran)
	}
	first := f.reqs[0]
	if len(first.Tools) != len(toolDefs) || first.ToolChoice != "auto" || first.Messages[1].Content != "Cabang: KMJ.\n\nHalo" || first.Messages[2].Role != "assistant" {
		t.Errorf("first request: %+v", first)
	}
	// The second request carries the assistant's tool calls and one tool message per call.
	second := f.reqs[1].Messages
	n := len(second)
	if n != 7 || second[n-3].Role != "assistant" || len(second[n-3].ToolCalls) != 2 ||
		second[n-2].Role != "tool" || second[n-2].ToolCallID != "call_1" || second[n-2].Content != `{"rows":2}` ||
		second[n-1].ToolCallID != "call_2" || !strings.HasPrefix(second[n-1].Content, "Error: ") {
		t.Errorf("second request: %+v", second)
	}
}

func TestDeepSeekChatStopsCallingTools(t *testing.T) {
	call := reply("tool_calls", dsMessage{Role: "assistant", ToolCalls: []dsToolCall{{ID: "c", Type: "function", Function: dsFunctionCall{Name: ToolTrenPenyakit, Arguments: ``}}}})
	replies := []func(http.ResponseWriter){}
	for range MaxToolRounds {
		replies = append(replies, call)
	}
	replies = append(replies, reply("stop", dsMessage{Role: "assistant", Content: "Selesai."}))
	d, f := newFakeDeepSeek(t, replies...)
	calls := 0
	got, err := d.Chat(context.Background(), "k", []ChatMessage{{Role: "user", Content: "tren?"}}, func(context.Context, string, json.RawMessage) (string, error) {
		calls++
		return "{}", nil
	})
	if err != nil || got != "Selesai." || calls != MaxToolRounds {
		t.Fatalf("got %q %v after %d tool calls", got, err, calls)
	}
	if last := f.reqs[len(f.reqs)-1]; last.ToolChoice != "none" || len(last.Tools) != len(toolDefs) {
		t.Errorf("last round must forbid tools but keep them declared: %+v", last.ToolChoice)
	}
}

func TestDeepSeekErrors(t *testing.T) {
	// 429 then success: retried once.
	d, f := newFakeDeepSeek(t, status(http.StatusTooManyRequests, `{"error":{"message":"rate limited"}}`), reply("stop", dsMessage{Content: "ok"}))
	if got, err := d.Ringkasan(context.Background(), "k", "{}"); err != nil || got != "ok" || len(f.reqs) != 2 {
		t.Errorf("retry: %q %v %d", got, err, len(f.reqs))
	}
	// 401: not retried, the API's message is kept.
	d, f = newFakeDeepSeek(t, status(http.StatusUnauthorized, `{"error":{"message":"Authentication Fails"}}`))
	if _, err := d.Ringkasan(context.Background(), "k", "{}"); err == nil || !strings.Contains(err.Error(), "HTTP 401: Authentication Fails") || len(f.reqs) != 1 {
		t.Errorf("401: %v %d", err, len(f.reqs))
	}
	// Content filter: a refusal.
	d, _ = newFakeDeepSeek(t, reply("content_filter", dsMessage{}))
	if _, err := d.Ringkasan(context.Background(), "k", "{}"); !errors.Is(err, ErrRefused) {
		t.Errorf("content filter: %v", err)
	}
	// 500 twice: gives up after the retry.
	d, f = newFakeDeepSeek(t, status(500, "down"), status(500, "down"))
	if _, err := d.Ringkasan(context.Background(), "k", "{}"); err == nil || len(f.reqs) != 2 {
		t.Errorf("5xx: %v %d", err, len(f.reqs))
	}
}

func TestDeepSeekSaranICD(t *testing.T) {
	d, f := newFakeDeepSeek(t, reply("stop", dsMessage{Content: `{"kandidat": [{"kode": "J06.9", "alasan": "ISPA"}, {"kode": "J00", "alasan": "Common cold"}]}`}))
	got, err := d.SaranICD(context.Background(), "batuk pilek")
	if err != nil || len(got) != 2 || got[0].Kode != "J06.9" || got[1].Alasan != "Common cold" {
		t.Fatalf("got %+v %v", got, err)
	}
	req := f.reqs[0]
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" || len(req.Tools) != 0 ||
		req.Messages[1].Content != "Deskripsi klinis: batuk pilek" {
		t.Errorf("request: %+v", req)
	}
	d, _ = newFakeDeepSeek(t, reply("stop", dsMessage{Content: "bukan json"}))
	if _, err := d.SaranICD(context.Background(), "x"); err == nil {
		t.Error("non-JSON answer accepted")
	}
}
