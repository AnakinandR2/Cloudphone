/**
 * 集成鼠标控制模块
 * 替代mouse-multi.js，与新的模块化系统集成
 */

import { matrixCache } from "../math/matrix.js";
import { getTime } from "../utils/storage.js";
import { CoordinateTransformer } from "./coordinate-transform.js";
import { KeyboardControllerFactory } from "./keyboard-controller.js";
/**
 * 鼠标控制器类
 * 为单个设备提供鼠标和触摸控制
 */
export class MouseController {
  constructor(deviceId, videoElement) {
    this.deviceId = deviceId;
    this.videoElement = videoElement;
    this.coordinateTransformer = new CoordinateTransformer(matrixCache);

    // 鼠标状态
    this.isMouseDown = false;
    this.lastMousePos = { x: 0, y: 0 };
    this.isDragging = false;
    this.dragStartPos = { x: 0, y: 0 };

    // 触摸状态
    this.lastTouchTime = 0;
    this.touchStartTime = 0;
    this.touchStartPos = { x: 0, y: 0 };

    // 滚轮控制
    this.lastScrollTime = 0;
    this.scrollCooldown = 50;

    // 唯一id
    this.uniqueId = "";

    // 初始化键盘控制器
    this.keyboardController =
      KeyboardControllerFactory.createController(deviceId);

    this.bindEvents();
    console.log(
      getTime(),
      `🖱️ [${this.deviceId}] MouseController initialized with keyboard support`
    );
  }

  /**
   * 绑定事件监听器
   */
  bindEvents() {
    if (!this.videoElement) return;

    // 鼠标事件
    this.videoElement.addEventListener(
      "mousedown",
      this.onMouseDown.bind(this)
    );
    this.videoElement.addEventListener(
      "mousemove",
      this.onMouseMove.bind(this)
    );
    this.videoElement.addEventListener("mouseup", this.onMouseUp.bind(this));
    this.videoElement.addEventListener("click", this.onClick.bind(this));
    this.videoElement.addEventListener(
      "dblclick",
      this.onDoubleClick.bind(this)
    );
    this.videoElement.addEventListener(
      "contextmenu",
      this.onContextMenu.bind(this)
    );
    this.videoElement.addEventListener("wheel", this.onWheel.bind(this));

    // 触摸事件
    this.videoElement.addEventListener(
      "touchstart",
      this.onTouchStart.bind(this)
    );
    this.videoElement.addEventListener(
      "touchmove",
      this.onTouchMove.bind(this)
    );
    this.videoElement.addEventListener("touchend", this.onTouchEnd.bind(this));

    // 防止默认行为
    this.videoElement.addEventListener("selectstart", this.preventDefault);
    this.videoElement.addEventListener("dragstart", this.preventDefault);

    // 焦点管理 - 激活键盘控制器
    this.videoElement.addEventListener("mouseenter", () => {
      this.setActiveDevice();
    });
    // 离开手机界面，鼠标抬起
    this.videoElement.addEventListener("mouseleave", this.onMouseUp.bind(this));

    console.log(getTime(), `🔗 [${this.deviceId}] Event listeners bound`);
  }

  /**
   * 移除事件监听器
   */
  unbindEvents() {
    if (!this.videoElement) return;

    this.videoElement.removeEventListener("mousedown", this.onMouseDown);
    this.videoElement.removeEventListener("mousemove", this.onMouseMove);
    this.videoElement.removeEventListener("mouseup", this.onMouseUp);
    this.videoElement.removeEventListener("click", this.onClick);
    this.videoElement.removeEventListener("dblclick", this.onDoubleClick);
    this.videoElement.removeEventListener("contextmenu", this.onContextMenu);
    this.videoElement.removeEventListener("wheel", this.onWheel);

    this.videoElement.removeEventListener("touchstart", this.onTouchStart);
    this.videoElement.removeEventListener("touchmove", this.onTouchMove);
    this.videoElement.removeEventListener("touchend", this.onTouchEnd);

    this.videoElement.removeEventListener("selectstart", this.preventDefault);
    this.videoElement.removeEventListener("dragstart", this.preventDefault);

    console.log(getTime(), `🔗 [${this.deviceId}] Event listeners removed`);
  }

  /**
   * 阻止默认事件
   */
  preventDefault(e) {
    e.preventDefault();
    return false;
  }

