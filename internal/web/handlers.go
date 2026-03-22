package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"go-port-forward/internal/firewall"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/models"
	"go-port-forward/internal/storage"
	"go-port-forward/pkg/os/wsl"
	"go-port-forward/pkg/serializer/json"
)

const maxJSONBodyBytes int64 = 1 << 20

type handler struct {
	mgr *forward.Manager
	fw  firewall.Manager
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: data})
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, models.APIResponse{Success: false, Message: msg})
}

func decodeBody[T any](w http.ResponseWriter, r *http.Request, dst *T) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeBodyError(err)
	}

	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("请求体只能包含单个 JSON 对象 | request body must contain a single JSON object")
	}
	return nil
}

func decodeBodyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, io.EOF):
		return fmt.Errorf("请求体不能为空 | request body is required")
	default:
		return fmt.Errorf("无效的请求体 | invalid JSON body: %v", err)
	}
}

func writeAPIError(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, storage.ErrRuleNotFound):
		fail(w, http.StatusNotFound, err.Error())
	case errors.Is(err, forward.ErrInvalidRule):
		fail(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, forward.ErrPortConflict):
		fail(w, http.StatusConflict, err.Error())
	case errors.Is(err, wsl.ErrNotSupported):
		fail(w, http.StatusNotImplemented, err.Error())
	default:
		fail(w, http.StatusInternalServerError, err.Error())
	}
}

// --- Rules CRUD ---

func (h *handler) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.mgr.ListRules()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, rules)
}

func (h *handler) createRule(w http.ResponseWriter, r *http.Request) {
	var req models.CreateRuleRequest
	if err := decodeBody(w, r, &req); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := models.ValidateCreateRuleRequest(&req); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	rule, err := h.mgr.AddRule(&req)
	if err != nil {
		writeAPIError(w, err)
		return
	}

	// Optionally add firewall rule.
	if req.AddFirewall && h.fw != nil {
		_ = h.fw.AddRule(firewall.Rule{
			Name:     rule.Name,
			Port:     rule.ListenPort,
			Protocol: rule.Protocol,
		})
	}
	writeJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: rule})
}

func (h *handler) getRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rule, err := h.mgr.GetRule(id)
	if err != nil {
		fail(w, http.StatusNotFound, err.Error())
		return
	}
	ok(w, rule)
}

func (h *handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req models.UpdateRuleRequest
	if err := decodeBody(w, r, &req); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	rule, err := h.mgr.UpdateRule(id, &req)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ok(w, rule)
}

func (h *handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Fetch before delete so we can remove the firewall rule.
	existing, err := h.mgr.GetRule(id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if err := h.mgr.DeleteRule(id); err != nil {
		writeAPIError(w, err)
		return
	}
	if existing != nil && existing.AddFirewall && h.fw != nil {
		_ = h.fw.DeleteRule(firewall.Rule{
			Name:     existing.Name,
			Port:     existing.ListenPort,
			Protocol: existing.Protocol,
		})
	}
	ok(w, nil)
}

func (h *handler) toggleRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decodeBody(w, r, &body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Enabled == nil {
		fail(w, http.StatusBadRequest, "enabled 字段不能为空 | enabled field is required")
		return
	}
	rule, err := h.mgr.ToggleRule(id, *body.Enabled)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ok(w, rule)
}

func (h *handler) globalStats(w http.ResponseWriter, _ *http.Request) {
	ok(w, h.mgr.GlobalStats())
}
