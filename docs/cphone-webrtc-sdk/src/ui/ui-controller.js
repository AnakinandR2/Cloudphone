/**
 * UI控制模块
 * 负责处理用户界面事件、按钮点击、设置变更等
 */

import { DeviceManagerFactory } from "../device/device-manager.js";
import { NotificationManager } from "../ui/notification-manager.js";

/**
 * 设备UI控制器
 * 处理单个设备的UI交互和事件
 */
export class DeviceUIController {
  constructor(deviceId) {
    this.deviceId = deviceId;
    this.deviceManager = DeviceManagerFactory.getManager(deviceId);
    this.notificationManager = new NotificationManager();

    // UI元素引用
    this.elements = {};

    // 初始化
    this.bindElements();
    this.bindEvents();
    this.setupDeviceCallbacks();

    console.log(`🎮 [${this.deviceId}] DeviceUIController initialized`);
  }

  /**
   * 绑定DOM元素
   */
  bindElements() {
    // const num = this.deviceId.slice(-1);
    const num = this.deviceId.replace("device", "");

    this.elements = {
      // 控制按钮
      connectBtn: document.getElementById(`connectBtn${num}`),
      disconnectBtn: document.getElementById(`disconnectBtn${num}`),
      muteBtn: document.getElementById(`muteBtn${num}`),
      statsBtn: document.getElementById(`statsBtn${num}`),

      // 设置选择器
      roomSelect: document.getElementById(`roomSelect${num}`),
      resolutionSelect: document.getElementById(`resolutionSelect${num}`),
      bitrateSelect: document.getElementById(`bitrateSelect${num}`),
      keyFrameSelect: document.getElementById(`keyFrameSelect${num}`),
      qualitySelect: document.getElementById(`qualitySelect${num}`),
      encoderFpsSelect: document.getElementById(`encoderFpsSelect${num}`),
      rotationSelect: document.getElementById(`rotationSelect${num}`),
      scaleSelect: document.getElementById(`scaleSelect${num}`),

      // 状态显示
      statusIcon: document.getElementById(`statusIcon${num}`),
      statusText: document.getElementById(`statusText${num}`),
      dataChannelIcon: document.getElementById(`dataChannelIcon${num}`),
      dataChannelStatus: document.getElementById(`dataChannelStatus${num}`),

      // 剪贴板功能
      clipboardGetBtn: document.getElementById(`clipboardGetBtn${num}`),
      clipboardSetBtn: document.getElementById(`clipboardSetBtn${num}`),
      clipboardInput: document.getElementById(`clipboardInput${num}`),

      // 设备控制
      homeBtn: document.getElementById(`homeBtn${num}`),
      backBtn: document.getElementById(`backBtn${num}`),
      recentBtn: document.getElementById(`recentBtn${num}`),
      powerBtn: document.getElementById(`powerBtn${num}`),
      lockBtn: document.getElementById(`lockBtn${num}`),

      // 视频容器和视频元素
      videoContainer: document.getElementById(`videoContainer${num}`),
      remoteVideo: document.getElementById(`remoteVideo${num}`),

      // 配置面板
      configPanel: document.getElementById(`configPanel${num}`),
      toggleConfigBtn: document.getElementById(`toggleConfigBtn${num}`)
    };
  }

