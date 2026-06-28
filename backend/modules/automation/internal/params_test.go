package automation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userP = 7100

// commentLua 把数组格式 schema 包成脚本顶部 --[[ ]] 注释（真源），跟上脚本 body。
func commentLua(schemaArray, body string) string {
	return "--[[\n" + schemaArray + "\n]]\n" + body
}

// validateSchema：合法 / 重复 key / 非法 key / 默认值类型不匹配 / 未知类型 / 空。
func TestValidateSchema(t *testing.T) {
	// 空 = 无参数，合法。
	specs, err := validateSchema("")
	require.NoError(t, err)
	assert.Nil(t, specs)

	// 合法（四种类型）。
	specs, err = validateSchema(`[
		{"key":"name","type":"string","required":true},
		{"key":"count","type":"number","default":3},
		{"key":"flag","type":"boolean"},
		{"key":"list","type":"table"}
	]`)
	require.NoError(t, err)
	assert.Len(t, specs, 4)

	// 非法 JSON。
	_, err = validateSchema(`{not array}`)
	require.Error(t, err)

	// 重复 key。
	_, err = validateSchema(`[{"key":"x","type":"string"},{"key":"x","type":"number"}]`)
	require.Error(t, err)

	// 非法 key。
	_, err = validateSchema(`[{"key":"1bad","type":"string"}]`)
	require.Error(t, err)

	// 默认值类型不匹配（number 给字符串）。
	_, err = validateSchema(`[{"key":"n","type":"number","default":"oops"}]`)
	require.Error(t, err)

	// 未知类型（含已移除的 enum）→ 拒绝。
	_, err = validateSchema(`[{"key":"k","type":"weird"}]`)
	require.Error(t, err)
	_, err = validateSchema(`[{"key":"m","type":"enum"}]`)
	require.Error(t, err)
}

// scriptParams 必须是中台控制台格式：每个参数带完整定义 + value（实测抓包）。
// type 决定中台注入形态（int→裸/string→进引号/array→Lua表），value 始终是字符串。
func TestSerializeParamsMidConsoleFormat(t *testing.T) {
	specs := []ParamSpec{
		{Key: "string_param", Type: ParamString, Required: true, Description: "字符串"},
		{Key: "int_param", Type: ParamNumber, Required: true},
		{Key: "bool_param", Type: ParamBoolean, Required: true},
		{Key: "array_param", Type: ParamTable, Required: true},
	}
	values := map[string]any{
		"string_param": "aa", "int_param": float64(11), "bool_param": true,
		"array_param": "{1,2}",
	}
	got := serializeParams(specs, values)
	assert.Contains(t, got, `"string_param":{"desc":"字符串","type":"string","required":true,"value":"aa"}`)
	assert.Contains(t, got, `"int_param":{"desc":"int_param","type":"int","required":true,"value":"11"}`)
	assert.Contains(t, got, `"bool_param":{"desc":"bool_param","type":"bool","required":true,"value":"true"}`)
	assert.Contains(t, got, `"array_param":{"desc":"array_param","type":"array","required":true,"value":"{1,2}"}`)
}

// 一次性运行：共用参数对每台填同一串；缺省回填 default。
func TestRunNowSharedParamsAndDefaults(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-s1")
	seedPhone(t, userP, "cp-s2")

	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: commentLua(`[{"key":"name","type":"string","required":true},{"key":"greet","type":"string","default":"hello"}]`, "log(1)"),
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-s1", "cp-s2"}, "t",
		map[string]any{"name": "world"}, nil)
	require.NoError(t, err)

	// 两台共用同一串；未提供的 greet 回填 default。中台控制台嵌套格式。
	want := `{"greet":{"desc":"greet","type":"string","required":false,"value":"hello"},"name":{"desc":"name","type":"string","required":true,"value":"world"}}`
	assert.Equal(t, want, f.lastTaskParams["cp-s1"])
	assert.Equal(t, want, f.lastTaskParams["cp-s2"])
}

// 逐台覆盖：override 覆盖共用值，其余台沿用共用。
func TestRunNowPerPhoneOverride(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-o1")
	seedPhone(t, userP, "cp-o2")

	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: commentLua(`[{"key":"name","type":"string","required":true}]`, "log(1)"),
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-o1", "cp-o2"}, "t",
		map[string]any{"name": "shared"},
		map[string]map[string]any{"cp-o2": {"name": "special"}})
	require.NoError(t, err)

	assert.Equal(t, `{"name":{"desc":"name","type":"string","required":true,"value":"shared"}}`, f.lastTaskParams["cp-o1"])
	assert.Equal(t, `{"name":{"desc":"name","type":"string","required":true,"value":"special"}}`, f.lastTaskParams["cp-o2"])
}

// 必填缺失 → 拒绝。
func TestRunNowRequiredMissing(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-r1")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: commentLua(`[{"key":"name","type":"string","required":true}]`, "log(1)"),
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-r1"}, "t", nil, nil)
	require.Error(t, err)
}

// bool 参数即便 required:true 且无 default、运行时不传值，也不报错：缺省为 false。
// （复选框无「未填」态；兼容手写注释如 example 的 bool_param required:true。）
func TestRunNowBoolRequiredDefaultsFalse(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-bl")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: commentLua(`[{"key":"flag","type":"boolean","required":true}]`, "local f=${flag}"),
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-bl"}, "t", nil, nil)
	require.NoError(t, err)
	// 下发的 scriptParams 里 flag 缺省为 false、type 为 bool。
	assert.Contains(t, f.lastTaskParams["cp-bl"], `"flag":{"desc":"flag","type":"bool","required":true,"value":"false"}`)
}

