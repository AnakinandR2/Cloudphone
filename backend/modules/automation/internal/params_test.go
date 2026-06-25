package automation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const userP = 7100

// validateSchema：合法 / 重复 key / 非法 key / enum 缺候选 / 默认值类型不匹配 / 空。
func TestValidateSchema(t *testing.T) {
	// 空 = 无参数，合法。
	specs, err := validateSchema("")
	require.NoError(t, err)
	assert.Nil(t, specs)

	// 合法（含全 JSON 类型 + enum）。
	specs, err = validateSchema(`[
		{"key":"name","type":"string","required":true},
		{"key":"count","type":"number","default":3},
		{"key":"flag","type":"boolean"},
		{"key":"mode","type":"enum","options":["a","b"],"default":"a"},
		{"key":"list","type":"array"},
		{"key":"obj","type":"object"}
	]`)
	require.NoError(t, err)
	assert.Len(t, specs, 6)

	// 非法 JSON。
	_, err = validateSchema(`{not array}`)
	require.Error(t, err)

	// 重复 key。
	_, err = validateSchema(`[{"key":"x","type":"string"},{"key":"x","type":"number"}]`)
	require.Error(t, err)

	// 非法 key。
	_, err = validateSchema(`[{"key":"1bad","type":"string"}]`)
	require.Error(t, err)

	// enum 缺候选。
	_, err = validateSchema(`[{"key":"m","type":"enum"}]`)
	require.Error(t, err)

	// enum 默认值不在候选内。
	_, err = validateSchema(`[{"key":"m","type":"enum","options":["a"],"default":"z"}]`)
	require.Error(t, err)

	// 默认值类型不匹配（number 给字符串）。
	_, err = validateSchema(`[{"key":"n","type":"number","default":"oops"}]`)
	require.Error(t, err)

	// 未知类型。
	_, err = validateSchema(`[{"key":"k","type":"weird"}]`)
	require.Error(t, err)
}

// 一次性运行：共用参数对每台填同一串；缺省回填 default。
func TestRunNowSharedParamsAndDefaults(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-s1")
	seedPhone(t, userP, "cp-s2")

	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: "log(1)",
		ParamsSchema: `[{"key":"name","type":"string","required":true},{"key":"greet","type":"string","default":"hello"}]`,
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-s1", "cp-s2"}, "t",
		map[string]any{"name": "world"}, nil)
	require.NoError(t, err)

	// 两台共用同一串；未提供的 greet 回填 default。
	want := `{"greet":"hello","name":"world"}`
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
		Name: "p", LuaContent: "log(1)",
		ParamsSchema: `[{"key":"name","type":"string","required":true}]`,
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-o1", "cp-o2"}, "t",
		map[string]any{"name": "shared"},
		map[string]map[string]any{"cp-o2": {"name": "special"}})
	require.NoError(t, err)

	assert.Equal(t, `{"name":"shared"}`, f.lastTaskParams["cp-o1"])
	assert.Equal(t, `{"name":"special"}`, f.lastTaskParams["cp-o2"])
}

// 必填缺失 → 拒绝。
func TestRunNowRequiredMissing(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-r1")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: "log(1)",
		ParamsSchema: `[{"key":"name","type":"string","required":true}]`,
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-r1"}, "t", nil, nil)
	require.Error(t, err)
}

// enum 取值非法 → 拒绝。
func TestRunNowEnumInvalid(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-e1")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: "log(1)",
		ParamsSchema: `[{"key":"mode","type":"enum","options":["a","b"]}]`,
	})
	require.NoError(t, err)

	_, err = Service.RunNow(userP, rec.ID, []string{"cp-e1"}, "t",
		map[string]any{"mode": "z"}, nil)
	require.Error(t, err)
}

