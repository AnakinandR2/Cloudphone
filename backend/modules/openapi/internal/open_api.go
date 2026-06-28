package openapi

import (
	"net/http"
	"strconv"

	"manager-backend/framework"
	"manager-backend/framework/apperr"
	"manager-backend/modules/app"
	"manager-backend/modules/automation"
	"manager-backend/modules/phone"
	"manager-backend/modules/proxy"

	"github.com/gin-gonic/gin"
)

// OpenProxy 是开放 API 的代理视图（id 即创建手机/绑代理用的 proxyId；不含密码）。
type OpenProxy struct {
	ID       int    `json:"id" example:"1"`
	Name     string `json:"name" example:"美西代理1"`
	Protocol string `json:"protocol" example:"socks5"`
	Host     string `json:"host" example:"104.21.45.7"`
	Port     int    `json:"port" example:"1080"`
	Region   string `json:"region" example:"美国·洛杉矶"`
	Status   string `json:"status" example:"ok"`
	EgressIP string `json:"egressIp" example:"49.77.1.1"`
}

func toOpenProxy(p *proxy.Proxy) OpenProxy {
	return OpenProxy{
		ID: int(p.ID), Name: p.Name, Protocol: p.Protocol, Host: p.Host,
		Port: p.Port, Region: p.Region, Status: p.Status, EgressIP: p.EgressIP,
	}
}

// OpenApp 是开放 API 的可安装应用视图：appId 即「装应用」接口 {appIds} 用的值。
type OpenApp struct {
	Source      string `json:"source" example:"user"` // user(我的应用) | market(应用市场)，安装时与 refId 一起传
	RefID       uint   `json:"refId" example:"1092"`  // 来源内的引用 id：user=素材库文件 id，market=市场应用 id
	Name        string `json:"name" example:"微信"`
	PackageName string `json:"packageName" example:"com.tencent.mm"`
	Version     string `json:"version" example:"8.0.49"`
	Store       bool   `json:"store" example:"false"`
	Status      string `json:"status" example:"ready"` // parsing|ready|failed
}

// OpenScript 是开放 API 的可运行脚本视图：scriptId 即「跑脚本」接口 {scriptId} 用的值。
type OpenScript struct {
	ScriptID    uint   `json:"scriptId" example:"1"`
	Name        string `json:"name" example:"每日签到"`
	Description string `json:"description" example:""`
	Version     string `json:"version" example:"1.0.0"`
	Store       bool   `json:"store" example:"false"`
}

// ===== 开放 API 请求体（带 example，便于 Swagger「Try it out」自动填充）=====

type createPhoneReq struct {
	Name    string `json:"name" example:"我的云手机1"`
	ProxyID uint   `json:"proxyId" example:"0"` // 可选，绑定的代理 id（来自 GET /proxies）
}

type createProxyReq struct {
	Name     string `json:"name" example:"美西代理1"`
	Protocol string `json:"protocol" example:"socks5" enums:"socks5,http,https"`
	Host     string `json:"host" example:"104.21.45.7"`
	Port     int    `json:"port" example:"1080"`
	Username string `json:"username" example:""`
	Password string `json:"password" example:""`
	Region   string `json:"region" example:"美国·洛杉矶"`
	Remark   string `json:"remark" example:""`
}

type bindProxyReq struct {
	ProxyID uint `json:"proxyId" example:"1"`
}

type powerReq struct {
	Operation string `json:"operation" example:"on" enums:"on,off"`
}

// installReq 按 URL 安装（自有 S3）：apps 引用「我的应用/应用市场」，来自 list_apps 的 source/refId。
type installReq struct {
	Apps []installAppRef `json:"apps"`
}

type installAppRef struct {
	Source string `json:"source" example:"user"` // user(我的应用) | market(应用市场)
	RefID  uint   `json:"refId" example:"1092"`  // 来源内引用 id：user=素材库文件 id，market=市场应用 id
}

