package web

import (
	"net/http"

	"go-port-forward/internal/firewall"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/models"
	"go-port-forward/pkg/serializer/json"
)

type handler struct {
	mgr *forward.Manager
	fw  firewall.Manager
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: data})
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, models.APIResponse{Success: false, Message: msg})
}

func decodeBody[T any](r *http.Request, dst *T) bool {
	return json.NewDecoder(r.Body).Decode(dst) == nil
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
	if !decodeBody(r, &req) {
		fail(w, http.StatusBadRequest, "无效的请求体 | Invalid JSON body")
		return
	}
	if req.Name == "" {
		fail(w, http.StatusBadRequest, "规则名称不能为空 | Name is required")
		return
	}
	if req.TargetAddr == "" {
		fail(w, http.StatusBadRequest, "目标地址不能为空 | target_addr is required")
		return
	}
	if req.ListenPort <= 0 || req.ListenPort > 65535 {
		fail(w, http.StatusBadRequest, "监听端口超出范围 (1-65535) | listen_port out of range (1-65535)")
		return
	}
	if req.TargetPort <= 0 || req.TargetPort > 65535 {
		fail(w, http.StatusBadRequest, "目标端口超出范围 (1-65535) | target_port out of range (1-65535)")
		return
	}
	if req.Protocol == "" {
		req.Protocol = models.ProtocolTCP
	}
	if req.Protocol != models.ProtocolTCP && req.Protocol != models.ProtocolUDP && req.Protocol != models.ProtocolBoth {
		fail(w, http.StatusBadRequest, "协议必须为 tcp、udp 或 both | Protocol must be tcp, udp, or both")
		return
	}
	if req.ListenAddr == "" {
		req.ListenAddr = "0.0.0.0"
	}

	rule, err := h.mgr.AddRule(&req)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
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
	if !decodeBody(r, &req) {
		fail(w, http.StatusBadRequest, "无效的请求体 | Invalid JSON body")
		return
	}
	rule, err := h.mgr.UpdateRule(id, &req)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, rule)
}

func (h *handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Fetch before delete so we can remove the firewall rule.
	existing, _ := h.mgr.GetRule(id)
	if err := h.mgr.DeleteRule(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
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
		Enabled bool `json:"enabled"`
	}
	if !decodeBody(r, &body) {
		fail(w, http.StatusBadRequest, "无效的请求体 | Invalid JSON body")
		return
	}
	rule, err := h.mgr.ToggleRule(id, body.Enabled)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, rule)
}

func (h *handler) globalStats(w http.ResponseWriter, _ *http.Request) {
	ok(w, h.mgr.GlobalStats())
}
