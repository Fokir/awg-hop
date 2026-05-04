package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"awghop/internal/api/respond"
	"awghop/internal/db"
	"awghop/internal/domain"
)

const backupSchemaVersion = 1

type backupManifest struct {
	Version    int    `json:"version"`
	App        string `json:"app"`
	ExportedAt string `json:"exported_at"`
	Schema     int    `json:"schema_version,omitempty"`
}

func (h *Handlers) SystemStatus(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"time": time.Now().UTC().Format(time.RFC3339),
	}
	if h.Net != nil {
		for k, v := range h.Net.Status() {
			out[k] = v
		}

		in, err := h.Store.GetIngressSettings(r.Context())
		if err == nil {
			ingress := map[string]any{
				"interface_name": in.InterfaceName,
				"listen_port":    in.ListenPort,
			}
			if peers, err := h.Net.IngressStatus(r.Context(), in.InterfaceName); err == nil {
				ingress["peers"] = peers
			} else {
				ingress["peers_error"] = err.Error()
			}
			out["ingress"] = ingress
		}

		if list, err := h.Store.ListEgressTunnels(r.Context()); err == nil {
			arr := make([]map[string]any, 0, len(list))
			for _, t := range list {
				row := map[string]any{
					"id":             t.ID,
					"name":           t.Name,
					"interface_name": t.InterfaceName,
					"enabled":        t.Enabled,
				}
				if t.Enabled {
					row["status"] = h.Net.EgressStatus(r.Context(), t.InterfaceName)
				}
				arr = append(arr, row)
			}
			out["egress_tunnels"] = arr
		}
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *Handlers) SystemApply(w http.ResponseWriter, r *http.Request) {
	if h.Net == nil {
		respond.JSON(w, http.StatusOK, map[string]string{"status": "noop", "detail": "netctl disabled"})
		return
	}
	if err := h.Net.Apply(r.Context(), h.Store); err != nil {
		respond.Error(w, http.StatusInternalServerError, "apply_failed", err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) GetSystemSettings(w http.ResponseWriter, r *http.Request) {
	ss, err := h.Store.GetSystemSettings(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, ss)
}

func (h *Handlers) PutSystemSettings(w http.ResponseWriter, r *http.Request) {
	var body domain.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}
	if body.TunnelOfflinePolicy == "" {
		body.TunnelOfflinePolicy = domain.TunnelOfflineBlock
	}
	if body.TunnelOfflinePolicy != domain.TunnelOfflineBlock && body.TunnelOfflinePolicy != domain.TunnelOfflineIgnore {
		respond.Error(w, http.StatusBadRequest, "validation", "tunnel_offline_policy must be 'block' or 'ignore'")
		return
	}
	if body.ClientAllowedIPv4 == "" {
		body.ClientAllowedIPv4 = "0.0.0.0/0"
	}
	if err := h.Store.UpdateSystemSettings(r.Context(), body); err != nil {
		respond.Error(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	out, err := h.Store.GetSystemSettings(r.Context())
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

func (h *Handlers) BackupExport(w http.ResponseWriter, r *http.Request) {
	path := h.Cfg.DatabasePath
	f, err := os.Open(path)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "open_db", err.Error())
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="awghop-backup.zip"`)

	zw := zip.NewWriter(w)
	defer func() { _ = zw.Close() }()

	manifest, err := zw.Create("manifest.json")
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "zip", err.Error())
		return
	}
	schemaVer, _ := db.CurrentSchemaVersion(r.Context(), h.Store.DB())
	_ = json.NewEncoder(manifest).Encode(backupManifest{
		Version:    backupSchemaVersion,
		App:        "awg-hop",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Schema:     schemaVer,
	})

	out, err := zw.Create(filepath.Base(path))
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "zip", err.Error())
		return
	}
	if _, err := io.Copy(out, f); err != nil {
		return
	}
}

// BackupImport принимает zip из BackupExport: проверяет manifest, валидирует
// SQLite-файл и подменяет БД. После успешной замены вызывается Apply, чтобы
// перевыкатить интерфейсы и policy routing.
func (h *Handlers) BackupImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<20)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_form", err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "no_file", "missing 'file' field")
		return
	}
	defer file.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		respond.Error(w, http.StatusInternalServerError, "read_upload", err.Error())
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_zip", err.Error())
		return
	}

	var manifest *backupManifest
	dbBaseName := filepath.Base(h.Cfg.DatabasePath)
	var dbEntry *zip.File
	for _, zf := range zr.File {
		switch filepath.Base(zf.Name) {
		case "manifest.json":
			rc, err := zf.Open()
			if err != nil {
				respond.Error(w, http.StatusBadRequest, "bad_manifest", err.Error())
				return
			}
			var m backupManifest
			if err := json.NewDecoder(rc).Decode(&m); err != nil {
				rc.Close()
				respond.Error(w, http.StatusBadRequest, "bad_manifest", err.Error())
				return
			}
			rc.Close()
			manifest = &m
		case dbBaseName:
			dbEntry = zf
		}
	}
	if manifest == nil {
		respond.Error(w, http.StatusBadRequest, "bad_archive", "manifest.json missing")
		return
	}
	if manifest.App != "awg-hop" {
		respond.Error(w, http.StatusBadRequest, "bad_archive", "manifest app mismatch")
		return
	}
	if manifest.Version != backupSchemaVersion {
		respond.Error(w, http.StatusBadRequest, "bad_archive", fmt.Sprintf("unsupported backup version %d", manifest.Version))
		return
	}
	if dbEntry == nil {
		respond.Error(w, http.StatusBadRequest, "bad_archive", "database file missing in archive")
		return
	}

	rc, err := dbEntry.Open()
	if err != nil {
		respond.Error(w, http.StatusBadRequest, "bad_archive", err.Error())
		return
	}
	defer rc.Close()
	dbBytes, err := io.ReadAll(rc)
	if err != nil {
		respond.Error(w, http.StatusInternalServerError, "read_archive", err.Error())
		return
	}
	if !looksLikeSQLite(dbBytes) {
		respond.Error(w, http.StatusBadRequest, "bad_archive", "embedded database is not SQLite")
		return
	}

	if err := h.Store.ReplaceDatabase(r.Context(), h.Cfg.DatabasePath, dbBytes); err != nil {
		respond.Error(w, http.StatusInternalServerError, "replace_db", err.Error())
		return
	}

	if h.Net != nil {
		if err := h.Net.Apply(r.Context(), h.Store); err != nil {
			respond.JSON(w, http.StatusOK, map[string]any{
				"status":      "imported_with_apply_error",
				"apply_error": err.Error(),
			})
			return
		}
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func looksLikeSQLite(b []byte) bool {
	const magic = "SQLite format 3\x00"
	return len(b) >= len(magic) && bytes.HasPrefix(b, []byte(magic))
}

