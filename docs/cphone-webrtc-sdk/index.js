// 引入mouse-multi.js
// 使用动态导入避免ES6模块语法问题
let initApp = null;
let destroyApp = null;
let UIControllerFactory = null;
let DeviceManagerFactory = null;
let MouseControllerFactory = null;

// 动态加载main.js模块
(async function () {
  try {
    const mainModule = await import("./src/main.js");
    initApp = mainModule.initApp;
    destroyApp = mainModule.destroyApp;
    UIControllerFactory = mainModule.UIControllerFactory;
    DeviceManagerFactory = mainModule.DeviceManagerFactory;
    MouseControllerFactory = mainModule.MouseControllerFactory;
    console.log("CphoneWebRTC module loaded successfully");
  } catch (error) {
    console.error("Failed to load CphoneWebRTC module:", error);
  }
})();

// [兼容补丁 2026-06-11] deviceId 归一化:统一接受 1 / "1" / "device1" 三种写法。
// 解决原 XXSDK 不同方法对 deviceId 要求不一致的问题
//   (部分方法用 devices["device"+id] 需数字,部分用 devices[id] 需全键)。
// 本归一化仅【新增】接受的写法,不改变任何原有调用的行为 → 完全向后兼容。
// 建议中台将本归一化合回上游 XXSDK,避免长期分叉。
function _normDevice(deviceId) {
  const s = String(deviceId);
  const num = s.slice(0, 6) === "device" ? Number(s.slice(6)) : Number(s);
  return { num, key: "device" + num };
}

