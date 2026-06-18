const express = require("express");
const path = require("path");
const cors = require("cors");
const helmet = require("helmet");
const https = require("https");
const fs = require("fs");

const app = express();
const HTTP_PORT = process.env.HTTP_PORT || 3000;
const HTTPS_PORT = process.env.HTTPS_PORT || 3443;

// 安全中间件 - 同时支持HTTP和HTTPS
app.use(
  helmet({
    contentSecurityPolicy: false,
    crossOriginOpenerPolicy: false, // 禁用COOP策略以避免IP访问问题
    crossOriginResourcePolicy: false,
    hsts: false // 禁用HSTS，因为我们同时支持HTTP和HTTPS
  })
);

// CORS设置
app.use(
  cors({
    origin: true,
    credentials: true
  })
);

// HTTP服务 - 不重定向到HTTPS
app.use((req, res, next) => {
  // 移除HTTPS重定向逻辑，直接提供HTTP服务
  next();
});

// 静态文件服务
app.use(express.static(path.join(__dirname, "../public")));

// 动态生成多窗口页面的函数
function generateMultiWindowPage(windowCount) {
  const maxWindows = 12; // 最大支持12个窗口
  const validWindowCount = Math.min(
    Math.max(1, parseInt(windowCount) || 4),
    maxWindows
  );

  // 读取CSS和JS文件内容
  const cssPath = path.join(__dirname, "../public/css/main.css");
  const jsMousePath = path.join(__dirname, "../public/js/mouse-multi.js");
  const jsAdapterPath = path.join(__dirname, "../public/js/adapter-latest.js");
  const jsVideoStatsPath = path.join(
    __dirname,
    "../public/js/video-stats-enhanced.js"
  );

  let cssContent = "";
  let jsMouseContent = "";
  let jsAdapterContent = "";
  let jsVideoStatsContent = "";

  try {
    cssContent = fs.readFileSync(cssPath, "utf8");
    jsMouseContent = fs.readFileSync(jsMousePath, "utf8");
    jsAdapterContent = fs.readFileSync(jsAdapterPath, "utf8");
    jsVideoStatsContent = fs.readFileSync(jsVideoStatsPath, "utf8");
  } catch (err) {
    console.warn("⚠️  Failed to read static files:", err.message);
  }

  // 生成设备窗口HTML
  let deviceWindowsHtml = "";
  for (let i = 1; i <= validWindowCount; i++) {
    deviceWindowsHtml += generateDeviceWindow(i);
  }

  // 生成完整的HTML页面
  const html = `<!DOCTYPE html>
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
    
    <style>
        ${cssContent}
        
        /* 动态窗口数量样式调整 */
        .multi_window_container {
            display: grid;
            gap: 15px;
            padding: 20px;
            ${getGridStyles(validWindowCount)}
        }
        
        .device_window {
            min-height: 400px;
            /* 移除固定的max-height限制，让容器可以根据旋转动态调整 */
        }
        
        /* 视频旋转相关样式 */
        .image_div {
            overflow: visible; /* 改为visible，避免旋转时被裁剪 */
            display: flex;
            align-items: center;
            justify-content: center;
            position: relative;
            min-height: 300px;
            background: #000;
            transition: all 0.3s ease;
            /* 移除默认aspect-ratio，由JS动态设置 */
        }
        
        .image_screen {
            max-width: 100%;
            max-height: 100%;
            object-fit: contain;
            display: block;
        }
        
        /* 剪贴板按钮样式 */
        .clipboard_buttons {
            display: flex;
            flex-direction: column;
            gap: 8px;
            margin-top: 8px;
        }
        
        .clipboard_btn {
            background: linear-gradient(45deg, #FF9800, #FF6F00);
            color: white;
            border: none;
            border-radius: 8px;
            padding: 8px 12px;
            font-size: 12px;
            font-weight: bold;
            cursor: pointer;
            transition: all 0.3s ease;
            display: flex;
            align-items: center;
            justify-content: center;
            min-height: 20px;
            box-shadow: 0 2px 4px rgba(255, 152, 0, 0.3);
        }
        
        .clipboard_btn:hover {
            background: linear-gradient(45deg, #FFB74D, #FF8F00);
            transform: translateY(-1px);
            box-shadow: 0 4px 8px rgba(255, 152, 0, 0.4);
        }
        
        .clipboard_btn:active {
            transform: translateY(0);
            box-shadow: 0 1px 2px rgba(255, 152, 0, 0.3);
        }
        
        /* 通知样式和动画 */
        @keyframes slideIn {
            from {
                transform: translateX(100%);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }
        
        @keyframes slideOut {
            from {
                transform: translateX(0);
                opacity: 1;
            }
            to {
                transform: translateX(100%);
                opacity: 0;
            }
        }
        
        .notification {
            animation: slideIn 0.3s ease;
        }
        
        /* 编码器配置面板样式 */
        .encoder_config_panel {
            background: linear-gradient(45deg, #34495e, #2c3e50);
            color: white;
            padding: 10px 20px;
            border-bottom: 1px solid rgba(255,255,255,0.1);
        }
        
        .encoder_config_title {
            font-size: 13px;
            font-weight: 600;
            margin-bottom: 8px;
            color: #ecf0f1;
        }
        
        .encoder_controls {
            display: flex;
            gap: 8px;
            align-items: center;
            flex-wrap: wrap;
        }
        
        .bitrate_select {
            background: rgba(155, 89, 182, 0.2);
            color: white;
            border: 1px solid rgba(155, 89, 182, 0.4);
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            cursor: pointer;
            transition: all 0.2s ease;
        }
        
        .bitrate_select:hover {
            background: rgba(155, 89, 182, 0.3);
        }
        
        .bitrate_select option {
            background: #2c3e50;
            color: white;
        }
        
        .keyframe_select {
            background: rgba(52, 152, 219, 0.2);
            color: white;
            border: 1px solid rgba(52, 152, 219, 0.4);
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            cursor: pointer;
            transition: all 0.2s ease;
        }
        
        .keyframe_select:hover {
            background: rgba(52, 152, 219, 0.3);
        }
        
        .keyframe_select option {
            background: #2c3e50;
            color: white;
        }
        
        .quality_select {
            background: rgba(230, 126, 34, 0.2);
            color: white;
            border: 1px solid rgba(230, 126, 34, 0.4);
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            cursor: pointer;
            transition: all 0.2s ease;
        }
        
        .quality_select:hover {
            background: rgba(230, 126, 34, 0.3);
        }
        
        .quality_select option {
            background: #2c3e50;
            color: white;
        }
        
        .encoder_fps_select {
            background: rgba(46, 204, 113, 0.2);
            color: white;
            border: 1px solid rgba(46, 204, 113, 0.4);
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            cursor: pointer;
            transition: all 0.2s ease;
        }
        
        .encoder_fps_select:hover {
            background: rgba(46, 204, 113, 0.3);
        }
        
        .encoder_fps_select option {
            background: #2c3e50;
            color: white;
        }
        
        .rotation_select {
            background: rgba(231, 76, 60, 0.2);
            color: white;
            border: 1px solid rgba(231, 76, 60, 0.4);
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            cursor: pointer;
            transition: all 0.2s ease;
        }
        
        .rotation_select:hover {
            background: rgba(231, 76, 60, 0.3);
        }
        
        .rotation_select option {
            background: #2c3e50;
            color: white;
        }
        
        /* 响应式调整 */
        @media (max-width: 1920px) {
            .multi_window_container {
                ${getResponsiveGridStyles(validWindowCount)}
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
            <div class="window_count_controls">
                <label for="windowCountInput">窗口数量:</label>
                <input type="number" id="windowCountInput" min="1" max="${maxWindows}" value="${validWindowCount}" style="width: 60px; margin: 0 5px;">
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
            const validCount = Math.min(Math.max(1, count), ${maxWindows});
            window.location.href = '/multi?windows=' + validCount;
        }
        
        // 键盘快捷键
        document.addEventListener('keydown', function(event) {
            if (event.ctrlKey && event.key === 'Enter') {
                changeWindowCount();
            }
        });
        
        ${jsAdapterContent}
    </script>
    <script>
        ${jsVideoStatsContent}
    </script>
    <script>
        ${jsMouseContent}
    </script>
    
    <!-- 新的ES6模块架构 -->
    <script type="module" src="src/main.js"></script>
    <script type="module">
        // 初始化应用程序
        import { initApp } from './src/main.js';
        
        // 页面加载后初始化
        document.addEventListener('DOMContentLoaded', async () => {
            console.log('🚀 Starting WebScreen App...');
            await initApp();
            console.log('✅ WebScreen App initialized');
        });
    </script>
    
    <script>
        // 为了兼容性，保留一些老的全局函数，但重定向到新的模块化接口
        // 这些函数会在新的模块化系统加载后被重新定义
        
        // 多窗口WebRTC管理器
        const deviceManagers = {};
        
        // 设备管理器类
        class DeviceManager {
            constructor(deviceId) {
                this.deviceId = deviceId;
                this.pc = null;
                this.remoteStream = null;
                this.dataWebSocket = null;
                this.dataChannel = null;
                this.token = null;
                this.androidSessionId = null;
                this.roomId = 'vm:mobile_screen';  //TODO vm_id:containerid
                this.isConnected = false;
                this.turnServerUrl = null; // 将从信令消息中动态获取
                
                // 设置
                this.width = 1080;
                this.height = 1920;
                this.fps = 30;
                this.isMuted = false;
                
                // 编码器配置
                this.targetBitrate = 5000; // kbps
                this.keyFrameInterval = 10; // seconds
                this.qualityLevel = 80; // %
                this.targetEncoderFps = 30; // fps
                
                // 旋转配置
                this.rotation = 0; // 0, 90, 180, 270
                
                // 视频统计
                this.statsEnabled = true;
                this.statsInterval = null;
                this.lastBytesReceived = 0;
                this.lastTimestamp = 0;
                
                // 网络配置
                this.pcConfig = { iceServers: [] };
                
                // 绑定DOM元素
                this.bindElements();
                this.initSettings();
            }
            
            bindElements() {
                const num = this.deviceId.slice(-1);
                this.remoteVideo = document.querySelector(\`#screen\${num}\`);
                this.fpsValueElement = document.getElementById(\`fpsValue\${num}\`);
                this.videoResolutionElement = document.getElementById(\`videoResolution\${num}\`);
                this.droppedFramesElement = document.getElementById(\`droppedFrames\${num}\`);
                this.videoBitrateElement = document.getElementById(\`videoBitrate\${num}\`);
                this.dataChannelIconElement = document.getElementById(\`dataChannelIcon\${num}\`);
                this.dataChannelStatusElement = document.getElementById(\`dataChannelStatus\${num}\`);
                this.roomSelectElement = document.getElementById(\`roomSelect\${num}\`);
                this.resolutionSelectElement = document.getElementById(\`resolutionSelect\${num}\`);

                this.muteBtnElement = document.getElementById(\`muteBtn\${num}\`);
                
                // 编码器配置元素
                this.bitrateSelectElement = document.getElementById(\`bitrateSelect\${num}\`);
                this.keyFrameSelectElement = document.getElementById(\`keyFrameSelect\${num}\`);
                this.qualitySelectElement = document.getElementById(\`qualitySelect\${num}\`);
                this.encoderFpsSelectElement = document.getElementById(\`encoderFpsSelect\${num}\`);
                
                // 旋转控制元素
                this.rotationSelectElement = document.getElementById(\`rotationSelect\${num}\`);
            }
            
            initSettings() {
                this.initResolutionSettings();
                this.initFpsSettings();
                this.initMuteSettings();
                this.initEncoderSettings();
                this.initRotationSettings();
            }
            
            initResolutionSettings() {
                const saved = localStorage.getItem(\`resolution_\${this.deviceId}\`);
                if (saved && saved !== 'custom') {
                    const [width, height] = saved.split('x').map(Number);
                    this.width = width;
                    this.height = height;
                    if (this.resolutionSelectElement) {
                        this.resolutionSelectElement.value = saved;
                    }
                }
            }
            
            initFpsSettings() {
                // FPS现在由编码器FPS控制，从编码器FPS设置中读取
                const savedEncoderFps = localStorage.getItem(\`encoderFps_\${this.deviceId}\`);
                if (savedEncoderFps) {
                    this.fps = parseInt(savedEncoderFps);
                } else {
                    this.fps = this.targetEncoderFps; // 使用编码器FPS作为默认值
                }
            }
            
            initMuteSettings() {
                const saved = localStorage.getItem(\`muted_\${this.deviceId}\`);
                this.isMuted = saved === 'true';
                this.updateMuteButtonState();
            }
            
            initEncoderSettings() {
                // 初始化码率设置
                const savedBitrate = localStorage.getItem(\`bitrate_\${this.deviceId}\`);
                if (savedBitrate) {
                    this.targetBitrate = parseInt(savedBitrate);
                    if (this.bitrateSelectElement) {
                        this.bitrateSelectElement.value = savedBitrate;
                    }
                }
                
                // 初始化关键帧间隔设置
                const savedKeyFrame = localStorage.getItem(\`keyFrame_\${this.deviceId}\`);
                if (savedKeyFrame) {
                    this.keyFrameInterval = parseInt(savedKeyFrame);
                    if (this.keyFrameSelectElement) {
                        this.keyFrameSelectElement.value = savedKeyFrame;
                    }
                }
                
                // 初始化质量等级设置
                const savedQuality = localStorage.getItem(\`quality_\${this.deviceId}\`);
                if (savedQuality) {
                    this.qualityLevel = parseInt(savedQuality);
                    if (this.qualitySelectElement) {
                        this.qualitySelectElement.value = savedQuality;
                    }
                }
                
                // 初始化编码器FPS设置
                const savedEncoderFps = localStorage.getItem(\`encoderFps_\${this.deviceId}\`);
                if (savedEncoderFps) {
                    this.targetEncoderFps = parseInt(savedEncoderFps);
                    if (this.encoderFpsSelectElement) {
                        this.encoderFpsSelectElement.value = savedEncoderFps;
                    }
                }
            }
            
            initRotationSettings() {
                // 初始化旋转设置
                const savedRotation = localStorage.getItem(\`rotation_\${this.deviceId}\`);
                if (savedRotation) {
                    this.rotation = parseInt(savedRotation);
                    if (this.rotationSelectElement) {
                        this.rotationSelectElement.value = savedRotation;
                    }
                    this.applyRotation();
                }
            }
            
            // WebSocket连接
            connect() {
                if (this.isConnected) return;
                
                const wsUrl = 'ws://192.168.9.1:9551/ws';
                console.log(\`🔗 [\${this.deviceId}] Connecting to: \${wsUrl}\`);
                
                try {
                    this.dataWebSocket = new WebSocket(wsUrl);
                    this.dataWebSocket.onopen = (event) => this.onWsOpen(event);
                    this.dataWebSocket.onclose = (event) => this.onWsClose(event);
                    this.dataWebSocket.onerror = (error) => this.onWsError(error);
                    this.dataWebSocket.onmessage = (event) => this.onWsMessage(event);
                } catch (error) {
                    console.error(\`❌ [\${this.deviceId}] Connection failed:\`, error);
                }
            }
            
            disconnect() {
                if (this.dataWebSocket) {
                    this.sendMessage({ type: 'leave', roomId: this.roomId });
                    this.dataWebSocket.close();
                }
                this.destroyPeerConnection();
                this.isConnected = false;
                this.updateConnectionStatus(false, '未连接');
            }
            
            onWsOpen(event) {
                console.log(\`✅ [\${this.deviceId}] WebSocket opened\`);
                
                // 当质量为100%时，传递给app端99
                const appQualityLevel = this.qualityLevel === 100 ? 99 : this.qualityLevel;
                
                this.sendMessage({
                    type: 'join',
                    clientType: 'web',
                    roomId: this.roomId,
                    data: this.roomId,
                    meta: {
                        resolution: {
                            width: this.width,
                            height: this.height
                        },
                        encoderConfig: {
                            targetBitrate: this.targetBitrate * 1000, // 转换为bps
                            keyFrameInterval: this.keyFrameInterval,
                            qualityLevel: appQualityLevel, // 100%时传递99
                            targetFps: this.targetEncoderFps
                        }
                    }
                });
                this.isConnected = true;
                this.updateConnectionStatus(true, '已连接');
            }
            
            onWsClose(event) {
                console.log(\`🔌 [\${this.deviceId}] WebSocket closed\`);
                this.androidSessionId = null;
                this.destroyPeerConnection();
                this.isConnected = false;
                this.updateConnectionStatus(false, '连接断开');
            }
            
            onWsError(error) {
                console.error(\`❌ [\${this.deviceId}] WebSocket error:\`, error);
                this.updateConnectionStatus(false, '连接错误');
            }
            
            onWsMessage(event) {
                const message = JSON.parse(event.data);

                console.log(\`🔗 [\Receice   msg : \${event.data}\`);

                switch(message.type) {
                    case 'welcome':
                    case 'joined':
                        console.log(\`✅ [\${this.deviceId}] Joined room: \${this.roomId}\`);
                        if (message.from) this.realSessionId = message.from;
                        if (message.data) {
                             if (message.data.token) this.token = message.data.token;
                             if (message.data.server) this.turnServerUrl = message.data.server;
                        }
                        console.log(\`🔗 [\${this.deviceId}] TURN Server URL: \${this.turnServerUrl}\`);
                        this.initNetworkConfig();
                        break;
                        
                    case 'offer':
                        this.androidSessionId = message.from;
                        this.handleSdpMessage({ type: 'sdp', sdp: message.data });
                        break;
                        
                    case 'ice-candidate':
                        this.handleIceMessage({ type: 'ice', ice: message.data });
                        break;
                        
                    case 'remote-closed':
                        console.log(\`📱 [\${this.deviceId}] Remote streaming ended\`);
                        this.handleRemoteHangup();
                        break;
                        
                    case 'app-connection-failed':
                        console.log(\`❌ [\${this.deviceId}] App connection failed:\`, message.data);
                        this.handleAppConnectionFailed(message.data);
                        break;
                        
                    case 'user-left':
                        console.log(\`👋 [\${this.deviceId}] User left:\`, message);
                        this.handleUserLeft(message);
                        break;
                }
            }
            
            sendMessage(message) {
                if (this.dataWebSocket && this.dataWebSocket.readyState === WebSocket.OPEN) {
                    this.dataWebSocket.send(JSON.stringify(message));
                }
            }
            
            initNetworkConfig() {
                // 使用动态获取的TURN服务器URL，如果没有则使用默认值

                if (this.turnServerUrl) {
                    this.pcConfig.iceServers = [
                            {
                                urls: this.turnServerUrl,
                                username: this.token,
                                credential: this.roomId
                            }
                        ];
                    console.log(\`🔧 [\${this.deviceId}] Network config initialized with TURN: \${this.turnServerUrl}\`);

                } else {
                    console.log(\`🔧 [\${this.deviceId}] Network config initialized failed  turnServerUrl null \`);
                }

            }
            
            createPeerConnection() {
                try {
              // 添加ICE超时配置
                    this.pcConfig.iceCandidatePoolSize = 10;
                    this.pcConfig.iceConnectionReceivingTimeout = 60000;
                    this.pcConfig.iceCheckMinInterval = 2000;
                    this.pcConfig.iceBackupCandidatePairPingInterval = 25000;
                    this.pcConfig.continualGatheringPolicy = 'gather_continually';

                                    // 🔧 设置 Unified Plan SDP 语义（与 Android 端保持一致）
                    this.pcConfig.sdpSemantics = 'unified-plan';


                    this.pc = new RTCPeerConnection(this.pcConfig);
                    this.pc.onicecandidate = (event) => this.handleIceCandidate(event);
                    this.pc.onaddstream = (event) => this.handleRemoteStreamAdded(event);
                    this.pc.onremovestream = (event) => this.handleRemoteStreamRemoved(event);
                    
                    this.pc.ondatachannel = (event) => {
                        this.dataChannel = event.channel;
                        this.setupDataChannelListeners();
                    };
                    
                    this.pc.oniceconnectionstatechange = () => {
                        if (this.pc) {
                            this.handleConnectionStateChange();
                        }
                    };
                } catch (e) {
                    console.error(\`❌ [\${this.deviceId}] Failed to create PeerConnection:\`, e);
                }
            }
            
            handleSdpMessage(message) {
                if (message.sdp.type === 'offer') {
                    this.createPeerConnection();
                    this.pc.setRemoteDescription(new RTCSessionDescription(message.sdp));
                    this.doAnswer();
                }
            }
            
            doAnswer() {
                this.pc.createAnswer().then(
                    (sessionDescription) => this.setLocalAndSendMessage(sessionDescription),
                    (error) => console.error(\`❌ [\${this.deviceId}] Create answer error:\`, error)
                );
            }
            
            setLocalAndSendMessage(sessionDescription) {
                this.pc.setLocalDescription(sessionDescription);
                this.sendSdpMessage(sessionDescription);
            }
            
            sendSdpMessage(message) {
                if (!this.androidSessionId) return;
                
                this.sendMessage({
                    type: 'answer',
                    roomId: this.roomId,
                    to: this.androidSessionId,
                    data: message
                });
            }
            
            handleIceMessage(message) {
                if (message.ice.type === 'candidate') {
                    let candidate = new RTCIceCandidate({
                        sdpMLineIndex: message.ice.label,
                        candidate: message.ice.candidate
                    });
                    this.pc.addIceCandidate(candidate);
                }
            }
            
            handleIceCandidate(event) {
                if (event.candidate && this.androidSessionId) {
                    this.sendMessage({
                        type: 'ice-candidate',
                        roomId: this.roomId,
                        to: this.androidSessionId,
                        data: {
                            type: 'candidate',
                            label: event.candidate.sdpMLineIndex,
                            id: event.candidate.sdpMid,
                            candidate: event.candidate.candidate
                        }
                    });
                }
            }
            
            handleRemoteStreamAdded(event) {
                console.log(\`📺 [\${this.deviceId}] Remote stream added\`);
                this.remoteStream = event.stream;
                this.remoteVideo.srcObject = this.remoteStream;
                this.applyMuteState();
                this.applyRotation(); // 应用旋转设置
                this.tryAutoPlay();
                
                // 清理矩阵缓存以确保新视频流使用正确的变换
                if (window.multiDeviceMouseController && window.multiDeviceMouseController.matrixCache) {
                    window.multiDeviceMouseController.matrixCache.clear();
                    console.log(\`🔧 [\${this.deviceId}] 清理矩阵缓存以应用新视频流\`);
                }
                
                // 启动视频统计监测
                this.startVideoStats();
                
                // 初始化鼠标控制（视频流准备好后）
                this.initMouseControlWhenReady();
            }
            
            tryAutoPlay() {
                const playPromise = this.remoteVideo.play();
                if (playPromise !== undefined) {
                    playPromise.then(() => {
                        console.log(\`▶️ [\${this.deviceId}] Video started\`);
                    }).catch(error => {
                        console.warn(\`⚠️ [\${this.deviceId}] Autoplay failed:\`, error);
                    });
                }
            }
            
            initMouseControlWhenReady() {
                console.log(\`🔍 [\${this.deviceId}] initMouseControlWhenReady 被调用\`);
                
                // 等待视频元素准备好再初始化鼠标控制
                const checkVideoReady = () => {
                    console.log(\`🔍 [\${this.deviceId}] checkVideoReady 被调用，视频状态: \${this.remoteVideo ? this.remoteVideo.videoWidth + 'x' + this.remoteVideo.videoHeight : 'null'}\`);
                    
                    if (this.remoteVideo && 
                        this.remoteVideo.videoWidth > 0 && 
                        this.remoteVideo.videoHeight > 0) {
                        
                        console.log(\`🖱️ [\${this.deviceId}] Initializing mouse control - Video ready: \${this.remoteVideo.videoWidth}x\${this.remoteVideo.videoHeight}\`);
                        
                        // 视频尺寸准备好后，重新应用旋转以确保缩放计算正确
                        this.applyRotation();
                        
                        // 初始化鼠标控制
                        if (typeof initMouseControl === 'function') {
                            console.log(\`🔍 [\${this.deviceId}] 调用 initMouseControl 函数\`);
                            initMouseControl(this.deviceId);
                            console.log(\`✅ [\${this.deviceId}] initMouseControl 调用完成\`);
                        } else {
                            console.warn(\`⚠️ [\${this.deviceId}] initMouseControl function not found\`);
                        }
                    } else {
                        // 视频还没准备好，继续等待
                        console.log(\`⏳ [\${this.deviceId}] Waiting for video to be ready...\`);
                        setTimeout(checkVideoReady, 100);
                    }
                };
                
                // 立即检查一次，或者在元数据加载后检查
                if (this.remoteVideo.readyState >= 1) {
                    console.log(\`🔍 [\${this.deviceId}] 视频已准备好，立即检查\`);
                    checkVideoReady();
                } else {
                    console.log(\`🔍 [\${this.deviceId}] 视频未准备好，添加事件监听器\`);
                    this.remoteVideo.addEventListener('loadedmetadata', checkVideoReady, { once: true });
                    setTimeout(checkVideoReady, 1000); // 兜底检查
                }
            }
            
            handleRemoteStreamRemoved(event) {
                console.log(\`📺 [\${this.deviceId}] Remote stream removed\`);
                this.clearVideoDisplay();
            }
            
            handleRemoteHangup() {
                this.destroyPeerConnection();
                this.clearVideoDisplay();
            }
            
            handleAppConnectionFailed(data) {
                const errorMessage = data.message || 'App端连接失败';
                
                console.error(\`❌ [\${this.deviceId}] \${errorMessage}\`);
                
                // 显示错误通知
                showNotification(this.deviceId, \`❌ \${errorMessage}\`, 'error');
                
                // 清理连接状态
                this.destroyPeerConnection();
                this.clearVideoDisplay();
                
                // 断开WebSocket连接
                if (this.dataWebSocket) {
                    this.dataWebSocket.close();
                    this.dataWebSocket = null;
                }
                
                // 更新连接状态
                this.isConnected = false;
                this.updateConnectionStatus(false, 'App连接失败');
                
                // 连接失败后不提供重连，用户需手动点击连接按钮
            }
            
            handleUserLeft(message) {
                const clientType = message.clientType;
                const fromUser = message.from;
                const data = message.data || 'User left';
                
                console.log(\`👋 [\${this.deviceId}] User left - ClientType: \${clientType}, From: \${fromUser}\`);
                
                // 只有当App端离开时才需要停止推流
                if (clientType === 'app') {
                    console.log(\`📱 [\${this.deviceId}] App user left, stopping stream\`);
                    
                    // 显示App离开通知
                    showNotification(this.deviceId, '📱 App端已离开房间，推流已停止', 'info');
                    
                    // 执行和remote-closed相同的处理逻辑
                    this.handleRemoteHangup();
                    
                    // 关闭WebSocket连接（重要：确保完全断开）
                    if (this.dataWebSocket) {
                        this.dataWebSocket.close();
                        this.dataWebSocket = null;
                    }
                    
                    // 更新连接状态
                    this.isConnected = false;
                    this.updateConnectionStatus(false, 'App已离开');
                    
                    // 清理会话相关状态
                    this.androidSessionId = null;
                    this.realSessionId = null;
                    this.token = null;
                    this.turnServerUrl = null;
                } else {
                    // 其他用户离开，只记录日志
                    console.log(\`👤 [\${this.deviceId}] Web user left: \${fromUser}\`);
                    showNotification(this.deviceId, '👤 其他观看者已离开', 'info');
                }
            }
            
            destroyPeerConnection() {
                // 停止视频统计
                this.stopVideoStats();
                
                if (this.dataChannel) this.dataChannel = null;
                if (this.pc) {
                    this.pc.close();
                    this.pc = null;
                }
                this.clearVideoDisplay();
            }
            
            setupDataChannelListeners() {
                this.dataChannel.onopen = () => {
                    console.log(\`📡 [\${this.deviceId}] DataChannel opened\`);
                    this.updateDataChannelStatus(true, '控制就绪');
                    window[\`dataChannel_\${this.deviceId}\`] = this.dataChannel;
                };
                
                this.dataChannel.onclose = () => {
                    console.log(\`📡 [\${this.deviceId}] DataChannel closed\`);
                    this.updateDataChannelStatus(false, '控制断开');
                    window[\`dataChannel_\${this.deviceId}\`] = null;
                };
                
                this.dataChannel.onmessage = (event) => {
                    try {
                        const message = JSON.parse(event.data);
                        this.handleDataChannelMessage(message);
                    } catch (error) {
                        console.warn(\`⚠️ [\${this.deviceId}] Failed to parse DataChannel message:\`, error);
                    }
                };
            }
            
            handleDataChannelMessage(message) {
                console.log(\`📨 [\${this.deviceId}] DataChannel message received:\`, message);
                
                switch(message.type) {
                    case 'clipboard_content':
                        this.handleClipboardContent(message.content);
                        break;
                    case 'clipboard_set_result':
                        this.handleClipboardSetResult(message.success, message.error);
                        break;
                    default:
                        console.log(\`📨 [\${this.deviceId}] Unknown message type:\`, message.type);
                }
            }
            
            handleClipboardContent(content) {
                if (content && content.trim()) {
                    // 显示剪贴板内容
                    const displayContent = content.length > 100 ? content.substring(0, 100) + '...' : content;
                    showNotification(this.deviceId, \`📋 剪贴板内容: \${displayContent}\`, 'success');
                    
                    // 询问是否复制到本地剪贴板
                    setTimeout(() => {
                        if (confirm(\`获取到手机剪贴板内容:\\n\\n\${content}\\n\\n是否复制到本地剪贴板?\`)) {
                            this.copyToLocalClipboard(content);
                        }
                    }, 500);
                } else {
                    showNotification(this.deviceId, '📋 手机剪贴板为空', 'info');
                }
            }
            
            handleClipboardSetResult(success, error) {
                if (success) {
                    showNotification(this.deviceId, '✅ 剪贴板设置成功', 'success');
                } else {
                    showNotification(this.deviceId, \`❌ 剪贴板设置失败: \${error || '未知错误'}\`, 'error');
                }
            }
            
            copyToLocalClipboard(text) {
                if (navigator.clipboard && navigator.clipboard.writeText) {
                    navigator.clipboard.writeText(text).then(() => {
                        showNotification(this.deviceId, '📋 已复制到本地剪贴板', 'success');
                    }).catch(error => {
                        console.warn(\`⚠️ [\${this.deviceId}] Failed to copy to local clipboard:\`, error);
                        this.fallbackCopyToClipboard(text);
                    });
                } else {
                    this.fallbackCopyToClipboard(text);
                }
            }
            
            fallbackCopyToClipboard(text) {
                // 创建临时文本域
                const textArea = document.createElement('textarea');
                textArea.value = text;
                textArea.style.position = 'fixed';
                textArea.style.opacity = '0';
                document.body.appendChild(textArea);
                textArea.select();
                
                try {
                    document.execCommand('copy');
                    showNotification(this.deviceId, '📋 已复制到本地剪贴板', 'success');
                } catch (error) {
                    console.warn(\`⚠️ [\${this.deviceId}] Fallback copy failed:\`, error);
                    showNotification(this.deviceId, '❌ 复制到本地剪贴板失败', 'error');
                } finally {
                    document.body.removeChild(textArea);
                }
            }
            
            handleConnectionStateChange() {
                switch(this.pc.iceConnectionState) {
                    case 'connected':
                    case 'completed':
                        this.updateDataChannelStatus(true, '控制就绪');
                        break;
                    case 'disconnected':
                    case 'failed':
                    case 'closed':
                        this.updateDataChannelStatus(false, '控制断开');
                        this.clearVideoDisplay();
                        break;
                }
            }
            
            // 视频统计相关方法
            startVideoStats() {
                if (!this.pc || !this.statsEnabled) return;
                
                console.log(\`📊 [\${this.deviceId}] Starting video stats monitoring with WebRTC-Stats\`);
                this.stopVideoStats(); // 确保清理之前的定时器
                
                // 使用WebRTC-Stats库初始化统计
                try {
                    this.webrtcStats = new WebRTCStats({
                        getStatsInterval: 1000, // 每秒更新一次
                        rawStats: false,
                        statsObject: true,
                        filteredStats: false,
                        remote: false
                    });
                    
                    // 添加peer connection
                    this.webrtcStats.addPeer(this.pc, this.deviceId);
                    
                    // 监听统计事件
                    this.webrtcStats.on('stats', (ev) => {
                        this.handleWebRTCStats(ev);
                    });
                    
                    console.log(\`📊 [\${this.deviceId}] WebRTC-Stats initialized successfully\`);
                } catch (error) {
                    console.error(\`❌ [\${this.deviceId}] Failed to initialize WebRTC-Stats:\`, error);
                    // 回退到原有方法
                    this.fallbackToNativeStats();
                }
            }
            
            stopVideoStats() {
                if (this.webrtcStats) {
                    try {
                        this.webrtcStats.removePeer(this.deviceId);
                        this.webrtcStats = null;
                        console.log(\`📊 [\${this.deviceId}] WebRTC-Stats monitoring stopped\`);
                    } catch (error) {
                        console.warn(\`⚠️ [\${this.deviceId}] Error stopping WebRTC-Stats:\`, error);
                    }
                }
                
                if (this.statsInterval) {
                    clearInterval(this.statsInterval);
                    this.statsInterval = null;
                }
                
                // 重置显示
                this.resetStatsDisplay();
            }
            
            handleWebRTCStats(event) {
                if (!event || !event.data || event.peerId !== this.deviceId) return;
                
                try {
                    const stats = event.data;
                    
                    // 提取视频接收统计
                    const videoReceiver = stats.video && stats.video.inbound && stats.video.inbound[0];
                    if (videoReceiver) {
                        const fps = videoReceiver.framesPerSecond || 0;
                        const resolution = \`\${videoReceiver.frameWidth || '--'}x\${videoReceiver.frameHeight || '--'}\`;
                        const droppedFrames = videoReceiver.framesDropped || 0;
                        const decodedFrames = videoReceiver.framesDecoded || 0;
                        const bitrate = Math.round((videoReceiver.bitrate || 0) / 1000); // 转换为kbps
                        const dropRate = decodedFrames > 0 ? ((droppedFrames / decodedFrames) * 100).toFixed(1) : '0.0';
                        
                        // 更新UI显示
                        this.updateStatsDisplay({
                            fps: Math.round(fps),
                            resolution: resolution,
                            droppedFrames: droppedFrames,
                            dropRate: dropRate,
                            bitrate: bitrate
                        });
                    }
                } catch (error) {
                    console.warn(\`⚠️ [\${this.deviceId}] Error processing WebRTC-Stats data:\`, error);
                }
            }
            
            // 回退到原生WebRTC统计方法
            fallbackToNativeStats() {
                console.log(\`📊 [\${this.deviceId}] Falling back to native WebRTC stats\`);
                this.statsInterval = setInterval(() => {
                    this.updateVideoStats();
                }, 1000);
            }
            
            async updateVideoStats() {
                if (!this.pc || this.pc.iceConnectionState !== 'connected') return;
                
                try {
                    const stats = await this.pc.getStats();
                    let videoStats = {};
                    let connectionStats = {};
                    
                    stats.forEach(report => {
                        if (report.type === 'inbound-rtp' && report.mediaType === 'video') {
                            videoStats = report;
                        } else if (report.type === 'candidate-pair' && report.state === 'succeeded') {
                            connectionStats = report;
                        }
                    });
                    
                    this.processVideoStats(videoStats, connectionStats);
                } catch (error) {
                    console.warn(\`⚠️ [\${this.deviceId}] Failed to get video stats:\`, error);
                }
            }
            
            processVideoStats(videoStats, connectionStats) {
                const currentTime = Date.now();
                const timeDiff = currentTime - this.lastTimestamp;
                
                if (timeDiff < 500) return; // 避免更新过于频繁
                
                // 计算FPS
                const fps = Math.round(videoStats.framesPerSecond || 0);
                
                // 计算码率
                let bitrate = 0;
                if (this.lastBytesReceived && this.lastTimestamp) {
                    const bytesDiff = (videoStats.bytesReceived || 0) - this.lastBytesReceived;
                    const timeDiffSec = timeDiff / 1000;
                    bitrate = Math.round((bytesDiff * 8) / (timeDiffSec * 1000)); // kbps
                }
                
                // 更新记录
                this.lastBytesReceived = videoStats.bytesReceived || 0;
                this.lastTimestamp = currentTime;
                
                // 获取其他统计信息
                const resolution = \`\${videoStats.frameWidth || '--'}x\${videoStats.frameHeight || '--'}\`;
                const droppedFrames = videoStats.framesDropped || 0;
                const decodedFrames = videoStats.framesDecoded || 0;
                const dropRate = decodedFrames > 0 ? ((droppedFrames / decodedFrames) * 100).toFixed(1) : '0.0';
                
                // 更新UI显示
                this.updateStatsDisplay({
                    fps: fps,
                    resolution: resolution,
                    droppedFrames: droppedFrames,
                    dropRate: dropRate,
                    bitrate: bitrate
                });
            }
            
            updateStatsDisplay(stats) {
                // 更新FPS
                if (this.fpsValueElement) {
                    this.fpsValueElement.textContent = stats.fps || '--';
                    this.fpsValueElement.style.color = this.getFpsColor(stats.fps);
                }
                
                // 更新分辨率
                if (this.videoResolutionElement) {
                    this.videoResolutionElement.textContent = stats.resolution || '--x--';
                }
                
                // 更新丢帧信息
                if (this.droppedFramesElement) {
                    this.droppedFramesElement.textContent = \`\${stats.droppedFrames} (\${stats.dropRate}%)\`;
                    this.droppedFramesElement.style.color = this.getDropRateColor(parseFloat(stats.dropRate));
                }
                
                // 更新码率
                if (this.videoBitrateElement) {
                    this.videoBitrateElement.textContent = stats.bitrate > 0 ? \`\${stats.bitrate} kbps\` : '--';
                }
            }
            
            resetStatsDisplay() {
                const elements = [
                    { element: this.fpsValueElement, text: '--' },
                    { element: this.videoResolutionElement, text: '--x--' },
                    { element: this.droppedFramesElement, text: '--' },
                    { element: this.videoBitrateElement, text: '--' }
                ];
                
                elements.forEach(({ element, text }) => {
                    if (element) {
                        element.textContent = text;
                        element.style.color = '#4CAF50';
                    }
                });
            }
            
            getFpsColor(fps) {
                if (fps >= 25) return '#4CAF50';      // 绿色 - 优秀
                if (fps >= 20) return '#8BC34A';      // 浅绿 - 良好
                if (fps >= 15) return '#FF9800';      // 橙色 - 一般
                if (fps >= 10) return '#FF5722';      // 深橙 - 较差
                return '#F44336';                     // 红色 - 很差
            }
            
            getDropRateColor(dropRate) {
                if (dropRate < 1) return '#4CAF50';   // 绿色 - 优秀
                if (dropRate < 3) return '#8BC34A';   // 浅绿 - 良好
                if (dropRate < 5) return '#FF9800';   // 橙色 - 一般
                if (dropRate < 10) return '#FF5722';  // 深橙 - 较差
                return '#F44336';                     // 红色 - 很差
            }
            
            toggleVideoStats() {
                this.statsEnabled = !this.statsEnabled;
                
                if (this.statsEnabled && this.remoteStream) {
                    this.startVideoStats();
                    console.log(\`📊 [\${this.deviceId}] Video stats enabled\`);
                } else {
                    this.stopVideoStats();
                    console.log(\`📊 [\${this.deviceId}] Video stats disabled\`);
                }
                
                return this.statsEnabled;
            }
            
            clearVideoDisplay() {
                // 停止视频统计
                this.stopVideoStats();
                
                if (this.remoteVideo) {
                    this.remoteVideo.pause();
                    if (this.remoteVideo.srcObject) {
                        const tracks = this.remoteVideo.srcObject.getTracks();
                        tracks.forEach(track => track.stop());
                        this.remoteVideo.srcObject = null;
                    }
                    this.remoteVideo.style.backgroundColor = '#000';
                }
                if (this.remoteStream) {
                    const tracks = this.remoteStream.getTracks();
                    tracks.forEach(track => track.stop());
                    this.remoteStream = null;
                }
                
                // 清理鼠标控制
                if (typeof cleanupMouseControl === 'function') {
                    cleanupMouseControl(this.deviceId);
                    console.log(\`🗑️ [\${this.deviceId}] Mouse control cleaned up\`);
                }
                
                this.updateDataChannelStatus(false, '未连接');
            }
            
            toggleMute() {
                this.isMuted = !this.isMuted;
                localStorage.setItem(\`muted_\${this.deviceId}\`, this.isMuted.toString());
                this.applyMuteState();
                this.updateMuteButtonState();
                
                console.log(\`🔊 [\${this.deviceId}] Mute toggled: \${this.isMuted ? '静音' : '有声'}\`);
            }
            
            applyMuteState() {
                if (this.remoteVideo) {
                    this.remoteVideo.muted = this.isMuted;
                    
                    // 检查音频轨道
                    if (this.remoteStream) {
                        const audioTracks = this.remoteStream.getAudioTracks();
                        console.log(\`🎵 [\${this.deviceId}] Audio tracks count: \${audioTracks.length}\`);
                        
                        if (audioTracks.length === 0) {
                            console.warn(\`⚠️ [\${this.deviceId}] No audio tracks in stream, mute button may not work\`);
                        } else {
                            audioTracks.forEach((track, index) => {
                                console.log(\`🎵 [\${this.deviceId}] Audio track \${index}: enabled=\${track.enabled}, muted=\${track.muted}\`);
                            });
                        }
                    }
                    
                    console.log(\`🔊 [\${this.deviceId}] Video element muted: \${this.remoteVideo.muted}\`);
                }
            }
            
            updateMuteButtonState() {
                if (this.muteBtnElement) {
                    this.muteBtnElement.innerHTML = this.isMuted ? '🔇 静音' : '🔊 声音';
                    this.muteBtnElement.classList.toggle('muted', this.isMuted);
                }
            }
            
            updateConnectionStatus(isConnected, statusText = '') {
                const statusElement = document.querySelector(\`#\${this.deviceId} .device_header h3\`);
                if (statusElement) {
                    const deviceNum = this.deviceId.slice(-1);
                    statusElement.textContent = \`📱 设备 \${deviceNum} \${isConnected ? '🟢' : '🔴'}\`;
                }
            }
            
            updateDataChannelStatus(isConnected, statusText = '') {
                if (this.dataChannelIconElement && this.dataChannelStatusElement) {
                    this.dataChannelIconElement.textContent = isConnected ? '🟢' : '🔴';
                    this.dataChannelStatusElement.textContent = statusText || (isConnected ? '控制就绪' : '控制断开');
                    this.dataChannelStatusElement.style.color = isConnected ? '#4CAF50' : '#F44336';
                }
            }
            
            setResolution(width, height) {
                this.width = width;
                this.height = height;
                localStorage.setItem(\`resolution_\${this.deviceId}\`, \`\${width}x\${height}\`);
                if (this.resolutionSelectElement) {
                    const optionValue = \`\${width}x\${height}\`;
                    this.resolutionSelectElement.value = optionValue;
                }
                
                // 清理矩阵缓存以确保使用新的分辨率变换
                if (window.multiDeviceMouseController && window.multiDeviceMouseController.matrixCache) {
                    window.multiDeviceMouseController.matrixCache.clear();
                    console.log(\`🔧 [\${this.deviceId}] 清理矩阵缓存以应用新分辨率\`);
                }
                
                if (this.isConnected) {
                    this.disconnect();
                    setTimeout(() => this.connect(), 1000);
                }
            }
            

            
            setBitrate(bitrate) {
                this.targetBitrate = bitrate;
                localStorage.setItem(\`bitrate_\${this.deviceId}\`, bitrate.toString());
                if (this.bitrateSelectElement) {
                    this.bitrateSelectElement.value = bitrate.toString();
                }
                if (this.isConnected) {
                    this.disconnect();
                    setTimeout(() => this.connect(), 1000);
                }
            }
            
            setKeyFrameInterval(interval) {
                this.keyFrameInterval = interval;
                localStorage.setItem(\`keyFrame_\${this.deviceId}\`, interval.toString());
                if (this.keyFrameSelectElement) {
                    this.keyFrameSelectElement.value = interval.toString();
                }
                if (this.isConnected) {
                    this.disconnect();
                    setTimeout(() => this.connect(), 1000);
                }
            }
            
            setQualityLevel(quality) {
                this.qualityLevel = quality;
                localStorage.setItem(\`quality_\${this.deviceId}\`, quality.toString());
                if (this.qualitySelectElement) {
                    this.qualitySelectElement.value = quality.toString();
                }
                if (this.isConnected) {
                    this.disconnect();
                    setTimeout(() => this.connect(), 1000);
                }
            }
            
            setEncoderFps(fps) {
                this.targetEncoderFps = fps;
                this.fps = fps; // 同时更新采集FPS，保持一致
                localStorage.setItem(\`encoderFps_\${this.deviceId}\`, fps.toString());
                if (this.encoderFpsSelectElement) {
                    this.encoderFpsSelectElement.value = fps.toString();
                }
                if (this.isConnected) {
                    this.disconnect();
                    setTimeout(() => this.connect(), 1000);
                }
            }
            
            setRotation(rotation) {
                this.rotation = rotation;
                localStorage.setItem(\`rotation_\${this.deviceId}\`, rotation.toString());
                if (this.rotationSelectElement) {
                    this.rotationSelectElement.value = rotation.toString();
                }
                this.applyRotation();
                
                // 清理矩阵缓存以确保使用新的旋转变换
                if (window.multiDeviceMouseController && window.multiDeviceMouseController.matrixCache) {
                    window.multiDeviceMouseController.matrixCache.clear();
                    console.log(\`🔧 [\${this.deviceId}] 清理矩阵缓存以应用新旋转\`);
                }
                
                // 延迟重新应用，确保容器尺寸计算正确
                setTimeout(() => {
                    this.applyRotation();
                }, 100);
                
                console.log(\`🔄 [\${this.deviceId}] Rotation set to: \${rotation}°\`);
            }
            
            applyRotation() {
                if (this.remoteVideo) {
                    console.log(\`🔄 [\${this.deviceId}] Applying rotation: \${this.rotation}°\`);
                    
                    const videoContainer = this.remoteVideo.parentElement;
                    let transform = \`rotate(\${this.rotation}deg)\`;
                    
                    // 直接设置容器的具体尺寸，避免aspect-ratio兼容性问题
                    if (videoContainer) {
                        // 重置所有相关样式
                        videoContainer.style.aspectRatio = '';
                        
                        // 获取视频的实际尺寸和当前显示状态
                        const videoWidth = this.remoteVideo.videoWidth || 0;
                        const videoHeight = this.remoteVideo.videoHeight || 0;
                        const isVideoReady = videoWidth > 0 && videoHeight > 0;
                        
                        // 计算容器尺寸
                        let containerWidth = '100%';
                        let containerHeight = '500px';
                        let containerMaxHeight = '600px';
                        
                                                 if (isVideoReady) {
                             // 根据视频实际尺寸计算合适的容器尺寸
                             const videoAspectRatio = videoWidth / videoHeight;
                             
                             if (this.rotation === 90 || this.rotation === 270) {
                                 // 90°/270°旋转：需要为旋转后的视频提供足够空间
                                 // 720x1280旋转90度后，视觉上变成1280宽x720高的横向视频
                                 
                                 const maxContainerWidth = videoContainer.parentElement.clientWidth || 400;
                                 
                                 // 计算旋转后需要的容器尺寸
                                 // 旋转后视频的视觉宽度是原高度(1280)，视觉高度是原宽度(720)
                                 const rotatedVisualWidth = videoHeight;  // 1280
                                 const rotatedVisualHeight = videoWidth;  // 720
                                 
                                 // 根据容器宽度计算缩放后的高度
                                 const scale = maxContainerWidth * 0.9 / rotatedVisualWidth; // 0.9是为了给旋转留空间
                                 let calculatedHeight = rotatedVisualHeight * scale;
                                 
                                 // 确保有足够的空间容纳旋转后的视频
                                 calculatedHeight = Math.max(calculatedHeight, 400);
                                 calculatedHeight = Math.min(calculatedHeight, 800);
                                 
                                 containerHeight = \`\${calculatedHeight}px\`;
                                 containerMaxHeight = \`\${calculatedHeight + 50}px\`;
                                 
                                 console.log(\`🔄 [\${this.deviceId}] 旋转90°/270°: 视频\${videoWidth}x\${videoHeight} -> 容器\${maxContainerWidth}x\${calculatedHeight} (缩放\${scale.toFixed(3)})\`);
                             } else {
                                 // 0°/180°旋转：保持原有比例
                                 const maxContainerWidth = videoContainer.parentElement.clientWidth || 400;
                                 const calculatedHeight = Math.min(maxContainerWidth / videoAspectRatio, 800);
                                 
                                 containerHeight = \`\${calculatedHeight}px\`;
                                 containerMaxHeight = \`\${calculatedHeight + 100}px\`;
                                 
                                 console.log(\`🔄 [\${this.deviceId}] 旋转0°/180°: 视频\${videoWidth}x\${videoHeight} -> 容器高度\${calculatedHeight}px\`);
                             }
                         } else {
                            // 视频尺寸未知时，使用默认值
                            if (this.rotation === 90 || this.rotation === 270) {
                                // 为横屏旋转预留更多高度
                                containerHeight = '600px';
                                containerMaxHeight = '700px';
                                console.log(\`🔄 [\${this.deviceId}] 使用默认横屏旋转尺寸\`);
                            } else {
                                // 竖屏默认值
                                containerHeight = '500px';
                                containerMaxHeight = '600px';
                                console.log(\`🔄 [\${this.deviceId}] 使用默认竖屏尺寸\`);
                            }
                        }
                        
                        // 应用计算出的容器尺寸
                        videoContainer.style.width = containerWidth;
                        videoContainer.style.height = containerHeight;
                        videoContainer.style.maxHeight = containerMaxHeight;
                        
                        // 确保容器样式
                        videoContainer.style.display = 'flex';
                        videoContainer.style.alignItems = 'center';
                        videoContainer.style.justifyContent = 'center';
                        videoContainer.style.overflow = 'visible'; // 改为visible避免旋转时被裁剪
                        videoContainer.style.background = '#000';
                        videoContainer.style.position = 'relative';
                        
                        // 对于旋转情况，确保容器有足够的空间
                        if (this.rotation === 90 || this.rotation === 270) {
                            videoContainer.style.minHeight = '450px';
                        }
                        
                        console.log(\`🔄 [\${this.deviceId}] Container size set to: \${containerWidth} x \${containerHeight}\`);
                    }
                    
                    // 应用旋转变换
                    this.remoteVideo.style.transform = transform;
                    this.remoteVideo.style.transformOrigin = 'center center';
                    this.remoteVideo.style.transition = 'transform 0.3s ease';
                    
                    // 设置视频元素的尺寸和适应方式
                    this.remoteVideo.style.visibility = 'visible';
                    this.remoteVideo.style.opacity = '1';
                    this.remoteVideo.style.display = 'block';
                    
                    // 获取用户设置的缩放比例
                    const userScale = this.getUserVideoScale();
                    
                    // 设置视频基本样式 - 让视频保持原始比例
                    if (this.rotation === 90 || this.rotation === 270) {
                        // 旋转时，视频应该保持自身尺寸，然后旋转
                        this.remoteVideo.style.width = 'auto';
                        this.remoteVideo.style.height = 'auto';
                        // 应用用户缩放设置到最大尺寸限制
                        this.remoteVideo.style.maxWidth = \`\${90 * userScale}%\`;
                        this.remoteVideo.style.maxHeight = \`\${90 * userScale}%\`;
                        this.remoteVideo.style.objectFit = 'contain';
                        
                        console.log(\`🔄 [\${this.deviceId}] 旋转模式：设置video为auto尺寸，max \${90 * userScale}% (用户缩放: \${userScale})\`);
                    } else {
                        // 正常显示时使用标准设置
                        this.remoteVideo.style.width = 'auto';
                        this.remoteVideo.style.height = 'auto';
                        // 应用用户缩放设置到最大尺寸限制
                        this.remoteVideo.style.maxWidth = \`\${100 * userScale}%\`;
                        this.remoteVideo.style.maxHeight = \`\${100 * userScale}%\`;
                        this.remoteVideo.style.objectFit = 'contain';
                        
                        console.log(\`🔄 [\${this.deviceId}] 正常模式：设置video为auto尺寸，max \${100 * userScale}% (用户缩放: \${userScale})\`);
                    }
                    
                    console.log(\`🔄 [\${this.deviceId}] Transform applied: \${transform}\`);
                    console.log(\`🔄 [\${this.deviceId}] Video dimensions: \${this.remoteVideo.videoWidth}x\${this.remoteVideo.videoHeight}\`);
                }
            }
            
            // 获取用户设置的视频缩放比例
            getUserVideoScale() {
                try {
                    const savedScale = localStorage.getItem(\`videoScale_\${this.deviceId}\`);
                    const scale = savedScale ? parseFloat(savedScale) : 1.0;
                    return Math.min(Math.max(scale, 0.1), 2.0); // 限制范围在0.1-2.0之间
                } catch (error) {
                    console.warn(\`⚠️ [\${this.deviceId}] 获取用户缩放设置失败:\`, error);
                    return 1.0;
                }
            }
            
            // 应用用户的视频缩放设置
            applyUserVideoScale() {
                console.log(\`🎯 [\${this.deviceId}] 应用用户缩放设置...\`);
                
                // 延迟执行，确保旋转变换已经完成
                setTimeout(() => {
                    // 尝试获取鼠标控制器
                    if (window.multiDeviceMouseController) {
                        const controller = window.multiDeviceMouseController.getController(this.deviceId);
                        if (controller && this.remoteVideo) {
                            const videoWidth = this.remoteVideo.videoWidth;
                            const videoHeight = this.remoteVideo.videoHeight;
                            
                            if (videoWidth > 0 && videoHeight > 0) {
                                console.log(\`🎯 [\${this.deviceId}] 调用控制器重新设置视频尺寸: \${videoWidth}x\${videoHeight}\`);
                                controller.resizeVideoElement(videoWidth, videoHeight);
                            } else {
                                console.warn(\`⚠️ [\${this.deviceId}] 视频尺寸无效，跳过缩放应用\`);
                            }
                        } else {
                            console.warn(\`⚠️ [\${this.deviceId}] 鼠标控制器或视频元素未找到\`);
                        }
                    } else {
                        console.warn(\`⚠️ [\${this.deviceId}] multiDeviceMouseController未初始化\`);
                    }
                }, 100);
            }
            
            changeRoom(newRoomId) {
                const wasConnected = this.isConnected;
                if (wasConnected) this.disconnect();
                this.roomId = newRoomId;
                if (wasConnected) {
                    setTimeout(() => this.connect(), 1000);
                }
            }
        }
        
        // 全局设备管理
        const devices = {};
        
        // 初始化设备管理器
        function initializeDevices() {
            console.log('🚀 Multi-device WebRTC manager initializing...');
            const deviceWindows = document.querySelectorAll('.device_window');
            
            deviceWindows.forEach((window, index) => {
                const deviceId = window.id;
                if (deviceId) {
                    devices[deviceId] = new DeviceManager(deviceId);
                    deviceManagers[deviceId] = devices[deviceId];
                    console.log(\`✅ Created device manager for: \${deviceId}\`);
                }
            });
            
            console.log(\`✅ Multi-device manager ready with \${Object.keys(devices).length} devices\`);
            
            // 设置全局变量供鼠标控制器使用
            window.devices = devices;
        }
        
        // 全局控制函数
        function connectAllDevices() {
            Object.values(devices).forEach(device => {
                if (device && !device.isConnected) {
                    device.connect();
                }
            });
        }
        
        function disconnectAllDevices() {
            Object.values(devices).forEach(device => {
                if (device) device.disconnect();
            });
        }
        
        function toggleAllVideoStats() {
            let allEnabled = true;
            let hasAnyDevice = false;
            
            // 检查是否所有设备都启用了统计
            Object.values(devices).forEach(device => {
                if (device) {
                    hasAnyDevice = true;
                    if (!device.statsEnabled) {
                        allEnabled = false;
                    }
                }
            });
            
            if (!hasAnyDevice) {
                console.log('📊 No devices available for stats toggle');
                return;
            }
            
            // 切换所有设备的统计状态
            const targetState = !allEnabled;
            let toggledCount = 0;
            
            Object.values(devices).forEach(device => {
                if (device) {
                    const currentState = device.toggleVideoStats();
                    if (currentState === targetState) {
                        toggledCount++;
                    }
                }
            });
            
            console.log(\`📊 Video stats \${targetState ? 'enabled' : 'disabled'} for \${toggledCount} devices\`);
        }
        
        function connectDevice(deviceId) {
            const device = devices[deviceId];
            if (device) device.connect();
        }
        
        function disconnectDevice(deviceId) {
            const device = devices[deviceId];
            if (device) device.disconnect();
        }
        
        function changeRoom(deviceId) {
            const device = devices[deviceId];
            const selectElement = document.getElementById(\`roomSelect\${deviceId.slice(-1)}\`);
            if (device && selectElement) {
                device.changeRoom(selectElement.value);
            }
        }
        

        
        function changeBitrate(deviceId) {
            const device = devices[deviceId];
            const selectElement = document.getElementById(\`bitrateSelect\${deviceId.slice(-1)}\`);
            if (device && selectElement) {
                device.setBitrate(parseInt(selectElement.value));
            }
        }
        
        function changeKeyFrame(deviceId) {
            const device = devices[deviceId];
            const selectElement = document.getElementById(\`keyFrameSelect\${deviceId.slice(-1)}\`);
            if (device && selectElement) {
                device.setKeyFrameInterval(parseInt(selectElement.value));
            }
        }
        
        function changeQuality(deviceId) {
            const device = devices[deviceId];
            const selectElement = document.getElementById(\`qualitySelect\${deviceId.slice(-1)}\`);
            if (device && selectElement) {
                device.setQualityLevel(parseInt(selectElement.value));
            }
        }
        
        function changeEncoderFps(deviceId) {
            const device = devices[deviceId];
            const selectElement = document.getElementById(\`encoderFpsSelect\${deviceId.slice(-1)}\`);
            if (device && selectElement) {
                device.setEncoderFps(parseInt(selectElement.value));
            }
        }
        
        function changeRotation(deviceId) {
            const device = devices[deviceId];
            const selectElement = document.getElementById(\`rotationSelect\${deviceId.slice(-1)}\`);
            if (device && selectElement) {
                device.setRotation(parseInt(selectElement.value));
            }
        }
        
        function changeVideoScale(deviceId) {
            const selectElement = document.getElementById(\`scaleSelect\${deviceId.slice(-1)}\`);
            if (selectElement) {
                const scaleValue = parseFloat(selectElement.value);
                console.log(\`🔍 [\${deviceId}] 页面设置缩放比例: \${(scaleValue * 100).toFixed(0)}%\`);
                
                // 保存缩放设置到localStorage（这是关键，确保设置被持久化）
                localStorage.setItem(\`videoScale_\${deviceId}\`, scaleValue.toString());
                console.log(\`💾 [\${deviceId}] 缩放设置已保存到localStorage\`);
                
                // 尝试立即应用缩放（如果控制器已就绪）
                if (window.multiDeviceMouseController) {
                    console.log(\`🔍 [\${deviceId}] multiDeviceMouseController存在，查找控制器...\`);
                    
                    // 调试：查看所有已注册的控制器
                    const allControllers = Array.from(window.multiDeviceMouseController.deviceControllers.keys());
                    console.log(\`🔍 [\${deviceId}] 已注册的控制器: \${allControllers.join(', ')}\`);
                    
                    const controller = window.multiDeviceMouseController.getController(deviceId);
                    if (controller) {
                        controller.setVideoScale(scaleValue);
                        console.log(\`✅ [\${deviceId}] 缩放比例已立即应用: \${(scaleValue * 100).toFixed(0)}%\`);
                        return;
                    } else {
                        console.warn(\`⚠️ [\${deviceId}] 控制器未找到，但multiDeviceMouseController存在\`);
                    }
                } else {
                    console.warn(\`⚠️ [\${deviceId}] multiDeviceMouseController不存在\`);
                }
                
                // 如果控制器未就绪，不进行重试，依赖控制器初始化时的自动恢复
                console.log(\`📝 [\${deviceId}] 缩放设置已保存，将在控制器初始化时自动应用\`);
            } else {
                console.warn(\`⚠️ [\${deviceId}] 缩放选择器未找到\`);
            }
        }
        
        function toggleMute(deviceId) {
            const device = devices[deviceId];
            if (device) device.toggleMute();
        }
        
        function toggleAllMute() {
            const muteAllBtn = document.getElementById('muteAllBtn');
            const firstDevice = Object.values(devices).find(device => device);
            if (!firstDevice) return;
            
            const hasUnmutedDevice = Object.values(devices).some(device => device && !device.isMuted);
            const targetMuteState = hasUnmutedDevice;
            
            Object.values(devices).forEach(device => {
                if (device) {
                    if (device.isMuted !== targetMuteState) {
                        device.toggleMute();
                    }
                }
            });
            
            if (muteAllBtn) {
                muteAllBtn.innerHTML = targetMuteState ? '🔇 全部静音' : '🔊 全部声音';
                muteAllBtn.classList.toggle('muted', targetMuteState);
            }
        }
        
        // 控制按钮处理
        function powerButtonHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'power' });
        }
        
        function recentButtonHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'recent' });
        }
        
        function homeButtonHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'home' });
        }
        
        function backButtonHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'back' });
        }
        
        function lockButtonHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'lock' });
        }
        
        function volumeUpHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'volume_up' });
        }
        
        function volumeDownHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'volume_down' });
        }
        
        function getClipboardHandler(deviceId) {
            sendControlMessage(deviceId, { type: 'get_clipboard' });
        }
        
        function setClipboardHandler(deviceId) {
            const clipboardText = prompt('请输入要设置到手机剪贴板的内容:', '');
            if (clipboardText !== null && clipboardText.trim() !== '') {
                sendControlMessage(deviceId, { 
                    type: 'set_clipboard', 
                    text: clipboardText.trim() 
                });
                console.log(\`📋 [\${deviceId}] 设置剪贴板内容: \${clipboardText.substring(0, 50)}\${clipboardText.length > 50 ? '...' : ''}\`);
            }
        }
        
        function sendControlMessage(deviceId, message) {
            const device = devices[deviceId];
            const dataChannel = window[\`dataChannel_\${deviceId}\`];
            
            if (device && dataChannel && dataChannel.readyState === 'open') {
                let controlMessage;
                
                // 处理剪贴板消息
                if (message.type === 'get_clipboard') {
                    controlMessage = { type: 'button_get_clipboard' };
                } else if (message.type === 'set_clipboard') {
                    controlMessage = { 
                        type: 'button_set_clipboard',
                        text: message.text 
                    };
                } else {
                    // 其他控制消息
                    controlMessage = { type: 'button_' + message.type };
                }
                
                dataChannel.send(JSON.stringify(controlMessage));
                console.log(\`🎮 [\${deviceId}] Control message sent:\`, controlMessage);
                
                // 剪贴板操作的用户反馈
                if (message.type === 'get_clipboard') {
                    showNotification(deviceId, '📋 正在获取手机剪贴板...', 'info');
                } else if (message.type === 'set_clipboard') {
                    showNotification(deviceId, '📝 剪贴板内容已发送到手机', 'success');
                }
            } else {
                console.warn(\`⚠️ [\${deviceId}] Cannot send control message - DataChannel not ready\`);
                showNotification(deviceId, '❌ 设备未连接，无法发送剪贴板命令', 'error');
            }
        }
        
        // 显示通知的辅助函数
        function showNotification(deviceId, message, type = 'info') {
            const deviceElement = document.getElementById(deviceId);
            if (!deviceElement) return;
            
            // 创建通知元素
            const notification = document.createElement('div');
            notification.className = \`notification notification-\${type}\`;
            notification.textContent = message;
            notification.style.cssText = \`
                position: absolute;
                top: 10px;
                right: 10px;
                padding: 8px 12px;
                border-radius: 6px;
                font-size: 12px;
                font-weight: bold;
                z-index: 1000;
                animation: slideIn 0.3s ease;
                background: \${type === 'success' ? '#4CAF50' : type === 'error' ? '#F44336' : '#2196F3'};
                color: white;
                box-shadow: 0 2px 6px rgba(0,0,0,0.2);
            \`;
            
            // 添加到设备窗口
            deviceElement.style.position = 'relative';
            deviceElement.appendChild(notification);
            
            // 3秒后自动移除
            setTimeout(() => {
                if (notification.parentNode) {
                    notification.style.animation = 'slideOut 0.3s ease';
                    setTimeout(() => {
                        if (notification.parentNode) {
                            notification.parentNode.removeChild(notification);
                        }
                    }, 300);
                }
            }, 3000);
        }
        
        function changeResolution(deviceId) {
            const resolutionSelect = document.getElementById(\`resolutionSelect\${deviceId.slice(-1)}\`);
            const selectedValue = resolutionSelect.value;
            const device = devices[deviceId];
            if (!device) return;
            
            if (selectedValue === 'custom') {
                const customWidth = prompt('请输入宽度 (像素):', device.width);
                const customHeight = prompt('请输入高度 (像素):', device.height);
                
                if (customWidth && customHeight) {
                    const width = parseInt(customWidth);
                    const height = parseInt(customHeight);
                    if (width > 0 && height > 0) {
                        device.setResolution(width, height);
                    } else {
                        alert('分辨率参数无效');
                        resolutionSelect.value = \`\${device.width}x\${device.height}\`;
                    }
                } else {
                    resolutionSelect.value = \`\${device.width}x\${device.height}\`;
                }
            } else {
                const [width, height] = selectedValue.split('x').map(Number);
                device.setResolution(width, height);
            }
        }
        
        // 全局函数：获取设备旋转角度（供鼠标控制器使用）
        window.getDeviceRotation = function(deviceId) {
            const device = devices[deviceId];
            return device ? device.rotation : 0;
        };
        
        // 页面加载完成后初始化
        document.addEventListener('DOMContentLoaded', function() {
            console.log('📄 DOMContentLoaded - Device manager initialization started');
            initializeDevices();
        });
        
        // 备用初始化（确保一定会执行）
        window.onload = function() {
            console.log('🚀 Window onload - Checking device manager status');
            if (Object.keys(devices).length === 0) {
                console.log('🔄 Backup initialization triggered');
                initializeDevices();
            } else {
                console.log('✅ Device manager initialized with ' + Object.keys(devices).length + ' devices');
            }
        };
        
        // 延迟初始化（最后的保障）
        setTimeout(function() {
            if (Object.keys(devices).length === 0) {
                console.log('⏰ Delayed initialization triggered');
                initializeDevices();
            }
        }, 1000);
        
        window.onbeforeunload = function() {
            disconnectAllDevices();
        };
        
        // ===========================================
        // 兼容性函数：将老的onclick调用重定向到新的模块化接口
        // ===========================================
        
        // 等待新的模块化系统加载完成
        let moduleSystemReady = false;
        let compatibilityFunctions = {};
        
        window.addEventListener('moduleSystemReady', function() {
            moduleSystemReady = true;
            console.log('📦 Module system ready, compatibility functions enabled');
        });
        
        // 连接设备的兼容性函数
        function connectDevice(deviceId) {
            console.log('=== CONNECT DEVICE CALLED ===');
            console.log('[Compatibility] Connect device request: ' + deviceId);
            console.log('[Compatibility] moduleSystemReady: ' + moduleSystemReady);
            console.log('[Compatibility] window.getApp exists: ' + (typeof window.getApp === 'function'));
            
            if (moduleSystemReady && window.getApp) {
                console.log('[Compatibility] Using NEW ES6 module interface');
                // 使用新的模块化接口
                const app = window.getApp();
                console.log('[Compatibility] App instance: ', app);
                const uiController = app.deviceControllers.get(deviceId);
                console.log('[Compatibility] UI Controller: ', uiController);
                if (uiController) {
                    console.log('[Compatibility] Calling uiController.handleConnect()');
                    uiController.handleConnect();
                    return;
                } else {
                    console.warn('[Compatibility] UI Controller not found for: ' + deviceId);
                }
            }
            
            console.log('[Compatibility] Using OLD interface fallback');
            // 回退到老的接口
            const device = devices[deviceId];
            console.log('[Compatibility] Old device manager: ', device);
            if (device) {
                console.log('[Compatibility] Calling device.connect()');
                device.connect();
            } else {
                console.warn('[Compatibility] Device ' + deviceId + ' not found');
            }
        }
        
        // 断开设备的兼容性函数
        function disconnectDevice(deviceId) {
            console.log('[Compatibility] Disconnect device request: ' + deviceId);
            
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    uiController.handleDisconnect();
                    return;
                }
            }
            
            const device = devices[deviceId];
            if (device) {
                device.disconnect();
            }
        }
        
        // 静音切换的兼容性函数
        function toggleMute(deviceId) {
            console.log('[Compatibility] Toggle mute request: ' + deviceId);
            
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    uiController.handleMuteToggle();
                    return;
                }
            }
            
            const device = devices[deviceId];
            if (device) {
                device.toggleMute();
            }
        }
        
        // 其他设置变更的兼容性函数
        function changeRoom(deviceId) {
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    const roomSelect = document.getElementById('roomSelect' + deviceId.slice(-1));
                    if (roomSelect) {
                        uiController.handleRoomChange(roomSelect.value);
                    }
                    return;
                }
            }
            
            // 回退到老的接口
            const device = devices[deviceId];
            if (device) {
                const roomSelect = document.getElementById('roomSelect' + deviceId.slice(-1));
                if (roomSelect) {
                    device.setRoom(roomSelect.value);
                }
            }
        }
        
        function changeBitrate(deviceId) {
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    const bitrateSelect = document.getElementById('bitrateSelect' + deviceId.slice(-1));
                    if (bitrateSelect) {
                        uiController.handleBitrateChange(parseInt(bitrateSelect.value));
                    }
                    return;
                }
            }
            
            const device = devices[deviceId];
            if (device) {
                const bitrateSelect = document.getElementById('bitrateSelect' + deviceId.slice(-1));
                if (bitrateSelect) {
                    device.setBitrate(parseInt(bitrateSelect.value));
                }
            }
        }
        
        function changeKeyFrame(deviceId) {
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    const keyFrameSelect = document.getElementById('keyFrameSelect' + deviceId.slice(-1));
                    if (keyFrameSelect) {
                        uiController.handleKeyFrameChange(parseInt(keyFrameSelect.value));
                    }
                    return;
                }
            }
            
            const device = devices[deviceId];
            if (device) {
                const keyFrameSelect = document.getElementById('keyFrameSelect' + deviceId.slice(-1));
                if (keyFrameSelect) {
                    device.setKeyFrameInterval(parseInt(keyFrameSelect.value));
                }
            }
        }
        
        function changeQuality(deviceId) {
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    const qualitySelect = document.getElementById('qualitySelect' + deviceId.slice(-1));
                    if (qualitySelect) {
                        uiController.handleQualityChange(parseInt(qualitySelect.value));
                    }
                    return;
                }
            }
            
            const device = devices[deviceId];
            if (device) {
                const qualitySelect = document.getElementById('qualitySelect' + deviceId.slice(-1));
                if (qualitySelect) {
                    device.setQuality(parseInt(qualitySelect.value));
                }
            }
        }
        
        function changeEncoderFps(deviceId) {
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    const encoderFpsSelect = document.getElementById('encoderFpsSelect' + deviceId.slice(-1));
                    if (encoderFpsSelect) {
                        uiController.handleEncoderFpsChange(parseInt(encoderFpsSelect.value));
                    }
                    return;
                }
            }
            
            const device = devices[deviceId];
            if (device) {
                const encoderFpsSelect = document.getElementById('encoderFpsSelect' + deviceId.slice(-1));
                if (encoderFpsSelect) {
                    device.setEncoderFps(parseInt(encoderFpsSelect.value));
                }
            }
        }
        
        function changeRotation(deviceId) {
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    const rotationSelect = document.getElementById('rotationSelect' + deviceId.slice(-1));
                    if (rotationSelect) {
                        uiController.handleRotationChange(parseInt(rotationSelect.value));
                    }
                    return;
                }
            }
            
            const device = devices[deviceId];
            if (device) {
                const rotationSelect = document.getElementById('rotationSelect' + deviceId.slice(-1));
                if (rotationSelect) {
                    device.setRotation(parseInt(rotationSelect.value));
                }
            }
        }
        
        function changeVideoScale(deviceId) {
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                const uiController = app.deviceControllers.get(deviceId);
                if (uiController) {
                    const scaleSelect = document.getElementById('scaleSelect' + deviceId.slice(-1));
                    if (scaleSelect) {
                        uiController.handleScaleChange(parseFloat(scaleSelect.value));
                    }
                    return;
                }
            }
            
            // 回退到老的接口
            const device = devices[deviceId];
            if (device) {
                const scaleSelect = document.getElementById('scaleSelect' + deviceId.slice(-1));
                if (scaleSelect) {
                    device.setVideoScale(parseFloat(scaleSelect.value));
                }
            }
        }
        
        // 全局控制函数
        function connectAllDevices() {
            console.log('[Compatibility] Connect all devices request');
            
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                app.connectAllDevices();
                return;
            }
            
            // 回退到老的接口
            Object.values(devices).forEach(device => {
                if (!device.isConnected) {
                    device.connect();
                }
            });
        }
        
        function disconnectAllDevices() {
            console.log('[Compatibility] Disconnect all devices request');
            
            if (moduleSystemReady && window.getApp) {
                const app = window.getApp();
                app.disconnectAllDevices();
                return;
            }
            
            Object.values(devices).forEach(device => {
                if (device.isConnected) {
                    device.disconnect();
                }
            });
        }
        
        function toggleAllVideoStats() {
            console.log('[Compatibility] Toggle all video stats');
            
            Object.values(devices).forEach(device => {
                if (device.toggleVideoStats) {
                    device.toggleVideoStats();
                }
            });
        }
        
        function toggleAllMute() {
            console.log('[Compatibility] Toggle all mute');
            
            Object.values(devices).forEach(device => {
                if (device.toggleMute) {
                    device.toggleMute();
                }
            });
        }
        
        // 暴露兼容性函数到全局作用域
        window.connectDevice = connectDevice;
        window.disconnectDevice = disconnectDevice;
        window.toggleMute = toggleMute;
        window.changeRoom = changeRoom;
        window.changeBitrate = changeBitrate;
        window.changeKeyFrame = changeKeyFrame;
        window.changeQuality = changeQuality;
        window.changeEncoderFps = changeEncoderFps;
        window.changeRotation = changeRotation;
        window.changeVideoScale = changeVideoScale;
        window.connectAllDevices = connectAllDevices;
        window.disconnectAllDevices = disconnectAllDevices;
        window.toggleAllVideoStats = toggleAllVideoStats;
        window.toggleAllMute = toggleAllMute;
        
        console.log('[Compatibility] Compatibility functions setup completed, supporting new/old architecture switching');
    </script>
</body>
</html>`;

  return html;
}

