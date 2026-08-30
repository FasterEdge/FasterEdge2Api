package server

import (
	"net/http"

	"github.com/FasterEdge/FasterEdge/ability"
)

// requireEdge 检查 EdgeRoleAbility 是否可用;不可用时写出错误并返回 false。
func (s *Server) requireEdge(w http.ResponseWriter, r *http.Request) bool {
	if s.engine.HasAbility("EdgeRoleAbility") {
		return true
	}
	writeError(w, http.StatusNotImplemented, "this node is not an edge node (FE2A_ROLE=edge)")
	return false
}

// handleEdgeDescribe 返回边缘节点完整状态。
func (s *Server) handleEdgeDescribe(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandDescribe, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleEdgeGetZone 返回边缘节点区域。
func (s *Server) handleEdgeGetZone(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandGetZone, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// setZoneReq 是设置区域的请求体。
type setZoneReq struct {
	Zone string `json:"zone"`
}

// handleEdgeSetZone 设置边缘节点区域。
func (s *Server) handleEdgeSetZone(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	var req setZoneReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandSetZone,
		ability.EdgeRoleSetZoneArgs{Zone: req.Zone})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleEdgeListCapabilities 返回能力清单。
func (s *Server) handleEdgeListCapabilities(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandListCaps, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// capabilityReq 是添加/移除能力的请求体。
type capabilityReq struct {
	Name string `json:"name"`
}

// handleEdgeAddCapability 添加一项能力。
func (s *Server) handleEdgeAddCapability(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	var req capabilityReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandAddCap,
		ability.EdgeRoleCapabilityArg{Name: req.Name})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// setCapabilitiesReq 是整体覆盖能力清单的请求体。
type setCapabilitiesReq struct {
	Capabilities []string `json:"capabilities"`
}

// handleEdgeSetCapabilities 整体覆盖能力清单。
func (s *Server) handleEdgeSetCapabilities(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	var req setCapabilitiesReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandSetCaps,
		ability.EdgeRoleSetCapabilitiesArgs{Capabilities: req.Capabilities})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleEdgeRemoveCapability 移除一项能力。
func (s *Server) handleEdgeRemoveCapability(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	name := r.PathValue("name")
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandRemoveCap,
		ability.EdgeRoleCapabilityArg{Name: name})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// recordLatencyReq 是记录延迟的请求体。
type recordLatencyReq struct {
	LatencyMs float64 `json:"latency_ms"`
}

// handleEdgeRecordLatency 记录一次延迟采样。
func (s *Server) handleEdgeRecordLatency(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	var req recordLatencyReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandRecordLatency,
		ability.EdgeRoleRecordLatencyArgs{LatencyMs: req.LatencyMs})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleEdgeGetMetrics 返回边缘节点运行指标。
func (s *Server) handleEdgeGetMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandGetMetrics, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// setOnlineReq 是设置在线状态的请求体。
type setOnlineReq struct {
	Online bool `json:"online"`
}

// handleEdgeSetOnline 设置边缘节点在线状态。
func (s *Server) handleEdgeSetOnline(w http.ResponseWriter, r *http.Request) {
	if !s.requireEdge(w, r) {
		return
	}
	var req setOnlineReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "EdgeRoleAbility", ability.EdgeRoleCommandSetOnline,
		ability.EdgeRoleSetOnlineArgs{Online: req.Online})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}
