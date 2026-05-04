package middleware

import (
	"context"
	"net/http"
	"strings"

	"awghop/internal/api/respond"
	"awghop/internal/store"
)

type ctxKey int

const sessionCtxKey ctxKey = 1

func SessionToken(r *http.Request) string {
	c, err := r.Cookie("awghop_session")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

func Auth(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := SessionToken(r)
			if tok == "" {
				respond.Error(w, http.StatusUnauthorized, "unauthorized", "missing session")
				return
			}
			ok, err := st.SessionValid(r.Context(), tok)
			if err != nil {
				respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
				return
			}
			if !ok {
				respond.Error(w, http.StatusUnauthorized, "unauthorized", "invalid session")
				return
			}
			ctx := context.WithValue(r.Context(), sessionCtxKey, tok)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CORS поддерживает явный allow-list origins (production-friendly) и dev-режим.
// В dev-режиме allow-list игнорируется, любой Origin отражается обратно.
func CORS(dev bool, allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				switch {
				case dev:
					w.Header().Set("Access-Control-Allow-Origin", origin)
				default:
					if _, ok := allowed[origin]; ok {
						w.Header().Set("Access-Control-Allow-Origin", origin)
					}
				}
			} else if dev {
				w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
