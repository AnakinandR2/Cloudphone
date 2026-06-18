/**
 * 通知管理模块
 * 负责显示各种类型的通知消息
 */

/**
 * 通知类型枚举
 */
export const NotificationTypes = {
  SUCCESS: "success",
  ERROR: "error",
  WARNING: "warning",
  INFO: "info"
};

/**
 * 通知管理器
 * 处理通知的显示、排队和自动清理
 */
export class NotificationManager {
  constructor(options = {}) {
    this.options = {
      duration: 5000, // 默认显示时长（毫秒）
      maxNotifications: 5, // 最大通知数量
      position: "top-right", // 通知位置
      ...options
    };

    this.notifications = [];
    this.container = null;
    this.nextId = 1;

    this.init();
  }

  /**
   * 初始化通知管理器
   */
  init() {
    this.createContainer();
    this.setupStyles();
    console.log("📢 NotificationManager initialized");
  }

  /**
   * 创建通知容器
   */
  createContainer() {
    this.container = document.getElementById("notifications-container");

    if (!this.container) {
      this.container = document.createElement("div");
      this.container.id = "notifications-container";
      this.container.className = `notifications-container ${this.options.position}`;
      document.body.appendChild(this.container);
    }
  }

  /**
   * 设置通知样式
   */
  setupStyles() {
    const styleId = "notification-styles";

    if (!document.getElementById(styleId)) {
      const style = document.createElement("style");
      style.id = styleId;
      style.textContent = `
                .notifications-container {
                    position: fixed;
                    z-index: 999;
                    pointer-events: none;
                    max-width: 300px;
                }
                
                .notifications-container.top-right {
                    top: 10px;
                    right: 10px;
                }
                
                .notifications-container.top-left {
                    top: 20px;
                    left: 20px;
                }
                
                .notifications-container.bottom-right {
                    bottom: 20px;
                    right: 20px;
                }
                
                .notifications-container.bottom-left {
                    bottom: 20px;
                    left: 20px;
                }
                
                .notifications-container.top-center {
                    top: 20px;
                    left: 50%;
                    transform: translateX(-50%);
                }
                
                .notifications-container.bottom-center {
                    bottom: 20px;
                    left: 50%;
                    transform: translateX(-50%);
                }
                
                .notification {
                    pointer-events: auto;
                    margin-bottom: 10px;
                    padding: 12px 16px;
                    border-radius: 8px;
                    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
                    font-size: 14px;
                    line-height: 1.4;
                    word-wrap: break-word;
                    opacity: 0;
                    transform: translateX(100%);
                    transition: all 0.3s ease;
                    display: flex;
                    align-items: center;
                    gap: 8px;
                    min-width: 250px;
                    max-width: 400px;
                }
                
                .notification.show {
                    opacity: 1;
                    transform: translateX(0);
                }
                
                .notification.success {
                    background-color: #d4edda;
                    color: #155724;
                    border: 1px solid #c3e6cb;
                }
                
                .notification.error {
                    background-color: #f8d7da;
                    color: #721c24;
                    border: 1px solid #f5c6cb;
                }
                
                .notification.warning {
                    background-color: #fff3cd;
                    color: #856404;
                    border: 1px solid #ffeaa7;
                }
                
                .notification.info {
                    background-color: #d1ecf1;
                    color: #0c5460;
                    border: 1px solid #bee5eb;
                }
                
                .notification-icon {
                    flex-shrink: 0;
                    font-size: 16px;
                }
                
                .notification-content {
                    flex: 1;
                }
                
                .notification-title {
                    font-weight: 600;
                    margin-bottom: 2px;
                }
                
                .notification-message {
                    font-size: 13px;
                    opacity: 0.9;
                }
                
                .notification-close {
                    flex-shrink: 0;
                    background: none;
                    border: none;
                    font-size: 16px;
                    cursor: pointer;
                    opacity: 0.6;
                    transition: opacity 0.2s;
                    padding: 0;
                    margin: 0;
                    width: 20px;
                    height: 20px;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                }
                
                .notification-close:hover {
                    opacity: 1;
                }
                
                .notification-progress {
                    position: absolute;
                    bottom: 0;
                    left: 0;
                    height: 2px;
                    background-color: rgba(0, 0, 0, 0.2);
                    transition: width linear;
                }
                
                /* 暗色主题 */
                [data-theme="dark"] .notification.success {
                    background-color: #1e3a2e;
                    color: #90ee90;
                    border-color: #2d5a3d;
                }
                
                [data-theme="dark"] .notification.error {
                    background-color: #3d1e1e;
                    color: #ff6b6b;
                    border-color: #5d2d2d;
                }
                
                [data-theme="dark"] .notification.warning {
                    background-color: #3d3d1e;
                    color: #ffeb3b;
                    border-color: #5d5d2d;
                }
                
                [data-theme="dark"] .notification.info {
                    background-color: #1e2a3d;
                    color: #64b5f6;
                    border-color: #2d3a5d;
                }
                
                /* 响应式设计 */
                @media (max-width: 768px) {
                    .notifications-container {
                        max-width: calc(100vw - 40px);
                        left: 20px !important;
                        right: 20px !important;
                        transform: none !important;
                    }
                    
                    .notification {
                        min-width: auto;
                        max-width: none;
                    }
                }
            `;
      document.head.appendChild(style);
    }
  }

