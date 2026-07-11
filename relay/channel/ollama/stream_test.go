package ollama

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestOllamaToolCallsToOpenAI(t *testing.T) {
	call := OllamaToolCall{}
	call.Function.Name = "lookup"
	call.Function.Arguments = map[string]any{"city": "Shanghai"}
	converted, next := ollamaToolCallsToOpenAI([]OllamaToolCall{call}, 2, true)
	if next != 3 || len(converted) != 1 {
		t.Fatalf("unexpected conversion length=%d next=%d", len(converted), next)
	}
	if converted[0].ID != "call_2" || converted[0].Function.Name != "lookup" {
		t.Fatalf("unexpected tool call: %+v", converted[0])
	}
	var args map[string]any
	if err := common.Unmarshal([]byte(converted[0].Function.Arguments), &args); err != nil {
		t.Fatal(err)
	}
	if args["city"] != "Shanghai" {
		t.Fatalf("unexpected arguments: %+v", args)
	}
}

func TestOllamaToolCallsToOpenAIDefaultsInvalidArguments(t *testing.T) {
	call := OllamaToolCall{}
	call.Function.Name = "empty"
	call.Function.Arguments = make(chan int)
	converted, _ := ollamaToolCallsToOpenAI([]OllamaToolCall{call}, 0, false)
	if len(converted) != 1 || converted[0].Function.Arguments != "{}" {
		t.Fatalf("unexpected arguments: %+v", converted)
	}
}
