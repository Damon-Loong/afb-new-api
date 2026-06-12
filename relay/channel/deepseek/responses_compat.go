package deepseek

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type responsesToolCallBuilder struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func normalizeResponsesUsage(usage *dto.Usage) *dto.Usage {
	if usage == nil {
		return nil
	}
	if usage.InputTokens == 0 {
		usage.InputTokens = usage.PromptTokens
	}
	if usage.OutputTokens == 0 {
		usage.OutputTokens = usage.CompletionTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if usage.InputTokensDetails == nil {
		usage.InputTokensDetails = &usage.PromptTokensDetails
	}
	return usage
}

func responsesStatus(status string) []byte {
	data, _ := common.Marshal(status)
	return data
}

func textResponsesOutput(text string) []dto.ResponsesOutput {
	if text == "" {
		return nil
	}
	return []dto.ResponsesOutput{
		{
			Type:   "message",
			ID:     fmt.Sprintf("msg_%s", common.GetUUID()),
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{
				{
					Type:        "output_text",
					Text:        text,
					Annotations: []interface{}{},
				},
			},
		},
	}
}

func toolResponsesOutput(toolCalls []dto.ToolCallResponse) []dto.ResponsesOutput {
	output := make([]dto.ResponsesOutput, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		name := strings.TrimSpace(toolCall.Function.Name)
		if name == "" {
			continue
		}
		callID := strings.TrimSpace(toolCall.ID)
		if callID == "" {
			callID = fmt.Sprintf("call_%s", common.GetUUID())
		}
		output = append(output, dto.ResponsesOutput{
			Type:      "function_call",
			ID:        callID,
			CallId:    callID,
			Name:      name,
			Arguments: toolCall.Function.Arguments,
			Status:    "completed",
		})
	}
	return output
}

func requestToolResponsesOutput(toolCalls []dto.ToolCallRequest) []dto.ResponsesOutput {
	output := make([]dto.ResponsesOutput, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		name := strings.TrimSpace(toolCall.Function.Name)
		if name == "" {
			continue
		}
		callID := strings.TrimSpace(toolCall.ID)
		if callID == "" {
			callID = fmt.Sprintf("call_%s", common.GetUUID())
		}
		output = append(output, dto.ResponsesOutput{
			Type:      "function_call",
			ID:        callID,
			CallId:    callID,
			Name:      name,
			Arguments: toolCall.Function.Arguments,
			Status:    "completed",
		})
	}
	return output
}

func chatCreatedUnix(created any) int64 {
	switch value := created.(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case float64:
		return int64(value)
	case json.Number:
		n, _ := value.Int64()
		return n
	default:
		return 0
	}
}

func buildChatAsResponsesResponse(id string, created int64, model string, output []dto.ResponsesOutput, usage *dto.Usage) *dto.OpenAIResponsesResponse {
	if strings.TrimSpace(id) == "" {
		id = fmt.Sprintf("resp_%s", common.GetUUID())
	}
	if created <= 0 {
		created = common.GetTimestamp()
	}
	return &dto.OpenAIResponsesResponse{
		ID:        id,
		Object:    "response",
		CreatedAt: int(created),
		Status:    responsesStatus("completed"),
		Model:     model,
		Output:    output,
		Usage:     normalizeResponsesUsage(usage),
	}
}

func chatResponseToResponses(chatResp *dto.OpenAITextResponse) *dto.OpenAIResponsesResponse {
	if chatResp == nil {
		return buildChatAsResponsesResponse("", 0, "", nil, nil)
	}
	output := make([]dto.ResponsesOutput, 0, 2)
	for _, choice := range chatResp.Choices {
		output = append(output, textResponsesOutput(choice.Message.StringContent())...)
		output = append(output, requestToolResponsesOutput(choice.Message.ParseToolCalls())...)
	}
	usage := chatResp.Usage
	return buildChatAsResponsesResponse(chatResp.Id, chatCreatedUnix(chatResp.Created), chatResp.Model, output, &usage)
}

func sendResponsesEvent(c *gin.Context, streamResponse dto.ResponsesStreamResponse) {
	jsonData, err := common.Marshal(streamResponse)
	if err != nil {
		logger.LogError(c, "failed to marshal responses stream event: "+err.Error())
		return
	}
	helper.ResponseChunkData(c, streamResponse, string(jsonData))
}

func deepSeekChatCompletionsAsResponsesHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if info != nil && info.IsStream {
		return deepSeekChatCompletionsStreamAsResponsesHandler(c, resp, info)
	}
	return deepSeekChatCompletionsResponseAsResponsesHandler(c, resp)
}

