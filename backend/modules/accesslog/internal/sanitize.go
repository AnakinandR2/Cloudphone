package accesslog

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"
)

const maxBodyBytes = 2048

// sensitiveBodyFields 请求/响应体中需脱敏的字段名（小写匹配）。
var sensitiveBodyFields = map[string]bool{
	"password":      true,
	"secret":        true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"api_key":       true,
}

func sanitizeRequestHeaders(h http.Header) string {
	m := make(map[string]string, len(h))
	for k, v := range h {
		val := strings.Join(v, "; ")
		switch strings.ToLower(k) {
		case "authorization":
			if strings.HasPrefix(val, "Bearer ") {
				val = "Bearer ******"
			} else {
				val = "******"
			}
		case "cookie":
			val = "[REDACTED]"
		}
		m[k] = val
	}
	data, _ := json.Marshal(m)
	return string(data)
}

func sanitizeResponseHeaders(h http.Header) string {
	m := make(map[string]string, len(h))
	for k, v := range h {
		val := strings.Join(v, "; ")
		if strings.ToLower(k) == "set-cookie" {
			val = "[REDACTED]"
		}
		m[k] = val
	}
	data, _ := json.Marshal(m)
	return string(data)
}

func sanitizeBody(body, contentType string) string {
	if strings.Contains(contentType, "multipart/form-data") {
		return "[multipart/form-data]"
	}
	if body == "" {
		return ""
	}
	var data interface{}
	if err := json.Unmarshal([]byte(body), &data); err == nil {
		sanitizeJSON(&data)
		if out, err := json.Marshal(data); err == nil {
			return string(out)
		}
	}
	return body
}

func sanitizeJSON(v *interface{}) {
	switch val := (*v).(type) {
	case map[string]interface{}:
		for key, child := range val {
			if sensitiveBodyFields[strings.ToLower(key)] {
				val[key] = "******"
			} else {
				sanitizeJSON(&child)
				val[key] = child
			}
		}
	case []interface{}:
		for i, item := range val {
			sanitizeJSON(&item)
			val[i] = item
		}
	}
}

func truncBody(s string) string {
	if len(s) > maxBodyBytes {
		return truncUTF8(s, maxBodyBytes) + "...[truncated]"
	}
	return s
}

func truncStr(s string, max int) string {
	if len(s) > max {
		return truncUTF8(s, max)
	}
	return s
}

// truncUTF8 在不超过 maxBytes 的前提下，在完整 UTF-8 字符边界截断
func truncUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
}
