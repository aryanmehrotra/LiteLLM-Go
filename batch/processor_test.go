package batch

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"aryanmehrotra/llm-gateway/cache"
	"aryanmehrotra/llm-gateway/provider"
	"aryanmehrotra/llm-gateway/routing"
	"aryanmehrotra/llm-gateway/workerpool"
)

func TestNewProcessor(t *testing.T) {
	reg := provider.NewRegistry("openai")
	c := cache.New(300)
	router := routing.NewRouter(routing.DefaultRetryPolicy(3, 0), nil, &routing.SimpleStrategy{})

	pool, err := workerpool.NewWorkerPool(workerpool.PoolConfig{
		Name:      "test",
		Workers:   2,
		QueueSize: 10,
	})
	assert.NoError(t, err)

	bp := NewProcessor(reg, c, router, pool)
	assert.NotNil(t, bp)
	assert.Equal(t, reg, bp.registry)
	assert.Equal(t, c, bp.cache)
	assert.Equal(t, router, bp.router)
	assert.Equal(t, pool, bp.pool)
}
