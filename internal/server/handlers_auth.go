// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package server

import (
	"net/http"
	"time"

	"github.com/FasterEdge/FasterEdge/ability"
)

// issueTokenReq 是签发令牌的请求体。
type issueTokenReq struct {
	Subject   string `json:"subject"`
	TTL       string `json:"ttl"`       // Go duration 字符串,例如 "1h";空则用默认
	Algorithm string `json:"algorithm"` // 可选,默认 HMAC-SHA256
}

// handleAuthIssue 为指定主体签发令牌。
// 注意:该端点本身要求凭据,所以首次令牌请使用 CLI(`fasteredge2api token`)引导。
func (s *Server) handleAuthIssue(w http.ResponseWriter, r *http.Request) {
	var req issueTokenReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validIdentifier(req.Subject) {
		writeError(w, http.StatusBadRequest, "subject must be 1-128 safe characters")
		return
	}
	var ttl time.Duration
	if req.TTL != "" {
		d, err := time.ParseDuration(req.TTL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ttl: "+err.Error())
			return
		}
		ttl = d
	}
	out, err := s.authed(r, "OneKeyAbility", ability.OneKeyCommandIssueToken,
		ability.OneKeyIssueTokenArgs{Subject: req.Subject, TTL: ttl, Algorithm: req.Algorithm})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	issued, ok := out.(ability.OneKeyToken)
	if !ok {
		writeError(w, http.StatusInternalServerError, "token issuance failed")
		return
	}
	val, err := toStringMap(issued)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token issuance failed")
		return
	}
	// 直接返回传输编码,避免客户端往返 RFC3339 时丢失纳秒精度。
	val["token"] = ability.EncodeForTransmission(issued)
	writeData(w, val)
}

// verifyTokenReq 是校验令牌的请求体。token 为传输编码字符串。
type verifyTokenReq struct {
	Token string `json:"token"`
}

// handleAuthVerify 校验一个传输编码形式的令牌,返回其主体。
func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	var req verifyTokenReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	tok, err := ability.DecodeFromTransmission(req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid token encoding: "+err.Error())
		return
	}
	out, err := s.authed(r, "OneKeyAbility", ability.OneKeyCommandVerifyToken,
		ability.OneKeyVerifyTokenArgs{Subject: tok.Subject, IssuedAt: tok.IssuedAt, ExpiresAt: tok.ExpiresAt, Signature: tok.Signature})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, out)
}

// revokeTokenReq 是吊销令牌的请求体。
type revokeTokenReq struct {
	Subject string `json:"subject"`
}

// handleAuthRevoke 吊销指定主体的令牌。
func (s *Server) handleAuthRevoke(w http.ResponseWriter, r *http.Request) {
	var req revokeTokenReq
	if err := readJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validIdentifier(req.Subject) {
		writeError(w, http.StatusBadRequest, "subject must be 1-128 safe characters")
		return
	}
	val, err := s.authed(r, "OneKeyAbility", ability.OneKeyCommandRevokeToken,
		ability.OneKeyRevokeTokenArgs{Subject: req.Subject})
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleAuthRevokeAll 吊销全部令牌,返回被吊销数量。
func (s *Server) handleAuthRevokeAll(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "OneKeyAbility", ability.OneKeyCommandRevokeAll, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleAuthListTokens 列出全部令牌条目(不含签名)。
func (s *Server) handleAuthListTokens(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "OneKeyAbility", ability.OneKeyCommandListTokens, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleAuthStatus 返回 Keyring 状态(算法、指纹、活动令牌数等)。
func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "OneKeyAbility", ability.OneKeyCommandStatus, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}

// handleAuthRotate 轮换共享密钥(所有已签发令牌立即失效)。
func (s *Server) handleAuthRotate(w http.ResponseWriter, r *http.Request) {
	val, err := s.authed(r, "OneKeyAbility", ability.OneKeyCommandRotate, nil)
	if err != nil {
		writeCommandErr(w, err)
		return
	}
	writeData(w, val)
}