window.CphoneWebRTC = {
  isSync: false, // 是否开启同步
  // 从控默认配置
  config: {},
  // 初始化
  init(data) {
    window.CphoneWebRTC.onInitSuccess = data.onInitSuccess;
    window.CphoneWebRTC.onConnectSuccess = data.onConnectSuccess;
    window.CphoneWebRTC.onConnectFailed = data.onConnectFailed;
    window.CphoneWebRTC.onKickOff = data?.onKickOff;
    window.CphoneWebRTC.config = {
      qualityLevel: 30,
      targetFps: 5
    };
    window.CphoneWebRTC.isSync = false;
    if (initApp) {
      initApp(data);
    } else {
      console.error("CphoneWebRTC module not loaded yet");
    }
  },
  // 发送控制消息
  sendControlMessage(deviceId, params) {
    if (typeof params === "string") {
      params = { type: params };
    }
    const { num, key } = _normDevice(deviceId);
    const deviceItem = window.devices[key];
    const isMain = window.CphoneWebRTCConfig?.roomList?.find(
      item => item.deviceId === num
    )?.isMain;
    if (isMain && window.CphoneWebRTC.isSync) {
      Object.values(window.devices).forEach(item => {
        item.dataChannel?.send(JSON.stringify(params));
      });
    } else {
      deviceItem.dataChannel?.send(
        JSON.stringify({
          ...params
        })
      );
    }
  },
  // 设置主控手机
  setMain(deviceId) {
    this.updateConfig(
      {
        qualityLevel: 60,
        targetFps: 30
      },
      deviceId
    );
    this.updateConfig(window.CphoneWebRTC.config);
  },
  // 销毁
  destroy(deviceId) {
    if (deviceId) {
      const { num, key } = _normDevice(deviceId);
      MouseControllerFactory.destroyController(key);
      DeviceManagerFactory.destroyManager(key);
      UIControllerFactory.destroyDeviceController(key);
      window.devices[key]?.disconnect();
      window.devices[key]?.destroy();
      delete window.devices[key];
      window.getApp()?.deviceControllers?.delete(key);
      if (window.CphoneWebRTCConfig?.roomList) {
        window.CphoneWebRTCConfig.roomList = window.CphoneWebRTCConfig.roomList.filter(
          item => item.deviceId !== num
        );
      }
    } else {
      destroyApp();
      window.CphoneWebRTCConfig = null;
      window.devices = {};
    }
  },
  // 开始同步
  startSync() {
    window.CphoneWebRTC.isSync = true;
  },
  // 停止同步
  stopSync() {
    window.CphoneWebRTC.isSync = false;
  },
  // 连接所有手机
  connectAll() {
    Object.keys(window.devices).forEach(key => {
      window.connectDevice(key);
    });
  },
  addDevice(deviceId) {
    window.addDevice(deviceId);
  },
  // 更新配置
  updateConfig(config, id) {
    const {
      width = null,
      height = null,
      keyFrameInterval = null,
      qualityLevel = null,
      targetFps = null,
      enableVideo = null
    } = config;

    const params = {
      type: "encoder_config_update",
      config: {
        resolution: {
          width,
          height
        },
        targetBitrate: 40000000, // 可以写死
        keyFrameInterval,
        qualityLevel,
        targetFps,
        enableVideo // 是否推视频流
      }
    };
    if (!window.getApp) {
      return;
    }
    if (id) {
      const { num, key } = _normDevice(id);
      const { deviceControllers = [] } = window.getApp();
      deviceControllers.forEach(item => {
        if (item.deviceId === key) {
          try {
            item.deviceManager?.dataChannel?.send(JSON.stringify(params));
            if ((width || height) && item.deviceManager) {
              item.deviceManager.width = width || item.deviceManager.width;
              item.deviceManager.height = height || item.deviceManager.height;
            }
          } catch (error) {
            console.error("[ERROR] updateConfig failed:", error);
          }
        }
      });
      console.log(`✅[${num}] 更新配置:`, params);
    } else {
      const deviceId = window.CphoneWebRTCConfig?.roomList?.find(
        item => item.isMain
      )?.deviceId;
      const { deviceControllers } = window.getApp();
      deviceControllers.forEach(item => {
        if (item.deviceId !== `device${deviceId}`) {
          try {
            item.deviceManager?.dataChannel?.send(JSON.stringify(params));
            if ((width || height) && item.deviceManager) {
              item.deviceManager.width = width || item.deviceManager.width;
              item.deviceManager.height = height || item.deviceManager.height;
            }
          } catch (error) {
            console.error("[ERROR] updateConfig failed:", error);
          }
          // console.log(`✅[${item.deviceId}] 更新配置:`, params);
        }
      });
      window.CphoneWebRTC.config = { ...window.CphoneWebRTC.config, ...config };
    }
  },
  // 得到当前 云手机是否是横屏
  getDeviceIsLandscape(deviceId) {
    const { key } = _normDevice(deviceId);
    const device =
      window.devices && window.devices[key]
        ? window.devices[key].elements.remoteVideo
        : null;
    const videoWidth = device?.videoWidth || 0;
    const videoHeight = device?.videoHeight || 0;
    return videoWidth > videoHeight;
  },
  getVideoResolution(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices && window.devices[key];
    return device.rotation;
  },
  checkVideoSizeChanges(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices && window.devices[key];
    return device.checkVideoSizeChanges();
  },
  // 旋转
  rotateDevice(deviceId) {
    const { key } = _normDevice(deviceId);
    console.log("rotateDevice", key);
    // 优先尝试使用 globalRotateDevice
    if (
      window.globalRotateDevice &&
      typeof window.globalRotateDevice === "function"
    ) {
      try {
        return window.globalRotateDevice(key);
      } catch (error) {
        console.error("[ERROR] globalRotateDevice failed:", error);
      }
    }
    // 尝试直接访问设备并调用旋转
    if (window.devices && window.devices[key]) {
      try {
        const device = window.devices[key];
        const currentRotation = device.rotation || 0;
        const nextRotation = currentRotation === 0 ? -90 : 0;

        device.updateRotation(nextRotation);
        return true;
      } catch (error) {
        console.error("[ERROR] Direct device rotation failed:", error);
      }
    }
    return false;
  },
  //获取当前旋转角度
  getRotation(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    return device.rotation;
  },
  // 获取推流的宽高
  getStreamSize(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    return {
      width: device.lastVideoWidth,
      height: device.lastVideoHeight
    };
  },
  // ==================== 摄像头功能(2026-06-11 从新版 xx-sdk 移植)====================
  // 发送摄像头控制消息(底层:只发 camera_control 信令,不采集本地摄像头)
  sendCameraMessage(deviceId, action) {
    if (!action || (action !== "open" && action !== "close")) {
      console.warn(`[CphoneWebRTC] 无效的摄像头操作: ${action}，应为 "open" 或 "close"`);
      return;
    }
    const cameraMessage = { type: "camera_control", action: action };
    const { num, key } = _normDevice(deviceId);
    const deviceItem = window.devices[key];
    const isMain = window.CphoneWebRTCConfig?.roomList?.find(
      item => item.deviceId === num
    )?.isMain;
    if (isMain && window.CphoneWebRTC.isSync) {
      Object.values(window.devices).forEach(item => {
        item.dataChannel?.send(JSON.stringify(cameraMessage));
      });
    } else {
      if (!deviceItem || !deviceItem.dataChannel) {
        console.warn(`[CphoneWebRTC] 设备 ${deviceId} 的 dataChannel 未连接`);
        return;
      }
      if (deviceItem.dataChannel.readyState !== "open") {
        console.warn(`[CphoneWebRTC] 设备 ${deviceId} 的 dataChannel 未打开`);
        return;
      }
      deviceItem.dataChannel.send(JSON.stringify(cameraMessage));
      console.log(`[CphoneWebRTC] 已发送摄像头控制消息: ${action} (设备: ${deviceId})`);
    }
  },
  // 获取本地摄像头设备列表(enumerateDevices)
  async getCameraList(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    if (!device) throw new Error(`设备 ${deviceId} 不存在`);
    return await device.getCameraList();
  },
  // 打开摄像头(采集本地摄像头 → replaceTrack 注入云手机 + camera_control 信令 + 麦克风PCM)
  async openCamera(deviceId, cameraDeviceId = null) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    if (!device) throw new Error(`设备 ${deviceId} 不存在`);
    return await device.openCamera(cameraDeviceId);
  },
  // 关闭摄像头(replaceTrack 切回黑屏占位轨 + 停麦克风)
  async closeCamera(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    if (!device) throw new Error(`设备 ${deviceId} 不存在`);
    return await device.closeCamera();
  },
  // 切换到指定摄像头
  async switchToCamera(deviceId, cameraDeviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    if (!device) throw new Error(`设备 ${deviceId} 不存在`);
    if (!cameraDeviceId) throw new Error("请提供摄像头设备ID");
    return await device.switchToCamera(cameraDeviceId);
  },
  // 获取摄像头状态
  getCameraStatus(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    if (!device) {
      return { enabled: false, selectedDeviceId: null, error: `设备 ${deviceId} 不存在` };
    }
    return {
      enabled: device.cameraEnabled || false,
      selectedDeviceId: device.selectedCameraDeviceId || null,
      mirrorEnabled:
        device.cameraMirrorEnabled !== undefined ? device.cameraMirrorEnabled : true
    };
  },
  // 切换摄像头镜像翻转(自拍镜像)
  async toggleCameraMirror(deviceId, enabled = null) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    if (!device) throw new Error(`设备 ${deviceId} 不存在`);
    return await device.toggleCameraMirror(enabled);
  },
  // 获取摄像头镜像状态
  getCameraMirrorStatus(deviceId) {
    const { key } = _normDevice(deviceId);
    const device = window.devices[key];
    if (!device) return true;
    return device.getCameraMirrorStatus
      ? device.getCameraMirrorStatus()
      : device.cameraMirrorEnabled !== undefined
        ? device.cameraMirrorEnabled
        : true;
  }
};
