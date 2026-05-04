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

// egressView расширяет туннель runtime-статусом.
type egressView struct {
	domain.EgressTunnel
	Status *domain.EgressTunnelStatus `json:"status,omitempty"`
}

func (h *Handlers) resolveEgressSpec(ctx context.Context, et domain.EgressType, tunnelID *int64) (*int64, error) {
	switch et {
	case domain.EgressDirect:
		return nil, nil
	case domain.EgressViaTunnel:
		if tunnelID == nil {
			return nil, errors.New("egress_tunnel_id is required for egress_awg")
		}
		t, err := h.Store.GetEgressTunnel(ctx, *tunnelID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("egress tunnel not found")
		}
		if err != nil {
			return nil, err
		}
		if !t.Enabled {
			return nil, errors.New("egress tunnel is disabled")
		}
		return tunnelID, nil
	default:
		return nil, errors.New("invalid egress_type")
	}
}

func (h *Handlers) ListEgressTunnels(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.ListEgressTunnels(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	out := make([]egressView, 0, len(list))
	for _, t := range list {
		v := egressView{EgressTunnel: t}
		if h.Net != nil && t.Enabled {
			st := h.Net.EgressStatus(r.Context(), t.InterfaceName)
			v.Status = &st
		}
		out = append(out, v)
	}
	respond.JSON(w, http.StatusOK, out)
}

type createEgressBody struct {
	Name          string `json:"name"`
	InterfaceName string `json:"interface_name"`
	ConfigText    string `json:"config_text"`
	Enabled       *bool  `json:"enabled"`
}

func (h *Handlers) CreateEgressTunnel(w http.ResponseWriter, r *http.Request) {
	var body createEgressBody
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
	t := domain.EgressTunnel{Name: body.Name, InterfaceName: body.InterfaceName, ConfigText: body.ConfigText, Enabled: en}
	id, err := h.Store.InsertEgressTunnel(r.Context(), t)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "insert", err.Error())
		return
	}
	out, err := h.Store.GetEgressTunnel(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "reload", err.Error())
		return
	}
	respond.JSON(w, http.StatusCreated, out)
}

func (h *Handlers) GetEgressTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	t, err := h.Store.GetEgressTunnel(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	v := egressView{EgressTunnel: t}
	if h.Net != nil && t.Enabled {
		st := h.Net.EgressStatus(r.Context(), t.InterfaceName)
		v.Status = &st
	}
	respond.JSON(w, http.StatusOK, v)
}

type patchEgressBody struct {
	Name          *string `json:"name"`
	InterfaceName *string `json:"interface_name"`
	ConfigText    *string `json:"config_text"`
	Enabled       *bool   `json:"enabled"`
}

func (h *Handlers) PatchEgressTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	t, err := h.Store.GetEgressTunnel(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.Error(w, http.StatusNotFound, "not_found", "")
			return
		}
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	var body patchEgressBody
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
		n, err := h.Store.CountPeersOnEgressTunnel(r.Context(), id)
		if err != nil {
			respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		if n > 0 {
			respond.Error(w, http.StatusConflict, "in_use", "cannot disable tunnel while peers reference it")
			return
		}
	}
	if err := h.Store.UpdateEgressTunnel(r.Context(), t); err != nil {
		respond.Error(w, http.StatusInternalServerError, "update", err.Error())
		return
	}
	out, err := h.Store.GetEgressTunnel(r.Context(), id)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "reload", err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *Handlers) DeleteEgressTunnel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_id", "")
		return
	}
	err = h.Store.DeleteEgressTunnel(r.Context(), id)
	if errors.Is(err, store.ErrEgressInUse) {
		respond.Error(w, http.StatusConflict, "in_use", "tunnel is referenced by peers; reassign peers first")
		return
	}
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "delete", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
