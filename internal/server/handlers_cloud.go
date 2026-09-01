// ─────────────────────────────────────────────────────────────
// FasterEdge 开源项目
// Github: https://github.com/FasterEdge
// Gitee:  https://gitee.com/FasterEdge
// ─────────────────────────────────────────────────────────────
package server

import (
	"net/http"

	"github.com/FasterEdge/FasterEdge/ability"
)

// requireCloud 检查 CloudRoleAbility 是否可用;不可用时写出错误并返回 false。
func (s *Server) requireCloud(w http.ResponseWriter, r *http.Request) bool {
	if s.engine.HasAbility("CloudRoleAbility") {
		return true
	}
	writeError(w, http.StatusNotImplemented, "this node is not a cloud node (FE2A_ROLE=cloud)")
	return false
}

// handleCloudDescribe 返回云端节点完整状态。
func (s *Server) handleCloudDescribe(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandDescribe, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleCloudGetController 返回控制器地址。
func (s *Server) handleCloudGetController(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandGetController, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// setControllerReq 是设置控制器的请求体。
type setControllerReq struct {
	URL string `json:"url"`
}

// handleCloudSetController 设置控制器地址。
func (s *Server) handleCloudSetController(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	var req setControllerReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandSetController,
		ability.CloudRoleSetControllerArgs{URL: req.URL})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleCloudListServices 返回服务清单。
func (s *Server) handleCloudListServices(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandListServices, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// registerServiceReq 是注册服务的请求体。
type registerServiceReq struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Endpoint string `json:"endpoint"`
}

// handleCloudRegisterService 注册对外服务。
func (s *Server) handleCloudRegisterService(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	var req registerServiceReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandRegister,
		ability.CloudRoleRegisterServiceArgs{Name: req.Name, Version: req.Version, Endpoint: req.Endpoint})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleCloudUnregisterService 注销服务。
func (s *Server) handleCloudUnregisterService(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	name := r.PathValue("name")
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandUnregister,
		ability.CloudRoleUnregisterServiceArgs{Name: name})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleCloudGetStatus 返回云端节点状态。
func (s *Server) handleCloudGetStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandGetStatus, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// setStatusReq 是设置状态的请求体。
type setStatusReq struct {
	Status string `json:"status"`
}

// handleCloudSetStatus 设置云端节点状态(healthy/degraded/offline)。
func (s *Server) handleCloudSetStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	var req setStatusReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandSetStatus,
		ability.CloudRoleSetStatusArgs{Status: ability.CloudRoleStatus(req.Status)})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleCloudHeartbeat 记录一次心跳。
func (s *Server) handleCloudHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !s.requireCloud(w, r) {
		return
	}
	val, err := s.authed(r, "CloudRoleAbility", ability.CloudRoleCommandHeartbeat, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}
