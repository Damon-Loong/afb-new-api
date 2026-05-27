package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func responsesRequestToOpenAIChatRequest(request dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	chatRequest := &dto.GeneralOpenAIRequest{
		Model:       request.Model,
		Stream:      request.Stream,
		MaxTokens:   request.MaxOutputTokens,
		Temperature: request.Temperature,
		TopP:        request.TopP,
		User:        request.User,
	}
	if request.Reasoning != nil {
		chatRequest.ReasoningEffort = request.Reasoning.Effort
	}

	if len(request.Instructions) > 0 && common.GetJsonType(request.Instructions) != "null" {
		instructions := responsesRawText(request.Instructions)
		if strings.TrimSpace(instructions) != "" {
			chatRequest.Messages = append(chatRequest.Messages, dto.Message{
				Role:    "system",
				Content: instructions,
			})
		}
	}

	messages, err := responsesInputToChatMessages(request.Input)
	if err != nil {
		return nil, err
	}
	chatRequest.Messages = append(chatRequest.Messages, messages...)

	if len(request.Tools) > 0 {
		tools, webSearchOptions, err := responsesToolsToChatTools(request.Tools)
		if err != nil {
			return nil, err
		}
		chatRequest.Tools = tools
		chatRequest.WebSearchOptions = webSearchOptions
	}

	if len(request.ToolChoice) > 0 && common.GetJsonType(request.ToolChoice) != "null" {
		var toolChoice any
		if err := common.Unmarshal(request.ToolChoice, &toolChoice); err == nil {
			chatRequest.ToolChoice = responsesToolChoiceToChatToolChoice(toolChoice)
		}
	}
	if len(request.ParallelToolCalls) > 0 && common.GetJsonType(request.ParallelToolCalls) != "null" {
		var parallelToolCalls bool
		if err := common.Unmarshal(request.ParallelToolCalls, &parallelToolCalls); err == nil {
			chatRequest.ParallelTooCalls = &parallelToolCalls
		}
	}

	return chatRequest, nil
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
			content := responsesContentToChatContent(normalizeResponsesAny(item["content"]))
			messages = append(messages, dto.Message{
				Role:    role,
				Content: content,
			})
		}
	}
	return messages, nil
}

func responsesContentToChatContent(content any) any {
	switch value := content.(type) {
	case string:
		return value
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
			case "input_image":
				parts = append(parts, map[string]any{
					"type":      dto.ContentTypeImageURL,
					"image_url": part["image_url"],
				})
			case "input_file":
				parts = append(parts, map[string]any{
					"type": dto.ContentTypeFile,
					"file": part["file"],
				})
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
			return parts
		}
	}
	return fmt.Sprintf("%v", content)
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

func responsesToolsToChatTools(raw json.RawMessage) ([]dto.ToolCallRequest, *dto.WebSearchOptions, error) {
	var toolItems []map[string]any
	if err := common.Unmarshal(raw, &toolItems); err != nil {
		return nil, nil, err
	}

	tools := make([]dto.ToolCallRequest, 0, len(toolItems))
	var webSearchOptions *dto.WebSearchOptions
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
		case "web_search", dto.BuildInToolWebSearchPreview:
			webSearchOptions = &dto.WebSearchOptions{SearchContextSize: "medium"}
			if contextSize := common.Interface2String(item["search_context_size"]); contextSize != "" {
				webSearchOptions.SearchContextSize = contextSize
			}
			if userLocation, ok := item["user_location"]; ok && userLocation != nil {
				if b, err := common.Marshal(userLocation); err == nil {
					webSearchOptions.UserLocation = b
				}
			}
		}
	}
	return tools, webSearchOptions, nil
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
