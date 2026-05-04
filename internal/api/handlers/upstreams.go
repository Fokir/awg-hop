package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"awghop/internal/api/respond"
	"awghop/internal/domain"
	"awghop/internal/store"
	"awghop/internal/wgquick"
)

// upstreamView расширяет описание upstream-туннеля runtime-статусом.
type upstreamView struct {
	domain.UpstreamTunnel
	Status *domain.UpstreamStatus `json:"status,omitempty"`
}

// resolveUpstreamSpec проверяет, что комбинация upstream_type/upstream_tunnel_id валидна.
func (h *Handlers) resolveUpstreamSpec(ctx context.Context, ut domain.UpstreamType, tunnelID *int64) (*int64, error) {
	switch ut {
	case domain.UpstreamDirect:
		return nil, nil
	case domain.UpstreamViaTunnel:
		if tunnelID == nil {
			return nil, errors.New("upstream_tunnel_id is required for via_upstream")
		}
		t, err := h.Store.GetUpstreamTunnel(ctx, *tunnelID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("upstream tunnel not found")
		}
		if err != nil {
			return nil, err
		}
		if !t.Enabled {
			return nil, errors.New("upstream tunnel is disabled")
		}
		return tunnelID, nil
	default:
		return nil, errors.New("invalid upstream_type")
	}
}

func (h *Handlers) ListUpstreams(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.ListUpstreamTunnels(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	out := make([]upstreamView, 0, len(list))
	for _, t := range list {
		v := upstreamView{UpstreamTunnel: t}
		if h.Net != nil && t.Enabled {
			st := h.Net.UpstreamStatus(r.Context(), t.InterfaceName)
			v.Status = &st
		}
		out = append(out, v)
	}
	respond.JSON(w, http.StatusOK, out)
}

type createUpstreamBody struct {
	Name          string `json:"name"`
	InterfaceName string `json:"interface_name"`
	ConfigText    string `json:"config_text"`
	Enabled       *bool  `json:"enabled"`
}

func (h *Handlers) CreateUpstream(w http.ResponseWriter, r *http.Request) {
	var body createUpstreamBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.InterfaceName = strings.TrimSpace(body.InterfaceName)
	if body.Name == "" || body.InterfaceName == "" {
		respond.Error(w, http.StatusBadRequest, "validation", "name and interface_name are required")
		return
	}
	if err := wgquick.ValidateInterfaceName(body.InterfaceName); err != nil {
		respond.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	en := true
	if body.Enabled != nil {
		en = *body.Enabled
	}
	t := domain.UpstreamTunnel{Name: body.Name, InterfaceName: body.InterfaceName, ConfigText: body.ConfigText, Enabled: en}
	id, err := h.Store.InsertUpstreamTunnel(r.Context(), t)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "insert", err.Error())
		return
	}
	out, err := h.Store.GetUpstreamTunnel(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "reload", err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, out)
}

func (h *Handlers) GetUpstream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	t, err := h.Store.GetUpstreamTunnel(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	v := upstreamView{UpstreamTunnel: t}
	if h.Net != nil && t.Enabled {
		st := h.Net.UpstreamStatus(r.Context(), t.InterfaceName)
		v.Status = &st
	}
	respond.JSON(w, http.StatusOK, v)
}

type patchUpstreamBody struct {
	Name          *string `json:"name"`
	InterfaceName *string `json:"interface_name"`
	ConfigText    *string `json:"config_text"`
	Enabled       *bool   `json:"enabled"`
}

func (h *Handlers) PatchUpstream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	t, err := h.Store.GetUpstreamTunnel(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	var body patchUpstreamBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	wasEnabled := t.Enabled
	if body.Name != nil {
		t.Name = strings.TrimSpace(*body.Name)
	}
	if body.InterfaceName != nil {
		t.InterfaceName = strings.TrimSpace(*body.InterfaceName)
		if err := wgquick.ValidateInterfaceName(t.InterfaceName); err != nil {
			respond.Error(w, http.StatusBadRequest, "validation", err.Error())
			return
		}
	}
	if body.ConfigText != nil {
		t.ConfigText = *body.ConfigText
	}
	if body.Enabled != nil {
		t.Enabled = *body.Enabled
	}
	if wasEnabled && !t.Enabled {
		n, err := h.Store.CountClientsOnUpstreamTunnel(r.Context(), id)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		if n > 0 {
			respond.Error(w, http.StatusConflict, "in_use", "cannot disable upstream while clients reference it")
			return
		}
	}
	if err := h.Store.UpdateUpstreamTunnel(r.Context(), t); err != nil {
		respond.Error(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	out, err := h.Store.GetUpstreamTunnel(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "reload", err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *Handlers) DeleteUpstream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	err = h.Store.DeleteUpstreamTunnel(r.Context(), id)
	if errors.Is(err, store.ErrUpstreamInUse) {
		respond.Error(w, http.StatusConflict, "in_use", "upstream is referenced by clients; reassign clients first")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
