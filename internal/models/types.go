package models

// ModelSpec describes a model's capabilities as reported by a model registry.
type ModelSpec struct {
	ID               string `json:"id"`                // e.g. "anthropic/claude-sonnet-4-5-20250929"
	Name             string `json:"name"`              // display name
	ContextLength    int    `json:"context_length"`    // max context window in tokens
	MaxOutputTokens  int    `json:"max_output_tokens"` // max completion tokens (0 = unknown)
	SupportsTools    bool   `json:"supports_tools"`
	SupportsReasoning bool  `json:"supports_reasoning"`
	SupportsImages   bool   `json:"supports_images"`
}
