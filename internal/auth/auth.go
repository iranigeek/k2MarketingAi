package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"k2MarketingAi/internal/storage"
)

// ErrInvalidCredentials is returned when email/password don't match.
var ErrInvalidCredentials = errors.New("invalid credentials")

type contextKey string

const userContextKey contextKey = "auth/user"

// SessionManager signs and validates lightweight session tokens.
type SessionManager struct {
	Secret       []byte
	Duration     time.Duration
	CookieName   string
	SecureCookie bool
}

// Claims captures decoded session data.
type Claims struct {
	UserID    string
	ExpiresAt time.Time
}

// Middleware attaches the authenticated user to the request context when a valid session cookie exists.
type Middleware struct {
	Store    storage.Store
	Sessions SessionManager
}

// Handler exposes auth endpoints for registering and logging in users.
type Handler struct {
	Store    storage.Store
	Sessions SessionManager
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// InjectUser parses the session cookie (if present) and loads the user into context.
func (m Middleware) InjectUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(m.Sessions.cookieName())
		if err == nil && cookie.Value != "" {
			if claims, err := m.Sessions.Parse(cookie.Value); err == nil && claims.ExpiresAt.After(time.Now()) {
				if user, err := m.Store.GetUserByID(r.Context(), claims.UserID); err == nil && user.Approved {
					r = r.WithContext(WithUser(r.Context(), user))
				}
			} else if err != nil {
				// Clear unusable cookies to avoid loops.
				clear := m.Sessions.expiredCookie()
				http.SetCookie(w, &clear)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth ensures a user exists in context or returns 401.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); !ok {
			http.Error(w, "inloggning kr\u00e4vs", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Register handles POST /api/auth/register.
func (h Handler) Register(w http.ResponseWriter, r *http.Request) {
	var payload authRequest
	if err := decodeJSON(r, &payload); err != nil {
		http.Error(w, "ogiltig beg\u00e4ran", http.StatusBadRequest)
		return
	}

	email := normalizeEmail(payload.Email)
	if email == "" || len(payload.Password) < 6 {
		http.Error(w, "e-post och l\u00f6senord kr\u00e4vs (minst 6 tecken)", http.StatusBadRequest)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "kunde inte skapa anv\u00e4ndare", http.StatusInternalServerError)
		return
	}

	user := storage.User{
		Email:        email,
		PasswordHash: string(hashed),
		CreatedAt:    time.Now(),
		Approved:     false,
	}
	created, err := h.Store.CreateUser(r.Context(), user)
	if err != nil {
		if errors.Is(err, storage.ErrUserExists) {
			http.Error(w, "e-postadress \u00e4r redan registrerad", http.StatusConflict)
			return
		}
		http.Error(w, "kunde inte spara anv\u00e4ndare", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = jsonResponse(w, http.StatusCreated, map[string]any{
		"id":         created.ID,
		"email":      created.Email,
		"created_at": created.CreatedAt,
		"approved":   created.Approved,
		"status":     "pending_approval",
	})
}

// Login handles POST /api/auth/login.
func (h Handler) Login(w http.ResponseWriter, r *http.Request) {
	var payload authRequest
	if err := decodeJSON(r, &payload); err != nil {
		http.Error(w, "ogiltig beg\u00e4ran", http.StatusBadRequest)
		return
	}

	email := normalizeEmail(payload.Email)
	if email == "" || payload.Password == "" {
		http.Error(w, "e-post och l\u00f6senord kr\u00e4vs", http.StatusBadRequest)
		return
	}

	user, err := h.Store.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Error(w, "felaktiga inloggningsuppgifter", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)); err != nil {
		http.Error(w, "felaktiga inloggningsuppgifter", http.StatusUnauthorized)
		return
	}
	if !user.Approved {
		http.Error(w, "kontot v\u00e4ntar p\u00e5 godk\u00e4nnande", http.StatusForbidden)
		return
	}

	h.setSessionCookie(w, user.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = jsonResponse(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"created_at": user.CreatedAt,
		"approved":   user.Approved,
	})
}

// Logout handles POST /api/auth/logout.
func (h Handler) Logout(w http.ResponseWriter, _ *http.Request) {
	cookie := h.Sessions.expiredCookie()
	http.SetCookie(w, &cookie)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the current user profile.
func (h Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		http.Error(w, "ingen aktiv session", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = jsonResponse(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"created_at": user.CreatedAt,
		"approved":   user.Approved,
	})
}

func (h Handler) notifyAdmins(user storage.User) {}

// Parse validates a token and returns session claims.
func (sm SessionManager) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, errors.New("invalid token format")
	}
	payload := parts[0]
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("decode signature: %w", err)
	}

	mac := hmac.New(sha256.New, sm.Secret)
	mac.Write([]byte(payload))
	if !hmac.Equal(mac.Sum(nil), sig) {
		return Claims{}, errors.New("signature mismatch")
	}

	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 2 {
		return Claims{}, errors.New("invalid payload")
	}
	userID := payloadParts[0]
	expUnix, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil {
		return Claims{}, fmt.Errorf("parse expiry: %w", err)
	}
	return Claims{UserID: userID, ExpiresAt: time.Unix(expUnix, 0)}, nil
}

// Issue builds a signed session token for the given user.
func (sm SessionManager) Issue(userID string) (string, time.Time, error) {
	if len(sm.Secret) == 0 {
		return "", time.Time{}, errors.New("session secret missing")
	}
	expires := time.Now().Add(sm.sessionDuration())
	payload := fmt.Sprintf("%s|%d", userID, expires.Unix())
	mac := hmac.New(sha256.New, sm.Secret)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	token := payload + "." + base64.RawURLEncoding.EncodeToString(sig)
	return token, expires, nil
}

// WithUser stores the authenticated user in context.
func WithUser(ctx context.Context, user storage.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext extracts the authenticated user from context if present.
func UserFromContext(ctx context.Context) (storage.User, bool) {
	user, ok := ctx.Value(userContextKey).(storage.User)
	return user, ok
}

func (h Handler) setSessionCookie(w http.ResponseWriter, userID string) {
	token, exp, err := h.Sessions.Issue(userID)
	if err != nil {
		http.Error(w, "kunde inte skapa session", http.StatusInternalServerError)
		return
	}
	cookie := h.Sessions.cookie(token, exp)
	http.SetCookie(w, &cookie)
}

func (sm SessionManager) cookie(token string, expires time.Time) http.Cookie {
	return http.Cookie{
		Name:     sm.cookieName(),
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   sm.SecureCookie,
	}
}

func (sm SessionManager) expiredCookie() http.Cookie {
	return http.Cookie{
		Name:     sm.cookieName(),
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   sm.SecureCookie,
	}
}

func (sm SessionManager) cookieName() string {
	if sm.CookieName != "" {
		return sm.CookieName
	}
	return "session_token"
}

func (sm SessionManager) sessionDuration() time.Duration {
	if sm.Duration <= 0 {
		return 7 * 24 * time.Hour
	}
	return sm.Duration
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func jsonResponse(w http.ResponseWriter, status int, payload any) error {
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(payload)
}