  /**
   * 获取设备管理器
   */
  getDeviceManager() {
    return window.devices && window.devices[this.deviceId];
  }

  /**
   * 获取设备旋转角度
   */
  getDeviceRotation() {
    const deviceManager = this.getDeviceManager();
    return deviceManager ? deviceManager.rotation : 0;
  }

  /**
   * 判断当前视频流是否为横屏
   */
  isVideoLandscape() {
    if (
      !this.videoElement ||
      !this.videoElement.videoWidth ||
      !this.videoElement.videoHeight
    ) {
      return false; // 默认竖屏
    }

    // 根据实际视频流的宽高比判断横竖屏
    return this.videoElement.videoWidth > this.videoElement.videoHeight;
  }

  /**
   * 获取相对坐标
   */
  getRelativePosition(event) {
    const rect = this.videoElement.getBoundingClientRect();
    const deviceManager = this.getDeviceManager();

    if (!deviceManager) {
      console.warn(`⚠️ [${this.deviceId}] Device manager not found`);
      return { x: 0, y: 0, width: 1080, height: 1920 };
    }

    let clientX, clientY;

    if (event.touches && event.touches.length > 0) {
      clientX = event.touches[0].clientX;
      clientY = event.touches[0].clientY;
    } else if (event.changedTouches && event.changedTouches.length > 0) {
      clientX = event.changedTouches[0].clientX;
      clientY = event.changedTouches[0].clientY;
    } else {
      clientX = event.clientX;
      clientY = event.clientY;
    }

    // 计算在视频元素上的相对坐标
    const displayX = clientX - rect.left;
    const displayY = clientY - rect.top;

    // 获取选择的分辨率
    const selectedWidth = deviceManager.width;
    const selectedHeight = deviceManager.height;

    // 获取当前旋转角度
    const rotation = this.getDeviceRotation();

    // 判断当前视频流是否为横屏
    const isLandscape = this.isVideoLandscape();

    // 根据当前是否为横屏来确定返回的分辨率
    let currentWidth, currentHeight;
    if (isLandscape) {
      // 横屏：宽度取较大值，高度取较小值
      currentWidth = Math.max(selectedWidth, selectedHeight);
      currentHeight = Math.min(selectedWidth, selectedHeight);
    } else {
      // 竖屏：宽度取较小值，高度取较大值
      currentWidth = Math.min(selectedWidth, selectedHeight);
      currentHeight = Math.max(selectedWidth, selectedHeight);
    }

    // 显示区域信息
    const displayRect = {
      width: rect.width,
      height: rect.height
    };

    console.log(getTime(), `📱 [${this.deviceId}] 设备信息:`, {
      原始分辨率: { width: selectedWidth, height: selectedHeight },
      当前横屏: isLandscape,
      当前分辨率: { width: currentWidth, height: currentHeight },
      当前旋转: rotation,
      显示区域: displayRect,
      点击位置: { displayX, displayY }
    });

    // 使用坐标转换器
    const deviceCoords =
      this.coordinateTransformer.convertDisplayToDeviceCoords(
        displayX,
        displayY,
        displayRect,
        currentWidth,
        currentHeight,
        rotation,
        this.videoElement
      );

    return {
      x: deviceCoords.x,
      y: deviceCoords.y,
      width: currentWidth, // 根据当前横竖屏状态返回分辨率
      height: currentHeight // 根据当前横竖屏状态返回分辨率
    };
  }
  // 因分辨率不一样，等比例转换 计算x,y
  getEqualScalePosition(message = {}, deviceManager = {}) {
    if (
      message?.width === deviceManager?.width &&
      message?.height === deviceManager?.height
    ) {
      return message;
    }
    const scale = deviceManager.width / message.width;
    return {
      ...message,
      width: deviceManager.width,
      height: deviceManager.height,
      x: message.x * scale,
      y: message.y * scale
    };
  }