  /**
   * 绑定事件监听器
   */
  bindEvents() {
    const { elements } = this;

    // 连接控制事件
    if (elements.connectBtn) {
      elements.connectBtn.addEventListener("click", () => this.handleConnect());
    }

    if (elements.disconnectBtn) {
      elements.disconnectBtn.addEventListener("click", () =>
        this.handleDisconnect()
      );
    }

    if (elements.muteBtn) {
      elements.muteBtn.addEventListener("click", () => this.handleMuteToggle());
    }

    if (elements.statsBtn) {
      elements.statsBtn.addEventListener("click", () =>
        this.handleStatsToggle()
      );
    }

    // 设置变更事件
    if (elements.roomSelect) {
      elements.roomSelect.addEventListener("change", e =>
        this.handleRoomChange(e.target.value)
      );
    }

    if (elements.resolutionSelect) {
      elements.resolutionSelect.addEventListener("change", e =>
        this.handleResolutionChange(e.target.value)
      );
    }

    if (elements.bitrateSelect) {
      elements.bitrateSelect.addEventListener("change", e =>
        this.handleBitrateChange(parseInt(e.target.value))
      );
    }

    if (elements.keyFrameSelect) {
      elements.keyFrameSelect.addEventListener("change", e =>
        this.handleKeyFrameChange(parseInt(e.target.value))
      );
    }

    if (elements.qualitySelect) {
      elements.qualitySelect.addEventListener("change", e =>
        this.handleQualityChange(parseInt(e.target.value))
      );
    }

    if (elements.encoderFpsSelect) {
      elements.encoderFpsSelect.addEventListener("change", e =>
        this.handleEncoderFpsChange(parseInt(e.target.value))
      );
    }

    if (elements.rotationSelect) {
      elements.rotationSelect.addEventListener("change", e =>
        this.handleRotationChange(parseInt(e.target.value))
      );
    }

    if (elements.scaleSelect) {
      elements.scaleSelect.addEventListener("change", e =>
        this.handleScaleChange(parseFloat(e.target.value))
      );
    }

    // 剪贴板功能事件
    if (elements.clipboardGetBtn) {
      elements.clipboardGetBtn.addEventListener("click", () =>
        this.handleClipboardGet()
      );
    }

    if (elements.clipboardSetBtn) {
      elements.clipboardSetBtn.addEventListener("click", () =>
        this.handleClipboardSet()
      );
    }

    // 设备控制事件
    if (elements.homeBtn) {
      elements.homeBtn.addEventListener("click", () =>
        this.handleDeviceControl("home")
      );
    }

    if (elements.backBtn) {
      elements.backBtn.addEventListener("click", () =>
        this.handleDeviceControl("back")
      );
    }

    if (elements.recentBtn) {
      elements.recentBtn.addEventListener("click", () =>
        this.handleDeviceControl("recent")
      );
    }

    if (elements.powerBtn) {
      elements.powerBtn.addEventListener("click", () =>
        this.handleDeviceControl("power")
      );
    }

    if (elements.lockBtn) {
      elements.lockBtn.addEventListener("click", () =>
        this.handleDeviceControl("lock")
      );
    }

    // 配置面板切换
    if (elements.toggleConfigBtn) {
      elements.toggleConfigBtn.addEventListener("click", () =>
        this.handleConfigToggle()
      );
    }

    // 键盘快捷键
    document.addEventListener("keydown", e => this.handleKeyDown(e));
  }

  /**
   * 设置设备管理器回调
   */
  setupDeviceCallbacks() {
    this.deviceManager.on("onConnectionStateChange", data => {
      this.updateConnectionStatus(data.connected, data.status);
    });

    this.deviceManager.on("onVideoReady", data => {
      this.handleVideoReady(data);
    });

    this.deviceManager.on("onStatsUpdate", data => {
      this.handleStatsUpdate(data);
    });

    this.deviceManager.on("onError", data => {
      this.handleError(data);
    });
  }

  // === 事件处理方法 ===

  /**
   * 处理连接按钮点击
   */
  async handleConnect() {
    try {
      this.updateConnectionStatus(null, "连接中...");
      await this.deviceManager.connect();
      this.notificationManager.showSuccess(`[${this.deviceId}] 开始连接`);
    } catch (error) {
      this.notificationManager.showError(
        `[${this.deviceId}] 连接失败: ${error.message}`
      );
      this.updateConnectionStatus(false, "连接失败");
    }
  }

  /**
   * 处理断开连接按钮点击
   */
  handleDisconnect() {
    this.deviceManager.disconnect();
    this.notificationManager.showInfo(`[${this.deviceId}] 已断开连接`);
  }

  /**
   * 处理静音切换
   */
  handleMuteToggle() {
    const isMuted = this.deviceManager.toggleMute();
    // 移除静音切换通知
  }

