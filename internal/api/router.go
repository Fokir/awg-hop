package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"awghop/internal/api/handlers"
	apmw "awghop/internal/api/middleware"
	"awghop/internal/config"
	"awghop/internal/netctl"
	"awghop/internal/store"
)

// NewRouter собирает HTTP-роутер AWG Hop. Все state-changing эндпоинты под /api/v1
// защищены сессией (кроме health/setup/login) и CSRF (двойная cookie).
func NewRouter(st *store.Store, cfg config.Config, ui fs.FS, nc *netctl.Controller) http.Handler {
	h := &handlers.Handlers{Store: st, Cfg: cfg, Net: nc}
	loginLimiter := apmw.NewLoginRateLimiter()

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(apmw.CORS(cfg.DevCORS, cfg.AllowedOrigins))
	r.Use(apmw.EnsureCSRFCookie(cfg.SecureCookies))

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", h.Health)
		r.Get("/setup/status", h.SetupStatus)

		// Bootstrap и login требуют CSRF и rate-limit.
		r.Group(func(r chi.Router) {
			r.Use(apmw.CSRF())
			r.With(loginLimiter.Middleware).Post("/setup/bootstrap", h.Bootstrap)
			r.With(loginLimiter.Middleware).Post("/auth/login", h.Login)
			r.Post("/auth/logout", h.Logout)
		})

		// Авторизованные state-changing — CSRF включён.
		r.Group(func(r chi.Router) {
			r.Use(apmw.Auth(st))
			r.Use(apmw.CSRF())

			r.Get("/settings/ingress", h.GetIngress)
			r.Put("/settings/ingress", h.PutIngress)

			r.Get("/settings/system", h.GetSystemSettings)
			r.Put("/settings/system", h.PutSystemSettings)

			r.Get("/peers", h.ListPeers)
			r.Post("/peers", h.CreatePeer)
			r.Get("/peers/{id}", h.GetPeer)
			r.Patch("/peers/{id}", h.PatchPeer)
			r.Delete("/peers/{id}", h.DeletePeer)
			r.Get("/peers/{id}/config", h.PeerConfig)
			r.Get("/peers/{id}/qrcode", h.PeerQR)
			r.Post("/peers/{id}/enable", h.EnablePeer)
			r.Post("/peers/{id}/disable", h.DisablePeer)

			r.Get("/egress-tunnels", h.ListEgressTunnels)
			r.Post("/egress-tunnels", h.CreateEgressTunnel)
			r.Get("/egress-tunnels/{id}", h.GetEgressTunnel)
			r.Patch("/egress-tunnels/{id}", h.PatchEgressTunnel)
			r.Delete("/egress-tunnels/{id}", h.DeleteEgressTunnel)

			r.Post("/system/apply", h.SystemApply)
			r.Get("/system/status", h.SystemStatus)

			r.Get("/backup/export", h.BackupExport)
			r.Post("/backup/import", h.BackupImport)

			r.Post("/setup/wg-easy-import", h.WgEasyImport)
		})
	})

	fileServer := http.FileServer(http.FS(ui))
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		upath := strings.TrimPrefix(req.URL.Path, "/")
		if upath != "" {
			if _, err := fs.Stat(ui, upath); err != nil {
				req2 := req.Clone(req.Context())
				req2.URL.Path = "/index.html"
				fileServer.ServeHTTP(w, req2)
				return
			}
		}
		fileServer.ServeHTTP(w, req)
	})

	return r
}
