package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/FasterEdge/FasterEdge/ability"
)

// ctxKey 是请求上下文中凭据的键类型。
type ctxKey struct{}

// withCredential 把 OneKey 凭据放入请求上下文。
func withCredential(ctx context.Context, cred ability.OneKeyCredential) context.Context {
	return context.WithValue(ctx, ctxKey{}, cred)
}

// credentialFromContext 取出请求上下文中的 OneKey 凭据。
func credentialFromContext(ctx context.Context) (ability.OneKeyCredential, bool) {
	cred, ok := ctx.Value(ctxKey{}).(ability.OneKeyCredential)
	return cred, ok
}

// credentialFromRequest 从 Authorization 头解析 OneKey 凭据。
// 期望格式:Authorization: Bearer <subject.issuedNanos.expiresNanos.signature>
func credentialFromRequest(r *http.Request) (ability.OneKeyCredential, bool) {
	header := r.Header.Get("Authorization")
	raw, ok := strings.CutPrefix(header, "Bearer ")
	if !ok {
		raw, ok = strings.CutPrefix(header, "bearer ")
	}
	if !ok {
		// 兼容无 scheme 形式。
		raw = strings.TrimSpace(header)
		if raw == "" {
			return ability.OneKeyCredential{}, false
		}
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ability.OneKeyCredential{}, false
	}
	tok, err := ability.DecodeFromTransmission(raw)
	if err != nil {
		return ability.OneKeyCredential{}, false
	}
	return ability.OneKeyCredential{
		Subject:   tok.Subject,
		IssuedAt:  tok.IssuedAt,
		ExpiresAt: tok.ExpiresAt,
		Signature: tok.Signature,
	}, true
}
