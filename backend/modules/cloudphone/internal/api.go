package cloudphone

import (
	"net/http"
	"strconv"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

	"github.com/gin-gonic/gin"
)

// notConfigured 在中台未配置时统一返回 503。
func notConfigured(c *gin.Context) {
	framework.Fail(c, http.StatusServiceUnavailable, "云手机中台未配置（缺少 MIDPLAT_BASE_URL/ACCESS_KEY/SECRET_KEY）")
}

// gateway 把中台调用错误统一映射为 502（上游错误）。
func gateway(c *gin.Context, err error) {
	framework.Fail(c, http.StatusBadGateway, err.Error())
}

// ListZones 查询可用区列表。
// @Summary 可用区列表（云手机中台）
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response
// @Router /cloudphone/zones [get]
func ListZones(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	ctx, cancel := callCtx()
	defer cancel()
	zones, err := client.ListAvailZones(ctx)
	if err != nil {
		gateway(c, err)
		return
	}
	framework.OKWithData(c, zones)
}

// phoneSpecRow / vmSpecRow 在原始规格上附带所属可用区（规格本身不含可用区信息）。
// 内嵌结构体的字段会平铺到 JSON 顶层，再补 zoneId / zoneName 两列。
type phoneSpecRow struct {
	midplat.PhoneSpec
	ZoneID   int64  `json:"zoneId"`
	ZoneName string `json:"zoneName"`
}

type vmSpecRow struct {
	midplat.VirtualSpec
	ZoneID   int64  `json:"zoneId"`
	ZoneName string `json:"zoneName"`
}

// ListSpecs 查询规格列表。kind=phone（手机规格，默认）| vm（虚机规格）。
//
// zoneId 可选：不传则聚合「全部可用区」的规格（每条带 zoneId/zoneName 列），
// 传了则只返回该可用区。规格接口是按可用区查询的，这里负责遍历可用区并合并、
// 单个可用区出错时跳过（best-effort），不让一个坏区拖垮整页。
// @Summary 规格列表（默认全部可用区）
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Param kind query string false "phone(默认)|vm"
// @Param zoneId query int false "可用区ID（不传=全部）"
// @Success 200 {object} framework.Response
// @Router /cloudphone/specs [get]
func ListSpecs(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	ctx, cancel := callCtx()
	defer cancel()

	// 先取全部可用区（用于拿到名字 + 确定遍历范围）。
	allZones, err := client.ListAvailZones(ctx)
	if err != nil {
		gateway(c, err)
		return
	}
	zones := allZones
	if z, _ := strconv.ParseInt(c.Query("zoneId"), 10, 64); z != 0 {
		zones = nil
		for _, az := range allZones {
			if az.ID == z {
				zones = append(zones, az)
				break
			}
		}
		if len(zones) == 0 {
			zones = []midplat.AvailZone{{ID: z}}
		}
	}

	if c.DefaultQuery("kind", "phone") == "vm" {
		out := []vmSpecRow{}
		for _, az := range zones {
			list, err := client.ListVirtualSpecsByZone(ctx, az.ID)
			if err != nil {
				continue
			}
			for _, s := range list {
				out = append(out, vmSpecRow{VirtualSpec: s, ZoneID: az.ID, ZoneName: az.Name})
			}
		}
		framework.OKWithData(c, out)
		return
	}

	out := []phoneSpecRow{}
	for _, az := range zones {
		list, err := client.ListPhoneSpecsByZone(ctx, az.ID)
		if err != nil {
			continue
		}
		for _, s := range list {
			out = append(out, phoneSpecRow{PhoneSpec: s, ZoneID: az.ID, ZoneName: az.Name})
		}
	}
	framework.OKWithData(c, out)
}

