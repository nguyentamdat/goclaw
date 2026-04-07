package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	openRouterModelsURL = "https://openrouter.ai/api/v1/models"
	defaultCacheTTL     = 24 * time.Hour
	fetchTimeout        = 30 * time.Second
)

// Registry provides model capability lookups backed by the OpenRouter model registry.
// Results are cached in memory with a configurable TTL.
type Registry struct {
	mu        sync.RWMutex
	specs     map[string]*ModelSpec // keyed by canonical model ID (lowercase)
	aliases   map[string]string     // alias → canonical ID (for fuzzy matching)
	fetchedAt time.Time
	ttl       time.Duration
	client    *http.Client
}

// NewRegistry creates a model registry with the given cache TTL.
// Pass 0 for default (24h).
func NewRegistry(ttl time.Duration) *Registry {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	return &Registry{
		specs:   make(map[string]*ModelSpec),
		aliases: make(map[string]string),
		ttl:     ttl,
		client:  &http.Client{Timeout: fetchTimeout},
	}
}

// Lookup returns the model spec for the given model ID.
// It tries exact match first, then fuzzy matching (strip provider prefix, version suffixes).
// Returns nil if not found. Does NOT trigger a fetch — call Refresh separately.
func (r *Registry) Lookup(model string) *ModelSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if spec := r.lookupLocked(model); spec != nil {
		return spec
	}
	return nil
}

func (r *Registry) lookupLocked(model string) *ModelSpec {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return nil
	}

	// 1. Exact match on full ID (e.g. "anthropic/claude-sonnet-4-5-20250929")
	if spec, ok := r.specs[normalized]; ok {
		return spec
	}

	// 2. Check aliases
	if canonical, ok := r.aliases[normalized]; ok {
		if spec, ok := r.specs[canonical]; ok {
			return spec
		}
	}

	// 3. Try matching by model name only (strip provider prefix)
	// e.g. "claude-sonnet-4-5-20250929" should match "anthropic/claude-sonnet-4-5-20250929"
	for id, spec := range r.specs {
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			if id[idx+1:] == normalized {
				return spec
			}
		}
	}

	// 4. Fuzzy: strip common suffixes/prefixes for Ollama-style names
	// e.g. "qwen3:8b" → try "qwen/qwen3" or match partial
	cleaned := cleanModelName(normalized)
	if cleaned != normalized {
		if spec, ok := r.specs[cleaned]; ok {
			return spec
		}
		if canonical, ok := r.aliases[cleaned]; ok {
			if spec, ok := r.specs[canonical]; ok {
				return spec
			}
		}
	}

	return nil
}

// cleanModelName normalizes model names for fuzzy matching.
// Strips Ollama tag suffixes (":8b", ":latest") and common prefixes.
func cleanModelName(name string) string {
	// Strip Ollama-style tags: "qwen3:8b" → "qwen3"
	if idx := strings.Index(name, ":"); idx > 0 {
		name = name[:idx]
	}
	return name
}

// Stale returns true if the cache is empty or expired.
func (r *Registry) Stale() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.specs) == 0 || time.Since(r.fetchedAt) > r.ttl
}

// Count returns the number of cached model specs.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.specs)
}

// Refresh fetches the model registry from OpenRouter and updates the cache.
// Safe to call concurrently — only one fetch runs at a time.
func (r *Registry) Refresh(ctx context.Context) error {
	models, err := r.fetchModels(ctx)
	if err != nil {
		return fmt.Errorf("models.registry: fetch failed: %w", err)
	}

	specs := make(map[string]*ModelSpec, len(models))
	aliases := make(map[string]string, len(models)*2)

	for _, m := range models {
		id := strings.ToLower(m.ID)
		spec := &ModelSpec{
			ID:            m.ID,
			Name:          m.Name,
			ContextLength: int(m.ContextLength),
		}

		if m.TopProvider.MaxCompletionTokens != nil {
			spec.MaxOutputTokens = int(*m.TopProvider.MaxCompletionTokens)
		}

		spec.SupportsTools = slices.Contains(m.SupportedParams, "tools")
		spec.SupportsReasoning = slices.Contains(m.SupportedParams, "reasoning")
		spec.SupportsImages = slices.Contains(m.Architecture.InputModalities, "image")

		specs[id] = spec

		// Register alias: model name without provider prefix
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			shortName := id[idx+1:]
			// Only register if unambiguous (first wins)
			if _, exists := aliases[shortName]; !exists {
				aliases[shortName] = id
			}
		}
	}

	r.mu.Lock()
	r.specs = specs
	r.aliases = aliases
	r.fetchedAt = time.Now()
	r.mu.Unlock()

	slog.Info("models.registry.refreshed", "count", len(specs))
	return nil
}

// RefreshIfStale refreshes only if the cache is stale. Returns nil if cache is fresh.
func (r *Registry) RefreshIfStale(ctx context.Context) error {
	if !r.Stale() {
		return nil
	}
	return r.Refresh(ctx)
}

// StartBackgroundRefresh launches a goroutine that refreshes the registry periodically.
// Stops when ctx is cancelled.
func (r *Registry) StartBackgroundRefresh(ctx context.Context) {
	// Initial fetch
	go func() {
		if err := r.Refresh(ctx); err != nil {
			slog.Warn("models.registry.initial_refresh", "error", err)
		}

		ticker := time.NewTicker(r.ttl)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := r.Refresh(ctx); err != nil {
					slog.Warn("models.registry.refresh", "error", err)
				}
			}
		}
	}()
}

// --- OpenRouter API types ---

type openRouterResponse struct {
	Data []openRouterModel `json:"data"`
}

type openRouterModel struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	ContextLength   int64               `json:"context_length"`
	Architecture    openRouterArch      `json:"architecture"`
	TopProvider     openRouterProvider   `json:"top_provider"`
	SupportedParams []string            `json:"supported_parameters"`
}

type openRouterArch struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Tokenizer        string   `json:"tokenizer"`
}

type openRouterProvider struct {
	ContextLength       int64  `json:"context_length"`
	MaxCompletionTokens *int64 `json:"max_completion_tokens"`
}

func (r *Registry) fetchModels(ctx context.Context) ([]openRouterModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openRouterModelsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "GoClaw/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}

	var result openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return result.Data, nil
}
