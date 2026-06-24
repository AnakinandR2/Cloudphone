package automation

import (
	"encoding/json"
	"regexp"
	"strings"

	"manager-backend/framework/apperr"
)

// ParamType 是脚本参数的类型（覆盖全 JSON 类型 + enum 下拉）。
type ParamType string

const (
	ParamString  ParamType = "string"
	ParamNumber  ParamType = "number"
	ParamBoolean ParamType = "boolean"
	ParamEnum    ParamType = "enum"
	ParamArray   ParamType = "array"
	ParamObject  ParamType = "object"
)

var validParamTypes = map[ParamType]bool{
	ParamString: true, ParamNumber: true, ParamBoolean: true,
	ParamEnum: true, ParamArray: true, ParamObject: true,
}

// ParamSpec 是单个参数定义（automation_scripts.params_schema 数组项）。
type ParamSpec struct {
	Key         string    `json:"key"`
	Label       string    `json:"label,omitempty"`
	Type        ParamType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Default     any       `json:"default,omitempty"`
	Description string    `json:"description,omitempty"`
	Options     []string  `json:"options,omitempty"` // 仅 enum：候选值
}

var paramKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validateSchema 解析并校验 params_schema 原始 JSON。空串 = 无参数（合法，返回 nil 切片）。
func validateSchema(raw string) ([]ParamSpec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var specs []ParamSpec
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		return nil, apperr.Validation("参数定义不是合法的 JSON 数组")
	}
	seen := make(map[string]bool, len(specs))
	for _, sp := range specs {
		if !paramKeyRe.MatchString(sp.Key) {
			return nil, apperr.Validation("参数 key 非法（须字母或下划线开头）：" + sp.Key)
		}
		if seen[sp.Key] {
			return nil, apperr.Validation("参数 key 重复：" + sp.Key)
		}
		seen[sp.Key] = true
		if !validParamTypes[sp.Type] {
			return nil, apperr.Validation("参数类型非法：" + sp.Key)
		}
		if sp.Type == ParamEnum {
			if len(sp.Options) == 0 {
				return nil, apperr.Validation("enum 参数需要候选项：" + sp.Key)
			}
			if sp.Default != nil {
				dv, ok := sp.Default.(string)
				if !ok || !containsStr(sp.Options, dv) {
					return nil, apperr.Validation("enum 参数默认值不在候选项内：" + sp.Key)
				}
			}
		} else if sp.Default != nil && !valueMatchesType(sp.Type, sp.Default) {
			return nil, apperr.Validation("参数默认值类型不匹配：" + sp.Key)
		}
	}
	return specs, nil
}

// valueMatchesType 判断一个（JSON 反序列化得到的）值是否符合参数类型。
func valueMatchesType(t ParamType, v any) bool {
	switch t {
	case ParamString, ParamEnum:
		_, ok := v.(string)
		return ok
	case ParamNumber:
		switch v.(type) {
		case float64, float32, int, int8, int16, int32, int64, json.Number:
			return true
		}
		return false
	case ParamBoolean:
		_, ok := v.(bool)
		return ok
	case ParamArray:
		_, ok := v.([]any)
		return ok
	case ParamObject:
		_, ok := v.(map[string]any)
		return ok
	}
	return false
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func labelOf(sp ParamSpec) string {
	if sp.Label != "" {
		return sp.Label
	}
	return sp.Key
}

// buildParams 按 schema 校验并构造「最终参数」：只含已声明的 key，缺失时回填 default。
// values 里 schema 未声明的 key 被忽略（容错，见设计 §3.3）。
func buildParams(specs []ParamSpec, values map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(specs))
	for _, sp := range specs {
		v, ok := values[sp.Key]
		if !ok || v == nil {
			if sp.Default != nil {
				out[sp.Key] = sp.Default
				continue
			}
			if sp.Required {
				return nil, apperr.Validation(labelOf(sp) + " 为必填")
			}
			continue
		}
		if sp.Type == ParamEnum {
			sv, ok := v.(string)
			if !ok || !containsStr(sp.Options, sv) {
				return nil, apperr.Validation(labelOf(sp) + " 取值不在候选项内")
			}
		} else if !valueMatchesType(sp.Type, v) {
			return nil, apperr.Validation(labelOf(sp) + " 类型应为 " + string(sp.Type))
		}
		out[sp.Key] = v
	}
	return out, nil
}

// mergeParams 浅合并：override 覆盖 base，返回新 map（不改动入参）。
func mergeParams(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

// serializeParams 把最终参数序列化为中台 scriptParams 字符串；空则返回 ""（透传时 omitempty 丢弃）。
func serializeParams(params map[string]any) string {
	if len(params) == 0 {
		return ""
	}
	b, err := json.Marshal(params)
	if err != nil {
		return ""
	}
	return string(b)
}
