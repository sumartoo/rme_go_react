package complaint

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// Queue talks to n8n: Enqueue starts a complaint's AI analysis in the
// background, Forward sends one smart-router request and waits for the answer.
type Queue interface {
	Enqueue(id int64)
	Forward(ctx context.Context, req RouterRequest) (int, []byte, error)
}

type Handler struct {
	repo   Repository
	queue  Queue
	secret string // N8N_WEBHOOK_SECRET; empty disables the callback
}

func NewHandler(repo Repository, queue Queue, secret string) *Handler {
	return &Handler{repo: repo, queue: queue, secret: secret}
}

// Routes registers the app endpoints (JWT) and the n8n callback (shared secret).
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(f) }
	mux.Handle("POST /api/complaints", guard(h.Create))
	mux.Handle("GET /api/complaints", guard(h.List))
	mux.Handle("GET /api/complaints/summary", guard(h.Summary))
	mux.Handle("GET /api/complaints/{id}", guard(h.Get))
	mux.Handle("PATCH /api/complaints/{id}/status", guard(h.SetStatus))
	mux.Handle("POST /api/complaints/{id}/retry", guard(h.Retry))
	mux.Handle("POST /api/n8n/requests", guard(h.Forward))
	mux.HandleFunc("POST /api/n8n/complaints/callback", h.Callback)
}

func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	return id.Email
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "complaint not found")
		return 0, false
	}
	return id, true
}

// Create saves a complaint and starts its AI analysis. It answers 202: the
// analysis arrives later (poll GET /api/complaints/{id}).
//
// The body is the n8n smart-router shape. Type "ai" (the default) is a
// complaint: saved, analysed and retried. Types "erp" and "email" are not
// complaints: they are forwarded to n8n once, like POST /api/n8n/requests, and
// n8n's answer is returned with 200.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input
		Recipient string `json:"recipient"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if t := strings.ToLower(strings.TrimSpace(body.Type)); t == "erp" || t == "email" {
		h.forward(w, r, RouterRequest{Title: body.Title, Subject: body.Subject, Message: body.Message, Type: t, Recipient: body.Recipient})
		return
	}
	in := body.Input
	if errs := in.Normalize(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	id, err := h.repo.Create(r.Context(), in, user(r))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	h.queue.Enqueue(id)
	c, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, c)
}

var (
	aiStatuses = map[string]bool{AIPending: true, AIProcessing: true, AIAnalyzed: true, AIFailed: true}
	sentimentV = map[string]bool{"positive": true, "neutral": true, "negative": true}
	priorityV  = map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}
)

// List answers ?aiStatus=&handlingStatus=&sentiment=&priority=&category=&q=&page=&pageSize=
// sorted by priority (urgent first), then newest.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := Filter{
		AIStatus:       q.Get("aiStatus"),
		HandlingStatus: q.Get("handlingStatus"),
		Sentiment:      strings.ToLower(q.Get("sentiment")),
		Priority:       strings.ToLower(q.Get("priority")),
		Category:       strings.ToLower(strings.TrimSpace(q.Get("category"))),
		Query:          strings.TrimSpace(q.Get("q")),
		Page:           1,
		PageSize:       20,
	}
	errs := httpx.FieldErrors{}
	check := func(field, v string, allowed map[string]bool) {
		if v != "" && !allowed[v] {
			errs[field] = "Unknown value"
		}
	}
	check("aiStatus", f.AIStatus, aiStatuses)
	check("handlingStatus", f.HandlingStatus, handlingStatuses)
	check("sentiment", f.Sentiment, sentimentV)
	check("priority", f.Priority, priorityV)
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			f.Page = n
		} else {
			errs["page"] = "Page must be a number from 1"
		}
	}
	if v := q.Get("pageSize"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			f.PageSize = n
		} else {
			errs["pageSize"] = "Page size must be from 1 to 100"
		}
	}
	if len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	list, total, err := h.repo.List(r.Context(), f)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"complaints": list, "total": total, "page": f.Page, "pageSize": f.PageSize})
}

// Summary is the dashboard: counts per category, sentiment, priority and status.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.repo.Summary(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s)
}

// Get returns a complaint with its AI pipeline log.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	logs, err := h.repo.Logs(r.Context(), id)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"complaint": c, "logs": logs})
}

// SetStatus updates the handling status: {"handlingStatus": "in_progress", "note": "..."}.
func (h *Handler) SetStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		HandlingStatus string `json:"handlingStatus"`
		Note           string `json:"note"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	body.HandlingStatus = strings.TrimSpace(body.HandlingStatus)
	body.Note = strings.TrimSpace(body.Note)
	if !handlingStatuses[body.HandlingStatus] {
		httpx.WriteValidation(w, httpx.FieldErrors{"handlingStatus": "Must be open, in_progress, resolved or closed"})
		return
	}
	if utf8.RuneCountInString(body.Note) > 2000 {
		httpx.WriteValidation(w, httpx.FieldErrors{"note": "Note must be at most 2000 characters"})
		return
	}
	if err := h.repo.SetHandling(r.Context(), id, body.HandlingStatus, body.Note, user(r)); err != nil {
		writeRepoError(w, err)
		return
	}
	c, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c)
}

