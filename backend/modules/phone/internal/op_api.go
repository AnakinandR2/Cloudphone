package phone

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"

	"manager-backend/framework"
	"manager-backend/framework/midplat"

	"github.com/gin-gonic/gin"
)

// opCloudPhone 是操作类 handler 的公共前缀：取登录用户 + 解析 id。
func opCloudPhone(c *gin.Context) (uid, id int, ok bool) {
	uid, authed := currentUserID(c)
	if !authed {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return 0, 0, false
	}
	id, ok = parseID(c)
	return uid, id, ok
}

// PowerCloudPhone 开机 / 关机
// @Summary 开机/关机
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object true "{operation: 开机|关机}"
// @Router /phone/{id}/power [post]
func PowerCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Operation string `json:"operation"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.Power(uid, id, req.Operation); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// RestartCloudPhone 重启
// @Summary 重启云手机
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/restart [post]
func RestartCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	if err := PhoneService.Restart(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// ResetCloudPhone 重置（可换镜像）
// @Summary 重置云手机
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object false "{imageId?}"
// @Router /phone/{id}/reset [post]
func ResetCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		ImageID string `json:"imageId"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.Reset(uid, id, req.ImageID); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// NewDeviceCloudPhone 一键新机（一键刷新）
// @Summary 一键新机
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/new-device [post]
func NewDeviceCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	if err := PhoneService.NewDevice(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// DestroyCloudPhone 销毁实例（并移除本地档案）
// @Summary 销毁云手机
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/destroy [post]
func DestroyCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	if err := PhoneService.Destroy(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// WebRTCAuthCloudPhone 远程控制：申请 WebRTC 接入凭证
// @Summary 申请 WebRTC 凭证
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /phone/{id}/webrtc-auth [post]
func WebRTCAuthCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	info, err := PhoneService.WebRTCAuth(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, info)
}

// WebRTCStateCloudPhone 远程控制：查询串流状态
// @Summary 查询 WebRTC 状态
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/webrtc-state [get]
func WebRTCStateCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	inWebRTC, err := PhoneService.WebRTCState(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"in_webrtc": inWebRTC})
}

