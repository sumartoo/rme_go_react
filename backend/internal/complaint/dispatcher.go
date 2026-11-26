package complaint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// SecretHeader carries the shared secret on calls to n8n and on n8n's callback.
const SecretHeader = "X-Webhook-Secret"

// DispatcherConfig comes from the environment (see config.Config).
type DispatcherConfig struct {
	WebhookURL  string        // N8N_WEBHOOK_URL; empty = n8n not configured
	Secret      string        // N8N_WEBHOOK_SECRET
	Timeout     time.Duration // per attempt, N8N_TIMEOUT
	MaxAttempts int           // N8N_MAX_RETRIES
	Backoff     time.Duration // wait before attempt 2; doubles each time
	CallbackURL string        // sent to n8n so it knows where to post the result
}

// Dispatcher sends complaints to the n8n webhook in the background.
type Dispatcher struct {
	repo   Repository
	cfg    DispatcherConfig
	client *http.Client
	ctx    context.Context // stops retries on shutdown
	wg     sync.WaitGroup
}

func NewDispatcher(ctx context.Context, repo Repository, cfg DispatcherConfig) *Dispatcher {
	if cfg.MaxAttempts < 1 {
		cfg.MaxAttempts = 1
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.Backoff <= 0 {
		cfg.Backoff = 2 * time.Second
	}
	return &Dispatcher{repo: repo, cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}, ctx: ctx}
}

// Enqueue sends the complaint in a goroutine; the caller does not wait.
func (d *Dispatcher) Enqueue(id int64) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.Dispatch(d.ctx, id)
	}()
}

// Wait blocks until every running dispatch has returned.
func (d *Dispatcher) Wait() { d.wg.Wait() }

// RouterRequest is the body of the n8n smart-router webhook; its Switch node
// routes on Type (ai, erp, email). ComplaintID and CallbackURL are extra
// fields the workflow may ignore; they let it post the result back later.
type RouterRequest struct {
	Title       string `json:"title"`
	Subject     string `json:"subject"`
	Message     string `json:"message"`
	Type        string `json:"type"`
	Recipient   string `json:"recipient,omitempty"`
	ComplaintID int64  `json:"complaintId,omitempty"`
	CallbackURL string `json:"callbackUrl,omitempty"`
}

// Dispatch posts the complaint to n8n, retrying timeouts, network errors, 408,
// 429 and 5xx with exponential backoff, and logs every attempt. If n8n answers
// with the analysis itself it is validated and saved; otherwise the complaint
// stays processing until the callback arrives.
func (d *Dispatcher) Dispatch(ctx context.Context, id int64) {
	// Bookkeeping must still be written while shutting down.
	bg := context.WithoutCancel(ctx)

	c, err := d.repo.Get(bg, id)
	if err != nil {
		log.Printf("complaint %d: load: %v", id, err)
		return
	}
	if d.cfg.WebhookURL == "" {
		d.fail(bg, id, 0, ErrNotConfigured.Error())
		return
	}
	started, err := d.repo.MarkProcessing(bg, id)
	if err != nil {
		log.Printf("complaint %d: mark processing: %v", id, err)
		return
	}
	if !started {
		log.Printf("complaint %d: already processing, skipped", id)
		return
	}

	body, _ := json.Marshal(RouterRequest{
		Title: c.Title, Subject: c.Subject, Message: c.Message, Type: TypeAI,
		ComplaintID: c.ID, CallbackURL: d.cfg.CallbackURL,
	})

	var lastErr string
	for attempt := 1; attempt <= d.cfg.MaxAttempts; attempt++ {
		start := time.Now()
		status, respBody, err := d.post(ctx, body)
		ms := int(time.Since(start).Milliseconds())

		if err == nil && status >= 200 && status < 300 {
			d.addLog(bg, id, Log{Attempt: attempt, Event: "sent", HTTPStatus: &status, DurationMs: &ms})
			log.Printf("complaint %d: sent to n8n (attempt %d, %d, %dms)", id, attempt, status, ms)
			d.handleSyncResponse(bg, id, attempt, respBody)
			return
		}

		retryable := true
		var statusPtr *int
		if err != nil {
			lastErr = err.Error()
			if errors.Is(err, context.Canceled) {
				retryable = false
				lastErr = "interrupted: server shutting down"
			}
		} else {
			statusPtr = &status
			lastErr = fmt.Sprintf("n8n answered HTTP %d: %s", status, truncate(string(respBody), 300))
			retryable = status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
		}
		d.addLog(bg, id, Log{Attempt: attempt, Event: "error", HTTPStatus: statusPtr, Message: lastErr, DurationMs: &ms})
		log.Printf("complaint %d: n8n attempt %d/%d failed (%dms): %s", id, attempt, d.cfg.MaxAttempts, ms, lastErr)

		if !retryable || attempt == d.cfg.MaxAttempts {
			break
		}
		wait := d.cfg.Backoff << (attempt - 1)
		select {
		case <-ctx.Done():
			d.fail(bg, id, attempt, "interrupted: server shutting down")
			return
		case <-time.After(wait):
		}
	}
	d.fail(bg, id, d.cfg.MaxAttempts, lastErr)
}

