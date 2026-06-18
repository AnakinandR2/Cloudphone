/**
 * 键盘输入控制器
 * 负责处理发送给设备的键盘输入事件
 */

/**
 * 设备键盘输入控制器
 * 处理单个设备的键盘输入并发送给设备
 */
export class KeyboardController {
  constructor(deviceId) {
    this.deviceId = deviceId;
    this.isActive = false;

    // 跟踪鼠标位置
    this.mousePosition = { x: 0, y: 0 };

    // 绑定事件处理器
    this.boundHandleKeyDown = this.handleKeyDown.bind(this);
    this.boundHandleMouseMove = this.handleMouseMove.bind(this);

    // 绑定键盘事件
    this.bindEvents();

    console.log(`⌨️ [${this.deviceId}] KeyboardController initialized`);
  }

  /**
   * 绑定键盘事件监听器
   */
  bindEvents() {
    document.addEventListener("keydown", this.boundHandleKeyDown);
    document.addEventListener("mousemove", this.boundHandleMouseMove);
    console.log(`⌨️ [${this.deviceId}] Keyboard events bound`);
  }

  /**
   * 移除键盘事件监听器
   */
  unbindEvents() {
    document.removeEventListener("keydown", this.boundHandleKeyDown);
    document.removeEventListener("mousemove", this.boundHandleMouseMove);
    console.log(`⌨️ [${this.deviceId}] Keyboard events unbound`);
  }

  /**
   * 处理鼠标移动事件，跟踪鼠标位置
   */
  handleMouseMove(event) {
    this.mousePosition.x = event.clientX;
    this.mousePosition.y = event.clientY;
  }

  /**
   * 激活键盘控制器
   */
  activate() {
    this.isActive = true;
    KeyboardControllerFactory.setActiveDevice(this.deviceId);
    console.log(`⌨️ [${this.deviceId}] Keyboard controller activated`);
  }

  // 设置剪贴板内容
  setClipboardContent(content) {
    document.querySelector("input").value = content;
  }