// 生成单个设备窗口的HTML
function generateDeviceWindow(deviceNum) {
  return `
        <!-- 设备${deviceNum} -->
        <div class="device_window" id="device${deviceNum}">
            <div class="device_header">
                <h3>📱 设备 ${deviceNum}</h3>
                <div class="device_controls">
                    <button onclick="connectDevice('device${deviceNum}')" class="device_btn">🔗 连接</button>
                    <button onclick="disconnectDevice('device${deviceNum}')" class="device_btn">❌ 断开</button>
                    <button onclick="toggleMute('device${deviceNum}')" id="muteBtn${deviceNum}" class="device_btn mute_btn">🔊 声音</button>
                    <select id="roomSelect${deviceNum}" onchange="changeRoom('device${deviceNum}')" class="room_select">
                        <option value="mobile_screen">默认房间</option>
                        <option value="mobile_screen_2">房间2</option>
                        <option value="mobile_screen_3">房间3</option>
                        <option value="mobile_screen_4">房间4</option>
                        <option value="mobile_screen_5">房间5</option>
                        <option value="mobile_screen_6">房间6</option>
                    </select>
                    <select id="resolutionSelect${deviceNum}" onchange="changeResolution('device${deviceNum}')" class="resolution_select">
                        <option value="1080x1920">1080x1920 (FHD)</option>
                        <option value="720x1280">720x1280 (HD)</option>
                        <option value="480x854">480x854 (FWVGA)</option>
                        <option value="1440x2560">1440x2560 (QHD)</option>
                        <option value="540x960">540x960 (qHD)</option>
                        <option value="custom">🔧 自定义</option>
                    </select>

                </div>
            </div>
            
            <div class="encoder_config_panel">
                <div class="encoder_config_title">🔧 编码器配置</div>
                <div class="encoder_controls">
                    <select id="bitrateSelect${deviceNum}" onchange="changeBitrate('device${deviceNum}')" class="bitrate_select" title="目标码率">
                        <option value="4000">4M</option>
                        <option value="5000" selected>5M</option>
                        <option value="6000">6M</option>
                        <option value="7000">7M</option>
                        <option value="8000">8M</option>
                        <option value="9000">9M</option>
                        <option value="10000">10M</option>
                        <option value="12000">12M</option>
                        <option value="15000">15M</option>
                    </select>
                    
                    <select id="keyFrameSelect${deviceNum}" onchange="changeKeyFrame('device${deviceNum}')" class="keyframe_select" title="关键帧间隔">
                        <option value="5">5s间隔</option>
                        <option value="10" selected>10s间隔</option>
                        <option value="15">15s间隔</option>
                        <option value="20">20s间隔</option>
                        <option value="30">30s间隔</option>
                    </select>
                    
                    <select id="qualitySelect${deviceNum}" onchange="changeQuality('device${deviceNum}')" class="quality_select" title="编码质量">
                        <option value="10">质量10%</option>
                        <option value="20">质量20%</option>
                        <option value="30">质量30%</option>
                        <option value="40">质量40%</option>
                        <option value="50">质量50%</option>
                        <option value="60">质量60%</option>
                        <option value="70">质量70%</option>
                        <option value="80" selected>质量80%</option>
                        <option value="90">质量90%</option>
                        <option value="100">质量100%</option>
                    </select>
                    
                    <select id="encoderFpsSelect${deviceNum}" onchange="changeEncoderFps('device${deviceNum}')" class="encoder_fps_select" title="目标帧率">
                        <option value="3">3 FPS</option>
                        <option value="5">5 FPS</option>
                        <option value="10">10 FPS</option>
                        <option value="15">15 FPS</option>
                        <option value="20">20 FPS</option>
                        <option value="25">25 FPS</option>
                        <option value="30" selected>30 FPS</option>
                        <option value="35">35 FPS</option>
                        <option value="40">40 FPS</option>
                        <option value="45">45 FPS</option>
                        <option value="50">50 FPS</option>
                        <option value="55">55 FPS</option>
                        <option value="60">60 FPS</option>
                    </select>
                    
                    <select id="rotationSelect${deviceNum}" onchange="changeRotation('device${deviceNum}')" class="rotation_select" title="画面旋转">
                        <option value="0" selected>0°</option>
                        <option value="90">90°</option>
                        <option value="180">180°</option>
                        <option value="270">270°</option>
                    </select>
                    
                    <select id="scaleSelect${deviceNum}" onchange="changeVideoScale('device${deviceNum}')" class="scale_select" title="视频缩放比例">
                        <option value="0.5">50%</option>
                        <option value="0.6">60%</option>
                        <option value="0.7">70%</option>
                        <option value="0.8">80%</option>
                        <option value="0.9">90%</option>
                        <option value="1.0" selected>100%</option>
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

// 根据窗口数量生成网格样式
function getGridStyles(windowCount) {
  if (windowCount <= 2) {
    return "grid-template-columns: repeat(2, 1fr);";
  } else if (windowCount <= 4) {
    return "grid-template-columns: repeat(2, 1fr);";
  } else if (windowCount <= 6) {
    return "grid-template-columns: repeat(3, 1fr);";
  } else if (windowCount <= 9) {
    return "grid-template-columns: repeat(3, 1fr);";
  } else {
    return "grid-template-columns: repeat(4, 1fr);";
  }
}

// 响应式网格样式
function getResponsiveGridStyles(windowCount) {
  if (windowCount <= 2) {
    return "grid-template-columns: repeat(2, 1fr);";
  } else if (windowCount <= 4) {
    return "grid-template-columns: repeat(2, 1fr);";
  } else {
    return "grid-template-columns: repeat(3, 1fr);";
  }
}

// 动态多窗口页面路由
app.get("/multi", (req, res) => {
  const windowCount = req.query.windows || 4;
  const html = generateMultiWindowPage(windowCount);
  res.send(html);
});

// SSL证书配置
let sslOptions;
try {
  sslOptions = {
    key: fs.readFileSync(path.join(__dirname, "../ssl/key.pem")),
    cert: fs.readFileSync(path.join(__dirname, "../ssl/cert.pem"))
  };
} catch (err) {
  console.warn("⚠️  SSL certificates not found, HTTPS server will not start");
  console.warn("   SSL error:", err.message);
}

// 启动HTTP服务器 (兼容localhost访问)
app.listen(HTTP_PORT, "0.0.0.0", () => {
  console.log(`🌐 HTTP WebScreen Frontend Server running on:`);
  console.log(`   Local:    http://localhost:${HTTP_PORT}`);
  console.log(`   Network:  http://192.168.3.158:${HTTP_PORT}`);
  console.log(`\n✅ HTTP适用于: localhost访问 (WebRTC兼容)`);
});

// 启动HTTPS服务器 (WebRTC完全兼容)
if (sslOptions) {
  https.createServer(sslOptions, app).listen(HTTPS_PORT, "0.0.0.0", () => {
    console.log(`🔒 HTTPS WebScreen Frontend Server running on:`);
    console.log(`   Local:    https://localhost:${HTTPS_PORT}`);
    console.log(`   Network:  https://192.168.3.158:${HTTPS_PORT}`);
    console.log(`\n✅ HTTPS适用于: IP地址访问 (WebRTC完全兼容)`);
    console.log(`📱 推荐使用HTTPS地址以获得最佳WebRTC兼容性`);
  });
}

// 优雅关闭
process.on("SIGTERM", () => {
  console.log("\n👋 Shutting down server gracefully...");
  process.exit(0);
});

process.on("SIGINT", () => {
  console.log("\n👋 Shutting down server gracefully...");
  process.exit(0);
});
