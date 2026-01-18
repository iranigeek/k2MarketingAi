package llm

import "context"

type contextKey string

const modelContextKey contextKey = "llm-model-override"

// WithModel returns a context carrying a preferred model override.
func WithModel(ctx context.Context, model string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	model = normalizeModel(model)
	if model == "" {
		return ctx
	}
	return context.WithValue(ctx, modelContextKey, model)
}

// modelFromContext extracts the requested model override, if any.
func modelFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(modelContextKey).(string); ok {
		return normalizeModel(value)
	}
	return ""
}