  /**
   * 处理统计显示切换
   */
  handleStatsToggle() {
    this.deviceManager.toggleVideoStats();
    const enabled = this.deviceManager.statsEnabled;
    // 移除视频统计切换通知

    // 更新按钮状态
    if (this.elements.statsBtn) {
      this.elements.statsBtn.textContent = enabled
        ? "📊 关闭统计"
        : "📊 开启统计";
      this.elements.statsBtn.classList.toggle("active", enabled);
    }
  }

  /**
   * 处理房间变更
   */
  handleRoomChange(roomId) {
    this.deviceManager.updateRoom(roomId);
    // 移除房间变更通知
  }

  /**
   * 处理分辨率变更
   */
  handleResolutionChange(resolution) {
    if (resolution === "custom") {
      this.showCustomResolutionDialog();
      return;
    }

    const [width, height] = resolution.split("x").map(Number);
    this.deviceManager.updateResolution(width, height);
    // 移除分辨率变更通知
  }

  /**
   * 处理码率变更
   */
  handleBitrateChange(bitrate) {
    this.deviceManager.updateBitrate(bitrate);
    // 移除码率变更通知
  }

  /**
   * 处理关键帧间隔变更
   */
  handleKeyFrameChange(interval) {
    this.deviceManager.updateKeyFrameInterval(interval);
    // 移除关键帧间隔变更通知
  }

  /**
   * 处理质量等级变更
   */
  handleQualityChange(quality) {
    this.deviceManager.updateQuality(quality);
    // 移除质量等级变更通知
  }

  /**
   * 处理编码器FPS变更
   */
  handleEncoderFpsChange(fps) {
    this.deviceManager.updateEncoderFps(fps);
    // 移除编码器FPS变更通知
  }

  /**
   * 处理旋转变更
   */
  handleRotationChange(rotation) {
    this.deviceManager.updateRotation(rotation);
    // 移除旋转角度变更通知
  }

  /**
   * 处理缩放变更
   */
  handleScaleChange(scale) {
    // 直接处理缩放逻辑，避免递归调用
    console.log(
      `📏 [${this.deviceId}] Applying video scale: ${Math.round(scale * 100)}%`
    );

    // 保存缩放设置到本地存储
    if (typeof window.storage !== "undefined" && window.storage.setVideoScale) {
      window.storage.setVideoScale(this.deviceId, scale);
      console.log(`💾 [${this.deviceId}] Saved scale to storage: ${scale}`);
    }

    // 查找视频元素（多种方式）
    let videoElement = this.elements.remoteVideo;

    // 如果通过elements找不到，尝试其他方式
    if (!videoElement) {
      // const num = this.deviceId.slice(-1);
      const num = this.deviceId.replace("device", "");
      videoElement = document.getElementById(`remoteVideo${num}`);
    }

    // 再次尝试通过全局devices查找
    if (!videoElement && window.devices && window.devices[this.deviceId]) {
      videoElement = window.devices[this.deviceId].elements?.remoteVideo;
    }

    // 应用缩放
    if (videoElement) {
      console.log(
        `✅ [${this.deviceId}] Found video element, applying transform...`
      );

      // 获取当前的旋转角度（如果有的话）
      const currentTransform = window.getComputedStyle(videoElement).transform;
      let rotationDegrees = 0;

      // 从当前transform中提取旋转角度
      if (currentTransform && currentTransform !== "none") {
        const match = currentTransform.match(/rotate\(([^)]+)deg\)/);
        if (match) {
          rotationDegrees = parseFloat(match[1]) || 0;
        }
      }

      // 也从设备管理器获取旋转角度作为备份
      if (this.deviceManager && this.deviceManager.rotation !== undefined) {
        rotationDegrees = this.deviceManager.rotation;
      }

      // 组合旋转和缩放变换
      let combinedTransform;
      if (rotationDegrees !== 0) {
        combinedTransform = `rotate(${rotationDegrees}deg) scale(${scale})`;
        console.log(
          `🔄 [${this.deviceId}] Combined transform: rotate(${rotationDegrees}deg) scale(${scale})`
        );
      } else {
        combinedTransform = `scale(${scale})`;
        console.log(`📏 [${this.deviceId}] Scale only: scale(${scale})`);
      }

      videoElement.style.transform = combinedTransform;
      videoElement.style.transformOrigin = "center center"; // 居中缩放
      videoElement.style.transition = "transform 0.3s ease";

      // 验证样式是否应用成功
      const appliedTransform = window.getComputedStyle(videoElement).transform;
      console.log(`🔍 [${this.deviceId}] Applied transform:`, appliedTransform);

      // 如果设备管理器存在且有旋转角度，重新应用旋转以确保同步
      if (this.deviceManager && this.deviceManager.rotation !== 0) {
        console.log(
          `🔄 [${this.deviceId}] Re-applying rotation ${this.deviceManager.rotation}° to sync with new scale`
        );
        setTimeout(() => {
          if (this.deviceManager.applyRotation) {
            this.deviceManager.applyRotation();
          }
        }, 100); // 稍微延迟以避免变换冲突
      }
    } else {
      console.error(`❌ [${this.deviceId}] Video element not found!`);
      console.log(`🔍 [${this.deviceId}] Debug info:`, {
        "this.elements.remoteVideo": this.elements.remoteVideo,
        "document.getElementById": document.getElementById(
          `remoteVideo${this.deviceId.replace("device", "")}`
        ),
        "window.devices": window.devices?.[this.deviceId]?.elements?.remoteVideo
      });
    }