// ScreenshotCloudPhone 截屏
// @Summary 截屏
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object false "{format?: png|jpeg}"
// @Router /phone/{id}/screenshot [post]
func ScreenshotCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Format string `json:"format"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.Screenshot(uid, id, req.Format); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// VolumeCloudPhone 调整音量
// @Summary 调整音量
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object true "{volume: 0-100}"
// @Router /phone/{id}/volume [post]
func VolumeCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Volume int `json:"volume"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := PhoneService.SetVolume(uid, id, req.Volume); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// RotateCloudPhone 屏幕旋转
// @Summary 屏幕旋转
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object true "{orientation: landscape|portrait}"
// @Router /phone/{id}/rotate [post]
func RotateCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Orientation string `json:"orientation"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.Rotate(uid, id, req.Orientation); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// ShakeCloudPhone 摇一摇
// @Summary 摇一摇
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/shake [post]
func ShakeCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	if err := PhoneService.Shake(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// InstalledAppsCloudPhone 已安装应用列表
// @Summary 已安装应用
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/apps [get]
func InstalledAppsCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	apps, err := PhoneService.InstalledApps(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, apps)
}

// appOpBody 是安装/卸载/启停应用共用的请求体。
type appOpBody struct {
	AppIDs       []int64  `json:"appIds"`
	PackageNames []string `json:"packageNames"`
}

// InstallAppCloudPhone 安装应用
// @Summary 安装应用
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body appOpBody true "{appIds}"
// @Router /phone/{id}/apps/install [post]
func InstallAppCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req appOpBody
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.InstallApp(uid, id, req.AppIDs); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// UninstallAppCloudPhone 卸载应用
// @Summary 卸载应用
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body appOpBody true "{appIds|packageNames}"
// @Router /phone/{id}/apps/uninstall [post]
func UninstallAppCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req appOpBody
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.UninstallApp(uid, id, req.AppIDs, req.PackageNames); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// StartAppCloudPhone 启动应用
// @Summary 启动应用
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body appOpBody true "{appIds|packageNames}"
// @Router /phone/{id}/apps/start [post]
func StartAppCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req appOpBody
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.StartApp(uid, id, req.AppIDs, req.PackageNames); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// StopAppCloudPhone 停止应用
// @Summary 停止应用
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body appOpBody true "{appIds|packageNames}"
// @Router /phone/{id}/apps/stop [post]
func StopAppCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req appOpBody
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.StopApp(uid, id, req.AppIDs, req.PackageNames); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// KillAllAppsCloudPhone 关闭全部应用
// @Summary 关闭全部应用
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/apps/kill-all [post]
func KillAllAppsCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	if err := PhoneService.KillAllApps(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// AdbInfoCloudPhone 查询 ADB 连接信息（地址 / token / 过期 / 是否开启）
// @Summary ADB 连接信息
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/adb [get]
func AdbInfoCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	info, err := PhoneService.AdbInfo(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, info)
}

// EnableAdbCloudPhone 开启 ADB（签发 Token）；再次调用即续期（签发全新 token）。
// 有效期由中台后台配置固定，无需入参。
// @Summary 开启/续期 ADB
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/adb/enable [post]
func EnableAdbCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	info, err := PhoneService.AdbEnable(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, info)
}

// RunLogsCloudPhone 云手机运行日志（分页，spec §2.9）
// @Summary 运行日志列表
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Router /phone/{id}/run-logs [get]
func RunLogsCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	res, err := PhoneService.RunLogs(uid, id, page, size)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, res.Data, res.TotalSize)
}

// DisableAdbCloudPhone 关闭 ADB
// @Summary 关闭 ADB
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Router /phone/{id}/adb/disable [post]
func DisableAdbCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	if err := PhoneService.AdbDisable(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// RootCloudPhone 开启 / 关闭云手机 root 权限（§3.4.1 update-root）
// @Summary 切换 Root
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object true "{enable: true 开启 / false 关闭}"
// @Router /phone/{id}/root [post]
func RootCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if err := PhoneService.Root(uid, id, req.Enable); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// FileListCloudPhone 列出云手机指定目录下的文件 / 子目录
// @Summary 列云机目录文件
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object false "{path?: 目录绝对路径，默认 /sdcard}"
// @Router /phone/{id}/files/list [post]
func FileListCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Path == "" {
		req.Path = "/sdcard"
	}
	files, err := PhoneService.FileList(uid, id, req.Path)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, files)
}

// FileDownloadCloudPhone 流式下载云手机上的单个文件
// @Summary 下载云机文件
// @Tags 我的云手机
// @Accept json
// @Produce application/octet-stream
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object true "{path: 文件绝对路径}"
// @Router /phone/{id}/files/download [post]
func FileDownloadCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Path == "" {
		framework.Fail(c, http.StatusBadRequest, "path 不能为空")
		return
	}
	data, err := PhoneService.FileDownload(uid, id, req.Path)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", path.Base(req.Path)))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// FileDeleteCloudPhone 删除云手机上的一个或多个文件
// @Summary 删除云机文件
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body object true "{paths: 文件绝对路径数组}"
// @Router /phone/{id}/files/delete [post]
func FileDeleteCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	var req struct {
		Paths []string `json:"paths"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.FileDelete(uid, id, req.Paths); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// FileUploadCloudPhone 上传一个或多个本地文件到云手机的目标目录
// @Summary 上传文件到云机
// @Tags 我的云手机
// @Accept multipart/form-data
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param folderPath formData string false "目标目录，默认 /sdcard/Download"
// @Param files formData file true "一个或多个文件"
// @Router /phone/{id}/files/upload [post]
func FileUploadCloudPhone(c *gin.Context) {
	uid, id, ok := opCloudPhone(c)
	if !ok {
		return
	}
	form, err := c.MultipartForm()
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求需为 multipart/form-data")
		return
	}
	headers := form.File["files"]
	if len(headers) == 0 {
		framework.Fail(c, http.StatusBadRequest, "未选择文件")
		return
	}
	files := make([]midplat.UploadFile, 0, len(headers))
	for _, fh := range headers {
		f, err := fh.Open()
		if err != nil {
			framework.Fail(c, http.StatusInternalServerError, "读取上传文件失败")
			return
		}
		data, err := io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			framework.Fail(c, http.StatusInternalServerError, "读取上传文件失败")
			return
		}
		files = append(files, midplat.UploadFile{Name: fh.Filename, Data: data})
	}
	if err := PhoneService.FileUpload(uid, id, c.PostForm("folderPath"), files); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// ListPhoneTags 列出当前用户已有的云手机标签（去重，供选择）
// @Summary 已有标签列表
// @Tags 我的云手机
// @Produce json
// @Security Bearer
// @Router /phone/tags [get]
func ListPhoneTags(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	tags, err := PhoneService.ListTags(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, tags)
}

// SetPhoneTags 批量设置云手机标签
// @Summary 批量设置标签
// @Tags 我的云手机
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body object true "{ids: number[], tags: string[]}"
// @Router /phone/tags [post]
func SetPhoneTags(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		IDs  []int `json:"ids"`
		Tags []Tag `json:"tags"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := PhoneService.SetTags(uid, req.IDs, req.Tags); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
