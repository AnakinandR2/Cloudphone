/**
 * 页面生成器模块
 * 负责生成多窗口页面的HTML内容
 */

import {
  ENCODER_CONFIG,
  ROTATION_CONFIG,
  SCALE_CONFIG,
  SERVER_CONFIG,
  VIDEO_CONFIG
} from "../core/config.js";

/**
 * 页面生成器类
 */
export class PageGenerator {
  constructor() {
    // 浏览器环境中不需要文件系统访问
    console.log("📄 PageGenerator initialized");
  }

  /**
   * 生成网格样式
   * @param {number} windowCount 窗口数量
   * @returns {string} CSS网格样式
   */
  getGridStyles(windowCount) {
    if (windowCount <= 2) {
      return "grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(1, minmax(550px, auto));";
    } else if (windowCount <= 4) {
      return "grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(2, minmax(520px, auto));";
    } else if (windowCount <= 6) {
      return "grid-template-columns: repeat(3, 1fr); grid-template-rows: repeat(2, minmax(500px, auto));";
    } else if (windowCount <= 9) {
      return "grid-template-columns: repeat(3, 1fr); grid-template-rows: repeat(3, minmax(480px, auto));";
    } else {
      return "grid-template-columns: repeat(4, 1fr); grid-template-rows: repeat(3, minmax(460px, auto));";
    }
  }

  /**
   * 生成响应式网格样式
   * @param {number} windowCount 窗口数量
   * @returns {string} 响应式CSS网格样式
   */
  getResponsiveGridStyles(windowCount) {
    if (windowCount <= 2) {
      return "grid-template-columns: repeat(2, 1fr);";
    } else if (windowCount <= 4) {
      return "grid-template-columns: repeat(2, 1fr);";
    } else {
      return "grid-template-columns: repeat(3, 1fr);";
    }
  }

  /**
   * 生成分辨率选项HTML
   * @returns {string} 分辨率选项HTML
   */
  generateResolutionOptions() {
    return (
      VIDEO_CONFIG.SUPPORTED_RESOLUTIONS.map(
        res =>
          `<option value="${res.width}x${res.height}">${res.label}</option>`
      ).join("\n                        ") +
      '\n                        <option value="custom">🔧 自定义</option>'
    );
  }

  /**
   * 生成编码器选项HTML
   * @param {string} type 选项类型 (bitrate, keyframe, quality, fps)
   * @param {number} defaultValue 默认值
   * @returns {string} 选项HTML
   */
  generateEncoderOptions(type, defaultValue) {
    let options = [];
    let labels = {};

    switch (type) {
      case "bitrate":
        options = ENCODER_CONFIG.BITRATE_OPTIONS;
        labels = value => `${value}MB/s`;
        break;
      case "keyframe":
        options = ENCODER_CONFIG.KEYFRAME_OPTIONS;
        labels = value => `${value}s间隔`;
        break;
      case "quality":
        options = ENCODER_CONFIG.QUALITY_OPTIONS;
        labels = value => `质量${value}%`;
        break;
      case "fps":
        options = ENCODER_CONFIG.FPS_OPTIONS;
        labels = value => `${value} FPS`;
        break;
    }

    return options
      .map(value => {
        const selected = value === defaultValue ? " selected" : "";
        const label = typeof labels === "function" ? labels(value) : `${value}`;
        return `<option value="${value}"${selected}>${label}</option>`;
      })
      .join("\n                        ");
  }

  /**
   * 生成旋转选项HTML
   * @returns {string} 旋转选项HTML
   */
  generateRotationOptions() {
    return ROTATION_CONFIG.OPTIONS.map(rotation => {
      const selected = rotation === ROTATION_CONFIG.DEFAULT ? " selected" : "";
      return `<option value="${rotation}"${selected}>${rotation}°</option>`;
    }).join("\n                        ");
  }