  // 备用剪贴板读取方法
  tryFallbackClipboardRead() {
    console.log("🔍 尝试备用剪贴板读取方法...");

    // 创建隐藏的文本域用于获取剪贴板内容
    const textArea = document.createElement("textarea");
    textArea.style.position = "fixed";
    textArea.style.opacity = "0";
    textArea.style.left = "-9999px";
    textArea.style.top = "-9999px";
    document.body.appendChild(textArea);

    // 聚焦并尝试粘贴
    textArea.focus();

    try {
      // 模拟Ctrl+V操作
      if (document.execCommand("paste")) {
        const clipboardText = textArea.value;
        if (clipboardText) {
          console.log("🔍🔍🔍🔍 备用方法获取剪贴板内容:", clipboardText);
        } else {
          console.warn("🔍 备用方法未获取到剪贴板内容");
        }
      } else {
        console.warn("🔍 备用方法: execCommand paste 失败");
      }
    } catch (error) {
      console.warn("🔍 备用方法异常:", error.message);
    } finally {
      // 清理
      document.body.removeChild(textArea);
    }
  }
  // 判断鼠标是否在设备窗口上
  isMouseInDeviceWindow() {
    const deviceWindow = document.querySelector(
      `#screen${this.deviceId.replace(/\D/g, "")}`
    );
    if (deviceWindow) {
      const rect = deviceWindow.getBoundingClientRect();
      const mouseX = this.mousePosition.x;
      const mouseY = this.mousePosition.y;
      return (
        mouseX > rect.left &&
        mouseX < rect.right &&
        mouseY > rect.top &&
        mouseY < rect.bottom
      );
    }
    return false;
  }
  /**
   * 处理键盘按键
   */
  async handleKeyDown(event) {
    // 只处理活跃设备的键盘输入
    if (KeyboardControllerFactory.activeDevice !== this.deviceId) {
      return;
    }

    // 检查是否需要忽略的特殊情况
    if (this.shouldIgnoreEvent(event)) {
      return;
    }

    console.log(`⌨️ [${this.deviceId}] Key event:`, {
      key: event.key,
      code: event.code,
      ctrlKey: event.ctrlKey,
      altKey: event.altKey,
      shiftKey: event.shiftKey
    });

    // 生成键盘消息
    const keyMessage = this.createKeyMessage(event);

    if (keyMessage) {
      // event.preventDefault();
      if (!event.ctrlKey) {
        this.sendControlMessage(keyMessage);
      }
      console.log(`⌨️ [${this.deviceId}] Key sent:`, keyMessage);
    }
    // 判断是否ctrl+c
    if (event.ctrlKey && (event.key === "c" || event.key === "C")) {
      // 使用跟踪的鼠标位置来判断鼠标是否在设备窗口上
      if (this.isMouseInDeviceWindow()) {
        this.sendControlMessage({ type: "clipboard", action: "get" });
      } else {
        // 延迟读取剪贴板，等待系统完成复制操作
        setTimeout(async () => {
          try {
            // 检查剪贴板权限
            const permission = await navigator.permissions.query({
              name: "clipboard-read"
            });
            if (permission.state === "denied") {
              console.warn("🔍 剪贴板读取权限被拒绝");
              return;
            }

            const text = await navigator.clipboard.readText();
            console.log("读取-剪贴板内容:", text);

            // 如果获取到内容，可以选择同步到设备
            if (text && text.trim()) {
              // this.sendControlMessage({ type: "clipboard", action: "set", content: text });
            }
          } catch (error) {
            console.warn("🔍 读取剪贴板失败:", error.message);

            // 尝试备用方案
            this.tryFallbackClipboardRead();
          }
        }, 100); // 延迟100ms等待系统完成复制
      }
    }
    // 判断是否ctrl+v
    if (event.ctrlKey && (event.key === "v" || event.key === "V")) {
      // this.setClipboardContent("wwww");
      if (this.isMouseInDeviceWindow()) {
        // this.sendControlMessage({ type: "clipboard", action: "set", content: "wwww" });
        // this.sendControlMessage({ type: "clipboard", action: "set" });
        // 延迟读取剪贴板，等待系统完成复制操作
        setTimeout(async () => {
          try {
            // 检查剪贴板权限
            const permission = await navigator.permissions.query({
              name: "clipboard-read"
            });
            if (permission.state === "denied") {
              console.warn("🔍 剪贴板读取权限被拒绝");
              return;
            }

            const text = await navigator.clipboard.readText();
            console.log("剪贴-读取-剪贴板内容:", text);

            // 如果获取到内容，可以选择同步到设备
            if (text && text.trim()) {
              this.sendControlMessage({
                type: "clipboard",
                action: "set",
                content: text
              });
            }
          } catch (error) {
            console.warn("🔍 读取剪贴板失败:", error.message);

            // 尝试备用方案
            this.tryFallbackClipboardRead();
          }
        }, 100);
      } else {
        // setTimeout(async () => {
        //   const text = await this.getClipboardContent();
        //   this.sendControlMessage({ type: "clipboard", action: "set", content: text });
        // }, 100);
      }
    }
  }

  /**
   * 检查是否应该忽略此键盘事件
   */
  shouldIgnoreEvent(event) {
    // 忽略UI快捷键组合
    if (event.ctrlKey) {
      const ctrlKeys = ["enter", "q", "m", "s"];
      if (ctrlKeys.includes(event.key.toLowerCase())) {
        return true;
      }
    }

    // 忽略设备控制快捷键组合
    if (event.altKey) {
      const altKeys = ["h", "b", "r", "p", "l"];
      if (altKeys.includes(event.key.toLowerCase())) {
        return true;
      }
    }

    // 忽略全局快捷键组合
    if (event.ctrlKey && event.altKey) {
      const globalKeys = ["r", "f", "t", "c"];
      if (globalKeys.includes(event.key.toLowerCase())) {
        return true;
      }
    }

    // 忽略在输入框中的输入
    const activeElement = document.activeElement;
    if (
      activeElement &&
      (activeElement.tagName === "INPUT" ||
        activeElement.tagName === "TEXTAREA" ||
        activeElement.contentEditable === "true")
    ) {
      return true;
    }

    return false;
  }