// ListVMs 查询云主机（租户已分配的云虚机）列表，支持按状态/编号过滤。
// 走 spec §5.4 /server/page（租户维度），不是平台级 /virtual-machine/list。
// @Summary 云主机列表
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Param status query string false "虚机状态（见 §2.16 VM 服务器状态枚举）"
// @Param vmUid query string false "虚机编号（模糊）"
// @Param vmIp query string false "虚机 IP（模糊）"
// @Success 200 {object} framework.Response
// @Router /cloudphone/vms [get]
func ListVMs(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	req := midplat.ListServersRequest{
		VmUID: c.Query("vmUid"),
		VmIP:  c.Query("vmIp"),
	}
	if s := c.Query("status"); s != "" {
		req.VmStatusList = []string{s}
	}
	ctx, cancel := callCtx()
	defer cancel()
	servers, err := client.ListServers(ctx, req)
	if err != nil {
		gateway(c, err)
		return
	}
	framework.OKWithData(c, servers)
}

// ListVMEnums 查询虚机状态枚举（前端筛选项用）。
// @Summary 虚机状态枚举
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response
// @Router /cloudphone/vm-statuses [get]
func ListVMEnums(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	ctx, cancel := callCtx()
	defer cancel()
	out, err := client.ListVMStatuses(ctx)
	if err != nil {
		gateway(c, err)
		return
	}
	framework.OKWithData(c, out)
}

// ListImageList 查询镜像列表（全量）。
// @Summary 镜像列表
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response
// @Router /cloudphone/images [get]
func ListImageList(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	ctx, cancel := callCtx()
	defer cancel()
	out, err := client.ListImages(ctx, midplat.ListImagesRequest{})
	if err != nil {
		gateway(c, err)
		return
	}
	framework.OKWithData(c, out)
}

// ListBootPlans 按云主机规格查可用云手机套餐（规格）。供「云主机」行操作「查看可用云手机规格」用。
// 这些套餐就是在该主机上能创建的云手机规格（core/memory/storage/bandwidth + 分辨率）。
// @Summary 按规格查可用云手机套餐
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Param specId query int true "云主机规格 ID（VirtualMachine.specificationId）"
// @Router /cloudphone/boot-plans [get]
func ListBootPlans(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	specID, _ := strconv.Atoi(c.Query("specId"))
	if specID <= 0 {
		framework.Fail(c, http.StatusBadRequest, "specId 必填")
		return
	}
	ctx, cancel := callCtx()
	defer cancel()
	plans, err := client.ListBootPlansBySpec(ctx, specID)
	if err != nil {
		gateway(c, err)
		return
	}
	framework.OKWithData(c, plans)
}

// ListSpecImages 按云主机规格查可用镜像：取该规格下全部套餐的镜像并集（按 imageId 去重）。
// 供「云主机」行操作「查看可用镜像」用。
// @Summary 按规格查可用镜像
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Param specId query int true "云主机规格 ID（VirtualMachine.specificationId）"
// @Router /cloudphone/spec-images [get]
func ListSpecImages(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	specID, _ := strconv.Atoi(c.Query("specId"))
	if specID <= 0 {
		framework.Fail(c, http.StatusBadRequest, "specId 必填")
		return
	}
	ctx, cancel := callCtx()
	defer cancel()
	plans, err := client.ListBootPlansBySpec(ctx, specID)
	if err != nil {
		gateway(c, err)
		return
	}
	seen := make(map[string]bool)
	out := make([]midplat.PlanImage, 0)
	for _, p := range plans {
		imgs, err := client.ListImagesByPlan(ctx, p.ID)
		if err != nil {
			continue // best-effort：单个套餐查镜像失败不影响整体
		}
		for _, im := range imgs {
			if seen[im.ImageID] {
				continue
			}
			seen[im.ImageID] = true
			out = append(out, im)
		}
	}
	framework.OKWithData(c, out)
}

// ListAppList 查询应用市场列表（可选 appName 过滤，一次取较大页供前端本地分页）。
// @Summary 应用市场列表
// @Tags 云手机资源
// @Produce json
// @Security Bearer
// @Param appName query string false "应用名（模糊）"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(200)
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /cloudphone/apps [get]
func ListAppList(c *gin.Context) {
	if client == nil {
		notConfigured(c)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "200"))
	ctx, cancel := callCtx()
	defer cancel()
	resp, err := client.ListApps(ctx, midplat.ListAppsRequest{
		Page: page, PageSize: size, AppName: c.Query("appName"),
	})
	if err != nil {
		gateway(c, err)
		return
	}
	framework.OKWithPage(c, resp.List, resp.TotalSize)
}
