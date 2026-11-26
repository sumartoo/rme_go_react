package complaint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// Analysis is the AI result for one complaint, after validation.
type Analysis struct {
	Category  string `json:"category"`
	Sentiment string `json:"sentiment"` // positive, neutral, negative
	Priority  string `json:"priority"`  // low, medium, high, urgent
	Summary   string `json:"summary"`
}

// ValidationError lists why an AI output was rejected.
type ValidationError struct {
	Fields httpx.FieldErrors
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for k, v := range e.Fields {
		parts = append(parts, k+": "+v)
	}
	return "invalid AI output: " + strings.Join(parts, "; ")
}

// The model may answer in English or Indonesian; both map to one stored value.
var sentiments = map[string]string{
	"positive": "positive", "positif": "positive",
	"neutral": "neutral", "netral": "neutral",
	"negative": "negative", "negatif": "negative",
}

var priorities = map[string]string{
	"low": "low", "rendah": "low",
	"medium": "medium", "sedang": "medium", "normal": "medium",
	"high": "high", "tinggi": "high",
	"urgent": "urgent", "critical": "urgent", "kritis": "urgent", "mendesak": "urgent",
}

var keyAliases = map[string]string{"kategori": "category", "sentimen": "sentiment", "prioritas": "priority", "ringkasan": "summary"}

var categoryPattern = regexp.MustCompile(`^[\p{L}\p{N} _&/-]+$`)

// wrapperKeys are fields n8n nodes commonly put the model's answer under
// (AI Agent: "output", Basic LLM Chain: "text").
var wrapperKeys = []string{"analysis", "output", "text", "result", "data"}

const maxSummaryLen = 2000

// ParseAnalysis validates an AI output against the schema
//
//	{"category": string(1-50), "sentiment": positive|neutral|negative,
//	 "priority": low|medium|high|urgent, "summary": string (optional, <= 2000)}
//
// The output may be a JSON object, or a string holding one (LLMs often wrap
// it in ```json fences or add text around it), optionally nested one level
// under a wrapper key such as "output". It returns the normalized analysis.
func ParseAnalysis(raw []byte) (Analysis, error) {
	obj, err := toObject(raw, 0)
	if err != nil {
		return Analysis{}, &ValidationError{Fields: httpx.FieldErrors{"analysis": err.Error()}}
	}
	// A structured output parser may use Indonesian keys.
	for id, en := range keyAliases {
		if _, ok := obj[en]; !ok {
			if v, ok := obj[id]; ok {
				obj[en] = v
			}
		}
	}

	errs := httpx.FieldErrors{}
	str := func(key string) string {
		v, ok := obj[key]
		if !ok || v == nil {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			errs[key] = "must be a string"
			return ""
		}
		return strings.TrimSpace(s)
	}

	var a Analysis
	a.Category = strings.ToLower(str("category"))
	switch {
	case errs["category"] != "":
	case a.Category == "":
		errs["category"] = "is required"
	case utf8.RuneCountInString(a.Category) > 50:
		errs["category"] = "must be at most 50 characters"
	case !categoryPattern.MatchString(a.Category):
		errs["category"] = "may only contain letters, digits, spaces and _ & / -"
	}

	sentiment := strings.ToLower(str("sentiment"))
	if errs["sentiment"] == "" {
		if a.Sentiment = sentiments[sentiment]; a.Sentiment == "" {
			errs["sentiment"] = fmt.Sprintf("must be positive, neutral or negative (got %q)", sentiment)
		}
	}

	priority := strings.ToLower(str("priority"))
	if errs["priority"] == "" {
		if a.Priority = priorities[priority]; a.Priority == "" {
			errs["priority"] = fmt.Sprintf("must be low, medium, high or urgent (got %q)", priority)
		}
	}

	a.Summary = str("summary")
	if errs["summary"] == "" && utf8.RuneCountInString(a.Summary) > maxSummaryLen {
		errs["summary"] = fmt.Sprintf("must be at most %d characters", maxSummaryLen)
	}

	if len(errs) > 0 {
		return Analysis{}, &ValidationError{Fields: errs}
	}
	return a, nil
}

// LooksLikeAnalysis reports whether raw seems to carry an analysis at all, so a
// plain acknowledgement from n8n ({"message":"Workflow was started"}) is not
// mistaken for a broken one.
func LooksLikeAnalysis(raw []byte) bool {
	return bytes.Contains(bytes.ToLower(raw), []byte("sentimen")) // also matches "sentiment"
}

func toObject(raw []byte, depth int) (map[string]any, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, fmt.Errorf("is empty")
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		// Not JSON at all: maybe free text with a JSON object inside.
		return objectFromText(string(raw), depth)
	}
	switch t := v.(type) {
	case map[string]any:
		if hasSentiment(t) || depth >= 2 {
			return t, nil
		}
		for _, k := range wrapperKeys {
			if inner, ok := t[k]; ok && inner != nil {
				b, _ := json.Marshal(inner)
				return toObject(b, depth+1)
			}
		}
		return t, nil
	case []any:
		// n8n items: [{...}]
		if len(t) == 1 && depth < 2 {
			b, _ := json.Marshal(t[0])
			return toObject(b, depth+1)
		}
		return nil, fmt.Errorf("must be a JSON object, got an array")
	case string:
		if depth >= 2 {
			return nil, fmt.Errorf("must be a JSON object")
		}
		return objectFromText(t, depth+1)
	default:
		return nil, fmt.Errorf("must be a JSON object")
	}
}

var fence = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

func objectFromText(s string, depth int) (map[string]any, error) {
	if m := fence.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("does not contain a JSON object")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s[start:end+1]), &obj); err != nil {
		return nil, fmt.Errorf("is not valid JSON: %v", err)
	}
	if !hasSentiment(obj) && depth < 2 {
		b, _ := json.Marshal(obj)
		return toObject(b, depth+1)
	}
	return obj, nil
}

func hasSentiment(m map[string]any) bool {
	_, en := m["sentiment"]
	_, id := m["sentimen"]
	return en || id
}
