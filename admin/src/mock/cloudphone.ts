import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

// ---- 可用区 ----
const zones = [
  { id: 1, name: '华北-北京', regionName: 'cn-north-1', supplier: '火山引擎', zoneCode: 'cn-north-1a', createTime: '2026-01-10T09:12:31' },
  { id: 2, name: '华东-上海', regionName: 'cn-east-2', supplier: '阿里云', zoneCode: 'cn-east-2b', createTime: '2026-02-18T14:05:07' },
  { id: 3, name: '华南-广州', regionName: 'cn-south-1', supplier: '腾讯云', zoneCode: 'cn-south-1c', createTime: '2026-03-02T11:40:55' },
]

// ---- 规格：按 zoneId + kind 返回 ----
const phoneSpecsByZone: Record<number, any[]> = {
  1: [
    { id: 101, name: 'cp.standard.2c4g', supplier: '火山引擎', core: 2, memory: 4, storage: 32, type: 'STANDARD', feature: '通用型', createTime: '2026-01-12T10:00:00' },
    { id: 102, name: 'cp.standard.4c8g', supplier: '火山引擎', core: 4, memory: 8, storage: 64, type: 'STANDARD', feature: '通用型', createTime: '2026-01-12T10:05:00' },
    { id: 103, name: 'cp.game.8c16g', supplier: '火山引擎', core: 8, memory: 16, storage: 128, type: 'GAME', feature: '高帧游戏', createTime: '2026-01-15T16:20:00' },
  ],
  2: [
    { id: 201, name: 'cp.lite.2c3g', supplier: '阿里云', core: 2, memory: 3, storage: 32, type: 'LITE', feature: '轻量', createTime: '2026-02-20T09:00:00' },
    { id: 202, name: 'cp.standard.4c6g', supplier: '阿里云', core: 4, memory: 6, storage: 64, type: 'STANDARD', feature: '通用型', createTime: '2026-02-20T09:10:00' },
  ],
  3: [
    { id: 301, name: 'cp.standard.4c8g', supplier: '腾讯云', core: 4, memory: 8, storage: 64, type: 'STANDARD', feature: '通用型', createTime: '2026-03-05T08:30:00' },
    { id: 302, name: 'cp.render.16c32g', supplier: '腾讯云', core: 16, memory: 32, storage: 256, type: 'RENDER', feature: '云渲染', createTime: '2026-03-06T13:00:00' },
  ],
}

const vmSpecsByZone: Record<number, any[]> = {
  1: [
    { id: 1101, name: 'vm.gpu.30c112g', supplier: '火山引擎', core: 30, memory: 112, storage: 700, card: 'NVIDIA A10', maxPhone: 60, type: 'GPU', feature: 'GPU 虚机', hasVM: true, createTime: '2026-01-11T10:00:00' },
    { id: 1102, name: 'vm.gpu.60c224g', supplier: '火山引擎', core: 60, memory: 224, storage: 1400, card: 'NVIDIA A10*2', maxPhone: 120, type: 'GPU', feature: 'GPU 虚机', hasVM: true, createTime: '2026-01-11T10:30:00' },
  ],
  2: [
    { id: 1201, name: 'vm.gpu.32c128g', supplier: '阿里云', core: 32, memory: 128, storage: 800, card: 'NVIDIA T4', maxPhone: 64, type: 'GPU', feature: 'GPU 虚机', hasVM: true, createTime: '2026-02-19T09:00:00' },
    { id: 1202, name: 'vm.cpu.16c64g', supplier: '阿里云', core: 16, memory: 64, storage: 400, card: '-', maxPhone: 32, type: 'CPU', feature: '纯 CPU', hasVM: false, createTime: '2026-02-19T09:20:00' },
  ],
  3: [
    { id: 1301, name: 'vm.gpu.48c192g', supplier: '腾讯云', core: 48, memory: 192, storage: 1200, card: 'NVIDIA A100', maxPhone: 96, type: 'GPU', feature: 'GPU 虚机', hasVM: true, createTime: '2026-03-04T08:00:00' },
  ],
}