// Retry sends a complaint to n8n again (e.g. after it failed, or when the
// callback never came). 409 while a dispatch is still running.
func (h *Handler) Retry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if c.AIStatus == AIProcessing && r.URL.Query().Get("force") != "true" {
		httpx.WriteError(w, http.StatusConflict, "analysis is still processing; add ?force=true if the callback never arrived")
		return
	}
	if c.AIStatus == AIProcessing {
		// force: release it so the dispatcher can take it again.
		if err := h.repo.MarkFailed(r.Context(), id, "retried manually"); err != nil {
			httpx.InternalError(w, err)
			return
		}
	}
	h.queue.Enqueue(id)
	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"id": id, "message": "queued for analysis"})
}

// callbackBody is what n8n posts back. The analysis may also be sent at the
// top level ({"complaintId": 1, "category": ..., "sentiment": ...}).
type callbackBody struct {
	ComplaintID json.Number     `json:"complaintId"`
	Analysis    json.RawMessage `json:"analysis"`
	Error       string          `json:"error"`
}

// Callback receives the analysis from n8n. It requires the X-Webhook-Secret
// header and validates the AI output before saving it; an invalid output marks
// the complaint failed and answers 422 with the reasons.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	if h.secret == "" {
		httpx.WriteError(w, http.StatusServiceUnavailable, "callback is disabled: N8N_WEBHOOK_SECRET is not set")
		return
	}
	got := r.Header.Get(SecretHeader)
	if subtle.ConstantTimeCompare([]byte(got), []byte(h.secret)) != 1 {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid webhook secret")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var body callbackBody
	if err := json.Unmarshal(raw, &body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id, err := strconv.ParseInt(body.ComplaintID.String(), 10, 64)
	if err != nil || id <= 0 {
		httpx.WriteValidation(w, httpx.FieldErrors{"complaintId": "complaintId is required"})
		return
	}
	ctx := r.Context()
	if _, err := h.repo.Get(ctx, id); err != nil {
		writeRepoError(w, err)
		return
	}

	if msg := strings.TrimSpace(body.Error); msg != "" {
		msg = truncate(msg, 1000)
		_ = h.repo.AddLog(ctx, id, Log{Event: "callback_error", Message: msg})
		if err := h.repo.MarkFailed(ctx, id, "n8n reported: "+msg); err != nil {
			httpx.InternalError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "aiStatus": AIFailed})
		return
	}

	analysis := []byte(body.Analysis)
	if len(analysis) == 0 || string(analysis) == "null" {
		analysis = raw
	}
	if err := Apply(ctx, h.repo, id, 0, analysis); err != nil {
		var verr *ValidationError
		if errors.As(err, &verr) {
			httpx.WriteJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "AI output failed validation", "errors": verr.Fields})
			return
		}
		writeRepoError(w, err)
		return
	}
	c, err := h.repo.Get(ctx, id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, c)
}

var routerTypes = map[string]bool{"ai": true, "erp": true, "email": true}

// Forward passes a smart-router request {title, subject, message, type,
// recipient} to n8n as is and returns n8n's answer. Use it for erp and email;
// complaints that should be stored and analysed go to POST /api/complaints.
func (h *Handler) Forward(w http.ResponseWriter, r *http.Request) {
	var req RouterRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	h.forward(w, r, req)
}

// forward validates a smart-router request, sends it to n8n and writes n8n's answer.
func (h *Handler) forward(w http.ResponseWriter, r *http.Request, req RouterRequest) {
	for _, f := range []*string{&req.Title, &req.Subject, &req.Message, &req.Type, &req.Recipient} {
		*f = strings.TrimSpace(*f)
	}
	req.Type = strings.ToLower(req.Type)
	req.ComplaintID, req.CallbackURL = 0, ""
	errs := httpx.FieldErrors{}
	if !routerTypes[req.Type] {
		errs["type"] = "Type must be ai, erp or email"
	}
	if req.Title == "" {
		errs["title"] = "Title is required"
	}
	if req.Subject == "" {
		errs["subject"] = "Subject is required"
	}
	if req.Message == "" {
		errs["message"] = "Message is required"
	}
	for field, n := range map[string]int{"title": 100, "subject": 150, "message": 5000, "recipient": 254} {
		v := map[string]string{"title": req.Title, "subject": req.Subject, "message": req.Message, "recipient": req.Recipient}[field]
		if _, set := errs[field]; !set && utf8.RuneCountInString(v) > n {
			errs[field] = "Too long"
		}
	}
	if req.Type == "email" {
		if a, err := mail.ParseAddress(req.Recipient); err != nil || a.Address != req.Recipient {
			errs["recipient"] = "Recipient must be an email address"
		}
	}
	if len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	status, body, err := h.queue.Forward(r.Context(), req)
	switch {
	case errors.Is(err, ErrNotConfigured):
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	case err != nil:
		httpx.WriteJSON(w, http.StatusBadGateway, map[string]string{"error": "n8n request failed", "detail": err.Error()})
		return
	}
	var parsed any = string(body)
	if json.Valid(body) && len(body) > 0 {
		parsed = json.RawMessage(body)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"n8nStatus": status, "response": parsed})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "complaint not found")
	default:
		httpx.InternalError(w, err)
	}
}