// ErrNotConfigured means N8N_WEBHOOK_URL is empty.
var ErrNotConfigured = errors.New("N8N_WEBHOOK_URL is not configured")

// Forward sends one smart-router request to n8n and returns its answer.
// Only type "ai" is retried: erp and email change things (a purchase order, a
// sent email), so a timeout could otherwise run them twice.
func (d *Dispatcher) Forward(ctx context.Context, req RouterRequest) (int, []byte, error) {
	if d.cfg.WebhookURL == "" {
		return 0, nil, ErrNotConfigured
	}
	body, _ := json.Marshal(req)
	attempts := 1
	if req.Type == TypeAI {
		attempts = d.cfg.MaxAttempts
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		start := time.Now()
		status, respBody, err := d.post(ctx, body)
		ms := time.Since(start).Milliseconds()
		if err == nil && status != http.StatusRequestTimeout && status != http.StatusTooManyRequests && status < 500 {
			log.Printf("n8n %s/%s: HTTP %d (attempt %d, %dms)", req.Type, req.Subject, status, attempt, ms)
			return status, respBody, nil
		}
		if err == nil {
			err = fmt.Errorf("n8n answered HTTP %d: %s", status, truncate(string(respBody), 300))
		}
		lastErr = err
		log.Printf("n8n %s/%s: attempt %d/%d failed (%dms): %v", req.Type, req.Subject, attempt, attempts, ms, err)
		if errors.Is(err, context.Canceled) || attempt == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case <-time.After(d.cfg.Backoff << (attempt - 1)):
		}
	}
	return 0, nil, lastErr
}

func (d *Dispatcher) post(ctx context.Context, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(SecretHeader, d.cfg.Secret)
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, respBody, err
}

// handleSyncResponse saves the analysis when n8n's "Respond to Webhook" node
// returns it directly. A plain acknowledgement means: wait for the callback.
func (d *Dispatcher) handleSyncResponse(ctx context.Context, id int64, attempt int, body []byte) {
	if !LooksLikeAnalysis(body) {
		return
	}
	if err := Apply(ctx, d.repo, id, attempt, body); err != nil {
		log.Printf("complaint %d: %v", id, err)
	}
}

// Apply validates an AI output and saves it, or marks the complaint failed.
// Shared by the synchronous response and the callback.
func Apply(ctx context.Context, repo Repository, id int64, attempt int, raw []byte) error {
	a, err := ParseAnalysis(raw)
	if err != nil {
		_ = repo.AddLog(ctx, id, Log{Attempt: attempt, Event: "invalid_output", Message: err.Error() + " | raw: " + truncate(string(raw), 1000)})
		_ = repo.MarkFailed(ctx, id, err.Error())
		return err
	}
	if err := repo.SaveAnalysis(ctx, id, a); err != nil {
		return err
	}
	_ = repo.AddLog(ctx, id, Log{Attempt: attempt, Event: "analyzed",
		Message: fmt.Sprintf("category=%s sentiment=%s priority=%s", a.Category, a.Sentiment, a.Priority)})
	log.Printf("complaint %d: analyzed (category=%s sentiment=%s priority=%s)", id, a.Category, a.Sentiment, a.Priority)
	return nil
}

func (d *Dispatcher) fail(ctx context.Context, id int64, attempt int, reason string) {
	d.addLog(ctx, id, Log{Attempt: attempt, Event: "failed", Message: reason})
	if err := d.repo.MarkFailed(ctx, id, reason); err != nil {
		log.Printf("complaint %d: mark failed: %v", id, err)
	}
	log.Printf("complaint %d: AI analysis failed: %s", id, reason)
}

func (d *Dispatcher) addLog(ctx context.Context, id int64, l Log) {
	if err := d.repo.AddLog(ctx, id, l); err != nil {
		log.Printf("complaint %d: write log: %v", id, err)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
