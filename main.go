package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service"

	"aryanmehrotra/litellm-go/batch"
	"aryanmehrotra/litellm-go/cache"
	"aryanmehrotra/litellm-go/cost"
	"aryanmehrotra/litellm-go/guardrails"
	"aryanmehrotra/litellm-go/handler"
	"aryanmehrotra/litellm-go/middleware"
	"aryanmehrotra/litellm-go/migrations"
	"aryanmehrotra/litellm-go/provider"
	"aryanmehrotra/litellm-go/routing"
	"aryanmehrotra/litellm-go/websearch"
	"aryanmehrotra/litellm-go/workerpool"
)

func main() {
	app := gofr.New()

	// Read configuration
	openaiKey := app.Config.GetOrDefault("OPENAI_API_KEY", "")
	anthropicKey := app.Config.GetOrDefault("ANTHROPIC_API_KEY", "")
	groqKey := app.Config.GetOrDefault("GROQ_API_KEY", "")
	deepseekKey := app.Config.GetOrDefault("DEEPSEEK_API_KEY", "")
	geminiKey := app.Config.GetOrDefault("GEMINI_API_KEY", "")
	togetherKey := app.Config.GetOrDefault("TOGETHER_API_KEY", "")
	fireworksKey := app.Config.GetOrDefault("FIREWORKS_API_KEY", "")
	perplexityKey := app.Config.GetOrDefault("PERPLEXITY_API_KEY", "")
	xaiKey := app.Config.GetOrDefault("XAI_API_KEY", "")
	mistralKey := app.Config.GetOrDefault("MISTRAL_API_KEY", "")
	cohereKey := app.Config.GetOrDefault("COHERE_API_KEY", "")
	azureKey := app.Config.GetOrDefault("AZURE_OPENAI_API_KEY", "")
	azureEndpoint := app.Config.GetOrDefault("AZURE_OPENAI_ENDPOINT", "")
	azureAPIVersion := app.Config.GetOrDefault("AZURE_OPENAI_API_VERSION", "2024-02-01")
	azureDeployments := app.Config.GetOrDefault("AZURE_OPENAI_DEPLOYMENTS", "")
	awsAccessKey := app.Config.GetOrDefault("AWS_ACCESS_KEY_ID", "")
	awsSecretKey := app.Config.GetOrDefault("AWS_SECRET_ACCESS_KEY", "")
	awsRegion := app.Config.GetOrDefault("AWS_REGION", "us-east-1")
	cerebrasKey := app.Config.GetOrDefault("CEREBRAS_API_KEY", "")
	sambanovaKey := app.Config.GetOrDefault("SAMBANOVA_API_KEY", "")
	ai21Key := app.Config.GetOrDefault("AI21_API_KEY", "")
	openrouterKey := app.Config.GetOrDefault("OPENROUTER_API_KEY", "")
	novitaKey := app.Config.GetOrDefault("NOVITA_API_KEY", "")
	nvidiaKey := app.Config.GetOrDefault("NVIDIA_API_KEY", "")
	cloudflareToken := app.Config.GetOrDefault("CLOUDFLARE_API_TOKEN", "")
	cloudflareAccountID := app.Config.GetOrDefault("CLOUDFLARE_ACCOUNT_ID", "")
	vertexProject := app.Config.GetOrDefault("VERTEX_PROJECT", "")
	vertexLocation := app.Config.GetOrDefault("VERTEX_LOCATION", "us-central1")
	vertexAccessToken := app.Config.GetOrDefault("VERTEX_ACCESS_TOKEN", "")
	hfKey := app.Config.GetOrDefault("HUGGINGFACE_API_KEY", "")
	openaiBaseURL := app.Config.GetOrDefault("OPENAI_BASE_URL", "https://api.openai.com")
	ollamaBaseURL := app.Config.GetOrDefault("OLLAMA_BASE_URL", "http://localhost:11434")
	defaultProvider := app.Config.GetOrDefault("DEFAULT_PROVIDER", "openai")
	gatewayKeys := app.Config.GetOrDefault("GATEWAY_API_KEYS", "")
	cacheTTL, _ := strconv.Atoi(app.Config.GetOrDefault("CACHE_TTL_SECONDS", "300"))
	fallbackChain := app.Config.GetOrDefault("FALLBACK_CHAIN", "")
	masterKey := app.Config.GetOrDefault("GATEWAY_MASTER_KEY", "")

	// Routing & reliability configuration
	retryMax, _ := strconv.Atoi(app.Config.GetOrDefault("RETRY_MAX", "3"))
	retryBackoffBaseMs, _ := strconv.Atoi(app.Config.GetOrDefault("RETRY_BACKOFF_BASE_MS", "500"))
	cooldownThreshold, _ := strconv.Atoi(app.Config.GetOrDefault("COOLDOWN_THRESHOLD", "5"))
	cooldownPeriodSec, _ := strconv.Atoi(app.Config.GetOrDefault("COOLDOWN_PERIOD_SECONDS", "60"))
	routingStrategy := app.Config.GetOrDefault("ROUTING_STRATEGY", "simple")
	cbThreshold, _ := strconv.Atoi(app.Config.GetOrDefault("CB_THRESHOLD", "5"))
	cbIntervalSec, _ := strconv.Atoi(app.Config.GetOrDefault("CB_INTERVAL_SECONDS", "30"))

	// Advanced routing configuration
	latencyAlpha, _ := strconv.ParseFloat(app.Config.GetOrDefault("LATENCY_EMA_ALPHA", "0.2"), 64)
	usageResetSec, _ := strconv.Atoi(app.Config.GetOrDefault("USAGE_RESET_PERIOD_SECONDS", "60"))
	keyConfig := app.Config.GetOrDefault("GATEWAY_KEY_CONFIG", "")
	queueEnabled := app.Config.GetOrDefault("REQUEST_QUEUE_ENABLED", "false") == "true"
	queueSize, _ := strconv.Atoi(app.Config.GetOrDefault("REQUEST_QUEUE_SIZE", "100"))

	// Per-provider timeout config (milliseconds)
	openaiTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("OPENAI_TIMEOUT_MS", "0"))
	anthropicTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("ANTHROPIC_TIMEOUT_MS", "0"))
	groqTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("GROQ_TIMEOUT_MS", "0"))
	deepseekTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("DEEPSEEK_TIMEOUT_MS", "0"))
	geminiTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("GEMINI_TIMEOUT_MS", "0"))
	ollamaTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("OLLAMA_TIMEOUT_MS", "0"))
	togetherTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("TOGETHER_TIMEOUT_MS", "0"))
	fireworksTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("FIREWORKS_TIMEOUT_MS", "0"))
	perplexityTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("PERPLEXITY_TIMEOUT_MS", "0"))
	xaiTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("XAI_TIMEOUT_MS", "0"))
	mistralTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("MISTRAL_TIMEOUT_MS", "0"))
	cohereTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("COHERE_TIMEOUT_MS", "0"))
	azureTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("AZURE_TIMEOUT_MS", "0"))
	bedrockTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("BEDROCK_TIMEOUT_MS", "0"))
	cerebrasTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("CEREBRAS_TIMEOUT_MS", "0"))
	sambanovaTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("SAMBANOVA_TIMEOUT_MS", "0"))
	ai21Timeout, _ := strconv.Atoi(app.Config.GetOrDefault("AI21_TIMEOUT_MS", "0"))
	openrouterTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("OPENROUTER_TIMEOUT_MS", "0"))
	novitaTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("NOVITA_TIMEOUT_MS", "0"))
	nvidiaTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("NVIDIA_TIMEOUT_MS", "0"))
	cloudflareTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("CLOUDFLARE_TIMEOUT_MS", "0"))
	vertexTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("VERTEX_TIMEOUT_MS", "0"))
	hfTimeout, _ := strconv.Atoi(app.Config.GetOrDefault("HUGGINGFACE_TIMEOUT_MS", "0"))

	// Batch processing configuration
	batchWorkers, _ := strconv.Atoi(app.Config.GetOrDefault("BATCH_WORKERS", "5"))
	batchTaskTimeoutSec, _ := strconv.Atoi(app.Config.GetOrDefault("BATCH_TASK_TIMEOUT_SECONDS", "120"))

	// Guardrails configuration
	globalGuardrails := guardrails.ParseGlobalConfig(app.Config.GetOrDefault)

	// Web search configuration
	webSearchCfg := websearch.ParseConfig(app.Config.GetOrDefault)

	// GoFr service options for connection pooling, circuit breakers, and health checks.
	poolCfg := &service.ConnectionPoolConfig{MaxIdleConnsPerHost: 20}
	cbCfg := &service.CircuitBreakerConfig{
		Threshold: cbThreshold,
		Interval:  time.Duration(cbIntervalSec) * time.Second,
	}

	// Register HTTP services for each LLM provider with reliability options.
	app.AddHTTPService("openai", openaiBaseURL, poolCfg, cbCfg)
	app.AddHTTPService("anthropic", "https://api.anthropic.com", poolCfg, cbCfg)
	app.AddHTTPService("ollama", ollamaBaseURL, poolCfg, cbCfg)
	app.AddHTTPService("groq", "https://api.groq.com/openai", poolCfg, cbCfg)
	app.AddHTTPService("deepseek", "https://api.deepseek.com", poolCfg, cbCfg)
	app.AddHTTPService("gemini", "https://generativelanguage.googleapis.com", poolCfg, cbCfg)
	app.AddHTTPService("togetherai", "https://api.together.xyz", poolCfg, cbCfg)
	app.AddHTTPService("fireworks", "https://api.fireworks.ai/inference", poolCfg, cbCfg)
	app.AddHTTPService("perplexity", "https://api.perplexity.ai", poolCfg, cbCfg)
	app.AddHTTPService("xai", "https://api.x.ai", poolCfg, cbCfg)
	app.AddHTTPService("mistral", "https://api.mistral.ai", poolCfg, cbCfg)
	app.AddHTTPService("cohere", "https://api.cohere.com", poolCfg, cbCfg)
	app.AddHTTPService("cerebras", "https://api.cerebras.ai", poolCfg, cbCfg)
	app.AddHTTPService("sambanova", "https://api.sambanova.ai", poolCfg, cbCfg)
	app.AddHTTPService("ai21", "https://api.ai21.com/studio", poolCfg, cbCfg)
	app.AddHTTPService("openrouter", "https://openrouter.ai", poolCfg, cbCfg)
	app.AddHTTPService("novita", "https://api.novita.ai", poolCfg, cbCfg)
	app.AddHTTPService("nvidia", "https://integrate.api.nvidia.com", poolCfg, cbCfg)
	app.AddHTTPService("cloudflare", "https://api.cloudflare.com", poolCfg, cbCfg)
	app.AddHTTPService("huggingface", "https://api-inference.huggingface.co", poolCfg, cbCfg)

	if azureEndpoint != "" {
		app.AddHTTPService("azure", azureEndpoint, poolCfg, cbCfg)
	}

	if awsRegion != "" {
		bedrockURL := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", awsRegion)
		app.AddHTTPService("bedrock", bedrockURL, poolCfg, cbCfg)
	}

	if vertexProject != "" {
		vertexURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com", vertexLocation)
		app.AddHTTPService("vertex", vertexURL, poolCfg, cbCfg)
	}

	// Register web search HTTP service if enabled
	if webSearchCfg.Enabled {
		searchBaseURL := webSearchCfg.BaseURL
		switch webSearchCfg.Provider {
		case "tavily":
			searchBaseURL = "https://api.tavily.com"
		case "duckduckgo":
			searchBaseURL = "https://api.duckduckgo.com"
		}

		app.AddHTTPService("websearch", searchBaseURL, poolCfg, cbCfg)
	}

	// Build provider registry — only register providers that have valid credentials.
	// Providers with empty or placeholder keys are skipped entirely.
	reg := provider.NewRegistry(defaultProvider)

	if isValidKey(openaiKey) {
		reg.Register(provider.NewOpenAI(openaiKey, time.Duration(openaiTimeout)*time.Millisecond))
	}

	if isValidKey(anthropicKey) {
		reg.Register(provider.NewAnthropic(anthropicKey, time.Duration(anthropicTimeout)*time.Millisecond))
	}

	// Ollama is always registered (local, no API key needed)
	ollamaProvider := provider.NewOllama(time.Duration(ollamaTimeout) * time.Millisecond)
	reg.Register(ollamaProvider)

	if isValidKey(groqKey) {
		reg.Register(provider.NewGroq(groqKey, time.Duration(groqTimeout)*time.Millisecond))
	}

	if isValidKey(deepseekKey) {
		reg.Register(provider.NewDeepSeek(deepseekKey, time.Duration(deepseekTimeout)*time.Millisecond))
	}

	if isValidKey(geminiKey) {
		reg.Register(provider.NewGemini(geminiKey, time.Duration(geminiTimeout)*time.Millisecond))
	}

	if isValidKey(togetherKey) {
		reg.Register(provider.NewTogetherAI(togetherKey, time.Duration(togetherTimeout)*time.Millisecond))
	}

	if isValidKey(fireworksKey) {
		reg.Register(provider.NewFireworks(fireworksKey, time.Duration(fireworksTimeout)*time.Millisecond))
	}

	if isValidKey(perplexityKey) {
		reg.Register(provider.NewPerplexity(perplexityKey, time.Duration(perplexityTimeout)*time.Millisecond))
	}

	if isValidKey(xaiKey) {
		reg.Register(provider.NewXAI(xaiKey, time.Duration(xaiTimeout)*time.Millisecond))
	}

	if isValidKey(mistralKey) {
		reg.Register(provider.NewMistral(mistralKey, time.Duration(mistralTimeout)*time.Millisecond))
	}

	if isValidKey(cohereKey) {
		reg.Register(provider.NewCohere(cohereKey, time.Duration(cohereTimeout)*time.Millisecond))
	}

	if azureEndpoint != "" && isValidKey(azureKey) {
		reg.Register(provider.NewAzure(azureKey, azureAPIVersion, azureDeployments, time.Duration(azureTimeout)*time.Millisecond))
	}

	if isValidKey(awsAccessKey) && awsSecretKey != "" {
		reg.Register(provider.NewBedrock(awsAccessKey, awsSecretKey, awsRegion, time.Duration(bedrockTimeout)*time.Millisecond))
	}

	if isValidKey(cerebrasKey) {
		reg.Register(provider.NewCerebras(cerebrasKey, time.Duration(cerebrasTimeout)*time.Millisecond))
	}

	if isValidKey(sambanovaKey) {
		reg.Register(provider.NewSambaNova(sambanovaKey, time.Duration(sambanovaTimeout)*time.Millisecond))
	}

	if isValidKey(ai21Key) {
		reg.Register(provider.NewAI21(ai21Key, time.Duration(ai21Timeout)*time.Millisecond))
	}

	if isValidKey(openrouterKey) {
		reg.Register(provider.NewOpenRouter(openrouterKey, time.Duration(openrouterTimeout)*time.Millisecond))
	}

	if isValidKey(novitaKey) {
		reg.Register(provider.NewNovita(novitaKey, time.Duration(novitaTimeout)*time.Millisecond))
	}

	if isValidKey(nvidiaKey) {
		reg.Register(provider.NewNvidianim(nvidiaKey, time.Duration(nvidiaTimeout)*time.Millisecond))
	}

	if cloudflareAccountID != "" && isValidKey(cloudflareToken) {
		reg.Register(provider.NewCloudflare(cloudflareToken, cloudflareAccountID, time.Duration(cloudflareTimeout)*time.Millisecond))
	}

	if vertexProject != "" && isValidKey(vertexAccessToken) {
		reg.Register(provider.NewVertex(vertexProject, vertexLocation, vertexAccessToken, time.Duration(vertexTimeout)*time.Millisecond))
	}

	if isValidKey(hfKey) {
		reg.Register(provider.NewHuggingFace(hfKey, time.Duration(hfTimeout)*time.Millisecond))
	}

	// Build routing components
	retryPolicy := routing.DefaultRetryPolicy(retryMax, time.Duration(retryBackoffBaseMs)*time.Millisecond)
	cooldown := routing.NewCooldownTracker(cooldownThreshold, time.Duration(cooldownPeriodSec)*time.Second)

	// Build strategy with trackers
	inFlight := routing.NewInFlightTracker()
	latencyTracker := routing.NewLatencyTracker(latencyAlpha)
	usageTracker := routing.NewUsageTracker(time.Duration(usageResetSec) * time.Second)

	strategy := buildStrategy(routingStrategy, inFlight, latencyTracker, usageTracker)
	router := routing.NewRouter(retryPolicy, cooldown, strategy)
	router.InFlight = inFlight
	router.Latency = latencyTracker
	router.Usage = usageTracker

	// Request queue
	queue := routing.NewRequestQueue(queueSize, queueEnabled)
	_ = queue // Available for future handler integration

	// Custom pricing overrides
	customPricing := app.Config.GetOrDefault("CUSTOM_PRICING", "")
	cost.ParseCustomPricing(customPricing)

	// Register cost metrics with GoFr
	cost.RegisterMetrics(app)

	// Database migrations (auto-run if DB is configured)
	app.Migrate(migrations.All())

	// Optional fallback chain (e.g. FALLBACK_CHAIN=openai,anthropic,ollama)
	if fallbackChain != "" {
		registerFallbackChain(reg, fallbackChain, cooldown)
	}

	// Redis response cache
	c := cache.New(cacheTTL)

	// Virtual key store (in-memory, loaded from DB on start, write-through on changes)
	keyStore := middleware.NewKeyStore()

	app.OnStart(func(ctx *gofr.Context) error {
		return keyStore.LoadFromDB(ctx)
	})

	// Create default organization if none exist
	app.OnStart(func(ctx *gofr.Context) error {
		var count int
		if err := ctx.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM organizations").Scan(&count); err != nil {
			return nil // DB not configured, skip
		}

		if count == 0 {
			_, err := ctx.SQL.ExecContext(ctx, "INSERT INTO organizations (name) VALUES ('default')")
			if err != nil {
				ctx.Errorf("create default org: %v", err)
			}
		}

		return nil
	})

	// Discover locally available ollama models
	app.OnStart(func(ctx *gofr.Context) error {
		ollamaProvider.RefreshModels(ctx)
		return nil
	})

	// Startup banner
	app.OnStart(func(_ *gofr.Context) error {
		line := strings.Repeat("\u2500", 35)
		fmt.Println()
		fmt.Println("  LLM Gateway v1.0")
		fmt.Println("  " + line)
		fmt.Println("  Providers:")

		for _, name := range reg.ProviderNames() {
			p, ok := reg.GetProvider(name)
			if !ok {
				continue
			}

			modelCount := len(p.Models())

			// Include embedding models if the provider supports them
			if ep, ok := p.(provider.EmbeddingProvider); ok {
				modelCount += len(ep.EmbeddingModels())
			}

			fmt.Printf("    \u2713 %-12s (%d models)\n", name, modelCount)
		}

		fmt.Println()

		masterKeyStatus := "not set"
		if masterKey != "" {
			masterKeyStatus = "configured"
		}

		fmt.Printf("  Routing: %s | Retry: %d | Cache TTL: %ds\n", routingStrategy, retryMax, cacheTTL)
		fmt.Printf("  Master key: %s\n", masterKeyStatus)

		if webSearchCfg.Enabled {
			fmt.Printf("  Web Search: %s | Max: %d results | Cache: %ds\n",
				webSearchCfg.Provider, webSearchCfg.MaxResults, webSearchCfg.CacheTTL)
		} else {
			fmt.Println("  Web Search: disabled")
		}

		fmt.Println("  " + line)
		fmt.Println()

		return nil
	})

	// Worker pool for batch processing
	pool, _ := workerpool.NewWorkerPool(workerpool.PoolConfig{
		Name:        "batch-processor",
		Workers:     batchWorkers,
		QueueSize:   batchWorkers * 10,
		TaskTimeout: time.Duration(batchTaskTimeoutSec) * time.Second,
	})

	_ = pool.Start()

	bp := batch.NewProcessor(reg, c, router, pool)

	// API key authentication middleware with per-key config
	// Master key is also added to valid keys so it passes auth
	if gatewayKeys != "" || masterKey != "" {
		keys := middleware.ParseAPIKeys(gatewayKeys)
		if masterKey != "" {
			keys[masterKey] = true
		}

		keyConfigs := middleware.ParseKeyConfigs(keyConfig)
		app.UseMiddleware(middleware.APIKeyAuth(keys, keyConfigs, keyStore))
	}

	// --- Web search service ---
	var searchSvc *websearch.Service

	if webSearchCfg.Enabled {
		searchRegistry := websearch.NewRegistry(webSearchCfg.Provider)
		searchRegistry.Register(&websearch.SearXNG{})
		searchRegistry.Register(websearch.NewTavily(webSearchCfg.APIKey))
		searchRegistry.Register(&websearch.DuckDuckGo{})

		searchSvc = &websearch.Service{
			Registry:  searchRegistry,
			Cache:     websearch.NewSearchCache(webSearchCfg.CacheTTL),
			Config:    webSearchCfg,
			LLMRouter: router,
			LLMReg:    reg,
		}
	}

	// --- Handler structs ---
	api := &handler.APIHandler{
		Registry:   reg,
		Cache:      c,
		Router:     router,
		Guardrails: globalGuardrails,
		Search:     searchSvc,
	}

	admin := &handler.AdminHandler{
		MasterKey: masterKey,
		KeyStore:  keyStore,
	}

	batches := &handler.BatchHandler{
		Processor: bp,
	}

	assistants := &handler.AssistantHandler{
		API: api,
	}

	// Serve admin UI — explicit handler for the root page, static files for assets
	app.AddStaticFiles("/admin", "./admin/static")
	app.GET("/admin", admin.AdminPage())

	// Routes — OpenAI-compatible API
	app.POST("/v1/chat/completions", api.ChatCompletion())
	app.POST("/v1/completions", api.Completions())
	app.POST("/v1/embeddings", api.Embeddings())
	app.POST("/v1/moderations", api.Moderations())
	app.POST("/v1/images/generations", api.ImageGenerations())
	app.POST("/v1/images/edits", api.ImageEdits())
	app.POST("/v1/images/variations", api.ImageVariations())
	app.POST("/v1/audio/speech", api.AudioSpeech())
	app.POST("/v1/audio/transcriptions", api.AudioTranscriptions())
	app.POST("/v1/rerank", api.Rerank())
	app.GET("/v1/models", api.ListModels())
	app.GET("/health", api.Health())
	app.GET("/health/providers", api.HealthProviders())
	app.GET("/spend/report", admin.SpendReport())
	app.GET("/spend/self", admin.SpendSelf())

	// Responses API (OpenAI Responses API — the modern agentic format)
	app.POST("/v1/responses", api.CreateResponse())

	// Agent execution loop — runs multi-turn tool-use automatically
	app.POST("/v1/agents/run", api.AgentRun())

	// Files API
	app.POST("/v1/files", api.UploadFile())
	app.GET("/v1/files", api.ListFiles())
	app.GET("/v1/files/{id}", api.GetFile())
	app.DELETE("/v1/files/{id}", api.DeleteFile())
	app.GET("/v1/files/{id}/content", api.GetFileContent())

	// Fine-tuning API
	app.POST("/v1/fine_tuning/jobs", api.CreateFineTuningJob())
	app.GET("/v1/fine_tuning/jobs", api.ListFineTuningJobs())
	app.GET("/v1/fine_tuning/jobs/{id}", api.GetFineTuningJob())
	app.POST("/v1/fine_tuning/jobs/{id}/cancel", api.CancelFineTuningJob())
	app.GET("/v1/fine_tuning/jobs/{id}/events", api.ListFineTuningEvents())

	// Assistants API
	app.POST("/v1/assistants", assistants.CreateAssistant())
	app.GET("/v1/assistants", assistants.ListAssistants())
	app.GET("/v1/assistants/{id}", assistants.GetAssistant())
	app.DELETE("/v1/assistants/{id}", assistants.DeleteAssistant())

	// Threads API
	app.POST("/v1/threads", assistants.CreateThread())
	app.GET("/v1/threads/{id}", assistants.GetThread())
	app.DELETE("/v1/threads/{id}", assistants.DeleteThread())

	// Thread Messages API
	app.POST("/v1/threads/{id}/messages", assistants.CreateMessage())
	app.GET("/v1/threads/{id}/messages", assistants.ListMessages())

	// Thread Runs API
	app.POST("/v1/threads/{id}/runs", assistants.CreateRun())
	app.GET("/v1/threads/{id}/runs", assistants.ListRuns())
	app.GET("/v1/threads/{id}/runs/{run_id}", assistants.GetRun())
	app.POST("/v1/threads/{id}/runs/{run_id}/cancel", assistants.CancelRun())

	// Auth check — returns role (admin vs user) for the authenticated key
	app.GET("/auth/check", admin.AuthCheck())

	// Self-service key info — virtual key holders can see their own key info
	app.GET("/key/self", admin.KeySelf())

	// Batch API
	app.POST("/v1/batches", batches.Submit())
	app.GET("/v1/batches", batches.List())
	app.GET("/v1/batches/{id}", batches.Status())
	app.GET("/v1/batches/{id}/results", batches.Results())
	app.POST("/v1/batches/{id}/cancel", batches.Cancel())

	// Virtual key management (requires master key)
	app.POST("/key/generate", admin.KeyGenerate())
	app.GET("/key/info", admin.KeyInfo())
	app.DELETE("/key/{id}", admin.KeyDelete())
	app.POST("/key/{id}/rotate", admin.KeyRotate())
	app.GET("/keys", admin.ListKeys())

	// Multi-tenancy CRUD (requires master key)
	app.POST("/teams", admin.CreateTeam())
	app.GET("/teams", admin.ListTeams())
	app.DELETE("/teams/{id}", admin.DeleteTeam())
	app.POST("/users", admin.CreateUser())
	app.GET("/users", admin.ListUsers())
	app.DELETE("/users/{id}", admin.DeleteUser())
	app.POST("/organizations", admin.CreateOrg())
	app.GET("/organizations", admin.ListOrgs())
	app.DELETE("/organizations/{id}", admin.DeleteOrg())
	app.GET("/audit/log", admin.AuditLog())

	// Nested org routes — same handlers, org_id injected via path param
	app.GET("/organizations/{org_id}/teams", admin.ListTeams())
	app.GET("/organizations/{org_id}/users", admin.ListUsers())
	app.GET("/organizations/{org_id}/keys", admin.ListKeys())

	// Guardrail config management (requires master key)
	app.GET("/guardrails", admin.ListGuardrails())
	app.POST("/guardrails", admin.UpsertGuardrail())
	app.DELETE("/guardrails/{id}", admin.DeleteGuardrail())

	// Gateway settings (requires master key)
	app.GET("/settings", admin.Settings(&handler.SettingsConfig{
		GetOrDefault: app.Config.GetOrDefault,
	}))
	app.PUT("/settings/providers", admin.SaveProviderConfig())

	// WebSocket streaming endpoint
	app.WebSocket("/v1/chat/completions/stream", api.ChatCompletionStream())

	app.Run()

	// Graceful shutdown of worker pool
	_ = pool.ShutdownGraceful(context.Background())
}