    // 移除缩放比例变更通知
  }

  /**
   * 处理剪贴板获取
   */
  handleClipboardGet() {
    if (
      this.deviceManager.dataChannel &&
      this.deviceManager.dataChannel.readyState === "open"
    ) {
      const controlMessage = { type: "button_get_clipboard" };
      this.deviceManager.dataChannel.send(JSON.stringify(controlMessage));

      console.log(
        `📋 [${this.deviceId}] Clipboard get message sent:`,
        controlMessage
      );
      // 移除剪贴板获取通知
    } else {
      this.notificationManager.showWarning(
        `[${this.deviceId}] 数据通道未连接，无法获取剪贴板`
      );
    }
  }

  /**
   * 处理剪贴板设置
   */
  handleClipboardSet() {
    // 使用prompt获取用户输入，与原始实现保持一致
    const clipboardText = prompt("请输入要设置到手机剪贴板的内容:", "");

    if (clipboardText !== null && clipboardText.trim() !== "") {
      if (
        this.deviceManager.dataChannel &&
        this.deviceManager.dataChannel.readyState === "open"
      ) {
        const controlMessage = {
          type: "button_set_clipboard",
          text: clipboardText.trim()
        };
        this.deviceManager.dataChannel.send(JSON.stringify(controlMessage));

        console.log(
          `📋 [${this.deviceId}] Clipboard set message sent:`,
          controlMessage
        );
        // 移除剪贴板设置通知
      } else {
        this.notificationManager.showWarning(
          `[${this.deviceId}] 数据通道未连接，无法设置剪贴板`
        );
      }
    }
  }

  /**
   * 处理设备控制
   */
  handleDeviceControl(action) {
    if (
      this.deviceManager.dataChannel &&
      this.deviceManager.dataChannel.readyState === "open"
    ) {
      // 发送正确格式的控制消息
      const controlMessage = { type: "button_" + action };
      this.deviceManager.dataChannel.send(JSON.stringify(controlMessage));

      console.log(
        `🎮 [${this.deviceId}] Control message sent:`,
        controlMessage
      );
      // 移除设备控制操作通知
    } else {
      this.notificationManager.showWarning(
        `[${this.deviceId}] 数据通道未连接，无法发送控制命令`
      );
    }
  }

  /**
   * 处理配置面板切换
   */
  handleConfigToggle() {
    if (this.elements.configPanel) {
      const isVisible = this.elements.configPanel.style.display !== "none";
      this.elements.configPanel.style.display = isVisible ? "none" : "block";

      if (this.elements.toggleConfigBtn) {
        this.elements.toggleConfigBtn.textContent = isVisible
          ? "⚙️ 显示配置"
          : "⚙️ 隐藏配置";
      }
    }
  }

  /**
   * 处理键盘按键
   */
  handleKeyDown(event) {
    // 检查是否是当前设备的焦点区域
    if (!this.isDeviceFocused()) return;

    const key = event.key.toLowerCase();

    // 快捷键处理
    if (event.ctrlKey) {
      switch (key) {
        case "enter":
          event.preventDefault();
          this.handleConnect();
          break;
        case "q":
          event.preventDefault();
          this.handleDisconnect();
          break;
        case "m":
          event.preventDefault();
          this.handleMuteToggle();
          break;
        case "s":
          event.preventDefault();
          this.handleStatsToggle();
          break;
      }
    }

    // 设备控制快捷键
    if (event.altKey) {
      switch (key) {
        case "h":
          event.preventDefault();
          this.handleDeviceControl("home");
          break;
        case "b":
          event.preventDefault();
          this.handleDeviceControl("back");
          break;
        case "r":
          event.preventDefault();
          this.handleDeviceControl("recent");
          break;
        case "p":
          event.preventDefault();
          this.handleDeviceControl("power");
          break;
        case "l":
          event.preventDefault();
          this.handleDeviceControl("lock");
          break;
      }
    }
  }

  /**
   * 检查当前设备是否聚焦
   */
  isDeviceFocused() {
    const activeElement = document.activeElement;
    if (!activeElement) return false;

    const deviceContainer = activeElement.closest(
      `[data-device="${this.deviceId}"]`
    );
    return !!deviceContainer;
  }

  // === 设备管理器回调处理 ===

  /**
   * 更新连接状态
   */
  updateConnectionStatus(connected, status) {
    if (this.elements.statusIcon) {
      this.elements.statusIcon.textContent =
        connected === null ? "🔄" : connected ? "🟢" : "🔴";
    }

    if (this.elements.statusText) {
      this.elements.statusText.textContent = status;
    }

    // 更新按钮状态
    if (this.elements.connectBtn) {
      this.elements.connectBtn.disabled = connected === true;
    }

    if (this.elements.disconnectBtn) {
      this.elements.disconnectBtn.disabled = connected === false;
    }

    // 更新视频容器状态
    if (this.elements.videoContainer) {
      this.elements.videoContainer.classList.toggle(
        "connected",
        connected === true
      );
      this.elements.videoContainer.classList.toggle(
        "disconnected",
        connected === false
      );
    }
  }

  /**
   * 处理视频准备就绪
   */
  handleVideoReady(data) {
    console.log(`📺 [${this.deviceId}] Video ready:`, data);
    // this.notificationManager.showSuccess(`[${this.deviceId}] 视频流已连接`);

    // 可以在这里添加视频就绪后的UI更新逻辑
  }

  /**
   * 处理统计数据更新
   */
  handleStatsUpdate(data) {
    // 这里可以添加额外的统计数据处理逻辑
    // 例如：更新图表、记录历史数据等
  }

  /**
   * 处理错误
   */
  handleError(data) {
    let message = `[${this.deviceId}] 错误`;

    switch (data.type) {
      case "connection":
        message += `: 连接失败 - ${data.error.message}`;
        break;
      case "websocket":
        message += `: WebSocket错误 - ${data.error.message}`;
        break;
      case "peerconnection":
        message += `: WebRTC连接错误 - ${data.error.message}`;
        break;
      case "app-connection":
        message += `: ${data.message}`;
        break;
      default:
        message += `: 未知错误 - ${JSON.stringify(data)}`;
    }

    this.notificationManager.showError(message);
    this.updateConnectionStatus(false, "错误");
  }

  // === 辅助方法 ===

  /**
   * 显示自定义分辨率对话框
   */
  showCustomResolutionDialog() {
    const width = prompt("请输入宽度:", "1080");
    const height = prompt("请输入高度:", "1920");

    if (width && height) {
      const w = parseInt(width);
      const h = parseInt(height);

      if (w > 0 && h > 0) {
        this.deviceManager.updateResolution(w, h);
        // 移除自定义分辨率设置通知
      } else {
        this.notificationManager.showError(`[${this.deviceId}] 无效的分辨率值`);
      }
    }

    // 重置选择器
    if (this.elements.resolutionSelect) {
      this.elements.resolutionSelect.value = `${this.deviceManager.width}x${this.deviceManager.height}`;
    }
  }

  /**
   * 获取设备状态
   */
  getDeviceStatus() {
    return {
      ...this.deviceManager.getDeviceStatus(),
      uiState: {
        configPanelVisible: this.elements.configPanel?.style.display !== "none",
        statsEnabled: this.deviceManager.statsEnabled
      }
    };
  }

  /**
   * 销毁UI控制器
   */
  destroy() {
    // 移除事件监听器
    document.removeEventListener("keydown", this.handleKeyDown);

    // 清理元素引用
    Object.keys(this.elements).forEach(key => {
      this.elements[key] = null;
    });

    console.log(`🗑️ [${this.deviceId}] DeviceUIController destroyed`);
  }
}

