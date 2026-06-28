package automation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"manager-backend/framework/apperr"
)

// ParamType 是脚本参数的类型。table = 普通 Lua table（值为 Lua table 字面量文本）。
type ParamType string

const (
	ParamString  ParamType = "string"
	ParamNumber  ParamType = "number"
	ParamBoolean ParamType = "boolean"
	ParamTable   ParamType = "table"
)

var validParamTypes = map[ParamType]bool{
	ParamString: true, ParamNumber: true, ParamBoolean: true, ParamTable: true,
}

// ParamSpec 是单个参数定义（脚本顶部注释里的一项）。
type ParamSpec struct {
	Key         string    `json:"key"`
	Type        ParamType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Default     any       `json:"default,omitempty"` // 默认值（table 为 Lua 字面量文本）
	Description string    `json:"description,omitempty"`
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
		if sp.Default != nil && !valueMatchesType(sp.Type, sp.Default) {
			return nil, apperr.Validation("参数默认值类型不匹配：" + sp.Key)
		}
	}
	return specs, nil
}

// valueMatchesType 判断一个（JSON 反序列化得到的）值是否符合参数类型。
func valueMatchesType(t ParamType, v any) bool {
	switch t {
	case ParamString:
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
	case ParamTable:
		// table 值通常是 Lua table 字面量文本（string）；也容忍 API 直接传 []any/map。
		switch v.(type) {
		case string, []any, map[string]any:
			return true
		}
		return false
	}
	return false
}

func labelOf(sp ParamSpec) string {
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
			// boolean 天生总有值：缺省即 false，不因 required 报错（复选框无「未填」态）。
			if sp.Type == ParamBoolean {
				out[sp.Key] = false
				continue
			}
			if sp.Required {
				return nil, apperr.Validation(labelOf(sp) + " 为必填")
			}
			continue
		}
		if !valueMatchesType(sp.Type, v) {
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

// midScriptParam 是中台 scriptParams 里的单个参数项（实测控制台下发格式）。
// 中台靠 type 决定注入方式（int→裸、string→进引号、array→Lua 表），value 始终是字符串。
type midScriptParam struct {
	Desc     string `json:"desc"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Value    string `json:"value"`
}

// serializeParams 把最终参数序列化为中台 scriptParams 字符串。
//
// 关键（实测控制台抓包）：中台要的是「每个参数带完整定义 + value」的嵌套结构，
// 而不是扁平的 {key:value}：
//
//	{"int_param":{"desc":"...","type":"int","required":true,"value":"11"}, ...}
//
// 中台据此对脚本里的 ${key} 做替换：type 决定注入形态、value 是字符串值。
// 扁平 {key:value} 中台找不到 .value，不会替换，裸 ${} 进 Lua 编译报错。
// 空则返回 ""（透传 omitempty 丢弃）。
func serializeParams(specs []ParamSpec, values map[string]any) string {
	out := make(map[string]midScriptParam, len(specs))
	for _, sp := range specs {
		v, ok := values[sp.Key]
		if !ok || v == nil {
			continue
		}
		desc := sp.Description
		if desc == "" {
			desc = sp.Key
		}
		out[sp.Key] = midScriptParam{
			Desc:     desc,
			Type:     midType(sp.Type),
			Required: sp.Required,
			Value:    luaText(v),
		}
	}
	if len(out) == 0 {
		return ""
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

// luaText 把一个值渲染为「注入脚本的 Lua 字面量文本」（不含字符串外层引号——
// 那由脚本自身的 '${x}' 提供）。string 原样透传（也覆盖 table 已是 Lua 文本的情况）。
func luaText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case []any, map[string]any:
		return luaTableLiteral(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ===== Lua 字面量渲染（array/object → Lua table 文本）=====

func luaTableLiteral(v any) string {
	switch t := v.(type) {
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, luaValue(e))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys) // 稳定顺序
		parts := make([]string, 0, len(t))
		for _, k := range keys {
			parts = append(parts, luaKey(k)+"="+luaValue(t[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return luaValue(v)
	}
}

func luaValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "nil"
	case string:
		return "'" + luaEscapeSingle(t) + "'"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case []any, map[string]any:
		return luaTableLiteral(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// luaKey 合法标识符直接用 k=，否则用 ["k"]=（Lua 双引号字符串键）。
func luaKey(k string) string {
	if paramKeyRe.MatchString(k) {
		return k
	}
	return "[\"" + luaEscapeDouble(k) + "\"]"
}

func luaEscapeSingle(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}

func luaEscapeDouble(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// ===== 顶部注释 schema 提取与解析（真源）=====

// 匹配脚本开头的 --[[ ... ]] 长注释块（首块）。
var schemaCommentRe = regexp.MustCompile(`(?s)^\s*--\[\[(.*?)\]\]`)

// extractSchemaComment 取脚本顶部 --[[ ... ]] 注释里的内容（无则空串）。
func extractSchemaComment(lua string) string {
	m := schemaCommentRe.FindStringSubmatch(lua)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// midParamEntry 是中台 object 格式注释里的单项（+ 我们的扩展字段）。
type midParamEntry struct {
	Desc        string `json:"desc"`
	Description string `json:"description"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	UIType      string `json:"uiType"` // 我们的真实类型（number/table），用于无损回环
	Required    bool   `json:"required"`
	Default     any    `json:"default"`
}

// mapMidType 把中台/通用类型词映射到我们的内部类型。
// 旧的 enum 已移除：未知/enum 一律落 string（旧 enum 脚本退化为普通字符串输入）。
func mapMidType(t string) ParamType {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "int", "integer", "number", "float", "double", "long":
		return ParamNumber
	case "bool", "boolean":
		return ParamBoolean
	case "array", "table", "list", "object", "map":
		return ParamTable // array/object 统一并入 table
	default:
		return ParamString
	}
}