func deepSeekChatCompletionsResponseAsResponsesHandler(c *gin.Context, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responsesResp := chatResponseToResponses(&chatResp)
	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return normalizeResponsesUsage(responsesResp.Usage), nil
}

func deepSeekChatCompletionsStreamAsResponsesHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	usage := &dto.Usage{}
	responseID := helper.GetResponseID(c)
	createdAt := common.GetTimestamp()
	model := info.UpstreamModelName
	var textBuilder strings.Builder
	toolBuilders := map[int]*responsesToolCallBuilder{}
	createdSent := false

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			logger.LogError(c, "failed to unmarshal chat stream chunk: "+err.Error())
			sr.Error(err)
			return
		}
		if chunk.Id != "" {
			responseID = chunk.Id
		}
		if chunk.Created > 0 {
			createdAt = chunk.Created
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if !createdSent {
			createdSent = true
			sendResponsesEvent(c, dto.ResponsesStreamResponse{
				Type:     "response.created",
				Response: buildChatAsResponsesResponse(responseID, createdAt, model, nil, nil),
			})
		}
		if chunk.Usage != nil {
			usage = normalizeResponsesUsage(chunk.Usage)
		}
		for _, choice := range chunk.Choices {
			if delta := choice.Delta.GetContentString(); delta != "" {
				textBuilder.WriteString(delta)
				sendResponsesEvent(c, dto.ResponsesStreamResponse{
					Type:  "response.output_text.delta",
					Delta: delta,
				})
			}
			for callIndex, toolCall := range choice.Delta.ToolCalls {
				if toolCall.Index != nil {
					callIndex = *toolCall.Index
				}
				builder := toolBuilders[callIndex]
				if builder == nil {
					builder = &responsesToolCallBuilder{}
					toolBuilders[callIndex] = builder
				}
				if toolCall.ID != "" {
					builder.ID = toolCall.ID
				}
				if toolCall.Function.Name != "" {
					builder.Name = toolCall.Function.Name
				}
				if toolCall.Function.Arguments != "" {
					builder.Arguments.WriteString(toolCall.Function.Arguments)
				}
			}
		}
	})

	if usage == nil {
		usage = &dto.Usage{}
	}
	if usage.CompletionTokens == 0 && textBuilder.Len() > 0 {
		usage.CompletionTokens = service.CountTextToken(textBuilder.String(), model)
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	usage = normalizeResponsesUsage(usage)

	output := textResponsesOutput(textBuilder.String())
	toolCalls := make([]dto.ToolCallResponse, 0, len(toolBuilders))
	toolIndexes := make([]int, 0, len(toolBuilders))
	for index := range toolBuilders {
		toolIndexes = append(toolIndexes, index)
	}
	sort.Ints(toolIndexes)
	for _, index := range toolIndexes {
		builder := toolBuilders[index]
		toolCalls = append(toolCalls, dto.ToolCallResponse{
			ID:   builder.ID,
			Type: "function",
			Function: dto.FunctionResponse{
				Name:      builder.Name,
				Arguments: builder.Arguments.String(),
			},
		})
	}
	output = append(output, toolResponsesOutput(toolCalls)...)

	sendResponsesEvent(c, dto.ResponsesStreamResponse{
		Type:     "response.completed",
		Response: buildChatAsResponsesResponse(responseID, createdAt, model, output, usage),
	})
	return usage, nil
}