type uninstallReq struct {
	AppIDs       []int64  `json:"appIds" example:"1092"`
	PackageNames []string `json:"packageNames" example:"com.tencent.mm"`
}

type runScriptReq struct {
	ScriptID uint `json:"scriptId" example:"1"`
}

// openCtx 是开放 API handler 公共前缀：取属主用户 + 解析路径 id。
func openCtx(c *gin.Context) (uid, id int, ok bool) {
	uid, authed := currentUserID(c)
	if !authed {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return 0, 0, false
	}
	n, err := strconv.Atoi(c.Param("id"))
	if err != nil || n <= 0 {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return 0, 0, false
	}
	return uid, n, true
}

// OpenListPhones 列出云手机
// @Summary 列出云手机
// @Description 列出当前密钥所属用户的云手机（含中台实时状态）。
// @Tags 云手机
// @Security ApiKey
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Success 200 {object} framework.PageResponse
// @Router /phones [get]
func OpenListPhones(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	list, total, err := phone.List(uid, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// OpenGetPhone 云手机详情
// @Summary 云手机详情
// @Tags 云手机
// @Security ApiKey
// @Produce json
// @Param id path int true "云手机 ID"
// @Success 200 {object} framework.Response
// @Router /phones/{id} [get]
func OpenGetPhone(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	item, err := phone.GetDisplay(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// OpenCreatePhone 创建云手机
// @Summary 创建云手机
// @Tags 云手机
// @Security ApiKey
// @Accept json
// @Produce json
// @Param body body createPhoneReq true "云手机创建参数"
// @Success 200 {object} framework.Response
// @Router /phones [post]
func OpenCreatePhone(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var body createPhoneReq
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	// imageId 由后端自动选择（按可用服务器/套餐），不对外暴露。
	item, err := phone.Create(uid, body.Name, "", body.ProxyID)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// OpenDestroyPhone 销毁云手机
// @Summary 销毁云手机
// @Tags 云手机
// @Security ApiKey
// @Produce json
// @Param id path int true "云手机 ID"
// @Success 200 {object} framework.Response
// @Router /phones/{id} [delete]
func OpenDestroyPhone(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	if err := phone.Destroy(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// OpenPower 开机 / 关机
// @Summary 开机 / 关机
// @Tags 云手机
// @Security ApiKey
// @Accept json
// @Produce json
// @Param id path int true "云手机 ID"
// @Param body body powerReq true "开关机"
// @Success 200 {object} framework.Response
// @Router /phones/{id}/power [post]
func OpenPower(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	var body powerReq
	_ = c.ShouldBindJSON(&body)
	var op string
	switch body.Operation {
	case "on":
		op = "开机"
	case "off":
		op = "关机"
	default:
		framework.Fail(c, http.StatusBadRequest, "operation 只能是 on / off")
		return
	}
	if err := phone.Power(uid, id, op); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// OpenRestart 重启云手机
// @Summary 重启云手机
// @Tags 云手机
// @Security ApiKey
// @Produce json
// @Param id path int true "云手机 ID"
// @Success 200 {object} framework.Response
// @Router /phones/{id}/restart [post]
func OpenRestart(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	if err := phone.Restart(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// OpenListApps 可安装应用列表
// @Summary 可安装应用列表
// @Description 列出可装到云手机的应用：我的应用库 + 应用商店。返回的 appId 用于「装应用」接口。
// @Tags 应用
// @Security ApiKey
// @Produce json
// @Param source query string false "来源筛选" Enums(mine, store, all) default(all)
// @Success 200 {object} framework.Response
// @Router /apps [get]
func OpenListApps(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	source := c.DefaultQuery("source", "all")
	var apps []app.AppListItem
	if source == "mine" || source == "all" {
		mine, err := app.ListUserApps(uid)
		if err != nil {
			framework.FailErr(c, err)
			return
		}
		apps = append(apps, mine...)
	}
	if source == "store" || source == "all" {
		store, err := app.ListMarketApps()
		if err != nil {
			framework.FailErr(c, err)
			return
		}
		apps = append(apps, store...)
	}
	out := make([]OpenApp, 0, len(apps))
	for _, a := range apps {
		out = append(out, OpenApp{
			Source: a.Ref.Source, RefID: a.Ref.ID, Name: a.Name, PackageName: a.PackageName,
			Version: a.Version, Store: a.Store, Status: a.ParseStatus,
		})
	}
	framework.OKWithData(c, out)
}

// OpenListScripts 可运行脚本列表
// @Summary 可运行脚本列表
// @Description 列出可运行的脚本：我的脚本 + 商店脚本（仅启用）。返回的 scriptId 用于「跑脚本」接口。
// @Tags 自动化
// @Security ApiKey
// @Produce json
// @Success 200 {object} framework.Response
// @Router /scripts [get]
func OpenListScripts(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	scripts, err := automation.ListUsableScripts(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	out := make([]OpenScript, 0, len(scripts))
	for _, s := range scripts {
		out = append(out, OpenScript{
			ScriptID: s.ID, Name: s.Name, Description: s.Description,
			Version: s.Version, Store: s.Store,
		})
	}
	framework.OKWithData(c, out)
}

// ===== 代理（闭环：查看 / 添加 / 编辑 / 删除，供绑定云手机）=====

// OpenListProxies 代理列表
// @Summary 代理列表
// @Description 列出我的代理。返回的 id 用于创建云手机的 proxyId 或绑定代理接口。
// @Tags 代理
// @Security ApiKey
// @Produce json
// @Success 200 {object} framework.Response
// @Router /proxies [get]
func OpenListProxies(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := proxy.List(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	out := make([]OpenProxy, 0, len(list))
	for i := range list {
		out = append(out, toOpenProxy(&list[i]))
	}
	framework.OKWithData(c, out)
}

// OpenCreateProxy 新增代理
// @Summary 新增代理
// @Tags 代理
// @Security ApiKey
// @Accept json
// @Produce json
// @Param body body createProxyReq true "代理参数"
// @Success 200 {object} framework.Response
// @Router /proxies [post]
func OpenCreateProxy(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var body createProxyReq
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	p, err := proxy.Create(uid, proxyInput(body))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, toOpenProxy(p))
}

// OpenUpdateProxy 编辑代理
// @Summary 编辑代理
// @Tags 代理
// @Security ApiKey
// @Accept json
// @Produce json
// @Param id path int true "代理 ID"
// @Param body body createProxyReq true "代理参数（留空字段不更新）"
// @Success 200 {object} framework.Response
// @Router /proxies/{id} [put]
func OpenUpdateProxy(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	var body createProxyReq
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	p, err := proxy.Update(uid, id, proxyInput(body))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, toOpenProxy(p))
}

// OpenDeleteProxy 删除代理
// @Summary 删除代理
// @Tags 代理
// @Security ApiKey
// @Produce json
// @Param id path int true "代理 ID"
// @Success 200 {object} framework.Response
// @Router /proxies/{id} [delete]
func OpenDeleteProxy(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	if err := proxy.Delete(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// OpenBindProxy 给云手机绑定/改绑代理
// @Summary 绑定/改绑代理
// @Description 给指定云手机绑定（或更换）代理。proxyId 来自 GET /proxies。
// @Tags 云手机
// @Security ApiKey
// @Accept json
// @Produce json
// @Param id path int true "云手机 ID"
// @Param body body bindProxyReq true "{proxyId}"
// @Success 200 {object} framework.Response
// @Router /phones/{id}/proxy [post]
func OpenBindProxy(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	var body bindProxyReq
	if err := c.ShouldBindJSON(&body); err != nil || body.ProxyID == 0 {
		framework.Fail(c, http.StatusBadRequest, "请提供 proxyId")
		return
	}
	if err := phone.BindProxy(uid, id, body.ProxyID); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// proxyInput 把开放请求体转为 proxy 门面入参。
func proxyInput(b createProxyReq) proxy.ProxyInput {
	return proxy.ProxyInput{
		Name: b.Name, Protocol: b.Protocol, Host: b.Host, Port: b.Port,
		Username: b.Username, Password: b.Password, Region: b.Region, Remark: b.Remark,
	}
}

// OpenApps 已装应用列表
// @Summary 已装应用列表
// @Tags 应用
// @Security ApiKey
// @Produce json
// @Param id path int true "云手机 ID"
// @Success 200 {object} framework.Response
// @Router /phones/{id}/apps [get]
func OpenApps(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	apps, err := phone.InstalledApps(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, apps)
}

// OpenInstall 安装应用
// @Summary 安装应用
// @Tags 应用
// @Security ApiKey
// @Accept json
// @Produce json
// @Param id path int true "云手机 ID"
// @Param body body installReq true "按 URL 安装：apps 引用（source/refId，来自 list_apps）"
// @Success 200 {object} framework.Response
// @Router /phones/{id}/apps/install [post]
func OpenInstall(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	var body installReq
	_ = c.ShouldBindJSON(&body)
	refs := make([]phone.AppRef, 0, len(body.Apps))
	for _, a := range body.Apps {
		refs = append(refs, phone.AppRef{Source: a.Source, ID: a.RefID})
	}
	tasks, err := phone.InstallByURL(uid, []int{id}, refs)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"task_info_list": tasks})
}

// OpenUninstall 卸载应用
// @Summary 卸载应用
// @Tags 应用
// @Security ApiKey
// @Accept json
// @Produce json
// @Param id path int true "云手机 ID"
// @Param body body uninstallReq true "卸载：按 appId 或包名"
// @Success 200 {object} framework.Response
// @Router /phones/{id}/apps/uninstall [post]
func OpenUninstall(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	var body uninstallReq
	_ = c.ShouldBindJSON(&body)
	if err := phone.UninstallApp(uid, id, body.AppIDs, body.PackageNames); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// OpenRunScript 下发脚本任务
// @Summary 下发脚本任务
// @Description 对该云手机下发一次性自动化脚本任务，返回任务主键供查询。
// @Tags 自动化
// @Security ApiKey
// @Accept json
// @Produce json
// @Param id path int true "云手机 ID"
// @Param body body runScriptReq true "脚本 ID"
// @Success 200 {object} framework.Response
// @Router /phones/{id}/run-script [post]
func OpenRunScript(c *gin.Context) {
	uid, id, ok := openCtx(c)
	if !ok {
		return
	}
	var body runScriptReq
	if err := c.ShouldBindJSON(&body); err != nil || body.ScriptID == 0 {
		framework.Fail(c, http.StatusBadRequest, "请提供 scriptId")
		return
	}
	cpID, err := phone.CpIDOf(uid, id)
	if err != nil {
		framework.FailErr(c, apperr.NotFound("云手机不存在"))
		return
	}
	if cpID == "" {
		framework.FailErr(c, apperr.Validation("云手机尚未开通，无法运行脚本"))
		return
	}
	rows, err := automation.RunScript(uid, body.ScriptID, []string{cpID})
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	if len(rows) == 0 {
		framework.FailErr(c, apperr.Internal("任务下发失败"))
		return
	}
	framework.OKWithData(c, gin.H{"taskId": rows[0].MidTaskID, "taskNo": rows[0].TaskNo})
}

// OpenTaskDetail 查脚本任务状态/报告
// @Summary 查脚本任务
// @Description 查任务状态；终态时附带日志、截图、结果。
// @Tags 自动化
// @Security ApiKey
// @Produce json
// @Param taskId path int true "任务主键（run-script 返回的 taskId）"
// @Success 200 {object} framework.Response
// @Router /tasks/{taskId} [get]
func OpenTaskDetail(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil || taskID <= 0 {
		framework.Fail(c, http.StatusBadRequest, "任务 ID 非法")
		return
	}
	detail, err := automation.TaskDetail(uid, taskID)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, detail)
}
