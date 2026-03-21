package web

import (
	"net/http"

	"go-port-forward/internal/models"
	"go-port-forward/pkg/os/wsl"
)

func (h *handler) wslListDistros(w http.ResponseWriter, r *http.Request) {
	distros, err := wsl.ListDistros()
	if err != nil {
		fail(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	ok(w, distros)
}

func (h *handler) wslListPorts(w http.ResponseWriter, r *http.Request) {
	distro := r.PathValue("distro")
	ports, err := wsl.ListPorts(distro)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	ok(w, ports)
}

func (h *handler) wslImport(w http.ResponseWriter, r *http.Request) {
	var req models.WSLImportRequest
	if !decodeBody(r, &req) {
		fail(w, http.StatusBadRequest, "无效的请求体 | Invalid JSON body")
		return
	}
	if req.TargetAddr == "" {
		// 自动检测 WSL2 IP | Auto-detect WSL2 IP.
		ip, err := wsl.GetIP(req.Distro)
		if err != nil {
			fail(w, http.StatusServiceUnavailable, "无法检测 WSL2 IP | Cannot detect WSL2 IP: "+err.Error())
			return
		}
		req.TargetAddr = ip
	}

	var created []*models.ForwardRule
	for _, p := range req.Ports {
		proto := models.Protocol(p.Protocol)
		if proto == "" {
			proto = models.ProtocolTCP
		}
		rule, err := h.mgr.AddRule(&models.CreateRuleRequest{
			Name:       req.Distro + ":" + p.Process + ":" + string(proto) + "/" + itoa(p.Port),
			ListenAddr: "0.0.0.0",
			ListenPort: p.Port,
			Protocol:   proto,
			TargetAddr: req.TargetAddr,
			TargetPort: p.Port,
			Enabled:    true,
			Comment:    "从 WSL2 发行版导入 | Imported from WSL2 distro " + req.Distro,
		})
		if err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
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
