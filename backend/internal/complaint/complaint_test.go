package complaint

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseAnalysis(t *testing.T) {
	want := Analysis{Category: "pengiriman", Sentiment: "negative", Priority: "high", Summary: "Paket telat"}
	for name, raw := range map[string]string{
		"object":       `{"category":"Pengiriman","sentiment":"negative","priority":"high","summary":"Paket telat"}`,
		"indonesian":   `{"category":"pengiriman","sentiment":"Negatif","priority":"tinggi","summary":"Paket telat"}`,
		"string":       `"{\"category\":\"pengiriman\",\"sentiment\":\"negative\",\"priority\":\"high\",\"summary\":\"Paket telat\"}"`,
		"fenced":       "```json\n{\"category\":\"pengiriman\",\"sentiment\":\"negative\",\"priority\":\"high\",\"summary\":\"Paket telat\"}\n```",
		"agent output": `{"output":"Berikut hasilnya:\n` + "```json" + `\n{\"category\":\"pengiriman\",\"sentiment\":\"negative\",\"priority\":\"high\",\"summary\":\"Paket telat\"}\n` + "```" + `"}`,
		"n8n items":    `[{"analysis":{"category":"pengiriman","sentiment":"negative","priority":"high","summary":"Paket telat"}}]`,
	} {
		got, err := ParseAnalysis([]byte(raw))
		if err != nil || got != want {
			t.Errorf("%s: got %+v, %v", name, got, err)
		}
	}

	for name, c := range map[string]struct{ raw, field string }{
		"bad sentiment":   {`{"category":"x","sentiment":"angry","priority":"high"}`, "sentiment"},
		"bad priority":    {`{"category":"x","sentiment":"negative","priority":"asap"}`, "priority"},
		"no category":     {`{"sentiment":"negative","priority":"high"}`, "category"},
		"number category": {`{"category":5,"sentiment":"negative","priority":"high"}`, "category"},
		"long category":   {`{"category":"` + strings.Repeat("a", 51) + `","sentiment":"negative","priority":"high"}`, "category"},
		"not json":        {`maaf, saya tidak bisa`, "analysis"},
	} {
		_, err := ParseAnalysis([]byte(c.raw))
		var verr *ValidationError
		if !errors.As(err, &verr) || verr.Fields[c.field] == "" {
			t.Errorf("%s: want error on %s, got %v", name, c.field, err)
		}
	}
}

func TestNormalize(t *testing.T) {
	in := Input{Message: " Pelanggan mengeluhkan keterlambatan pengiriman "}
	if errs := in.Normalize(); len(errs) != 0 || in.Title != DefaultTitle || in.Subject != DefaultSubject || in.Type != "ai" || in.Channel != "app" {
		t.Fatalf("got %v %+v", errs, in)
	}
	if errs := (&Input{}).Normalize(); errs["message"] == "" {
		t.Errorf("want message required, got %v", errs)
	}
	if errs := (&Input{Message: "x", Type: "sms"}).Normalize(); errs["type"] == "" {
		t.Errorf("unknown type must be refused, got %v", errs)
	}
}

// fakeRepo keeps one complaint in memory.
type fakeRepo struct {
	mu       sync.Mutex
	c        Complaint
	logs     []Log
	analysis *Analysis
}

func newFake() *fakeRepo {
	return &fakeRepo{c: Complaint{ID: 1, CustomerName: "Budi", Channel: "app", Message: "Paket telat", AIStatus: AIPending, HandlingStatus: "open"}}
}

func (f *fakeRepo) Create(context.Context, Input, string) (int64, error) { return 1, nil }
func (f *fakeRepo) Get(_ context.Context, id int64) (Complaint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id != f.c.ID {
		return Complaint{}, ErrNotFound
	}
	return f.c, nil
}
func (f *fakeRepo) List(context.Context, Filter) ([]Complaint, int, error) { return nil, 0, nil }
func (f *fakeRepo) Summary(context.Context) (Summary, error)               { return Summary{}, nil }
func (f *fakeRepo) Logs(context.Context, int64) ([]Log, error)             { return f.logs, nil }
func (f *fakeRepo) AddLog(_ context.Context, _ int64, l Log) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, l)
	if l.Event == "sent" || l.Event == "error" {
		f.c.AIAttempts++
	}
	return nil
}
func (f *fakeRepo) MarkProcessing(context.Context, int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.c.AIStatus == AIProcessing {
		return false, nil
	}
	f.c.AIStatus = AIProcessing
	return true, nil
}
func (f *fakeRepo) MarkFailed(_ context.Context, _ int64, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.c.AIStatus, f.c.AIError = AIFailed, &reason
	return nil
}
func (f *fakeRepo) SaveAnalysis(_ context.Context, _ int64, a Analysis) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.analysis, f.c.AIStatus = &a, AIAnalyzed
	return nil
}
func (f *fakeRepo) SetHandling(context.Context, int64, string, string, string) error { return nil }

