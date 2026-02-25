package auth

import (
	"encoding/json"
	"net/http"

	"k2MarketingAi/internal/storage"
)

// UsageLimiter is middleware that checks whether the user still has free-tier
// quota or an active subscription before allowing expensive AI operations.
type UsageLimiter struct {
	Store storage.Store
}

// RequirePaidOrQuota blocks the request with 402 when the free tier is exhausted
// and no active subscription exists.  It also increments the usage counter on
// every successful pass-through.
func (ul UsageLimiter) RequirePaidOrQuota(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "inloggning krävs", http.StatusUnauthorized)
			return
		}

		if !user.CanUseAI() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error":       "usage_limit_reached",
				"message":     "Du har använt alla dina kostnadsfria anrop. Uppgradera till en prenumeration för att fortsätta.",
				"usage_count": user.UsageCount,
				"usage_limit": storage.FreeUsageLimit,
			})
			return
		}

		// Increment usage (fire-and-forget; don't block on errors).
		newCount, _ := ul.Store.IncrementUsage(r.Context(), user.ID)

		// Update the user in context with the new count so downstream
		// handlers can read the latest value.
		user.UsageCount = newCount
		r = r.WithContext(WithUser(r.Context(), user))

		next.ServeHTTP(w, r)
	})
}
