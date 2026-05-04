package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"

	"awghop/internal/api/respond"
)

const (
	CSRFCookieName = "awghop_csrf"
	CSRFHeaderName = "X-CSRF-Token"
)

// EnsureCSRFCookie ставит httpОnly=false cookie со случайным токеном, если её ещё нет.
// SPA читает её через document.cookie и шлёт в заголовке X-CSRF-Token.
func EnsureCSRFCookie(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c, err := r.Cookie(CSRFCookieName); err == nil && len(c.Value) >= 32 {
				next.ServeHTTP(w, r)
				return
			}
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				respond.Error(w, http.StatusInternalServerError, "csrf_init", err.Error())
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:     CSRFCookieName,
				Value:    hex.EncodeToString(b),
				Path:     "/",
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
				Secure:   secure,
			})
			next.ServeHTTP(w, r)
		})
	}
}

// CSRF проверяет совпадение cookie и заголовка X-CSRF-Token для state-changing методов.
func CSRF() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			c, err := r.Cookie(CSRFCookieName)
			if err != nil || c.Value == "" {
				respond.Error(w, http.StatusForbidden, "csrf_missing", "missing csrf cookie")
				return
			}
			h := r.Header.Get(CSRFHeaderName)
			if h == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(h)) != 1 {
				respond.Error(w, http.StatusForbidden, "csrf_mismatch", "invalid csrf token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
