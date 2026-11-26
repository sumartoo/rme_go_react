// Package complaint stores customer complaints, sends them to the n8n workflow
// for AI analysis (category, sentiment, priority), receives the result on a
// callback, and serves a list and a dashboard summary.
//
// Flow: app -> POST /api/complaints (JWT) -> saved as pending -> Dispatcher
// posts it to N8N_WEBHOOK_URL (X-Webhook-Secret, retry with backoff) -> n8n
// answers with the analysis, or later posts it to
// POST /api/n8n/complaints/callback (X-Webhook-Secret) -> validated -> saved.
package complaint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var ErrNotFound = errors.New("complaint not found")

const (
	AIPending    = "pending"
	AIProcessing = "processing"
	AIAnalyzed   = "analyzed"
	AIFailed     = "failed"
)

var handlingStatuses = map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}

// Complaint is a complaints row.
type Complaint struct {
	ID              int64      `json:"id"`
	Title           string     `json:"title"`
	Subject         string     `json:"subject"`
	CustomerName    string     `json:"customerName"`
	CustomerContact string     `json:"customerContact"`
	Channel         string     `json:"channel"`
	Message         string     `json:"message"`
	Category        *string    `json:"category"`
	Sentiment       *string    `json:"sentiment"`
	Priority        *string    `json:"priority"`
	Summary         *string    `json:"summary"`
	AIStatus        string     `json:"aiStatus"`
	AIError         *string    `json:"aiError"`
	AIAttempts      int        `json:"aiAttempts"`
	HandlingStatus  string     `json:"handlingStatus"`
	HandlingNote    *string    `json:"handlingNote"`
	CreatedBy       string     `json:"createdBy"`
	UpdatedBy       *string    `json:"updatedBy"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	AnalyzedAt      *time.Time `json:"analyzedAt"`
}

// Log is one event of the AI pipeline for a complaint.
type Log struct {
	ID         int64     `json:"id"`
	Attempt    int       `json:"attempt"`
	Event      string    `json:"event"` // sent, error, analyzed, invalid_output, callback_error, failed
	HTTPStatus *int      `json:"httpStatus"`
	Message    string    `json:"message"`
	DurationMs *int      `json:"durationMs"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Defaults of the n8n smart-router body for a complaint.
const (
	DefaultTitle   = "AI Assistant"
	DefaultSubject = "Analyze Customer Complaint"
	TypeAI         = "ai"
)

// Input is a new complaint from the app, in the n8n smart-router shape
// {title, subject, message, type}. Only message is required. The handler sends
// types erp and email straight to n8n, so Normalize only sees "ai".
type Input struct {
	Title           string `json:"title"`
	Subject         string `json:"subject"`
	Message         string `json:"message"`
	Type            string `json:"type"`
	CustomerName    string `json:"customerName"`
	CustomerContact string `json:"customerContact"`
	Channel         string `json:"channel"`
}

// Normalize trims text and validates against the column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	for _, f := range []*string{&in.Title, &in.Subject, &in.Type, &in.CustomerName, &in.CustomerContact, &in.Channel, &in.Message} {
		*f = strings.TrimSpace(*f)
	}
	if in.Title == "" {
		in.Title = DefaultTitle
	}
	if in.Subject == "" {
		in.Subject = DefaultSubject
	}
	in.Type = strings.ToLower(in.Type)
	if in.Type == "" {
		in.Type = TypeAI
	}
	if in.Channel == "" {
		in.Channel = "app"
	}
	errs := httpx.FieldErrors{}
	maxLen := func(field, label, v string, n int) {
		if _, done := errs[field]; !done && utf8.RuneCountInString(v) > n {
			errs[field] = fmt.Sprintf("%s must be at most %d characters", label, n)
		}
	}
	if in.Type != TypeAI {
		errs["type"] = "Type must be ai, erp or email"
	}
	if in.Message == "" {
		errs["message"] = "Message is required"
	}
	maxLen("title", "Title", in.Title, 100)
	maxLen("subject", "Subject", in.Subject, 150)
	maxLen("customerName", "Customer name", in.CustomerName, 100)
	maxLen("customerContact", "Customer contact", in.CustomerContact, 100)
	maxLen("channel", "Channel", in.Channel, 30)
	maxLen("message", "Message", in.Message, 5000)
	return errs
}

