# 中台对外接口规格说明

本文按业务域分组，给出中台 76 条对外接口的标准接口文档：HTTP method、wire path、请求体字段、响应体字段、错误码、调用注意。配套 [midplat-callmap.md](./midplat-callmap.md) 提供路径总览。

## 共通约定

### 1. 鉴权
所有请求需带以下 AKSK 签名头：

| Header | 类型 | 说明 |
|---|---|---|
| `X-Access-Key` | string | 中台分配的 AK |
| `X-Timestamp` | string | Unix 毫秒时间戳 |
| `X-Signature` | string | HMAC-SHA256 签名（按规范拼串）|
| `X-Internal-Request` | string | 内部调用标识 |
| `X-Trace-Id` | string | 请求链路 ID（从 gin ctx 透传） |
| `Content-Type` | string | `application/json` （文件上传为 `multipart/form-data`） |

### 2. 响应外壳

中台 Java 端所有接口统一返回 `CallResult<T>`：

```json
{
  "code": "200",
  "message": "成功",
  "data": <T>,
  "traceId": "abcd-..."
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| code | string | `"200"` 成功；`"500"` 业务失败；其他错误枚举 |
| message | string | 描述信息 |
| data | T \| null | 真正的业务数据（本文以 `data` 一节给出） |
| traceId | string | 与请求 `X-Trace-Id` 对应 |

`code != "200"` 视为业务失败；本文的"响应"一节只描述成功时的 `data` 部分。

### 3. 分页请求

所有分页接口走 `PageRequest`：

```go
type PageRequest struct {
    PageNum  int `json:"page"`      // ⚠️ 字段名是 page 不是 pageNum
    PageSize int `json:"pageSize"`
}
```

⚠️ **错把字段名发成 `pageNum` 中台会静默忽略，回落第 1 页**。

### 4. 分页响应

```go
type PageResponse[T] struct {
    Data      []T   `json:"data"`
    PageNum   int   `json:"pageNum"`   // ⚠️ 响应里是 pageNum
    PageSize  int   `json:"pageSize"`
    TotalSize int64 `json:"totalSize"` // 部分接口叫 total
}
```

### 5. 路径占位符

`{xxx}` 表示路径参数（如 `/img/query/by-plan/{planId}`），直接 URL 拼接，不在 body 里。

### 6. 接口文档模板（**所有端点节按此格式书写**）

每个端点严格遵循以下结构。空块（如无 path 参数 / 无错误码细节）可省略其小节标题，但已有的块要保持顺序。

```
### N.M 接口名（简短描述）

> 一句话说明业务用途。

**接口信息**

| HTTP | POST /open/api/vendor/v1/xxx |
| 中台 controller | XxxController.method（中台 controller 源码锚点）|
| 超时 | 30s（普通）/ 300s（长任务，如文件上传 / 串行下发）|
| 业务时机 | 何时被调（如"前端列表页"/"调度任务每 N 分钟一次"/"创建云手机时同步触发"）|

**请求**

[小节顺序：Path 参数 → Query 参数 → Body 字段 → 示例]

字段表统一列：**字段 / 类型 / 必填 / 描述 / 枚举**

**响应**

成功返回外壳同 §0.2 共通约定，`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |

完整 JSON 响应示例（含外壳）。

**枚举详解**（仅当字段表里引用且枚举值 ≥3 个时单独列）

**错误码**（仅当存在 ≠ 通用 200/500 的特殊枚举时单独列）

**curl 示例**（高频接口提供）
```

#### 6.1 字段表约定

| 列 | 规则 |
|---|---|
| **字段** | wire 上的 JSON 字段名（嵌套对象用 `.` 表示）|
| **类型** | wire 上的 JSON 类型：`int` / `Long → number` / `string` / `string (LocalDateTime)` / `boolean` / `string[]` / `object` |
| **必填**（请求）| `✓`=必填，空=可选 |
| **可空**（响应）| `Y`=可能 null，`N`=保证有值 |
| **描述 / 枚举** | 一句话语义；枚举值 ≤ 2 个直接行内写（如"0=未绑定，1=已绑定"）；枚举值 ≥ 3 个引用下面的**枚举详解**段 |

#### 6.2 字段说明里允许的标记

- 🔒 中台端不对外暴露字段（如 `@Schema(hidden=true)`）
- ⭐ 主力 / 必读字段
- 📜 历史包袱字段（重构窗口期保留，不要新增引用）

#### 6.3 JSON 示例约定

- **必须**给一份**完整成功响应**示例，**含外壳** `code/message/data/traceId`
- 请求示例只展示必填字段 + 1-2 个有代表性的可选字段
- 时间用 LocalDateTime 形态 `"2026-06-03T10:00:00"`（无时区，北京时间）；少量字段是 `"2026-06-03 10:00:00"`（空格分隔），按实际字段标注
- 密码 / Token 一律用 `"******"` 占位

---

## §1 应用管理（9 接口）

### 1.1 应用列表分页