// ---- 云主机（对齐中台 §5.4 /server/page，租户已分配服务器）----
const vms = [
  { id: 1, vmUid: 'VM00000924', vmId: 'vm-bj-0001', vmIp: '10.30.12.11', vmStatus: 'ONLINE', isMaintain: false, specificationId: 30, specificationName: 'vm.gpu.30c112g', core: 30, memory: 112, storage: 700, bandwidth: 100, maxStartCount: 48, maxPhone: 60, createPhoneNumber: 48, createTime: '2026-04-01 08:19:57', expireTime: '2026-10-01 08:19:57', expired: false },
  { id: 2, vmUid: 'VM00000925', vmId: 'vm-sh-0001', vmIp: '10.40.5.21', vmStatus: 'OFFLINE', isMaintain: false, specificationId: 32, specificationName: 'vm.gpu.32c128g', core: 32, memory: 128, storage: 800, bandwidth: 100, maxStartCount: 60, maxPhone: 64, createPhoneNumber: 0, createTime: '2026-04-05 15:42:33', expireTime: '2026-10-05 15:42:33', expired: false },
]

const vmStatuses = [
  { ONLINE: '在线' },
  { OFFLINE: '离线' },
  { DESTROYED: '已销毁' },
]

// ---- 镜像 ----
const images = [
  { id: 1, imageId: 'img-android13-001', imageName: 'Android 13 标准版', imageVersion: '13.0.2', isForbid: 0, downloadUrl: 'https://cdn.cloudphone.example.com/images/android13-std-13.0.2.img', createTime: '2026-01-20T10:00:00', updateTime: '2026-03-15T12:30:00' },
  { id: 2, imageId: 'img-android12-002', imageName: 'Android 12 游戏版', imageVersion: '12.1.0', isForbid: 0, downloadUrl: 'https://cdn.cloudphone.example.com/images/android12-game-12.1.0.img', createTime: '2026-02-01T09:00:00', updateTime: '2026-02-28T16:10:00' },
  { id: 3, imageId: 'img-android11-003', imageName: 'Android 11 兼容版', imageVersion: '11.0.5', isForbid: 1, downloadUrl: 'https://cdn.cloudphone.example.com/images/android11-compat-11.0.5.img', createTime: '2025-11-10T08:00:00', updateTime: '2026-01-05T11:20:00' },
  { id: 4, imageId: 'img-android14-004', imageName: 'Android 14 预览版', imageVersion: '14.0.0-beta', isForbid: 0, downloadUrl: 'https://cdn.cloudphone.example.com/images/android14-beta-14.0.0.img', createTime: '2026-05-01T14:00:00', updateTime: '2026-05-20T09:45:00' },
]

// ---- 应用市场 ----
const appNames = [
  ['微信', 'com.tencent.mm', '8.0.49', 'SOCIAL'],
  ['抖音', 'com.ss.android.ugc.aweme', '28.5.0', 'VIDEO'],
  ['王者荣耀', 'com.tencent.tmgp.sgame', '3.85.1.6', 'GAME'],
  ['和平精英', 'com.tencent.tmgp.pubgmhd', '3.6.0', 'GAME'],
  ['淘宝', 'com.taobao.taobao', '10.36.0', 'SHOPPING'],
  ['支付宝', 'com.eg.android.AlipayGphone', '10.5.96', 'FINANCE'],
  ['QQ', 'com.tencent.mobileqq', '9.0.15', 'SOCIAL'],
  ['哔哩哔哩', 'tv.danmaku.bili', '7.65.0', 'VIDEO'],
  ['原神', 'com.miHoYo.Yuanshen', '4.7.0', 'GAME'],
  ['美团', 'com.sankuai.meituan', '12.18.4', 'LIFE'],
  ['钉钉', 'com.alibaba.android.rimet', '7.5.5', 'OFFICE'],
  ['网易云音乐', 'com.netease.cloudmusic', '9.1.40', 'MUSIC'],
]

const tenants = ['默认租户', '游戏工作室A', '电商运营B', '测试租户']

