/**
 * WebScreen前端主入口文件
 * 新架构整合所有拆分的模块
 */

// 核心模块
import {
  ENCODER_CONFIG,
  ROTATION_CONFIG,
  SERVER_CONFIG,
  VIDEO_CONFIG,
  WEBRTC_CONFIG
} from "./core/config.js";

// 设备管理模块
import {
  DeviceManager,
  DeviceManagerFactory
} from "./device/device-manager.js";

// UI控制模块
import {
  DeviceUIController,
  GlobalUIController,
  UIControllerFactory
} from "./ui/ui-controller.js";

// 通知管理模块
import {
  NotificationManager,
  showError,
  showInfo,
  showSuccess,
  showWarning
} from "./ui/notification-manager.js";

// 页面生成模块
import { generateMultiWindowPage } from "./ui/page-generator.js";

// 工具模块
import { storage } from "./utils/storage.js";

// 数学模块
import { AffineMatrix } from "./math/matrix.js";

// 输入处理模块
import { CoordinateTransformer } from "./input/coordinate-transform.js";
import { MouseControllerFactory } from "./input/mouse-controller.js";

/**
 * 主应用程序类
 * 负责初始化和协调所有模块
 */
class WebScreenApp {
  constructor() {
    this.initialized = false;
    this.globalUIController = null;
    this.deviceControllers = new Map();
    this.notificationManager = new NotificationManager();

    // 应用配置
    this.config = {
      defaultWindowCount: 1,
      maxWindowCount: SERVER_CONFIG.MAX_WINDOWS,
      theme: "light",
      ...this.loadAppConfig()
    };

    console.log("🚀 WebScreen App initializing...");
  }

  /**
   * 初始化应用
   */
  async init() {
    if (this.initialized) {
      console.warn("⚠️ App already initialized");
      return;
    }
    try {
      // 1. 初始化主题
      this.initTheme();

      // 2. 生成页面
      //   await this.generatePage();

      // 3. 初始化全局UI控制器
      this.initGlobalController();

      // 4. 初始化设备控制器
      await this.initDeviceControllers();

      // 5. 设置全局函数（供其他模块使用）
      this.setupGlobalFunctions();

      // 6. 绑定全局事件
      this.bindGlobalEvents();

      // 6. 显示启动完成通知
      this.showInitialNotification();

      this.initialized = true;
      console.log("✅ WebScreen App initialized successfully");

      // 回调初始化完成
      if (window.CphoneWebRTC.onInitSuccess) {
        window.CphoneWebRTC.onInitSuccess();
      }

      // 发布模块系统就绪事件，让兼容性函数能够使用新的接口
      this.publishModuleSystemReady();
    } catch (error) {
      console.error("❌ App initialization failed:", error);
      this.notificationManager.showError("应用初始化失败: " + error.message);
    }
  }

  /**
   * 初始化主题
   */
  initTheme() {
    const savedTheme = localStorage.getItem("theme") || this.config.theme;
    document.documentElement.setAttribute("data-theme", savedTheme);
    console.log(`🎨 Theme initialized: ${savedTheme}`);
  }

  /**
   * 生成页面
   */
  async generatePage() {
    const windowCount = this.getWindowCount();

    // 生成多窗口页面
    const pageContent = generateMultiWindowPage(windowCount);

    // 插入页面内容
    const container = document.getElementById("app-container") || document.body;
    container.innerHTML = pageContent;

    console.log(`📄 Generated page with ${windowCount} windows`);
  }

  /**
   * 初始化全局UI控制器
   */
  initGlobalController() {
    this.globalUIController = UIControllerFactory.getGlobalController();
    console.log("🌐 Global UI controller initialized");
  }

  /**
   * 初始化设备控制器
   */
  async initDeviceControllers() {
    const windowCount = this.getWindowCount();
    const initPromises = [];

    for (let i = 1; i <= windowCount; i++) {
      const deviceId = `device${i}`;
      const promise = this.initDeviceController(deviceId);
      initPromises.push(promise);
    }

    await Promise.all(initPromises);
    console.log(`📱 Initialized ${windowCount} device controllers`);
  }

