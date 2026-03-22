package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"go-port-forward/internal/firewall"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/logger"
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

type dashboardResponse struct {
	Rules []*models.ForwardRule `json:"rules"`
	Stats *models.Stats         `json:"stats"`
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

func okWithMessage(w http.ResponseWriter, data interface{}, msg string) {
	writeJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: data, Message: msg})
}

func createdWithMessage(w http.ResponseWriter, data interface{}, msg string) {
	writeJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: data, Message: msg})
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

func (h *handler) firewallRule(rule *models.ForwardRule) firewall.Rule {
	return firewall.Rule{Name: rule.Name, Port: rule.ListenPort, Protocol: rule.Protocol}
}

func (h *handler) syncFirewallOnCreate(rule *models.ForwardRule) string {
	if h.fw == nil || rule == nil || !rule.AddFirewall || !rule.Enabled {
		return ""
	}
	if err := h.fw.AddRule(h.firewallRule(rule)); err != nil {
		logger.S.Warnw("Firewall sync failed after rule create", "rule_id", rule.ID, "rule", rule.Name, "err", err)
		return "规则已创建，但防火墙规则同步失败 | rule created, but firewall sync failed: " + err.Error()
	}
	return ""
}

func (h *handler) syncFirewallOnDelete(rule *models.ForwardRule) string {
	if h.fw == nil || rule == nil || !rule.AddFirewall {
		return ""
	}
	if err := h.fw.DeleteRule(h.firewallRule(rule)); err != nil {
		logger.S.Warnw("Firewall sync failed after rule delete", "rule_id", rule.ID, "rule", rule.Name, "err", err)
		return "规则已删除，但防火墙规则清理失败 | rule deleted, but firewall cleanup failed: " + err.Error()
	}
	return ""
}

func (h *handler) syncFirewallOnUpdate(before, after *models.ForwardRule) string {
	if h.fw == nil || before == nil || after == nil {
		return ""
	}

	oldRule := h.firewallRule(before)
	newRule := h.firewallRule(after)
	oldManaged := before.AddFirewall && before.Enabled
	newManaged := after.AddFirewall && after.Enabled

	switch {
	case after.AddFirewall && !after.Enabled:
		if before.AddFirewall {
			if err := h.fw.DeleteRule(oldRule); err != nil {
				logger.S.Warnw("Firewall cleanup failed while disabling rule", "rule_id", after.ID, "rule", after.Name, "err", err)
				return "规则已更新，但防火墙规则清理失败 | rule updated, but firewall cleanup failed: " + err.Error()
			}
		}
		return ""
	case !after.AddFirewall:
		if before.AddFirewall {
			if err := h.fw.DeleteRule(oldRule); err != nil {
				logger.S.Warnw("Firewall cleanup failed after disabling firewall option", "rule_id", after.ID, "rule", after.Name, "err", err)
				return "规则已更新，但防火墙规则清理失败 | rule updated, but firewall cleanup failed: " + err.Error()
			}
		}
		return ""
	case !oldManaged && newManaged:
		if err := h.fw.AddRule(newRule); err != nil {
			logger.S.Warnw("Firewall sync failed after enabling firewall option", "rule_id", after.ID, "rule", after.Name, "err", err)
			return "规则已更新，但防火墙规则同步失败 | rule updated, but firewall sync failed: " + err.Error()
		}
		return ""
	case oldManaged && newManaged && firewallRuleEqual(oldRule, newRule):
		return ""
	case oldManaged && newManaged:
		if err := h.fw.DeleteRule(oldRule); err != nil {
			logger.S.Warnw("Firewall cleanup failed before re-adding updated rule", "rule_id", after.ID, "rule", after.Name, "err", err)
			return "规则已更新，但旧防火墙规则清理失败 | rule updated, but old firewall rule cleanup failed: " + err.Error()
		}
		if err := h.fw.AddRule(newRule); err != nil {
			logger.S.Warnw("Firewall sync failed after rule endpoint change", "rule_id", after.ID, "rule", after.Name, "err", err)
			return "规则已更新，但新防火墙规则同步失败 | rule updated, but new firewall rule sync failed: " + err.Error()
		}
	}
	return ""
}

func firewallRuleEqual(a, b firewall.Rule) bool {
	return a.Name == b.Name && a.Port == b.Port && models.NormalizeProtocol(a.Protocol) == models.NormalizeProtocol(b.Protocol)
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
	createdWithMessage(w, rule, h.syncFirewallOnCreate(rule))
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
	before, err := h.mgr.GetRule(id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	rule, err := h.mgr.UpdateRule(id, &req)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	okWithMessage(w, rule, h.syncFirewallOnUpdate(before, rule))
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
	okWithMessage(w, nil, h.syncFirewallOnDelete(existing))
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
	before, err := h.mgr.GetRule(id)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	rule, err := h.mgr.ToggleRule(id, *body.Enabled)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	okWithMessage(w, rule, h.syncFirewallOnUpdate(before, rule))
}

func (h *handler) globalStats(w http.ResponseWriter, _ *http.Request) {
	_, stats, err := h.mgr.Snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, stats)
}

func (h *handler) dashboard(w http.ResponseWriter, _ *http.Request) {
	rules, stats, err := h.mgr.Snapshot()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, dashboardResponse{Rules: rules, Stats: stats})
}