// 对齐中台 §1.1 /app/page records[]：id 字符串、appCode、iconUrl、fileSize 为已格式化字符串、createUser、createTime 空格分隔。
const apps = appNames.map((a, i) => ({
  id: String(i + 1),
  appCode: `${a[1].split('.').pop()}-app-${1000 + i}`,
  appName: a[0],
  packageName: a[1],
  version: a[2],
  iconUrl: `https://api.dicebear.com/7.x/shapes/svg?seed=${encodeURIComponent(a[1])}`,
  fileSize: `${50 + i * 37}MB`,
  createUser: tenants[i % tenants.length],
  createTime: `2026-0${(i % 5) + 1}-1${i % 9} 1${i % 9}:2${i % 6}:30`,
}))

export default defineFakeRoute([
  {
    url: '/v1/cloudphone/zones',
    method: 'get',
    response: () => ok(zones),
  },
  {
    url: '/v1/cloudphone/specs',
    method: 'get',
    response: ({ query }) => {
      const kind = (query.kind as string) || 'phone'
      const table = kind === 'vm' ? vmSpecsByZone : phoneSpecsByZone
      // 不传 zoneId = 聚合全部可用区；传了只返回该区。每行附带 zoneId/zoneName。
      const wantZone = query.zoneId ? Number(query.zoneId) : 0
      const rows: any[] = []
      for (const z of zones) {
        if (wantZone && z.id !== wantZone)
          continue
        for (const s of table[z.id] || [])
          rows.push({ ...s, zoneId: z.id, zoneName: z.name })
      }
      return ok(rows)
    },
  },
  {
    url: '/v1/cloudphone/vms',
    method: 'get',
    response: ({ query }) => {
      let list = vms
      if (query.status) list = list.filter(v => v.vmStatus === query.status)
      if (query.vmUid) list = list.filter(v => v.vmUid.includes(query.vmUid as string))
      if (query.vmIp) list = list.filter(v => v.vmIp.includes(query.vmIp as string))
      return ok(list)
    },
  },
  {
    url: '/v1/cloudphone/vm-statuses',
    method: 'get',
    response: () => ok(vmStatuses),
  },
  {
    url: '/v1/cloudphone/images',
    method: 'get',
    response: () => ok(images),
  },
  {
    url: '/v1/cloudphone/boot-plans',
    method: 'get',
    response: () => ok([
      // 单台手机：min/maxCore、min/maxMemory、storage；spec* 是宿主总量（演示用，不展示）
      { id: 1, specId: 1, planName: '标准型', scenarioName: '社交媒体方案', specName: '6600显卡-P31', supplier: 'XIAOXI', resourceUtilization: 'SHARED', width: 720, height: 1280, fps: 30, core: null, minCore: 0.2, maxCore: 0.8, memory: null, minMemory: 1, maxMemory: 4, storage: 20, bandwidth: 200, specCore: 30, specMemory: 112, specStorage: 500, specBandwidth: 200, maxPhone: 20, phoneCount: 8 },
      { id: 2, specId: 1, planName: '性能型', scenarioName: '游戏方案', specName: '6600显卡-P31', supplier: 'XIAOXI', resourceUtilization: 'EXCLUSIVE', width: 1080, height: 1920, fps: 60, core: null, minCore: 1, maxCore: 2, memory: null, minMemory: 4, maxMemory: 8, storage: 32, bandwidth: 200, specCore: 30, specMemory: 112, specStorage: 500, specBandwidth: 200, maxPhone: 10, phoneCount: 3 },
    ]),
  },
  {
    url: '/v1/cloudphone/spec-images',
    method: 'get',
    response: () => ok([
      { imageId: 'img-android13-prod', imageName: 'Android 13 生产版', androidVersion: '13' },
      { imageId: 'img-android12-prod', imageName: 'Android 12 生产版', androidVersion: '12' },
      { imageId: 'img-android11-prod', imageName: 'Android 11 生产版', androidVersion: '11' },
    ]),
  },
  {
    url: '/v1/cloudphone/apps',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const appName = (query.appName as string) || ''
      let list = apps
      if (appName) {
        list = list.filter(
          a => a.appName.includes(appName) || a.packageName.includes(appName),
        )
      }
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size), total })
    },
  },
])
