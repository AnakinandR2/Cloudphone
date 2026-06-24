package mcp

// toolGroupView 是工具清单接口的输出：按领域分组，仅含工具 code。
// 描述与参数详情留给开放 API 文档（www /api-docs），此处只暴露 code 供前端列出。
type toolGroupView struct {
	Group string   `json:"group"`
	Tools []string `json:"tools"`
}

// toolCatalog 按领域分组列出全部工具 code。
// 复用 phoneTools/appTools/scriptTools/proxyTools，与 MCP 注册同一数据源——
// 工具增删/改名只要进了对应分组就自动反映；drift 守卫测试（catalog_test.go）兜底。
func toolCatalog() []toolGroupView {
	names := func(ts []regTool) []string {
		out := make([]string, 0, len(ts))
		for _, t := range ts {
			out = append(out, t.tool.Name)
		}
		return out
	}
	return []toolGroupView{
		{Group: "phone", Tools: names(phoneTools())},
		{Group: "app", Tools: names(appTools())},
		{Group: "script", Tools: names(scriptTools())},
		{Group: "proxy", Tools: names(proxyTools())},
	}
}