  /**
   * 生成缩放选项HTML
   * @returns {string} 缩放选项HTML
   */
  generateScaleOptions() {
    return SCALE_CONFIG.OPTIONS.map(scale => {
      const selected = scale === SCALE_CONFIG.DEFAULT ? " selected" : "";
      const percentage = (scale * 100).toFixed(0);
      return `<option value="${scale}"${selected}>${percentage}%</option>`;
    }).join("\n                        ");
  }

  /**
   * 生成单个设备窗口HTML
   * @param {number} deviceNum 设备编号
   * @returns {string} 设备窗口HTML
   */
  generateDeviceWindow(deviceNum) {
    return `
        <!-- 设备${deviceNum} -->
        <div class="device_window" id="device${deviceNum}" data-device="device${deviceNum}">
            <div class="device_header">
                <h3>📱 设备 ${deviceNum}</h3>
                <div class="device_controls">
                    <button onclick="connectDevice('device${deviceNum}')" class="device_btn">🔗 连接</button>
                    <button onclick="disconnectDevice('device${deviceNum}')" class="device_btn">❌ 断开</button>
                    <button id="muteBtn${deviceNum}" class="device_btn mute_btn">🔊 声音</button>
                    <select id="roomSelect${deviceNum}" onchange="changeRoom('device${deviceNum}')" class="room_select">
                        <option value="mobile_screen">默认房间</option>
                        <option value="mobile_screen_2">房间2</option>
                        <option value="mobile_screen_3">房间3</option>
                        <option value="mobile_screen_4">房间4</option>
                        <option value="mobile_screen_5">房间5</option>
                        <option value="mobile_screen_6">房间6</option>
                    </select>
                    <select id="resolutionSelect${deviceNum}" onchange="changeResolution('device${deviceNum}')" class="resolution_select">
                        ${this.generateResolutionOptions()}
                    </select>
                </div>
            </div>
            
            <div class="encoder_config_panel">
                <div class="encoder_config_title">🔧 编码器配置</div>
                <div class="encoder_controls">
                    <select id="bitrateSelect${deviceNum}" onchange="changeBitrate('device${deviceNum}')" class="bitrate_select" title="目标码率">
                        ${this.generateEncoderOptions(
                          "bitrate",
                          ENCODER_CONFIG.DEFAULT_BITRATE
                        )}
                    </select>
                    
                    <select id="keyFrameSelect${deviceNum}" onchange="changeKeyFrame('device${deviceNum}')" class="keyframe_select" title="关键帧间隔">
                        ${this.generateEncoderOptions(
                          "keyframe",
                          ENCODER_CONFIG.DEFAULT_KEYFRAME_INTERVAL
                        )}
                    </select>
                    
                    <select id="qualitySelect${deviceNum}" onchange="changeQuality('device${deviceNum}')" class="quality_select" title="编码质量">
                        ${this.generateEncoderOptions(
                          "quality",
                          ENCODER_CONFIG.DEFAULT_QUALITY
                        )}
                    </select>
                    
                    <select id="encoderFpsSelect${deviceNum}" onchange="changeEncoderFps('device${deviceNum}')" class="encoder_fps_select" title="目标帧率">
                        ${this.generateEncoderOptions(
                          "fps",
                          ENCODER_CONFIG.DEFAULT_ENCODER_FPS
                        )}
                    </select>
                    
                    <select id="rotationSelect${deviceNum}" onchange="changeRotation('device${deviceNum}')" class="rotation_select" title="画面旋转">
                        ${this.generateRotationOptions()}
                    </select>
                    
                    <select id="scaleSelect${deviceNum}" class="scale_select" title="视频缩放比例">
                        ${this.generateScaleOptions()}
                    </select>
                </div>
            </div>
            
            <div class="image_div">
                <video id="screen${deviceNum}" class="image_screen" autoplay playsinline></video>
                
                <!-- 帧率显示 -->
                <div class="fps_display">
                    <div class="fps_counter">
                        <span class="fps_label">📺 FPS:</span>
                        <span id="fpsValue${deviceNum}" class="fps_value">--</span>
                    </div>
                    <div class="fps_stats">
                        <span id="videoResolution${deviceNum}" class="video_resolution">--x--</span>
                    </div>
                    <div class="video_quality_stats">
                        <div class="quality_item">
                            <span class="quality_label">🚫 丢帧:</span>
                            <span id="droppedFrames${deviceNum}" class="quality_value">--</span>
                        </div>
                        <div class="quality_item">
                            <span class="quality_label">📡 码率:</span>
                            <span id="videoBitrate${deviceNum}" class="quality_value">--</span>
                        </div>
                    </div>
                </div>
                
                <!-- DataChannel状态指示器 -->
                <div class="datachannel_status">
                    <div class="status_indicator">
                        <span id="dataChannelIcon${deviceNum}" class="status_icon">🔴</span>
                        <span id="dataChannelStatus${deviceNum}" class="status_text">未连接</span>
                    </div>
                </div>
                
                <!-- Android虚拟按键 -->
                <div class="virtual_buttons">
                    <!-- 主要导航按键 -->
                    <div class="nav_buttons">
                        <button onclick="backButtonHandler('device${deviceNum}')" class="nav_btn" title="返回">
                            <img src="img/back.svg" alt="返回" class="btn_icon">
                        </button>
                        <button onclick="homeButtonHandler('device${deviceNum}')" class="nav_btn" title="主屏幕">
                            <img src="img/home.svg" alt="主屏幕" class="btn_icon">
                        </button>
                        <button onclick="recentButtonHandler('device${deviceNum}')" class="nav_btn" title="应用列表">
                            <img src="img/recent.svg" alt="应用列表" class="btn_icon">
                        </button>
                    </div>
                    
                    <!-- 系统控制按键 -->
                    <div class="system_buttons">
                        <button onclick="powerButtonHandler('device${deviceNum}')" class="system_btn power_btn" title="电源">
                            <img src="img/power.svg" alt="电源" class="btn_icon">
                        </button>
                        <button onclick="lockButtonHandler('device${deviceNum}')" class="system_btn" title="锁屏">
                            <img src="img/lock.svg" alt="锁屏" class="btn_icon">
                        </button>
                        <div class="volume_controls">
                            <button onclick="volumeUpHandler('device${deviceNum}')" class="volume_btn" title="音量+">🔊+</button>
                            <button onclick="volumeDownHandler('device${deviceNum}')" class="volume_btn" title="音量-">🔉-</button>
                        </div>
                    </div>
                    
                    <!-- 剪贴板控制按键 -->
                    <div class="clipboard_buttons">
                        <button onclick="getClipboardHandler('device${deviceNum}')" class="clipboard_btn" title="获取剪贴板">📋 获取</button>
                        <button onclick="setClipboardHandler('device${deviceNum}')" class="clipboard_btn" title="设置剪贴板">📝 设置</button>
                    </div>
                </div>
            </div>
        </div>`;
  }