type noQueue struct{ got *RouterRequest }

func (noQueue) Enqueue(int64) {}
func (q noQueue) Forward(_ context.Context, req RouterRequest) (int, []byte, error) {
	if q.got != nil {
		*q.got = req
	}
	return 200, []byte(`{"ok":true}`), nil
}

func callback(t *testing.T, repo *fakeRepo, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(repo, noQueue{}, "s3cret-s3cret-s3cret").Routes(mux, func(h http.Handler) http.Handler { return h })
	req := httptest.NewRequest(http.MethodPost, "/api/n8n/complaints/callback", strings.NewReader(body))
	req.Header.Set(SecretHeader, secret)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestCallback(t *testing.T) {
	ok := `{"complaintId":1,"analysis":{"category":"pengiriman","sentiment":"negative","priority":"urgent"}}`
	if rec := callback(t, newFake(), "wrong", ok); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong secret: got %d", rec.Code)
	}

	repo := newFake()
	if rec := callback(t, repo, "s3cret-s3cret-s3cret", ok); rec.Code != http.StatusOK || repo.c.AIStatus != AIAnalyzed || repo.analysis.Priority != "urgent" {
		t.Errorf("valid: got %d %s", rec.Code, rec.Body)
	}

	repo = newFake()
	rec := callback(t, repo, "s3cret-s3cret-s3cret", `{"complaintId":1,"analysis":{"category":"x","sentiment":"meh","priority":"high"}}`)
	if rec.Code != http.StatusUnprocessableEntity || repo.c.AIStatus != AIFailed || repo.logs[0].Event != "invalid_output" {
		t.Errorf("invalid: got %d %s %+v", rec.Code, repo.c.AIStatus, repo.logs)
	}

	if rec := callback(t, newFake(), "s3cret-s3cret-s3cret", `{"complaintId":9,"sentiment":"negative"}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown id: got %d", rec.Code)
	}

	repo = newFake()
	if rec := callback(t, repo, "s3cret-s3cret-s3cret", `{"complaintId":1,"error":"DeepSeek timeout"}`); rec.Code != http.StatusOK || repo.c.AIStatus != AIFailed {
		t.Errorf("error report: got %d %s", rec.Code, repo.c.AIStatus)
	}
}

func forward(t *testing.T, body string, got *RouterRequest) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(newFake(), noQueue{got: got}, "").Routes(mux, func(h http.Handler) http.Handler { return h })
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/n8n/requests", strings.NewReader(body)))
	return rec
}

func TestCreateForwardsEmailAndErp(t *testing.T) {
	for _, typ := range []string{"email", "erp"} {
		var got RouterRequest
		mux := http.NewServeMux()
		NewHandler(newFake(), noQueue{got: &got}, "").Routes(mux, func(h http.Handler) http.Handler { return h })
		rec := httptest.NewRecorder()
		body := `{"title":"T","subject":"send_notification","type":"` + typ + `","recipient":"a@b.com","message":"Pesanan diproses"}`
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/complaints", strings.NewReader(body)))
		if rec.Code != http.StatusOK || got.Type != typ || !strings.Contains(rec.Body.String(), `"n8nStatus":200`) {
			t.Errorf("%s: %d %s %+v", typ, rec.Code, rec.Body, got)
		}
	}
	mux := http.NewServeMux()
	NewHandler(newFake(), noQueue{}, "").Routes(mux, func(h http.Handler) http.Handler { return h })
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/complaints", strings.NewReader(`{"title":"Email","subject":"s","type":"email","message":"m"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("email without recipient: %d", rec.Code)
	}
}

func TestForward(t *testing.T) {
	var got RouterRequest
	rec := forward(t, `{"title":"Email","subject":"send_notification","type":"email","recipient":"a@b.com","message":"Pesanan diproses"}`, &got)
	if rec.Code != http.StatusOK || got.Recipient != "a@b.com" || got.Type != "email" || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Errorf("email: %d %s %+v", rec.Code, rec.Body, got)
	}
	if rec := forward(t, `{"title":"Email","subject":"s","type":"email","message":"m"}`, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("email without recipient: %d", rec.Code)
	}
	if rec := forward(t, `{"title":"X","subject":"s","type":"sms","message":"m"}`, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown type: %d", rec.Code)
	}
}

