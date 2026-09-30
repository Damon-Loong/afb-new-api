package common

import "strings"

// UsesMaxCompletionTokens identifies chat models that reject max_tokens.
func UsesMaxCompletionTokens(model string) bool {
	return strings.HasPrefix(model, "o") || strings.HasPrefix(model, "gpt-5") || strings.HasPrefix(model, "gpt-6")
}
