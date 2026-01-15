package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	aiplatform "cloud.google.com/go/aiplatform/apiv1"
	"cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/structpb"

	"k2MarketingAi/internal/media"
)

// ImagenClient renders images using Vertex AI Imagen.
type ImagenClient interface {
	Edit(ctx context.Context, payload ImagenPayload) (ImageResult, error)
}

// ImagenPayload describes prompts and reference imagery.
type ImagenPayload struct {
	Prompt        string
	BaseImageURL  string
	BaseImageData string
}

// VertexImagen implements ImagenClient via the Vertex AI SDK.
type VertexImagen struct {
	projectID          string
	location           string
	model              string
	apiKey             string
	serviceAccount     string
	serviceAccountJSON string
	uploader           mediaUploader
}

// VertexImagenConfig describes how to connect to Imagen.
type VertexImagenConfig struct {
	ProjectID          string
	Location           string
	Model              string
	APIKey             string
	ServiceAccount     string
	ServiceAccountJSON string
}

// NewVertexImagen wires a VertexImagen client.
func NewVertexImagen(cfg VertexImagenConfig, uploader media.Uploader) *VertexImagen {
	return &VertexImagen{
		projectID:          strings.TrimSpace(cfg.ProjectID),
		location:           strings.TrimSpace(cfg.Location),
		model:              strings.TrimSpace(cfg.Model),
		apiKey:             strings.TrimSpace(cfg.APIKey),
		serviceAccount:     strings.TrimSpace(cfg.ServiceAccount),
		serviceAccountJSON: strings.TrimSpace(cfg.ServiceAccountJSON),
		uploader:           uploader,
	}
}

type mediaUploader interface {
	Upload(ctx context.Context, input media.UploadInput) (media.UploadResult, error)
}

// Edit runs an Imagen edit request and uploads the result.
func (v *VertexImagen) Edit(ctx context.Context, payload ImagenPayload) (ImageResult, error) {
	if v == nil || v.uploader == nil {
		return ImageResult{}, fmt.Errorf("imagen: client not configured")
	}
	if v.projectID == "" || v.location == "" || v.model == "" {
		return ImageResult{}, fmt.Errorf("imagen: missing project/location/model")
	}
	if strings.TrimSpace(payload.Prompt) == "" {
		return ImageResult{}, fmt.Errorf("imagen: prompt is required")
	}

	encoded, err := v.prepareBaseImage(ctx, payload)
	if err != nil {
		return ImageResult{}, err
	}

	instance, err := structpb.NewValue(map[string]any{
		"prompt": payload.Prompt,
		"image": map[string]any{
			"bytesBase64Encoded": encoded,
		},
	})
	if err != nil {
		return ImageResult{}, err
	}

	params, err := structpb.NewValue(map[string]any{
		"sampleCount": 1,
		"editMode":    "inpainting-free-form",
	})
	if err != nil {
		return ImageResult{}, err
	}

	endpoint := fmt.Sprintf("projects/%s/locations/%s/publishers/google/models/%s", v.projectID, v.location, v.model)
	options := []option.ClientOption{option.WithEndpoint(fmt.Sprintf("%s-aiplatform.googleapis.com:443", v.location))}
	if v.serviceAccountJSON != "" {
		options = append(options, option.WithCredentialsJSON([]byte(v.serviceAccountJSON)))
	} else if v.serviceAccount != "" {
		options = append(options, option.WithCredentialsFile(v.serviceAccount))
	} else if v.apiKey != "" {
		options = append(options, option.WithAPIKey(v.apiKey))
	}

	client, err := aiplatform.NewPredictionClient(ctx, options...)
	if err != nil {
		return ImageResult{}, fmt.Errorf("imagen: prediction client: %w", err)
	}
	defer client.Close()

	resp, err := client.Predict(ctx, &aiplatformpb.PredictRequest{
		Endpoint:   endpoint,
		Instances:  []*structpb.Value{instance},
		Parameters: params,
	})
	if err != nil {
		return ImageResult{}, fmt.Errorf("imagen: predict: %w", err)
	}
	if len(resp.Predictions) == 0 {
		return ImageResult{}, fmt.Errorf("imagen: empty prediction response")
	}

	field := resp.Predictions[0].GetStructValue().GetFields()["bytesBase64Encoded"]
	if field == nil {
		return ImageResult{}, fmt.Errorf("imagen: prediction missing bytes")
	}

	data, err := base64.StdEncoding.DecodeString(field.GetStringValue())
	if err != nil {
		return ImageResult{}, fmt.Errorf("imagen: decode result: %w", err)
	}

	result, err := v.uploader.Upload(ctx, media.UploadInput{
		Filename:    "imagen-render.png",
		ContentType: "image/png",
		Body:        bytes.NewReader(data),
		Size:        int64(len(data)),
	})
	if err != nil {
		return ImageResult{}, fmt.Errorf("imagen: upload render: %w", err)
	}
	return ImageResult{URL: result.URL, Key: result.Key}, nil
}

func (v *VertexImagen) prepareBaseImage(ctx context.Context, payload ImagenPayload) (string, error) {
	if trimmed := strings.TrimSpace(payload.BaseImageData); trimmed != "" {
		return stripDataPrefix(trimmed)
	}
	if strings.TrimSpace(payload.BaseImageURL) == "" {
		return "", fmt.Errorf("imagen: reference image is required")
	}
	imageBytes, err := fetchImage(ctx, payload.BaseImageURL)
	if err != nil {
		return "", fmt.Errorf("imagen: fetch base image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(imageBytes), nil
}

func stripDataPrefix(raw string) (string, error) {
	if !strings.HasPrefix(raw, "data:") {
		return raw, nil
	}
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data URL")
	}
	return parts[1], nil
}

func fetchImage(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