  /**
   * 初始化单个设备控制器
   */
  async initDeviceController(deviceId) {
    try {
      // 创建UI控制器（会自动创建对应的设备管理器）
      const uiController = UIControllerFactory.createDeviceController(deviceId);
      this.deviceControllers.set(deviceId, uiController);

      console.log(`📱 [${deviceId}] Controller initialized`);
      return uiController;
    } catch (error) {
      console.error(
        `❌ [${deviceId}] Controller initialization failed:`,
        error
      );
      throw error;
    }
  }

  /**
   * 设置全局函数
   */
  setupGlobalFunctions() {
    // 设置全局变量供鼠标控制器使用
    window.devices = {};

    // 收集所有设备管理器到全局变量中
    this.deviceControllers.forEach((uiController, deviceId) => {
      window.devices[deviceId] = uiController.deviceManager;
    });

    // 全局获取设备旋转角度的函数（供鼠标控制器使用）
    window.getDeviceRotation = function (deviceId) {
      const device = window.devices[deviceId];
      return device ? device.rotation : 0;
    };

    // 设备连接控制函数（供HTML按钮使用）
    window.connectDevice = async (deviceId, onconnectStatus) => {
      if (onconnectStatus) {
        window.onconnectStatus = onconnectStatus;
      }
      const device = window.devices[deviceId];
      if (device) {
        console.log(`🔗 [${deviceId}] Connecting device...`);
        await device.connect();
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.disconnectDevice = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        console.log(`❌ [${deviceId}] Disconnecting device...`);
        device.disconnect();
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    // 设备控制函数（供HTML按钮使用）
    window.toggleMute = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        console.log(`🔊 [${deviceId}] Toggling mute...`);
        device.toggleMute();
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.changeRoom = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        const roomSelect = document.getElementById(
          `roomSelect${deviceId.replace("device", "")}`
        );
        if (roomSelect) {
          console.log(`🏠 [${deviceId}] Changing room to: ${roomSelect.value}`);
          device.updateRoom(roomSelect.value);
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.changeResolution = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        const resolutionSelect = document.getElementById(
          `resolutionSelect${deviceId.replace("device", "")}`
        );
        if (resolutionSelect) {
          const [width, height] = resolutionSelect.value.split("x").map(Number);
          if (width && height) {
            console.log(
              `📺 [${deviceId}] Changing resolution to: ${width}x${height}`
            );
            device.updateResolution(width, height);
          }
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.changeBitrate = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        const bitrateSelect = document.getElementById(
          `bitrateSelect${deviceId.replace("device", "")}`
        );
        if (bitrateSelect) {
          const bitrate = parseInt(bitrateSelect.value);
          console.log(`📡 [${deviceId}] Changing bitrate to: ${bitrate}K`);
          device.updateBitrate(bitrate);
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.changeKeyFrame = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        const keyFrameSelect = document.getElementById(
          `keyFrameSelect${deviceId.replace("device", "")}`
        );
        if (keyFrameSelect) {
          const interval = parseInt(keyFrameSelect.value);
          console.log(
            `🎞️ [${deviceId}] Changing keyframe interval to: ${interval}s`
          );
          device.updateKeyFrameInterval(interval);
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.changeQuality = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        const qualitySelect = document.getElementById(
          `qualitySelect${deviceId.replace("device", "")}`
        );
        if (qualitySelect) {
          const quality = parseInt(qualitySelect.value);
          console.log(`🎨 [${deviceId}] Changing quality to: ${quality}%`);
          device.updateQuality(quality);
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.changeEncoderFps = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        const encoderFpsSelect = document.getElementById(
          `encoderFpsSelect${deviceId.replace("device", "")}`
        );
        if (encoderFpsSelect) {
          const fps = parseInt(encoderFpsSelect.value);
          console.log(`🎬 [${deviceId}] Changing encoder FPS to: ${fps}`);
          device.updateEncoderFps(fps);
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    window.changeRotation = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        const rotationSelect = document.getElementById(
          `rotationSelect${deviceId.replace("device", "")}`
        );
        if (rotationSelect) {
          const rotation = parseInt(rotationSelect.value);
          console.log(`🔄 [${deviceId}] Changing rotation to: ${rotation}°`);
          device.updateRotation(rotation);
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    // 虚拟按键函数（供HTML按钮使用）
    window.backButtonHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "back");
    };

    window.homeButtonHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "home");
    };

    window.recentButtonHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "recent");
    };

    window.powerButtonHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "power");
    };

    window.lockButtonHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "lock");
    };