func TestForwardDoesNotRetryEmail(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	d := NewDispatcher(context.Background(), newFake(), DispatcherConfig{WebhookURL: srv.URL, MaxAttempts: 3, Backoff: time.Millisecond})
	if _, _, err := d.Forward(context.Background(), RouterRequest{Type: "email"}); err == nil || calls.Load() != 1 {
		t.Errorf("email: calls=%d err=%v", calls.Load(), err)
	}
	calls.Store(0)
	if _, _, err := d.Forward(context.Background(), RouterRequest{Type: "ai"}); err == nil || calls.Load() != 3 {
		t.Errorf("ai: calls=%d err=%v", calls.Load(), err)
	}
}

func TestDispatchSendsRouterBody(t *testing.T) {
	var got RouterRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		// Basic LLM Chain + structured output parser, answered by Respond to Webhook.
		_, _ = w.Write([]byte(`[{"output":{"kategori":"pengiriman","sentimen":"negatif","prioritas":"tinggi","ringkasan":"Telat"}}]`))
	}))
	defer srv.Close()
	repo := newFake()
	repo.c.Title, repo.c.Subject = DefaultTitle, DefaultSubject
	NewDispatcher(context.Background(), repo, DispatcherConfig{WebhookURL: srv.URL, MaxAttempts: 1}).Dispatch(context.Background(), 1)
	if got.Type != "ai" || got.Title != DefaultTitle || got.Subject != DefaultSubject || got.Message != "Paket telat" || got.ComplaintID != 1 {
		t.Errorf("body: %+v", got)
	}
	if repo.c.AIStatus != AIAnalyzed || repo.analysis.Priority != "high" || repo.analysis.Summary != "Telat" {
		t.Errorf("status=%s analysis=%+v logs=%+v", repo.c.AIStatus, repo.analysis, repo.logs)
	}
}

func TestDispatchRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(SecretHeader) != "s3cret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"output":"{\"category\":\"produk\",\"sentiment\":\"negatif\",\"priority\":\"sedang\"}"}`))
	}))
	defer srv.Close()

	repo := newFake()
	d := NewDispatcher(context.Background(), repo, DispatcherConfig{WebhookURL: srv.URL, Secret: "s3cret", MaxAttempts: 3, Backoff: time.Millisecond})
	d.Dispatch(context.Background(), 1)

	if calls.Load() != 3 || repo.c.AIStatus != AIAnalyzed || repo.analysis.Priority != "medium" || repo.c.AIAttempts != 3 {
		t.Fatalf("calls=%d status=%s attempts=%d logs=%+v", calls.Load(), repo.c.AIStatus, repo.c.AIAttempts, repo.logs)
	}
}

func TestDispatchTimeoutFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	repo := newFake()
	d := NewDispatcher(context.Background(), repo, DispatcherConfig{WebhookURL: srv.URL, MaxAttempts: 2, Timeout: 20 * time.Millisecond, Backoff: time.Millisecond})
	d.Dispatch(context.Background(), 1)

	if repo.c.AIStatus != AIFailed || repo.c.AIAttempts != 2 || repo.logs[len(repo.logs)-1].Event != "failed" {
		t.Fatalf("status=%s attempts=%d logs=%+v", repo.c.AIStatus, repo.c.AIAttempts, repo.logs)
	}
}

func TestDispatchAckWaitsForCallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":"Workflow was started"}`))
	}))
	defer srv.Close()

	repo := newFake()
	NewDispatcher(context.Background(), repo, DispatcherConfig{WebhookURL: srv.URL, MaxAttempts: 3}).Dispatch(context.Background(), 1)
	if repo.c.AIStatus != AIProcessing {
		t.Fatalf("status=%s", repo.c.AIStatus)
	}
}

func TestDispatchClientErrorIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	repo := newFake()
	NewDispatcher(context.Background(), repo, DispatcherConfig{WebhookURL: srv.URL, MaxAttempts: 3, Backoff: time.Millisecond}).Dispatch(context.Background(), 1)
	if calls.Load() != 1 || repo.c.AIStatus != AIFailed {
		t.Fatalf("calls=%d status=%s", calls.Load(), repo.c.AIStatus)
	}
}