/**
 * 全局UI控制器
 * 处理多设备UI和全局交互
 */
export class GlobalUIController {
  constructor() {
    this.deviceControllers = new Map();
    this.notificationManager = new NotificationManager();

    // 全局元素
    this.elements = {
      windowCountSelect: document.getElementById("windowCountSelect"),
      refreshBtn: document.getElementById("refreshBtn"),
      fullscreenBtn: document.getElementById("fullscreenBtn"),
      themeToggleBtn: document.getElementById("themeToggleBtn"),
      globalStatsBtn: document.getElementById("globalStatsBtn"),
      globalConfigBtn: document.getElementById("globalConfigBtn")
    };

    this.bindGlobalEvents();
    console.log(`🌐 GlobalUIController initialized`);
  }

  /**
   * 绑定全局事件
   */
  bindGlobalEvents() {
    // 窗口数量变更
    if (this.elements.windowCountSelect) {
      this.elements.windowCountSelect.addEventListener("change", e => {
        this.handleWindowCountChange(parseInt(e.target.value));
      });
    }

    // 刷新按钮
    if (this.elements.refreshBtn) {
      this.elements.refreshBtn.addEventListener("click", () => {
        this.handleRefresh();
      });
    }

    // 全屏按钮
    if (this.elements.fullscreenBtn) {
      this.elements.fullscreenBtn.addEventListener("click", () => {
        this.handleFullscreen();
      });
    }

    // 主题切换
    if (this.elements.themeToggleBtn) {
      this.elements.themeToggleBtn.addEventListener("click", () => {
        this.handleThemeToggle();
      });
    }

    // 全局统计
    if (this.elements.globalStatsBtn) {
      this.elements.globalStatsBtn.addEventListener("click", () => {
        this.handleGlobalStats();
      });
    }

    // 全局配置
    if (this.elements.globalConfigBtn) {
      this.elements.globalConfigBtn.addEventListener("click", () => {
        this.handleGlobalConfig();
      });
    }

    // 页面卸载时清理
    window.addEventListener("beforeunload", () => {
      this.destroy();
    });
  }