// buildStrategy creates a routing strategy from config, wiring up trackers as needed.
func buildStrategy(name string, inFlight *routing.InFlightTracker, latency *routing.LatencyTracker, usage *routing.UsageTracker) routing.Strategy {
	switch name {
	case "round-robin":
		return &routing.RoundRobinStrategy{}
	case "weighted":
		return &routing.WeightedStrategy{Weights: make(map[string]int)}
	case "least-busy":
		return &routing.LeastBusyStrategy{Tracker: inFlight}
	case "latency":
		return &routing.LatencyStrategy{Tracker: latency}
	case "usage":
		return &routing.UsageStrategy{Tracker: usage}
	default:
		return &routing.SimpleStrategy{}
	}
}

// isValidKey returns true if an API key looks real (not empty, not a placeholder).
func isValidKey(key string) bool {
	if key == "" {
		return false
	}

	placeholders := []string{"your-", "sk-xxx", "changeme", "placeholder", "TODO", "FIXME"}
	for _, p := range placeholders {
		if strings.HasPrefix(strings.ToLower(key), strings.ToLower(p)) {
			return false
		}
	}

	return true
}

// registerFallbackChain parses a comma-separated provider list and registers
// a "fallback" composite provider in the registry.
func registerFallbackChain(reg *provider.Registry, chain string, cooldown *routing.CooldownTracker) {
	names := strings.Split(chain, ",")

	var providers []provider.Provider

	for _, name := range names {
		name = strings.TrimSpace(name)

		p, _, err := reg.ResolveProvider(name + "/dummy")
		if err != nil {
			continue
		}

		providers = append(providers, p)
	}

	if len(providers) > 1 {
		fb := provider.NewFallbackProvider("fallback", providers)
		fb.SetCooldown(cooldown)
		reg.Register(fb)
	}
}
