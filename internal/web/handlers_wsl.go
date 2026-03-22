package web

import (
	"fmt"
	"net/http"
	"strings"

	"go-port-forward/internal/models"
	"go-port-forward/pkg/os/wsl"
)

func (h *handler) wslListDistros(w http.ResponseWriter, r *http.Request) {
	distros, err := wsl.ListDistros()
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ok(w, distros)
}

func (h *handler) wslListPorts(w http.ResponseWriter, r *http.Request) {
	distro := r.PathValue("distro")
	if strings.TrimSpace(distro) == "" {
		fail(w, http.StatusBadRequest, "发行版名称不能为空 | distro is required")
		return
	}
	ports, err := wsl.ListPorts(distro)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	ok(w, ports)
}

func (h *handler) wslImport(w http.ResponseWriter, r *http.Request) {
	var req models.WSLImportRequest
	if err := decodeBody(w, r, &req); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Distro = strings.TrimSpace(req.Distro)
	req.TargetAddr = strings.TrimSpace(req.TargetAddr)
	if req.Distro == "" {
		fail(w, http.StatusBadRequest, "发行版名称不能为空 | distro is required")
		return
	}
	if len(req.Ports) == 0 {
		fail(w, http.StatusBadRequest, "至少选择一个端口 | at least one port is required")
		return
	}
	if req.TargetAddr == "" {
		// 自动检测 WSL2 IP | Auto-detect WSL2 IP.
		ip, err := wsl.GetIP(req.Distro)
		if err != nil {
			writeAPIError(w, fmt.Errorf("无法检测 WSL2 IP | cannot detect WSL2 IP: %w", err))
			return
		}
		req.TargetAddr = ip
	}

	batchKeys := make(map[string]struct{}, len(req.Ports))
	createReqs := make([]models.CreateRuleRequest, 0, len(req.Ports))
	for _, p := range req.Ports {
		proto := models.NormalizeProtocol(models.Protocol(p.Protocol))
		if proto == "" {
			proto = models.ProtocolTCP
		}
		createReq := models.CreateRuleRequest{
			Name:       req.Distro + ":" + p.Process + ":" + string(proto) + "/" + itoa(p.Port),
			ListenAddr: "0.0.0.0",
			ListenPort: p.Port,
			Protocol:   proto,
			TargetAddr: req.TargetAddr,
			TargetPort: p.Port,
			Enabled:    true,
			Comment:    "从 WSL2 发行版导入 | Imported from WSL2 distro " + req.Distro,
		}
		if err := h.mgr.ValidateCreateRequest(&createReq); err != nil {
			writeAPIError(w, err)
			return
		}
		key := fmt.Sprintf("%s:%d/%s", createReq.ListenAddr, createReq.ListenPort, createReq.Protocol)
		if _, exists := batchKeys[key]; exists {
			fail(w, http.StatusBadRequest, "导入列表包含重复端口/协议 | duplicate listen port/protocol in import list")
			return
		}
		batchKeys[key] = struct{}{}
		createReqs = append(createReqs, createReq)
	}

	var created []*models.ForwardRule
	for i := range createReqs {
		rule, err := h.mgr.AddRule(&createReqs[i])
		if err != nil {
			writeAPIError(w, err)
			return
		}
		created = append(created, rule)
	}
	writeJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: created})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
