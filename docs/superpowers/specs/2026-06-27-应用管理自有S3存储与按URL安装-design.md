# 应用管理/应用市场改用自有 S3 + 自解析 + 按 URL 安装 — 设计

- 日期：2026-06-27
- 范围：把应用管理（用户上传）与应用市场（管理后台维护）从「上传到中台 OSS、中台解析、按中台 app_id 安装」改为「存我们自己的 S3、我们自己解析、安装时只给中台一个 URL」。用户上传的应用占用素材库容量；存量旧应用废弃、全部重传。
- 前置：素材库（私有桶 + 配额 reserve/confirm/lock + `FileTypeApp`）、`library.OpenFile/OpenFileContent` 门面、中台 `install-by-url` 接口（见 `docs/中台根据URL安装应用接口文档.md`）均已就绪。

## 1. 目标与非目标

### 目标
1. 应用二进制存我们自己的 S3：
   - **用户应用 = 素材库文件**（`file_type=app`），占用素材库配额（解决用户上传治理）。
   - **应用市场 = 平台资产**，存平台公有桶，不计任何用户配额。
2. 我们自己解析 APK/XAPK 元数据：应用名、版本、包名、图标、MD5、大小。
3. 安装只给中台 URL：调 `POST /open/api/vendor/v1/cp/apps/install-by-url`，由中台从该 URL 下载安装。
4. 应用管理页展示素材库容量 + 扩容购买入口。
5. 与素材库一致：相同文件不去重，每个用户维护自己的。

### 非目标（YAGNI）
- 文件去重、断点续传到中台、迁移存量旧应用（直接废弃重传）。
- 安装结果实时回填（异步任务，已安装态以中台已安装列表为准，沿用现有轮询）。
- 自研 AXML 解析（明确使用成熟外部库）。

## 2. 现状（被替换 / 被复用）

### 被替换（旧链路，废弃）
- `backend/modules/app/`：`customer_apps` 表（`Store` 区分用户/市场，`CpAppID` 绑定中台 app_info）。
- 上传：浏览器分片 → 中台 OSS（`InitiateAppUpload/UploadPart/CompleteAppUpload/GetAppInfoFromFile/CreateAppFromUploadedFile/QueryUploadStatus`，秒传命中等）→ 中台解析元数据。
- 安装：`phoneApi.installApp(phoneId,[cpAppId])` → `midplat.InstallApp(cpIds, appIds)`（按中台 app_id）。
- 路由：`/app/list`、`/app/market`、`/app/upload`、`/app/upload/*`、`/admin/apps*`、`/admin/apps/store*`。
- 前端：`my` `AppLibraryView.vue` + `UploadAppDialog.vue`（分片上传到中台）；`RemoteAppPanel.vue`（三页签装机）；`admin` `cloudphone/AppsView.vue`（市场）+ `ops/AppsView.vue`（用户应用治理）。

### 被复用
- 素材库配额：`reserveUpload` / `confirmActiveWithinCapacity` / `lockUsageRow` / `assertNotLocked` / `LibraryUsage`；`PresignUpload` / `ConfirmUpload` 两段式；`classifyFileType`（apk/xapk → `FileTypeApp`）。
- 门面：`library.OpenFile`（presigned GET）、`library.OpenFileContent`（GetObject 流）。
- 平台 S3：`framework.S3`（公有桶，有 `PublicBaseURL`）；`framework.S3Library`（私有桶）；`Client.GetObject/PutObject/PresignGetURL`。
- 中台 SDK 保留：`UninstallApp`（按包名）、`GetInstalledApps`、`StartApp`、`StopApp`。
- 素材库容量/扩容前端：`libraryApi.overview` + `PackagePanel.vue` + 容量条/锁定态。

## 3. 中台 install-by-url 契约（关键）
`POST /open/api/vendor/v1/cp/apps/install-by-url`（AKSK 签名，异步）。
- 请求：`{ cpIds: string[], apps: [{ appName?, downloadUrl(必填, http/https), md5(必填,32hex), packageName(必填), version(必填), fileSize? }] }`
- 响应：`{ code:"SUCCESS", data:{ taskInfoList:[{taskId, instanceId}] } }`
- 重要：**不写入中台应用库、不创建 app_info**（纯下发，无状态）。→ 新模型彻底去掉中台 app 注册 / cpAppId / CREATING-NORMAL 状态机。
- 约束：`downloadUrl` 必须能被中台底层安装环境访问；`md5` 必须与文件一致；仅支持小西云手机；只能操作本租户有权限的云手机。