// Filter narrows the list; empty fields match everything.
type Filter struct {
	AIStatus       string
	HandlingStatus string
	Sentiment      string
	Priority       string
	Category       string
	Query          string // in customer name or message
	Page           int
	PageSize       int
}

// Summary is the dashboard: counts per value of each dimension.
type Summary struct {
	Total            int            `json:"total"`
	ByCategory       map[string]int `json:"byCategory"`
	BySentiment      map[string]int `json:"bySentiment"`
	ByPriority       map[string]int `json:"byPriority"`
	ByAIStatus       map[string]int `json:"byAiStatus"`
	ByHandlingStatus map[string]int `json:"byHandlingStatus"`
}

type Repository interface {
	Create(ctx context.Context, in Input, user string) (int64, error)
	Get(ctx context.Context, id int64) (Complaint, error)
	List(ctx context.Context, f Filter) ([]Complaint, int, error)
	Summary(ctx context.Context) (Summary, error)
	Logs(ctx context.Context, id int64) ([]Log, error)
	// AddLog records an event; events "sent" and "error" also count an attempt.
	AddLog(ctx context.Context, id int64, l Log) error
	// MarkProcessing moves a complaint to processing unless it already is;
	// false means another dispatch is running.
	MarkProcessing(ctx context.Context, id int64) (bool, error)
	MarkFailed(ctx context.Context, id int64, reason string) error
	SaveAnalysis(ctx context.Context, id int64, a Analysis) error
	SetHandling(ctx context.Context, id int64, status, note, user string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const columns = `id, title, subject, customer_name, COALESCE(customer_contact, ''), channel, message, category, sentiment, priority, summary,
	ai_status, ai_error, ai_attempts, handling_status, handling_note, created_by, updated_by, created_at, updated_at, analyzed_at`

func scan(r pgx.CollectableRow) (Complaint, error) {
	var c Complaint
	err := r.Scan(&c.ID, &c.Title, &c.Subject, &c.CustomerName, &c.CustomerContact, &c.Channel, &c.Message, &c.Category, &c.Sentiment, &c.Priority, &c.Summary,
		&c.AIStatus, &c.AIError, &c.AIAttempts, &c.HandlingStatus, &c.HandlingNote, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt, &c.AnalyzedAt)
	return c, err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int64, error) {
	var id int64
	err := p.pool.QueryRow(ctx,
		`INSERT INTO complaints (title, subject, customer_name, customer_contact, channel, message, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		in.Title, in.Subject, in.CustomerName, nullIfEmpty(in.CustomerContact), in.Channel, in.Message, user).Scan(&id)
	return id, err
}

func (p *PostgresRepository) Get(ctx context.Context, id int64) (Complaint, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+columns+` FROM complaints WHERE id = $1`, id)
	if err != nil {
		return Complaint{}, err
	}
	c, err := pgx.CollectOneRow(rows, scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return Complaint{}, ErrNotFound
	}
	return c, err
}

func (p *PostgresRepository) List(ctx context.Context, f Filter) ([]Complaint, int, error) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.AIStatus != "" {
		add("ai_status = $%d", f.AIStatus)
	}
	if f.HandlingStatus != "" {
		add("handling_status = $%d", f.HandlingStatus)
	}
	if f.Sentiment != "" {
		add("sentiment = $%d", f.Sentiment)
	}
	if f.Priority != "" {
		add("priority = $%d", f.Priority)
	}
	if f.Category != "" {
		add("category = $%d", f.Category)
	}
	if f.Query != "" {
		add("(customer_name ILIKE $%[1]d OR message ILIKE $%[1]d OR subject ILIKE $%[1]d)", "%"+f.Query+"%")
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM complaints WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := p.pool.Query(ctx,
		fmt.Sprintf(`SELECT %s FROM complaints WHERE %s
		 ORDER BY CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 WHEN 'low' THEN 3 ELSE 4 END, created_at DESC
		 LIMIT $%d OFFSET $%d`, columns, cond, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if list == nil {
		list = []Complaint{}
	}
	return list, total, err
}

// summaryColumns are the grouped columns; constants, never user input.
var summaryColumns = []string{"category", "sentiment", "priority", "ai_status", "handling_status"}

func (p *PostgresRepository) Summary(ctx context.Context) (Summary, error) {
	s := Summary{}
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM complaints`).Scan(&s.Total); err != nil {
		return s, err
	}
	targets := []*map[string]int{&s.ByCategory, &s.BySentiment, &s.ByPriority, &s.ByAIStatus, &s.ByHandlingStatus}
	for i, col := range summaryColumns {
		rows, err := p.pool.Query(ctx,
			`SELECT COALESCE(`+col+`, 'unanalyzed'), count(*) FROM complaints GROUP BY 1`)
		if err != nil {
			return s, err
		}
		m := map[string]int{}
		var k string
		var n int
		if _, err := pgx.ForEachRow(rows, []any{&k, &n}, func() error { m[k] = n; return nil }); err != nil {
			return s, err
		}
		*targets[i] = m
	}
	return s, nil
}

func (p *PostgresRepository) Logs(ctx context.Context, id int64) ([]Log, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, attempt, event, http_status, COALESCE(message, ''), duration_ms, created_at
		 FROM complaint_ai_logs WHERE complaint_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Log, error) {
		var l Log
		err := r.Scan(&l.ID, &l.Attempt, &l.Event, &l.HTTPStatus, &l.Message, &l.DurationMs, &l.CreatedAt)
		return l, err
	})
	if list == nil {
		list = []Log{}
	}
	return list, err
}

