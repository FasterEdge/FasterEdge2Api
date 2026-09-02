// FasterEdge 开源项目 - Github: https://github.com/FasterEdge - Gitee: https://gitee.com/FasterEdge
package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/FasterEdge/FasterEdge/ability"
)

type credentialKey struct{}
type subjectKey struct{}

// withCredential 把已验证的 OneKey 凭据与主体放入请求上下文。
func withCredential(ctx context.Context, cred ability.OneKeyCredential, subject string) context.Context {
	ctx = context.WithValue(ctx, credentialKey{}, cred)
	return context.WithValue(ctx, subjectKey{}, subject)
}

func credentialFromContext(ctx context.Context) (ability.OneKeyCredential, bool) {
	cred, ok := ctx.Value(credentialKey{}).(ability.OneKeyCredential)
	return cred, ok
}

func subjectFromContext(ctx context.Context) (string, bool) {
	subject, ok := ctx.Value(subjectKey{}).(string)
	return subject, ok && subject != ""
}

// credentialFromRequest 严格解析 RFC 6750 Bearer 凭据。
func credentialFromRequest(r *http.Request) (ability.OneKeyCredential, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ability.OneKeyCredential{}, false
	}
	tok, err := ability.DecodeFromTransmission(parts[1])
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