  sendControlMessageAll(message) {
    // sdsdfsadf
    if (window.getApp().deviceControllers instanceof Map) {
      window.getApp().deviceControllers.forEach(controller => {
        try {
          const dataChannel = controller.deviceManager?.dataChannel;
          if (dataChannel && dataChannel.readyState === "open") {
            dataChannel.send(
              JSON.stringify(
                this.getEqualScalePosition(message, controller.deviceManager)
              )
            );
          } else {
            console.warn(
              `⚠️ [${controller.deviceManager?.deviceId || "unknown"}] DataChannel not available or not open, readyState: ${dataChannel?.readyState}`
            );
          }
        } catch (error) {
          console.error(
            `❌ [${controller.deviceManager?.deviceId || "unknown"}] Failed to send control:`,
            error
          );
        }
      });
    } else {
      // 如果是数组，使用 map 遍历
      window.getApp().deviceControllers.map(controller => {
        try {
          const dataChannel = controller.deviceManager?.dataChannel;
          if (dataChannel && dataChannel.readyState === "open") {
            dataChannel.send(
              JSON.stringify(
                this.getEqualScalePosition(message, controller.deviceManager)
              )
            );
          } else {
            console.warn(
              `⚠️ [${controller.deviceManager?.deviceId || "unknown"}] DataChannel not available or not open, readyState: ${dataChannel?.readyState}`
            );
          }
        } catch (error) {
          console.error(
            `❌ [${controller.deviceManager?.deviceId || "unknown"}] Failed to send control:`,
            error
          );
        }
      });
    }
  }
  /**
   * 发送控制消息
   */
  sendControlMessage(message) {
    const deviceManager = this.getDeviceManager();
    const deviceId = deviceManager?.deviceId;
    if (window.CphoneWebRTC.isSync) {
      const isMain = window.CphoneWebRTCConfig.roomList.find(
        item => "device" + item.deviceId === deviceId
      )?.isMain;
      console.log(getTime(), `🎮 [${this.deviceId}] Control sent:`, message);
      if (isMain) {
        console.log(getTime(), `🎮 [${this.deviceId}] Control sent:`, message);
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
      console.log(getTime(), `🎮 [${this.deviceId}] Control sent:`, message);
    } catch (error) {
      console.error(`❌ [${this.deviceId}] Failed to send control:`, error);
    }
  }
  //生成唯一的id
  generateUniqueId() {
    // 生成原始UUID部分
    const uuid = "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(
      /[xy]/g,
      function (c) {
        const r = (Math.random() * 16) | 0;
        const v = c === "x" ? r : (r & 0x3) | 0x8;
        return v.toString(16);
      }
    );

    // 获取当前时间并格式化为 '20240506 12:45:12:123'
    const now = new Date();
    const year = now.getFullYear();
    const month = (now.getMonth() + 1).toString().padStart(2, "0");
    const day = now.getDate().toString().padStart(2, "0");
    const hours = now.getHours().toString().padStart(2, "0");
    const minutes = now.getMinutes().toString().padStart(2, "0");
    const seconds = now.getSeconds().toString().padStart(2, "0");
    const milliseconds = now.getMilliseconds().toString().padStart(3, "0");

    // 格式化时间字符串：年月日 + 时分秒毫秒
    const timeString = `${year}${month}${day} ${hours}:${minutes}:${seconds}:${milliseconds}`;

    // 组合UUID和时间
    return `${uuid}-${timeString}`;
  }

  // === 鼠标事件处理 ===

  onMouseDown(event) {
    this.uniqueId = this.generateUniqueId();
    event.preventDefault();

    // 激活当前设备（鼠标和键盘）
    this.setActiveDevice();

    this.isMouseDown = true;
    const pos = this.getRelativePosition(event);
    const rotation = this.getDeviceRotation();
    this.lastMousePos = pos;
    this.dragStartPos = pos;

    this.sendControlMessage({
      type: "mouse_down",
      x: pos.x,
      y: pos.y,
      button: event.button,
      rotation: rotation,
      width: pos.width,
      height: pos.height,
      messageId: this.uniqueId
    });

    console.log(
      getTime(),
      `🖱️ [${this.deviceId}] =====================Mouse down at (${pos.x}, ${pos.y})`,
      {
        type: "mouse_down",
        x: pos.x,
        y: pos.y,
        button: event.button,
        rotation: rotation,
        width: pos.width,
        height: pos.height,
        messageId: this.uniqueId
      }
    );
  }

  onMouseMove(event) {
    event.preventDefault();

    if (!this.isMouseDown) return;

    const pos = this.getRelativePosition(event);
    const rotation = this.getDeviceRotation();

    // 检查是否开始拖拽
    if (!this.isDragging) {
      const distance = Math.sqrt(
        Math.pow(pos.x - this.dragStartPos.x, 2) +
          Math.pow(pos.y - this.dragStartPos.y, 2)
      );

      if (distance > 5) {
        this.isDragging = true;
      }
    }

    if (this.isDragging) {
      this.sendControlMessage({
        type: "mouse_move",
        x: pos.x,
        y: pos.y,
        deltaX: pos.x - this.lastMousePos.x,
        deltaY: pos.y - this.lastMousePos.y,
        rotation: rotation,
        width: pos.width,
        height: pos.height,
        downEventId: this.uniqueId,
        messageId: this.generateUniqueId()
      });
    }

    this.lastMousePos = pos;
  }