// 无 schema 的脚本：scriptParams 为空（向后兼容）。
func TestRunNowNoSchemaBackwardCompat(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-b1")
	rec, err := Service.CreateUserScript(userP, ScriptInput{Name: "p", LuaContent: "log(1)"})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-b1"}, "t", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "", f.lastTaskParams["cp-b1"], "无 schema 应不下发 scriptParams")
}

// 周期计划：全局 scriptParams = 共用参数 JSON。
func TestCreatePlanGlobalParams(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-pp1")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: commentLua(`[{"key":"store","type":"string","required":true}]`, "log(1)"),
	})
	require.NoError(t, err)

	_, err = Service.CreatePlan(userP, PlanInput{
		ScriptLocalID: rec.ID, Name: "每5分钟", Frequency: "INTERVAL", IntervalValue: 5,
		StartTime: "2026-06-25T00:00:00", EndTime: "2027-06-25T00:00:00",
		CpIDs: []string{"cp-pp1"}, Params: map[string]any{"store": "shop1.com"},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"store":{"desc":"store","type":"string","required":true,"value":"shop1.com"}}`, f.lastPlan.ScriptParams)
}

// 保存脚本时注释里 schema 非法 → 拒绝。
func TestCreateScriptRejectsBadSchema(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	_, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: commentLua(`[{"key":"1bad","type":"string"}]`, "log(1)"),
	})
	require.Error(t, err)
}

// schema 真源 = 脚本顶部 --[[ ]] 注释（中台 object 格式），int→number、bool→boolean 映射，保序。
func TestDeriveSchemaFromComment(t *testing.T) {
	lua := "--[[\n{\n  \"name\": {\"desc\":\"名字\",\"type\":\"string\",\"required\":true},\n" +
		"  \"count\": {\"desc\":\"次数\",\"type\":\"int\"},\n" +
		"  \"flag\": {\"desc\":\"开关\",\"type\":\"bool\"},\n" +
		"  \"tags\": {\"desc\":\"标签\",\"type\":\"array\"}\n}\n]]\n" +
		"local name='${name}'\nlocal count=${count}\n"

	_, specs, err := deriveSchema(lua, "")
	require.NoError(t, err)
	require.Len(t, specs, 4)
	// 保序
	assert.Equal(t, []string{"name", "count", "flag", "tags"},
		[]string{specs[0].Key, specs[1].Key, specs[2].Key, specs[3].Key})
	// 类型映射
	assert.Equal(t, ParamString, specs[0].Type)
	assert.True(t, specs[0].Required)
	assert.Equal(t, ParamNumber, specs[1].Type)  // int → number
	assert.Equal(t, ParamBoolean, specs[2].Type) // bool → boolean
	assert.Equal(t, ParamTable, specs[3].Type)   // array → table
}

// array 值渲染成 Lua table 文本注入 scriptParams。
func TestRunNowRendersLuaTable(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-tbl")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "t", LuaContent: commentLua(`[{"key":"tags","type":"table"}]`, "local a=${tags}"),
	})
	require.NoError(t, err)
	_, err = Service.RunNow(userP, rec.ID, []string{"cp-tbl"}, "t",
		map[string]any{"tags": []any{"a", "b", "c"}}, nil)
	require.NoError(t, err)
	assert.Contains(t, f.lastTaskParams["cp-tbl"], `{'a', 'b', 'c'}`)
}

// table 值若已是 Lua 文本（前端直接编辑），原样透传（不二次渲染）。
func TestRunNowTableAsLuaText(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-tt")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "t", LuaContent: commentLua(`[{"key":"tags","type":"table"}]`, "local a=${tags}"),
	})
	require.NoError(t, err)
	_, err = Service.RunNow(userP, rec.ID, []string{"cp-tt"}, "t",
		map[string]any{"tags": "{1, 2, 3}"}, nil)
	require.NoError(t, err)
	assert.Contains(t, f.lastTaskParams["cp-tt"], `"value":"{1, 2, 3}"`)
}

// object 值渲染成 Lua table（键稳定排序）。
func TestRunNowRendersLuaObject(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-obj")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "o", LuaContent: commentLua(`[{"key":"cfg","type":"table"}]`, "local c=${cfg}"),
	})
	require.NoError(t, err)
	_, err = Service.RunNow(userP, rec.ID, []string{"cp-obj"}, "t",
		map[string]any{"cfg": map[string]any{"k": "v", "n": float64(2)}}, nil)
	require.NoError(t, err)
	assert.Contains(t, f.lastTaskParams["cp-obj"], `{k='v', n=2}`)
}

// 标量按中台控制台格式下发：每项带 type + value（value 是字符串），中台据 type 注入。
func TestRunNowScalarsNested(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-sc")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "s", LuaContent: commentLua(`[{"key":"name","type":"string"},{"key":"count","type":"number"},{"key":"flag","type":"boolean"}]`, "x"),
	})
	require.NoError(t, err)
	_, err = Service.RunNow(userP, rec.ID, []string{"cp-sc"}, "t",
		map[string]any{"name": "world", "count": float64(3), "flag": true}, nil)
	require.NoError(t, err)
	got := f.lastTaskParams["cp-sc"]
	assert.Contains(t, got, `"name":{"desc":"name","type":"string","required":false,"value":"world"}`)
	assert.Contains(t, got, `"count":{"desc":"count","type":"int","required":false,"value":"3"}`)
	assert.Contains(t, got, `"flag":{"desc":"flag","type":"bool","required":false,"value":"true"}`)
}
