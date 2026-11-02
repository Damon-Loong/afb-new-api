package helper

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ApplyResponsesReasoningPreference translates the OPC control after model routing.
// Ordinary API clients that do not send reasoning_preference are left untouched.
// The gateway-only field is never sent to an upstream, including in passthrough mode.
func ApplyResponsesReasoningPreference(body []byte, model string) ([]byte, error) {
	preference := gjson.GetBytes(body, "reasoning_preference")
	if !preference.Exists() {
		return body, nil
	}
	if preference.Type != gjson.String {
		return nil, fmt.Errorf("reasoning_preference must be off, low, medium, or high")
	}
	value := preference.String()
	switch value {
	case "off", "low", "medium", "high":
	default:
		return nil, fmt.Errorf("reasoning_preference must be off, low, medium, or high")
	}
	body, err := sjson.DeleteBytes(body, "reasoning_preference")
	if err != nil {
		return nil, err
	}
	effort, supported := responsesReasoningEffort(model, value)
	if !supported {
		return body, nil // Other providers retain their existing request behavior.
	}
	return sjson.SetBytes(body, "reasoning.effort", effort)
}

func responsesReasoningEffort(model, preference string) (string, bool) {
	model = reasoningModelName(model)
	if strings.HasPrefix(model, "claude-") {
		if preference == "off" {
			if claudeThinkingAlwaysOn(model) {
				return "low", true
			}
			return "none", true
		}
		return preference, true
	}
	if strings.HasPrefix(model, "deepseek-") {
		if preference == "off" {
			return "none", true
		}
		if preference == "medium" {
			return "high", true
		}
		return preference, true
	}
	if strings.HasPrefix(model, "doubao-seed-") {
		if preference == "off" {
			return "minimal", true
		}
		if preference == "medium" && (strings.HasPrefix(model, "doubao-seed-2-1-") || model == "doubao-seed-evolving") {
			return "high", true
		}
		return preference, true
	}
	if strings.HasPrefix(model, "gpt-5") || strings.HasPrefix(model, "gpt-6") {
		if model == "gpt-5-pro" || strings.HasPrefix(model, "gpt-5-pro-") {
			return "high", true
		}
		if strings.HasPrefix(model, "gpt-5.2-pro") || strings.HasPrefix(model, "gpt-5.4-pro") {
			if preference == "off" || preference == "low" {
				return "medium", true
			}
		}
		if preference != "off" {
			return preference, true
		}
		if strings.HasPrefix(model, "gpt-6-astra") || strings.HasPrefix(model, "gpt-6.1-sol") || model == "gpt-5-codex" {
			return "low", true
		}
		// The original GPT-5 family uses minimal instead of none.
		if model == "gpt-5" || strings.HasPrefix(model, "gpt-5-2025-") || strings.HasPrefix(model, "gpt-5-mini") || strings.HasPrefix(model, "gpt-5-nano") {
			return "minimal", true
		}
		return "none", true
	}
	return "", false
}

func reasoningModelName(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(model, "/"); i >= 0 {
		model = model[i+1:]
	}
	return model
}

func claudeThinkingAlwaysOn(model string) bool {
	return strings.HasPrefix(model, "claude-fable-5") ||
		strings.HasPrefix(model, "claude-mythos-") ||
		strings.HasPrefix(model, "claude-opus-5-5") ||
		strings.HasPrefix(model, "claude-sonnet-5-5")
}

func claudeSupportsAdaptiveThinking(model string) bool {
	return claudeThinkingAlwaysOn(model) ||
		strings.HasPrefix(model, "claude-opus-5") ||
		strings.HasPrefix(model, "claude-sonnet-5") ||
		strings.HasPrefix(model, "claude-haiku-5") ||
		strings.HasPrefix(model, "claude-opus-4-6") ||
		strings.HasPrefix(model, "claude-opus-4-7") ||
		strings.HasPrefix(model, "claude-opus-4-8") ||
		strings.HasPrefix(model, "claude-sonnet-4-6")
}

// ApplyClaudeReasoningPreference runs after Responses-to-Messages conversion so
// legacy budget conversion cannot overwrite the selected adaptive effort.
// It is called only when an OPC request explicitly supplies the new preference.
func ApplyClaudeReasoningPreference(request *dto.ClaudeRequest, preference string) (string, error) {
	switch preference {
	case "off", "low", "medium", "high":
	default:
		return "", fmt.Errorf("reasoning_preference must be off, low, medium, or high")
	}
	model := reasoningModelName(request.Model)
	adaptive := claudeSupportsAdaptiveThinking(model)
	extended := strings.HasPrefix(model, "claude-sonnet-4") ||
		strings.HasPrefix(model, "claude-opus-4") ||
		strings.HasPrefix(model, "claude-haiku-4-5") ||
		strings.HasPrefix(model, "claude-3-7-sonnet")
	if !adaptive && !extended {
		return "", nil // Models without thinking retain the existing behavior.
	}
	effort, _ := responsesReasoningEffort(model, preference)
	if adaptive {
		request.Temperature, request.TopP, request.TopK = nil, nil, nil
		if effort == "none" {
			request.Thinking = &dto.Thinking{Type: "disabled"}
			request.OutputConfig = []byte(`{"effort":"low"}`)
		} else {
			request.Thinking = &dto.Thinking{Type: "adaptive", Display: "summarized"}
			request.OutputConfig = []byte(fmt.Sprintf(`{"effort":"%s"}`, effort))
		}
		return effort, nil
	}
	request.OutputConfig = nil
	if preference == "off" {
		request.Thinking = &dto.Thinking{Type: "disabled"}
		return "none", nil
	}
	// Legacy models only offer a token budget. Reuse the existing converter's
	// low/medium/high budgets, capped below the caller's output-token limit.
	budget := map[string]int{"low": 1280, "medium": 2048, "high": 4096}[preference]
	if request.MaxTokens == nil || *request.MaxTokens <= 1024 {
		return "", fmt.Errorf("max_output_tokens must be greater than 1024 for Claude extended thinking")
	}
	if uint(budget) >= *request.MaxTokens {
		budget = int(*request.MaxTokens - 1)
	}
	request.Thinking = &dto.Thinking{Type: "enabled", BudgetTokens: &budget}
	one := 1.0
	request.Temperature, request.TopP, request.TopK = &one, nil, nil
	return effort, nil
}