    window.volumeUpHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "volume_up");
    };

    window.volumeDownHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "volume_down");
    };

    window.getClipboardHandler = deviceId => {
      this.sendDeviceControlMessage(deviceId, "get_clipboard");
    };

    window.setClipboardHandler = deviceId => {
      const clipboardText = prompt("请输入要设置到手机剪贴板的内容:", "");
      if (clipboardText !== null && clipboardText.trim() !== "") {
        this.sendDeviceControlMessage(
          deviceId,
          "set_clipboard",
          clipboardText.trim()
        );
      }
    };

    window.setClipboardHandler = deviceId => {
      const device = window.devices[deviceId];
      if (device && device.dataChannel) {
        const text = prompt("请输入要设置的剪贴板内容:");
        if (text) {
          console.log(`📝 [${deviceId}] Setting clipboard...`);
          device.dataChannel.send(
            JSON.stringify({ type: "clipboard", action: "set", content: text })
          );
        }
      }
    };

    // 全局控制函数（供HTML按钮使用）
    window.connectAllDevices = async () => {
      console.log("🔗 Connecting all devices...");
      if (this.globalUIController) {
        await this.globalUIController.connectAllDevices();
      } else {
        // 回退方案：直接调用设备管理器
        const promises = [];
        for (const device of Object.values(window.devices)) {
          promises.push(device.connect());
        }
        await Promise.all(promises);
      }
    };

    window.disconnectAllDevices = () => {
      console.log("❌ Disconnecting all devices...");
      if (this.globalUIController) {
        this.globalUIController.disconnectAllDevices();
      } else {
        // 回退方案：直接调用设备管理器
        for (const device of Object.values(window.devices)) {
          device.disconnect();
        }
      }
    };

    window.toggleAllMute = () => {
      console.log("🔊 Toggling all mute...");
      if (this.globalUIController) {
        this.globalUIController.toggleAllMute();
      } else {
        // 回退方案：直接调用设备管理器
        for (const device of Object.values(window.devices)) {
          device.toggleMute();
        }
      }
    };

    window.toggleAllVideoStats = () => {
      console.log("🔧 Toggling all video stats...");
      if (this.globalUIController) {
        this.globalUIController.toggleAllVideoStats();
      } else {
        // 回退方案：直接调用设备管理器
        for (const device of Object.values(window.devices)) {
          device.toggleVideoStats();
        }
      }
    };

    // 视频缩放控制函数（供HTML和调试使用）
    window.changeVideoScale = (deviceId, scale) => {
      // 验证参数
      if (typeof scale !== "number" || isNaN(scale)) {
        console.error(`❌ [${deviceId}] Invalid scale value:`, scale);
        return;
      }

      const device = window.devices[deviceId];
      if (device) {
        console.log(
          `📏 [${deviceId}] Changing video scale to: ${Math.round(
            scale * 100
          )}%`
        );

        // 通过设备控制器处理缩放
        const controller = this.deviceControllers.get(deviceId);
        if (controller && controller.handleScaleChange) {
          controller.handleScaleChange(scale);
        } else {
          // 直接设置样式作为回退
          const videoElement = device.elements.remoteVideo;
          if (videoElement) {
            videoElement.style.transform = `scale(${scale})`;
            videoElement.style.transformOrigin = "top left";
          }
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    // 旋转设备函数（横竖屏切换）
    window.globalRotateDevice = deviceId => {
      const device = window.devices[deviceId];
      if (device) {
        // 检查当前视频分辨率是否为横屏
        const videoElement = device.elements?.remoteVideo;
        const isVideoLandscape =
          videoElement &&
          videoElement.videoWidth > 0 &&
          videoElement.videoHeight > 0 &&
          videoElement.videoWidth > videoElement.videoHeight;

        // 当前旋转状态
        const currentRotation = device.rotation || 0;
        let nextRotation;

        if (isVideoLandscape) {
          // 如果当前是横屏分辨率，则直接发送0度命令到后端，绕过前端状态同步
          console.log(
            `🔄 [${deviceId}] 检测到横屏分辨率，直接发送取消旋转命令到后端`
          );
          // 直接调用sendRotationEvent而不是updateRotation，避免自动同步状态
          device.sendRotationEvent(0);
        } else {
          // 如果是竖屏分辨率，则在0°和90°之间切换，使用正常流程
          nextRotation = currentRotation === 0 ? -90 : 0;
          const rotationName = nextRotation === 0 ? "竖屏" : "横屏";
          console.log(
            `🔄 [${deviceId}] 前端旋转: ${currentRotation}° → ${nextRotation}° (${rotationName})`
          );
          // 更新前端显示旋转和发送后端事件
          device.updateRotation(nextRotation);
        }
      } else {
        console.error(`❌ [${deviceId}] Device not found`);
      }
    };

    // 应用初始化完成的标志
    window.WebScreenApp = this;

    // 初始化所有设备的鼠标控制器
    this.deviceControllers.forEach((uiController, deviceId) => {
      // 为每个设备创建鼠标控制器
      try {
        MouseControllerFactory.createController(deviceId);
        console.log(`🖱️ [${deviceId}] Mouse controller initialized`);
      } catch (error) {
        console.error(
          `❌ [${deviceId}] Failed to initialize mouse controller:`,
          error
        );
      }
    });

    console.log("🌐 Global functions set up");
    console.log("📱 Available devices:", Object.keys(window.devices));
  }

  /**
   * 绑定全局事件
   */
  bindGlobalEvents() {
    // 页面卸载时清理资源
    window.addEventListener("beforeunload", () => {
      this.destroy();
    });

    // 窗口大小变化时调整布局
    window.addEventListener("resize", () => {
      this.handleWindowResize();
    });

    // 键盘快捷键
    document.addEventListener("keydown", e => {
      this.handleGlobalKeydown(e);
    });

    // 可见性变化时暂停/恢复连接
    document.addEventListener("visibilitychange", () => {
      this.handleVisibilityChange();
    });

    console.log("⌨️ Global events bound");
  }

  /**
   * 显示初始通知
   */
  showInitialNotification() {
    const windowCount = this.getWindowCount();
    this.notificationManager.showSuccess(
      `WebScreen已启动，当前窗口数: ${windowCount}`,
      "系统就绪",
      { duration: 3000 }
    );
  }

  /**
   * 获取窗口数量
   */
  getWindowCount() {
    const windowCount = window.CphoneWebRTCConfig?.roomList?.length || 1;
    // const urlParams = new URLSearchParams(window.location.search);
    // const paramCount = parseInt(urlParams.get('windows'));
    // const savedCount = parseInt(localStorage.getItem('windowCount'));

    let count = windowCount || this.config.defaultWindowCount;

    // 限制窗口数量
    count = Math.max(1, Math.min(count, this.config.maxWindowCount));

    return count;
  }

  /**
   * 加载应用配置
   */
  loadAppConfig() {
    try {
      const saved = localStorage.getItem("appConfig");
      return saved ? JSON.parse(saved) : {};
    } catch (error) {
      console.warn("⚠️ Failed to load app config:", error);
      return {};
    }
  }

  /**
   * 保存应用配置
   */
  saveAppConfig() {
    try {
      localStorage.setItem("appConfig", JSON.stringify(this.config));
    } catch (error) {
      console.error("❌ Failed to save app config:", error);
    }
  }

  /**
   * 处理窗口大小变化
   */
  handleWindowResize() {
    // 通知所有设备控制器调整布局
    this.deviceControllers.forEach((controller, deviceId) => {
      // 如果设备管理器有resizeVideoElement方法，调用它
      if (
        controller.deviceManager &&
        typeof controller.deviceManager.resizeVideoElement === "function"
      ) {
        controller.deviceManager.resizeVideoElement();
      }
    });
  }

  /**
   * 处理全局键盘快捷键
   */
  handleGlobalKeydown(event) {
    if (event.ctrlKey && event.altKey) {
      switch (event.key.toLowerCase()) {
        case "r":
          event.preventDefault();
          this.handleRefresh();
          break;
        case "f":
          event.preventDefault();
          this.handleFullscreen();
          break;
        case "t":
          event.preventDefault();
          this.handleThemeToggle();
          break;
        case "c":
          event.preventDefault();
          this.handleClearNotifications();
          break;
      }
    }
  }

  /**
   * 处理可见性变化
   */
  handleVisibilityChange() {
    if (document.hidden) {
      console.log("📱 Page hidden, maintaining connections");
      // 页面隐藏时保持连接但可以降低统计频率
      this.deviceControllers.forEach((controller, deviceId) => {
        if (controller.deviceManager && controller.deviceManager.statsEnabled) {
          // 可以在这里降低统计频率
        }
      });
    } else {
      console.log("📱 Page visible, resuming normal operation");
      // 页面可见时恢复正常操作
      this.deviceControllers.forEach((controller, deviceId) => {
        if (controller.deviceManager && controller.deviceManager.statsEnabled) {
          // 恢复正常统计频率
        }
      });
    }
  }

  /**
   * 处理刷新
   */
  handleRefresh() {
    if (confirm("确定要刷新页面吗？这将断开所有连接。")) {
      window.location.reload();
    }
  }

  /**
   * 处理全屏切换
   */
  handleFullscreen() {
    if (document.fullscreenElement) {
      document.exitFullscreen();
    } else {
      document.documentElement.requestFullscreen();
    }
  }

  /**
   * 处理主题切换
   */
  handleThemeToggle() {
    const currentTheme = document.documentElement.getAttribute("data-theme");
    const newTheme = currentTheme === "dark" ? "light" : "dark";

    document.documentElement.setAttribute("data-theme", newTheme);
    localStorage.setItem("theme", newTheme);

    this.config.theme = newTheme;
    this.saveAppConfig();

    // 移除主题切换通知
  }

  /**
   * 处理清除通知
   */
  handleClearNotifications() {
    this.notificationManager.clear();
  }

  /**
   * 动态添加设备
   */
  async addDevice() {
    const currentCount = this.deviceControllers.size;
    if (currentCount >= this.config.maxWindowCount) {
      this.notificationManager.showWarning(
        `最多支持${this.config.maxWindowCount}个设备`
      );
      return;
    }

    const newDeviceId = `device${currentCount + 1}`;

    try {
      // 生成新的设备窗口HTML
      // const deviceHTML = generateDeviceWindow(currentCount + 1);

      // // 插入到页面中
      // const container =
      //   document.getElementById('device-container') || document.body;
      // container.insertAdjacentHTML('beforeend', deviceHTML);

      // 初始化新设备控制器
      await this.initDeviceController(newDeviceId);

      // 移除设备添加成功通知
    } catch (error) {
      console.error(`❌ Failed to add device ${newDeviceId}:`, error);
      this.notificationManager.showError(`添加设备失败: ${error.message}`);
    }
  }

  /**
   * 动态移除设备
   */
  removeDevice(deviceId) {
    const controller = this.deviceControllers.get(deviceId);
    if (!controller) {
      this.notificationManager.showWarning(`设备${deviceId}不存在`);
      return;
    }

    try {
      // 销毁控制器
      UIControllerFactory.destroyDeviceController(deviceId);
      this.deviceControllers.delete(deviceId);

      // 移除DOM元素
      const deviceElement = document.querySelector(
        `[data-device="${deviceId}"]`
      );
      if (deviceElement) {
        deviceElement.remove();
      }

      // 移除设备移除成功通知
    } catch (error) {
      console.error(`❌ Failed to remove device ${deviceId}:`, error);
      this.notificationManager.showError(`移除设备失败: ${error.message}`);
    }
  }

  /**
   * 连接所有设备
   */
  async connectAllDevices() {
    const promises = Array.from(this.deviceControllers.values()).map(
      controller => {
        return controller.deviceManager.connect().catch(error => {
          console.error(`❌ [${controller.deviceId}] Connect failed:`, error);
          return { deviceId: controller.deviceId, error };
        });
      }
    );

    const results = await Promise.allSettled(promises);

    const successCount = results.filter(r => r.status === "fulfilled").length;
    const failedCount = results.length - successCount;

    if (failedCount === 0) {
      // 移除全局连接成功通知
    } else {
      this.notificationManager.showWarning(
        `部分设备连接失败 (${successCount}/${results.length})`
      );
    }
  }

  /**
   * 断开所有设备
   */
  disconnectAllDevices() {
    this.deviceControllers.forEach(controller => {
      controller.deviceManager.disconnect();
    });

    // 移除断开所有设备通知
  }

  /**
   * 获取应用状态
   */
  getAppStatus() {
    const deviceStatuses = Array.from(this.deviceControllers.values()).map(
      controller => controller.getDeviceStatus()
    );

    return {
      initialized: this.initialized,
      config: this.config,
      connectedDevices: deviceStatuses.filter(s => s.isConnected).length,
      totalDevices: deviceStatuses.length,
      devices: deviceStatuses
    };
  }

  /**
   * 发布模块系统就绪事件
   */
  publishModuleSystemReady() {
    // 设置模块系统就绪标志
    window.moduleSystemReady = true;

    // 暴露应用实例到全局作用域，供兼容性函数使用
    window.getApp = () => this;

    // 发布自定义事件
    const event = new CustomEvent("moduleSystemReady", {
      detail: { app: this }
    });
    window.dispatchEvent(event);

    console.log("📡 Module system ready event published");
    console.log(
      "✅ Placeholder functions will now redirect to module implementations"
    );
  }

  /**
   * 发送设备控制消息（统一处理虚拟按键）
   * @param {string} deviceId 设备ID
   * @param {string} action 操作类型
   * @param {string} data 额外数据（可选）
   */
  sendDeviceControlMessage(deviceId, action, data = null) {
    const device = window.devices[deviceId];
    if (!device) {
      console.warn(`⚠️ [${deviceId}] Device not found`);
      this.notificationManager.showWarning(`[${deviceId}] 设备未找到`);
      return;
    }

    if (!device.dataChannel || device.dataChannel.readyState !== "open") {
      console.warn(`⚠️ [${deviceId}] DataChannel not ready`);
      this.notificationManager.showWarning(`[${deviceId}] 数据通道未连接`);
      return;
    }

    let controlMessage;

    // 根据操作类型构造不同的消息格式
    switch (action) {
      case "get_clipboard":
        controlMessage = { type: "clipboard", action: "get" };
        console.log(`📋 [${deviceId}] Getting clipboard...`);
        break;

      case "set_clipboard":
        controlMessage = { type: "clipboard", action: "set", text: data };
        console.log(`📝 [${deviceId}] Setting clipboard:`, data);
        break;

      case "back":
        controlMessage = { type: "button_back" };
        console.log(`⬅️ [${deviceId}] Back button pressed`);
        break;

      case "home":
        controlMessage = { type: "button_home" };
        console.log(`🏠 [${deviceId}] Home button pressed`);
        break;

      case "recent":
        controlMessage = { type: "button_recent" };
        console.log(`📋 [${deviceId}] Recent button pressed`);
        break;

      case "power":
        controlMessage = { type: "button_power" };
        console.log(`⚡ [${deviceId}] Power button pressed`);
        break;

      case "lock":
        controlMessage = { type: "button_lock" };
        console.log(`🔒 [${deviceId}] Lock button pressed`);
        break;

      case "volume_up":
        controlMessage = { type: "button_volume_up" };
        console.log(`🔊 [${deviceId}] Volume up pressed`);
        break;

      case "volume_down":
        controlMessage = { type: "button_volume_down" };
        console.log(`🔉 [${deviceId}] Volume down pressed`);
        break;

      default:
        console.warn(`⚠️ [${deviceId}] Unknown action: ${action}`);
        return;
    }

    try {
      device.dataChannel.send(JSON.stringify(controlMessage));
      console.log(`🎮 [${deviceId}] Control message sent:`, controlMessage);

      // 移除操作反馈通知
    } catch (error) {
      console.error(`❌ [${deviceId}] Failed to send control message:`, error);
      this.notificationManager.showError(
        `[${deviceId}] 控制消息发送失败: ${error.message}`
      );
    }
  }

  /**
   * 销毁应用
   */
  destroy() {
    console.log("🗑️ Destroying WebScreen App...");

    // 销毁所有控制器
    UIControllerFactory.destroyAll();

    // 销毁设备管理器
    DeviceManagerFactory.destroyAll();

    // 销毁鼠标控制器
    MouseControllerFactory.destroyAll();

    // 清理通知
    this.notificationManager.destroy();

    // 清理引用
    this.globalUIController = null;
    this.deviceControllers.clear();

    // 清理全局变量
    if (window.getApp) {
      delete window.getApp;
    }

    this.initialized = false;
    console.log("✅ WebScreen App destroyed");
  }
}

/**
 * 全局应用实例
 */
let appInstance = null;

/**
 * 获取应用实例
 */
export function getApp() {
  if (!appInstance) {
    appInstance = new WebScreenApp();
  }
  return appInstance;
}

/**
 * 初始化应用
 */
export async function initApp(config = {}) {
  const app = getApp();

  // 如果有配置参数，设置到全局变量中供其他模块使用
  if (config) {
    window.CphoneWebRTCConfig = config;
    console.log("CphoneWebRTC config set:", config);
  }

  await app.init();
  return app;
}

/**
 * 销毁应用
 */
export function destroyApp() {
  if (appInstance) {
    appInstance.destroy();
    appInstance = null;
  }
}

/**
 * 导出常用功能
 */
export {
  AffineMatrix,
  CoordinateTransformer,
  // 设备管理
  DeviceManager,
  DeviceManagerFactory,
  // UI控制
  DeviceUIController,
  ENCODER_CONFIG,
  GlobalUIController,
  MouseControllerFactory,
  // 通知系统
  NotificationManager,
  ROTATION_CONFIG,
  showError,
  showInfo,
  showSuccess,
  showWarning,
  // 工具函数
  storage,
  UIControllerFactory,
  VIDEO_CONFIG,
  // 配置
  WEBRTC_CONFIG,
  // 核心类
  WebScreenApp
};

/**
 * 页面加载完成后自动初始化
 * 注意：这个初始化由server.js生成的HTML页面中的模块系统控制
 */
// 已移除自动初始化，改为由HTML页面中的模块系统控制

// 开发调试工具
if (typeof window !== "undefined") {
  window.WebScreenApp = {
    getApp,
    initApp,
    destroyApp,
    DeviceManagerFactory,
    UIControllerFactory,
    storage,
    showSuccess,
    showError,
    showWarning,
    showInfo
  };
}

console.log("🚀 WebScreen main module loaded");
