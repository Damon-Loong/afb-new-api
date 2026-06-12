package openaicompat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("responses request is nil")
	}
	if strings.TrimSpace(req.PreviousResponseID) != "" {
		return nil, errors.New("previous_response_id is not supported by chat completions fallback")
	}

	chatReq := &dto.GeneralOpenAIRequest{
		Model:         req.Model,
		Stream:        req.Stream,
		StreamOptions: req.StreamOptions,
		MaxTokens:     req.MaxOutputTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		User:          req.User,
	}
	if req.Reasoning != nil {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}

	if len(req.Instructions) > 0 && common.GetJsonType(req.Instructions) != "null" {
		instructions := responsesRawText(req.Instructions)
		if strings.TrimSpace(instructions) != "" {
			chatReq.Messages = append(chatReq.Messages, dto.Message{
				Role:    "system",
				Content: instructions,
			})
		}
	}

	messages, err := responsesInputToChatMessages(req.Input)
	if err != nil {
		return nil, err
	}
	chatReq.Messages = append(chatReq.Messages, messages...)

	if len(req.Tools) > 0 && common.GetJsonType(req.Tools) != "null" {
		tools, err := responsesToolsToChatTools(req.Tools)
		if err != nil {
			return nil, err
		}
		chatReq.Tools = tools
	}

	if len(req.ToolChoice) > 0 && common.GetJsonType(req.ToolChoice) != "null" {
		var toolChoice any
		if err := common.Unmarshal(req.ToolChoice, &toolChoice); err == nil {
			chatReq.ToolChoice = responsesToolChoiceToChatToolChoice(toolChoice)
		}
	}
	if len(req.ParallelToolCalls) > 0 && common.GetJsonType(req.ParallelToolCalls) != "null" {
		var parallelToolCalls bool
		if err := common.Unmarshal(req.ParallelToolCalls, &parallelToolCalls); err == nil {
			chatReq.ParallelTooCalls = &parallelToolCalls
		}
	}

	return chatReq, nil
}

func responsesRawText(raw json.RawMessage) string {
	switch common.GetJsonType(raw) {
	case "string":
		var s string
		_ = common.Unmarshal(raw, &s)
		return s
	default:
		return string(raw)
	}
}

func responsesInputToChatMessages(raw json.RawMessage) ([]dto.Message, error) {
	if len(raw) == 0 || common.GetJsonType(raw) == "null" {
		return nil, nil
	}
	if common.GetJsonType(raw) == "string" {
		var text string
		if err := common.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		return []dto.Message{{Role: "user", Content: text}}, nil
	}
	if common.GetJsonType(raw) != "array" {
		return []dto.Message{{Role: "user", Content: responsesRawText(raw)}}, nil
	}

	var items []map[string]any
	if err := common.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	messages := make([]dto.Message, 0, len(items))
	for _, item := range items {
		itemType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch itemType {
		case "function_call":
			message := dto.Message{
				Role:    "assistant",
				Content: nil,
			}
			message.SetToolCalls([]dto.ToolCallRequest{
				{
					ID:   common.Interface2String(item["call_id"]),
					Type: "function",
					Function: dto.FunctionRequest{
						Name:      common.Interface2String(item["name"]),
						Arguments: common.Interface2String(item["arguments"]),
					},
				},
			})
			messages = append(messages, message)
		case "function_call_output":
			messages = append(messages, dto.Message{
				Role:       "tool",
				Content:    common.Interface2String(item["output"]),
				ToolCallId: common.Interface2String(item["call_id"]),
			})
		default:
			role := strings.TrimSpace(common.Interface2String(item["role"]))
			if role == "" {
				role = "user"
			}
			content, err := responsesContentToChatContent(normalizeResponsesAny(item["content"]))
			if err != nil {
				return nil, err
			}
			messages = append(messages, dto.Message{
				Role:    role,
				Content: content,
			})
		}
	}
	return messages, nil
}

func responsesContentToChatContent(content any) (any, error) {
	switch value := content.(type) {
	case nil:
		return "", nil
	case string:
		return value, nil
	case []any:
		parts := make([]any, 0, len(value))
		for _, partAny := range value {
			part, ok := partAny.(map[string]any)
			if !ok {
				continue
			}
			partType := common.Interface2String(part["type"])
			switch partType {
			case "input_text", "output_text", "text":
				parts = append(parts, map[string]any{
					"type": dto.ContentTypeText,
					"text": common.Interface2String(part["text"]),
				})
			case "input_image", "input_file":
				return nil, fmt.Errorf("%s is not supported by chat completions fallback", partType)
			default:
				if text := common.Interface2String(part["text"]); text != "" {
					parts = append(parts, map[string]any{
						"type": dto.ContentTypeText,
						"text": text,
					})
				}
			}
		}
		if len(parts) > 0 {
			return parts, nil
		}
	}
	return fmt.Sprintf("%v", content), nil
}

func normalizeResponsesAny(value any) any {
	switch v := value.(type) {
	case json.RawMessage:
		var decoded any
		decoder := json.NewDecoder(bytes.NewReader(v))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err == nil {
			return normalizeResponsesAny(decoded)
		}
		return string(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeResponsesAny(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalizeResponsesAny(item)
		}
		return out
	default:
		return value
	}
}

func responsesToolsToChatTools(raw json.RawMessage) ([]dto.ToolCallRequest, error) {
	var toolItems []map[string]any
	if err := common.Unmarshal(raw, &toolItems); err != nil {
		return nil, err
	}

	tools := make([]dto.ToolCallRequest, 0, len(toolItems))
	for _, item := range toolItems {
		toolType := strings.TrimSpace(common.Interface2String(item["type"]))
		switch toolType {
		case "function":
			name := common.Interface2String(item["name"])
			description := common.Interface2String(item["description"])
			parameters := item["parameters"]
			if function, ok := item["function"].(map[string]any); ok {
				if name == "" {
					name = common.Interface2String(function["name"])
				}
				if description == "" {
					description = common.Interface2String(function["description"])
				}
				if parameters == nil {
					parameters = function["parameters"]
				}
			}
			if name == "" {
				continue
			}
			tools = append(tools, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        name,
					Description: description,
					Parameters:  parameters,
				},
			})
		default:
			return nil, fmt.Errorf("responses built-in tool %q is not supported by chat completions fallback", toolType)
		}
	}
	return tools, nil
}

func responsesToolChoiceToChatToolChoice(toolChoice any) any {
	if choice, ok := toolChoice.(map[string]any); ok {
		if choiceType := common.Interface2String(choice["type"]); choiceType == "function" {
			if name := common.Interface2String(choice["name"]); name != "" {
				return map[string]any{
					"type": "function",
					"function": map[string]any{
						"name": name,
					},
				}
			}
		}
	}
	return toolChoice
}
