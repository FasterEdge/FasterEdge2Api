package server

import (
	"net/http"

	"github.com/FasterEdge/FasterEdge/ability"
	"github.com/FasterEdge/FasterEdge/data"
)

// handleTopology 返回本节点 + 对等节点拓扑快照。
func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "NetMapAbility", ability.NetMapCommandGetTopology, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleNodeInfo 返回本节点网络信息(NetMapData info)。
func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "NetMapData", data.NetMapCommandInfo, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// setNodeNameReq 是设置节点名的请求体。
type setNodeNameReq struct {
	Name string `json:"name"`
}

// handleSetNodeName 设置本节点名。
func (s *Server) handleSetNodeName(w http.ResponseWriter, r *http.Request) {
	var req setNodeNameReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "NetMapData", data.NetMapCommandSetNodeName,
		data.NetMapSetNodeNameArgs{Name: req.Name})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleNodeInterfaces 刷新并返回本机网卡接口列表。
func (s *Server) handleNodeInterfaces(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "NetMapData", data.NetMapCommandInterfaces, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// setDefaultIfaceReq 是设置默认出网接口的请求体。
type setDefaultIfaceReq struct {
	Name string `json:"name"`
}

// handleSetDefaultIface 设置默认出网接口。
func (s *Server) handleSetDefaultIface(w http.ResponseWriter, r *http.Request) {
	var req setDefaultIfaceReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "NetMapData", data.NetMapCommandSetDefaultIface,
		data.NetMapSetDefaultIfaceArgs{Name: req.Name})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleListPeers 返回对等节点列表。
func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "NetMapAbility", ability.NetMapCommandListPeers, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// registerPeerReq 是注册对等节点的请求体。
type registerPeerReq struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Role    string `json:"role"`
}

// handleRegisterPeer 注册一个对等节点。
func (s *Server) handleRegisterPeer(w http.ResponseWriter, r *http.Request) {
	var req registerPeerReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "NetMapAbility", ability.NetMapCommandRegisterPeer,
		ability.NetMapRegisterPeerArgs{Name: req.Name, Address: req.Address, Role: req.Role})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleLookupPeer 按名称查询对等节点。
func (s *Server) handleLookupPeer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	val, err := s.authed(r, "NetMapAbility", ability.NetMapCommandLookupPeer,
		ability.NetMapLookupPeerArgs{Name: name})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// updatePeerReq 是更新对等节点的请求体。
type updatePeerReq struct {
	NewAddress    string `json:"new_address"`
	NewRole       string `json:"new_role"`
	TouchLastSeen bool   `json:"touch_last_seen"`
}

// handleUpdatePeer 更新对等节点(零值字段不生效)。
func (s *Server) handleUpdatePeer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req updatePeerReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	val, err := s.authed(r, "NetMapAbility", ability.NetMapCommandUpdatePeer,
		ability.NetMapUpdatePeerArgs{Name: name, NewAddress: req.NewAddress, NewRole: req.NewRole, TouchLastSeen: req.TouchLastSeen})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleUnregisterPeer 移除对等节点。
func (s *Server) handleUnregisterPeer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	val, err := s.authed(r, "NetMapAbility", ability.NetMapCommandUnregisterPeer,
		ability.NetMapLookupPeerArgs{Name: name})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}