  /**
   * 创建键盘消息
   */
  createKeyMessage(event) {
    switch (event.code) {
      case "Escape":
        return { type: "key_back" };
      case "Home":
        return { type: "key_home" };
      case "Enter":
        return { type: "key_enter" };
      case "Backspace":
        return { type: "key_backspace" };
      case "Delete":
        return { type: "key_delete" };
      case "Tab":
        return { type: "key_tab" };
      case "Space":
        return { type: "key_space" };
      case "ArrowUp":
        return { type: "key_arrow", direction: "up" };
      case "ArrowDown":
        return { type: "key_arrow", direction: "down" };
      case "ArrowLeft":
        return { type: "key_arrow", direction: "left" };
      case "ArrowRight":
        return { type: "key_arrow", direction: "right" };
      default:
        // 普通字符
        if (event.key.length === 1) {
          return {
            type: "key_char",
            char: event.key,
            code: event.code
          };
        }
        break;
    }

    return null;
  }

  /**
   * 获取设备管理器
   */
  getDeviceManager() {
    return window.devices && window.devices[this.deviceId];
  }
  sendControlMessageAll(message) {
    // 如果 deviceControllers 是 Map 对象，使用 forEach 遍历
    if (window.getApp().deviceControllers instanceof Map) {
      window.getApp().deviceControllers.forEach(controller => {
        // controller.sendControlMessage(message);
        try {
          const dataChannel = controller.deviceManager?.dataChannel;
          if (dataChannel && dataChannel.readyState === "open") {
            dataChannel.send(JSON.stringify(message));
          } else {
            console.warn(
              `⚠️ [${controller.deviceManager?.deviceId || "unknown"}] DataChannel not available or not open, readyState: ${dataChannel?.readyState}`
            );
          }
        } catch (error) {
          console.error(`❌ [${this.deviceId}] Failed to send control:`, error);
        }
      });
    } else {
      // 如果是数组，使用 map 遍历
      window.getApp().deviceControllers.map(controller => {
        // controller.sendControlMessage(message);
        try {
          const dataChannel = controller.deviceManager?.dataChannel;
          if (dataChannel && dataChannel.readyState === "open") {
            dataChannel.send(JSON.stringify(message));
          } else {
            console.warn(
              `⚠️ [${controller.deviceManager?.deviceId || "unknown"}] DataChannel not available or not open, readyState: ${dataChannel?.readyState}`
            );
          }
        } catch (error) {
          console.error(`❌ [${this.deviceId}] Failed to send control:`, error);
        }
      });
    }
  }
  /**
   * 发送控制消息
   */
  sendControlMessage(message) {
    const deviceManager = this.getDeviceManager();
    const deviceId = deviceManager.deviceId;
    if (window.CphoneWebRTC.isSync) {
      const isMain = window.CphoneWebRTCConfig.roomList.find(
        item => "device" + item.deviceId === deviceId
      )?.isMain;
      if (isMain) {
        this.sendControlMessageAll(message);
        return;
      }
    }

    if (!deviceManager) {
      console.warn(`⚠️ [${this.deviceId}] Device manager not found`);
      return;
    }

    // 获取DataChannel
    const dataChannel =
      deviceManager.dataChannel || window[`dataChannel_${this.deviceId}`];

    if (!dataChannel || dataChannel.readyState !== "open") {
      console.warn(`⚠️ [${this.deviceId}] DataChannel not available`);
      return;
    }

    try {
      const jsonMessage = JSON.stringify(message);
      dataChannel.send(jsonMessage);
      console.log(`⌨️ [${this.deviceId}] Control sent:`, message);
    } catch (error) {
      console.error(`❌ [${this.deviceId}] Failed to send control:`, error);
    }
  }

  /**
   * 销毁键盘控制器
   */
  destroy() {
    this.unbindEvents();
    this.isActive = false;
    console.log(`⌨️ [${this.deviceId}] KeyboardController destroyed`);
  }
}

/**
 * 键盘控制器工厂
 * 管理多个设备的键盘控制器
 */
export class KeyboardControllerFactory {
  static controllers = new Map();
  static activeDevice = null;