// 无 schema 的老脚本：scriptParams 为空（向后兼容）。
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
		Name: "p", LuaContent: "log(1)",
		ParamsSchema: `[{"key":"store","type":"string","required":true}]`,
	})
	require.NoError(t, err)

	_, err = Service.CreatePlan(userP, PlanInput{
		ScriptLocalID: rec.ID, Name: "每5分钟", Frequency: "INTERVAL", IntervalValue: 5,
		StartTime: "2026-06-25T00:00:00", EndTime: "2027-06-25T00:00:00",
		CpIDs: []string{"cp-pp1"}, Params: map[string]any{"store": "shop1.com"},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"store":"shop1.com"}`, f.lastPlan.ScriptParams)
}

// 保存脚本时 schema 非法 → 拒绝。
func TestCreateScriptRejectsBadSchema(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	_, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "p", LuaContent: "log(1)", ParamsSchema: `[{"key":"1bad","type":"string"}]`,
	})
	require.Error(t, err)
}

// schema 真源 = 脚本顶部 --[[ ]] 注释（中台 object 格式），int→number、bool→boolean 映射，保序。
func TestCreateScriptDerivesSchemaFromComment(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	lua := "--[[\n{\n  \"name\": {\"desc\":\"名字\",\"type\":\"string\",\"required\":true},\n" +
		"  \"count\": {\"desc\":\"次数\",\"type\":\"int\"},\n" +
		"  \"flag\": {\"desc\":\"开关\",\"type\":\"bool\"},\n" +
		"  \"tags\": {\"desc\":\"标签\",\"type\":\"array\"}\n}\n]]\n" +
		"local name='${name}'\nlocal count=${count}\n"
	rec, err := Service.CreateUserScript(userP, ScriptInput{Name: "c", LuaContent: lua})
	require.NoError(t, err)

	specs, err := validateSchema(rec.ParamsSchema)
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
	assert.Equal(t, ParamArray, specs[3].Type)
}

// 注释优先于显式字段（注释为真源）。
func TestCommentOverridesSchemaField(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	lua := "--[[\n{\"fromComment\":{\"type\":\"string\"}}\n]]\nlog(1)"
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "c", LuaContent: lua, ParamsSchema: `[{"key":"fromField","type":"string"}]`,
	})
	require.NoError(t, err)
	specs, err := validateSchema(rec.ParamsSchema)
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, "fromComment", specs[0].Key)
}

// array 值渲染成 Lua table 文本注入 scriptParams。
func TestRunNowRendersLuaTable(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-tbl")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "t", LuaContent: "local a=${tags}",
		ParamsSchema: `[{"key":"tags","type":"array"}]`,
	})
	require.NoError(t, err)
	_, err = Service.RunNow(userP, rec.ID, []string{"cp-tbl"}, "t",
		map[string]any{"tags": []any{"a", "b", "c"}}, nil)
	require.NoError(t, err)
	assert.Contains(t, f.lastTaskParams["cp-tbl"], `{'a', 'b', 'c'}`)
}

// object 值渲染成 Lua table（键稳定排序）。
func TestRunNowRendersLuaObject(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-obj")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "o", LuaContent: "local c=${cfg}",
		ParamsSchema: `[{"key":"cfg","type":"object"}]`,
	})
	require.NoError(t, err)
	_, err = Service.RunNow(userP, rec.ID, []string{"cp-obj"}, "t",
		map[string]any{"cfg": map[string]any{"k": "v", "n": float64(2)}}, nil)
	require.NoError(t, err)
	assert.Contains(t, f.lastTaskParams["cp-obj"], `{k='v', n=2}`)
}

// 标量保持 JSON 原生（字符串原文、数字裸），供 ${} 文本替换。
func TestRunNowScalarsStayNative(t *testing.T) {
	f := &fakeOps{}
	withFakeOps(t, f)
	seedPhone(t, userP, "cp-sc")
	rec, err := Service.CreateUserScript(userP, ScriptInput{
		Name: "s", LuaContent: "x",
		ParamsSchema: `[{"key":"name","type":"string"},{"key":"count","type":"number"},{"key":"flag","type":"boolean"}]`,
	})
	require.NoError(t, err)
	_, err = Service.RunNow(userP, rec.ID, []string{"cp-sc"}, "t",
		map[string]any{"name": "world", "count": float64(3), "flag": true}, nil)
	require.NoError(t, err)
	got := f.lastTaskParams["cp-sc"]
	assert.Contains(t, got, `"name":"world"`)
	assert.Contains(t, got, `"count":3`)
	assert.Contains(t, got, `"flag":true`)
}
