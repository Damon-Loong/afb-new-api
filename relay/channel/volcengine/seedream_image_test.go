package volcengine

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
)

func TestNormalizeSeedreamImageRequestMapsNToSequential(t *testing.T) {
	n := uint(4)
	got := normalizeSeedreamImageRequest(dto.ImageRequest{
		Model:  "doubao-seedream-4-5-251128",
		Prompt: "cats",
		N:      &n,
	})

	if got.SequentialImageGeneration != "auto" {
		t.Fatalf("expected auto, got %q", got.SequentialImageGeneration)
	}
	if got.SequentialImageGenerationOptions == nil || got.SequentialImageGenerationOptions.MaxImages != 4 {
		t.Fatalf("expected max_images=4, got %+v", got.SequentialImageGenerationOptions)
	}
}

func TestNormalizeSeedreamImageRequestDisabledForSingle(t *testing.T) {
	got := normalizeSeedreamImageRequest(dto.ImageRequest{
		Model:                         "doubao-seedream-4-5-251128",
		Prompt:                        "cat",
		SequentialImageGeneration:     "disabled",
	})

	if got.SequentialImageGeneration != "disabled" {
		t.Fatalf("expected disabled, got %q", got.SequentialImageGeneration)
	}
	if got.SequentialImageGenerationOptions != nil {
		t.Fatalf("expected nil options, got %+v", got.SequentialImageGenerationOptions)
	}
}
