> 模块：应用管理 / OpenAPI  
> 服务：cloudphone-vendor-service  
> 更新时间：2026-06-26

## 1. 接口概述

该接口用于外部系统直接传入应用下载 URL、MD5、包名、版本等信息，并指定云手机集合，由中台向底层小西云手机发起应用安装任务。

接口为异步下发接口：调用成功表示安装任务已提交，不代表应用已安装完成。调用方可根据返回的 `taskInfoList[].taskId` 查询后续任务状态。

## 2. 请求信息

| 项目 | 内容 |
|---|---|
| HTTP Method | `POST` |
| 外部访问路径 | `/open/api/vendor/v1/cp/apps/install-by-url` |
| 服务相对路径 | `/cp/apps/install-by-url` |
| Content-Type | `application/json` |
| 鉴权方式 | AKSK 签名 |

## 3. 请求头

| Header | 必填 | 说明 |
|---|---:|---|
| `X-Access-Key` | 是 | 中台分配的 AccessKey |
| `X-Timestamp` | 是 | Unix 毫秒时间戳，建议与服务端时间误差不超过 5 分钟 |
| `X-Signature` | 是 | HMAC-SHA256 签名 |
| `X-Nonce` | 建议 | 随机字符串；如接入方签名规范启用 nonce，需要携带 |
| `X-Trace-Id` | 否 | 调用方生成的链路追踪 ID，排障时建议提供 |
| `Content-Type` | 是 | 固定为 `application/json` |

> 租户信息由中台根据 AKSK 上下文自动识别，调用方不需要在请求体中传 `tenantId`。

## 4. 请求参数

| 字段 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| `cpIds` | `string[]` | 是 | 云手机实例编号列表，至少 1 个；重复编号会被中台去重 |
| `apps` | `object[]` | 是 | 待安装应用列表，至少 1 个 |
| `apps[].appName` | `string` | 否 | 应用名称 |
| `apps[].downloadUrl` | `string` | 是 | 应用下载地址，必须以 `http://` 或 `https://` 开头 |
| `apps[].md5` | `string` | 是 | 应用文件 MD5，32 位十六进制字符串，大小写均可 |
| `apps[].packageName` | `string` | 是 | Android 应用包名 |
| `apps[].version` | `string` | 是 | 应用版本 |
| `apps[].fileSize` | `string` | 否 | 应用文件大小，展示或透传字段，例如 `12MB` |

## 5. 请求示例

```bash
curl -X POST "https://midplat.example.com/open/api/vendor/v1/cp/apps/install-by-url" \
  -H "Content-Type: application/json" \
  -H "X-Access-Key: ${AK}" \
  -H "X-Timestamp: ${TS}" \
  -H "X-Nonce: ${NONCE}" \
  -H "X-Signature: ${SIG}" \
  -H "X-Trace-Id: trace-install-url-001" \
  -d '{
    "cpIds": ["cp-aaa-001", "cp-bbb-002"],
    "apps": [
      {
        "appName": "Demo App",
        "downloadUrl": "https://example.com/apk/demo.apk",
        "md5": "0123456789abcdef0123456789abcdef",
        "packageName": "com.example.demo",
        "version": "1.0.0",
        "fileSize": "12MB"
      }
    ]
  }'
```

## 6. 成功响应

当前代码成功时返回 `CallResult<AsyncOperationResponse>`，`data.taskInfoList` 中包含每台云手机对应的异步任务信息。

```json
{
  "code": "SUCCESS",
  "message": "成功",
  "data": {
    "taskInfoList": [
      {
        "taskId": "wf-install-aaa",
        "instanceId": "cp-aaa-001"
      },
      {
        "taskId": "wf-install-aaa",
        "instanceId": "cp-bbb-002"
      }
    ]
  },
  "traceId": null
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|---|---|---|
| `code` | `string` | 业务结果码，成功为 `SUCCESS` |
| `message` | `string` | 结果说明 |
| `data.taskInfoList` | `object[]` | 异步任务信息列表 |
| `data.taskInfoList[].taskId` | `string` | 异步任务 ID；小西云场景下对应底层返回的 workflow ID |
| `data.taskInfoList[].instanceId` | `string` | 云手机实例编号，即请求中的 `cpId` |
| `traceId` | `string` | 链路追踪 ID，可能为空 |

## 7. 失败响应

```json
{
  "code": "FAIL",
  "message": "仅支持小西云手机: cp-xxx-001",
  "data": null,
  "traceId": null
}
```

常见失败原因：

| 场景 | 返回说明示例 |
|---|---|
| AKSK 未识别到租户 | `获取租户ID失败` |
| 云手机不存在或当前租户无权限 | `手机不存在或无权限: cp-xxx-001` |
| 云手机不是小西云来源 | `仅支持小西云手机: cp-xxx-001` |
| 云手机状态不允许应用管理 | `云手机状态为...，不允许应用管理操作` |
| 下载地址格式错误 | `应用下载地址必须是http或https地址` |
| MD5 格式错误 | `应用MD5格式不正确` |
| 底层安装接口下发失败 | `URL安装应用失败: ...` |

## 8. 任务状态查询

该接口是异步任务下发接口。调用方拿到 `taskInfoList[].taskId` 后，可按工作流 ID 查询任务总览或任务明细。

推荐查询接口：

| 用途 | 接口 |
|---|---|
| 批量查询工作流状态 | `POST /open/api/vendor/v1/api-task/workflow/query-by-codes` |
| 查询任务明细 | `POST /open/api/vendor/v1/api-task/detail/page` |

查询建议：

- 轮询间隔建议不低于 5 秒。
- `taskId` 可作为工作流查询接口中的 `code` 使用。
- 如需定位具体哪台云手机失败，可使用任务明细接口按 `workflowId` 查询。

## 9. 接入注意事项

- 当前接口只支持小西云手机。
- 调用方只能操作当前 AKSK 租户有权限的云手机。
- 接口不会把应用写入中台应用库，也不会创建 `app_info` 记录。
- 调用方需要保证 `downloadUrl` 可被底层安装环境访问。
- 调用方需要保证 `md5` 与下载文件一致，否则底层安装可能失败。
- 一次请求可以传多个云手机、多个应用；中台会按可用区和服务器分组下发。
- 成功响应只代表任务提交成功，最终安装结果以任务状态或云手机已安装应用列表为准。