  /**
   * 显示通知
   * @param {string} type 通知类型
   * @param {string} message 消息内容
   * @param {string} title 标题（可选）
   * @param {Object} options 选项
   */
  show(type, message, title = null, options = {}) {
    const notification = {
      id: this.nextId++,
      type,
      message,
      title,
      options: { ...this.options, ...options },
      timestamp: Date.now()
    };

    // 限制通知数量
    if (this.notifications.length >= this.options.maxNotifications) {
      this.removeOldest();
    }

    this.notifications.push(notification);
    this.render(notification);

    // 自动移除
    if (notification.options.duration > 0) {
      setTimeout(() => {
        this.remove(notification.id);
      }, notification.options.duration);
    }

    return notification.id;
  }

  /**
   * 显示成功通知
   */
  showSuccess(message, title = null, options = {}) {
    return this.show(NotificationTypes.SUCCESS, message, title, options);
  }

  /**
   * 显示错误通知
   */
  showError(message, title = null, options = {}) {
    return this.show(NotificationTypes.ERROR, message, title, {
      duration: 8000,
      ...options
    });
  }

  /**
   * 显示警告通知
   */
  showWarning(message, title = null, options = {}) {
    return this.show(NotificationTypes.WARNING, message, title, options);
  }

  /**
   * 显示信息通知
   */
  showInfo(message, title = null, options = {}) {
    return this.show(NotificationTypes.INFO, message, title, options);
  }

  /**
   * 渲染通知
   */
  render(notification) {
    const element = document.createElement("div");
    element.className = `notification ${notification.type}`;
    element.setAttribute("data-notification-id", notification.id);

    // 图标
    const icon = this.getIcon(notification.type);

    // 内容
    const content = document.createElement("div");
    content.className = "notification-content";

    if (notification.title) {
      const titleElement = document.createElement("div");
      titleElement.className = "notification-title";
      titleElement.textContent = notification.title;
      content.appendChild(titleElement);
    }

    const messageElement = document.createElement("div");
    messageElement.className = "notification-message";
    messageElement.textContent = notification.message;
    content.appendChild(messageElement);

    // 关闭按钮
    const closeButton = document.createElement("button");
    closeButton.className = "notification-close";
    closeButton.innerHTML = "×";
    closeButton.onclick = () => this.remove(notification.id);

    // 进度条
    const progressBar = document.createElement("div");
    progressBar.className = "notification-progress";

    // 组装元素
    element.appendChild(document.createTextNode(icon));
    element.appendChild(content);
    element.appendChild(closeButton);
    element.appendChild(progressBar);

    // 添加到容器
    this.container.appendChild(element);

    // 触发显示动画
    setTimeout(() => {
      element.classList.add("show");
    }, 10);

    // 设置进度条动画
    if (notification.options.duration > 0) {
      progressBar.style.width = "100%";
      setTimeout(() => {
        progressBar.style.width = "0%";
        progressBar.style.transitionDuration = `${notification.options.duration}ms`;
      }, 50);
    }
  }

