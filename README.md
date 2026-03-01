<div align="center">

# LLM Gateway

**Go-native LLM proxy. Single binary. Zero Python.**

A drop-in [LiteLLM](https://github.com/BerriAI/litellm) alternative built on [GoFr](https://gofr.dev) — ship a single static binary with routing, guardrails, virtual keys, cost tracking, batch processing, and an admin dashboard out of the box.

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev)
[![GoFr](https://img.shields.io/badge/Built%20with-GoFr-6C63FF?style=for-the-badge)](https://gofr.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-green?style=for-the-badge)](LICENSE)
[![Providers](https://img.shields.io/badge/Providers-6-orange?style=for-the-badge)](#providers)

[Quick Start](#quick-start) · [API Reference](#api-reference) · [Configuration](#configuration) · [Admin UI](#admin-dashboard) · [Comparison](#feature-comparison-llm-gateway-vs-litellm)

<br>

<img src="https://github.com/user-attachments/assets/83f4741a-7d02-46da-af32-57656c5b9b1f" alt="LLM Gateway Admin Dashboard" width="100%">

</div>

---

## Why LLM Gateway?

- **Single binary** — `go build` and deploy. No virtualenv, no pip, no Docker required.
- **~15 MB memory** — vs ~200 MB+ for Python-based proxies.
- **< 100 ms startup** — vs 3-5 seconds for LiteLLM.
- **Batteries included** — routing, retries, circuit breakers, caching, guardrails, budgets, virtual keys, batch API, and admin UI in one binary.
- **OpenAI-compatible** — swap `base_url` in any OpenAI SDK and go.

---

## Architecture

```
              ┌──────────────────────────────────────────────────────┐
              │                    LLM Gateway                       │
              │                                                      │
┌─────────┐  │  ┌──────────┐   ┌──────────┐   ┌──────────────┐     │  ┌──────────┐
│  Client  │──│─▶│Middleware│──▶│  Router  │──▶│   Provider   │──── │─▶│  OpenAI  │
│ (OpenAI  │  │  │  (Auth,  │   │ (Retry,  │   │ (Translate,  │     │  ├──────────┤
│  SDK)    │◀─│──│  Rate    │   │ Cooldown,│   │   Stream,    │──── │─▶│Anthropic │
└─────────┘  │  │  Limit,  │   │ Strategy)│   │   Fallback)  │     │  ├──────────┤
             │  │  Guard-  │   └─────┬────┘   └──────────────┘     │  │  Gemini  │
┌─────────┐  │  │  rails)  │        │               │              │  ├──────────┤
│  Admin   │──│─▶└─────┬───┘   ┌────▼─────┐   ┌────▼─────┐       │  │   Groq   │
│   UI     │  │        │       │ Trackers  │   │  Cache   │       │  ├──────────┤
└─────────┘  │  ┌──────▼──┐   │(InFlight, │   │ (Redis)  │       │  │ DeepSeek │
             │  │KeyStore  │   │ Latency,  │   └──────────┘       │  ├──────────┤
             │  │ (In-Mem) │   │  Usage)   │        │              │  │  Ollama  │
             │  └─────┬────┘   └──────────┘   ┌────▼─────┐       │  └──────────┘
             │  ┌─────▼───────────────────┐   │  Budget  │       │
             │  │      PostgreSQL          │   │ Tracking │       │
             │  │ (Keys, Teams, Audit,     │   └──────────┘       │
             │  │  Guardrails, Batches)    │                      │
             │  └─────────────────────────┘                      │
             └──────────────────────────────────────────────────────┘
```

---

## Features

### Providers

All providers expose the same OpenAI-compatible API. Use `provider/model` format to route to any backend.

| Provider | Chat | Stream | Embeddings | Function Calling | Format Translation |
|----------|:----:|:------:|:----------:|:----------------:|:------------------:|
| **OpenAI** | Yes | Yes | Yes | Native | Passthrough |
| **Anthropic** | Yes | Yes | — | Native | Full (OpenAI ↔ Anthropic) |
| **Gemini** | Yes | Yes | — | Native | Full (OpenAI ↔ Gemini) |
| **Groq** | Yes | Yes | — | Native | Passthrough |
| **DeepSeek** | Yes | Yes | — | Native | Passthrough |
| **Ollama** | Yes | Yes | Yes | Supported models | Full (OpenAI ↔ Ollama) |

### Routing & Reliability

| Strategy | Description |
|----------|-------------|
| `simple` | First available deployment |
| `round-robin` | Rotate through deployments |
| `weighted` | Weighted random distribution |
| `least-busy` | Lowest in-flight request count |
| `latency` | Lowest exponential moving average latency |
| `usage` | Lowest token usage in sliding window |

Plus: exponential backoff with jitter, cooldown tracking, GoFr-native circuit breakers, per-provider timeouts, error-aware fallback chains, and a channel-based request queue.

### Guardrails & Content Filtering

Pre-call and post-call content filtering applied per request. Configurable globally via env or per-key via database.

| Check | Stage | Description |
|-------|-------|-------------|
| **Blocked keywords** | Pre-call | Case-insensitive substring match against a configurable blocklist |
| **PII detection** | Pre-call | Regex-based detection of email, phone, SSN, credit card, IPv4 |
| **PII blocking** | Pre-call | Reject requests containing PII (`pii_action=block`) |
| **PII redaction** | Post-call | Replace PII in responses with `[REDACTED_*]` placeholders (`pii_action=redact`) |
| **PII logging** | Post-call | Log PII types found without modifying the response (`pii_action=log`) |
| **Input length** | Pre-call | Approximate token count limit on input messages |
| **Output length** | Post-call | Truncate responses exceeding configured token limit |

### Batch Processing API

OpenAI-compatible batch endpoint for submitting multiple requests at once. Backed by a configurable worker pool with panic recovery, per-task timeouts, and graceful shutdown.

```bash
# Submit a batch
curl -X POST http://localhost:9000/v1/batches \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"requests":[
    {"custom_id":"req-1","method":"POST","url":"/v1/chat/completions",
     "body":{"model":"openai/gpt-4o","messages":[{"role":"user","content":"Hello"}]}},
    {"custom_id":"req-2","method":"POST","url":"/v1/chat/completions",
     "body":{"model":"anthropic/claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hi"}]}}
  ]}'

# Check status
curl http://localhost:9000/v1/batches/{id} -H "Authorization: Bearer $KEY"

# Retrieve results
curl http://localhost:9000/v1/batches/{id}/results -H "Authorization: Bearer $KEY"
```

### Virtual Keys & Multi-Tenancy

- Generate virtual API keys with per-key rate limits (RPM + TPM), budgets, model restrictions, and expiry
- In-memory keystore with write-through to PostgreSQL — zero DB calls in the auth hot path
- Teams, users, organizations CRUD with hierarchical budgets
- Full audit trail for all admin operations
- Tag-based routing for deployment targeting

### Cost Tracking & Budgets

- Real-time cost calculation returned in every response (`"cost": 0.0023`)
- Per-key, per-user, per-team, per-org budget enforcement with automatic blocking
- Spend reporting with flexible `GROUP BY` (provider, model, key, team, user, org)
- Budget alerts at 50%, 80%, 100% thresholds
- Custom pricing overrides for self-hosted or fine-tuned models
- Prometheus counters and histograms via GoFr

### Admin Dashboard

Built-in single-page admin UI at `/admin` — no separate frontend deployment needed.

- **Dashboard** — gateway health, model count, key count, batch summary
- **Virtual Keys** — list, generate, delete keys with rate limits and budgets
- **Spend** — cost breakdown by model, provider, or team
- **Batches** — monitor batch jobs, view progress, cancel or inspect results
- **Guardrails** — configuration reference for env vars and per-key overrides
- **Playground** — test API calls directly from your browser
- **API Docs** — complete built-in documentation with examples
- **Settings** — view and manage provider configuration, routing, and security

Authenticate with your master key or a virtual key. Static assets are served directly by GoFr.

<details>
<summary><strong>🔐 Login</strong></summary>
<br>
<img src="https://github.com/user-attachments/assets/30dc1476-a0be-4fe8-99c9-cf24320fe333" alt="Login" width="100%">
</details>

<details>
<summary><strong>🔑 Virtual Keys</strong></summary>
<br>
<img src="https://github.com/user-attachments/assets/2eb3082e-183a-4289-987a-8ebe75730e76" alt="Virtual Keys" width="100%">
</details>

<details>
<summary><strong>💰 Spend & Usage</strong></summary>
<br>
<img src="https://github.com/user-attachments/assets/d8f7810d-faaa-436e-a6a3-cb9d6adde408" alt="Spend & Usage" width="100%">
</details>

<details>
<summary><strong>🛝 Playground</strong></summary>
<br>
<img src="https://github.com/user-attachments/assets/598f10fa-d730-4e2a-bbe5-8a3841cd087f" alt="Playground" width="100%">
</details>

<details>
<summary><strong>📖 API Docs</strong></summary>
<br>
<img src="https://github.com/user-attachments/assets/d989fc87-6583-42de-a9e7-7c32358e6813" alt="API Docs" width="100%">
</details>

<details>
<summary><strong>⚙️ Settings</strong></summary>
<br>
<img src="https://github.com/user-attachments/assets/990bb9a3-bed4-439f-8f26-b9f416ab7b62" alt="Settings" width="100%">
</details>

### Observability (GoFr Built-in)

All of this comes free with GoFr — no extra configuration:

- Structured JSON logging
- OpenTelemetry distributed tracing
- Prometheus metrics (HTTP latency, error rates, connection pools + custom LLM cost/token metrics)
- Pre-configured Grafana dashboard with 54 panels

### WebSocket Streaming

Real-time token delivery via native GoFr WebSocket:

```javascript
const ws = new WebSocket("ws://localhost:9000/v1/chat/completions/stream");
ws.send(JSON.stringify({
  model: "openai/gpt-4o",
  messages: [{ role: "user", content: "Hello!" }]
}));
ws.onmessage = (event) => console.log(JSON.parse(event.data));
```

---

## Quick Start

### Use with any OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:9000/v1",
    api_key="sk-gateway-key-1"
)

response = client.chat.completions.create(
    model="anthropic/claude-sonnet-4-20250514",  # provider/model format
    messages=[{"role": "user", "content": "Hello!"}]
)
```

### Option 1: Docker Compose (recommended)

```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...

cd docker && docker-compose up -d
```

| Service | URL |
|---------|-----|
| Gateway API | http://localhost:9000 |
| Admin Dashboard | http://localhost:9000/admin |
| Metrics | http://localhost:2121/metrics |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 (admin/admin) |

### Option 2: Local Development

```bash
# Prerequisites: Go 1.25+, Redis, PostgreSQL (optional)

# Start Redis
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Start PostgreSQL (optional — needed for virtual keys, budgets, guardrails, batches)
docker run -d --name postgres -p 5432:5432 \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=llmgw postgres:16-alpine

# Configure
cp configs/.env configs/.env.local  # edit with your API keys

# Run
go run .
```

### Option 3: Static Binary

```bash
CGO_ENABLED=0 go build -o llm-gateway .
./llm-gateway
```

---

## API Reference

### Core Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/chat/completions` | Chat completions (OpenAI format) |
| POST | `/v1/completions` | Legacy text completions |
| POST | `/v1/embeddings` | Text embeddings |
| POST | `/v1/moderations` | Content moderation |
| POST | `/v1/images/generations` | Image generation |
| POST | `/v1/images/edits` | Image editing |
| POST | `/v1/images/variations` | Image variations |
| POST | `/v1/audio/speech` | Text-to-speech |
| POST | `/v1/audio/transcriptions` | Speech-to-text |
| POST | `/v1/rerank` | Document reranking |
| GET | `/v1/models` | List available models |
| GET | `/health` | Health check |
| WS | `/v1/chat/completions/stream` | WebSocket streaming |

### Batch Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/batches` | Submit a batch of requests |
| GET | `/v1/batches` | List batches (supports `limit`, `offset`) |
| GET | `/v1/batches/{id}` | Get batch status and progress |
| GET | `/v1/batches/{id}/results` | Get completed batch results |
| POST | `/v1/batches/{id}/cancel` | Cancel a pending/processing batch |

### Admin Endpoints (require master key)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/keys` | List all virtual keys |
| POST | `/key/generate` | Generate virtual API key |
| GET | `/key/info?key=sk-...` | Get key metadata |
| DELETE | `/key/{id}` | Revoke key |
| POST | `/key/{id}/rotate` | Rotate key (deactivate old, generate new) |
| GET | `/spend/report` | Spend reporting with `group_by` |
| POST | `/teams` | Create team |
| GET | `/teams` | List teams |
| DELETE | `/teams/{id}` | Delete team |
| POST | `/users` | Create user |
| GET | `/users` | List users |
| DELETE | `/users/{id}` | Delete user |
| POST | `/organizations` | Create organization |
| GET | `/organizations` | List organizations |
| DELETE | `/organizations/{id}` | Delete organization |
| GET | `/audit/log` | View audit trail |

### Examples

<details>
<summary><strong>Chat completion</strong></summary>

```bash
curl -X POST http://localhost:9000/v1/chat/completions \
  -H "Authorization: Bearer sk-gateway-key-1" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

</details>

<details>
<summary><strong>Function calling (works across all 6 providers)</strong></summary>

```bash
curl -X POST http://localhost:9000/v1/chat/completions \
  -H "Authorization: Bearer sk-gateway-key-1" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "anthropic/claude-sonnet-4-20250514",
    "messages": [{"role": "user", "content": "What is the weather in Paris?"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get current weather",
        "parameters": {
          "type": "object",
          "properties": {"location": {"type": "string"}},
          "required": ["location"]
        }
      }
    }]
  }'
```

The gateway translates OpenAI tool format to each provider's native format and back.

</details>

<details>
<summary><strong>Generate a virtual key</strong></summary>

```bash
curl -X POST http://localhost:9000/key/generate \
  -H "Authorization: Bearer $GATEWAY_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-key",
    "allowed_models": ["openai/gpt-4o", "anthropic/claude-sonnet-4-20250514"],
    "rate_limit_rpm": 60,
    "rate_limit_tpm": 100000,
    "max_budget": 50.00,
    "team_id": "engineering",
    "expires_in_days": 90
  }'
# Response: {"key":"sk-abc123...","key_prefix":"sk-abc123...","name":"production-key"}
```

</details>

<details>
<summary><strong>Spend reporting</strong></summary>

```bash
# By provider
curl "http://localhost:9000/spend/report?group_by=provider"

# By model for a date range
curl "http://localhost:9000/spend/report?group_by=model&start_date=2026-01-01&end_date=2026-02-28"

# By team
curl "http://localhost:9000/spend/report?group_by=team"
```

</details>

<details>
<summary><strong>Fallback chains</strong></summary>

```bash
# Global fallback chain in .env
FALLBACK_CHAIN=openai,anthropic,ollama

# Per-key fallback overrides
GATEWAY_KEY_CONFIG=sk-key-1:openai,anthropic;sk-key-2:anthropic,ollama
```

When a provider fails with a retryable error, the gateway automatically tries the next provider in the chain.

</details>

---

## Configuration

All configuration is via environment variables or a `.env` file in `configs/`.

### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `9000` | API port |
| `DEFAULT_PROVIDER` | `openai` | Default provider when model has no prefix |
| `GATEWAY_API_KEYS` | — | Comma-separated static API keys |
| `GATEWAY_MASTER_KEY` | — | Master key for admin endpoints and dashboard |

### Provider Keys

| Variable | Description |
|----------|-------------|
| `OPENAI_API_KEY` | OpenAI API key |
| `OPENAI_BASE_URL` | Custom OpenAI-compatible endpoint (default: `https://api.openai.com`) |
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `GROQ_API_KEY` | Groq API key |
| `DEEPSEEK_API_KEY` | DeepSeek API key |
| `GEMINI_API_KEY` | Google Gemini API key |
| `OLLAMA_BASE_URL` | Ollama URL (default: `http://localhost:11434`) |

### Routing & Reliability

| Variable | Default | Description |
|----------|---------|-------------|
| `ROUTING_STRATEGY` | `simple` | `simple`, `round-robin`, `weighted`, `least-busy`, `latency`, `usage` |
| `RETRY_MAX` | `3` | Max retry attempts |
| `RETRY_BACKOFF_BASE_MS` | `500` | Base backoff delay in ms |
| `COOLDOWN_THRESHOLD` | `5` | Failures before provider cooldown |
| `COOLDOWN_PERIOD_SECONDS` | `60` | Cooldown duration |
| `CB_THRESHOLD` | `5` | Circuit breaker failure threshold |
| `CB_INTERVAL_SECONDS` | `30` | Circuit breaker check interval |
| `FALLBACK_CHAIN` | — | Comma-separated provider names |
| `LATENCY_EMA_ALPHA` | `0.2` | EMA weight for latency-based routing |
| `USAGE_RESET_PERIOD_SECONDS` | `60` | Token usage window for usage-based routing |

### Per-Provider Timeouts

All in milliseconds. `0` = no timeout.

| Variable | Description |
|----------|-------------|
| `OPENAI_TIMEOUT_MS` | OpenAI request timeout |
| `ANTHROPIC_TIMEOUT_MS` | Anthropic request timeout |
| `GROQ_TIMEOUT_MS` | Groq request timeout |
| `DEEPSEEK_TIMEOUT_MS` | DeepSeek request timeout |
| `GEMINI_TIMEOUT_MS` | Gemini request timeout |
| `OLLAMA_TIMEOUT_MS` | Ollama request timeout |

### Guardrails

| Variable | Default | Description |
|----------|---------|-------------|
| `GUARDRAIL_ENABLED` | `false` | Enable guardrails globally |
| `GUARDRAIL_BLOCKED_KEYWORDS` | — | Comma-separated blocked keywords |
| `GUARDRAIL_PII_ACTION` | `none` | PII handling: `none`, `block`, `redact`, `log` |
| `GUARDRAIL_MAX_INPUT_TOKENS` | `0` | Max input tokens (`0` = unlimited) |
| `GUARDRAIL_MAX_OUTPUT_TOKENS` | `0` | Max output tokens (`0` = unlimited) |

Per-key overrides are stored in the `guardrail_configs` table and take precedence over global defaults.

### Batch Processing

| Variable | Default | Description |
|----------|---------|-------------|
| `BATCH_WORKERS` | `5` | Number of concurrent batch worker goroutines |
| `BATCH_TASK_TIMEOUT_SECONDS` | `120` | Timeout per batch item |

### Database & Cache

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `CACHE_TTL_SECONDS` | `300` | Response cache TTL |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | — | Database password |
| `DB_NAME` | `llmgw` | Database name |
| `DB_DIALECT` | `postgres` | Database dialect |

### Cost & Budgets

| Variable | Description |
|----------|-------------|
| `CUSTOM_PRICING` | Override pricing: `model:input_per_1k:output_per_1k,...` |

---

## Grafana Dashboard

The gateway ships with a pre-configured Grafana dashboard (54 panels) auto-provisioned on `docker-compose up`.

**GoFr built-in panels:** App info, goroutines, memory, inbound/outbound HTTP metrics, circuit breaker status, SQL and Redis query metrics.

**LLM Gateway panels:** Total cost/requests/tokens, average cost per request, request rate by provider, cost rate by provider, token rate by provider, cost distribution histogram, requests by model, cost by model.

Access at http://localhost:3000 after starting Docker Compose (login: admin/admin).

---

## Feature Comparison: LLM Gateway vs LiteLLM

| Feature | LLM Gateway | LiteLLM |
|---------|:-----------:|:-------:|
| **Language** | Go (single binary) | Python |
| **Memory footprint** | ~15 MB | ~200 MB+ |
| **Startup time** | < 100 ms | ~3-5 s |
| **Providers** | 6 | 100+ |
| **Chat completions** | Yes | Yes |
| **Embeddings** | Yes | Yes |
| **Images / Audio / Rerank** | Yes | Yes |
| **Function calling** | Full translation (all providers) | Partial |
| **Streaming** | WebSocket | SSE |
| **Routing strategies** | 6 | 5 |
| **Fallback chains** | Yes (error-aware + context window) | Yes |
| **Retries + backoff** | Yes (exponential + jitter) | Yes |
| **Circuit breakers** | Yes (GoFr native) | No |
| **Connection pooling** | Yes (GoFr native) | No |
| **Guardrails** | Yes (keywords, PII, token limits) | Yes (7+ providers) |
| **Virtual keys** | Yes | Yes |
| **Per-key rate limits** | Yes (RPM + TPM via Redis) | Yes |
| **Per-key model restrictions** | Yes | Yes |
| **Multi-tenancy** | Yes (teams, users, orgs) | Yes |
| **Cost tracking** | Yes (per-request + budgets) | Yes |
| **Batch API** | Yes | Yes |
| **Admin UI** | Yes (built-in SPA) | Yes |
| **Prometheus metrics** | Yes (GoFr native + custom) | Yes |
| **OpenTelemetry tracing** | Yes (GoFr native) | Yes |
| **Grafana dashboard** | Yes (54 panels, auto-provisioned) | Community dashboards |
| **Config hot-reload** | Yes (file watcher + SIGHUP) | Partial |
| **YAML config** | Yes | Yes |
| **Azure / Bedrock** | Not yet | Yes |

---

## Project Structure

```
llm-gateway/
├── main.go                        # Entry point — all wiring
├── gateway.go                     # Thread-safe config swap for hot-reload
│
├── handler/                       # HTTP handlers
│   ├── chat.go                    # Chat completions + guardrails + cost tracking
│   ├── stream.go                  # WebSocket streaming + guardrails
│   ├── batch.go                   # Batch submit / status / results / cancel / list
│   ├── completions.go             # Legacy completions
│   ├── embeddings.go              # Embeddings
│   ├── moderations.go             # Content moderation
│   ├── images.go                  # Image generation / edit / variations
│   ├── audio.go                   # TTS + STT
│   ├── rerank.go                  # Document reranking
│   ├── keys.go                    # Virtual key CRUD + list
│   ├── teams.go                   # Team CRUD
│   ├── users.go                   # User CRUD
│   ├── orgs.go                    # Org CRUD
│   ├── audit.go                   # Audit log
│   ├── spend.go                   # Spend reporting
│   └── validation.go              # Tool schema validation
│
├── provider/                      # LLM provider integrations
│   ├── provider.go                # Interfaces + Registry
│   ├── openai_compat.go           # Shared OpenAI-compatible base
│   ├── openai.go, anthropic.go    # Full providers
│   ├── gemini.go, ollama.go       # Full providers (format translation)
│   ├── groq.go, deepseek.go       # OpenAI-compatible thin wrappers
│   ├── fallback.go                # Error-aware fallback chain
│   ├── capabilities.go            # Model capability detection
│   └── tool_injection.go          # Tool prompt injection for non-tool models
│
├── routing/                       # Request routing engine
│   ├── router.go                  # Central router (retry + cooldown + strategy)
│   ├── strategy.go                # Simple, RoundRobin, Weighted
│   ├── strategy_leastbusy.go      # Least-busy strategy
│   ├── strategy_latency.go        # Latency-based strategy
│   ├── strategy_usage.go          # Usage-based strategy
│   ├── deployment.go              # Deployment struct + tag filtering
│   ├── retry.go                   # Exponential backoff + jitter
│   ├── cooldown.go                # Provider cooldown tracker
│   ├── errors.go                  # Error classification (6 types)
│   ├── inflight.go                # In-flight request tracker
│   ├── latency.go                 # EMA latency tracker
│   ├── usage.go                   # Windowed token usage tracker
│   └── queue.go                   # Request queue
│
├── guardrails/                    # Content filtering engine
│   ├── guardrails.go              # Check (pre-call) + Filter (post-call) + config loader
│   ├── keywords.go                # Keyword blocklist
│   └── pii.go                     # PII detection + redaction
│
├── workerpool/                    # Generic worker pool
│   └── pool.go                    # Configurable workers, queue, timeout, panic recovery
│
├── batch/                         # Batch processing
│   └── processor.go               # Async item processing via worker pool
│
├── models/                        # OpenAI-compatible request/response types
│   ├── models.go                  # Chat, streaming, tool types
│   ├── batch.go                   # Batch types
│   ├── embeddings.go, completions.go, moderations.go
│   ├── images.go, audio.go, rerank.go
│   └── ...
│
├── middleware/                    # HTTP middleware
│   ├── apikey.go                  # Bearer token auth + per-key config
│   ├── keystore.go                # In-memory virtual key cache
│   └── ratelimit.go               # Redis sliding window (RPM + TPM)
│
├── cache/                         # Caching
│   ├── cache.go                   # Redis response cache
│   └── tool_cache.go              # Redis tool result cache
│
├── cost/                          # Cost tracking
│   ├── cost.go                    # Pricing table + calculator
│   └── metrics.go                 # Prometheus metrics via GoFr
│
├── budget/                        # Budget enforcement
│   ├── budget.go                  # Budget checking + spend recording
│   └── alerts.go                  # Threshold-based alerts
│
├── config/                        # Configuration
│   ├── config.go                  # YAML config loader
│   ├── builder.go                 # Build registry/router from YAML
│   ├── validate.go                # Config validation
│   └── watcher.go                 # Hot-reload (file watch + SIGHUP)
│
├── audit/audit.go                 # Audit log helper
├── migrations/migrations.go       # PostgreSQL migrations (11 tables)
│
├── admin/static/                  # Admin dashboard SPA
│   ├── index.html                 # Dashboard shell
│   ├── app.js                     # Client-side logic
│   └── style.css                  # Dark-themed styling
│
├── configs/
│   ├── .env                       # Environment configuration template
│   └── config.yaml                # YAML configuration example
│
└── docker/
    ├── Dockerfile                 # Multi-stage build
    ├── docker-compose.yaml        # Full stack
    ├── prometheus/                # Prometheus config
    └── grafana/                   # Dashboard + provisioning
```

---

## Testing

```bash
# Run all tests
go test ./...

# With verbose output
go test ./... -v

# With coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

| Package | Tests | Coverage Area |
|---------|-------|---------------|
| `routing` | 63 | All 6 strategies, retry, cooldown, trackers, deployment filtering |
| `cost` | 37 | Pricing, calculation, custom pricing parsing |
| `provider` | 30 | Capability detection, param stripping |
| `middleware` | 26 | API key auth, keystore, parsing |
| `guardrails` | 20 | PII detection/redaction, keywords, Check/Filter, config parsing |
| `models` | 20 | JSON marshal/unmarshal, round-trip, omitempty |
| `handler` | 14 | Tool validation |
| `workerpool` | 10 | Submit, queue full, timeout, panic recovery, shutdown |
| `batch` | 1 | Processor creation |

---

## Development

```bash
# Build
go build ./...

# Vet
go vet ./...

# Run locally
go run .

# Docker build
docker build -t llm-gateway -f docker/Dockerfile .
```

---

## License

Part of the [GoFr](https://gofr.dev) examples collection.