  onMouseUp(event) {
    event.preventDefault();
    if (!this.isMouseDown) return;
    const pos = this.getRelativePosition(event);
    const rotation = this.getDeviceRotation();

    this.sendControlMessage({
      type: "mouse_up",
      x: pos.x,
      y: pos.y,
      button: event.button,
      rotation: rotation,
      width: pos.width,
      height: pos.height,
      downEventId: this.uniqueId,
      messageId: this.generateUniqueId()
    });

    this.isMouseDown = false;
    this.isDragging = false;

    console.log(
      getTime(),
      `🖱️ [${this.deviceId}] Mouse up at (${pos.x}, ${pos.y})`
    );
  }

  onClick(event) {
    event.preventDefault();

    if (this.isDragging) return; // 忽略拖拽后的点击

    const pos = this.getRelativePosition(event);
    console.log(
      getTime(),
      `🖱️ [${this.deviceId}] Click at (${pos.x}, ${pos.y})`
    );
  }

  onDoubleClick(event) {
    event.preventDefault();

    const pos = this.getRelativePosition(event);

    this.sendControlMessage({
      type: "mouse_double_click",
      x: pos.x,
      y: pos.y,
      width: pos.width,
      height: pos.height,
      downEventId: this.uniqueId,
      messageId: this.generateUniqueId()
    });

    console.log(
      getTime(),
      `🖱️ [${this.deviceId}] Double click at (${pos.x}, ${pos.y})`
    );
  }

  onContextMenu(event) {
    event.preventDefault();

    const pos = this.getRelativePosition(event);

    this.sendControlMessage({
      type: "mouse_right_click",
      x: pos.x,
      y: pos.y,
      width: pos.width,
      height: pos.height,
      downEventId: this.uniqueId,
      messageId: this.generateUniqueId(),
      downEventId: this.uniqueId,
      messageId: this.generateUniqueId()
    });

    console.log(
      getTime(),
      `🖱️ [${this.deviceId}] Right click at (${pos.x}, ${pos.y})`
    );
  }

  onWheel(event) {
    event.preventDefault();

    const currentTime = Date.now();
    if (currentTime - this.lastScrollTime < this.scrollCooldown) {
      return;
    }

    const pos = this.getRelativePosition(event);
    const rotation = this.getDeviceRotation();
    const deltaY = event.deltaY;
    const absDeltaY = Math.abs(deltaY);

    let scrollType;
    if (event.ctrlKey) {
      scrollType = deltaY > 0 ? "mouse_zoom_out" : "mouse_zoom_in";
    } else {
      scrollType = deltaY > 0 ? "mouse_scroll_up" : "mouse_scroll_down";
    }

    let intensity = 1;
    if (absDeltaY > 50) {
      intensity = 3;
    } else if (absDeltaY > 20) {
      intensity = 2;
    }

    this.sendControlMessage({
      type: scrollType,
      x: pos.x,
      y: pos.y,
      deltaY: deltaY,
      intensity: intensity,
      rotation: rotation,
      width: pos.width,
      height: pos.height,
      downEventId: this.uniqueId,
      messageId: this.generateUniqueId()
    });

    this.lastScrollTime = currentTime;
    console.log(
      getTime(),
      `🖱️ [${this.deviceId}] ${scrollType} at (${pos.x}, ${pos.y})`
    );
  }

  // === 触摸事件处理 ===

  onTouchStart(event) {
    event.preventDefault();

    if (event.touches.length === 1) {
      const pos = this.getRelativePosition(event);
      const rotation = this.getDeviceRotation();
      this.touchStartPos = pos;
      this.touchStartTime = Date.now();

      this.sendControlMessage({
        type: "mouse_down",
        x: pos.x,
        y: pos.y,
        rotation: rotation,
        width: pos.width,
        height: pos.height,
        downEventId: this.uniqueId,
        messageId: this.generateUniqueId()
      });

      console.log(
        getTime(),
        `👆 [${this.deviceId}] Touch start at (${pos.x}, ${pos.y})`
      );
    }
  }

