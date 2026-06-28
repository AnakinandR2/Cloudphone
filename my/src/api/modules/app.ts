import type { MarketApp, UserApp } from '@/types/app'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 应用管理：用户应用 = 素材库 app 文件 + meta（按用户隔离，需登录）；市场 = 平台资产（只读浏览）。
export default {
  // 当前用户应用列表（library app 文件 LEFT JOIN app_user_meta）。
  userList: () => api.get<unknown, R<UserApp[]>>('app/user'),

  // 应用市场：仅 parse_status=ready 的平台应用，供浏览安装。
  market: () => api.get<unknown, R<MarketApp[]>>('app/market'),

  // 上传 confirm 成功后调用：服务端回读对象解析元数据 + 图标 → 写 app_user_meta，返回该应用 DTO。
  finalize: (fileId: number) =>
    api.post<unknown, R<UserApp>>(`app/user/${fileId}/finalize`),

  // 批量删除：走素材库删文件（释放配额）+ 删 meta。
  userBatchDelete: (fileIds: number[]) =>
    api.post<unknown, R<null>>('app/user/batch-delete', { file_ids: fileIds }),
}
