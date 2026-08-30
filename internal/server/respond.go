package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// writeJSON 输出统一 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		_, _ = w.Write([]byte("null"))
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		// 编码失败时尽量给出可读错误。
		http.Error(w, `{"ok":false,"error":{"message":"response encode failed"}}`, http.StatusInternalServerError)
	}
}

// writeError 输出统一错误 JSON。
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"ok":    false,
		"error": map[string]any{"code": status, "message": message},
	})
}

// writeData 输出成功响应体。数据键会统一归一化为 snake_case。
func writeData(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": normalizeResponse(data)})
}

// normalizeResponse 把任意响应值先经 JSON 编解码为通用结构,
// 再递归把 map 的键从 Go 大驼峰归一化为 snake_case,
// 保证前端 / API 使用者拿到一致的字段名(不依赖底层结构体的 json tag)。
func normalizeResponse(v any) any {
	if v == nil {
		return nil
	}
	body, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var generic any
	if err := json.Unmarshal(body, &generic); err != nil {
		return v
	}
	return normalizeKeys(generic)
}

// normalizeKeys 递归地把 map 键转换为 snake_case。
func normalizeKeys(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[toSnake(k)] = normalizeKeys(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeKeys(val)
		}
		return out
	default:
		return v
	}
}

// toSnake 把 Go 大驼峰/首字母大写标识符转换为 snake_case。
// 缩写(如 IPv4 / HTTPServer)会整体小写:"IPv4"→"ipv4","HTTPServer"→"http_server"。
func toSnake(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			lower := r - 'A' + 'a'
			if i > 0 {
				prev := runes[i-1]
				switch {
				case prev >= 'a' && prev <= 'z', prev >= '0' && prev <= '9':
					// 小写/数字后遇到大写:单词边界。
					b.WriteByte('_')
				case i >= 2 && runes[i-1] >= 'A' && runes[i-1] <= 'Z' &&
					i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z':
					// 连续大写缩写(至少两个)后紧跟小写:缩写边界。
					// 例如 "HTTPServer" 的 S,"IPv4" 的 P 因缩写不足两个字母不触发。
					b.WriteByte('_')
				}
			}
			b.WriteRune(lower)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// writeCommandErr 依据错误映射写出错误响应。
func writeCommandErr(w http.ResponseWriter, err error) {
	status := errStatus(err)
	writeJSON(w, status, fmtErr(status, err))
}

// contextWithTimeout 是 context.WithTimeout 的短别名。
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// readJSONBody 解析请求体为 target 结构。空体返回空结构错误。
func readJSONBody(r *http.Request, target any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

// toStringMap 把任意值先经 JSON 编解码转换为 map[string]any,
// 便于对 time.Time 等类型做统一序列化。非对象类型返回错误。
func toStringMap(v any) (map[string]any, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
}