  /**
   * 获取通知图标
   */
  getIcon(type) {
    const icons = {
      success: "✅",
      error: "❌",
      warning: "⚠️",
      info: "ℹ️"
    };
    return icons[type] || "ℹ️";
  }

  /**
   * 移除通知
   */
  remove(id) {
    const element = this.container.querySelector(
      `[data-notification-id="${id}"]`
    );
    if (element) {
      element.classList.remove("show");
      setTimeout(() => {
        if (element.parentNode) {
          element.parentNode.removeChild(element);
        }
      }, 300);
    }

    this.notifications = this.notifications.filter(n => n.id !== id);
  }

  /**
   * 移除最旧的通知
   */
  removeOldest() {
    if (this.notifications.length > 0) {
      const oldest = this.notifications[0];
      this.remove(oldest.id);
    }
  }

  /**
   * 清空所有通知
   */
  clear() {
    this.notifications.forEach(notification => {
      this.remove(notification.id);
    });
    this.notifications = [];
  }

  /**
   * 获取所有通知
   */
  getNotifications() {
    return [...this.notifications];
  }

  /**
   * 获取指定类型的通知
   */
  getNotificationsByType(type) {
    return this.notifications.filter(n => n.type === type);
  }

  /**
   * 更新通知选项
   */
  updateOptions(options) {
    this.options = { ...this.options, ...options };
  }

  /**
   * 销毁通知管理器
   */
  destroy() {
    this.clear();

    if (this.container && this.container.parentNode) {
      this.container.parentNode.removeChild(this.container);
    }

    const styles = document.getElementById("notification-styles");
    if (styles && styles.parentNode) {
      styles.parentNode.removeChild(styles);
    }

    console.log("📢 NotificationManager destroyed");
  }
}

/**
 * 快速通知函数
 * 创建一个全局通知管理器实例
 */
let globalNotificationManager = null;

function getGlobalNotificationManager() {
  if (!globalNotificationManager) {
    globalNotificationManager = new NotificationManager();
  }
  return globalNotificationManager;
}

export function showNotification(type, message, title = null, options = {}) {
  return getGlobalNotificationManager().show(type, message, title, options);
}

export function showSuccess(message, title = null, options = {}) {
  return getGlobalNotificationManager().showSuccess(message, title, options);
}

export function showError(message, title = null, options = {}) {
  return getGlobalNotificationManager().showError(message, title, options);
}

export function showWarning(message, title = null, options = {}) {
  return getGlobalNotificationManager().showWarning(message, title, options);
}

export function showInfo(message, title = null, options = {}) {
  return getGlobalNotificationManager().showInfo(message, title, options);
}

export function clearNotifications() {
  getGlobalNotificationManager().clear();
}

/**
 * 控制台通知扩展
 * 为console对象添加通知方法
 */
export function extendConsole() {
  if (typeof console !== "undefined") {
    const originalLog = console.log;
    const originalError = console.error;
    const originalWarn = console.warn;
    const originalInfo = console.info;

    console.log = function (...args) {
      originalLog.apply(console, args);
      if (args.length > 0 && typeof args[0] === "string") {
        showInfo(args.join(" "));
      }
    };

    console.error = function (...args) {
      originalError.apply(console, args);
      if (args.length > 0 && typeof args[0] === "string") {
        showError(args.join(" "));
      }
    };

    console.warn = function (...args) {
      originalWarn.apply(console, args);
      if (args.length > 0 && typeof args[0] === "string") {
        showWarning(args.join(" "));
      }
    };

    console.info = function (...args) {
      originalInfo.apply(console, args);
      if (args.length > 0 && typeof args[0] === "string") {
        showInfo(args.join(" "));
      }
    };
  }
}