func (p *PostgresRepository) AddLog(ctx context.Context, id int64, l Log) error {
	return pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO complaint_ai_logs (complaint_id, attempt, event, http_status, message, duration_ms)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			id, l.Attempt, l.Event, l.HTTPStatus, nullIfEmpty(l.Message), l.DurationMs); err != nil {
			return err
		}
		if l.Event == "sent" || l.Event == "error" {
			_, err := tx.Exec(ctx, `UPDATE complaints SET ai_attempts = ai_attempts + 1, updated_at = now() WHERE id = $1`, id)
			return err
		}
		return nil
	})
}

func (p *PostgresRepository) MarkProcessing(ctx context.Context, id int64) (bool, error) {
	tag, err := p.pool.Exec(ctx,
		`UPDATE complaints SET ai_status = 'processing', ai_error = NULL, updated_at = now()
		 WHERE id = $1 AND ai_status <> 'processing'`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (p *PostgresRepository) MarkFailed(ctx context.Context, id int64, reason string) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE complaints SET ai_status = 'failed', ai_error = $2, updated_at = now() WHERE id = $1`, id, reason)
	return err
}

func (p *PostgresRepository) SaveAnalysis(ctx context.Context, id int64, a Analysis) error {
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	tag, err := p.pool.Exec(ctx,
		`UPDATE complaints SET category = $2, sentiment = $3, priority = $4, summary = $5, ai_raw = $6,
		        ai_status = 'analyzed', ai_error = NULL, analyzed_at = now(), updated_at = now()
		 WHERE id = $1`,
		id, a.Category, a.Sentiment, a.Priority, nullIfEmpty(a.Summary), string(raw))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) SetHandling(ctx context.Context, id int64, status, note, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE complaints SET handling_status = $2, handling_note = $3, updated_by = $4, updated_at = now() WHERE id = $1`,
		id, status, nullIfEmpty(note), user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