  /**
   * 注册设备控制器
   */
  registerDeviceController(deviceId, controller) {
    this.deviceControllers.set(deviceId, controller);
    console.log(`📝 [Global] Registered device controller: ${deviceId}`);
  }

  /**
   * 注销设备控制器
   */
  unregisterDeviceController(deviceId) {
    const controller = this.deviceControllers.get(deviceId);
    if (controller) {
      controller.destroy();
      this.deviceControllers.delete(deviceId);
      console.log(`📝 [Global] Unregistered device controller: ${deviceId}`);
    }
  }

  /**
   * 处理窗口数量变更
   */
  handleWindowCountChange(count) {
    // 移除窗口数量变更通知
    // 这里可以添加动态创建/销毁窗口的逻辑
  }

  /**
   * 处理页面刷新
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

    // 移除主题切换通知
  }

  /**
   * 处理全局统计
   */
  handleGlobalStats() {
    const allStats = Array.from(this.deviceControllers.values()).map(
      controller => controller.getDeviceStatus()
    );

    console.log("📊 Global Stats:", allStats);
    // 移除全局统计通知
  }

  /**
   * 处理全局配置
   */
  handleGlobalConfig() {
    // 这里可以添加全局配置面板的显示逻辑
    // 移除全局配置面板通知
  }