  /**
   * 生成多个设备窗口的HTML
   * @param {number} windowCount 窗口数量
   * @returns {string} 设备窗口HTML
   */
  generateDeviceWindows(windowCount) {
    const validWindowCount = Math.min(
      Math.max(1, parseInt(windowCount) || 4),
      12
    );
    let deviceWindowsHtml = "";

    for (let i = 1; i <= validWindowCount; i++) {
      deviceWindowsHtml += this.generateDeviceWindow(i);
    }

    return deviceWindowsHtml;
  }

  /**
   * 生成新架构页面HTML
   * @param {number} windowCount 窗口数量
   * @param {string} cssContent CSS内容（可选，用于Node.js环境）
   * @returns {Promise<string>} 完整的HTML页面
   */
  async generateNewArchitecturePage(windowCount, cssContent = "") {
    const maxWindows = 12; // 最大支持12个窗口
    const validWindowCount = Math.min(
      Math.max(1, parseInt(windowCount) || 4),
      maxWindows
    );

    // 生成完整的HTML页面 - 使用新的模块化架构
    const html = `<!DOCTYPE html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
    <link rel="icon" type="image/png" sizes="192x192" href="img/favicon-192x192.png">
    <link rel="icon" type="image/png" sizes="32x32" href="img/favicon-32x32.png">
    <link rel="icon" type="image/png" sizes="96x96" href="img/favicon-96x96.png">
    <link rel="icon" type="image/png" sizes="16x16" href="img/favicon-16x16.png">
    <title>WebScreen - 多窗口投屏 (${validWindowCount}个窗口) - 新架构</title>
    
    <!-- WebRTC-Stats官方统计库 -->
    <script src="https://unpkg.com/webrtc-stats@4.15.0/dist/webrtc-stats.min.js"></script>
    
    <!-- 外部CSS文件 -->
    <link rel="stylesheet" href="css/main.css">
    
    <style>
        ${cssContent}
        
        /* 通知容器样式 */
        #notifications-container {
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 10000;
            pointer-events: none;
            max-width: 400px;
        }
        
        /* 应用容器样式 */
        #app-container {
            width: 100%;
            height: 100vh;
        }
        
        /* 自动化测试按钮样式 */
        .test_btn {
            background: #9C27B0 !important;
            color: white !important;
        }
        
        .test_btn:hover {
            background: #7B1FA2 !important;
        }
        

    </style>
</head>
<body>
    <!-- 通知容器 -->
    <div id="notifications-container"></div>
    
    <!-- 应用容器 -->
    <div id="app-container">
        <!-- 页面标题 -->
        <div class="page_header">
            <h1>📱 多设备投屏监控 - ${validWindowCount}个窗口</h1>
            <div class="global_controls">
                <button onclick="toggleAllVideoStats()" class="global_btn">🔧 统计开关</button>
                <button onclick="connectAllDevices()" class="global_btn">🔗 连接所有</button>
                <button onclick="disconnectAllDevices()" class="global_btn">❌ 断开所有</button>
                <button onclick="toggleAllMute()" id="muteAllBtn" class="global_btn mute_all_btn">🔊 全部声音</button>
                <button onclick="window._startAutomatedTest && window._startAutomatedTest()" id="automatedTestBtn" class="global_btn test_btn">🧪 自动化测试</button>
                <div class="window_count_controls">
                    <label for="windowCountInput">窗口数量:</label>
                    <input type="number" id="windowCountInput" min="1" max="12" value="${validWindowCount}" style="width: 60px; margin: 0 5px;">
                    <button onclick="changeWindowCount()" class="global_btn">🔄 更新</button>
                </div>
            </div>
        </div>

        <!-- 多窗口容器 -->
        <div class="multi_window_container">
            ${this.generateDeviceWindows(validWindowCount)}
        </div>
    </div>
    
    <!-- 必要的外部脚本 -->
    <script src="js/adapter-latest.js"></script>
    <script src="js/video-stats-enhanced.js"></script>
    <script src="js/automated-test.js"></script>
    
    <!-- 启动脚本 -->
    <script>
        // 设置窗口数量
        localStorage.setItem('windowCount', '${validWindowCount}');
        const TOTAL_DEVICES = ${validWindowCount};
        
        // 窗口数量变更函数
        function changeWindowCount() {
            const input = document.getElementById('windowCountInput');
            const count = parseInt(input.value) || 1;
            const validCount = Math.min(Math.max(1, count), 12);
            window.location.href = '/new?windows=' + validCount;
        }
        
        // 等待模块化系统就绪的标志
        window.moduleSystemReady = false;
        
        // 基本的全局函数（防止控制台错误，会被模块化系统覆盖）
        function toggleAllVideoStats() {
            if (window.moduleSystemReady && window.toggleAllVideoStats) {
                return window.toggleAllVideoStats();
            }
            console.log('🔧 Toggle all video stats (placeholder)');
        }
        
        function connectAllDevices() {
            if (window.moduleSystemReady && window.connectAllDevices) {
                return window.connectAllDevices();
            }
            console.log('🔗 Connect all devices (placeholder)');
        }
        
        function disconnectAllDevices() {
            if (window.moduleSystemReady && window.disconnectAllDevices) {
                return window.disconnectAllDevices();
            }
            console.log('❌ Disconnect all devices (placeholder)');
        }
        
        function toggleAllMute() {
            if (window.moduleSystemReady && window.toggleAllMute) {
                return window.toggleAllMute();
            }
            console.log('🔊 Toggle all mute (placeholder)');
        }
        
        // 设备操作函数（防止控制台错误，会被模块化系统覆盖）
        function connectDevice(deviceId) {
            if (window.moduleSystemReady && window.connectDevice) {
                return window.connectDevice(deviceId);
            }
            console.log('🔗 Connect device (placeholder):', deviceId);
        }
        
        function disconnectDevice(deviceId) {
            if (window.moduleSystemReady && window.disconnectDevice) {
                return window.disconnectDevice(deviceId);
            }
            console.log('❌ Disconnect device (placeholder):', deviceId);
        }
        
        function toggleMute(deviceId) {
            if (window.moduleSystemReady && window.toggleMute) {
                return window.toggleMute(deviceId);
            }
            console.log('🔊 Toggle mute (placeholder):', deviceId);
        }
        
        function changeRoom(deviceId) {
            if (window.moduleSystemReady && window.changeRoom) {
                return window.changeRoom(deviceId);
            }
            console.log('🏠 Change room (placeholder):', deviceId);
        }
        
        function changeResolution(deviceId) {
            if (window.moduleSystemReady && window.changeResolution) {
                return window.changeResolution(deviceId);
            }
            console.log('📺 Change resolution (placeholder):', deviceId);
        }
        
        function changeBitrate(deviceId) {
            if (window.moduleSystemReady && window.changeBitrate) {
                return window.changeBitrate(deviceId);
            }
            console.log('📡 Change bitrate (placeholder):', deviceId);
        }
        
        function changeKeyFrame(deviceId) {
            if (window.moduleSystemReady && window.changeKeyFrame) {
                return window.changeKeyFrame(deviceId);
            }
            console.log('🎞️ Change keyframe (placeholder):', deviceId);
        }
        
        function changeQuality(deviceId) {
            if (window.moduleSystemReady && window.changeQuality) {
                return window.changeQuality(deviceId);
            }
            console.log('🎨 Change quality (placeholder):', deviceId);
        }
        
        function changeEncoderFps(deviceId) {
            if (window.moduleSystemReady && window.changeEncoderFps) {
                return window.changeEncoderFps(deviceId);
            }
            console.log('🎬 Change encoder FPS (placeholder):', deviceId);
        }
        
        function changeRotation(deviceId) {
            if (window.moduleSystemReady && window.changeRotation) {
                return window.changeRotation(deviceId);
            }
            console.log('🔄 Change rotation (placeholder):', deviceId);
        }
        
        function changeVideoScale(deviceId, scale) {
            if (window.moduleSystemReady && window.changeVideoScale) {
                return window.changeVideoScale(deviceId, scale);
            }
            console.log('📏 Change video scale (placeholder):', deviceId, scale);
        }
        
        // 占位符函数（避免控制台错误）
        
        // 虚拟按键函数
        function backButtonHandler(deviceId) {
            if (window.moduleSystemReady && window.backButtonHandler) {
                return window.backButtonHandler(deviceId);
            }
            console.log('⬅️ Back button (placeholder):', deviceId);
        }
        
        function homeButtonHandler(deviceId) {
            if (window.moduleSystemReady && window.homeButtonHandler) {
                return window.homeButtonHandler(deviceId);
            }
            console.log('🏠 Home button (placeholder):', deviceId);
        }
        
        function recentButtonHandler(deviceId) {
            if (window.moduleSystemReady && window.recentButtonHandler) {
                return window.recentButtonHandler(deviceId);
            }
            console.log('📋 Recent button (placeholder):', deviceId);
        }
        
        function powerButtonHandler(deviceId) {
            if (window.moduleSystemReady && window.powerButtonHandler) {
                return window.powerButtonHandler(deviceId);
            }
            console.log('⚡ Power button (placeholder):', deviceId);
        }
        
        function lockButtonHandler(deviceId) {
            if (window.moduleSystemReady && window.lockButtonHandler) {
                return window.lockButtonHandler(deviceId);
            }
            console.log('🔒 Lock button (placeholder):', deviceId);
        }
        
        function volumeUpHandler(deviceId) {
            if (window.moduleSystemReady && window.volumeUpHandler) {
                return window.volumeUpHandler(deviceId);
            }
            console.log('🔊 Volume up (placeholder):', deviceId);
        }
        
        function volumeDownHandler(deviceId) {
            if (window.moduleSystemReady && window.volumeDownHandler) {
                return window.volumeDownHandler(deviceId);
            }
            console.log('🔉 Volume down (placeholder):', deviceId);
        }
        
        function getClipboardHandler(deviceId) {
            if (window.moduleSystemReady && window.getClipboardHandler) {
                return window.getClipboardHandler(deviceId);
            }
            console.log('📋 Get clipboard (placeholder):', deviceId);
        }
        
        function setClipboardHandler(deviceId) {
            if (window.moduleSystemReady && window.setClipboardHandler) {
                return window.setClipboardHandler(deviceId);
            }
            console.log('📝 Set clipboard (placeholder):', deviceId);
        }
        

        
        // 错误处理
        window.addEventListener('error', (e) => {
            console.error('💥 Global error:', e.error);
            if (window.WebScreenApp && window.WebScreenApp.showError) {
                window.WebScreenApp.showError('应用错误: ' + e.message);
            }
        });
        
        // 未处理的Promise错误
        window.addEventListener('unhandledrejection', (e) => {
            console.error('💥 Unhandled promise rejection:', e.reason);
            if (window.WebScreenApp && window.WebScreenApp.showError) {
                window.WebScreenApp.showError('Promise错误: ' + e.reason);
            }
        });
        
        // 键盘快捷键
        document.addEventListener('keydown', function(event) {
            if (event.ctrlKey && event.key === 'Enter') {
                changeWindowCount();
            }
        });
        
        // 调试函数：手动测试连接
        window.debugConnect = function() {
            console.log('🔍 Debug Connect Test:');
            console.log('  moduleSystemReady:', window.moduleSystemReady);
            console.log('  devices:', window.devices);
            console.log('  connectDevice type:', typeof window.connectDevice);
            console.log('  WebScreenApp:', window.WebScreenApp);
            
            if (window.devices && window.devices.device1) {
                console.log('📱 Device1 found, attempting direct connect...');
                try {
                    window.devices.device1.connect();
                } catch (error) {
                    console.error('❌ Direct connect failed:', error);
                }
            } else {
                console.warn('⚠️ No devices found');
            }
        };
        
        // 提示用户如何调试
        console.log('🐛 Debug tip: Use debugConnect() to test connection manually');
    </script>
    
    <!-- 新的模块化架构主入口 -->
    <script type="module" src="src/main.js"></script>
    
    <!-- 模块化系统初始化 -->
    <script type="module">
        console.log('📦 ES6 Module script started');
        
        try {
            console.log('📥 Importing initApp from main.js...');
            const { initApp } = await import('./src/main.js');
            console.log('✅ Import successful, initApp type:', typeof initApp);
            
            // 等待DOM加载完成后初始化应用
            if (document.readyState === 'loading') {
                console.log('⏳ DOM loading, waiting for DOMContentLoaded...');
                document.addEventListener('DOMContentLoaded', async () => {
                    try {
                        console.log('🔧 DOM loaded, initializing WebScreen App...');
                        const app = await initApp();
                        console.log('✅ WebScreen App initialized successfully');
                        console.log('🔍 Module system ready status:', window.moduleSystemReady);
                        console.log('🔍 Available global functions:', {
                            connectDevice: typeof window.connectDevice,
                            devices: typeof window.devices,
                            WebScreenApp: typeof window.WebScreenApp
                        });
                    } catch (error) {
                        console.error('❌ Failed to initialize WebScreen App:', error);
                        console.error('📋 Error stack:', error.stack);
                    }
                });
            } else {
                // DOM已经加载完成
                console.log('🔧 DOM ready, initializing WebScreen App immediately...');
                try {
                    const app = await initApp();
                    console.log('✅ WebScreen App initialized successfully');
                    console.log('🔍 Module system ready status:', window.moduleSystemReady);
                    console.log('🔍 Available global functions:', {
                        connectDevice: typeof window.connectDevice,
                        devices: typeof window.devices,
                        WebScreenApp: typeof window.WebScreenApp
                    });
                } catch (error) {
                    console.error('❌ Failed to initialize WebScreen App:', error);
                    console.error('📋 Error stack:', error.stack);
                }
            }
        } catch (importError) {
            console.error('❌ Failed to import main.js:', importError);
            console.error('📋 Import error stack:', importError.stack);
            console.warn('🔄 Falling back to basic functionality...');
        }
    </script>
    
    <script>
        // 开发调试信息
        console.log('🚀 WebScreen New Architecture');
        console.log('📊 Windows:', ${validWindowCount});
        console.log('🏗️ Architecture: Modular');
        console.log('📍 Server Mode: Express + ES Modules');
        
        // 兼容性检查 - 使用更可靠的方法
        function checkES6ModuleSupport() {
            try {
                // 检查是否支持ES6模块
                const script = document.createElement('script');
                return 'noModule' in script;
            } catch (e) {
                return false;
            }
        }
        
        // 检查浏览器兼容性
        if (!checkES6ModuleSupport()) {
            console.warn('⚠️ 浏览器可能不支持ES6模块');
            // 显示友好的错误提示而不是弹窗
            const errorDiv = document.createElement('div');
            errorDiv.innerHTML = '<div style="position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); background: #ff4444; color: white; padding: 20px; border-radius: 10px; font-family: Arial, sans-serif; z-index: 10000; text-align: center;"><h3>⚠️ 浏览器兼容性提示</h3><p>您的浏览器可能不支持ES6模块特性</p><p>建议使用以下现代浏览器:</p><ul style="text-align: left; margin: 10px 0;"><li>Chrome 61+</li><li>Firefox 60+</li><li>Safari 10.1+</li><li>Edge 16+</li></ul><button onclick="this.parentElement.remove()" style="background: white; color: #ff4444; border: none; padding: 10px 20px; border-radius: 5px; cursor: pointer; margin-top: 10px;">继续尝试</button></div>';
            document.body.appendChild(errorDiv);
        }
    </script>
</body>
</html>`;

    return html;
  }

