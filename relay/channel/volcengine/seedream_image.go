package volcengine

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

const seedreamSequentialMaxImages = 15

func isSeedreamImageModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(model, "seedream") || strings.Contains(model, "doubao-seed")
}

// normalizeSeedreamImageRequest maps OpenAI-style `n` to Volcengine sequential_image_generation fields.
// Seedream ignores `n` for multi-image output; see:
// https://www.volcengine.com/docs/82379/1541523
func normalizeSeedreamImageRequest(request dto.ImageRequest) dto.ImageRequest {
	out := request
	mode := strings.TrimSpace(strings.ToLower(out.SequentialImageGeneration))

	if mode == "" {
		if out.N != nil && *out.N > 1 {
			mode = "auto"
		} else {
			mode = "disabled"
		}
	}

	out.SequentialImageGeneration = mode

	if mode != "auto" {
		out.SequentialImageGenerationOptions = nil
		return out
	}

	maxImages := 1
	if out.SequentialImageGenerationOptions != nil && out.SequentialImageGenerationOptions.MaxImages > 0 {
		maxImages = out.SequentialImageGenerationOptions.MaxImages
	} else if out.N != nil && *out.N > 0 {
		maxImages = int(*out.N)
	}
	if maxImages > seedreamSequentialMaxImages {
		maxImages = seedreamSequentialMaxImages
	}
	if maxImages < 1 {
		maxImages = 1
	}

	out.SequentialImageGenerationOptions = &dto.SequentialImageGenerationOptions{
		MaxImages: maxImages,
	}
	return out
}
