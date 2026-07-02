package automation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userNeg 是本文件独享的模拟用户 ID（unit 层，不涉及真实手机号/HTTP 登录）。
// 承接 Plan A 的 params_test.go（TC-14-040~046 已覆盖嵌套 console 格式正向路径），
// 本文件补 TC-14-047/048 两条负向不变式：
//   - TC-14-047：schema 未声明的 key 在下发时被静默过滤（buildParams 容错）。
//   - TC-14-048：serializeParams 只产嵌套 console 格式，绝不产出扁平 {key:value} 或
//     模板 startParamMap（中台 ${} 替换只认嵌套结构的 .value，其余两种格式实测不生效）。
const userNeg = 7200

// TestBuildParamsIgnoresUndeclaredKeys 覆盖 TC-14-047：
// schema 只声明 A，下发同时传 A 与未声明的 B，buildParams 的输出只含 A，B 被静默丢弃
// （不报错、不透传），从而 serializeParams 产出的 scriptParams 里也不会出现 B。
func TestBuildParamsIgnoresUndeclaredKeys(t *testing.T) {
	specs, err := validateSchema(`[{"key":"a","type":"string","required":true}]`)
	require.NoError(t, err)

	out, err := buildParams(specs, map[string]any{
		"a": "declared",
		"b": "undeclared-should-be-dropped",
	})
	require.NoError(t, err)

	// 只含已声明的 a，未声明的 b 完全不出现（既不在 key 里，也不在任何 value 里）。
	assert.Equal(t, map[string]any{"a": "declared"}, out)
	_, hasB := out["b"]
	assert.False(t, hasB, "schema 未声明的 key 必须被过滤，不能透传")
}

// TestRunNowIgnoresUndeclaredParamKeys 覆盖 TC-14-047 的端到端【结果】不变式（经 Service.RunNow）：
// 下发时额外传一个 schema 未声明的 key，最终下发到中台的 scriptParams 串里必须只含已声明的 key。
// 说明：这条端到端断言绑定的是「未声明 key 绝不上线」这个最终结果——该结果由 serializeParams
// 按 specs 逐项取值天然保证；对 buildParams 过滤逻辑本身的直接守护由同文件的
// TestBuildParamsIgnoresUndeclaredKeys（单元层变异测试可证真绑定）负责，二者互补。
func TestRunNowIgnoresUndeclaredParamKeys(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userNeg, "cp-neg-undeclared")

	rec, err := Service.CreateUserScript(userNeg, ScriptInput{
		Name:       "neg-undeclared",
		LuaContent: commentLua(`[{"key":"declaredKey","type":"string","required":true}]`, "log(1)"),
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userNeg, rec.ID, []string{"cp-neg-undeclared"}, "t",
		map[string]any{
			"declaredKey":   "kept",
			"undeclaredKey": "must-not-appear",
		}, nil)
	require.NoError(t, err)

	got := f.lastTaskParams["cp-neg-undeclared"]
	assert.Contains(t, got, `"declaredKey":{"desc":"declaredKey","type":"string","required":true,"value":"kept"}`)
	assert.NotContains(t, got, "undeclaredKey", "未声明的 key 不得出现在下发的 scriptParams 中")
	assert.NotContains(t, got, "must-not-appear", "未声明 key 的值不得泄漏进下发串")

	// 反解析确认顶层只有一个 key（declaredKey），不多不少。
	var parsed map[string]midScriptParam
	require.NoError(t, json.Unmarshal([]byte(got), &parsed))
	assert.Len(t, parsed, 1)
	_, ok := parsed["declaredKey"]
	assert.True(t, ok)
}

// TestSerializeParamsNeverFlatFormat 覆盖 TC-14-048（负向核对）：
// serializeParams 的输出必须是嵌套 console 格式 {"key":{"desc","type","required","value"}}，
// 顶层 value 绝不能直接是标量/字符串（那才是扁平 {key:value} 的形状）。
// 中台 ${} 替换只认嵌套结构的 .value 字段；扁平格式中台找不到 .value 不会替换，
// 裸 ${} 进 Lua 编译报错 —— 因此这里断言"顶层字段不是标量"来锁定"没有退化为扁平格式"。
func TestSerializeParamsNeverFlatFormat(t *testing.T) {
	specs := []ParamSpec{
		{Key: "name", Type: ParamString, Required: true},
		{Key: "count", Type: ParamNumber},
		{Key: "flag", Type: ParamBoolean},
	}
	values := map[string]any{
		"name":  "alice",
		"count": float64(3),
		"flag":  true,
	}
	got := serializeParams(specs, values)
	require.NotEmpty(t, got)

	// 若退化为扁平格式，顶层反解析到 map[string]any 时每个 value 会是标量
	// （string/float64/bool），而不是一个带 desc/type/required/value 字段的对象。
	var flatAttempt map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &flatAttempt))
	for k, v := range flatAttempt {
		_, isObject := v.(map[string]any)
		assert.True(t, isObject, "字段 %s 的顶层值必须是对象（嵌套 console 格式），不能是标量（扁平格式）", k)
	}

	// 严格反解析为嵌套结构：每项都必须能取出 desc/type/required/value 四个字段。
	var nested map[string]midScriptParam
	require.NoError(t, json.Unmarshal([]byte(got), &nested))
	require.Len(t, nested, 3)
	// string 参数的 value 应原样以字符串携带（断真实内容，而非仅断类型——item.Value 静态即
	// string，断类型恒真无意义；这里核对渲染后确实携带了输入值）。
	assert.Equal(t, "alice", nested["name"].Value, "string 参数 value 应原样携带")
	for _, sp := range specs {
		item, ok := nested[sp.Key]
		require.True(t, ok, "key %s 必须存在于嵌套结构中", sp.Key)
		assert.NotEmpty(t, item.Type)
		// value 以字符串形式携带（中台约定），且非空——若序列化丢了值这里会 FAIL。
		assert.NotEmpty(t, item.Value, "key %s 的 value 应已渲染携带", sp.Key)
	}

	// 绝不能出现「模板变量」形式的 startParamMap 惯用键名（如中台模板变量惯用的
	// startParamMap/paramMap 之类顶层包裹字段名），后端不应产出这种格式。
	assert.NotContains(t, got, "startParamMap")
	assert.NotContains(t, got, "paramMap")
}

// TestSerializeParamsOutputHasNoBareTopLevelValue 进一步核对 TC-14-048：
// 对单个 string 参数，扁平格式会是 {"name":"alice"}（顶层 value 直接是字符串字面量）；
// 嵌套格式则是 {"name":{"desc":...,"value":"alice"}}。逐字符串核对绝不产出前者。
func TestSerializeParamsOutputHasNoBareTopLevelValue(t *testing.T) {
	specs := []ParamSpec{{Key: "name", Type: ParamString, Required: true}}
	got := serializeParams(specs, map[string]any{"name": "alice"})

	// 扁平格式的字面样式：值紧跟在 key 后面就是被引号包住的字符串本身。
	assert.NotEqual(t, `{"name":"alice"}`, got)
	// 嵌套格式：value 必须是内层对象的一个字段，而不是顶层直接量。
	assert.Contains(t, got, `"value":"alice"`)
	assert.Contains(t, got, `"name":{`)
}
