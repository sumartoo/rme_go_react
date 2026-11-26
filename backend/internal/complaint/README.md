# Complaint analysis via n8n (Part 2)

```
App ──JWT──▶ POST /api/complaints ──▶ complaints (pending)
                                  └─▶ Dispatcher ──X-Webhook-Secret──▶ n8n smart router (type=ai)
                                        retry 408/429/5xx/timeout,          │ Switch on "type"
                                        backoff 2s, 4s, ...                 │ DeepSeek analysis
                                        log each attempt                    ▼
complaints (analyzed) ◀── validate schema ◀── POST /api/n8n/complaints/callback (X-Webhook-Secret)
                                               (or the analysis in n8n's direct response)
```

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `N8N_WEBHOOK_URL` | empty | n8n Webhook node URL. Empty: complaints are saved and marked failed. |
| `N8N_WEBHOOK_SECRET` | empty | At least 16 chars. Sent to n8n and required on the callback as `X-Webhook-Secret`. |
| `N8N_TIMEOUT` | `30s` | Timeout per attempt. |
| `N8N_MAX_RETRIES` | `3` | Total attempts (1 to 10). |
| `PUBLIC_BASE_URL` | empty | How n8n reaches this backend; sent to n8n as `callbackUrl`. |

## What n8n receives

```http
POST {N8N_WEBHOOK_URL}
Content-Type: application/json
X-Webhook-Secret: {N8N_WEBHOOK_SECRET}

{"title":"AI Assistant","subject":"Analyze Customer Complaint",
 "message":"Pelanggan mengeluhkan keterlambatan pengiriman","type":"ai",
 "complaintId":12,"callbackUrl":"https://api.example.com/api/n8n/complaints/callback"}
```

This is the smart-router body (`title`, `subject`, `message`, `type`, and `recipient` for email). `complaintId` and `callbackUrl` are extra; the workflow can ignore them unless it uses the callback.

For `erp` and `email`, the app can post the same body to **POST /api/complaints** or **POST /api/n8n/requests** (both JWT); the backend forwards it as is and returns n8n's answer. Nothing is stored, and only `ai` is retried, so a timeout cannot create a purchase order or send an email twice.

In n8n, check the secret first (an IF node on `{{$json.headers["x-webhook-secret"]}}`, or Header Auth on the Webhook node).

## Two ways to return the result

1. **Direct** (what the smart router's AI branch does): "Respond to Webhook AI" returns the analysis. The backend validates and saves it right away. Keep the workflow under `N8N_TIMEOUT`.
2. **Callback** (recommended when DeepSeek can be slow): the Webhook answers immediately, and at the end an HTTP Request node posts to `callbackUrl`:

```http
POST /api/n8n/complaints/callback
X-Webhook-Secret: {N8N_WEBHOOK_SECRET}

{"complaintId":12,"analysis":{"category":"pengiriman","sentiment":"negative","priority":"high","summary":"Paket terlambat 5 hari"}}
```

To report a failure inside n8n: `{"complaintId":12,"error":"DeepSeek timeout"}`.

## Schema of the AI output

Indonesian keys `kategori`, `sentimen`, `prioritas`, `ringkasan` are accepted too.

| Field | Rule |
|---|---|
| `category` | required, 1 to 50 chars, letters/digits/space/`_&/-`, stored lowercase |
| `sentiment` | `positive`, `neutral`, `negative` (also `positif`, `netral`, `negatif`) |
| `priority` | `low`, `medium`, `high`, `urgent` (also `rendah`, `sedang`, `tinggi`, `mendesak`, `critical`) |
| `summary` | optional, at most 2000 chars |

`analysis` may be an object or a string holding one: ```json fences, text around the object, and wrappers such as `{"output": "..."}` (AI Agent node) or `[{...}]` are unwrapped. An invalid output is not saved: the complaint becomes `failed`, the reason goes to `aiError` and the log, and the callback answers 422 with the field errors.

Suggested DeepSeek system prompt: *"Answer ONLY with JSON: {"category": string, "sentiment": "positive"|"neutral"|"negative", "priority": "low"|"medium"|"high"|"urgent", "summary": string}"*.

## App endpoints (Bearer JWT)

| Method | Path | |
|---|---|---|
| POST | `/api/complaints` | `{"title","subject","message","type","recipient"}`. `ai` (default; only `message` required) → 202, saved and analysed. `erp`/`email` → 200 with n8n's answer, not stored |
| POST | `/api/n8n/requests` | `{"title","subject","message","type":"ai/erp/email","recipient"}` → n8n's answer |
| GET | `/api/complaints` | list; `?aiStatus=&handlingStatus=&sentiment=&priority=&category=&q=&page=&pageSize=`, urgent first |
| GET | `/api/complaints/summary` | dashboard counts by category, sentiment, priority, aiStatus, handlingStatus |
| GET | `/api/complaints/{id}` | complaint + AI log (every attempt, HTTP status, duration, error) |
| PATCH | `/api/complaints/{id}/status` | `{"handlingStatus":"open|in_progress|resolved|closed","note":"..."}` |
| POST | `/api/complaints/{id}/retry` | send to n8n again; `?force=true` if stuck in processing |

## Try it

```bash
TOKEN=$(curl -s localhost:3001/api/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@medika.com","password":"Medika123"}' | jq -r .token)
curl -s localhost:3001/api/complaints -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"AI Assistant","subject":"Analyze Customer Complaint","message":"Pelanggan mengeluhkan keterlambatan pengiriman","type":"ai"}'
curl -s localhost:3001/api/complaints/1 -H "Authorization: Bearer $TOKEN"
curl -s localhost:3001/api/complaints/summary -H "Authorization: Bearer $TOKEN"
```