## 4. 数据模型（新）

### 4.1 用户应用元数据 `app_user_meta`
素材库文件的旁挂元数据，一对一。
```go
type AppUserMeta struct {
    LibraryFileID uint   `gorm:"primaryKey"`            // = library_files.id（属主由 library 保证）
    UserID        uint   `gorm:"index;not null"`
    PackageName   string `gorm:"size:255;index"`
    Version       string `gorm:"size:64"`
    AppName       string `gorm:"size:255"`             // 解析所得，可与文件名不同
    IconURL       string `gorm:"size:1024"`            // 公有桶图标 URL
    MD5           string `gorm:"size:64;index"`        // 安装必填，服务端权威计算
    ParseStatus   string `gorm:"size:16"`              // parsing|ready|failed
    ParseError    string `gorm:"size:512"`
    CreatedAt, UpdatedAt time.Time
}
```
- 列表/读取：以 `library` 的 app 类型文件为主，`LEFT JOIN app_user_meta`。
- 删除：删素材库文件即应用消失；`app_user_meta` 随 file_id 清理（删除文件时一并删）。

### 4.2 市场应用 `app_market`
平台资产，存公有桶，不计配额。
```go
type AppMarket struct {
    ID          uint   `gorm:"primaryKey"`
    S3Key       string `gorm:"size:512;uniqueIndex"`  // 公有桶对象键
    AppName     string `gorm:"size:255"`
    PackageName string `gorm:"size:255;index"`
    Version     string `gorm:"size:64"`
    IconURL     string `gorm:"size:1024"`
    MD5         string `gorm:"size:64"`               // 安装必填
    FileSize    int64  `gorm:"not null;default:0"`
    ParseStatus string `gorm:"size:16"`               // parsing|ready|failed
    ParseError  string `gorm:"size:512"`
    CreatedAt, UpdatedAt time.Time
}
```

### 4.3 图标
解析出的图标 PNG 上传公有桶 `app-icons/<uuid>.png` → `IconURL = framework.S3.PublicBaseURL + key`。用户应用与市场应用图标都放这里，不占用户配额。

### 4.4 废弃
删除旧 `customer_apps` 模型与其建表/seed；旧中台分片上传 + 注册 + 按 id 安装的代码路径移除。

## 5. APK/XAPK 解析（`modules/app/internal/apkparse`）

外部库：`github.com/shogo82148/androidbinary`（`apk` 子包）——成熟纯 Go，解析 manifest（package / versionName / app label）+ 图标。**明确采用外部库，不自研 AXML。**

- **APK**（zip）：`apk.OpenFile(path)` → `Manifest()` 取 `Package`、`VersionName`、label（`Instance().Label()` 解析 resources），`Icon()` 取最佳分辨率图标 → 编码 PNG。
- **XAPK**（zip）：读 `manifest.json`（`package_name` / `version_name` / `name` / `icon`），图标取包内 `icon.png`；若 manifest 缺字段，回退解析包内 base apk（用上面的 APK 路径）。
- **MD5 + 大小**：流式 `io.Copy` 到 md5 + 计数，权威服务端计算（不信前端）。
- 解析接口：
```go
type ParsedApp struct {
    PackageName string
    Version     string
    AppName     string
    IconPNG     []byte // 可空（无图标）
    MD5         string
    SizeBytes   int64
}
// 入参用 *os.File（实现 io.ReaderAt，androidbinary 需要）或临时文件路径。
func Parse(path string, ext string) (ParsedApp, error)
```
- 解析需要 `io.ReaderAt + size`：传临时文件路径（落 scratch / `os.CreateTemp`），解析后删除。

> 依赖拉取：`go get github.com/shogo82148/androidbinary` 经 GOPROXY。若代理拉取失败属实施期阻塞项，需先解决（本设计明确不走自研 AXML 兜底）。

## 6. 上传与解析流程