> 后台分页查应用清单，支持按 code / name / package / 创建人模糊搜。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/app/page |
| 中台 controller | [AppController.page](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AppController.java#L92)（`/app/page`） |
| 超时 | 30s |
| 业务时机 | 应用管理列表页打开 / 翻页 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码（≥ 1）|
| pageSize | int | ✓ | 每页条数 |
| appGenerateId | string |  | 应用 code 模糊匹配 |
| appName | string |  | 应用名模糊匹配 |
| packageName | string |  | 包名模糊匹配 |
| createBy | string |  | 创建者模糊匹配 |

请求示例：

```json
{ "page": 1, "pageSize": 20, "appName": "微信" }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| total | number | N | 总记录数 |
| records | object[] | N | 当前页记录列表 |
| records[].id | string | N | 应用 ID（字符串化的 Long，跨语言一致）|
| records[].appCode | string | Y | 应用 code |
| records[].appName | string | N | 应用名 |
| records[].packageName | string | N | 包名 |
| records[].version | string | Y | 版本号 |
| records[].iconUrl | string | Y | 图标 URL |
| records[].fileSize | string | Y | 文件大小（已格式化字符串，如 `"256MB"`，**不是字节数**）|
| records[].createUser | string | Y | 创建人 |
| records[].createTime | string | Y | 创建时间 `"2026-04-12 10:23:45"`（**空格分隔**，不是 `T`）|

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "total": 142,
    "records": [
      {
        "id": "12345",
        "appCode": "wechat-app-xxxx",
        "appName": "微信",
        "packageName": "com.tencent.mm",
        "version": "8.0.42",
        "iconUrl": "https://oss.example.com/icons/wechat.png",
        "fileSize": "256MB",
        "createUser": "alice",
        "createTime": "2026-04-12 10:23:45"
      }
    ]
  },
  "traceId": "abc-123-def"
}
```

<a id="12-delete-apps"></a>
### 1.2 批量删除应用

> 按 ID 列表软删 app_info；md5 引用计数为 0 时异步删 OSS 物理文件。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/app/batch/deleteAppInfo |
| 中台 controller | [AppController.batchDeleteAppInfo](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AppController.java#L69)（`/app/batch/deleteAppInfo`） |
| 超时 | 30s |
| 业务时机 | 应用管理列表勾选删除 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| ids | number[] | ✓ | app_info 主键列表 |

请求示例：

```json
{ "ids": [12345, 12346] }
```

**响应**

`data` 固定为 null。完整 JSON 示例：

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

> ⚠️ **跨租户越权风险**：中台当前未按 tenantId 过滤 ids，任意租户传入的 id 都能软删。引用计数 md5 分组按 OSS 异步清理（失败仅日志，best-effort）。

---

<a id="13-initiate-upload"></a>
### 1.3 启动分片上传会话

> 客户端开始上传前调一次：协商 partSize / totalParts；如果服务端已有同 md5 文件，返回 `uploadSuccess=true` + 完整 appInfo，前端可直接跳到 [§1.8 createFromUploadedFile](#18-create-app)。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/app/upload/initiate-app |
| 中台 controller | [AppController.initiateFileUpload](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AppController.java#L116)（`/app/upload/initiate-app`） |
| 超时 | 30s |
| 业务时机 | 应用上传向导第一步 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| fileName | string | ✓ | 文件名（含扩展名） |
| fileSize | number | ✓ | 文件总字节数 |
| contentMd5 | string |  | 整文件 MD5（秒传依赖此字段） |

请求示例：

```json
{
  "fileName": "wechat-8.0.42.apk",
  "fileSize": 268435456,
  "contentMd5": "5d41402abc4b2a76b9719d911017c592"
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| uploadId | number | Y | 上传会话 ID；秒传命中时为 null |
| partSize | number | N | 服务端分片大小（字节）|
| totalParts | int | N | 总分片数 |
| uploadedParts | object[] | Y | 已上传过的分片记录列表（支持断点续传，每项含 partNumber / etag / contentMd5 / completeTime 等）|
| uploadSuccess | boolean | N | `true`=秒传命中，跳过分片上传直接到 [§1.8](#18-create-app)；`false`=需要继续走分片流程 |
| appInfo | object | Y | 秒传命中时返回已存在的应用信息，否则 null。字段同 [§1.8 响应](#18-create-app) |

完整 JSON 示例（普通流程）：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "uploadId": 100001,
    "partSize": 5242880,
    "totalParts": 52,
    "uploadedParts": [],
    "uploadSuccess": false,
    "appInfo": null
  },
  "traceId": "abc-123"
}
```

秒传命中 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "uploadId": null,
    "partSize": 0,
    "totalParts": 0,
    "uploadSuccess": true,
    "appInfo": {
      "id": 12345,
      "appGenerateId": "wechat-app-xxxx",
      "appName": "微信",
      "packageName": "com.tencent.mm",
      "version": "8.0.42",
      "fileSize": "256MB",
      "fileSizeBytes": 268435456,
      "appMd5": "5d41402abc4b2a76b9719d911017c592",
      "iconPath": "https://oss.example.com/icons/wechat.png",
      "downloadUrl": "https://oss.example.com/wechat-8.0.42.apk",
      "uploadMethod": "LOCAL_UPLOAD",
      "appType": "apk"
    }
  },
  "traceId": "abc-123"
}
```

**枚举：uploadMethod**

| 值 | 含义 |
|---|---|
| `URL_DOWNLOAD` | URL 下载方式录入 |
| `LOCAL_UPLOAD` | 本地分片上传录入 |

**枚举：appType**

| 值 | 含义 |
|---|---|
| `apk` | 标准 APK |
| `xapk` | xapk 复合包 |

---

<a id="14-upload-part"></a>
### 1.4 上传分片

> 在 [§1.3](#13-initiate-upload) 拿到 `partSize/totalParts` 后，按 `partNumber=1..totalParts` 依次（或并发）上传每片二进制。可重传同一 `partNumber` 覆盖之前数据。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/files/upload/part-new |
| 中台 controller | [FileAboutController.uploadFilePartWithMultipart](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/FileAboutController.java#L232)（`/files/upload/part-new`） |
| Content-Type | `multipart/form-data` |
| 超时 | 300s（按单片大小，可配） |
| 业务时机 | 应用上传向导分片循环 |

**请求**

Query 参数：

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| uploadId | number | ✓ | 上传会话 ID（来自 §1.3）|
| partNumber | int | ✓ | 分片号，从 1 起 |
| contentMd5 | string | ✓ | 当前分片 MD5 |

Form 字段：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| file | binary | ✓ | 当前分片的二进制内容 |

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| uploadId | number | N | 回显 |
| partNumber | int | N | 回显分片号 |
| etag | string | N | OSS 分片 etag（合并时需要按顺序提交）|
| completeTime | string | N | 完成时间 |
| success | boolean | N | 单片是否上传成功 |
| uploadSuccess | boolean | Y | 全部分片是否都上传完成（最后一片可能 true）|
| errorMessage | string | Y | 失败时的错误描述 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "uploadId": 100001,
    "partNumber": 1,
    "etag": "\"3858f62230ac3c915f300c664312c11f-1\"",
    "completeTime": "2026-06-03T10:00:15",
    "success": true,
    "uploadSuccess": false,
    "errorMessage": null
  },
  "traceId": "abc-123"
}
```

---

<a id="15-complete-upload"></a>
### 1.5 完成分片合并

> 所有分片上传成功后调一次，触发 OSS 端的 multipart 合并。合并成功返回最终下载 URL；下一步前端调用 [§1.6](#16-parse-app) 解析 APK 元信息。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/files/upload/complete-new |
| 中台 controller | [FileAboutController.completeFileUpload](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/FileAboutController.java#L258)（`/files/upload/complete-new`） |
| 超时 | 300s |
| 业务时机 | 应用上传向导分片全部完成后 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| uploadId | number | ✓ | 上传会话 ID |

请求示例：

```json
{ "uploadId": 100001 }
```

**响应**

`data` 是合并后的 OSS 下载 URL 字符串。完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": "https://oss.example.com/uploads/wechat-8.0.42.apk",
  "traceId": "abc-123"
}
```

---

<a id="16-parse-app"></a>
### 1.6 解析已上传 APK 元信息

> 用 apksig 离线解析 APK 包，回填 appName / packageName / version / icon。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/app/getAppInfoFromFile |
| 中台 controller | [AppController.getAppInfoFromFile](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AppController.java#L124)（`/app/getAppInfoFromFile`） |
| 超时 | 300s（apksig 解析较耗时）|
| 业务时机 | 应用上传向导合并完成后 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| uploadId | number | ✓ | 上传会话 ID |

请求示例：

```json
{ "uploadId": 100001 }
```

**响应**

`data` 字段（同 [§1.8 响应](#18-create-app)）：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| id | number | Y | 解析阶段还未入库时为 null |
| appName | string | N | 解析出的应用名 |
| packageName | string | N | 解析出的包名 |
| version | string | N | 解析出的版本号 |
| md5 | string | Y | 文件 MD5 |
| fileSize | string | Y | 文件大小（格式化字符串，如 `"256MB"`） |
| iconPath | string | Y | 图标 OSS URL（中台自动从 APK 提取并上传 OSS） |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "id": null,
    "appName": "微信",
    "packageName": "com.tencent.mm",
    "version": "8.0.42",
    "md5": "5d41402abc4b2a76b9719d911017c592",
    "fileSize": "256MB",
    "iconPath": "https://oss.example.com/icons/wechat.png"
  },
  "traceId": "abc-123"
}
```

---

<a id="17-check-app-name"></a>
### 1.7 同租户重名校验

> 创建应用前调一次，提示用户改名。租户维度判定（不同租户重名 OK）。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/app/checkAppNameExists |
| 中台 controller | [AppController.checkAppNameExists](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AppController.java#L169)（`/app/checkAppNameExists`） |
| 超时 | 30s |
| 业务时机 | 应用上传向导填写应用名时 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| appName | string | ✓ | 待校验应用名 |

请求示例：

```json
{ "appName": "微信" }
```

**响应**

`data` 是 JSON boolean（不是嵌套对象）。完整 JSON 示例：

```json
{ "code": "200", "message": "成功", "data": true, "traceId": "abc-123" }
```

`true` = 已存在重名应用；`false` = 可以使用。

---

<a id="18-create-app"></a>
### 1.8 根据已上传文件创建应用

> 应用上传向导最后一步：用前面拿到的 uploadId + 用户改过的 appName 等字段，正式落 app_info 表。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/app/createFromUploadedFile |
| 中台 controller | [AppController.createFromUploadedFile](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AppController.java#L132)（`/app/createFromUploadedFile`） |
| 超时 | 300s |
| 业务时机 | 应用上传向导提交 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| uploadId | number | ✓ | 上传会话 ID |
| appName | string | ✓ | 应用名（前端可改 §1.6 解析结果） |
| packageName | string |  | 解析覆盖 |
| version | string |  | 解析覆盖 |
| md5 | string |  | 文件 MD5 |
| fileSize | string |  | 文件大小（格式化字符串）|
| iconPath | string |  | 图标 OSS 路径 |
| originIconPath | string |  | 解析得到的原始图标路径（如有人工换图，记录原始）|
| originAppName | string |  | 解析得到的原始应用名 |
| appDesc | string |  | 应用描述 |
| createBy | string |  | 创建人 |
| openToSubTenant | int |  | 是否对子租户可见：`0`=否，`1`=是 |

请求示例：

```json
{
  "uploadId": 100001,
  "appName": "微信",
  "packageName": "com.tencent.mm",
  "version": "8.0.42",
  "md5": "5d41402abc4b2a76b9719d911017c592",
  "fileSize": "256MB",
  "iconPath": "https://oss.example.com/icons/wechat.png",
  "openToSubTenant": 1
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| id | number | N | 入库后的 app_info 主键 |
| appName | string | N | 应用名 |
| packageName | string | N | 包名 |
| version | string | N | 版本号 |
| md5 | string | N | 文件 MD5 |
| fileSize | string | N | 文件大小 |
| iconPath | string | Y | 图标 URL |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "id": 12345,
    "appName": "微信",
    "packageName": "com.tencent.mm",
    "version": "8.0.42",
    "md5": "5d41402abc4b2a76b9719d911017c592",
    "fileSize": "256MB",
    "iconPath": "https://oss.example.com/icons/wechat.png"
  },
  "traceId": "abc-123"
}
```

---

<a id="19-query-upload-status"></a>
### 1.9 查询上传任务状态

> 前端轮询用：在分片上传 / 合并 / 解析 / 创建任意阶段被中断时，查会话状态。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/app/queryUploadStatus |
| 中台 controller | [AppController.queryUploadStatus](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AppController.java#L163)（`/app/queryUploadStatus`） |
| 超时 | 30s |
| 业务时机 | 前端断点续传 / 上传重试 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| uploadId | number | ✓ | 上传会话 ID |

请求示例：

```json
{ "uploadId": 100001 }
```

**响应**

`data` 是状态字符串。完整 JSON 示例：

```json
{ "code": "200", "message": "成功", "data": "OSS_UPLOADING", "traceId": "abc-123" }
```

**枚举：上传状态**

| 值 | 含义 |
|---|---|
| `OSS_UPLOADING` | 上传中（分片已开始）|
| `OSS_SUCCESS` | 上传完成（合并成功）|
| `OSS_FAILED` | 失败 |

> 实现：读 Redis key `cloudphone:app:oss:{uploadId}`，TTL 24h。key 不存在 → 抛 `"上传任务不存在或已过期"`。

---

## §2 云手机生命周期（16 接口）

### 2.1 创建云手机（按规格）

> 在指定 VM 上按指定方案 / 规格 / 镜像创建 N 台云手机。同步触发底层 VM 编排（异步落地），返回 cpId 列表 + 工作流 ID。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cp/create |
| 中台 controller | [OpenCloudPhoneController.create](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/open/OpenCloudPhoneController.java#L89)（`/cp/create`） |
| 超时 | 30s（创建过程异步） |
| 业务时机 | 控制台"添加云手机"向导 |

**请求**

字段较多，按用途分组。

**身份与数量**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| vmId | string | ✓ | 宿主 VM ID |
| imageId | string |  | 镜像 ID（不传走 plan 默认）|
| planId | int |  | 启动计划 ID |
| cpSpecId | int |  | 规格 ID |
| number | number | ✓ | 创建台数；不传时中台按 `maxBootCount - 当前台数` 自动计算 |
| availZoneId | int |  | 可用区 ID |
| imageName | string |  | 镜像名（回显用，可省略）|
| reCreateFlag | boolean |  | 是否走重建流程 |
| autoStart | boolean |  | 创建后是否自动开机；中台 `/cp/create` 强制覆盖为 `true` |

**资源规格**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| minCore | number | ✓ | 最小 CPU 核心数（可小数）|
| maxCore | number | ✓ | 最大 CPU 核心数 |
| minMemory | int | ✓ | 最小内存（MB）|
| maxMemory | int | ✓ | 最大内存（MB）|
| storage | int | ✓ | 存储（GB）|
| bandwidth | int |  | 网络带宽（Mbps）|
| storageCluster | object |  | 存储集群信息（可省略，中台按 plan 决定）|
| volumeId | string |  | 存储卷 ID |

**启动参数**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| bootParamId | int |  | 启动参数 ID |
| bootParamType | string |  | `SYSTEM`=系统默认，`CUSTOM`=自定义 |
| width | int | ✓ | 分辨率宽 |
| height | int | ✓ | 分辨率高 |
| fps | int | ✓ | 帧率 |
| maxBootCount | int | ✓ | 该 VM 最大可开台数 |
| recommendedBootCount | int |  | 推荐开机台数 |
| maxStartCount | int |  | 并发开机上限 |
| bootParams | object |  | 嵌套启动参数（含 width/height/fps/maxBootCount 等覆盖值）|

**设备 / 镜像**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| phoneModelId | number |  | 手机型号 ID |
| androidVersion | string |  | 安卓版本字符串（如 `"11"`） |
| root | boolean |  | 创建后是否开 root |

**仿真**（顶层平铺，非嵌套）

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| regionOption | string |  | 地区跟随：`FOLLOW_PROXY`=跟随代理，`CUSTOM`=自定义。不传默认 `CUSTOM` |
| timezoneOption | string |  | 时区跟随：同上 |
| languageOption | string |  | 语言跟随：同上 |
| region | string |  | 自定义地区 |
| sysLocale | string |  | 系统语言（如 `"zh-CN"`） |
| timezone | string |  | 时区（如 `"Asia/Shanghai"`） |
| latitudeDegrees | number |  | GPS 纬度 |
| longitudeDegrees | number |  | GPS 经度 |
| networkType | int |  | `0`=WiFi，`1`=移动网络 |
| operatorId | number |  | 运营商 ID |

**代理**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| proxyMode | string |  | 代理方式：`ADDED`=已添加代理，`CUSTOM`=自定义代理，`SHENLONG`=神龙代理 |
| proxy | object |  | 单代理对象（结构见**嵌套：proxy 字段**） |
| proxyId | number |  | 已绑代理 ID |
| proxyIds | number[] |  | 多代理 ID 列表 |
| proxyList | object[] |  | 多代理对象列表 |
| proxyInfo | string[] |  | 代理元信息（已废弃但仍兼容） |
| proxyType | int |  | `0`=HTTP，`1`=SOCKS5 |
| proxyOrderType | string |  | 代理订单类型：`STATIC`/`DYNAMIC`/`TIKTOK`/`CUSTOM` |
| proxyVendor | string |  | 代理供应商：`IPIPGO`/`SHENLONG`/`IPVIBE` 等 |
| mealId | number |  | 代理套餐 ID（购买场景）|
| packageName | string |  | 代理套餐名 |
| mealTime | int |  | 套餐时长（小时） |
| ipList | object[] |  | 国家/地区 + 数量列表（购买场景）|
| ipType | string |  | IP 类型 |
| exclusiveBandwidth | string |  | 独享带宽 |
| flow | number |  | 流量配额 |

**嵌套：proxy 字段**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number |  | 代理 ID |
| enabled | boolean |  | 是否启用（默认 true） |
| host | string |  | 代理主机 |
| port | int |  | 代理端口 |
| username | string |  | 用户名 |
| password | string |  | 密码 |
| type | int |  | `0`=HTTP，`1`=SOCKS5 |
| blacks | string[] |  | 黑名单 IP / 域名（不走代理）|

**应用**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| appIds | int[] |  | 创建时自动安装应用 ID 列表 |

请求示例：

```json
{
  "vmId": "vm-aaa-111",
  "imageId": "img-android11-prod",
  "planId": 23,
  "number": 5,
  "minCore": 1.0, "maxCore": 2.0,
  "minMemory": 2048, "maxMemory": 4096,
  "storage": 16,
  "bandwidth": 50,
  "width": 720, "height": 1280, "fps": 30,
  "maxBootCount": 20,
  "androidVersion": "11",
  "regionOption": "FOLLOW_PROXY",
  "timezoneOption": "FOLLOW_PROXY",
  "languageOption": "FOLLOW_PROXY",
  "networkType": 0,
  "proxyMode": null,
  "appIds": [12345, 12346]
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| cpList | string[] | N | 创建出来的 cpId 列表 |
| workflowId | string | Y | 异步工作流 ID |
| planId | int | Y | 启动计划 ID 回显 |
| planName | string | Y | 启动计划名 |
| imageId | string | Y | 镜像 ID 回显 |
| imageName | string | Y | 镜像名 |
| imageVersion | string | Y | 镜像版本 |
| availZoneId | int | Y | 可用区 ID |
| availZoneName | string | Y | 可用区名 |
| regionId | int | Y | 区域 ID |
| regionName | string | Y | 区域名 |
| supplierId | int | Y | 供应商 ID |
| supplierName | string | Y | 供应商名 |
| cpu | number | Y | 实际分配 CPU |
| memory | int | Y | 实际分配内存（MB）|
| storage | int | Y | 实际分配存储（GB） |
| bandwidth | int | Y | 实际带宽（Mbps） |
| uploadSpeedLimit | int | Y | 上行限速（kbps） |
| downloadSpeedLimit | int | Y | 下行限速（kbps） |
| fps | int | Y | 帧率 |
| width | int | Y | 分辨率宽 |
| height | int | Y | 分辨率高 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "cpList": ["cp-xxx-001", "cp-xxx-002", "cp-xxx-003", "cp-xxx-004", "cp-xxx-005"],
    "workflowId": "wf-create-aaa",
    "planId": 23,
    "planName": "标准 plan",
    "imageId": "img-android11-prod",
    "imageName": "android-11-prod",
    "imageVersion": "1.2.3",
    "availZoneId": 1,
    "availZoneName": "华南-1",
    "regionId": 100,
    "regionName": "华南",
    "supplierId": 5,
    "supplierName": "tencent",
    "cpu": 1.0,
    "memory": 2048,
    "storage": 16,
    "bandwidth": 50,
    "fps": 30,
    "width": 720,
    "height": 1280
  },
  "traceId": "abc-123"
}
```

**枚举：OptionModeEnum（regionOption / timezoneOption / languageOption）**

| 值 | 含义 |
|---|---|
| `FOLLOW_PROXY` | 跟随代理（代理检测后回填）|
| `CUSTOM` | 自定义（用本请求 `region/timezone/sysLocale` 字段）|

**枚举：BootParamTypeEnum（bootParamType）**

| 值 | 含义 |
|---|---|
| `SYSTEM` | 系统默认 |
| `CUSTOM` | 自定义 |

**枚举：ProxyModeEnum（proxyMode）**

| 值 | 含义 |
|---|---|
| `ADDED` | 选已添加的代理 |
| `CUSTOM` | 自定义代理 |
| `SHENLONG` | 神龙代理（特殊供应商通道）|

<a id="22-list-cloud-phones-v1"></a>
### 2.2 v1 云手机分页（兜底用）

> ⚠️ **不是"查询云手机列表"主入口**。主入口是 [§2.16 v2 `/cloud-phone/page`](#216-list-open-cloud-phones)。
> 本端点作为 v1 兜底，仅在 v2 响应里 `brand` / `phoneModel` / `countryName` 字段为空时由 [§2.3](#23-batch-get-phone-specs) / [§2.4](#24-batch-get-sim-country) 触发回填。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cp/page |
| 中台 controller | [OpenCloudPhoneController.pageCloudPhoneList](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/open/OpenCloudPhoneController.java#L221)（`/cp/page`） |
| 超时 | 30s |
| 业务时机 | v1 字段兜底回填（[§2.3](#23-batch-get-phone-specs)/[§2.4](#24-batch-get-sim-country)） |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码（≥ 1）|
| pageSize | int | ✓ | 每页条数 |
| cpId | string |  | 手机编号模糊匹配 |
| cpIds | string[] |  | 手机编号精确列表（IN）|
| filterCpIds | string[] |  | 排除列表（NOT IN）|
| brandModelList | string[] |  | 品牌型号列表（最多 5 个）|
| androidVersionList | string[] |  | 安卓版本列表（最多 5 个）|
| status | string |  | 云手机状态（见**枚举：云手机状态**）|
| statusList | string[] |  | 云手机状态多选 |
| vmUid | string |  | 服务器 uid 精确 |
| isMaintain | boolean |  | `false`=非维护，`true`=维护中 |
| bindStatus | int |  | `0`=未绑定，`1`=已绑定 |
| expiredStatus | string |  | `normal` / `expired` |
| expiredCpIds | string[] |  | 当 `expiredStatus=expired` 时合并这些手机的关联关系 |
| inWebrtc | boolean |  | 是否在 WebRTC 中 |
| tenantCpIds | string[] | 🔒 | 中台内部使用 |
| tenantVmUids | string[] | 🔒 | 中台内部使用 |

请求示例：

```json
{ "page": 1, "pageSize": 50, "cpIds": ["cp-xxx-001"] }
```

**响应**

`data` 字段：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "data": [
      {
        "cpId": "cp-xxx-001",
        "brand": "Samsung",
        "phoneModel": "Galaxy S22",
        "deviceModelCode": "SM-S908E",
        "androidVersion": "11",
        "width": 720,
        "height": 1280,
        "minCore": 1.0,
        "maxCore": 2.0,
        "minMemory": 2048,
        "maxMemory": 4096,
        "fps": 30,
        "storage": 16,
        "bandwidth": 50,
        "countryName": "美国",
        "status": "NORMAL",
        "isRooted": false,
        "isMaintain": false,
        "bindStatus": 1,
        "expired": false,
        "expireTime": "2026-12-31T23:59:59",
        "createTime": "2026-05-12T08:23:45",
        "vmId": "vm-aaa-111",
        "vmUid": "vm-uid-xxx",
        "cpSource": "SELF",
        "zoneId": 1,
        "inWebrtc": false,
        "isGroupControl": false,
        "webrtcCount": 0,
        "webrtcChannelsNum": 3,
        "scriptTaskStatus": "IDLE",
        "luaBaseLibVersion": "1.0.5",
        "proxyInfo": {
          "id": 9001,
          "proxyType": "socks5",
          "proxyOrderType": "静态",
          "ip": "us-proxy.ipvibe.io",
          "port": 1080,
          "username": "tenant-7-key",
          "password": "******",
          "country": "US",
          "countryName": "United States",
          "province": "California",
          "provinceEn": "California",
          "city": "Los Angeles",
          "cityEn": "Los Angeles",
          "detectionTime": "2026-06-03T10:00:00",
          "egressIp": "1.2.3.4",
          "testingResult": 1,
          "expireTime": "2026-12-31T23:59:59"
        }
      }
    ],
    "pageNum": 1,
    "pageSize": 20,
    "totalSize": 142
  },
  "traceId": "abc-123-def"
}
```

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| pageNum | int | N | 当前页码 |
| pageSize | int | N | 每页条数 |
| totalSize | number | N | 总记录数 |
| data | object[] | N | 当前页云手机列表 |
| data[].cpId | string | N | 云手机实例编号 |
| data[].brand | string | Y | 手机品牌 |
| data[].phoneModel | string | Y | 手机型号 |
| data[].deviceModelCode | string | Y | 设备型号代码 |
| data[].androidVersion | string | Y | 安卓版本（镜像版本）|
| data[].width | int | N | 分辨率宽（像素）|
| data[].height | int | N | 分辨率高（像素）|
| data[].minCore | number | N | 最小 CPU 核心数（可小数）|
| data[].maxCore | number | N | 最大 CPU 核心数 |
| data[].minMemory | int | N | 最小内存（MB）|
| data[].maxMemory | int | N | 最大内存（MB）|
| data[].fps | int | N | 帧率（10/15/30/60）|
| data[].storage | int | N | 存储空间（GB）|
| data[].bandwidth | int | N | 网络带宽（Mbps）|
| data[].countryName | string | Y | 仿真国家（中文）|
| data[].status | string | N | 云手机状态（见**枚举：云手机状态**）|
| data[].isRooted | boolean | Y | `false`=未 root，`true`=已 root |
| data[].isMaintain | boolean | Y | `false`=非维护，`true`=维护中 |
| data[].bindStatus | int | Y | `0`=未绑定，`1`=已绑定 |
| data[].expired | boolean | Y | 是否已过期 |
| data[].expireTime | string | Y | 过期时间 `"2026-12-31T23:59:59"`（无时区，北京时间）|
| data[].createTime | string | N | 创建时间 |
| data[].vmId | string | N | 虚拟机 ID |
| data[].vmUid | string | N | 虚拟机 UID |
| data[].cpSource | string | Y | 🔒 云手机来源 |
| data[].zoneId | int | Y | 🔒 可用区 ID |
| data[].inWebrtc | boolean | Y | 是否在 WebRTC 中 |
| data[].isGroupControl | boolean | Y | 是否群控 |
| data[].webrtcCount | int | Y | 当前 WebRTC 连接数 |
| data[].webrtcChannelsNum | int | Y | WebRTC 通道上限 |
| data[].scriptTaskStatus | string | Y | `BUSY`=任务中，`IDLE`=空闲 |
| data[].luaBaseLibVersion | string | Y | Lua 基础库版本 |
| data[].proxyInfo | object | Y | 代理信息（结构见**嵌套：代理信息**）|

**枚举：云手机状态**

| 值 | 含义 | 值 | 含义 |
|---|---|---|---|
| `NORMAL` | 已开机 | `INITIALIZING` | 正在初始化 |
| `STARTING` | 正在开机 | `INIT_FAILED` | 初始化失败 |
| `STOPPING` | 正在关机 | `DISTROYING` 📜 | 正在销毁（拼写错误，wire 协议已固化）|
| `STOPPED` | 已关机 | `MAINTAINING` | 维护中 |
| `REBOOTING` | 正在重启 | `VM_RESETTING` | 服务器重置中 |
| `RESETTING` | 正在重置 | `VM_OFFLINE` | 服务器离线 |
| `REPLACING` | 换机中 | `VM_UPGRADING` | 服务器升级 |
| `UPDATING` | 正在更新 | `AGENT_OFFLINE` | 控制器离线 |
| `FAULTED` | 异常 | `DESTROYED` | 已销毁 |

**嵌套：代理信息**

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| id | number | N | 代理主键 ID |
| proxyType | string | N | 代理协议：`http` / `socks5`（小写）|
| proxyOrderType | string | Y | 代理订单类型（中文）：`动态` / `静态` / `TIKTOK` |
| ip | string | N | 代理 IP / 域名 |
| port | int | N | 端口 |
| username | string | Y | 代理用户名 |
| password | string | Y | 代理密码（密码可见性由中台脱敏策略决定）|
| country | string | Y | 国家 ISO 代码（如 `US`）|
| countryName | string | Y | 国家名（英文）|
| province / provinceEn | string | Y | 省份（中文 / 英文）|
| city / cityEn | string | Y | 城市（中文 / 英文）|
| detectionTime | string | Y | 上次检测时间 |
| egressIp | string | Y | 出口 IP |
| testingResult | int | Y | `1`=成功，`0`=失败 |
| expireTime | string | Y | 代理到期时间 |

<a id="23-batch-get-phone-specs"></a>
### 2.3 批量获取手机规格（v1 兜底）

> 当 [§2.16 v2 `/cloud-phone/page`](#216-list-open-cloud-phones) 响应里 `brand` / `phoneModel` / `deviceModelCode` 字段为空时，用此 v1 接口兜底回填。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cp/page |
| 中台 controller | 同 [§2.2](#22-list-cloud-phones-v1) |
| 超时 | 30s |
| 业务时机 | v2 字段回填触发 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| pageNum | int | ✓ | 页码（**注意 v1 兜底场景这里用 `pageNum` 不是 `page`**） |
| pageSize | int | ✓ | 每页条数（≤ 200） |
| cpIds | string[] | ✓ | 待回填规格的 cpId 列表 |

请求示例：

```json
{ "pageNum": 1, "pageSize": 200, "cpIds": ["cp-aaa", "cp-bbb"] }
```

**响应**

响应结构同 [§2.2](#22-list-cloud-phones-v1)，但只读取下面 5 个字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data[].brand | string | Y | 品牌 |
| data[].phoneModel | string | Y | 型号 |
| data[].deviceModelCode | string | Y | 设备型号代码 |
| data[].androidVersion | string | Y | 安卓版本 |
| data[].countryName | string | Y | 仿真国家（中文）|

> 调用方仅当该 cpId 至少有一项非空时才回填本地，避免覆盖 v2 已有字段。

---

<a id="24-batch-get-sim-country"></a>
### 2.4 批量获取 SIM 国家（v1 兜底）

> 当 [§2.16 v2 `/cloud-phone/page`](#216-list-open-cloud-phones) 响应里 `countryName` 字段为空时，用此 v1 接口兜底回填。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cp/page |
| 中台 controller | 同 [§2.2](#22-list-cloud-phones-v1) |
| 超时 | 30s |
| 业务时机 | v2 字段回填触发 |

**请求 / 响应**

请求体与 [§2.3](#23-batch-get-phone-specs) 完全相同（`pageNum` / `pageSize` / `cpIds`）。响应只读 `data[].countryName` 一个字段（其它字段不消费）。

<a id="25-start"></a>
### 2.5 批量开机

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cp/start |
| 中台 controller | [OpenCloudPhoneController.start](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/open/OpenCloudPhoneController.java#L152)（`/cp/start`） |
| 超时 | 30s |
| 业务时机 | 列表勾选 → 批量开机 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云手机 ID 列表 |

请求示例：

```json
{ "cpIds": ["cp-aaa", "cp-bbb"] }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| workflowIds | string[] | N | 异步工作流 ID 列表（每台机一条，前端可后续轮询）|

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": { "workflowIds": ["wf-001", "wf-002"] },
  "traceId": "abc-123"
}
```

---

<a id="26-shutdown"></a>
### 2.6 批量关机

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cp/shutdown |
| 中台 controller | [OpenCloudPhoneController.shutdown](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/open/OpenCloudPhoneController.java#L185)（`/cp/shutdown`） |
| 超时 | 30s |

请求 / 响应同 [§2.5](#25-start)。

---

<a id="27-restart"></a>
### 2.7 批量重启

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/restart |
| 中台 controller | [CloudPhoneController.restart](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L239)（`/cloud-phone/restart`，**注意不是 `/cp/restart`**） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 列表 |
| appIds | int[] |  | 重启后强制安装应用 ID 列表 |
| uninstallAppIds | int[] |  | 重启时附带卸载应用 ID 列表 |
| root | boolean |  | 重启后是否开 root |
| disableAdb | boolean |  | 重启后是否禁 ADB |
| adbWhiteIp | string[] |  | 重启后 ADB 白名单 IP |
| adbTtl | int |  | ADB token 有效期（秒）|

请求示例：

```json
{ "cpIds": ["cp-aaa"], "root": false }
```

**响应**

`data` 仅承载成功状态，无业务内容。完整 JSON 示例：

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="28-destroy"></a>
### 2.8 批量销毁

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/destroy |
| 中台 controller | [CloudPhoneController.destroy](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L223)（`/cloud-phone/destroy`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 列表 |
| appIds | string[] |  | 销毁前需要卸载的应用 ID（兜底场景）|
| destroyReason | string |  | 销毁原因枚举码 |
| reasonDesc | string |  | 销毁原因描述（自由文本）|
| destroyOperator | string |  | 操作人 |

请求示例：

```json
{ "cpIds": ["cp-aaa"], "destroyReason": "MANUAL", "reasonDesc": "测试销毁" }
```

**响应**

`data` 是销毁成功的 cpId 列表（部分成功部分失败时反映实际结果）。完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": ["cp-aaa"],
  "traceId": "abc-123"
}
```

---

<a id="29-reset"></a>
### 2.9 一键刷新（一键新机）

> 在保留 cpId 的前提下，重新初始化云手机：擦除数据 + 重装镜像 + 可选保留代理绑定。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/batchRefreshPhone |
| 中台 controller | [CloudPhoneController.batchRefreshPhone](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L289)（`/cloud-phone/batchRefreshPhone`） |
| 超时 | 30s（异步触发，实际刷机耗时长） |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 列表 |
| keepbindAgent | boolean |  | 是否保留绑定的代理。**默认 `true`**（保持代理）；若要解代理必须显式传 `false` |

> 📜 历史上前端曾用 `unbindProxy` 字段，中台**不识别**——下发任何"解代理"指令都必须用 `keepbindAgent: false`。

请求示例：

```json
{ "cpIds": ["cp-aaa", "cp-bbb"], "keepbindAgent": true }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="210-recycle"></a>
### 2.10 回收云手机

> 与 §2.8 销毁的区别是仍保留底层资源，运维侧可回滚。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/recycle-phone |
| 中台 controller | [CloudPhoneController.recyclePhone](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L1072)（`/cloud-phone/recycle-phone`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 列表 |
| needReset | boolean |  | 回收前是否先 reset（清数据）|

请求示例：

```json
{ "cpIds": ["cp-aaa"], "needReset": true }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="211-install-app"></a>
### 2.11 批量装应用

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/installApp |
| 中台 controller | [CloudPhoneController.install](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L417)（`/cloud-phone/installApp`） |
| 超时 | 30s（异步触发，实际安装由端侧异步） |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 目标云机 |
| appIds | int[] | ✓ | app_info 主键列表 |

请求示例：

```json
{ "cpIds": ["cp-aaa", "cp-bbb"], "appIds": [12345, 12346] }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| taskInfoList | object[] | N | 每台云机的任务下发记录 |
| taskInfoList[].taskId | string | N | 任务 ID |
| taskInfoList[].instanceId | string | N | 云机 ID |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "taskInfoList": [
      { "taskId": "tk-aaa", "instanceId": "cp-aaa" },
      { "taskId": "tk-bbb", "instanceId": "cp-bbb" }
    ]
  },
  "traceId": "abc-123"
}
```

---

<a id="212-uninstall-app"></a>
### 2.12 批量卸应用

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/uninstallApp |
| 中台 controller | [CloudPhoneController.uninstall](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L433)（`/cloud-phone/uninstallApp`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 目标云机 |
| appIds | int[] |  | app_info 主键列表（任一传即可）|
| packageNames | string[] |  | 包名列表（绕开 app_info 直接按包名卸载）|

请求示例：

```json
{ "cpIds": ["cp-aaa"], "packageNames": ["com.example.app"] }
```

**响应**

结构同 [§2.11](#211-install-app)。

---

<a id="213-get-installed-apps"></a>
### 2.13 查云手机已装应用列表

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/getInstalledApps |
| 中台 controller | [CloudPhoneController.getInstalledApps](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L495)（`/cloud-phone/getInstalledApps`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 通常单个 cpId |

请求示例：

```json
{ "cpIds": ["cp-aaa"] }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 每个 cpId 一条 |
| data[].cpId | string | N | 云机 ID |
| data[].apps | object[] | N | 该云机已安装应用列表 |
| data[].apps[].id | number | Y | app_info 主键（中台库内已知应用时有值；端侧自装应用为 null） |
| data[].apps[].packageName | string | N | 包名 |
| data[].apps[].appName | string | Y | 应用名 |
| data[].apps[].version | string | Y | 版本号 |
| data[].apps[].md5 | string | Y | 包 MD5 |
| data[].apps[].iconPath | string | Y | 图标 URL |
| data[].apps[].fileSize | string | Y | 文件大小（已格式化字符串） |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "cpId": "cp-aaa",
      "apps": [
        {
          "id": 12345,
          "packageName": "com.tencent.mm",
          "appName": "微信",
          "version": "8.0.42",
          "md5": "5d41402abc4b2a76b9719d911017c592",
          "iconPath": "https://oss.example.com/icons/wechat.png",
          "fileSize": "256MB"
        }
      ]
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="214-webrtc-auth"></a>
### 2.14 WebRTC 鉴权票据

> 前端发起 WebRTC 连接前必须先拿到一次性 token 和信令地址。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/webrtc-auth |
| 中台 controller | [CloudPhoneController.webrtcAuth](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L128)（`/cloud-phone/webrtc-auth`） |
| 超时 | 30s |
| 业务时机 | 用户点击"连接"按钮前 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 待连接云手机列表 |

请求示例：

```json
{ "cpIds": ["cp-aaa"] }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 每台手机一条 |
| data[].cpId | string | N | 云机 ID |
| data[].vmId | string | N | VM ID |
| data[].zoneId | int | Y | 可用区 ID |
| data[].pushStreamUrl | string | N | 推流地址 |
| data[].signalUrl | string | N | WebRTC 信令地址 |
| data[].authToken | string | N | 临时鉴权 token |
| data[].hasRunningTask | boolean | Y | 是否当前有运行中的脚本任务 |
| data[].hasUpcomingTask | boolean | Y | 是否有待执行任务 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "cpId": "cp-aaa",
      "vmId": "vm-001",
      "zoneId": 1,
      "pushStreamUrl": "rtmp://stream.example.com/live/cp-aaa",
      "signalUrl": "wss://signal.example.com/ws",
      "authToken": "******",
      "hasRunningTask": false,
      "hasUpcomingTask": false
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="215-batch-query-status"></a>
### 2.15 批量查云机状态（轮询）

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/batch-query-status |
| 中台 controller | [CloudPhoneController.batchQueryStatus](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L557)（`/cloud-phone/batch-query-status`） |
| 超时 | 30s |
| 业务时机 | 业务方分页轮询；中台限制单次 ≤100 台 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 列表（≤100）|

请求示例：

```json
{ "cpIds": ["cp-aaa", "cp-bbb"] }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| data | object[] | N | 每台云机一条 |
| data[].cpId | string | N | 云机 ID |
| data[].status | string | N | 云手机状态（见 [§2.2 **枚举：云手机状态**](#22-list-cloud-phones-v1)） |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    { "cpId": "cp-aaa", "status": "NORMAL" },
    { "cpId": "cp-bbb", "status": "STOPPED" }
  ],
  "traceId": "abc-123"
}
```

<a id="216-list-open-cloud-phones"></a>
### 2.16 v2 云手机分页（主力查询接口 ⭐）

> ⭐ **查询云手机列表的主入口**。返回的字段集是 v1（[§2.2](#22-list-cloud-phones-v1)）的 3 倍，建议所有新业务用此接口。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/page |
| 中台 controller | [CloudPhoneController.page](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L116)（`/cloud-phone/page`） |
| 超时 | 30s |
| 业务时机 | 列表页 / 状态后台同步 / 代理绑定同步 / 任务前云机校验 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码（≥ 1，**字段名是 `page`，不是 `pageNum`**） |
| pageSize | int | ✓ | 每页条数 |
| id | number |  | 主键 |
| cpId | string |  | 云机 ID 精确 |
| cpIds | string[] |  | 云机 ID 正向 IN |
| vmId | string |  | VM ID 精确 |
| vmIds | string[] |  | VM ID IN |
| vmUid | string |  | VM uid 精确 |
| status | string |  | 云手机状态（见 [§2.2 **枚举：云手机状态**](#22-list-cloud-phones-v1)） |
| statusList | string[] |  | 云手机状态多选 |
| imageId | string |  | 镜像 ID |
| imageName | string |  | 镜像名 |
| imageDisplayName | string |  | 镜像显示名 |
| imageVersion | string |  | 镜像版本 |
| androidVersion | string |  | 安卓版本 |
| jarPackageVersionId | string |  | Jar 包版本 ID |
| androidAgentVersion | string |  | Android Agent 版本 |
| specId | int |  | 规格 ID |
| specName | string |  | 规格名 |
| specSupplier | string |  | 规格供应商 |
| volumeId | string |  | 存储卷 ID |
| width | int |  | 分辨率宽 |
| height | int |  | 分辨率高 |
| fps | int |  | 帧率 |
| zoneId | int |  | 可用区 ID |
| zoneName | string |  | 可用区名 |
| pmId | string |  | 物理机 ID |
| cpSource | string |  | 来源 |
| tenantId | int |  | 租户 ID |
| tenantName | string |  | 租户名 |
| createTimeStart | string |  | 创建时间窗起（`"2026-01-01T00:00:00"`） |
| createTimeEnd | string |  | 创建时间窗止 |

> ⚠️ 中台**不支持**以下过滤维度：`filterCpIds` / `bindStatusList`（仅单选 `bindStatus`） / `countryName` / `proxyBound` / `keyword` / `androidVersionList` / `expiredStatus`。如需这些过滤，调用方先在本地 DB 预查后转成 `cpIds` 正向 IN 再调本接口；或走 [§2.2 v1](#22-list-cloud-phones-v1) 兜底（v1 原生支持其中几项）。

请求示例：

```json
{
  "page": 1,
  "pageSize": 50,
  "cpIds": ["cp-xxx-001", "cp-xxx-002"],
  "statusList": ["NORMAL", "STOPPED"]
}
```

**响应**

`data` 字段：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "data": [
      {
        "id": 12345,
        "cpId": "cp-xxx-001",
        "cpSource": "SELF",
        "vmId": "vm-aaa-111",
        "vmStatus": "ONLINE",
        "vmUid": "vm-uid-xxx",
        "vmIp": "10.1.2.3",
        "vmCpu": 16,
        "vmMemory": 64,
        "vmStorage": 500,
        "vmGpuCount": 1,
        "imageId": "img-android11-prod",
        "imageName": "android-11-prod",
        "imageDisplayName": "Android 11 - Production",
        "imageVersion": "1.2.3",
        "androidVersion": "11",
        "imageSource": "HARBOR",
        "status": "NORMAL",
        "isRooted": false,
        "width": 720,
        "height": 1280,
        "fps": 30,
        "adbAddress": "10.1.2.3:5555",
        "streamingServerAddress": "10.1.2.3:8088",
        "webrtcAddress": "10.1.2.3:7088",
        "adbToken": "tok-aaabbbccc",
        "adbTokenExpiredAt": "2026-06-04T10:00:00",
        "pmId": "pm-aaaa",
        "bindTime": "2026-05-12T08:23:45",
        "zoneId": 1,
        "zoneName": "华南-1",
        "zoneCode": "south-1",
        "volumeId": "vol-xxx",
        "specId": 7,
        "specName": "1c2g16g",
        "specSupplier": "tencent",
        "specCore": 1,
        "specMemory": 2048,
        "specStorage": 16,
        "specType": "SHARED",
        "specFeature": "GPU",
        "specFlag": "STANDARD",
        "tenantId": 7,
        "tenantName": "alice-corp",
        "parentTenants": "1,5,7",
        "currentTenantId": 7,
        "createTime": "2026-05-12T08:23:45",
        "updateTime": "2026-06-03T10:00:00",
        "lastStartTime": "2026-06-03T08:00:00",
        "createBy": "alice",
        "updateBy": "alice",
        "phoneCpu": 1,
        "phoneMemory": 2048,
        "phoneStorage": 16,
        "saleType": "EXCLUSIVE",
        "bindingStatus": "BINDED",
        "proxyInfo": {
          "id": 9001,
          "proxyType": "socks5",
          "proxyOrderType": "静态",
          "ip": "us-proxy.ipvibe.io",
          "port": 1080,
          "country": "US",
          "countryName": "United States",
          "egressIp": "1.2.3.4",
          "testingResult": 1,
          "detectionTime": "2026-06-03T10:00:00"
        },
        "outgoingIp": "1.2.3.4",
        "exceptionReason": null,
        "bootSchemeName": "default-boot",
        "tagList": [
          { "id": 11, "tagName": "prod", "tagColor": "#FF0000" }
        ],
        "inWebrtc": false,
        "webrtcCount": 0,
        "maxMemory": 4096,
        "webrtcChannelsNum": 3,
        "isMaintain": false,
        "uploadSpeedLimit": 50000,
        "downloadSpeedLimit": 50000,
        "vmRunningPhoneCount": 12,
        "vmMaxStartCount": 20,
        "destroyReason": null,
        "reasonDesc": null,
        "destroyTime": null,
        "destroyOperator": null,
        "scriptTaskStatus": "IDLE",
        "logLevel": "INFO",
        "logFilesCount": 0,
        "androidAgentVersion": "2.3.4",
        "luaBaseLibVersion": "1.0.5",
        "adbAddr": "10.1.2.3:5555"
      }
    ],
    "pageNum": 1,
    "pageSize": 50,
    "totalSize": 142
  },
  "traceId": "abc-123-def"
}
```

**响应字段详解**（按用途分 11 组）：

**组 A — ID / 来源（9 字段）**

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| id | Long | N | 主键 ID |
| cpId | string | N | 云手机实例编号 |
| cpSource | string | Y | 来源（如 `SELF` 自有 / 第三方供应商代码） |
| vmId | string | N | 所属云服务器编号 |
| vmUid | string | N | 所属云服务器唯一标识 |
| vmIp | string | Y | 所属云服务器 IP |
| pmId | string | Y | 物理机 ID |
| volumeId | string | Y | 存储卷 ID |
| currentTenantId | int | Y | 当前查询者租户 ID（用于多租户穿透标识） |

**组 B — VM 物理资源（5 字段）**

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| vmStatus | string | Y | 服务器状态枚举，见**VM 服务器状态枚举**表 |
| vmCpu | int | N | CPU 核心数 |
| vmMemory | int | N | 内存大小（GB） |
| vmStorage | int | N | 存储大小（GB） |
| vmGpuCount | int | Y | GPU 卡数 |

**VM 服务器状态枚举**（来自 [VMStatusEnum.java](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/enums/VMStatusEnum.java)）：

| 值 | 含义 |
|---|---|
| `ONLINE` | 在线 |
| `OFFLINE` | 服务器离线 |
| `REBOOTING` | 重启中 |
| `RESETTING` | 重置中 |
| `REPLACING` | 换机中 |
| `UPGRADING` | 升级中 |
| `AGENT_OFFLINE` | 控制器离线 |
| `MAINTENANCE` | 维护中 |

**组 C — 镜像信息（7 字段）**

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| imageId | string | N | 镜像 ID |
| imageName | string | N | 镜像名（代码用） |
| imageDisplayName | string | Y | 镜像显示名 |
| imageVersion | string | Y | 镜像版本号 |
| androidVersion | string | Y | 安卓版本 |
| imageSource | string | Y | 镜像源（如 `HARBOR`） |
| jarPackageVersionId | string | Y | Jar 包版本 ID |

**组 D — 云机状态 / 设备配置（12 字段）**

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| status | string | N | 云手机状态枚举，**与 §2.2.3 的 CloudPhoneEnum 同一套**（NORMAL/STARTING/STOPPING/REBOOTING/RESETTING/REPLACING/UPDATING/STOPPED/INITIALIZING/INIT_FAILED/DISTROYING/MAINTAINING/VM_RESETTING/VM_OFFLINE/VM_UPGRADING/AGENT_OFFLINE/FAULTED/DESTROYED） |
| isRooted | Boolean | Y | 是否已 root |
| width | int | N | 分辨率宽 |
| height | int | N | 分辨率高 |
| fps | int | N | 帧率 |
| isMaintain | Boolean | Y | 维护状态：`false`=非维护，`true`=维护中 |
| adbAddress | string | Y | ADB 内网地址 |
| streamingServerAddress | string | Y | 推流服务器地址 |
| webrtcAddress | string | Y | WebRTC 信令地址 |
| adbAddr | string | Y | ADB 短字段（与 `adbAddress` 冗余，历史包袱） |
| adbToken | string | Y | ADB 鉴权 token |
| adbTokenExpiredAt | string (LocalDateTime) | Y | ADB token 过期时间 |

**组 E — 绑定 / 销售（5 字段）**

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| saleType | string | Y | 销售类型：`SHARED`=共享，`EXCLUSIVE`=独占 |
| bindingStatus | string | Y | 绑定状态：`UNBIND`=未绑定，`BINDING`=绑定中，`BINDED`=已绑定（**注意：这里是字符串枚举，与 [§2.2 响应](#22-list-cloud-phones-v1) 里的 `bindStatus` int 0/1 不同**） |
| bindTime | string (LocalDateTime) | Y | 绑定时间 |
| exceptionReason | string | Y | 异常原因（status=FAULTED 时） |
| reasonDesc | string | Y | 异常描述（人类可读） |

**组 F — 可用区（3 字段）**

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| zoneId | int | Y | 可用区 ID |
| zoneName | string | Y | 可用区名（中文，如"华南-1"） |
| zoneCode | string | Y | 可用区 code（如 `south-1`） |

**组 G — 规格（9 字段）**

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| specId | int | Y | 规格 ID |
| specName | string | Y | 规格名（如 `1c2g16g`） |
| specSupplier | string | Y | 规格供应商（如 `tencent` / `baidu` / `xiaoxi`） |
| specCore | int | Y | 规格 CPU 核数 |
| specMemory | int | Y | 规格内存（MB） |
| specStorage | int | Y | 规格存储（GB） |
| specType | string | Y | 规格类型（业务自定义） |
| specFeature | string | Y | 规格特性（业务自定义，如 `GPU`） |
| specFlag | string | Y | 规格标签（业务自定义，如 `STANDARD`） |
| bootSchemeName | string | Y | 启动方案名 |

**组 H — 实际分配资源（3 字段）**

> 区分：`specXxx` 是规格表配置值，`phoneXxx` 是该云机分配后的实际值。

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| phoneCpu | int | Y | 该云机实际分到的 CPU 核数 |
| phoneMemory | int | Y | 实际内存（MB） |
| phoneStorage | int | Y | 实际存储（GB） |
| maxMemory | int | Y | 内存上限（MB） |

**组 I — 租户 / 审计（10 字段）**

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| tenantId | int | Y | 所属租户 ID |
| tenantName | string | Y | 租户名 |
| parentTenants | string | Y | 父租户链（CSV 形态，如 `"1,5,7"`） |
| createTime | string (LocalDateTime) | N | 创建时间 |
| updateTime | string (LocalDateTime) | Y | 更新时间 |
| lastStartTime | string (LocalDateTime) | Y | 最后一次开机时间 |
| createBy | string | Y | 创建人 |
| updateBy | string | Y | 更新人 |
| destroyReason | string | Y | 销毁原因（status=DESTROYED 时） |
| destroyTime | string (LocalDateTime) | Y | 销毁时间 |
| destroyOperator | string | Y | 销毁操作者 |

**组 J — 代理 / 网络（6 字段）**

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| proxyInfo | object | Y | 代理信息（结构同 [§2.2 嵌套：代理信息](#22-list-cloud-phones-v1)） |
| outgoingIp | string | Y | 出口 IP（由代理检测后回填） |
| uploadSpeedLimit | int | Y | 上行限速（kbps） |
| downloadSpeedLimit | int | Y | 下行限速（kbps） |
| vmRunningPhoneCount | int | Y | 同 VM 上正在跑的云机数 |
| vmMaxStartCount | int | Y | 同 VM 上并发开机上限 |

**组 K — WebRTC / 受控 / 脚本 / 日志 / 标签（10 字段）**

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| inWebrtc | Boolean | Y | 是否在 WebRTC 中 |
| webrtcCount | int | Y | 当前 WebRTC 连接数 |
| webrtcChannelsNum | int | Y | WebRTC 通道上限 |
| scriptTaskStatus | string | Y | 脚本任务状态：`BUSY`=任务中，`IDLE`=空闲 |
| logLevel | string | Y | 日志级别（`DEBUG`/`INFO`/`WARN`/`ERROR`） |
| logFilesCount | int | Y | 日志文件数 |
| androidAgentVersion | string | Y | Android Agent 版本 |
| luaBaseLibVersion | string | Y | Lua 基础库版本 |
| tagList | TagVO[] | Y | 标签列表（`{id, tagName, tagColor}`） |

**curl 示例**

```bash
curl -X POST https://midplat.example.com/open/api/vendor/v1/cloud-phone/page \
  -H "X-Access-Key: $AK" -H "X-Timestamp: $TS" -H "X-Signature: $SIG" \
  -H "X-Internal-Request: 1" -H "Content-Type: application/json" \
  -d '{
    "page": 1,
    "pageSize": 50,
    "cpIds": ["cp-xxx-001", "cp-xxx-002"],
    "statusList": ["NORMAL", "STOPPED"]
  }'
```

响应（截选）：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "data": [
      {
        "id": 12345,
        "cpId": "cp-xxx-001",
        "vmId": "vm-aaa-111",
        "vmUid": "vm-uid-xxx",
        "status": "NORMAL",
        "isRooted": false,
        "width": 720, "height": 1280, "fps": 30,
        "androidVersion": "11",
        "imageId": "img-android11-prod",
        "imageName": "android-11-prod",
        "specId": 7, "specName": "1c2g16g",
        "zoneId": 1, "zoneName": "华南-1",
        "tenantId": 7, "tenantName": "alice-corp",
        "createTime": "2026-05-12T08:23:45",
        "expireTime": "2026-12-31T23:59:59",
        "scriptTaskStatus": "IDLE",
        "inWebrtc": false,
        "webrtcCount": 0,
        "webrtcChannelsNum": 3,
        "proxyInfo": {
          "id": 9001,
          "proxyType": "socks5",
          "proxyOrderType": "静态",
          "ip": "us-proxy.ipvibe.io",
          "port": 1080,
          "country": "US",
          "countryName": "United States",
          "egressIp": "1.2.3.4",
          "testingResult": 1,
          "detectionTime": "2026-06-03T10:00:00"
        }
      }
    ],
    "pageNum": 1,
    "pageSize": 50,
    "totalSize": 142
  },
  "traceId": "abc-123-def"
}
```

---

## §3 云手机底层配置 + 文件操作（15 接口）

> 节点变更说明：原文档列了"按 OSS URL 分发"+"流式上传"两条，实际中台只有一条 multipart 入口（`/phone-command/batch-upload`），合并为 §3.13；下游编号顺移。

<a id="31-adb-operate"></a>
### 3.1 ADB 统一操作

> 三合一接口：开启 ADB / 关闭 ADB / 改白名单。`operation` 字段路由。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/adb/operate |
| 中台 controller | [AdbController.operateAdb](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AdbController.java#L123)（`/adb/operate`） |
| 超时 | 30s |
| 业务时机 | 云机详情页 ADB 面板操作 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| operation | string | ✓ | `enable` / `disable` / `update_whitelist`（**全小写**） |
| vmId | string |  | 宿主 VM ID（部分实现需要）|
| containers | string[] | ✓ | 云机 ID 列表 |
| whiteIp | string[] |  | 白名单 IP；`disable` 操作时可为 `[]` |
| ttl | int |  | ADB token 有效期（秒），默认 86400（24h） |

请求示例：

```json
{
  "operation": "enable",
  "containers": ["cp-aaa"],
  "whiteIp": ["1.2.3.4"],
  "ttl": 86400
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| operation | string | N | 回显操作类型 |
| vmId | string | Y | 回显 VM ID |
| allSuccess | boolean | N | 整批是否全部成功 |
| successContainers | string[] | N | 成功的 cpId 列表 |
| failedContainers | string[] | Y | 失败的 cpId 列表 |
| errorMessage | string | Y | 整体错误描述 |
| containerDetails | object[] | Y | 每台机详情 |
| containerDetails[].containerId | string | N | cpId |
| containerDetails[].proxy | string | Y | 该机当前代理（关联信息）|
| containerDetails[].success | boolean | N | 单机是否成功 |
| containerDetails[].error | string | Y | 单机错误描述 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "ADB开启成功",
  "data": {
    "operation": "enable",
    "vmId": "vm-aaa",
    "allSuccess": true,
    "successContainers": ["cp-aaa"],
    "failedContainers": [],
    "errorMessage": null,
    "containerDetails": [
      { "containerId": "cp-aaa", "proxy": null, "success": true, "error": null }
    ]
  },
  "traceId": "abc-123"
}
```

---

<a id="32-get-adb-whitelist"></a>
### 3.2 查云手机 ADB 白名单

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/vendor/v1/adb/whitelist/cp/{cpId} |
| 中台 controller | [AdbWhitelistController.getByCpId](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/AdbWhitelistController.java#L29)（`/adb/whitelist/cp/{cpId}`） |
| 超时 | 30s |
| 业务时机 | 详情页加载 ADB 白名单 |

**请求**

Path 参数：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpId | string | ✓ | 云机 ID（拼到 URL） |

**响应**

`data` 字段（白名单记录列表）：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 白名单记录列表 |
| data[].id | number | N | 记录主键 |
| data[].cpId | string | N | 云机 ID |
| data[].vmId | string | N | VM ID |
| data[].ipAddress | string | N | IP 地址 |
| data[].ipDesc | string | Y | IP 描述 |
| data[].status | int | N | 状态码 |
| data[].statusDesc | string | Y | 状态描述 |
| data[].expireTime | string | Y | 过期时间 |
| data[].expired | boolean | Y | 是否过期 |
| data[].createTime | string | N | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |
| data[].createBy | string | Y | 创建人 |
| data[].updateBy | string | Y | 更新人 |
| data[].cloudPhoneStatus | string | Y | 云机当前状态（见 [§2.2 **枚举：云手机状态**](#22-list-cloud-phones-v1)） |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "id": 1001,
      "cpId": "cp-aaa",
      "vmId": "vm-aaa",
      "ipAddress": "1.2.3.4",
      "ipDesc": "本地办公网",
      "status": 1,
      "statusDesc": "生效",
      "expireTime": "2026-12-31T23:59:59",
      "expired": false,
      "createTime": "2026-05-01T10:00:00",
      "updateTime": "2026-05-12T10:00:00",
      "createBy": "alice",
      "updateBy": "alice",
      "cloudPhoneStatus": "NORMAL"
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="33-get-network-strategy"></a>
### 3.3 查云手机网络策略

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/vendor/v1/network-strategy/query/by-phone/{cpId} |
| 中台 controller | [NetworkStrategyController.getNetworkStrategyByPhoneId](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/NetworkStrategyController.java#L88)（`/network-strategy/query/by-phone/{phoneId}`） |
| 超时 | 30s |
| 业务时机 | 详情页加载网络策略 |

**请求**

Path 参数：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpId | string | ✓ | 云机 ID（接 URL；中台命名 `phoneId`）|

**响应**

`data` 是策略数组（一台云机可能有多个策略）：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 策略列表 |
| data[].id | number | N | 策略主键 |
| data[].networkId | string | N | 网络 ID |
| data[].zoneId | int | Y | 可用区 ID |
| data[].gatewayId | int | Y | 网关 ID |
| data[].gatewayName | string | Y | 网关名 |
| data[].networkBandwidth | int | Y | 带宽（Mbps） |
| data[].uploadSpeedLimit | int | Y | 上行限速（kbps）|
| data[].downloadSpeedLimit | int | Y | 下行限速（kbps）|
| data[].mainCode | string | N | 主体编号（cpId）|
| data[].mainType | string | N | 主体类型 |
| data[].mainIp | string | Y | 主体 IP |
| data[].status | string | N | 状态 |
| data[].exitIp | string | Y | 当前出口 IP |
| data[].createTime | string | N | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "id": 5001,
      "networkId": "net-aaa",
      "zoneId": 1,
      "gatewayId": 100,
      "gatewayName": "默认网关",
      "networkBandwidth": 50,
      "uploadSpeedLimit": 50000,
      "downloadSpeedLimit": 50000,
      "mainCode": "cp-aaa",
      "mainType": "CLOUD_PHONE",
      "mainIp": "10.1.2.3",
      "status": "active",
      "exitIp": "1.2.3.4",
      "createTime": "2026-05-01T10:00:00",
      "updateTime": "2026-05-12T10:00:00"
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="34-update-bandwidth"></a>
### 3.4 改云手机网络限速

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/network-strategy/update/bandwidth |
| 中台 controller | [NetworkStrategyController.updateBandwidth](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/NetworkStrategyController.java#L60)（`/network-strategy/update/bandwidth`） |
| 超时 | 30s |
| 业务时机 | 详情页改限速 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 列表 |
| imageId | string |  | 镜像 ID（兼容字段，限速本身不需要）|
| bandWidth | int |  | 总带宽（Mbps） |
| uploadSpeedLimit | int |  | 上行限速（kbps） |
| downloadSpeedLimit | int |  | 下行限速（kbps） |

请求示例：

```json
{ "cpIds": ["cp-aaa"], "uploadSpeedLimit": 50000, "downloadSpeedLimit": 50000 }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="35-update-storage"></a>
### 3.5 改云手机存储容量

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/storage-info/edit-phone |
| 中台 controller | [StorageInfoController.editPhone](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/StorageInfoController.java#L46)（`/storage-info/edit-phone`） |
| 超时 | 30s |
| 业务时机 | 详情页改存储 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 云机主键（`cloud_phone.id`，**不是 cpId**） |
| storageCapacity | number |  | 新容量（GB） |
| maxIopsLimit | number |  | IOPS 上限，`0`=不限制 |
| maxBandwidthLimit | number |  | 存储带宽上限，`0`=不限制 |

请求示例：

```json
{ "id": 12345, "storageCapacity": 64 }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="36-update-name"></a>
### 3.6 改云手机名称

> path 含 `tencent` 是历史命名包袱，实际是**通用**改名入口（不限腾讯供应商）。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/tencent/instance/updateName |
| 中台 controller | [TencentInstanceController.updateInstanceName](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/TencentInstanceController.java#L118)（`/tencent/instance/updateName`） |
| 超时 | 30s |
| 业务时机 | 详情页改名称 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| instanceId | string | ✓ | 云机 ID（cpId）|
| name | string | ✓ | 新名称 |

请求示例：

```json
{ "instanceId": "cp-aaa", "name": "勿动-生产-1" }
```

**响应**

`data` 是供应商原始响应 JSON（已序列化字符串）。完整 JSON 示例：

```json
{
  "code": "200",
  "message": "修改成功",
  "data": "{\"RequestId\":\"xxxx-xxxx\"}",
  "traceId": "abc-123"
}
```

---

<a id="37-update-webrtc-channels"></a>
### 3.7 改 WebRTC 通道数

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/update-webrtc-channels |
| 中台 controller | [CloudPhoneController.updateWebrtcChannels](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L1029)（`/cloud-phone/update-webrtc-channels`） |
| 超时 | 30s |
| 业务时机 | 详情页改 WebRTC 通道 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpId | string | ✓ | 单台云机 ID（不是数组） |
| webrtcChannelsNum | int | ✓ | 新通道数（1-3） |

请求示例：

```json
{ "cpId": "cp-aaa", "webrtcChannelsNum": 2 }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="38-edit-tag"></a>
### 3.8 编辑云机标签

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/edit-tag |
| 中台 controller | [CloudPhoneController.editCloudPhoneTag](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L981)（`/cloud-phone/edit-tag`） |
| 超时 | 30s |
| 业务时机 | 详情页改标签 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| phoneId | number | ✓ | 云机主键（`cloud_phone.id`，**不是 cpId**） |
| tagIds | number[] | ✓ | 标签 ID 列表（全量覆盖语义） |

请求示例：

```json
{ "phoneId": 12345, "tagIds": [101, 102] }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="39-delete-tag"></a>

### 3.9 删云机标签

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/delete-tag |
| 中台 controller | [CloudPhoneController.deleteCloudPhoneTag](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L993)（`/cloud-phone/delete-tag`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| phoneId | number | ✓ | 云机主键 |
| tagIds | number[] |  | 要删的标签 ID 列表；**不传或为空**=删除该云机所有关联标签 |

请求示例：

```json
{ "phoneId": 12345, "tagIds": [101] }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="310-update-image"></a>
### 3.10 切换云机镜像

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/updateImage |
| 中台 controller | [CloudPhoneController.updateImage](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L509)（`/cloud-phone/updateImage`） |
| 超时 | 30s |
| 业务时机 | 详情页 / 列表批量切镜像 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 列表 |
| imageId | string | ✓ | 目标镜像 ID |
| bandWidth | int |  | 切镜像后带宽（Mbps） |
| uploadSpeedLimit | int |  | 切镜像后上行限速 |
| downloadSpeedLimit | int |  | 切镜像后下行限速 |

请求示例：

```json
{ "cpIds": ["cp-aaa"], "imageId": "img-android11-v2" }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="311-list-files"></a>
### 3.11 列云机目录文件

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/phone-command/file-list |
| 中台 controller | [PhoneCommandController.getFileList](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/PhoneCommandController.java#L57)（`/phone-command/file-list`） |
| 超时 | 30s |
| 业务时机 | 文件管理面板浏览 |

**请求**

> ⚠️ **字段名是 snake_case**（`vm_id` / `container_id`），与同 controller 其它接口的驼峰风格不一致。

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| vm_id | string | ✓ | VM ID |
| container_id | string | ✓ | 云机 ID（cpId）|
| path | string | ✓ | 目录绝对路径，如 `/sdcard/Download` |

请求示例：

```json
{ "vm_id": "vm-aaa", "container_id": "cp-aaa", "path": "/sdcard/Download" }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| data | object[] | N | 该目录下的文件 / 子目录列表 |
| data[].name | string | N | 文件名 |
| data[].path | string | N | 完整路径 |
| data[].type | string | N | 文件类型枚举（见**枚举：文件类型**） |
| data[].permission | string | Y | Unix 权限字符串，如 `"-rw-r--r--"` |
| data[].owner | string | Y | 所有者用户名 |
| data[].group | string | Y | 所属组名 |
| data[].size | number | N | 文件大小（字节） |
| data[].accessed | number | Y | 最后访问时间戳（毫秒）|
| data[].modified | number | Y | 最后修改时间戳 |
| data[].changed | number | Y | 状态改变时间戳 |
| data[].created | number | Y | 创建时间戳 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "name": "report.pdf",
      "path": "/sdcard/Download/report.pdf",
      "type": "file",
      "permission": "-rw-r--r--",
      "owner": "u0_a100",
      "group": "u0_a100",
      "size": 524288,
      "accessed": 1717459200000,
      "modified": 1717459200000,
      "changed": 1717459200000,
      "created": 1717459200000
    }
  ],
  "traceId": "abc-123"
}
```

**枚举：文件类型**

| 值 | 含义 |
|---|---|
| `file` | 普通文件 |
| `directory` | 目录 |
| `symlink` | 符号链接 |
| `block` | 块设备 |
| `char` | 字符设备 |
| `fifo` | 命名管道 |
| `socket` | Unix socket |

---

<a id="312-delete-file"></a>
### 3.12 删云机文件

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/phone-command/file-delete |
| 中台 controller | [PhoneCommandController.deleteFile](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/PhoneCommandController.java#L115)（`/phone-command/file-delete`） |
| 超时 | 30s |

**请求**

请求体同 [§3.11](#311-list-files)（`vm_id` / `container_id` / `path`），`path` 指要删的文件 / 目录绝对路径。

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="313-batch-upload-files"></a>
### 3.13 批量上传文件到云机

> 中台**只有一种实现**：multipart/form-data。如果调用方需要"按 OSS URL 分发"，请走 §3.13 的上层封装（前端先上传到 OSS 再调中台）。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/phone-command/batch-upload |
| Content-Type | `multipart/form-data` |
| 中台 controller | [PhoneCommandController.batchUpload](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/PhoneCommandController.java#L127)（`/phone-command/batch-upload`） |
| 超时 | 300s（按文件总大小 / 数量）|
| 业务时机 | 文件管理面板上传 |

**请求**

Form 字段：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| vmId | string |  | VM ID |
| containerId | string | ✓ | 云机 ID（cpId）|
| files | binary[] | ✓ | 多个文件部分 |
| folderPath | string |  | 推到云机内的目标目录，默认 `/sdcard/Download` |
| generateUniqueFileName | boolean |  | 重名是否自动改名，默认 `true` |

**响应**

```json
{ "code": "200", "message": "上传成功", "data": null, "traceId": "abc-123" }
```

---

<a id="314-download-file"></a>
### 3.14 流式下载云机文件

> **响应不走 CallResult 外壳**，直接返回文件二进制 + HTTP 头。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/phone-command/file-download-stream |
| 中台 controller | [PhoneCommandController.fileDownloadStream](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/PhoneCommandController.java#L75)（`/phone-command/file-download-stream`） |
| 超时 | 300s |
| 业务时机 | 文件管理面板下载 |

**请求**

请求体同 [§3.11](#311-list-files)（`vm_id` / `container_id` / `path`），`path` 指要下载的文件绝对路径。

**响应**

成功：

- HTTP 200
- `Content-Type`：按文件类型自动设置（如 `application/pdf` / `image/png`）
- `Content-Disposition: attachment; filename="report.pdf"`
- `Content-Length`：文件字节数
- Body：文件二进制

失败：

- HTTP 500
- Body：UTF-8 编码的错误描述

---

<a id="315-overview-statistics"></a>
### 3.15 云手机大屏总览统计

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/vendor/v1/cloud-phone/overview-statistics |
| 中台 controller | [CloudPhoneController.getCloudPhoneOverviewStatistics](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L1080)（`/cloud-phone/overview-statistics`） |
| 超时 | 30s |
| 业务时机 | 大屏 / 仪表盘 |

**请求**

无入参。

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| total | int | N | 云手机总数 |
| normalCount | int | N | 状态为 `NORMAL` 的数量（已开机） |
| stoppedCount | int | N | 状态为 `STOPPED` 的数量（已关机） |
| faultedCount | int | N | 状态为 `FAULTED` 的数量（异常） |
| initFailedCount | int | N | 状态为 `INIT_FAILED` 的数量（初始化失败） |
| controlledCount | int | N | 受控数量（WebRTC 中） |
| uncontrolledCount | int | N | 非受控数量 |
| busyCount | int | N | 任务中（有运行中脚本）|
| idleCount | int | N | 空闲 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "查询成功",
  "data": {
    "total": 1024,
    "normalCount": 850,
    "stoppedCount": 100,
    "faultedCount": 20,
    "initFailedCount": 4,
    "controlledCount": 320,
    "uncontrolledCount": 530,
    "busyCount": 180,
    "idleCount": 670
  },
  "traceId": "abc-123"
}
```

---

## §4 代理（13 接口）

> 节点变更说明：原 §4.14 `/script/execute` 在中台无对应 controller mapping，已确认为死路径，从本规范移除。下游编号未变（13 个有效接口）。

<a id="41-list-static-proxies"></a>
### 4.1 静态代理分页

> 查租户名下已有的静态代理（含购买和手动录入；TikTok 静态代理走 [§4.4](#44-list-tiktok-proxies)）。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/static/proxy/page |
| 中台 controller | [StaticProxyController.pageQuery](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/StaticProxyController.java#L34)（`/static/proxy/page`） |
| 超时 | 30s |
| 业务时机 | 代理实例管理 → 静态代理 Tab |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码（≥ 0）|
| pageSize | int | ✓ | 每页条数 |
| vendorId | number |  | 代理商 ID |
| tenantId | string |  | 租户 ID（字符串）|
| includeSubTenants | boolean |  | 是否包含子租户 |
| tenantIds | int[] |  | 多租户列表 |
| ipAddress | string |  | IP 模糊匹配 |
| regionType | int |  | 区域类型 |
| proxyType | string |  | 代理协议：`HTTP` / `SOCKS5` |
| country | string |  | 国家代码 |
| region | string |  | 地区代码 |
| status | int |  | `0`=正常，`1`=过期 |
| packageName | string |  | 套餐名 |
| proxyUsername | string |  | 用户名模糊匹配 |
| startTimeBegin | string |  | 开通时间窗起 |
| startTimeEnd | string |  | 开通时间窗止 |

请求示例：

```json
{ "page": 1, "pageSize": 20, "status": 0, "country": "US" }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| pageNum | int | N | 当前页码 |
| pageSize | int | N | 每页条数 |
| totalSize | number | N | 总记录数 |
| data | object[] | N | 静态代理列表 |
| data[].id | number | N | 代理主键 |
| data[].vendorId | number | Y | 代理商 ID |
| data[].vendorName | string | Y | 代理商名 |
| data[].regionType | int | Y | 区域类型 |
| data[].proxyType | string | Y | 代理协议：`HTTP` / `SOCKS5` |
| data[].proxyTypeDesc | string | Y | 协议描述 |
| data[].orderId | string | Y | 订单 ID |
| data[].packageId | string | Y | 套餐 ID |
| data[].packageName | string | Y | 套餐名 |
| data[].tenantId | string | Y | 租户 ID |
| data[].tenantName | string | Y | 租户名 |
| data[].parentTenants | string | Y | 父租户链 |
| data[].country | string | Y | 国家代码 |
| data[].region | string | Y | 地区代码 |
| data[].cpIds | string[] | Y | 已绑定云机列表 |
| data[].boundPhoneCount | int | Y | 绑定云机数量 |
| data[].startTime | string | Y | 开通时间 |
| data[].expireTime | string | Y | 到期时间 |
| data[].status | string | N | 状态：`0`=正常，`1`=过期 |
| data[].statusDesc | string | Y | 状态描述 |
| data[].expired | boolean | Y | 是否过期 |
| data[].remainingDays | number | Y | 剩余天数 |
| data[].createTime | string | N | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |
| data[].createBy | string | Y | 创建人 |
| data[].updateBy | string | Y | 更新人 |
| data[].acquireType | string | Y | 获取方式：`PURCHASE` / `MANUAL` |
| data[].egressIp | string | Y | 检测出口 IP |
| data[].testingCountry | string | Y | 检测国家（中文） |
| data[].testingCountryCode | string | Y | 检测国家代码 |
| data[].testingProvinceCn | string | Y | 检测省份（中文） |
| data[].testingProvinceEn | string | Y | 检测省份（英文） |
| data[].testingCityCn | string | Y | 检测城市（中文） |
| data[].testingCityEn | string | Y | 检测城市（英文） |
| data[].testingResult | int | Y | 检测结果：`1`=成功，`0`=失败 |
| data[].detectionTime | string | Y | 检测时间 |
| data[].tiktokFlag | int | Y | 是否 TikTok：`1`=是，`0`=否 |
| data[].exclusiveBandwidth | string | Y | 独享带宽（TikTok）|
| data[].cmiId | number | Y | 代理商内部 ID |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "data": [
      {
        "id": 9001,
        "vendorId": 5,
        "vendorName": "IPIPGO",
        "proxyType": "SOCKS5",
        "proxyTypeDesc": "SOCKS5代理",
        "orderId": "ord-aaa",
        "packageId": "pkg-001",
        "packageName": "ISP-30天",
        "tenantId": "7",
        "tenantName": "alice-corp",
        "country": "US",
        "region": "CA",
        "cpIds": ["cp-aaa"],
        "boundPhoneCount": 1,
        "startTime": "2026-05-01T10:00:00",
        "expireTime": "2026-12-31T23:59:59",
        "status": "0",
        "statusDesc": "正常",
        "expired": false,
        "remainingDays": 211,
        "createTime": "2026-05-01T10:00:00",
        "updateTime": "2026-05-12T10:00:00",
        "acquireType": "PURCHASE",
        "egressIp": "1.2.3.4",
        "testingCountry": "美国",
        "testingResult": 1,
        "detectionTime": "2026-06-03T10:00:00",
        "tiktokFlag": 0
      }
    ],
    "pageNum": 1,
    "pageSize": 20,
    "totalSize": 142
  },
  "traceId": "abc-123"
}
```

---

<a id="42-list-dynamic-proxies"></a>
### 4.2 动态代理分页

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/dynamic/proxy/page |
| 中台 controller | [DynamicProxyController.pageQuery](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/DynamicProxyController.java#L30)（`/dynamic/proxy/page`） |
| 超时 | 30s |
| 业务时机 | 代理实例管理 → 动态代理 Tab |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| page | int | ✓ | 页码 |
| pageSize | int | ✓ | 每页条数 |
| ip | string |  | IP 模糊匹配 |
| port | int |  | 端口 |
| username | string |  | 用户名 |
| password | string |  | 密码 |
| packageId | string |  | 套餐 ID |
| packageName | string |  | 套餐名 |
| orderId | string |  | 订单 ID |
| status | int |  | `0`=禁用，`1`=启用 |
| tenantId | int |  | 租户 |

请求示例：

```json
{ "page": 1, "pageSize": 20, "status": 1 }
```

**响应**

`data` 结构同 [§4.1](#41-list-static-proxies)，每条记录字段精简如下：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| data[].id | number | N | 主键 |
| data[].protocolType | string | N | 代理协议：`HTTP` / `SOCKS5` |
| data[].ip | string | N | IP |
| data[].port | int | N | 端口 |
| data[].username | string | Y | 用户名 |
| data[].password | string | Y | 密码（按脱敏策略） |
| data[].packageId | int | Y | 套餐 ID |
| data[].packageName | string | Y | 套餐名 |
| data[].tenantId | int | Y | 租户 |
| data[].status | int | N | `0`=禁用，`1`=启用 |
| data[].cpIds | string[] | Y | 已绑定云机 |
| data[].egressIp | string | Y | 检测出口 IP |
| data[].testingResult | int | Y | `1`=成功，`0`=失败 |
| data[].detectionTime | string | Y | 检测时间 |
| data[].createTime | string | N | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |

---

<a id="43-list-custom-proxies"></a>
### 4.3 自定义代理分页

> 自定义代理 = `acquireType=MANUAL` 的手动录入代理。查询同时跨 `static_proxy` 和 `dynamic_proxy` 两张表，按 `proxyType` 路由。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/custom/proxy/manual/page |
| 中台 controller | [CustomProxyController.pageQueryManualProxy](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CustomProxyController.java#L41)（`/custom/proxy/manual/page`） |
| 超时 | 30s |
| 业务时机 | 代理实例管理 → 自定义代理 Tab |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码 |
| pageSize | int | ✓ | 每页条数 |
| ip | string |  | IP 模糊匹配 |
| tenantId | int |  | 单租户过滤 |
| tenantIds | int[] |  | 多租户过滤 |
| proxyType | string |  | `STATIC` / `DYNAMIC` |
| status | int |  | `0`=禁用，`1`=启用 |
| expireTimeSort | int |  | 到期排序：`0`=倒序，`1`=正序 |
| boundPhoneCountSort | int |  | 绑定云机数排序：`0`=倒序，`1`=正序 |

请求示例：

```json
{ "page": 1, "pageSize": 20, "proxyType": "STATIC" }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| data | object[] | N | 自定义代理列表 |
| data[].id | number | N | 主键 |
| data[].proxyType | string | N | `STATIC` / `DYNAMIC` |
| data[].protocolType | string | N | `http` / `socks5` |
| data[].ip | string | N | IP 地址 |
| data[].port | int | N | 端口 |
| data[].username | string | Y | 用户名 |
| data[].password | string | Y | 密码 |
| data[].status | int | N | `0`=禁用，`1`=启用 |
| data[].cpIds | string[] | Y | 已绑定云机 |
| data[].boundPhoneCount | int | Y | 绑定云机数量 |
| data[].tenantId | int | Y | 租户 |
| data[].tenantName | string | Y | 租户名 |
| data[].egressIp | string | Y | 检测出口 IP |
| data[].testingCountry | string | Y | 检测国家（中文） |
| data[].testingCountryCode | string | Y | 检测国家代码 |
| data[].testingProvinceCn | string | Y | 检测省份（中文） |
| data[].testingCityCn | string | Y | 检测城市（中文） |
| data[].testingResult | int | Y | `1`=成功，`0`=失败 |
| data[].detectionTime | string | Y | 检测时间 |
| data[].expireTime | string | Y | 到期时间 |
| data[].createTime | string | N | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |
| pageNum | int | N |  |
| pageSize | int | N |  |
| totalSize | number | N |  |

---

<a id="44-list-tiktok-proxies"></a>
### 4.4 TikTok 代理分页

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/tiktok/proxy/page |
| 中台 controller | [TikTokProxyController.pageQuery](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/TikTokProxyController.java#L29)（`/tiktok/proxy/page`） |
| 超时 | 30s |
| 业务时机 | 代理实例管理 → TikTok Tab |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| page | int | ✓ | 页码 |
| pageSize | int | ✓ | 每页条数 |
| ipAddress | string |  | IP 模糊匹配 |
| tenantId | int |  | 单租户过滤 |
| tenantIds | int[] |  | 多租户过滤 |
| status | int |  | `0`=正常，`1`=过期 |
| startTimeBegin | string |  | 开通时间窗起 |
| startTimeEnd | string |  | 开通时间窗止 |

**响应**

响应结构同 [§4.1](#41-list-static-proxies)。区别仅在底层数据集（`tiktok_flag=1` 且 `order_id IS NOT NULL`）。

---

<a id="45-batch-update-proxy"></a>
### 4.5 批量下发代理到云手机 ⭐

> 把代理绑定关系下发到端侧 tun2socks。**同步调用，但实际 push 串行执行，耗时长（300s 超时）**。下发完成后中台异步触发代理检测事件，由仿真模块更新国家/时区/语言主数据。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/cloud-phone/proxy/batch-update |
| 中台 controller | [CloudPhoneController.batchUpdateProxy](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CloudPhoneController.java#L530)（`/cloud-phone/proxy/batch-update`） |
| 超时 | 300s |
| 业务时机 | 列表勾选 + 绑代理 / 解代理 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | object[] | ✓ | 云机 + 代理 pair 列表（**注意是对象数组，不是字符串数组**）|
| regionFollowProxy | boolean |  | 地区是否跟随代理检测结果回填 |
| timezoneFollowProxy | boolean |  | 时区是否跟随 |
| languageFollowProxy | boolean |  | 语言是否跟随 |
| ipType | string |  | IP 类型 |
| proxyOrderType | string |  | 全局覆盖：`STATIC` / `DYNAMIC` / `TIKTOK` / `CUSTOM` |
| proxyVendor | string |  | 全局覆盖代理商：`IPIPGO` / `SHENLONG` 等 |
| mealId | number |  | 套餐 ID（购买场景）|
| packageName | string |  | 套餐名 |
| mealTime | int |  | 套餐时长 |
| flow | number |  | 流量 |
| exclusiveBandwidth | int |  | 独享带宽 |
| ipList | object[] |  | IP 区域列表 |

**嵌套：cpIds[] 字段**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpId | string | ✓ | 单台云机 ID |
| proxy | object |  | 代理配置；**传 null = 解绑该云机代理** |
| proxy.proxyId | number |  | 代理记录 ID |
| proxy.proxyOrderType | string |  | `STATIC` / `DYNAMIC` / `TIKTOK` |
| proxy.host | string |  | 代理主机 |
| proxy.port | int |  | 端口 |
| proxy.username | string |  | 用户名 |
| proxy.password | string |  | 密码 |
| proxy.type | int |  | `0`=HTTP，`1`=SOCKS5 |
| proxy.simulationConfigType | string |  | 📜 历史字段，重构后已不再据此路由，保留兼容 |

请求示例（绑定 + 解绑 混合）：

```json
{
  "cpIds": [
    {
      "cpId": "cp-aaa",
      "proxy": {
        "proxyId": 9001,
        "proxyOrderType": "STATIC",
        "host": "us-proxy.ipvibe.io",
        "port": 1080,
        "username": "tenant-7-key",
        "password": "******",
        "type": 1
      }
    },
    { "cpId": "cp-bbb", "proxy": null }
  ],
  "regionFollowProxy": true,
  "timezoneFollowProxy": true,
  "languageFollowProxy": true
}
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

> 200 返回**仅代表 push 已下发**，不等于代理已在端侧生效。后续中台会触发代理检测事件 → 仿真模块异步覆盖国家/时区/语言（取决于 `regionFollowProxy` / `timezoneFollowProxy` / `languageFollowProxy`）。

---

<a id="46-add-custom-proxy"></a>
### 4.6 新建自定义代理

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/custom/proxy/add |
| 中台 controller | [CustomProxyController.addCustomProxy](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CustomProxyController.java#L51)（`/custom/proxy/add`） |
| 超时 | 30s |
| 业务时机 | 自定义代理 Tab → 添加 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| proxyType | string | ✓ | `DYNAMIC` / `STATIC` |
| protocolType | string | ✓ | `http` / `socks5`（按中台 ProxyTypeEnum） |
| ip | string | ✓ | IP 地址 |
| port | int | ✓ | 端口 |
| username | string |  | 用户名 |
| password | string |  | 密码 |
| remark | string |  | 备注 |

请求示例：

```json
{
  "proxyType": "STATIC",
  "protocolType": "socks5",
  "ip": "1.2.3.4",
  "port": 1080,
  "username": "u",
  "password": "******"
}
```

**响应**

`data` 是新建代理的 ID。完整 JSON 示例：

```json
{ "code": "200", "message": "成功", "data": 9001, "traceId": "abc-123" }
```

---

<a id="47-batch-delete-custom-proxy"></a>
### 4.7 批量删自定义代理

> 仅适用 `acquireType=MANUAL` 的代理；删除前需保证中台无云机仍绑定该代理。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/custom/proxy/batch/delete |
| 中台 controller | [CustomProxyController.batchDeleteCustomProxy](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/CustomProxyController.java#L69)（`/custom/proxy/batch/delete`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| proxyList | object[] | ✓ | 要删除的代理列表 |
| proxyList[].id | number | ✓ | 代理主键 |
| proxyList[].proxyType | string | ✓ | `STATIC` / `DYNAMIC`（用于路由到正确子表）|

请求示例：

```json
{
  "proxyList": [
    { "id": 1234, "proxyType": "STATIC" },
    { "id": 5678, "proxyType": "DYNAMIC" }
  ]
}
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="48-list-static-meals"></a>
### 4.8 静态代理套餐列表

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/proxy-management/static/meals |
| 中台 controller | [ProxyManagementController.getStaticMealList](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/fingerprint/proxy/interfaces/controller/ProxyManagementController.java#L66)（`/proxy-management/static/meals`） |
| 超时 | 30s |
| 业务时机 | 代理购买 → 套餐选择 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| vendor | string | ✓ | 代理商：`IPIPGO` / `SHENLONG` |

请求示例：

```json
{ "vendor": "IPIPGO" }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 套餐列表 |
| data[].mealId | number | N | 套餐 ID |
| data[].mealType | string | N | 套餐类型（如 `静态代理`） |
| data[].mealName | string | N | 套餐名 |
| data[].ipType | string | Y | IP 类型描述 |
| data[].mealTime | int | N | 套餐时长（**小时**）|
| data[].unifiedPrice | number | Y | 统一价（若各地区同价）|
| data[].countryRegionPriceVO | object[] | Y | 按国家/地区报价 |
| data[].countryRegionPriceVO[].country | string | N | 国家代码 |
| data[].countryRegionPriceVO[].countryName | string | Y | 国家名 |
| data[].countryRegionPriceVO[].region | string | Y | 地区代码 |
| data[].countryRegionPriceVO[].regionName | string | Y | 地区名 |
| data[].countryRegionPriceVO[].price | number | N | 单价 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "mealId": 273,
      "mealType": "静态代理",
      "mealName": "ISP-30天",
      "ipType": "ISP",
      "mealTime": 720,
      "unifiedPrice": 90.0,
      "countryRegionPriceVO": [
        { "country": "US", "countryName": "美国", "region": "CA", "regionName": "加利福尼亚", "price": 90.0 }
      ]
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="49-list-tiktok-meals"></a>
### 4.9 TikTok 代理套餐列表

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/proxy-management/tiktok/ip/list |
| 中台 controller | [ProxyManagementController.getTikTokIpList](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/fingerprint/proxy/interfaces/controller/ProxyManagementController.java#L151)（`/proxy-management/tiktok/ip/list`） |
| 超时 | 30s |
| 业务时机 | 代理购买 → TikTok 套餐选择 |

**请求**

请求体同 [§4.8](#48-list-static-meals)（`{ "vendor": "IPIPGO" }`）。

**响应**

字段基本同 [§4.8](#48-list-static-meals)，额外加：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data[].bandwidthList | int[] | Y | 可选带宽（Mbps），如 `[2, 3, 5, 10]` |
| data[].bandwidthPrice | number | Y | 带宽单价 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "mealId": 337,
      "mealType": "TikTok套餐",
      "mealName": "test-tiktok5天",
      "ipType": "独享静态",
      "mealTime": 120,
      "bandwidthList": [2, 3, 5, 10],
      "bandwidthPrice": 4.00,
      "countryRegionPriceVO": []
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="410-list-static-stock"></a>
### 4.10 静态代理库存

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/proxy-management/static/stock |
| 中台 controller | [ProxyManagementController.getStaticStock](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/fingerprint/proxy/interfaces/controller/ProxyManagementController.java#L96)（`/proxy-management/static/stock`） |
| 超时 | 30s |
| 业务时机 | 代理购买 → 选地区前查库存 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| vendor | string | ✓ | `IPIPGO` / `SHENLONG` |
| customerId | number |  | IPIPGO 二级客户 ID |
| ipType | number |  | 静态产品线：`4`=Hosting，`5`=ISP（单 ISP），`6`=双 ISP；不传走中台默认 |

请求示例：

```json
{ "vendor": "IPIPGO", "ipType": 5 }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 库存列表（按国家/地区聚合）|
| data[].country | string | N | 国家代码（如 `MMR`） |
| data[].region | string | Y | 地区代码（如 `YG`） |
| data[].countryStr | string | N | 国家名（中文） |
| data[].regionStr | string | Y | 地区名（中文） |
| data[].continentName | string | Y | 大洲名 |
| data[].total | int | N | 总数量 |
| data[].enableIP | int | N | 可用 IP 数量 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "country": "MMR",
      "region": "YG",
      "countryStr": "缅甸",
      "regionStr": "仰光省",
      "continentName": "亚洲",
      "total": 273,
      "enableIP": 273
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="411-list-tiktok-stock"></a>
### 4.11 TikTok 代理库存

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/proxy-management/tiktok/stock |
| 中台 controller | [ProxyManagementController.getTikTokStock](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/fingerprint/proxy/interfaces/controller/ProxyManagementController.java#L166)（`/proxy-management/tiktok/stock`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| vendor | string | ✓ | `IPIPGO` / `SHENLONG` |
| customerId | number | ✓ | TikTok 套餐 ID（**必填**，与 §4.10 不同） |

请求示例：

```json
{ "vendor": "IPIPGO", "customerId": 12345 }
```

**响应**

响应结构同 [§4.10](#410-list-static-stock)。

---

<a id="412-purchase-proxies"></a>
### 4.12 代理下单

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/proxy-management/purchase |
| 中台 controller | [ProxyManagementController.purchaseProxy](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/fingerprint/proxy/interfaces/controller/ProxyManagementController.java#L184)（`/proxy-management/purchase`） |
| 超时 | 30s |
| 业务时机 | 代理购买 → 提交订单 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| proxyOrderType | string | ✓ | `STATIC` / `DYNAMIC` / `TIKTOK` |
| proxyVendor | string | ✓ | `IPIPGO` / `SHENLONG` |
| mealId | number | ✓ | 套餐 ID |
| mealTime | int | ✓ | 套餐时长（小时） |
| packageName | string | ✓ | 套餐名（回显用） |
| ipType | string |  | IP 类型 |
| ipList | object[] |  | 按国家/地区指定购买 |
| ipList[].country | string | ✓ | 国家代码 |
| ipList[].region | string |  | 地区代码 |
| ipList[].countryName | string |  | 国家名 |
| ipList[].regionName | string |  | 地区名 |
| ipList[].price | number |  | 单价（回显用） |
| ipList[].quantity | int | ✓ | 数量 |
| flow | number |  | 动态代理流量 |
| exclusiveBandwidth | string |  | TikTok 独享带宽 |

请求示例：

```json
{
  "proxyOrderType": "STATIC",
  "proxyVendor": "IPIPGO",
  "mealId": 273,
  "mealTime": 720,
  "packageName": "ISP-30天",
  "ipType": "ISP",
  "ipList": [
    { "country": "US", "region": "CA", "quantity": 10 }
  ]
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 已分配代理连接信息列表 |
| data[].ip | string | N | IP |
| data[].port | int | N | 端口 |
| data[].username | string | Y | 用户名 |
| data[].password | string | Y | 密码 |
| data[].type | string | Y | 协议（`http` / `socks5`）|
| data[].country | string | Y | 国家代码 |
| data[].countryName | string | Y | 国家名 |
| data[].region | string | Y | 地区代码 |
| data[].regionName | string | Y | 地区名 |
| data[].proxyId | number | N | 代理主键 |
| data[].openTime | string | N | 开通时间 |
| data[].expireTime | string | N | 到期时间 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "ip": "1.2.3.4",
      "port": 1080,
      "username": "t7-aaa",
      "password": "******",
      "type": "socks5",
      "country": "US",
      "countryName": "美国",
      "region": "CA",
      "regionName": "加利福尼亚",
      "proxyId": 9001,
      "openTime": "2026-06-03T10:00:00",
      "expireTime": "2026-07-03T10:00:00"
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="413-batch-detect-proxy"></a>
### 4.13 批量代理可达性检测

> 调度 IpVibe + SOCKS5 + HTTP 多策略并发检测；返回每个代理的 success / outIp / latency / 地理位置。详见 [代理检测技术方案](../../../cloudphone-vendor-service/docs/04-proxy/代理检测技术方案.md)。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/proxy/batch-detect |
| 中台 controller | [ProxyMealRecordController.batchDetectProxyLocal](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/fingerprint/proxy/interfaces/controller/ProxyMealRecordController.java#L62)（`/proxy/batch-detect`） |
| 超时 | 300s |
| 业务时机 | 代理列表"立即检测" / 后台自动检测 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| proxyList | object[] | ✓ | 待检测代理列表 |
| proxyList[].proxyId | string |  | 中台代理主键（回写历史时用）|
| proxyList[].host | string | ✓ | 主机 |
| proxyList[].port | int | ✓ | 端口 |
| proxyList[].protocol | string | ✓ | 协议：`http` / `socks5` / `auto`（多策略路由） |
| proxyList[].username | string |  | 用户名 |
| proxyList[].user | string |  | 📜 兼容字段，与 username 等价；推荐传 username |
| proxyList[].password | string |  | 密码 |
| proxyList[].tenantId | int |  | 租户 ID |

请求示例：

```json
{
  "proxyList": [
    { "proxyId": "9001", "host": "1.2.3.4", "port": 1080,
      "protocol": "socks5", "username": "u", "password": "******" }
  ]
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 每个代理一条检测结果 |
| data[].proxyId | string | Y | 回显代理主键 |
| data[].proxyType | string | Y | 代理订单类型（`STATIC` / `DYNAMIC` / `TIKTOK`） |
| data[].host | string | N | 回显主机 |
| data[].port | int | N | 回显端口 |
| data[].protocol | string | N | 回显协议 |
| data[].username | string | Y | 回显用户名 |
| data[].success | boolean | N | 检测是否成功 |
| data[].errorMessage | string | Y | 失败描述（脱敏后） |
| data[].outIp | string | Y | 检测出口 IP |
| data[].country | string | Y | 出口国家代码（如 `US`）|
| data[].countryName | string | Y | 国家名（英文）|
| data[].province | string | Y | 省份（中文） |
| data[].provinceEn | string | Y | 省份（英文） |
| data[].city | string | Y | 城市（中文） |
| data[].cityEn | string | Y | 城市（英文） |
| data[].lat | string | Y | 纬度 |
| data[].lon | string | Y | 经度 |
| data[].timeZone | string | Y | 时区 |
| data[].postalCode | string | Y | 邮编 |
| data[].delay | number | Y | 延迟（毫秒） |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "proxyId": "9001",
      "proxyType": "STATIC",
      "host": "1.2.3.4",
      "port": 1080,
      "protocol": "socks5",
      "username": "u",
      "success": true,
      "errorMessage": null,
      "outIp": "5.6.7.8",
      "country": "US",
      "countryName": "United States",
      "province": "加利福尼亚",
      "provinceEn": "California",
      "city": "洛杉矶",
      "cityEn": "Los Angeles",
      "lat": "34.0522",
      "lon": "-118.2437",
      "timeZone": "America/Los_Angeles",
      "postalCode": "90001",
      "delay": 320
    }
  ],
  "traceId": "abc-123"
}
```

---

## §5 资源调度（8 接口）

> 节点变更说明：原 §5.1 / §5.2 历史上是 v1 / v2 两个独立条目，但都打到同一中台路径 `POST /resource-group/apply`，入参结构同一份。本文合并为单一 §5.1。下游编号顺移。

<a id="51-apply-resource"></a>
### 5.1 申请资源组

> 异步申请按规格 + 场景的 VM 资源，由后端编排器开通。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/resource-group/apply |
| 中台 controller | [ResourceGroupController.apply](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/ResourceGroupController.java#L112)（`/resource-group/apply`） |
| 超时 | 30s（开通过程异步） |
| 业务时机 | 资源申请向导提交 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| tenantId | int |  | 申请方租户 |
| scenarioId | int |  | 应用场景 ID |
| specId | int |  | 规格 ID |
| openDays | int |  | 开通时长（天）|
| count | int |  | 申请台数 |
| gatewayId | number |  | 网关 ID |
| availZoneId | int |  | 可用区 ID |
| cpu | int |  | CPU 核数（覆盖规格表） |
| memory | int |  | 内存（GB） |
| storage | int |  | 存储（GB） |
| gpuConfigs | object[] |  | GPU 配置列表（每项含 model / brand / cardCount） |

请求示例：

```json
{
  "tenantId": 7,
  "scenarioId": 1,
  "specId": 23,
  "openDays": 30,
  "count": 5,
  "availZoneId": 1
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| message | string | N | 处理结果描述 |
| inserted | int | N | 实际新建 VM 数 |
| vmList | object[] | N | 新建 VM 列表 |
| vmList[].vmId | string | N | VM ID |
| vmList[].vmUid | string | N | VM UID |
| vmList[].scenarioId | int | Y | 场景 ID |
| vmList[].scenarioName | string | Y | 场景名 |
| vmList[].specId | int | N | 规格 ID |
| vmList[].specName | string | Y | 规格名 |
| vmList[].zoneId | int | Y | 可用区 ID |
| vmList[].zoneName | string | Y | 可用区名 |
| vmList[].regionId | int | Y | 区域 ID |
| vmList[].regionName | string | Y | 区域名 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "message": "已提交 5 台开通申请",
    "inserted": 5,
    "vmList": [
      {
        "vmId": "vm-001",
        "vmUid": "vm-uid-001",
        "scenarioId": 1,
        "scenarioName": "默认场景",
        "specId": 23,
        "specName": "1c2g16g",
        "zoneId": 1,
        "zoneName": "华南-1",
        "regionId": 100,
        "regionName": "华南"
      }
    ]
  },
  "traceId": "abc-123"
}
```

---

<a id="52-max-openable"></a>
### 5.2 查最大可开通数

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/vendor/v1/resource-group/max-openable |
| 中台 controller | [ResourceGroupController.maxOpenable](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/ResourceGroupController.java#L98)（`/resource-group/max-openable`） |
| 超时 | 30s |
| 业务时机 | 资源申请向导计算可开数 |

**请求**

Query 参数：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| tenantId | int | ✓ | 租户 ID |
| specId | int | ✓ | 规格 ID |
| scenarioId | int | ✓ | 场景 ID |

请求示例：

```
GET /open/api/vendor/v1/resource-group/max-openable?tenantId=7&specId=23&scenarioId=1
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| maxOpenable | int | N | 最大可开通数（=min(库存, 配额剩余)）|
| stock | int | Y | 库存数 |
| quotaLeft | int | Y | 配额剩余 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": { "maxOpenable": 12, "stock": 18, "quotaLeft": 12 },
  "traceId": "abc-123"
}
```

> 中台未配置时返回零值（非报错）。

---

<a id="53-all-spec-stocks"></a>
### 5.3 全规格库存

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/vendor/v1/resource-group/all-spec-stocks |
| 中台 controller | [ResourceGroupController.allSpecStocks](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/ResourceGroupController.java#L61)（`/resource-group/all-spec-stocks`） |
| 超时 | 30s |
| 业务时机 | 资源申请向导初始化规格下拉 |

**请求**

Query 参数：

| 字段 | 类型 | 必填 | 默认 | 描述 / 枚举 |
|---|---|---|---|---|
| serviceType | string |  | `OPEN` | `OPEN`=本租户资源池库存，`APPLY`=父租户资源池库存 |

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 规格库存列表 |
| data[].specId | int | N | 规格 ID |
| data[].specName | string | N | 规格名 |
| data[].core | int | Y | CPU 核数 |
| data[].memory | int | Y | 内存（GB）|
| data[].storage | int | Y | 存储（GB）|
| data[].bandwidth | int | Y | 带宽（Mbps）|
| data[].gpuCount | int | Y | GPU 数 |
| data[].card | int | Y | 卡数 |
| data[].gpuInfoList | object[] | Y | GPU 详情（model / brand / cardCount） |
| data[].stock | int | N | 库存数 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "specId": 23,
      "specName": "1c2g16g",
      "core": 1,
      "memory": 2,
      "storage": 16,
      "bandwidth": 50,
      "gpuCount": 0,
      "stock": 18,
      "gpuInfoList": []
    }
  ],
  "traceId": "abc-123"
}
```

---

<a id="54-list-servers"></a>
### 5.4 服务器分页

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/server/page |
| 中台 controller | [OpenVirtualMachineController.pageVirtualMachineList](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/open/OpenVirtualMachineController.java#L56)（`/server/page`） |
| 超时 | 30s |
| 业务时机 | 服务器列表页 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码 |
| pageSize | int | ✓ | 每页条数 |
| vmUid | string |  | 虚机编号模糊匹配 |
| vmIp | string |  | 虚机 IP 模糊匹配 |
| vmStatusList | string[] |  | 状态多选（见 [§2.16 **VM 服务器状态枚举**](#216-list-open-cloud-phones)）|
| isMaintain | boolean |  | 维护状态 |
| createTimeStart | string |  | 创建时间窗起 |
| createTimeEnd | string |  | 创建时间窗止 |
| expired | boolean |  | 是否已过期 |

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| pageNum | int | N |  |
| pageSize | int | N |  |
| totalSize | number | N |  |
| data | object[] | N | 虚机列表 |
| data[].id | int | N | 主键 |
| data[].vmUid | string | N | 虚机唯一标识 |
| data[].vmId | string | N | 虚机编号 |
| data[].vmIp | string | Y | 虚机 IP |
| data[].vmStatus | string | N | 状态（见 [§2.16](#216-list-open-cloud-phones)）|
| data[].isMaintain | boolean | Y | 是否维护 |
| data[].specificationId | int | Y | 规格 ID |
| data[].specificationName | string | Y | 规格名 |
| data[].core | int | Y | CPU 核数 |
| data[].memory | int | Y | 内存 |
| data[].storage | int | Y | 存储 |
| data[].bandwidth | int | Y | 带宽 |
| data[].maxStartCount | int | Y | 最大并行开机数 |
| data[].maxPhone | int | Y | 最大可开手机数 |
| data[].createPhoneNumber | int | Y | 当前已开机数 |
| data[].createTime | string | N | 创建时间 |
| data[].expireTime | string | Y | 到期时间 |
| data[].expired | boolean | Y | 是否过期 |
| data[].gpuInfoList | object[] | Y | GPU 详情（id / model / brand / remark / cardCount） |

---

<a id="55-reset-vm"></a>
### 5.5 重置虚机

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/virtual-machine/reset |
| 中台 controller | [VirtualMachineController.reset](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/VirtualMachineController.java#L132)（`/virtual-machine/reset`） |
| 超时 | 30s |
| 业务时机 | 运维 → 重置服务器 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| vmIds | string[] | ✓ | 虚机 ID 列表 |

请求示例：

```json
{ "vmIds": ["vm-aaa", "vm-bbb"] }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="56-list-boot-plans-by-spec"></a>
### 5.6 按规格 + 场景查启动方案

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/vendor/v1/plan-mng/list/by-spec-and-scenario |
| 中台 controller | [PlanMngController.getPlanMngsBySpecAndScenario](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/PlanMngController.java#L97)（`/plan-mng/list/by-spec-and-scenario`） |
| 超时 | 30s |
| 业务时机 | 创建云手机向导加载启动方案下拉 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| specId | int | ✓ | 规格 ID |
| planNameId | int |  | 应用场景 ID（不传 = 全场景）|
| resourceUtilization | string |  | `SHARED`=共享，`EXCLUSIVE`=独享 |

请求示例：

```json
{ "specId": 23, "planNameId": 1 }
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| data | object[] | N | 启动方案列表 |
| data[].id | int | N | 启动方案 ID |
| data[].specId | int | N | 规格 ID |
| data[].planNameId | int | Y | 场景 ID |
| data[].planName | string | N | 方案名（来自场景表） |
| data[].scenarioName | string | Y | 场景名 |
| data[].specName | string | Y | 规格名 |
| data[].supplier | string | Y | 供应商 |
| data[].enableStatus | int | Y | 启用状态 |
| data[].resourceUtilization | string | Y | `SHARED` / `EXCLUSIVE` |
| data[].width | int | Y | 分辨率宽 |
| data[].height | int | Y | 分辨率高 |
| data[].fps | int | Y | 帧率 |
| data[].createNum | int | Y | 默认创建台数 |
| data[].status | int | Y | 状态 |
| data[].core | int | Y | CPU 核数 |
| data[].minCore | number | Y | 最小 CPU |
| data[].maxCore | number | Y | 最大 CPU |
| data[].memory | int | Y | 内存 |
| data[].specMemory | int | Y | 规格内存 |
| data[].specStorage | int | Y | 规格存储 |
| data[].specBandwidth | int | Y | 规格带宽 |
| data[].maxPhone | int | Y | 该方案最大手机数 |
| data[].phoneCount | int | Y | 当前已开手机数 |
| data[].gpuInfoList | object[] | Y | GPU 列表 |
| data[].bootParamsList | object[] | Y | 启动参数变体列表 |
| data[].bootParamsList[].id | number | N | 参数变体 ID |
| data[].bootParamsList[].planId | int | N | 方案 ID |
| data[].bootParamsList[].width | int | N | 分辨率宽 |
| data[].bootParamsList[].height | int | N | 分辨率高 |
| data[].bootParamsList[].fps | int | N | 帧率 |
| data[].bootParamsList[].maxBootCount | int | Y | 最大开机数 |
| data[].bootParamsList[].recommendedBootCount | int | Y | 推荐开机数 |
| data[].bootParamsList[].maxStartCount | int | Y | 并发开机上限 |

---

<a id="57-list-images-by-plan"></a>
### 5.7 按启动方案查镜像列表

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/vendor/v1/img/query/by-plan/{planId} |
| 中台 controller | [OpenImageController.getImagesByPlan](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/open/OpenImageController.java#L35)（`/img/query/by-plan/{planId}`） |
| 超时 | 30s |
| 业务时机 | 创建云手机向导加载镜像下拉 |

**请求**

Path 参数：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| planId | int | ✓ | 启动方案 ID |

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 镜像列表 |
| data[].id | number | N | 镜像主键 |
| data[].imageId | string | N | 镜像 ID |
| data[].templateId | string | Y | 模板 ID |
| data[].templateImageName | string | Y | 模板镜像名 |
| data[].imageName | string | N | 镜像名 |
| data[].imageDisplayName | string | Y | 镜像显示名 |
| data[].imageVersion | string | Y | 镜像版本（中台会回填 = androidVersion）|
| data[].imageType | int | Y | 镜像类型 |
| data[].imageTag | string | Y | 镜像 tag |
| data[].downloadUrl | string | Y | 下载 URL |
| data[].imageSource | string | Y | 镜像来源 |
| data[].imageDescription | string | Y | 描述 |
| data[].md5 | string | Y | 包 MD5 |
| data[].isForbid | int | Y | 是否禁用：`0`=启用，`1`=禁用 |
| data[].androidVersion | string | Y | 安卓版本 |
| data[].createTime | string | Y | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |
| data[].createBy | string | Y | 创建人 |
| data[].showName | string | Y | 展示名 |
| data[].isCurrent | int | Y | 是否当前默认 |
| data[].snapshotName | string | Y | 快照名 |

---

<a id="58-list-phone-models-by-image"></a>
### 5.8 按镜像查手机机型列表

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/vendor/v1/img/query/phone-models/{imageId} |
| 中台 controller | [OpenImageController.getPhoneModelsByImage](../../../cloudphone-vendor-service/src/main/java/com/vdmanager/controller/open/OpenImageController.java#L85)（`/img/query/phone-models/{imageId}`） |
| 超时 | 30s |
| 业务时机 | 创建云手机向导加载机型下拉 |

**请求**

Path 参数：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| imageId | string | ✓ | 镜像 ID |

**响应**

`data` 是级联树（品牌 → 机型）：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 根级品牌列表 |
| data[].value | string | N | 节点值（品牌 code） |
| data[].label | string | N | 节点显示名 |
| data[].status | int | Y | 状态：`0`=禁用，`1`=启用（仅机型节点有效） |
| data[].children | object[] | Y | 子节点（机型列表） |
| data[].children[].value | string | N | 机型 code |
| data[].children[].label | string | N | 机型名 |
| data[].children[].status | int | Y | 启停 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    {
      "value": "samsung",
      "label": "Samsung",
      "children": [
        { "value": "SM-S908E", "label": "Galaxy S22", "status": 1 },
        { "value": "SM-G998B", "label": "Galaxy S21", "status": 1 }
      ]
    }
  ],
  "traceId": "abc-123"
}
```

---

## §6 自动化脚本（15 接口）

> 走 **`cloudphone-autoscript-service`**（不是 vendor-service）。涉及 3 个 controller：
> - 模板管理 `AutoScriptController` (`/api/automation`)
> - 任务管理 `ScriptTaskController` (`/autoscript/task`)
> - 计划管理 `ScriptPlanController` (`/scriptPlan`)

<a id="61-list-script-templates"></a>
### 6.1 脚本模板分页

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/api/automation/templates/page |
| 中台 controller | [AutoScriptController.pageList](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/AutoScriptController.java#L160)（`/api/automation/templates/page`） |
| 超时 | 30s |
| 业务时机 | 脚本管理列表页 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| page | int | ✓ | 页码 |
| pageSize | int | ✓ | 每页条数 |
| name | string |  | 名称模糊匹配 |
| tenantId | int |  | 租户 ID |
| isPublic | int |  | `0`=否，`1`=公共脚本 |
| status | int |  | `0`=禁用，`1`=启用 |
| createBy | string |  | 创建人 |
| scriptVersion | string |  | 版本模糊匹配 |
| ids | number[] |  | 脚本 ID 集合（批量查询） |

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| pageNum / pageSize / totalSize | int / int / number | N | 分页 |
| data | object[] | N | 脚本模板列表 |
| data[].id | number | N | 模板主键 |
| data[].name | string | N | 名称 |
| data[].description | string | Y | 描述 |
| data[].expSkip | int | Y | 异常处理：`1`=跳过，`2`=中断 |
| data[].macroVariable | string | Y | 宏变量 |
| data[].configuration | string | Y | 配置（JSON 字符串）|
| data[].lastUserId | number | Y | 最后操作人 ID |
| data[].attributionManagerId | number | Y | 归属管理员 ID |
| data[].luaFileUrl | string | Y | Lua 文件 OSS URL |
| data[].luaFileName | string | Y | Lua 文件名 |
| data[].luaFileSize | number | Y | Lua 文件大小（MB） |
| data[].luaFileMd5 | string | Y | Lua 文件 MD5 |
| data[].luaContent | string | Y | Lua 源码（**列表接口可能不返**） |
| data[].startParamMap | string | Y | 启动参数定义（JSON 字符串）|
| data[].scriptVersion | string | Y | 版本号 |
| data[].tenantId | int | Y | 租户 ID |
| data[].isPublic | int | Y | 是否公共：`0`=否，`1`=是 |
| data[].status | int | Y | `0`=禁用，`1`=启用 |
| data[].createBy | string | Y | 创建人 |
| data[].createTime | string | N | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |

---

<a id="62-create-scheduled-tasks"></a>
### 6.2 创建定时任务 ⭐

> 把"在 N 台云机上跑某脚本"发布为一组 task。每台云机一条任务记录；返回的 task `id` 后续用于 [§6.5 查询](#65-query-tasks-by-ids) / [§6.6 取消](#66-cancel-task) / [§6.7 删除](#67-delete-tasks-by-ids)。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/create-scheduled |
| 中台 controller | [ScriptTaskController.createScheduledTasks](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L38)（`/autoscript/task/create-scheduled`） |
| 超时 | 30s |
| 业务时机 | 创建脚本任务 / 计划下发 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| scriptId | number | ✓ | 脚本模板 ID |
| taskName | string | ✓ | 任务名 |
| remark | string |  | 业务备注（推荐 JSON 字符串）|
| taskList | object[] | ✓ | 任务对象列表（每台云机一条） |
| taskList[].cpId | string | ✓ | 云机 ID |
| taskList[].publishTime | string | ✓ | 发布时间（LocalDateTime `"2026-06-04T09:00:00"`，无时区，北京时间） |
| taskList[].scriptParams | string |  | 单条任务的自定义参数（JSON 字符串） |

请求示例：

```json
{
  "scriptId": 88,
  "taskName": "微信 - 早班自动回复",
  "remark": "{\"customerId\":\"c-7\"}",
  "taskList": [
    { "cpId": "cp-aaa", "publishTime": "2026-06-04T09:00:00", "scriptParams": "{\"msg\":\"Hi\"}" },
    { "cpId": "cp-bbb", "publishTime": "2026-06-04T09:00:00" }
  ]
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| data | object[] | N | 每个 cpId 一条 |
| data[].id | number | N | 任务主键（**保存为本地业务 mid_task_id**） |
| data[].taskId | string | N | 任务编号 |
| data[].cpId | string | N | 云机 ID |
| data[].planPublishTime | string | N | 实际计划发布时间 |

完整 JSON 示例：

```json
{
  "code": "200",
  "message": "成功",
  "data": [
    { "id": 555001, "taskId": "tk-aaa", "cpId": "cp-aaa", "planPublishTime": "2026-06-04T09:00:00" },
    { "id": 555002, "taskId": "tk-bbb", "cpId": "cp-bbb", "planPublishTime": "2026-06-04T09:00:00" }
  ],
  "traceId": "abc-123"
}
```

---

<a id="63-task-page"></a>
### 6.3 脚本任务分页

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/page |
| 中台 controller | [ScriptTaskController.pageQuery](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L32)（`/autoscript/task/page`） |
| 超时 | 30s |
| 业务时机 | 任务列表页 / Scheduler 按 planUid 拉新任务 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码 |
| pageSize | int | ✓ | 每页条数 |
| taskId | string |  | 任务 ID 精确 |
| taskName | string |  | 任务名模糊匹配 |
| cpId | string |  | 云机 ID 精确 |
| vmUid | string |  | VM UID 精确 |
| taskStatus | string |  | 任务状态（见**枚举：脚本任务状态**） |
| planUid | string |  | 计划 UID 精确（Scheduler 用此过滤）|
| scriptId | number |  | 脚本 ID |

**响应**

`data` 字段（每条记录结构同 [§6.5 ScriptTaskVO](#65-query-tasks-by-ids)）：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "pageNum": 1, "pageSize": 50, "totalSize": 200,
    "data": [ /* ScriptTaskVO */ ]
  },
  "traceId": "abc-123"
}
```

**枚举：脚本任务状态**

| 值 | 含义 |
|---|---|
| `WAITING_PUBLISH` | 等待发布 |
| `WAITING_EXECUTE` | 等待执行 |
| `QUEUED` | 排队等待 |
| `EXECUTING` | 正在执行 |
| `COMPLETED` | 任务完成 |
| `FAILED` | 任务失败 |
| `CANCELLED` | 任务取消 |

---

<a id="64-update-publish-time"></a>
### 6.4 修改任务发布时间

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/update-publish-time |
| 中台 controller | [ScriptTaskController.updatePublishTime](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L44)（`/autoscript/task/update-publish-time`） |
| 超时 | 30s |
| 业务时机 | 任务详情页改发布时间 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 任务主键 |
| planPublishTime | string | ✓ | 新的计划发布时间（LocalDateTime） |

请求示例：

```json
{ "id": 555001, "planPublishTime": "2026-06-04T10:00:00" }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="65-query-tasks-by-ids"></a>
### 6.5 按 ids 查任务

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/query-by-ids |
| 中台 controller | [ScriptTaskController.queryTasksByIds](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L74)（`/autoscript/task/query-by-ids`） |
| 超时 | 30s |
| 业务时机 | 业务方 Poller（每分钟一次）轮询任务状态 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| ids | number[] | ✓ | 任务主键集合（**Long 数组，不是字符串数组**） |

请求示例：

```json
{ "ids": [555001, 555002] }
```

**响应**

`data` 字段（每条 = 1 个任务）：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| data | object[] | N | 任务列表 |
| data[].id | number | N | 任务主键 |
| data[].taskId | string | N | 任务编号 |
| data[].taskName | string | Y | 任务名 |
| data[].cpId | string | N | 云机 ID |
| data[].vmUid | string | Y | VM UID |
| data[].taskStatus | string | N | 状态（见 [§6.3 枚举](#63-task-page)） |
| data[].taskStatusDesc | string | Y | 状态描述 |
| data[].planUid | string | Y | 计划 UID |
| data[].scriptId | number | N | 脚本 ID |
| data[].tenantId | int | Y | 租户 ID |
| data[].tenantName | string | Y | 租户名 |
| data[].scriptParam | string | Y | 脚本参数（JSON 字符串） |
| data[].remark | string | Y | 业务备注（JSON 字符串） |
| data[].planPublishTime | string | Y | 计划发布时间 |
| data[].runStartTime | string | Y | 实际开始时间 |
| data[].runEndTime | string | Y | 实际结束时间 |
| data[].runDuration | number | Y | 运行耗时（毫秒） |
| data[].execResult | int | Y | `0`=失败，`1`=成功，`null`=未跑 |
| data[].createTime | string | N | 创建时间 |
| data[].updateTime | string | Y | 更新时间 |

---

<a id="66-cancel-task"></a>
### 6.6 取消任务

> 字段名是 **`id`（单数）**——一次只能取消一条任务。多条需循环调用。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/cancel |
| 中台 controller | [ScriptTaskController.cancelTask](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L50)（`/autoscript/task/cancel`） |
| 超时 | 30s |
| 业务时机 | 任务详情页 / 列表取消按钮 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 任务主键 |

请求示例：

```json
{ "id": 555001 }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

> 幂等：已取消的任务重复取消无影响。

---

<a id="67-cancel-tasks-by-cpids"></a>
### 6.7 按云机 ID 批量取消任务

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/cancel-by-cpids |
| 中台 controller | [ScriptTaskController.cancelTasksByCpIds](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L56)（`/autoscript/task/cancel-by-cpids`） |
| 超时 | 30s |
| 业务时机 | 销毁云机前清理关联任务 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| cpIds | string[] | ✓ | 云机 ID 集合 |

请求示例：

```json
{ "cpIds": ["cp-aaa", "cp-bbb"] }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="68-re-execute-task"></a>
### 6.8 重新执行任务

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/re-execute |
| 中台 controller | [ScriptTaskController.reExecuteTask](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L62)（`/autoscript/task/re-execute`） |
| 超时 | 30s |
| 业务时机 | 失败任务详情页 → 重试 |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 要重跑的任务主键 |

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="69-query-task-report"></a>
### 6.9 查任务执行报告

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/report |
| 中台 controller | [ScriptTaskController.queryTaskReport](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L68)（`/autoscript/task/report`） |
| 超时 | 30s |
| 业务时机 | 任务详情页查看执行报告（截图 / 日志） |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 任务主键 |

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| taskId | string | N | 任务编号 |
| taskStatus | string | N | `COMPLETED`=已完成，`FAILED`=失败 |
| planName | string | Y | 计划名 |
| runDuration | number | Y | 耗时（秒） |
| startTime | string | Y | 开始时间 |
| endTime | string | Y | 结束时间 |
| screenshotUrl | string | Y | 任务结束截图 URL |
| screenshotBase64 | string | Y | 截图 Base64（小图直接内嵌） |
| runLog | string | Y | 原始日志全文 |
| runLogList | object[] | Y | 解析后的日志时间线 |
| runLogList[].timestamp | string | N | 时间戳 |
| runLogList[].level | string | Y | 日志级别 |
| runLogList[].content | string | N | 日志内容 |

---

<a id="610-delete-tasks-by-ids"></a>
### 6.10 批量删任务

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/autoscript/task/delete-by-ids |
| 中台 controller | [ScriptTaskController.deleteTasksByIds](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptTaskController.java#L80)（`/autoscript/task/delete-by-ids`） |
| 超时 | 30s |

**请求**

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| ids | number[] | ✓ | 任务主键集合（**Long 数组**） |

请求示例：

```json
{ "ids": [555001, 555002] }
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

---

<a id="611-script-plan-create"></a>
### 6.11 创建脚本计划

> 与 [§6.2 一次性任务](#62-create-scheduled-tasks) 的区别：计划是循环执行的"定时器"，按 `executionFrequency` 周期性地派发任务。

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/scriptPlan/create |
| 中台 controller | [ScriptPlanController.createPlan](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptPlanController.java#L49)（`/scriptPlan/create`） |
| 超时 | 30s |
| 业务时机 | 创建脚本计划向导 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| scriptId | number | ✓ | 脚本模板 ID |
| planName | string | ✓ | 计划名 |
| scriptParams | string |  | 变量参数（JSON 字符串） |
| executionFrequency | string | ✓ | `INTERVAL`=间隔，`DAILY`=每天 |
| intervalValue | int |  | 间隔分钟数（当 `executionFrequency=INTERVAL`） |
| executionTime | string |  | 每次执行时间 `HH:mm:ss`（当 `executionFrequency=DAILY`） |
| startTime | string |  | 计划生效开始时间（LocalDateTime） |
| endTime | string |  | 计划生效结束时间 |
| remark | string |  | 备注 |
| cpIds | number[] |  | 云机主键列表 |
| cpIdList | string[] |  | 云机 cpId 字符串列表（与 `cpIds` 二选一） |

请求示例：

```json
{
  "scriptId": 88,
  "planName": "每天 9 点自动签到",
  "executionFrequency": "DAILY",
  "executionTime": "09:00:00",
  "startTime": "2026-06-04T00:00:00",
  "endTime": "2026-12-31T23:59:59",
  "cpIdList": ["cp-aaa", "cp-bbb"]
}
```

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 / 枚举 |
|---|---|---|---|
| id | number | N | 计划主键 |
| planUid | string | N | 计划 UID |
| scriptId | number | N | 脚本 ID |
| scriptParams | string | Y | 参数（JSON） |
| planName | string | N | 计划名 |
| planStatus | string | N | 见**枚举：脚本计划状态** |
| executionFrequency | string | N | `INTERVAL` / `DAILY` |
| intervalValue | int | Y | 间隔分钟数 |
| executionTime | string | Y | 每天执行时间 |
| startTime / endTime | string | Y | 生效时间窗 |
| remark | string | Y | 备注 |
| tenantId | int | Y | 租户 |
| createBy | string | Y | 创建人 |
| createTime | string | N | 创建时间 |
| updateTime | string | Y | 更新时间 |

**枚举：脚本计划状态**

| 值 | 含义 |
|---|---|
| `NOT_STARTED` | 未开始 |
| `PAUSED` | 暂停 |
| `ENABLING` | 启用中 |
| `FINISHED` | 已结束 |

---

<a id="612-script-plan-update"></a>
### 6.12 更新脚本计划

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/scriptPlan/update |
| 中台 controller | [ScriptPlanController.updatePlan](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptPlanController.java#L58)（`/scriptPlan/update`） |
| 超时 | 30s |

**请求**

请求体在 [§6.11 创建](#611-script-plan-create) 字段基础上增加 `id` 主键。

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 计划主键 |
| ...（其余同 §6.11） | | | |

**响应**

返回更新后的计划对象，结构同 [§6.11 响应](#611-script-plan-create)。

---

<a id="613-script-plan-page"></a>
### 6.13 脚本计划分页

**接口信息**

| | |
|---|---|
| HTTP | POST /open/api/autoScript/scriptPlan/list |
| 中台 controller | [ScriptPlanController.listPlans](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptPlanController.java#L94)（`/scriptPlan/list`） |
| 超时 | 30s |
| 业务时机 | 计划列表页 |

**请求**

| 字段 | 类型 | 必填 | 描述 / 枚举 |
|---|---|---|---|
| page | int | ✓ | 页码 |
| pageSize | int | ✓ | 每页条数 |
| planStatus | string |  | 见 [§6.11 **枚举：脚本计划状态**](#611-script-plan-create) |
| executionFrequency | string |  | `INTERVAL` / `DAILY` |
| startTimeBegin | string |  | 生效起始时间窗起 |
| startTimeEnd | string |  | 生效起始时间窗止 |
| endTimeBegin | string |  | 生效结束时间窗起 |
| endTimeEnd | string |  | 生效结束时间窗止 |
| tenantId | int |  | 租户 ID |

**响应**

`data` 字段（每条结构在 §6.11 响应基础上加 3 个聚合字段）：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| pageNum / pageSize / totalSize | int/int/number | N |  |
| data | object[] | N | 计划列表 |
| data[].* | | | 同 [§6.11 响应字段](#611-script-plan-create) |
| data[].taskCount | int | Y | 关联的任务总数 |
| data[].successTaskCount | int | Y | 成功任务数（状态 COMPLETED）|
| data[].parentTenants | string | Y | 父租户组 |

---

<a id="614-script-plan-toggle"></a>
### 6.14 启动 / 暂停 / 删除脚本计划

> 这三个端点共用同一种风格：**POST + Query 参数 `id`，无 body**。

**接口信息**

| 业务动作 | HTTP | 中台 controller |
|---|---|---|
| 启动计划 | POST /open/api/autoScript/scriptPlan/start | [ScriptPlanController.startPlan](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptPlanController.java#L76) |
| 暂停计划 | POST /open/api/autoScript/scriptPlan/pause | [ScriptPlanController.pausePlan](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptPlanController.java#L85) |
| 删除计划 | POST /open/api/autoScript/scriptPlan/delete | [ScriptPlanController.deletePlan](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptPlanController.java#L67) |

**请求**

Query 参数：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 计划主键 |

请求示例：

```
POST /open/api/autoScript/scriptPlan/start?id=12345
```

**响应**

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "abc-123" }
```

> 启动 / 暂停幂等；删除时若中台已无该 plan 会返回 500，调用方需先确认本地状态。

---

<a id="615-script-plan-detail"></a>
### 6.15 查脚本计划详情

**接口信息**

| | |
|---|---|
| HTTP | GET /open/api/autoScript/scriptPlan/detail |
| 中台 controller | [ScriptPlanController.getPlanDetail](../../../cloudphone-autoscript-service/src/main/java/com/vdmanager/autoscript/controller/ScriptPlanController.java#L103)（`/scriptPlan/detail`） |
| 超时 | 30s |

**请求**

Query 参数：

| 字段 | 类型 | 必填 | 描述 |
|---|---|---|---|
| id | number | ✓ | 计划主键 |

**响应**

`data` 字段：

| 字段 | 类型 | 可空 | 描述 |
|---|---|---|---|
| (基础字段同 [§6.11 响应](#611-script-plan-create)) | | | |
| taskCount | int | Y | 关联任务总数 |
| successTaskCount | int | Y | 成功任务数 |
| parentTenants | string | Y | 父租户组 |
| taskList | object[] | Y | 关联任务详情（结构同 [§6.5 ScriptTaskVO](#65-query-tasks-by-ids)）|

---

## 附录 A：响应状态码

| code | 含义 |
|---|---|
| `200` | 成功 |
| `500` | 业务失败（CallResult.fail）|
| `FAIL` | 通用错误（BaseErrorEnum.FAIL） |
| `DATA_NOT_EXIST` | 资源不存在 |
| `OPERATION_FAILED` | 操作失败 |
| 其他枚举 | 见各 controller 自定义 ErrorEnum |

## 附录 B：超时分组建议

| 类别 | 端点举例 | 建议超时 |
|---|---|---|
| 常规读 / 写 | 大部分接口 | 30s |
| 文件上传 / 批量代理下发 / 批量探测 | [§1.4](#14-upload-part) / [§4.5](#45-batch-update-proxy) / [§4.13](#413-batch-detect-proxy) | 300s |
| 流式下载 | [§3.14](#314-download-file) | 300s |
| 带断路器的查询 | 任务批量查询 | 30s + 断路器 |

## 附录 C：常见数据约定

| 内容 | 约定 |
|---|---|
| 时间格式 | LocalDateTime `2006-01-02T15:04:05`（无 TZ，北京时间）；少量字符串字段是 `2026-06-01 10:00:00` |
| 分页字段 | 请求里是 `page`；响应里是 `pageNum` |
| 状态枚举 | 多为 string，部分为 int；遇到字段名带 Desc 后缀的是人类可读描述 |
| 主键 ID | 中台多用 Long (int64) 表达，跨语言时建议 string 化避免精度丢失 |
| 代理协议 | wire 形态：HTTP / SOCKS5 / HTTPS（大写）；type=0 是 HTTP，type=1 是 SOCKS5 |
| 国家字段 | `country` 是 ISO 代码（"US"），`countryName` 是中文/英文名 |


## 附录 D：curl 调用速查（6 个高频端点）

下面用 curl 直接对中台 gateway 调试。`$AK`/`$SK` 是调用方分配的 AKSK，签名串规则：

```
$TS = unix 毫秒
$SIG = HMAC-SHA256(SK, "{method}\n{path}\n{TS}\n{body_md5}")
```

生产环境通常用客户端 SDK 自动签名，避免手动拼。

### D.1 应用列表分页

```bash
curl -X POST https://midplat.example.com/open/api/vendor/v1/app/page \
  -H "X-Access-Key: $AK" \
  -H "X-Timestamp: $TS" \
  -H "X-Signature: $SIG" \
  -H "X-Internal-Request: 1" \
  -H "Content-Type: application/json" \
  -d '{"page":1,"pageSize":20,"appName":"微信"}'
```

成功响应（示例）：

```json
{
  "code": "200",
  "message": "成功",
  "data": {
    "total": 3,
    "records": [
      {
        "id": "12345",
        "appCode": "wechat-app-xxxx",
        "appName": "微信",
        "packageName": "com.tencent.mm",
        "version": "8.0.42",
        "iconUrl": "https://oss.example.com/icons/wechat.png",
        "fileSize": "256MB",
        "createUser": "alice",
        "createTime": "2026-04-12 10:23:45"
      }
    ]
  },
  "traceId": "abc-123-def"
}
```

### D.2 启动 APK 分片上传（含秒传）

```bash
curl -X POST https://midplat.example.com/open/api/vendor/v1/app/upload/initiate-app \
  -H "X-Access-Key: $AK" -H "X-Timestamp: $TS" -H "X-Signature: $SIG" \
  -H "X-Internal-Request: 1" -H "Content-Type: application/json" \
  -d '{
    "fileName": "wechat-8.0.42.apk",
    "fileSize": 268435456,
    "contentMd5": "5d41402abc4b2a76b9719d911017c592",
    "chunkSize": 5242880
  }'
```

普通响应：

```json
{
  "code": "200", "message": "成功",
  "data": {
    "uploadId": "upl-abc-12345",
    "totalParts": 52,
    "partSize": 5242880,
    "uploadSuccess": false
  },
  "traceId": "..."
}
```

秒传命中响应：

```json
{
  "code": "200", "message": "成功",
  "data": {
    "uploadId": null,
    "totalParts": 0,
    "partSize": 0,
    "uploadSuccess": true,
    "appInfo": {
      "appCode": "wechat-app-xxxx",
      "appName": "微信",
      "packageName": "com.tencent.mm",
      "version": "8.0.42",
      "md5": "5d41402abc4b2a76b9719d911017c592",
      "downloadUrl": "https://oss.example.com/wechat-8.0.42.apk"
    }
  },
  "traceId": "..."
}
```

### D.3 创建云手机

```bash
curl -X POST https://midplat.example.com/open/api/vendor/v1/cp/create \
  -H "X-Access-Key: $AK" -H "X-Timestamp: $TS" -H "X-Signature: $SIG" \
  -H "X-Internal-Request: 1" -H "Content-Type: application/json" \
  -d '{
    "vmId": "vm-aaa-111",
    "imageId": "img-android11-prod",
    "planId": 23,
    "number": 5,
    "scenarioId": 7,
    "resourceUtilization": "SHARED",
    "minCore": 1.0, "maxCore": 2.0,
    "minMemory": 2048, "maxMemory": 4096,
    "storage": 16,
    "bandwidth": 50,
    "width": 720, "height": 1280,
    "resolution": "720*1280",
    "fps": 30,
    "phoneModelId": 1001,
    "androidVersion": "11",
    "regionOption": "FOLLOW_PROXY",
    "timezoneOption": "FOLLOW_PROXY",
    "languageOption": "FOLLOW_PROXY",
    "networkType": 0,
    "proxyMode": null,
    "proxyConfigs": [],
    "appIds": [12345, 12346]
  }'
```

响应：

```json
{
  "code": "200", "message": "成功",
  "data": {
    "cpList": ["cp-xxx-001", "cp-xxx-002", "cp-xxx-003", "cp-xxx-004", "cp-xxx-005"]
  },
  "traceId": "..."
}
```

### D.4 批量下发代理到云手机

```bash
curl -X POST https://midplat.example.com/open/api/vendor/v1/cloud-phone/proxy/batch-update \
  -H "X-Access-Key: $AK" -H "X-Timestamp: $TS" -H "X-Signature: $SIG" \
  -H "X-Internal-Request: 1" -H "Content-Type: application/json" \
  -d '{
    "cpIds": [
      {
        "cpId": "cp-xxx-001",
        "proxy": {
          "proxyId": 9001,
          "proxyOrderType": "STATIC",
          "host": "us-proxy.ipvibe.io",
          "port": 1080,
          "username": "tenant-7-key",
          "password": "xxxxx",
          "type": 1
        }
      },
      { "cpId": "cp-xxx-002", "proxy": null }
    ],
    "regionFollowProxy": true,
    "timezoneFollowProxy": true,
    "languageFollowProxy": true
  }'
```

响应：

```json
{ "code": "200", "message": "成功", "data": null, "traceId": "..." }
```

> ⚠️ 这条接口是同步的但 push 中台串行下发可能 30-60s；建议调用方放大超时到 300s。仿真主数据更新是后续异步事件链。

### D.5 v2 创建定时脚本任务

```bash
curl -X POST https://midplat.example.com/open/api/autoScript/autoscript/task/create-scheduled \
  -H "X-Access-Key: $AK" -H "X-Timestamp: $TS" -H "X-Signature: $SIG" \
  -H "X-Internal-Request: 1" -H "Content-Type: application/json" \
  -d '{
    "scriptId": 88,
    "taskName": "TK 养号 - 早班",
    "remark": "{\"tenantUid\":\"t-7\"}",
    "taskList": [
      {
        "cpId": "cp-xxx-001",
        "publishTime": "2026-06-04T09:00:00",
        "scriptParams": "{\"username\":\"@tk_alice\"}"
      },
      {
        "cpId": "cp-xxx-002",
        "publishTime": "2026-06-04T09:00:00",
        "scriptParams": "{\"username\":\"@tk_bob\"}"
      }
    ]
  }'
```

响应：

```json
{
  "code": "200", "message": "成功",
  "data": [
    { "id": 555001, "taskId": "tk-task-aaa", "cpId": "cp-xxx-001",
      "planPublishTime": "2026-06-04T09:00:00" },
    { "id": 555002, "taskId": "tk-task-bbb", "cpId": "cp-xxx-002",
      "planPublishTime": "2026-06-04T09:00:00" }
  ],
  "traceId": "..."
}
```

> `id` 是 mid_task_id (int64)，存到本地 `account_task_details.mid_task_id`，作为后续 ids 查询 / 取消 / 删除 的入参。

### D.6 按 ids 批量查任务（Poller 常用）

```bash
curl -X POST https://midplat.example.com/open/api/autoScript/autoscript/task/query-by-ids \
  -H "X-Access-Key: $AK" -H "X-Timestamp: $TS" -H "X-Signature: $SIG" \
  -H "X-Internal-Request: 1" -H "Content-Type: application/json" \
  -d '{ "ids": [555001, 555002] }'
```

响应：

```json
{
  "code": "200", "message": "成功",
  "data": [
    {
      "id": 555001,
      "taskId": "tk-task-aaa",
      "taskName": "TK 养号 - 早班",
      "cpId": "cp-xxx-001",
      "vmUid": "vm-aaa-111",
      "taskStatus": "SUCCESS",
      "taskStatusDesc": "执行成功",
      "planUid": null,
      "scriptId": 88,
      "tenantId": 7,
      "tenantName": "alice-corp",
      "scriptParam": "{\"username\":\"@tk_alice\"}",
      "remark": "{\"success\":true,\"watch_video_counts\":24,\"comments\":3}",
      "planPublishTime": "2026-06-04T09:00:00",
      "runStartTime": "2026-06-04T09:00:15",
      "runEndTime": "2026-06-04T09:08:42",
      "runDuration": 507000,
      "execResult": 1,
      "createTime": "2026-06-04T08:59:01",
      "updateTime": "2026-06-04T09:08:43"
    }
  ],
  "traceId": "..."
}
```

`remark` 字段是业务自定义 JSON 字符串,调用方按业务结构二次解析。

---

## 附录 E：按 wire path 字母索引

按 HTTP 路径字母排序，便于全文检索后定位章节：

| HTTP 路径 | Method | 章节 |
|---|---|---|
| `/open/api/autoScript/api/automation/templates/page` | POST | [§6.1 脚本模板分页](#61-list-script-templates) |
| `/open/api/autoScript/autoscript/task/cancel` | POST | [§6.6 取消任务](#66-cancel-task) |
| `/open/api/autoScript/autoscript/task/cancel-by-cpids` | POST | [§6.7 按 cpId 批量取消](#67-cancel-tasks-by-cpids) |
| `/open/api/autoScript/autoscript/task/create-scheduled` | POST | [§6.2 创建定时任务 ⭐](#62-create-scheduled-tasks) |
| `/open/api/autoScript/autoscript/task/delete-by-ids` | POST | [§6.10 批量删任务](#610-delete-tasks-by-ids) |
| `/open/api/autoScript/autoscript/task/page` | POST | [§6.3 任务分页](#63-task-page) |
| `/open/api/autoScript/autoscript/task/query-by-ids` | POST | [§6.5 按 ids 查任务](#65-query-tasks-by-ids) |
| `/open/api/autoScript/autoscript/task/re-execute` | POST | [§6.8 重新执行任务](#68-re-execute-task) |
| `/open/api/autoScript/autoscript/task/report` | POST | [§6.9 查任务报告](#69-query-task-report) |
| `/open/api/autoScript/autoscript/task/update-publish-time` | POST | [§6.4 改发布时间](#64-update-publish-time) |
| `/open/api/autoScript/scriptPlan/create` | POST | [§6.11 创建脚本计划](#611-script-plan-create) |
| `/open/api/autoScript/scriptPlan/delete?id={id}` | POST | [§6.14 启停删除计划](#614-script-plan-toggle) |
| `/open/api/autoScript/scriptPlan/detail?id={id}` | GET | [§6.15 计划详情](#615-script-plan-detail) |
| `/open/api/autoScript/scriptPlan/list` | POST | [§6.13 计划分页](#613-script-plan-page) |
| `/open/api/autoScript/scriptPlan/pause?id={id}` | POST | [§6.14 启停删除计划](#614-script-plan-toggle) |
| `/open/api/autoScript/scriptPlan/start?id={id}` | POST | [§6.14 启停删除计划](#614-script-plan-toggle) |
| `/open/api/autoScript/scriptPlan/update` | POST | [§6.12 更新计划](#612-script-plan-update) |
| `/open/api/vendor/v1/adb/operate` | POST | [§3.1 ADB 统一操作](#31-adb-operate) |
| `/open/api/vendor/v1/adb/whitelist/cp/{cpId}` | GET | [§3.2 查 ADB 白名单](#32-get-adb-whitelist) |
| `/open/api/vendor/v1/app/batch/deleteAppInfo` | POST | [§1.2 批量删除应用](#12-delete-apps) |
| `/open/api/vendor/v1/app/checkAppNameExists` | POST | [§1.7 重名校验](#17-check-app-name) |
| `/open/api/vendor/v1/app/createFromUploadedFile` | POST | [§1.8 创建应用](#18-create-app) |
| `/open/api/vendor/v1/app/getAppInfoFromFile` | POST | [§1.6 解析 APK](#16-parse-app) |
| `/open/api/vendor/v1/app/page` | POST | [§1.1 应用列表分页](#11-list-apps) |
| `/open/api/vendor/v1/app/queryUploadStatus` | POST | [§1.9 查询上传状态](#19-query-upload-status) |
| `/open/api/vendor/v1/app/upload/initiate-app` | POST | [§1.3 启动分片上传](#13-initiate-upload) |
| `/open/api/vendor/v1/cloud-phone/batch-query-status` | POST | [§2.15 批量查云机状态](#215-batch-query-status) |
| `/open/api/vendor/v1/cloud-phone/batchRefreshPhone` | POST | [§2.9 一键刷新](#29-reset) |
| `/open/api/vendor/v1/cloud-phone/delete-tag` | POST | [§3.9 删标签](#39-delete-tag) |
| `/open/api/vendor/v1/cloud-phone/destroy` | POST | [§2.8 批量销毁](#28-destroy) |
| `/open/api/vendor/v1/cloud-phone/edit-tag` | POST | [§3.8 编辑标签](#38-edit-tag) |
| `/open/api/vendor/v1/cloud-phone/getInstalledApps` | POST | [§2.13 查已装应用](#213-get-installed-apps) |
| `/open/api/vendor/v1/cloud-phone/installApp` | POST | [§2.11 批量装应用](#211-install-app) |
| `/open/api/vendor/v1/cloud-phone/overview-statistics` | GET | [§3.15 大屏统计](#315-overview-statistics) |
| `/open/api/vendor/v1/cloud-phone/page` ⭐ | POST | [§2.16 v2 云手机分页](#216-list-open-cloud-phones) |
| `/open/api/vendor/v1/cloud-phone/proxy/batch-update` | POST | [§4.5 批量下发代理 ⭐](#45-batch-update-proxy) |
| `/open/api/vendor/v1/cloud-phone/recycle-phone` | POST | [§2.10 回收云手机](#210-recycle) |
| `/open/api/vendor/v1/cloud-phone/restart` | POST | [§2.7 批量重启](#27-restart) |
| `/open/api/vendor/v1/cloud-phone/uninstallApp` | POST | [§2.12 批量卸应用](#212-uninstall-app) |
| `/open/api/vendor/v1/cloud-phone/updateImage` | POST | [§3.10 切镜像](#310-update-image) |
| `/open/api/vendor/v1/cloud-phone/update-webrtc-channels` | POST | [§3.7 改 WebRTC 通道](#37-update-webrtc-channels) |
| `/open/api/vendor/v1/cloud-phone/webrtc-auth` | POST | [§2.14 WebRTC 鉴权](#214-webrtc-auth) |
| `/open/api/vendor/v1/cp/create` | POST | [§2.1 创建云手机](#21-create-cloud-phone) |
| `/open/api/vendor/v1/cp/page` | POST | [§2.2 / §2.3 / §2.4 v1 兜底](#22-list-cloud-phones-v1) |
| `/open/api/vendor/v1/cp/shutdown` | POST | [§2.6 批量关机](#26-shutdown) |
| `/open/api/vendor/v1/cp/start` | POST | [§2.5 批量开机](#25-start) |
| `/open/api/vendor/v1/custom/proxy/add` | POST | [§4.6 新建自定义代理](#46-add-custom-proxy) |
| `/open/api/vendor/v1/custom/proxy/batch/delete` | POST | [§4.7 批量删自定义代理](#47-batch-delete-custom-proxy) |
| `/open/api/vendor/v1/custom/proxy/manual/page` | POST | [§4.3 自定义代理分页](#43-list-custom-proxies) |
| `/open/api/vendor/v1/dynamic/proxy/page` | POST | [§4.2 动态代理分页](#42-list-dynamic-proxies) |
| `/open/api/vendor/v1/files/upload/complete-new` | POST | [§1.5 完成分片合并](#15-complete-upload) |
| `/open/api/vendor/v1/files/upload/part-new` | POST | [§1.4 上传分片](#14-upload-part) |
| `/open/api/vendor/v1/img/query/by-plan/{planId}` | GET | [§5.7 按方案查镜像](#57-list-images-by-plan) |
| `/open/api/vendor/v1/img/query/phone-models/{imageId}` | GET | [§5.8 按镜像查机型](#58-list-phone-models-by-image) |
| `/open/api/vendor/v1/network-strategy/query/by-phone/{cpId}` | GET | [§3.3 查网络策略](#33-get-network-strategy) |
| `/open/api/vendor/v1/network-strategy/update/bandwidth` | POST | [§3.4 改限速](#34-update-bandwidth) |
| `/open/api/vendor/v1/phone-command/batch-upload` | POST | [§3.13 批量上传](#313-batch-upload-files) |
| `/open/api/vendor/v1/phone-command/file-delete` | POST | [§3.12 删文件](#312-delete-file) |
| `/open/api/vendor/v1/phone-command/file-download-stream` | POST | [§3.14 下载文件](#314-download-file) |
| `/open/api/vendor/v1/phone-command/file-list` | POST | [§3.11 列目录](#311-list-files) |
| `/open/api/vendor/v1/plan-mng/list/by-spec-and-scenario` | POST | [§5.6 按规格查启动方案](#56-list-boot-plans-by-spec) |
| `/open/api/vendor/v1/proxy-management/purchase` | POST | [§4.12 代理下单](#412-purchase-proxies) |
| `/open/api/vendor/v1/proxy-management/static/meals` | POST | [§4.8 静态套餐](#48-list-static-meals) |
| `/open/api/vendor/v1/proxy-management/static/stock` | POST | [§4.10 静态库存](#410-list-static-stock) |
| `/open/api/vendor/v1/proxy-management/tiktok/ip/list` | POST | [§4.9 TikTok 套餐](#49-list-tiktok-meals) |
| `/open/api/vendor/v1/proxy-management/tiktok/stock` | POST | [§4.11 TikTok 库存](#411-list-tiktok-stock) |
| `/open/api/vendor/v1/proxy/batch-detect` | POST | [§4.13 批量代理检测](#413-batch-detect-proxy) |
| `/open/api/vendor/v1/resource-group/all-spec-stocks` | GET | [§5.3 全规格库存](#53-all-spec-stocks) |
| `/open/api/vendor/v1/resource-group/apply` | POST | [§5.1 申请资源组](#51-apply-resource) |
| `/open/api/vendor/v1/resource-group/max-openable` | GET | [§5.2 查最大可开通](#52-max-openable) |
| `/open/api/vendor/v1/server/page` | POST | [§5.4 服务器分页](#54-list-servers) |
| `/open/api/vendor/v1/static/proxy/page` | POST | [§4.1 静态代理分页](#41-list-static-proxies) |
| `/open/api/vendor/v1/storage-info/edit-phone` | POST | [§3.5 改存储](#35-update-storage) |
| `/open/api/vendor/v1/tencent/instance/updateName` | POST | [§3.6 改名称](#36-update-name) |
| `/open/api/vendor/v1/tiktok/proxy/page` | POST | [§4.4 TikTok 代理分页](#44-list-tiktok-proxies) |
| `/open/api/vendor/v1/virtual-machine/reset` | POST | [§5.5 重置虚机](#55-reset-vm) |
