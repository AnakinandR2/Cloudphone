// Package apptest 存放应用级集成测试（HTTP 层）。
//
// 这些测试横跨多个模块（鉴权 + 业务路由），按 Modulith 原则只能依赖各模块「发布的
// 公开 API」——它无法、也不应 import 任何模块的 internal 实现。因此测试通过 HTTP
// 登录（内置 admin）获取令牌、经 REST 接口准备数据，而非直接调用内部服务。
//
// 模块内的数据/过滤/排序等细粒度正确性，由各模块 internal 包内的单元测试覆盖。
package apptest