### 6.1 用户应用（占配额，私有素材库桶）
1. 前端复用素材库上传：`library.presign`（`reserveUpload` 配额校验）→ PUT 到 S3Library → `library.confirm`（`confirmActiveWithinCapacity`，落 `used_bytes`）。仅允许 `.apk/.xapk`（`classifyFileType` → `FileTypeApp`）。
2. confirm 成功后调用新接口 **finalize**：`POST /app/user/:fileId/finalize`
   - 服务端 `library.OpenFileContent(userID, fileID)` 回读对象 → 落临时文件 → `apkparse.Parse` 算 MD5 + 元数据 + 图标。
   - 图标 PNG 上传公有桶 → `IconURL`。
   - 写 `app_user_meta`（`parse_status=ready`）；失败写 `failed` + `parse_error`。
   - 解析较慢：finalize 可同步执行（apk 解析仅读 manifest + 单图标，秒级），列表先显示 `parsing`，finalize 返回后转 `ready`。
3. 列表 `GET /app/user`：`library` app 文件 join `app_user_meta`，返回名称/版本/包名/图标/大小/解析状态/上传时间。
4. 删除：`DELETE` 走素材库删除（释放配额）+ 删 `app_user_meta`。

> 取舍：presigned 直传 → 后端拿不到字节 → finalize 需回读一次对象解析。apk 上传不频繁，可接受。

### 6.2 市场应用（不计配额，公有桶）
1. admin 上传走后端 multipart：`POST /admin/apps/market/upload`（后端直接拿字节，避免回读）。
2. 后端：落临时文件 → `apkparse.Parse` → PutObject 到公有桶 + 图标 → 写 `app_market`（`ready`/`failed`）。
3. 列表 `GET /admin/apps/market`（admin）、`GET /app/market`（用户端浏览，只读）。
4. 删除 `POST /admin/apps/market/batch-delete`：删公有桶对象 + 行。

## 7. 安装流程（替换旧安装）

### 7.1 SDK 新方法
`framework/midplat/cloud_phone_app.go`：
```go
type InstallByURLApp struct {
    AppName     string `json:"appName,omitempty"`
    DownloadURL string `json:"downloadUrl"`
    MD5         string `json:"md5"`
    PackageName string `json:"packageName"`
    Version     string `json:"version"`
    FileSize    string `json:"fileSize,omitempty"`
}
type InstallByURLRequest struct {
    CpIDs []string          `json:"cpIds"`
    Apps  []InstallByURLApp `json:"apps"`
}
func (c *Client) InstallAppByURL(ctx, req InstallByURLRequest) (*InstallAppResponse, error)
// POST /open/api/vendor/v1/cp/apps/install-by-url；复用现有 AKSK 签名 doJSON；复用 InstallAppResponse(taskInfoList)。
```

### 7.2 app 门面解析安装载荷
`modules/app/app.go`：
```go
type AppRef struct { Source string; ID uint } // source: "user"(library_file_id) | "market"(app_market.id)
type InstallSpec struct { AppName, DownloadURL, MD5, PackageName, Version, FileSize string }
func ResolveInstallSpecs(userID int, refs []AppRef) ([]InstallSpec, error)
```
- user：校验 `app_user_meta.parse_status=ready` → `library.OpenFileWithTTL(userID, fileID, installTTL)` 取 presigned GET → 组 InstallSpec（md5/pkg/version/appName/fileSize 来自 meta）。该门面做属主 + 超额锁定校验（同 `OpenFile`），仅多一个 TTL 入参（私有桶 key 不出 library，app 不能自行 presign，故必须由 library 门面提供 TTL 入参版本）。
- market：`app_market` 取行 → `downloadUrl = PublicBaseURL + s3_key`（公有永久可达）。
- 任一未就绪/不存在/锁定 → 整请求失败。

### 7.3 phone 安装入口
`phone` 模块新增（替换旧按 id 安装）：`POST /phone/apps/install-by-url`
- body：`{ phone_ids:[]int, apps:[{source, id}] }`
- handler：取 uid；逐个手机属主校验 + 解析 cpIds；调 `app.ResolveInstallSpecs(uid, refs)`；调 `midplat.InstallAppByURL({cpIds, apps})`；返回 `taskInfoList`（前端提示「已下发」）。
- 卸载、已安装列表沿用现有 `phoneApi.uninstallApp` / `phoneApi.apps`。

