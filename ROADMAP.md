# LLM Gateway Roadmap — What's Left

Go-native LiteLLM alternative built with [GoFr](https://gofr.dev). Items marked 🏗 have GoFr built-in support that significantly reduces implementation effort.

---

## Completed

| Phase | What's Done |
|---|---|
| **1. Core MVP** | OpenAI-compatible API, 6 providers (OpenAI, Anthropic, Gemini, Groq, DeepSeek, Ollama), Redis caching, API key auth, Docker Compose, full observability |
| **2. Streaming** | WebSocket streaming for all 6 providers, `StreamChunk` format, usage reporting |
| **3. Providers** | Gemini, Groq, DeepSeek added; `OpenAICompatible` base for thin constructors |
| **4. Routing** | Fallback chains (error-aware + context window + content policy), weighted round-robin, retries with exponential backoff + jitter, cooldown, circuit breakers |
| **6. Function Calling** | Tool use across all 6 providers (passthrough for OpenAI/Groq/DeepSeek, full translation for Anthropic/Gemini/Ollama), streaming tool_calls |
| **8. Cost** | Model pricing table + calculator (`cost/cost.go`) |
| **11. Observability** | OpenTelemetry export (OTLP/Jaeger/Zipkin), DataDog (via OTLP), dynamic log levels (GoFr remote), structured JSON logs |

---

## Phase 3: More Providers

### Tier 1 — Major cloud providers
- [ ] **Azure OpenAI** (`azure/gpt-4o`) — OpenAI-compatible + API version header
- [ ] **AWS Bedrock** (`bedrock/anthropic.claude-3-sonnet`) — Converse API, SigV4 auth

### Tier 2 — Fast inference & open models
- [ ] **Together AI** (`together/meta-llama/Llama-3-70b`) — OpenAI-compatible
- [ ] **Fireworks AI** (`fireworks/llama-v3-70b`) — OpenAI-compatible

### Tier 3 — Specialized providers
- [ ] **Cohere** (`cohere/command-r-plus`) — Command models, embeddings, rerank
- [ ] **Mistral AI** (`mistral/mistral-large-latest`) — Custom API format
- [ ] **Perplexity** (`perplexity/sonar-pro`) — Search-augmented models
- [ ] **xAI** (`xai/grok-2`) — OpenAI-compatible

### Infrastructure
- [ ] **Dynamic model discovery** — Query provider APIs for available models at startup
- [ ] **Custom/generic provider** — User-defined providers via config for any OpenAI-compatible endpoint

---

## Phase 4: Routing & Reliability (remaining)

### Load balancing
- [ ] **Multi-deployment support** — Multiple instances of the same model (`model_list` config)
- [ ] **Least-busy routing** — Route to deployment with fewest in-flight requests
- [ ] **Latency-based routing** — Route to fastest responding deployment
- [ ] **Usage-based routing** — Route to deployment with lowest utilization

### Fallbacks
- [ ] **Per-key fallback overrides** — Different fallback chains per API key

### Request management
- [ ] **Per-provider timeouts** — Configurable request timeouts
- [ ] **Request queuing** — Buffer during rate limit windows 🏗 GoFr Pub/Sub
- [ ] **Priority queues** — Higher-priority keys get served first 🏗 GoFr Pub/Sub

---

## Phase 5: Additional API Endpoints

- [ ] `POST /v1/embeddings` — Text embeddings (OpenAI, Cohere, Bedrock, Vertex, Ollama)
- [ ] Input type support (`search_document`, `search_query`, `classification`)
- [ ] Multi-provider embedding routing
- [ ] `POST /v1/completions` — Legacy text completions
- [ ] `POST /v1/moderations` — Content moderation
- [ ] `POST /v1/images/generations` — Image generation (DALL-E, Stability AI)
- [ ] `POST /v1/images/edits` — Image editing
- [ ] `POST /v1/images/variations` — Image variations
- [ ] `POST /v1/audio/speech` — Text-to-speech (OpenAI, ElevenLabs)
- [ ] `POST /v1/audio/transcriptions` — Speech-to-text (Whisper, Groq Whisper, Deepgram)
- [ ] `POST /v1/rerank` — Rerank search results (Cohere-compatible)

---

## Phase 6: Function Calling (remaining)

- [ ] **Drop unsupported params** — `drop_params` config to silently drop `tools` for non-supporting models
- [ ] **Tool schema validation** — Validate tool JSON schemas before forwarding
- [ ] **Tool result caching** — Cache deterministic tool call results
- [ ] **Prompt injection for non-tool models** — Append tool definitions to system prompt

---

## Phase 7: YAML Configuration

```yaml
model_list:
  - model_name: gpt-4o
    litellm_params:
      model: openai/gpt-4o
      api_key: os.environ/OPENAI_API_KEY
      tpm: 100000
      rpm: 1000
```

- [ ] `config.yaml` parser with `model_list`, `router_settings`, `general_settings`
- [ ] `os.environ/` syntax for secrets in config files
- [ ] Multi-deployment models (same `model_name`, different backends)
- [ ] Model aliases (`model_group_alias`)
- [ ] Config reload without restart (watch file / SIGHUP)
- [ ] Config validation on startup

---

## Phase 8: Cost Tracking & Budgets (remaining)

### Cost calculation
- [ ] **Real-time cost tracking** — Calculate cost for every request
- [ ] **Custom pricing overrides** — Per-deployment pricing in config

### Budgets
- [ ] **Per-key budgets** — Max spend per API key with reset periods
- [ ] **Per-user budgets** — Aggregate across user's keys
- [ ] **Per-team budgets** — Shared team spending limits
- [ ] **Per-organization budgets** — Top-level org limits
- [ ] **Budget duration** — Reset on interval (`1h`, `1d`, `7d`, `30d`)

### Spend tracking
- [ ] **PostgreSQL spend log** 🏗 GoFr SQL + migrations
- [ ] **Prometheus cost metrics** — `llm_gateway_cost_total` by provider, model, key, team 🏗 GoFr Prometheus
- [ ] **Budget alerts** — Webhook at 50%, 80%, 100% thresholds 🏗 GoFr Pub/Sub
- [ ] **Spend reporting API** — `GET /spend/report` with date range, grouping

---

## Phase 9: Virtual Keys & Multi-Tenancy

### Virtual keys
- [ ] **Key generation** — `POST /key/generate` with model restrictions, rate limits, budgets
- [ ] **Key validation** — DB-backed lookup 🏗 GoFr `EnableAPIKeyAuthWithValidator()` + SQL
- [ ] **Key info** — `GET /key/info` metadata, usage, remaining budget
- [ ] **Key update/delete** — `POST /key/update`, `POST /key/delete`
- [ ] **Key rotation** — Automatic rotation on configurable intervals
- [ ] **Master key** — Admin key for management endpoints

### Access control
- [ ] **Per-key model restrictions** — Limit which models a key can access
- [ ] **Per-key rate limits** — RPM and TPM limits 🏗 GoFr rate limiter middleware
- [ ] **Tag-based routing** — Route to specific deployments based on request tags
- [ ] **Blocked user lists**

### Multi-tenancy
- [ ] **Teams** — `POST /team/new` 🏗 GoFr SQL + migrations
- [ ] **Users** — `POST /user/new` 🏗 GoFr SQL + migrations
- [ ] **Organizations** — Top-level grouping 🏗 GoFr SQL + migrations
- [ ] **Audit logging** 🏗 GoFr structured logs + SQL

---

## Phase 10: Advanced Caching

- [ ] **In-memory L1 cache** — Local cache before Redis (dual-cache)
- [ ] **Semantic caching** — Vector embeddings, match similar prompts
- [ ] **Cache namespacing** — Per-key or per-team isolation
- [ ] **Cache invalidation API** — `DELETE /cache` with key/pattern/all
- [ ] **Cache analytics** — Hit rate, savings 🏗 GoFr Prometheus
- [ ] **Cache warming** — Pre-populate from seed file
- [ ] **S3/GCS cache backend** — Durable cached responses 🏗 GoFr file abstraction (S3, GCS, Azure)
- [ ] **Streaming cache** — Cache and replay SSE streams

---

## Phase 11: Observability & Callbacks (remaining)

### Callback system
- [ ] **Callback interface** — `OnSuccess(req, resp)`, `OnFailure(req, err)`, `OnPreCall(req)`
- [ ] **Async callbacks** 🏗 GoFr Pub/Sub
- [ ] **Multiple callbacks** — Register N callbacks simultaneously

### Integrations
- [ ] **Generic webhook callback** 🏗 GoFr HTTP services for outbound calls
- [ ] **Langfuse** — Prompt tracing and analytics
- [ ] **Custom callback plugins** — User-defined Go plugins

### Logging controls
- [ ] **Redact API keys** — Strip keys from all log output (custom sanitizer)
- [ ] **Per-team logging** — Route team logs to different Langfuse projects

---

## Phase 12: Guardrails

### Pre-call (before sending to LLM)
- [ ] **PII detection & masking** — Detect/redact emails, phone numbers, SSNs
- [ ] **Prompt injection detection** — Block known injection patterns
- [ ] **Banned keywords** — Block requests containing specific terms
- [ ] **Content moderation** — OpenAI moderation API check before forwarding

### Post-call (on LLM response)
- [ ] **Output content moderation** — Check response for policy violations
- [ ] **PII in responses** — Detect/redact PII in model output
- [ ] **Response validation** — JSON schema validation for structured outputs

### Framework
- [ ] **Guardrail interface** — `PreCall(req) error`, `PostCall(req, resp) error`
- [ ] **Per-key guardrail config** — Different guardrails per key
- [ ] **Logging-only mode** — Log violations without blocking
- [ ] **Custom guardrail plugins** — User-defined Go guardrails

---

## Phase 13: Admin UI & Management

- [ ] **Admin dashboard** — Server-rendered UI for key management, spend tracking, model status 🏗 GoFr `response.Template{}` + `app.AddStaticFiles()`
- [ ] **Swagger/OpenAPI** — Provide `static/openapi.json` 🏗 GoFr auto-renders at `/.well-known/swagger`
- [ ] **SSO** — OIDC/SAML login 🏗 GoFr `EnableOAuth()` with JWKS
- [ ] **JWT authentication** 🏗 GoFr `EnableOAuth(jwksEndpoint, refreshInterval)`
- [ ] **RBAC** — Proxy Admin, Org Admin, Team Admin, User 🏗 GoFr RBAC middleware

---

## Phase 14: Advanced Features

- [ ] **Batch API** — `POST /v1/batches` for async bulk processing
- [ ] **Fine-tuning management** — `POST /v1/fine_tuning/jobs`
- [ ] **File management** — `POST /v1/files` 🏗 GoFr file abstraction (S3, GCS, Azure, FTP/SFTP)
- [ ] **WebSocket realtime API** — `WS /v1/realtime` for voice/streaming
- [ ] **Pass-through endpoints** — Proxy arbitrary requests to any backend
- [ ] **MCP integration** — Model Context Protocol server/client
- [ ] **Config-based provider registration** — Add providers via config without code changes
- [ ] **Multi-region deployment** — Control plane / data plane architecture
- [ ] **Secret manager integration** — AWS KMS, Azure Key Vault, HashiCorp Vault, GCP Secret Manager

---

## Summary: Remaining Items by Phase

| Phase | Remaining | GoFr Helps |
|---|---|---|
| 3. Providers | 10 | OpenAI-compatible base makes most trivial |
| 4. Routing | 7 | Pub/Sub for queuing |
| 5. Endpoints | 11 | — |
| 6. Tools | 4 | — |
| 7. YAML Config | 6 | — |
| 8. Cost/Budgets | 9 | SQL + migrations, Prometheus, Pub/Sub |
| 9. Keys/Tenancy | 14 | API key validator, rate limiter, SQL, RBAC |
| 10. Caching | 8 | Prometheus, file abstraction (S3/GCS) |
| 11. Callbacks | 7 | Pub/Sub, HTTP services |
| 12. Guardrails | 11 | — |
| 13. Admin | 5 | Swagger UI, OAuth/JWT, RBAC, HTML templates, static files |
| 14. Advanced | 9 | File abstraction |
| **Total** | **~101** | **~30 items significantly easier via GoFr** |

---

## GoFr Features Leveraged

| GoFr Feature | Phases It Helps | Config |
|---|---|---|
| Remote log level | 11 | `REMOTE_LOG_URL`, `REMOTE_LOG_FETCH_INTERVAL` |
| `EnableOAuth()` | 13 | JWKS endpoint + refresh interval |
| `EnableAPIKeyAuthWithValidator()` | 9 | Custom validator function |
| RBAC middleware | 13 | Config-based role→permission mapping |
| Rate limiter middleware | 9 | `RequestsPerSecond`, `Burst`, `PerIP` |
| SQL + migrations | 8, 9 | `DB_HOST`, `DB_PORT`, `DB_NAME` |
| Pub/Sub | 4, 8, 11 | `PUBSUB_BACKEND` (Kafka, NATS, Google, MQTT, Redis) |
| File abstraction | 10, 14 | `FILE_STORE` (S3, GCS, Azure, local, FTP) |
| Swagger UI rendering | 13 | Place `static/openapi.json` |
| `response.Template{}` | 13 | HTML templates from `./templates/` via `html/template` |
| `AddStaticFiles()` | 13 | Serve CSS/JS/images from `./static/` |
| OTLP tracing | 11 | `TRACE_EXPORTER`, `TRACER_URL` |
| Prometheus metrics | 8, 10 | Built-in at `:2121/metrics` |
| Health checks | auto | `/.well-known/health` |

---

## How to Contribute a Provider

1. Create `provider/yourprovider.go`
2. Implement the `Provider` interface:
   ```go
   type Provider interface {
       Name() string
       ChatCompletion(ctx *gofr.Context, req models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)
       Models() []string
   }
   ```
3. Add request/response translation (provider format ↔ OpenAI format)
4. Register the HTTP service in `main.go`: `app.AddHTTPService("name", "https://api.provider.com")`
5. Register the provider: `reg.Register(provider.NewYourProvider(apiKey))`
6. Update this roadmap

See `provider/anthropic.go` as the reference for a non-OpenAI provider.