  /**
   * 连接所有设备
   */
  async connectAllDevices() {
    console.log("🔗 [Global] Connecting all devices...");
    const promises = [];

    this.deviceControllers.forEach((controller, deviceId) => {
      promises.push(controller.handleConnect());
    });

    try {
      await Promise.all(promises);
      // 移除批量连接成功通知
    } catch (error) {
      this.notificationManager.showError(`批量连接失败: ${error.message}`);
    }
  }

  /**
   * 断开所有设备连接
   */
  disconnectAllDevices() {
    console.log("❌ [Global] Disconnecting all devices...");

    this.deviceControllers.forEach((controller, deviceId) => {
      controller.handleDisconnect();
    });

    // 移除批量断开连接通知
  }

  /**
   * 切换所有设备静音状态
   */
  toggleAllMute() {
    console.log("🔊 [Global] Toggling all mute...");
    let mutedCount = 0;
    let totalCount = 0;

    this.deviceControllers.forEach((controller, deviceId) => {
      if (controller.deviceManager) {
        const wasMuted = controller.deviceManager.isMuted;
        controller.handleMuteToggle();
        totalCount++;
        if (!wasMuted) {
          // 如果之前没有静音，现在静音了
          mutedCount++;
        }
      }
    });

    // 移除全局静音切换通知
  }

  /**
   * 切换所有设备视频统计显示
   */
  toggleAllVideoStats() {
    console.log("🔧 [Global] Toggling all video stats...");
    let enabledCount = 0;
    let totalCount = 0;

    this.deviceControllers.forEach((controller, deviceId) => {
      if (controller.deviceManager) {
        const wasEnabled = controller.deviceManager.statsEnabled;
        controller.handleStatsToggle();
        totalCount++;
        if (!wasEnabled) {
          // 如果之前没有开启，现在开启了
          enabledCount++;
        }
      }
    });

    // 移除全局视频统计切换通知
  }

  /**
   * 销毁全局控制器
   */
  destroy() {
    // 销毁所有设备控制器
    this.deviceControllers.forEach((controller, deviceId) => {
      this.unregisterDeviceController(deviceId);
    });

    // 销毁DeviceManager工厂
    DeviceManagerFactory.destroyAll();

    console.log(`🗑️ [Global] GlobalUIController destroyed`);
  }
}

/**
 * UI控制器工厂
 */
export class UIControllerFactory {
  static globalController = null;
  static deviceControllers = new Map();

  /**
   * 获取全局控制器
   */
  static getGlobalController() {
    if (!this.globalController) {
      this.globalController = new GlobalUIController();
    }
    return this.globalController;
  }

  /**
   * 创建设备控制器
   */
  static createDeviceController(deviceId) {
    if (!this.deviceControllers.has(deviceId)) {
      const controller = new DeviceUIController(deviceId);
      this.deviceControllers.set(deviceId, controller);

      // 注册到全局控制器
      const globalController = this.getGlobalController();
      globalController.registerDeviceController(deviceId, controller);

      console.log(`🏭 [UIFactory] Created DeviceUIController for ${deviceId}`);
    }
    return this.deviceControllers.get(deviceId);
  }

  /**
   * 销毁设备控制器
   */
  static destroyDeviceController(deviceId) {
    const controller = this.deviceControllers.get(deviceId);
    if (controller) {
      // 从全局控制器注销
      const globalController = this.getGlobalController();
      globalController.unregisterDeviceController(deviceId);

      this.deviceControllers.delete(deviceId);
      console.log(
        `🏭 [UIFactory] Destroyed DeviceUIController for ${deviceId}`
      );
    }
  }

  /**
   * 销毁所有控制器
   */
  static destroyAll() {
    this.deviceControllers.forEach((controller, deviceId) => {
      this.destroyDeviceController(deviceId);
    });

    if (this.globalController) {
      this.globalController.destroy();
      this.globalController = null;
    }

    console.log(`🏭 [UIFactory] Destroyed all controllers`);
  }
}
