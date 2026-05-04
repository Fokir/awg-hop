package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"awghop/internal/api/respond"
	"awghop/internal/domain"
	"awghop/internal/wgeasy"
	"awghop/internal/wgk"
)

// WgEasyImport — POST /setup/wg-easy-import
//
// Принимает multipart `file` (wg-easy v15+ JSON c AmneziaWG-блоком) или
// JSON в теле запроса. Если AmneziaWG-блок отсутствует, импорт отклоняется
// согласно спецификации §5.5.
func (h *Handlers) WgEasyImport(w http.ResponseWriter, r *http.Request) {
	src, cleanup, err := openWGEasyBody(r)
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_body", err.Error())
		return
	}
	defer cleanup()

	res, err := wgeasy.Parse(src)
	if err != nil {
		if errors.Is(err, wgeasy.ErrUnsupportedFormat) {
			respond.Error(w, http.StatusUnprocessableEntity, "unsupported_format",
				"wg-easy export must include AmneziaWG block; vanilla WireGuard import is not supported in v0.2")
			return
		}
		respond.Error(w, http.StatusBadRequest, "parse", err.Error())
		return
	}

	in := res.Ingress
	if in.InterfaceName == "" {
		in.InterfaceName = "awg0"
	}
	if in.MTU == 0 {
		in.MTU = 1420
	}
	if in.Jc == 0 {
		in.Jc = 4
	}
	if in.Jmin == 0 {
		in.Jmin = 50
	}
	if in.Jmax == 0 {
		in.Jmax = 1000
	}

	if err := validateIngressSettings(in); err != nil {
		respond.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}

	if err := h.applyImport(r.Context(), in, res.Clients); err != nil {
		respond.Error(w, http.StatusInternalServerError, "import_failed", err.Error())
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"clients": len(res.Clients),
	})
}

func (h *Handlers) applyImport(ctx context.Context, in domain.IngressSettings, clients []domain.Client) error {
	if err := h.Store.UpdateIngressSettings(ctx, in); err != nil {
		return err
	}
	for _, c := range clients {
		if c.PrivateKey == "" {
			continue
		}
		if c.PublicKey == "" {
			c.PublicKey = wgk.MustPublicFromPrivate(c.PrivateKey)
		}
		if _, err := h.Store.InsertClient(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func openWGEasyBody(r *http.Request) (io.Reader, func(), error) {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			return nil, func() {}, err
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, func() {}, err
		}
		return f, func() { _ = f.Close() }, nil
	}
	return r.Body, func() {}, nil
}