  /**
   * 创建键盘控制器
   */
  static createController(deviceId) {
    if (this.controllers.has(deviceId)) {
      console.log(`⌨️ [${deviceId}] Keyboard controller already exists`);
      return this.controllers.get(deviceId);
    }

    const controller = new KeyboardController(deviceId);
    this.controllers.set(deviceId, controller);

    console.log(`⌨️ [${deviceId}] Keyboard controller created`);
    return controller;
  }

  /**
   * 获取键盘控制器
   */
  static getController(deviceId) {
    return this.controllers.get(deviceId);
  }

  /**
   * 设置活跃设备
   */
  static setActiveDevice(deviceId) {
    if (this.activeDevice === deviceId) return;

    // 取消之前活跃设备的激活状态
    if (this.activeDevice) {
      const prevController = this.controllers.get(this.activeDevice);
      if (prevController) {
        prevController.isActive = false;
      }
    }

    // 设置新的活跃设备
    this.activeDevice = deviceId;
    const controller = this.controllers.get(deviceId);
    if (controller) {
      controller.isActive = true;
    }

    console.log(`⌨️ Active keyboard device set to: ${deviceId}`);
  }

  /**
   * 销毁键盘控制器
   */
  static destroyController(deviceId) {
    const controller = this.controllers.get(deviceId);
    if (controller) {
      controller.destroy();
      this.controllers.delete(deviceId);

      // 如果销毁的是活跃设备，清除活跃状态
      if (this.activeDevice === deviceId) {
        this.activeDevice = null;
      }

      console.log(`⌨️ [${deviceId}] Keyboard controller destroyed`);
    }
  }

  /**
   * 销毁所有键盘控制器
   */
  static destroyAll() {
    this.controllers.forEach((controller, deviceId) => {
      controller.destroy();
    });
    this.controllers.clear();
    this.activeDevice = null;

    console.log("⌨️ All keyboard controllers destroyed");
  }

  /**
   * 获取所有键盘控制器
   */
  static getAllControllers() {
    return Array.from(this.controllers.values());
  }

  /**
   * 调试信息
   */
  static getDebugInfo() {
    return {
      activeDevice: this.activeDevice,
      controllerCount: this.controllers.size,
      controllers: Array.from(this.controllers.keys())
    };
  }
}

// 暴露到全局作用域供调试
window.KeyboardControllerFactory = KeyboardControllerFactory;

// 全局调试函数
window.debugKeyboard = function (deviceId = "device1") {
  console.log(`⌨️ 键盘系统调试 - 设备: ${deviceId}`);

  const debug = KeyboardControllerFactory.getDebugInfo();
  console.log("📊 键盘控制器状态:", debug);

  const controller = KeyboardControllerFactory.getController(deviceId);
  console.log(`🎯 设备${deviceId}的控制器:`, {
    exists: !!controller,
    isActive: controller?.isActive,
    activeDevice: KeyboardControllerFactory.activeDevice
  });

  // 检查设备管理器
  const deviceManager = window.devices && window.devices[deviceId];
  console.log("📱 设备管理器:", {
    exists: !!deviceManager,
    dataChannelReady: !!(
      deviceManager?.dataChannel &&
      deviceManager.dataChannel.readyState === "open"
    )
  });

  return debug;
};

window.testKeyboard = function (deviceId = "device1", char = "a") {
  console.log(`⌨️ 测试键盘输入: ${deviceId} - ${char}`);

  const controller = KeyboardControllerFactory.getController(deviceId);
  if (!controller) {
    console.error(`❌ 找不到设备${deviceId}的键盘控制器`);
    return false;
  }

  // 激活设备
  KeyboardControllerFactory.setActiveDevice(deviceId);

  // 模拟键盘事件
  const mockEvent = {
    key: char,
    code: `Key${char.toUpperCase()}`,
    ctrlKey: false,
    altKey: false,
    shiftKey: false,
    preventDefault: () => console.log("preventDefault called")
  };

  controller.handleKeyDown(mockEvent);

  return true;
};

console.log("⌨️ KeyboardController system initialized");