  /**
   * 生成多窗口页面HTML
   * @param {number} windowCount 窗口数量
   * @returns {string} 完整的HTML页面
   */
  generateMultiWindowPage(windowCount) {
    const validWindowCount = Math.min(
      Math.max(1, parseInt(windowCount) || 4),
      12
    );

    // 生成设备窗口HTML
    let deviceWindowsHtml = "";
    for (let i = 1; i <= validWindowCount; i++) {
      deviceWindowsHtml += this.generateDeviceWindow(i);
    }

    // 生成完整的HTML页面
    return `<!DOCTYPE html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
    <link rel="icon" type="image/png" sizes="192x192" href="img/favicon-192x192.png">
    <link rel="icon" type="image/png" sizes="32x32" href="img/favicon-32x32.png">
    <link rel="icon" type="image/png" sizes="96x96" href="img/favicon-96x96.png">
    <link rel="icon" type="image/png" sizes="16x16" href="img/favicon-16x16.png">
    <title>WebScreen - 多窗口投屏 (${validWindowCount}个窗口)</title>
    
    <!-- WebRTC-Stats官方统计库 -->
    <script src="https://unpkg.com/webrtc-stats@4.15.0/dist/webrtc-stats.min.js"></script>
    
    <!-- 外部CSS文件 -->
    <link rel="stylesheet" href="css/main.css">
    
    <style>
        
        /* 动态窗口数量样式调整 */
        .multi_window_container {
            display: grid;
            gap: 15px;
            padding: 20px;
            ${this.getGridStyles(validWindowCount)}
        }
        
        .device_window {
            min-height: 400px;
        }
        
        /* 响应式调整 */
        @media (max-width: 1920px) {
            .multi_window_container {
                ${this.getResponsiveGridStyles(validWindowCount)}
            }
        }
        
        @media (max-width: 1200px) {
            .multi_window_container {
                grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
            }
        }
    </style>
</head>
<body>
    <!-- 页面标题 -->
    <div class="page_header">
        <h1>📱 多设备投屏监控 - ${validWindowCount}个窗口</h1>
        <div class="global_controls">
            <button onclick="toggleAllVideoStats()" class="global_btn">🔧 统计开关</button>
            <button onclick="connectAllDevices()" class="global_btn">🔗 连接所有</button>
            <button onclick="disconnectAllDevices()" class="global_btn">❌ 断开所有</button>
            <button onclick="toggleAllMute()" id="muteAllBtn" class="global_btn mute_all_btn">🔊 全部声音</button>
            <button onclick="startAutomatedTest()" id="automatedTestBtn" class="global_btn test_btn">🧪 自动化测试</button>
            <div class="window_count_controls">
                <label for="windowCountInput">窗口数量:</label>
                <input type="number" id="windowCountInput" min="1" max="${
                  SERVER_CONFIG.MAX_WINDOWS
                }" value="${validWindowCount}" style="width: 60px; margin: 0 5px;">
                <button onclick="changeWindowCount()" class="global_btn">🔄 更新</button>
            </div>
        </div>
    </div>

    <!-- 多窗口容器 -->
    <div class="multi_window_container">
        ${deviceWindowsHtml}
    </div>

    <script>
        // 设备总数配置
        const TOTAL_DEVICES = ${validWindowCount};
        
        // 窗口数量变更函数
        function changeWindowCount() {
            const input = document.getElementById('windowCountInput');
            const count = parseInt(input.value) || 4;
            const validCount = Math.min(Math.max(1, count), ${
              SERVER_CONFIG.MAX_WINDOWS
            });
            window.location.href = '/multi?windows=' + validCount;
        }
        
        // 键盘快捷键
        document.addEventListener('keydown', function(event) {
            if (event.ctrlKey && event.key === 'Enter') {
                changeWindowCount();
            }
        });
    </script>
    
    <!-- 外部JavaScript文件 -->
    <script src="js/adapter-latest.js"></script>
    <script src="js/video-stats-enhanced.js"></script>
    <script src="js/automated-test.js"></script>
</body>
</html>`;
  }
}

/**
 * 默认页面生成器实例
 */
export const pageGenerator = new PageGenerator();

/**
 * 生成多窗口页面HTML（函数式接口）
 * @param {number} windowCount 窗口数量
 * @returns {string} 完整的HTML页面
 */
export function generateMultiWindowPage(windowCount) {
  return pageGenerator.generateMultiWindowPage(windowCount);
}

/**
 * 生成单个设备窗口HTML（函数式接口）
 * @param {number} deviceNum 设备编号
 * @returns {string} 设备窗口HTML
 */
export function generateDeviceWindow(deviceNum) {
  return pageGenerator.generateDeviceWindow(deviceNum);
}

/**
 * 生成新架构页面HTML（函数式接口）
 * @param {number} windowCount 窗口数量
 * @returns {Promise<string>} 完整的HTML页面
 */
export async function generateNewArchitecturePage(windowCount) {
  return await pageGenerator.generateNewArchitecturePage(windowCount);
}