  onTouchMove(event) {
    event.preventDefault();

    if (event.touches.length === 1) {
      const pos = this.getRelativePosition(event);
      const rotation = this.getDeviceRotation();

      this.sendControlMessage({
        type: "mouse_move",
        x: pos.x,
        y: pos.y,
        rotation: rotation,
        width: pos.width,
        height: pos.height,
        downEventId: this.uniqueId,
        messageId: this.generateUniqueId()
      });
    }
  }

  onTouchEnd(event) {
    event.preventDefault();

    if (event.changedTouches.length === 1) {
      const pos = this.getRelativePosition(event.changedTouches[0]);
      const rotation = this.getDeviceRotation();

      this.sendControlMessage({
        type: "mouse_up",
        x: pos.x,
        y: pos.y,
        rotation: rotation,
        width: pos.width,
        height: pos.height,
        downEventId: this.uniqueId,
        messageId: this.generateUniqueId()
      });

      console.log(
        getTime(),
        `👆 [${this.deviceId}] Touch end at (${pos.x}, ${pos.y})`
      );
    }
  }

  /**
   * 设置活跃设备（包括键盘）
   */
  setActiveDevice() {
    // 设置鼠标活跃设备
    if (window.setActiveDevice) {
      window.setActiveDevice(this.deviceId);
    }

    // 激活键盘控制器
    KeyboardControllerFactory.setActiveDevice(this.deviceId);

    console.log(
      getTime(),
      `🎯 [${this.deviceId}] Set as active device (mouse + keyboard)`
    );
  }

  /**
   * 销毁控制器
   */
  destroy() {
    this.unbindEvents();
    this.coordinateTransformer = null;

    // 销毁键盘控制器
    KeyboardControllerFactory.destroyController(this.deviceId);

    console.log(getTime(), `🗑️ [${this.deviceId}] MouseController destroyed`);
  }
}

/**
 * 鼠标控制器工厂
 */
export class MouseControllerFactory {
  static controllers = new Map();
  static activeDevice = null;

  /**
   * 创建鼠标控制器
   */
  static createController(deviceId) {
    if (this.controllers.has(deviceId)) {
      console.warn(`⚠️ [${deviceId}] MouseController already exists`);
      return this.controllers.get(deviceId);
    }

    const videoElement = document.querySelector(
      `#screen${deviceId.replace("device", "")}`
    );
    if (!videoElement) {
      console.error(`❌ [${deviceId}] Video element not found`);
      return null;
    }

    const controller = new MouseController(deviceId, videoElement);
    this.controllers.set(deviceId, controller);

    console.log(
      getTime(),
      `🏭 [Factory] Created MouseController for ${deviceId}`
    );
    return controller;
  }

  /**
   * 获取控制器
   */
  static getController(deviceId) {
    return this.controllers.get(deviceId);
  }

  /**
   * 设置活动设备
   */
  static setActiveDevice(deviceId) {
    this.activeDevice = deviceId;
    console.log(getTime(), `🎯 Active device set to: ${deviceId}`);
  }

  /**
   * 销毁控制器
   */
  static destroyController(deviceId) {
    const controller = this.controllers.get(deviceId);
    if (controller) {
      controller.destroy();
      this.controllers.delete(deviceId);
      console.log(
        getTime(),
        `🏭 [Factory] Destroyed MouseController for ${deviceId}`
      );
    }
  }

  /**
   * 销毁所有控制器
   */
  static destroyAll() {
    this.controllers.forEach((controller, deviceId) => {
      controller.destroy();
    });
    this.controllers.clear();
    this.activeDevice = null;
    console.log(getTime(), `🏭 [Factory] Destroyed all MouseControllers`);
  }

  /**
   * 获取所有控制器
   */
  static getAllControllers() {
    return this.controllers;
  }
}

// 设置全局函数供device-manager.js使用
window.initMouseControl = function (deviceId) {
  console.log(
    getTime(),
    `🔍 [${deviceId}] initMouseControl called (new system)`
  );
  MouseControllerFactory.createController(deviceId);
};

window.setActiveDevice = function (deviceId) {
  MouseControllerFactory.setActiveDevice(deviceId);
};

// 暴露工厂到全局
window.mouseControllerFactory = MouseControllerFactory;

console.log(getTime(), "🖱️ New MouseController system initialized");
