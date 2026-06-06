// Package cloudphone 是「云手机中台接入」模块的公开入口与契约。
//
// 模块实现位于 modules/cloudphone/internal，受 Go internal 机制保护；公开包仅用于在 main 中
// blank-import 以触发自注册。当前阶段只发布后台只读资源浏览接口（规格 / 云虚机 / 镜像 / 应用），
// 进页面即实时透传到云手机中台（midplat SDK），不落本地库、不做同步缓存。
package cloudphone

import _ "manager-backend/modules/cloudphone/internal"
