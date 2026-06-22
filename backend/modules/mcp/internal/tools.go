package mcp

import (
	"context"
	"encoding/json"

	"manager-backend/framework/apperr"
	"manager-backend/modules/app"
	"manager-backend/modules/automation"
	"manager-backend/modules/phone"
	"manager-backend/modules/proxy"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// buildServer 构建 MCP 服务并注册全部 18 个工具（与开放 API /api/open/v1 一一对应）。
func buildServer() *server.MCPServer {
	s := server.NewMCPServer(
		"Gloryphone 云手机",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithInstructions("云手机平台开放能力：管理云手机（增删查/电源/重启）、应用（库/商店/装卸）、自动化脚本与任务、代理（增删改查/绑定）。所有操作均限当前 API 密钥所属用户。"),
	)
	for _, t := range allTools() {
		s.AddTool(t.tool, t.handler)
	}
	return s
}

// regTool 把工具定义与处理器配对。
type regTool struct {
	tool    mcp.Tool
	handler server.ToolHandlerFunc
}

// ===== 输出视图（与开放 API 形态一致，便于 AI 直接取 id 喂下一步）=====

type appView struct {
	AppID       int64  `json:"appId"`
	Name        string `json:"name"`
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
	Store       bool   `json:"store"`
	Status      string `json:"status"`
}

type scriptView struct {
	ScriptID    uint   `json:"scriptId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Store       bool   `json:"store"`
}

type proxyView struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Region   string `json:"region"`
	Status   string `json:"status"`
	EgressIP string `json:"egressIp"`
}

func toProxyView(p *proxy.Proxy) proxyView {
	return proxyView{
		ID: int(p.ID), Name: p.Name, Protocol: p.Protocol, Host: p.Host,
		Port: p.Port, Region: p.Region, Status: p.Status, EgressIP: p.EgressIP,
	}
}

// ===== 通用结果/错误辅助 =====

// jsonResult 把任意值序列化为 MCP 文本结果。
func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("序列化结果失败：" + err.Error()), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

// fail 把业务错误转为 MCP 错误结果（isError=true），不抛传输层错误。
func fail(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}

// ok 返回一个简单成功标记。
func ok() (*mcp.CallToolResult, error) {
	return jsonResult(map[string]any{"ok": true})
}

// auth 统一取属主 userID。
func auth(ctx context.Context) (int, *mcp.CallToolResult) {
	uid, err := userIDFrom(ctx)
	if err != nil {
		r := mcp.NewToolResultError(err.Error())
		return 0, r
	}
	return uid, nil
}

// allTools 汇总 18 个工具。拆成几组便于阅读。
func allTools() []regTool {
	var ts []regTool
	ts = append(ts, phoneTools()...)
	ts = append(ts, appTools()...)
	ts = append(ts, scriptTools()...)
	ts = append(ts, proxyTools()...)
	return ts
}

// ===== 云手机 =====

func phoneTools() []regTool {
	return []regTool{
		{
			mcp.NewTool("list_phones",
				mcp.WithDescription("列出当前用户的云手机（含中台实时状态）。"),
				mcp.WithNumber("page", mcp.Description("页码，默认 1")),
				mcp.WithNumber("size", mcp.Description("每页数量，默认 20，最大 200")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				page := req.GetInt("page", 1)
				size := req.GetInt("size", 20)
				if page < 1 {
					page = 1
				}
				if size < 1 || size > 200 {
					size = 20
				}
				list, total, err := phone.List(uid, page, size)
				if err != nil {
					return fail(err)
				}
				return jsonResult(map[string]any{"items": list, "total": total})
			},
		},
		{
			mcp.NewTool("get_phone",
				mcp.WithDescription("查询单台云手机详情（中台实时状态）。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID（来自 list_phones）")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				item, err := phone.GetDisplay(uid, id)
				if err != nil {
					return fail(err)
				}
				return jsonResult(item)
			},
		},
		{
			mcp.NewTool("create_phone",
				mcp.WithDescription("创建一台云手机。镜像与地域由后端自动选择，无需指定。"),
				mcp.WithString("name", mcp.Required(), mcp.Description("云手机名称")),
				mcp.WithNumber("proxyId", mcp.Description("可选，绑定的代理 id（来自 list_proxies）；0 或不填表示不绑定")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				name, err := req.RequireString("name")
				if err != nil {
					return fail(apperr.Validation("name 必填"))
				}
				proxyID := uint(req.GetInt("proxyId", 0))
				item, err := phone.Create(uid, name, "", proxyID)
				if err != nil {
					return fail(err)
				}
				return jsonResult(item)
			},
		},
		{
			mcp.NewTool("destroy_phone",
				mcp.WithDescription("销毁一台云手机（不可恢复，带状态门禁）。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				if err := phone.Destroy(uid, id); err != nil {
					return fail(err)
				}
				return ok()
			},
		},
		{
			mcp.NewTool("power_phone",
				mcp.WithDescription("云手机开机或关机。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
				mcp.WithString("operation", mcp.Required(), mcp.Enum("on", "off"), mcp.Description("on=开机，off=关机")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				var op string
				switch req.GetString("operation", "") {
				case "on":
					op = "开机"
				case "off":
					op = "关机"
				default:
					return fail(apperr.Validation("operation 只能是 on / off"))
				}
				if err := phone.Power(uid, id, op); err != nil {
					return fail(err)
				}
				return ok()
			},
		},
		{
			mcp.NewTool("restart_phone",
				mcp.WithDescription("重启一台云手机。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				if err := phone.Restart(uid, id); err != nil {
					return fail(err)
				}
				return ok()
			},
		},
		{
			mcp.NewTool("bind_proxy",
				mcp.WithDescription("给指定云手机绑定（或更换）代理。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
				mcp.WithNumber("proxyId", mcp.Required(), mcp.Description("代理 id（来自 list_proxies）")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				proxyID := req.GetInt("proxyId", 0)
				if proxyID <= 0 {
					return fail(apperr.Validation("请提供 proxyId"))
				}
				if err := phone.BindProxy(uid, id, uint(proxyID)); err != nil {
					return fail(err)
				}
				return ok()
			},
		},
	}
}

// ===== 应用 =====

func appTools() []regTool {
	return []regTool{
		{
			mcp.NewTool("list_apps",
				mcp.WithDescription("列出可安装的应用：我的应用库 + 应用商店。返回的 appId 用于 install_app。"),
				mcp.WithString("source", mcp.Enum("all", "mine", "store"), mcp.Description("来源筛选，默认 all")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				source := req.GetString("source", "all")
				var apps []app.CustomerApp
				if source == "mine" || source == "all" {
					mine, err := app.List(uid)
					if err != nil {
						return fail(err)
					}
					apps = append(apps, mine...)
				}
				if source == "store" || source == "all" {
					store, err := app.StoreList()
					if err != nil {
						return fail(err)
					}
					apps = append(apps, store...)
				}
				out := make([]appView, 0, len(apps))
				for _, a := range apps {
					out = append(out, appView{
						AppID: a.CpAppID, Name: a.AppName, PackageName: a.PackageName,
						Version: a.Version, Store: a.Store, Status: a.Status,
					})
				}
				return jsonResult(out)
			},
		},
		{
			mcp.NewTool("list_installed_apps",
				mcp.WithDescription("列出某台云手机上已安装的应用。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				apps, err := phone.InstalledApps(uid, id)
				if err != nil {
					return fail(err)
				}
				return jsonResult(apps)
			},
		},
		{
			mcp.NewTool("install_app",
				mcp.WithDescription("在某台云手机上安装应用（按中台 appId，来自 list_apps）。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
				mcp.WithArray("appIds", mcp.Required(), mcp.WithNumberItems(), mcp.Description("要安装的 appId 列表")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				ids := toInt64Slice(req.GetIntSlice("appIds", nil))
				if len(ids) == 0 {
					return fail(apperr.Validation("appIds 必填"))
				}
				if err := phone.InstallApp(uid, id, ids); err != nil {
					return fail(err)
				}
				return ok()
			},
		},
		{
			mcp.NewTool("uninstall_app",
				mcp.WithDescription("在某台云手机上卸载应用，可按 appId 或包名。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
				mcp.WithArray("appIds", mcp.WithNumberItems(), mcp.Description("要卸载的 appId 列表（可选）")),
				mcp.WithArray("packageNames", mcp.WithStringItems(), mcp.Description("要卸载的包名列表（可选）")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				ids := toInt64Slice(req.GetIntSlice("appIds", nil))
				pkgs := req.GetStringSlice("packageNames", nil)
				if len(ids) == 0 && len(pkgs) == 0 {
					return fail(apperr.Validation("appIds 与 packageNames 至少提供一个"))
				}
				if err := phone.UninstallApp(uid, id, ids, pkgs); err != nil {
					return fail(err)
				}
				return ok()
			},
		},
	}
}

// ===== 自动化脚本 / 任务 =====

func scriptTools() []regTool {
	return []regTool{
		{
			mcp.NewTool("list_scripts",
				mcp.WithDescription("列出可运行的脚本：我的脚本 + 商店脚本（仅启用）。返回的 scriptId 用于 run_script。"),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				scripts, err := automation.ListUsableScripts(uid)
				if err != nil {
					return fail(err)
				}
				out := make([]scriptView, 0, len(scripts))
				for _, s := range scripts {
					out = append(out, scriptView{
						ScriptID: s.ID, Name: s.Name, Description: s.Description,
						Version: s.Version, Store: s.Store,
					})
				}
				return jsonResult(out)
			},
		},
		{
			mcp.NewTool("run_script",
				mcp.WithDescription("对某台云手机下发一次性自动化脚本任务，返回 taskId 供 get_task 查询。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("云手机 ID")),
				mcp.WithNumber("scriptId", mcp.Required(), mcp.Description("脚本 id（来自 list_scripts）")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				scriptID := req.GetInt("scriptId", 0)
				if scriptID <= 0 {
					return fail(apperr.Validation("请提供 scriptId"))
				}
				cpID, err := phone.CpIDOf(uid, id)
				if err != nil {
					return fail(apperr.NotFound("云手机不存在"))
				}
				if cpID == "" {
					return fail(apperr.Validation("云手机尚未开通，无法运行脚本"))
				}
				rows, err := automation.RunScript(uid, uint(scriptID), []string{cpID})
				if err != nil {
					return fail(err)
				}
				if len(rows) == 0 {
					return fail(apperr.Internal("任务下发失败"))
				}
				return jsonResult(map[string]any{"taskId": rows[0].MidTaskID, "taskNo": rows[0].TaskNo})
			},
		},
		{
			mcp.NewTool("get_task",
				mcp.WithDescription("查询脚本任务状态；终态时附带日志、截图、结果。"),
				mcp.WithNumber("taskId", mcp.Required(), mcp.Description("任务主键（run_script 返回的 taskId）")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				taskID := req.GetInt("taskId", 0)
				if taskID <= 0 {
					return fail(apperr.Validation("taskId 必填"))
				}
				detail, err := automation.TaskDetail(uid, int64(taskID))
				if err != nil {
					return fail(err)
				}
				return jsonResult(detail)
			},
		},
	}
}

// ===== 代理 =====

func proxyTools() []regTool {
	return []regTool{
		{
			mcp.NewTool("list_proxies",
				mcp.WithDescription("列出我的代理。返回的 id 用于 create_phone 的 proxyId 或 bind_proxy。"),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				list, err := proxy.List(uid)
				if err != nil {
					return fail(err)
				}
				out := make([]proxyView, 0, len(list))
				for i := range list {
					out = append(out, toProxyView(&list[i]))
				}
				return jsonResult(out)
			},
		},
		{
			mcp.NewTool("create_proxy",
				mcp.WithDescription("新增一条代理。"),
				mcp.WithString("name", mcp.Required(), mcp.Description("代理名称")),
				mcp.WithString("protocol", mcp.Required(), mcp.Enum("socks5", "http", "https"), mcp.Description("协议")),
				mcp.WithString("host", mcp.Required(), mcp.Description("主机/IP")),
				mcp.WithNumber("port", mcp.Required(), mcp.Description("端口")),
				mcp.WithString("username", mcp.Description("用户名（可选）")),
				mcp.WithString("password", mcp.Description("密码（可选）")),
				mcp.WithString("region", mcp.Description("地域备注（可选）")),
				mcp.WithString("remark", mcp.Description("备注（可选）")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				in, err := proxyInputFrom(req, true)
				if err != nil {
					return fail(err)
				}
				p, err := proxy.Create(uid, in)
				if err != nil {
					return fail(err)
				}
				return jsonResult(toProxyView(p))
			},
		},
		{
			mcp.NewTool("update_proxy",
				mcp.WithDescription("编辑一条代理（留空字段不更新）。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("代理 ID")),
				mcp.WithString("name", mcp.Description("代理名称")),
				mcp.WithString("protocol", mcp.Enum("socks5", "http", "https"), mcp.Description("协议")),
				mcp.WithString("host", mcp.Description("主机/IP")),
				mcp.WithNumber("port", mcp.Description("端口")),
				mcp.WithString("username", mcp.Description("用户名")),
				mcp.WithString("password", mcp.Description("密码")),
				mcp.WithString("region", mcp.Description("地域备注")),
				mcp.WithString("remark", mcp.Description("备注")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				in, _ := proxyInputFrom(req, false)
				p, err := proxy.Update(uid, id, in)
				if err != nil {
					return fail(err)
				}
				return jsonResult(toProxyView(p))
			},
		},
		{
			mcp.NewTool("delete_proxy",
				mcp.WithDescription("删除一条代理。"),
				mcp.WithNumber("id", mcp.Required(), mcp.Description("代理 ID")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				uid, errRes := auth(ctx)
				if errRes != nil {
					return errRes, nil
				}
				id, err := req.RequireInt("id")
				if err != nil {
					return fail(apperr.Validation("id 必填"))
				}
				if err := proxy.Delete(uid, id); err != nil {
					return fail(err)
				}
				return ok()
			},
		},
	}
}

// proxyInputFrom 从请求构造代理入参；requireCore=true 时校验创建必填项。
func proxyInputFrom(req mcp.CallToolRequest, requireCore bool) (proxy.ProxyInput, error) {
	in := proxy.ProxyInput{
		Name:     req.GetString("name", ""),
		Protocol: req.GetString("protocol", ""),
		Host:     req.GetString("host", ""),
		Port:     req.GetInt("port", 0),
		Username: req.GetString("username", ""),
		Password: req.GetString("password", ""),
		Region:   req.GetString("region", ""),
		Remark:   req.GetString("remark", ""),
	}
	if requireCore && (in.Name == "" || in.Protocol == "" || in.Host == "" || in.Port <= 0) {
		return in, apperr.Validation("name / protocol / host / port 均为必填")
	}
	return in, nil
}

// toInt64Slice 把 []int 转 []int64（facade 的 appId 用 int64）。
func toInt64Slice(in []int) []int64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]int64, len(in))
	for i, v := range in {
		out[i] = int64(v)
	}
	return out
}