// parseSchema 解析 schema JSON：支持我们的数组格式与中台的 object 格式（保序）。空 → nil。
func parseSchema(jsonText string) ([]ParamSpec, error) {
	jsonText = strings.TrimSpace(jsonText)
	if jsonText == "" {
		return nil, nil
	}
	if strings.HasPrefix(jsonText, "[") {
		return validateSchema(jsonText) // 我们的数组格式
	}
	// 中台 object 格式：按声明顺序解析。
	keys, err := objectKeysInOrder(jsonText)
	if err != nil {
		return nil, apperr.Validation("参数定义不是合法的 JSON 对象")
	}
	var raw map[string]midParamEntry
	if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
		return nil, apperr.Validation("参数定义不是合法的 JSON 对象")
	}
	specs := make([]ParamSpec, 0, len(keys))
	for _, k := range keys {
		e := raw[k]
		ptype := mapMidType(e.Type)
		if e.UIType != "" && validParamTypes[ParamType(e.UIType)] {
			ptype = ParamType(e.UIType) // 优先用我们记录的真实类型
		}
		desc := e.Description
		if desc == "" {
			desc = e.Desc
		}
		specs = append(specs, ParamSpec{
			Key:         k,
			Type:        ptype,
			Required:    e.Required,
			Default:     e.Default,
			Description: desc,
		})
	}
	// 复用数组校验逻辑：序列化回数组再校验。
	norm, _ := json.Marshal(specs)
	return validateSchema(string(norm))
}

// objectKeysInOrder 用流式 decoder 取顶层 object 的 key 顺序（encoding/json 的 map 不保序）。
func objectKeysInOrder(jsonText string) ([]string, error) {
	dec := json.NewDecoder(strings.NewReader(jsonText))
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("not a json object")
	}
	var keys []string
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := kt.(string)
		keys = append(keys, key)
		if err := skipJSONValue(dec); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// skipJSONValue 跳过 decoder 当前位置的一个完整值（含嵌套）。
func skipJSONValue(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); ok && (d == '{' || d == '[') {
		depth := 1
		for depth > 0 {
			tt, err := dec.Token()
			if err != nil {
				return err
			}
			if dd, ok := tt.(json.Delim); ok {
				if dd == '{' || dd == '[' {
					depth++
				} else {
					depth--
				}
			}
		}
	}
	return nil
}

// deriveSchema 从脚本推导 schema：顶部注释为真源；注释缺失时回退到显式 schema 字段。
// 返回 (规范化后的我们数组格式 JSON, specs)；无参数则 ("", nil)。
func deriveSchema(lua, fallbackSchema string) (string, []ParamSpec, error) {
	src := extractSchemaComment(lua)
	if src == "" {
		src = strings.TrimSpace(fallbackSchema)
	}
	if src == "" {
		return "", nil, nil
	}
	specs, err := parseSchema(src)
	if err != nil {
		return "", nil, err
	}
	if len(specs) == 0 {
		return "", nil, nil
	}
	norm, _ := json.Marshal(specs)
	return string(norm), specs, nil
}

// midType 把内部类型映射到中台 scriptParams 里的类型词（见 docs/external_params_example.lua）。
// 中台据此决定 ${} 的注入形态：int→裸数字、bool→裸布尔、array→Lua 表、string→进引号。
func midType(t ParamType) string {
	switch t {
	case ParamNumber:
		return "int"
	case ParamBoolean:
		return "bool"
	case ParamTable:
		return "array"
	default:
		return "string"
	}
}

