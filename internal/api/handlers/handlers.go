package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"awghop/internal/amnezia"
	"awghop/internal/api/middleware"
	"awghop/internal/api/respond"
	"awghop/internal/config"
	"awghop/internal/domain"
	"awghop/internal/ipalloc"
	"awghop/internal/netctl"
	"awghop/internal/store"
	"awghop/internal/wgk"
	"awghop/internal/wgquick"
)

type Handlers struct {
	Store *store.Store
	Cfg   config.Config
	Net   *netctl.Controller
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) SetupStatus(w http.ResponseWriter, r *http.Request) {
	done, err := h.Store.SetupComplete(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, domain.SetupStatus{SetupComplete: done})
}

type bootstrapBody struct {
	Password string              `json:"password"`
	Ingress  domain.IngressSettings `json:"ingress"`
}

func (h *Handlers) Bootstrap(w http.ResponseWriter, r *http.Request) {
	done, err := h.Store.SetupComplete(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if done {
		respond.Error(w, http.StatusConflict, "already_setup", "bootstrap already completed")
		return
	}
	var body bootstrapBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if len(body.Password) < 8 {
		respond.Error(w, http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
		return
	}
	in := body.Ingress
	if in.ListenPort == 0 {
		in.ListenPort = 51820
	}
	if in.TunnelSubnet == "" {
		in.TunnelSubnet = "10.8.0.0/24"
	}
	if in.DNSServers == "" {
		in.DNSServers = "1.1.1.1"
	}
	if in.MTU == 0 {
		in.MTU = 1420
	}
	if in.InterfaceName == "" {
		in.InterfaceName = "awg0"
	}
	if in.ServerTunnelIP == "" {
		in.ServerTunnelIP = "10.8.0.1"
	}
	if in.Jmax == 0 {
		in.Jmax = 1000
	}
	if in.Jmin == 0 {
		in.Jmin = 50
	}
	if in.Jc == 0 {
		in.Jc = 4
	}

	srvPriv, srvPub, err := wgk.GenerateKeyPair()
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "keygen", err.Error())
		return
	}
	in.ServerPrivateKey = srvPriv
	in.ServerPublicKey = srvPub

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "hash", err.Error())
		return
	}
	if err := h.Store.Bootstrap(r.Context(), string(hash), in, srvPriv, srvPub); err != nil {
		respond.Error(w, http.StatusInternalServerError, "bootstrap_failed", err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

type loginBody struct {
	Password string `json:"password"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	done, err := h.Store.SetupComplete(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if !done {
		respond.Error(w, http.StatusServiceUnavailable, "setup_required", "complete bootstrap first")
		return
	}
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	hash, err := h.Store.GetPasswordHash(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil {
		respond.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid password")
		return
	}
	tok := make([]byte, 32)
	if _, err := rand.Read(tok); err != nil {
		respond.Error(w, http.StatusInternalServerError, "random", err.Error())
		return
	}
	token := hex.EncodeToString(tok)
	exp := time.Now().UTC().Add(7 * 24 * time.Hour)
	if err := h.Store.CreateSession(r.Context(), token, exp); err != nil {
		respond.Error(w, http.StatusInternalServerError, "session", err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "awghop_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.Cfg.SecureCookies,
		Expires:  exp,
	})
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if tok := middleware.SessionToken(r); tok != "" {
		_ = h.Store.DeleteSession(r.Context(), tok)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "awghop_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) GetIngress(w http.ResponseWriter, r *http.Request) {
	in, err := h.Store.GetIngressSettings(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	in.ServerPrivateKey = ""
	respond.JSON(w, http.StatusOK, in)
}

func validateIngressSettings(in domain.IngressSettings) error {
	if err := wgquick.ValidateInterfaceName(in.InterfaceName); err != nil {
		return err
	}
	if in.ListenPort < 1 || in.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be 1..65535, got %d", in.ListenPort)
	}
	if in.MTU < 1280 || in.MTU > 9000 {
		return fmt.Errorf("mtu must be 1280..9000, got %d", in.MTU)
	}
	if _, err := netip.ParsePrefix(strings.TrimSpace(in.TunnelSubnet)); err != nil {
		return fmt.Errorf("invalid tunnel_subnet: %w", err)
	}
	if a := strings.TrimSpace(in.ServerTunnelIP); a != "" {
		if _, err := netip.ParseAddr(strings.Split(a, "/")[0]); err != nil {
			return fmt.Errorf("invalid server_tunnel_ip: %w", err)
		}
	}
	if in.Jmin < 0 || in.Jmax < in.Jmin || in.Jc < 0 || in.Jc > 32 {
		return fmt.Errorf("invalid jc/jmin/jmax: jc=%d jmin=%d jmax=%d", in.Jc, in.Jmin, in.Jmax)
	}
	if v := strings.TrimSpace(in.HostEndpoint); v != "" {
		if _, _, err := net.SplitHostPort(v); err != nil {
			return fmt.Errorf("invalid host_endpoint (want host:port): %w", err)
		}
	}
	if v := strings.TrimSpace(in.DNSServers); v != "" {
		for _, p := range strings.Split(v, ",") {
			if _, err := netip.ParseAddr(strings.TrimSpace(p)); err != nil {
				return fmt.Errorf("invalid dns server %q: %w", p, err)
			}
		}
	}
	return nil
}

func (h *Handlers) PutIngress(w http.ResponseWriter, r *http.Request) {
	var body domain.IngressSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	cur, err := h.Store.GetIngressSettings(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	if body.ServerPrivateKey == "" {
		body.ServerPrivateKey = cur.ServerPrivateKey
	}
	if body.ServerPublicKey == "" {
		body.ServerPublicKey = cur.ServerPublicKey
	}
	if err := validateIngressSettings(body); err != nil {
		respond.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	if err := h.Store.UpdateIngressSettings(r.Context(), body); err != nil {
		respond.Error(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	out := body
	out.ServerPrivateKey = ""
	respond.JSON(w, http.StatusOK, out)
}

// peerView сериализует пира вместе с runtime-статусом, если он доступен.
type peerView struct {
	domain.Peer
	Status *domain.PeerStatus `json:"status,omitempty"`
}

func (h *Handlers) ListPeers(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.ListPeers(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	statusMap, _ := h.peerStatusMap(r.Context())

	out := make([]peerView, 0, len(list))
	for _, p := range list {
		p.PrivateKey = ""
		v := peerView{Peer: p}
		if s, ok := statusMap[strings.TrimSpace(p.PublicKey)]; ok {
			s := s
			v.Status = &s
		}
		out = append(out, v)
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *Handlers) peerStatusMap(ctx context.Context) (map[string]domain.PeerStatus, error) {
	if h.Net == nil {
		return nil, nil
	}
	in, err := h.Store.GetIngressSettings(ctx)
	if err != nil {
		return nil, err
	}
	return h.Net.IngressStatus(ctx, in.InterfaceName)
}

type createPeerBody struct {
	Name             string `json:"name"`
	AllowedIPs       string `json:"allowed_ips"`
	PrivateKey       string `json:"private_key"`
	PublicKey        string `json:"public_key"`
	GeneratePSK      bool   `json:"generate_psk"`
	EgressType       string `json:"egress_type"`
	EgressTunnelID   *int64 `json:"egress_tunnel_id"`
}

func (h *Handlers) CreatePeer(w http.ResponseWriter, r *http.Request) {
	var body createPeerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		respond.Error(w, http.StatusBadRequest, "validation", "name is required")
		return
	}
	et := domain.EgressType(body.EgressType)
	if body.EgressType == "" {
		et = domain.EgressDirect
	}
	outTid, verr := h.resolveEgressSpec(r.Context(), et, body.EgressTunnelID)
	if verr != nil {
		respond.Error(w, http.StatusBadRequest, "validation", verr.Error())
		return
	}

	in, err := h.Store.GetIngressSettings(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	var priv, pub string
	if strings.TrimSpace(body.PrivateKey) != "" {
		if err := wgk.ValidateKey(body.PrivateKey); err != nil {
			respond.Error(w, http.StatusBadRequest, "invalid_key", err.Error())
			return
		}
		priv = body.PrivateKey
		if strings.TrimSpace(body.PublicKey) != "" {
			if err := wgk.ValidateKey(body.PublicKey); err != nil {
				respond.Error(w, http.StatusBadRequest, "invalid_key", err.Error())
				return
			}
			pub = body.PublicKey
		} else {
			pub = wgk.MustPublicFromPrivate(priv)
		}
	} else {
		priv, pub, err = wgk.GenerateKeyPair()
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "keygen", err.Error())
			return
		}
	}

	used, err := h.Store.ListPeerAllowedIPs(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	allowed := strings.TrimSpace(body.AllowedIPs)
	if allowed == "" {
		prefix, err := ipalloc.NextPeerPrefix(in.TunnelSubnet, in.ServerTunnelIP, used)
		if err != nil {
			respond.Error(w, http.StatusBadRequest, "allocation", err.Error())
			return
		}
		allowed = prefix.String()
	}

	var psk *string
	if body.GeneratePSK {
		s, err := wgk.GeneratePresharedKey()
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "psk", err.Error())
			return
		}
		psk = &s
	}

	p := domain.Peer{
		Name:           body.Name,
		PrivateKey:     priv,
		PublicKey:      pub,
		PresharedKey:   psk,
		AllowedIPs:     allowed,
		Enabled:        true,
		EgressType:     et,
		EgressTunnelID: outTid,
	}
	id, err := h.Store.InsertPeer(r.Context(), p)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "insert", err.Error())
		return
	}
	out, err := h.Store.GetPeer(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "reload", err.Error())
		return
	}
	out.PrivateKey = ""
	respond.JSON(w, http.StatusCreated, out)
}

func (h *Handlers) GetPeer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	p, err := h.Store.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	p.PrivateKey = ""
	respond.JSON(w, http.StatusOK, p)
}

type patchPeerBody struct {
	Name           *string `json:"name"`
	Enabled        *bool   `json:"enabled"`
	AllowedIPs     *string `json:"allowed_ips"`
	EgressType     *string `json:"egress_type"`
	EgressTunnelID *int64  `json:"egress_tunnel_id"`
}

func (h *Handlers) PatchPeer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	p, err := h.Store.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	var body patchPeerBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.Name != nil {
		p.Name = strings.TrimSpace(*body.Name)
	}
	if body.Enabled != nil {
		p.Enabled = *body.Enabled
	}
	if body.AllowedIPs != nil {
		p.AllowedIPs = *body.AllowedIPs
	}
	et := p.EgressType
	if body.EgressType != nil {
		et = domain.EgressType(*body.EgressType)
	}
	tid := p.EgressTunnelID
	if body.EgressTunnelID != nil {
		tid = body.EgressTunnelID
	}
	if et == domain.EgressDirect {
		tid = nil
	}
	outTid, verr := h.resolveEgressSpec(r.Context(), et, tid)
	if verr != nil {
		respond.Error(w, http.StatusBadRequest, "validation", verr.Error())
		return
	}
	p.EgressType = et
	p.EgressTunnelID = outTid
	if err := h.Store.UpdatePeer(r.Context(), p); err != nil {
		respond.Error(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	p.PrivateKey = ""
	respond.JSON(w, http.StatusOK, p)
}

func (h *Handlers) DeletePeer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	if err := h.Store.DeletePeer(r.Context(), id); err != nil {
		respond.Error(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) buildClientConfig(ctx context.Context, p domain.Peer) (string, error) {
	in, err := h.Store.GetIngressSettings(ctx)
	if err != nil {
		return "", err
	}
	sys, err := h.Store.GetSystemSettings(ctx)
	if err != nil {
		return "", err
	}
	return amnezia.BuildClientConf(p, in, sys), nil
}

func (h *Handlers) PeerConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	p, err := h.Store.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	conf, err := h.buildClientConfig(r.Context(), p)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="awghop-peer-`+strconv.FormatInt(id, 10)+`.conf"`)
	_, _ = w.Write([]byte(conf))
}

func (h *Handlers) PeerQR(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	p, err := h.Store.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	conf, err := h.buildClientConfig(r.Context(), p)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	png, err := qrcode.Encode(conf, qrcode.Medium, 256)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "qr", err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func (h *Handlers) EnablePeer(w http.ResponseWriter, r *http.Request) {
	h.setPeerEnabled(w, r, true)
}

func (h *Handlers) DisablePeer(w http.ResponseWriter, r *http.Request) {
	h.setPeerEnabled(w, r, false)
}

func (h *Handlers) setPeerEnabled(w http.ResponseWriter, r *http.Request, en bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	p, err := h.Store.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	p.Enabled = en
	if err := h.Store.UpdatePeer(r.Context(), p); err != nil {
		respond.Error(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	p.PrivateKey = ""
	respond.JSON(w, http.StatusOK, p)
}