### 7.4 presign TTL
用户应用安装 URL 必须够中台下载完成。新增可配 `S3_LIBRARY_INSTALL_GET_TTL`（默认放大，如 1h），由 library 暴露 `OpenFileWithTTL(userID, fileID, ttl)`（`OpenFile` 可改为内部委托给它、传默认 TTL），`ResolveInstallSpecs` 传 install TTL 而非默认 15m。市场应用走公有 URL 无 TTL 问题。

## 8. 前端

### 8.1 my / 应用管理（AppLibraryView 重写）
- 顶部：素材库容量条 + 锁定态 + 「扩容/管理套餐」入口（复用 `libraryApi.overview` + `PackagePanel`，与 `LibraryView` 一致）。
- 列表：`appApi.userList()`（library app 文件 + meta）：图标/应用名/包名/版本/大小/解析状态/上传时间；状态徽章 解析中(脉冲)/就绪/失败(红)。
- 上传：复用素材库 presign/confirm（限 apk/xapk）+ 调 finalize；超额锁定时禁用并提示扩容。
- 删除：调素材库删除。

### 8.2 my / RemoteAppPanel（装机改造）
- 三页签不变：我的应用（`appApi.userList`）/ 应用市场（`appApi.market`）/ 已安装。
- 安装：改调 `phoneApi.installByUrl(phoneIds, refs)`（refs 带 source）。仅 `ready` 可安装；返回 taskInfoList → toast「已下发」。群控广播沿用现状。
- 卸载/已安装沿用现状。

### 8.3 admin / 应用市场（cloudphone/AppsView）
- 上传走后端 multipart → 平台桶 + 解析；展示解析状态；删除走新接口。

### 8.4 admin / 应用管理（ops/AppsView）
- 跨用户列出用户素材库 app 文件 + meta（含上传者）；只读 + 删除治理（删素材库文件，释放该用户配额）。

### 8.5 类型 / api / i18n
- `my/src/types/app.ts`、`admin/src/types/app.ts`：用 `parse_status` 替换旧 `CREATING/NORMAL`；去 `cpAppId`；`AppRef`。
- `my/src/api/modules/app.ts`：`userList/market/finalize/delete`；`phone.ts` 加 `installByUrl`。
- i18n（zh-CN + en）：解析状态、容量/扩容、安装下发等；清理旧分片上传文案。

## 9. 错误处理
| 场景 | 行为 |
|---|---|
| 解析失败（坏包/缺字段） | `parse_status=failed`，列表红标，禁止安装 |
| 素材库超额锁定 | 上传/安装被 library 门面拦截，toast 扩容 |
| 安装未就绪应用 | `ResolveInstallSpecs` 前置失败 |
| 中台失败（非小西/状态/URL 不可达/MD5 不符） | 透传中台 message，前端提示 |
| presigned URL 过期（中台下载慢） | 调大 `S3_LIBRARY_INSTALL_GET_TTL` |
| 依赖拉取失败（androidbinary） | 实施期阻塞项，先解决 GOPROXY，不走自研兜底 |

## 10. 测试
- `apkparse`：小 apk + xapk 夹具，断言 package/version/label/icon 非空 + md5/size。
- finalize：回读解析 + 写 meta（ready/failed）；配额联动用 library `InitForTest`。
- `ResolveInstallSpecs`：user/market/未就绪拒绝/锁定拒绝/不存在拒绝；presign TTL 取安装专用。
- `InstallAppByURL`：请求体字段正确、cpIds 去重、响应解析。
- phone install-by-url：属主校验、cpIds 解析、调用载荷、taskInfoList 透传。
- 前端：`my` 与 `admin` `pnpm build`。

## 11. 实施顺序（单 spec，分阶段）
1. SDK `InstallAppByURL` + `apkparse` 包（外部库 + 夹具单测）。
2. 数据模型 `app_user_meta` / `app_market`；删除旧 `customer_apps` 与旧中台上传/注册/按 id 安装代码。
3. 用户应用：finalize 解析 + 配额联动 + 列表/删除接口。
4. 市场应用：admin multipart 上传 + 解析 + 列表/删除。
5. `ResolveInstallSpecs` + phone `install-by-url` 入口。
6. 前端：my 应用管理（容量+解析）、RemoteAppPanel 安装、admin 市场、ops 治理；types/api/i18n。
7. 验证：`go build/vet/test`；`my`+`admin` `pnpm build`。
