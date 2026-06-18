/**
 * 设备管理模块
 * 负责WebRTC设备连接、视频流控制、编码器配置等
 */
import {
  ENCODER_CONFIG,
  ROTATION_CONFIG,
  VIDEO_CONFIG,
  WEBRTC_CONFIG
} from "../core/config.js";
import { MouseControllerFactory } from "../input/mouse-controller.js";
import { getTime, storage } from "../utils/storage.js";
/**
 * WebRTC设备管理器
 * 处理单个设备的连接、流控制和配置管理
 */
export class DeviceManager {
  constructor({
    deviceId,
    wsUrl,
    roomId,
    token,
    fps,
    qualityLevel,
    isMain,
    width,
    height,
    disableSlideAcceleration,
    isGroupControl,
    isKickOff
  }) {
    console.log(
      getTime(),
      "初始化=============",
      wsUrl,
      roomId,
      token,
      fps,
      qualityLevel,
      isMain,
      width,
      height,
      disableSlideAcceleration,
      isGroupControl
    );
    this.deviceId = deviceId;
    this.wsUrl = wsUrl;
    this.roomId = roomId;
    this.tokenXX = token;
    this.isMain = isMain; // 是否为主控手机
    this.disableSlideAcceleration = disableSlideAcceleration || false;
    this.cpId = this.roomId?.split(":")[1];
    this.isGroupControl = isGroupControl || false;
    this.isKickOff = isKickOff || false; // 是否踢人，当前连接人数满了，为true，踢掉前一个人
    // // 自定义wsUrl
    // this.wsUrl = wsUrl;
    // // 自定义roomId
    // this.roomId = roomId;

    // WebRTC连接相关
    this.pc = null;
    this.remoteStream = null;
    this.dataWebSocket = null;
    this.dataChannel = null;
    this.isConnected = false;

    // 会话信息
    // this.token = null;
    this.androidSessionId = null;
    this.realSessionId = null;
    // this.roomId = WEBRTC_CONFIG.DEFAULT_ROOM;
    this.turnServerUrl = null;

    // 设备配置
    this.width = width || VIDEO_CONFIG.DEFAULT_RESOLUTION.width;
    this.height = height || VIDEO_CONFIG.DEFAULT_RESOLUTION.height;
    this.fps = VIDEO_CONFIG.DEFAULT_FPS;
    this.isMuted = false;

    // 编码器配置
    this.targetBitrate = ENCODER_CONFIG.DEFAULT_BITRATE;
    this.keyFrameInterval = ENCODER_CONFIG.DEFAULT_KEYFRAME_INTERVAL;
    this.qualityLevel = qualityLevel || ENCODER_CONFIG.DEFAULT_QUALITY;
    this.targetEncoderFps = fps || ENCODER_CONFIG.DEFAULT_ENCODER_FPS;

    // 视频变换配置
    this.rotation = ROTATION_CONFIG.DEFAULT;

    // 视频统计
    this.statsEnabled = true;
    this.statsInterval = null;
    this.webrtcStats = null;
    this.lastBytesReceived = 0;
    this.lastTimestamp = 0;

    // 网络配置
    this.pcConfig = {
      iceServers: [],
      ...WEBRTC_CONFIG.ICE_CONFIG
    };

    // DOM元素引用
    this.elements = {};
    this.loadingElement = null;

    // 事件回调
    this.callbacks = {
      onConnectionStateChange: null,
      onVideoReady: null,
      onStatsUpdate: null,
      onError: null
    };

    // 视频尺寸监控相关
    this.videoSizeMonitorInterval = null;
    this.lastVideoWidth = 0;
    this.lastVideoHeight = 0;
    this.lastClientWidth = 0;
    this.lastClientHeight = 0;
    this.boundOnWindowResize = null;

    // 摄像头相关
    this.localCameraStream = null; // 本地摄像头流
    this.cameraEnabled = false; // 摄像头开关状态
    this.cameraTrack = null; // 摄像头视频轨道
    this.cameraSender = null; // RtpSender 对象（预留轨道）
    this.placeholderCameraTrack = null; // 占位符黑屏轨道（关闭摄像头时使用）
    this.placeholderCameraStream = null; // 占位符流
    this.selectedCameraDeviceId = null; // 当前选中的摄像头设备ID
    this.mirrorVideoElement = null; // 镜像处理用的 video 元素
    this.mirrorCanvasElement = null; // 镜像处理用的 canvas 元素
    this.mirrorDrawingActive = false; // 镜像绘制循环是否激活
    this.cameraMirrorEnabled = false; // 摄像头镜像翻转开关（默认关闭）

    // DataChannel 音频传输相关（binary_pcm 协议）
    this.audioContext = null;
    this.audioProcessorNode = null;
    this.audioSourceNode = null;
    this.audioPcmSeq = 0; // PCM 包序列号

    // 初始化
    this.bindElements();
    // this.loadSettings();
  }

  /**
   * 绑定DOM元素
   */
  bindElements() {
    // const num = this.deviceId.slice(-1);
    const num = this.deviceId.replace("device", "");
    this.elements = {
      // 视频相关
      remoteVideo: document.querySelector(`#screen${num}`),
      fpsValue: document.getElementById(`fpsValue${num}`),
      videoResolution: document.getElementById(`videoResolution${num}`),
      droppedFrames: document.getElementById(`droppedFrames${num}`),
      videoBitrate: document.getElementById(`videoBitrate${num}`),

      // 连接状态
      dataChannelIcon: document.getElementById(`dataChannelIcon${num}`),
      dataChannelStatus: document.getElementById(`dataChannelStatus${num}`),

      // 控制元素
      roomSelect: document.getElementById(`roomSelect${num}`),
      resolutionSelect: document.getElementById(`resolutionSelect${num}`),
      muteBtn: document.getElementById(`muteBtn${num}`),

      // 编码器配置
      bitrateSelect: document.getElementById(`bitrateSelect${num}`),
      keyFrameSelect: document.getElementById(`keyFrameSelect${num}`),
      qualitySelect: document.getElementById(`qualitySelect${num}`),
      encoderFpsSelect: document.getElementById(`encoderFpsSelect${num}`),

      // 旋转控制
      rotationSelect: document.getElementById(`rotationSelect${num}`)
    };
  }

  /**
   * 加载设备设置
   */
  loadSettings() {
    const settings = storage.getDeviceSettings(this.deviceId);

    this.width = settings.resolution.width;
    this.height = settings.resolution.height;
    this.targetBitrate = settings.bitrate;
    this.keyFrameInterval = settings.keyFrame;
    this.qualityLevel = settings.quality;
    this.targetEncoderFps = settings.encoderFps;
    this.rotation = settings.rotation;
    this.isMuted = settings.muted;
    this.fps = settings.encoderFps; // FPS由编码器FPS控制

    this.updateUIFromSettings();

    // 应用旋转设置
    if (this.rotation !== 0) {
      this.applyRotation();
    }
  }

  /**
   * 根据设置更新UI元素
   */
  updateUIFromSettings() {
    const { elements } = this;

    if (elements.resolutionSelect) {
      elements.resolutionSelect.value = `${this.width}x${this.height}`;
    }

    if (elements.bitrateSelect) {
      elements.bitrateSelect.value = this.targetBitrate;
    }

    if (elements.keyFrameSelect) {
      elements.keyFrameSelect.value = this.keyFrameInterval;
    }

    if (elements.qualitySelect) {
      elements.qualitySelect.value = this.qualityLevel;
    }

    if (elements.encoderFpsSelect) {
      elements.encoderFpsSelect.value = this.targetEncoderFps;
    }

    if (elements.rotationSelect) {
      elements.rotationSelect.value = this.rotation;
    }

    this.updateMuteButtonState();
  }

  /**
   * 设置事件回调
   * @param {string} event 事件名称
   * @param {Function} callback 回调函数
   */
  on(event, callback) {
    if (this.callbacks.hasOwnProperty(event)) {
      this.callbacks[event] = callback;
    }
  }

  /**
   * 触发事件回调
   * @param {string} event 事件名称
   * @param {*} data 事件数据
   */
  emit(event, data) {
    if (this.callbacks[event] && typeof this.callbacks[event] === "function") {
      this.callbacks[event](data);
    }
  }

  /**
   * 连接到设备
   */
  async connect() {
    if (this.isConnected) {
      console.log(getTime(), `⚠️ [${this.deviceId}] Already connected`);
      return;
    }

    // const wsUrl = WEBRTC_CONFIG.WS_URL;
    console.log(
      getTime(),
      `🔗 [${this.deviceId}] Connecting to: ${this.wsUrl}`
    );

    try {
      this.dataWebSocket = new WebSocket(this.wsUrl);
      this.setupWebSocketHandlers();
    } catch (error) {
      console.error(`❌ [${this.deviceId}] Connection failed:`, error);
      this.emit("onError", { type: "connection", error });
    }
  }

  /**
   * 断开设备连接
   */
  disconnect() {
    if (this.dataWebSocket) {
      this.sendMessage({ type: "leave", roomId: this.roomId });
      this.dataWebSocket.close();
    }

    // 如果摄像头已开启，先关闭
    if (this.cameraEnabled) {
      this.closeCamera().catch(err => {
        console.error(
          getTime(),
          `❌ [${this.deviceId}] 断开连接时关闭摄像头失败:`,
          err
        );
      });
    }

    this.destroyPeerConnection();
    this.isConnected = false;
    this.updateConnectionStatus(false, "未连接");
    this.emit("onConnectionStateChange", {
      connected: false,
      status: "未连接"
    });
  }

  /**
   * 设置WebSocket事件处理器
   */
  setupWebSocketHandlers() {
    this.dataWebSocket.onopen = event => this.onWsOpen(event);
    this.dataWebSocket.onclose = event => this.onWsClose(event);
    this.dataWebSocket.onerror = error => this.onWsError(error);
    this.dataWebSocket.onmessage = event => this.onWsMessage(event);
  }

  /**
   * WebSocket连接打开事件
   */
  onWsOpen(event) {
    console.log(getTime(), `✅ [${this.deviceId}] WebSocket opened`);

    // 当质量为100%时，传递给app端99
    const appQualityLevel = this.qualityLevel === 100 ? 99 : this.qualityLevel;
    this.sendMessage({
      type: "join",
      clientType: "web",
      roomId: this.roomId,
      data: this.tokenXX,
      meta: {
        isGroupControl: this.isGroupControl || false, // 是否群控
        clipboardAutosync: this.isMain ? true : false,
        disableSlideAcceleration: this.disableSlideAcceleration,
        kickOff: this.isKickOff || false,
        resolution: {
          width: this.width,
          height: this.height
        },
        encoderConfig: {
          targetBitrate: this.targetBitrate * 1024 * 1024 * 8, // MB/s转换为bps (字节率*8=比特率)
          keyFrameInterval: this.keyFrameInterval,
          qualityLevel: appQualityLevel, // 100%时传递99
          targetFps: this.targetEncoderFps
        }
      }
    });
  }

  /**
   * WebSocket连接关闭事件
   */
  onWsClose(event) {
    console.log(
      getTime(),
      `❌ [${this.deviceId}] WebSocket closed:`,
      event.code,
      event.reason
    );

    // 如果摄像头已开启，关闭摄像头
    if (this.cameraEnabled) {
      this.closeCamera().catch(err => {
        console.error(
          getTime(),
          `❌ [${this.deviceId}] WebSocket关闭时关闭摄像头失败:`,
          err
        );
      });
    }

    // 清理连接资源
    this.destroyPeerConnection();
    this.clearVideoDisplay();

    this.isConnected = false;
    this.updateConnectionStatus(false, "连接已断开");
    this.emit("onConnectionStateChange", {
      connected: false,
      status: "连接已断开"
    });
  }

  /**
   * WebSocket错误事件
   */
  onWsError(error) {
    console.error(`❌ [${this.deviceId}] WebSocket error:`, error);
    this.emit("onError", { type: "websocket", error });

    // 清理连接状态和资源
    this.destroyPeerConnection();
    this.clearVideoDisplay();

    if (this.dataWebSocket) {
      try {
        this.dataWebSocket.close();
      } catch (err) {
        console.warn(
          getTime(),
          `⚠️ [${this.deviceId}] 关闭 WebSocket 时出错:`,
          err
        );
      }
      this.dataWebSocket = null;
    }

    this.isConnected = false;
    this.updateConnectionStatus(false, "WebSocket连接失败");
    this.handleConnectionFailed("websocket连接失败");
    this.emit("onConnectionStateChange", {
      connected: false,
      status: "WebSocket连接失败"
    });
  }

  /**
   * WebSocket消息处理
   */
  onWsMessage(event) {
    const message = JSON.parse(event.data);

    switch (message.type) {
      case "welcome":
      case "joined":
        console.log(
          getTime(),
          `✅ [${this.deviceId}] Joined room: ${this.roomId}`
        );
        if (message.from) this.realSessionId = message.from;
        if (message.data) {
          if (message.data.token) this.token = message.data.token;
          if (message.data.server) this.turnServerUrl = message.data.server;
        }
        console.log(
          getTime(),
          `🔗 [${this.deviceId}] TURN Server URL: ${this.turnServerUrl}`
        );
        this.initNetworkConfig();
        break;

      case "offer":
        this.androidSessionId = message.from;
        this.handleSdpMessage({ type: "sdp", sdp: message.data });
        break;

      case "ice-candidate":
        this.handleIceMessage({ type: "ice", ice: message.data });
        break;

      case "remote-closed":
        console.log(getTime(), `📱 [${this.deviceId}] Remote streaming ended`);
        this.handleRemoteHangup();
        break;

      case "app-connection-failed":
        console.log(
          getTime(),
          `❌ [${this.deviceId}] App connection failed:`,
          message.data
        );
        this.handleAppConnectionFailed(message.data);
        break;

      case "user-left":
        console.log(getTime(), `👋 [${this.deviceId}] User left:`, message);
        // this.handleUserLeft(message);
        break;

      case "error":
        console.warn(`⚠️ [${this.deviceId}] Server error:`, message.data);
        break;
      case "custom-msg":
        this.handleCustomMsg(message.data);
        break;
      default:
        console.log(
          getTime(),
          `📨 [${this.deviceId}] Unknown message:`,
          message
        );
    }
  }
  /**
   * 提示自定义消息
   * @param {Object} data 消息数据
   */
  handleShowMsg(data) {
    // 创建loading元素
    if (!this.loadingElement) {
      this.loadingElement = document.createElement("div");
    }
    this.loadingElement.style.cssText = `
            position: absolute;
            top: 8px;
            left: 8px;
            width: calc(100% - 16px);
            height: calc(100% - 16px);
            background: rgba(0, 0, 0, 0.7);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 9;
            flex-direction: column;
            border-radius: 32px;
        `;

    // 创建spinner
    const spinner = document.createElement("div");
    spinner.style.cssText = `
            width: 40px;
            height: 40px;
            border: 4px solid rgba(255, 255, 255, 0.3);
            border-top: 4px solid #ffffff;
            border-radius: 50%;
            animation: xphone-spin 1s linear infinite;
        `;

    // 创建文本
    const text = document.createElement("div");
    text.textContent = data?.isGroupControl
      ? "连接中断，设备由窗口同步占用，您无法通过 WebRTC 重新连接。如需操作，请操作窗口同步"
      : "连接中断，其他人已连接设备，请重新连接";
    if (data?.isGroupControl === true) {
      text.style.cssText = `
            color: #ffffff;
            font-size: 14px;
            margin-top: 12px;
            font-family: Arial, sans-serif;
            text-align: center;
            padding: 0 20px;
        `;
    } else {
      text.style.cssText = `
            color: #ffffff;
            font-size: 14px;
            margin-top: 12px;
            font-family: Arial, sans-serif;
            text-align: center;
        `;
    }

    // this.loadingElement.appendChild(spinner);
    this.loadingElement.appendChild(text);

    // 保存文本元素引用
    this.loadingTextElement = text;

    // 添加CSS动画
    if (!document.getElementById("xphone-loading-style")) {
      const style = document.createElement("style");
      style.id = "xphone-loading-style";
      style.textContent = `
                @keyframes xphone-spin {
                    0% { transform: rotate(0deg); }
                    100% { transform: rotate(360deg); }
                }
            `;
      document.head.appendChild(style);
    }

    // 确保容器有相对定位
    const videoContainer = this.elements.remoteVideo.parentElement;
    if (
      videoContainer.style.position !== "relative" &&
      videoContainer.style.position !== "absolute"
    ) {
      videoContainer.style.position = "relative";
    }
    videoContainer.appendChild(this.loadingElement);
  }
  handleCustomMsg(data) {
    if (data.type === "kickOff") {
      this.handleShowMsg(data);
      this.disconnect();
      window.CphoneWebRTC?.onKickOff(this.deviceId, true);
    }
    if (data.code === 1005) {
      this.disconnect();
    }
    if (data.code === 1001) {
      // userJoined failedjava.lang.RuntimeException: EGL creation errorjava.lang.RuntimeException: Task execution failed
      this.handleConnectionFailed("用户加入房间失败");
      this.disconnect();
    }
  }
  /**
   * 发送WebSocket消息
   */
  sendMessage(message) {
    if (
      this.dataWebSocket &&
      this.dataWebSocket.readyState === WebSocket.OPEN
    ) {
      this.dataWebSocket.send(JSON.stringify(message));
    }
  }

  /**
   * 初始化网络配置
   */
  initNetworkConfig() {
    if (this.turnServerUrl) {
      this.pcConfig.iceServers = [
        {
          urls: this.turnServerUrl,
          username: this.token,
          credential: this.roomId
        }
      ];
      console.log(
        getTime(),
        `🔧 [${this.deviceId}] Network config initialized with TURN: ${this.turnServerUrl}`
      );
    } else {
      console.log(
        getTime(),
        `🔧 [${this.deviceId}] Network config initialized without TURN server`
      );
    }

    // 标记为已连接并创建 PeerConnection
    this.isConnected = true;
    this.updateConnectionStatus(true, "已连接");
    this.emit("onConnectionStateChange", { connected: true, status: "已连接" });

    // 创建 PeerConnection
    if (!this.pc) {
      this.createPeerConnection();
    }
  }

  /**
   * 创建PeerConnection
   */
  createPeerConnection() {
    try {
      // 🔧 添加连接计数和垃圾回收机制（基于test1）
      window.activePeerConnections = window.activePeerConnections || 0;
      window.peerOperationCount = window.peerOperationCount || 0;
      window.peerOperationCount++;

      // 每20次操作强制垃圾回收一次（参考test1）
      if (window.peerOperationCount % 20 === 0) {
        this.forceGarbageCollection();
      }

      console.log(getTime(), "配置打印", this.pcConfig);
      this.pc = new RTCPeerConnection(this.pcConfig);
      window.activePeerConnections++;

      console.log(
        getTime(),
        `🔧 [${this.deviceId}] PeerConnection created (total: ${window.activePeerConnections}, operations: ${window.peerOperationCount})`
      );

      // 设置事件处理器
      this.pc.onicecandidate = event => this.handleIceCandidate(event);
      this.pc.onaddstream = event => this.handleRemoteStreamAdded(event);
      this.pc.onremovestream = event => this.handleRemoteStreamRemoved(event);
      this.pc.oniceconnectionstatechange = () =>
        this.handleConnectionStateChange();

      // 设置DataChannel监听器
      this.setupDataChannelListeners();

      // 🔥 预留摄像头轨道：添加一个黑屏轨道，避免后续打开摄像头时重协商
      this.addPlaceholderCameraTrack();
    } catch (error) {
      console.error(
        `❌ [${this.deviceId}] PeerConnection creation failed:`,
        error
      );
      this.emit("onError", { type: "peerconnection", error });
    }
  }

  /**
   * 强制垃圾回收 - 基于用户测试脚本优化的方法
   * 使用微任务队列延迟执行，创建大对象触发GC
   */
  forceGarbageCollection() {
    try {
      // 创建大对象强制触发GC（参考test1）
      const blob = new Blob([new ArrayBuffer(5e6)]); // 5MB
      const url = window.URL.createObjectURL(blob);

      // 立即释放，触发垃圾回收
      window.URL.revokeObjectURL(url);
      console.log(
        getTime(),
        `🧹 [${this.deviceId}] Forced garbage collection triggered (operation #${window.peerOperationCount})`
      );
    } catch (error) {
      console.warn(`⚠️ [${this.deviceId}] Force GC failed:`, error);
    }
  }

  /**
   * 销毁PeerConnection
   */
  destroyPeerConnection() {
    this.stopVideoStats();

    // 关闭摄像头
    if (this.cameraEnabled) {
      this.closeCamera();
    }

    if (this.pc) {
      // 关闭连接
      this.pc.close();
      this.pc = null;

      // 🔧 减少连接计数
      if (window.activePeerConnections > 0) {
        window.activePeerConnections--;
      }

      console.log(
        getTime(),
        `🔧 [${this.deviceId}] PeerConnection关闭 (剩余连接: ${window.activePeerConnections})`
      );
      // 🔧 销毁时也触发垃圾回收
      this.forceGarbageCollection();
    }

    // 🔥 清空摄像头 Sender，以便在重新创建 PeerConnection 时可以重新添加摄像头轨道
    if (this.cameraSender) {
      console.log(getTime(), `🔧 [${this.deviceId}] Clearing camera sender`);
      this.cameraSender = null;
    }
  }

  /**
   * 处理SDP消息
   */
  async handleSdpMessage(message) {
    if (message.sdp.type === "offer") {
      // 打印完整消息内容，调试用
      console.log(
        `[${this.deviceId}] Received offer, message:`,
        JSON.stringify(message, null, 2)
      );

      // 检查是否是ICE重启（iceRestart字段在message.sdp中，因为传入时sdp=message.data）
      const isIceRestart = message.sdp.iceRestart === true;
      console.log(`[${this.deviceId}] isIceRestart: ${isIceRestart}`);

      if (this.pc && this.pc.remoteDescription) {
        if (isIceRestart) {
          // ICE重启，保持PeerConnection
          console.log(`[${this.deviceId}] ICE Restart，保持PeerConnection`);
        } else {
          // 推流端重启，销毁重建PeerConnection
          console.log(`[${this.deviceId}] 推流端重启，销毁旧连接重建`);
          this.destroyPeerConnection();
        }
      }

      if (!this.pc) {
        console.log(
          `[${this.deviceId}] PeerConnection不存在，先初始化网络配置`
        );
        this.initNetworkConfig();
      }

      try {
        await this.pc.setRemoteDescription(
          new RTCSessionDescription(message.sdp)
        );
        this.doAnswer();
      } catch (error) {
        console.error(
          `[${this.deviceId}] Set remote description failed:`,
          error
        );
      }
    }
  }

  /**
   * 创建应答
   */
  async doAnswer() {
    try {
      const sessionDescription = await this.pc.createAnswer();
      this.setLocalAndSendMessage(sessionDescription);
    } catch (error) {
      console.error(`❌ [${this.deviceId}] Create answer failed:`, error);
      this.emit("onError", { type: "answer", error });
    }
  }

  /**
   * 设置本地描述并发送消息
   */
  async setLocalAndSendMessage(sessionDescription) {
    await this.pc.setLocalDescription(sessionDescription);
    this.sendSdpMessage(sessionDescription);
  }

  /**
   * 发送SDP消息
   */
  sendSdpMessage(sessionDescription) {
    this.sendMessage({
      type: sessionDescription.type, // 直接使用 'answer' 类型
      data: sessionDescription,
      to: this.androidSessionId,
      from: this.realSessionId,
      roomId: this.roomId
    });
  }

  /**
   * 处理ICE消息
   */
  async handleIceMessage(message) {
    if (message.ice && message.ice.type === "candidate") {
      try {
        console.log(getTime(), `📨 [${this.deviceId}] Adding ICE candidate`);
        //确保在处理ICE candidate前pc已经被初始化
        // if (!this.pc) {
        //   console.log(getTime(), `e[${this.deviceId}] PeerConnection不存在`);
        //   this.initNetworkConfig();
        // }

        // 按照旧版本格式处理：使用label作为sdpMLineIndex
        let candidate = new RTCIceCandidate({
          sdpMLineIndex: message.ice.label,
          candidate: message.ice.candidate
        });

        await this.pc.addIceCandidate(candidate);
        console.log(
          getTime(),
          `✅ [${this.deviceId}] ICE candidate added successfully`
        );
      } catch (error) {
        console.error(`❌ [${this.deviceId}] Add ICE candidate failed:`, error);
      }
    }
  }

  /**
   * 处理ICE候选者
   */
  handleIceCandidate(event) {
    if (event.candidate && this.androidSessionId) {
      this.sendMessage({
        type: "ice-candidate",
        roomId: this.roomId,
        to: this.androidSessionId,
        data: {
          type: "candidate",
          label: event.candidate.sdpMLineIndex,
          id: event.candidate.sdpMid,
          candidate: event.candidate.candidate
        }
      });
    }
  }

  /**
   * 处理远程流添加
   */
  handleRemoteStreamAdded(event) {
    console.log(getTime(), `📺 [${this.deviceId}] Remote stream added`);
    this.remoteStream = event.stream;

    if (this.elements.remoteVideo) {
      this.elements.remoteVideo.srcObject = this.remoteStream;
      this.tryAutoPlay();
      this.initMouseControlWhenReady();

      // 启动视频统计
      if (this.statsEnabled) {
        this.startVideoStats();
      }

      this.emit("onVideoReady", { stream: this.remoteStream });
    }
  }

  /**
   * 尝试自动播放
   */
  tryAutoPlay() {
    if (this.elements.remoteVideo) {
      const playPromise = this.elements.remoteVideo.play();
      if (playPromise !== undefined) {
        playPromise.catch(error => {
          console.warn(`⚠️ [${this.deviceId}] Auto-play failed:`, error);
        });
      }
    }
  }

  /**
   * 初始化鼠标控制
   */
  initMouseControlWhenReady() {
    const checkVideoReady = () => {
      // 使用新的MouseControllerFactory
      const mouseController = MouseControllerFactory.createController(
        this.deviceId
      );
      console.log(
        getTime(),
        `🖱️ [${this.deviceId}] Mouse control initialized with new architecture`
      );

      if (this.elements.remoteVideo.readyState >= 1) {
        const videoWidth = this.elements.remoteVideo.videoWidth;
        const videoHeight = this.elements.remoteVideo.videoHeight;
        console.log(
          getTime(),
          `📐 [${this.deviceId}] Video ready: ${videoWidth}x${videoHeight}`
        );

        // 启动视频尺寸监控
        this.startVideoSizeMonitoring();

        // 应用旋转和尺寸设置
        this.applyRotationAndSize();

        // 激活鼠标和键盘控制器
        if (mouseController) {
          mouseController.setActiveDevice();
        }
      }
    };

    this.elements.remoteVideo.addEventListener(
      "loadedmetadata",
      checkVideoReady,
      { once: true }
    );
    setTimeout(checkVideoReady, 1000); // 兜底检查
  }

  // === 视频尺寸监控相关方法 ===

  /**
   * 启动视频尺寸变化监听
   */
  startVideoSizeMonitoring() {
    // 如果已经在监听，先停止
    this.stopVideoSizeMonitoring();

    console.log(getTime(), `📹 [${this.deviceId}] 启动视频尺寸变化监听`);

    // 监听关键事件
    this.elements.remoteVideo.addEventListener(
      "loadedmetadata",
      this.onVideoMetadataLoaded.bind(this)
    );
    this.elements.remoteVideo.addEventListener(
      "resize",
      this.onVideoResize.bind(this)
    );

    // 监听窗口大小变化
    if (!this.boundOnWindowResize) {
      this.boundOnWindowResize = this.onWindowResize.bind(this);
    }
    window.addEventListener("resize", this.boundOnWindowResize);

    // 定期检查尺寸变化
    this.videoSizeMonitorInterval = setInterval(() => {
      this.checkVideoSizeChanges();
    }, 1000);

    // 立即检查一次
    setTimeout(() => this.checkVideoSizeChanges(), 100);
  }

  /**
   * 停止视频尺寸变化监听
   */
  stopVideoSizeMonitoring() {
    if (this.videoSizeMonitorInterval) {
      clearInterval(this.videoSizeMonitorInterval);
      this.videoSizeMonitorInterval = null;
      console.log(getTime(), `⏹️ [${this.deviceId}] 停止视频尺寸变化监听`);
    }

    // 移除窗口大小变化监听
    if (this.boundOnWindowResize) {
      window.removeEventListener("resize", this.boundOnWindowResize);
      this.boundOnWindowResize = null;
    }
  }

  /**
   * 检查视频尺寸变化
   */
  checkVideoSizeChanges() {
    const currentVideoWidth = this.elements.remoteVideo.videoWidth || 0;
    const currentVideoHeight = this.elements.remoteVideo.videoHeight || 0;
    const currentClientWidth = this.elements.remoteVideo.clientWidth || 0;
    const currentClientHeight = this.elements.remoteVideo.clientHeight || 0;

    let hasChanges = false;
    let shouldResizeVideo = false;

    // 检查原始视频尺寸变化
    if (
      currentVideoWidth !== this.lastVideoWidth ||
      currentVideoHeight !== this.lastVideoHeight
    ) {
      console.log(getTime(), `📺 [${this.deviceId}] 原始视频尺寸变化:`);
      console.log(
        getTime(),
        `  从: ${this.lastVideoWidth}x${this.lastVideoHeight}`
      );
      console.log(
        getTime(),
        `  到: ${currentVideoWidth}x${currentVideoHeight}`
      );

      if (this.lastVideoWidth > 0 && this.lastVideoHeight > 0) {
        const oldAspectRatio = this.lastVideoWidth / this.lastVideoHeight;
        const newAspectRatio = currentVideoWidth / currentVideoHeight;
        console.log(
          getTime(),
          `  宽高比变化: ${oldAspectRatio.toFixed(
            3
          )} → ${newAspectRatio.toFixed(3)}`
        );

        // 宽高比变化超过阈值时，清理矩阵缓存
        if (Math.abs(oldAspectRatio - newAspectRatio) > 0.1) {
          console.log(
            getTime(),
            `🔧 [${this.deviceId}] 宽高比显著变化，清理矩阵缓存`
          );
          if (
            window.multiDeviceMouseController &&
            window.multiDeviceMouseController.matrixCache
          ) {
            window.multiDeviceMouseController.matrixCache.clear();
          }
          shouldResizeVideo = true;
        }
      }

      // 首次获取视频尺寸时也需要设置尺寸
      if (this.lastVideoWidth === 0 && this.lastVideoHeight === 0) {
        shouldResizeVideo = true;
      }

      this.lastVideoWidth = currentVideoWidth;
      this.lastVideoHeight = currentVideoHeight;
      hasChanges = true;
    }

    // 检查显示尺寸变化
    if (
      currentClientWidth !== this.lastClientWidth ||
      currentClientHeight !== this.lastClientHeight
    ) {
      console.log(getTime(), `🖼️ [${this.deviceId}] 显示尺寸变化:`);
      console.log(
        getTime(),
        `  从: ${this.lastClientWidth}x${this.lastClientHeight}`
      );
      console.log(
        getTime(),
        `  到: ${currentClientWidth}x${currentClientHeight}`
      );

      this.lastClientWidth = currentClientWidth;
      this.lastClientHeight = currentClientHeight;
      hasChanges = true;
    }

    // 当视频尺寸变化时，重新应用旋转和尺寸设置
    if (shouldResizeVideo && currentVideoWidth > 0 && currentVideoHeight > 0) {
      this.applyRotationAndSize();
    }

    // 如果有变化，触发自定义事件
    if (hasChanges && currentVideoWidth > 0 && currentVideoHeight > 0) {
      const event = new CustomEvent("videoSizeChanged", {
        detail: {
          deviceId: this.deviceId,
          videoWidth: currentVideoWidth,
          videoHeight: currentVideoHeight,
          clientWidth: currentClientWidth,
          clientHeight: currentClientHeight,
          aspectRatio: currentVideoWidth / currentVideoHeight,
          isLandscape: currentVideoWidth > currentVideoHeight
        }
      });
      this.elements.remoteVideo.dispatchEvent(event);
    }
  }

  /**
   * 视频元数据加载事件处理
   */
  onVideoMetadataLoaded() {
    console.log(getTime(), `🎬 [${this.deviceId}] loadedmetadata 事件触发`);
    setTimeout(() => this.checkVideoSizeChanges(), 100);
  }

  /**
   * 视频尺寸调整事件处理
   */
  onVideoResize() {
    console.log(getTime(), `🔄 [${this.deviceId}] resize 事件触发`);
    setTimeout(() => this.checkVideoSizeChanges(), 100);
  }

  /**
   * 窗口大小变化事件处理
   */
  onWindowResize() {
    console.log(
      getTime(),
      `🪟 [${this.deviceId}] 窗口大小变化，重新调整视频尺寸`
    );
    setTimeout(() => {
      if (
        this.elements.remoteVideo.videoWidth > 0 &&
        this.elements.remoteVideo.videoHeight > 0
      ) {
        this.applyRotationAndSize();
      }
    }, 100);
  }

  /**
   * 处理远程流移除
   */
  handleRemoteStreamRemoved(event) {
    console.log(getTime(), `📺 [${this.deviceId}] Remote stream removed`);
    this.clearVideoDisplay();
  }

  /**
   * 处理远程挂断
   */
  handleRemoteHangup() {
    this.destroyPeerConnection();
    this.clearVideoDisplay();
  }

  /**
   * 处理App连接失败
   */
  handleAppConnectionFailed(data) {
    const errorMessage =
      data.message || "云手机当前无法连接，请检查云手机状态或稍后再试！";
    console.error(`❌ [${this.deviceId}] ${errorMessage}`);

    this.emit("onError", { type: "app-connection", message: errorMessage });

    // 清理连接状态
    this.destroyPeerConnection();
    this.clearVideoDisplay();

    if (this.dataWebSocket) {
      this.dataWebSocket.close();
      this.dataWebSocket = null;
    }

    this.isConnected = false;
    this.updateConnectionStatus(false, "App连接失败");
    this.handleConnectionFailed("App连接失败");
    this.emit("onConnectionStateChange", {
      connected: false,
      status: "App连接失败"
    });
  }

  /**
   * 处理用户离开
   */
  handleUserLeft(message) {
    const clientType = message.clientType;
    const fromUser = message.from;

    console.log(
      getTime(),
      `👋 [${this.deviceId}] User left - ClientType: ${clientType}, From: ${fromUser}`
    );

    if (clientType === "app") {
      console.log(
        getTime(),
        `📱 [${this.deviceId}] App user left, stopping stream`
      );

      this.handleRemoteHangup();

      if (this.dataWebSocket) {
        this.dataWebSocket.close();
        this.dataWebSocket = null;
      }

      this.isConnected = false;
      this.updateConnectionStatus(false, "App已离开");
      this.emit("onConnectionStateChange", {
        connected: false,
        status: "App已离开"
      });

      // 清理会话信息
      this.androidSessionId = null;
      this.realSessionId = null;
      this.token = null;
      this.turnServerUrl = null;
    }
  }

  /**
   * 设置DataChannel监听器
   */
  setupDataChannelListeners() {
    if (this.pc) {
      this.pc.ondatachannel = event => {
        this.dataChannel = event.channel;
        console.log(
          getTime(),
          `📡 [${this.deviceId}] DataChannel opened: ${this.dataChannel.label}`
        );

        this.dataChannel.onopen = () => {
          console.log(
            getTime(),
            `✅ [${this.deviceId}] DataChannel state: ${this.dataChannel.readyState}`
          );
          this.updateDataChannelStatus(true, "数据通道已连接");

          // 设置全局变量供鼠标控制器使用
          window[`dataChannel_${this.deviceId}`] = this.dataChannel;
          console.log(
            getTime(),
            `🔧 [${this.deviceId}] DataChannel set to global variable: dataChannel_${this.deviceId}`
          );
        };

        this.dataChannel.onclose = () => {
          console.log(getTime(), `❌ [${this.deviceId}] DataChannel closed`);
          this.updateDataChannelStatus(false, "数据通道已断开");

          // 清理全局变量
          window[`dataChannel_${this.deviceId}`] = null;
          console.log(
            getTime(),
            `🔧 [${this.deviceId}] DataChannel global variable cleared`
          );
        };

        this.dataChannel.onmessage = event => {
          this.handleDataChannelMessage(JSON.parse(event.data));
        };
      };
    }
  }

  /**
   * 处理DataChannel消息
   */
  handleDataChannelMessage(message) {
    console.log(
      getTime(),
      `✅  [${this.deviceId}] DataChannel message:`,
      message
    );
    switch (message.type) {
      case "clipboard_content":
        this.handleClipboardContent(message.content);
        break;
      case "clipboard-set-result":
        this.handleClipboardSetResult(message.success, message.error);
        break;
      case "camera_control":
        this.handleCameraControl(message);
        break;
      case "firstConnect":
        console.log(getTime(), `🎬 [${this.deviceId}] 首次连接`);
        // 触发全局回调，通知Vue组件启用摄像头功能
        if (window.CphoneWebRTC && typeof window.CphoneWebRTC.onFirstConnect === "function") {
          window.CphoneWebRTC.onFirstConnect(this.deviceId);
        }
        break;

      default:
        console.log(
          getTime(),
          `📨 [${this.deviceId}] DataChannel message:`,
          message
        );
    }
  }
  /**处理相机控制消息 */
  async handleCameraControl(message) {
    const { action, cameraId } = message;
    console.log(
      getTime(),
      `📷 [${this.deviceId}] 收到相机控制消息: ${action}, cameraId: ${cameraId}`
    );
    try {
      if (action === "OPEN_CAMERA") {
        // 系统相机已打开，自动打开Web端摄像头
        if (!this.cameraEnabled) {
          console.log(
            getTime(),
            `📷 [${this.deviceId}] 响应系统相机打开，自动启用Web摄像头...`
          );
          await this.openCamera();
          console.log(getTime(), `✅ [${this.deviceId}] Web摄像头已自动启用`);
        } else {
          console.log(
            getTime(),
            `ℹ️ [${this.deviceId}] Web摄像头已经处于开启状态`
          );
        }
      } else if (action === "CLOSE_CAMERA") {
        // 系统相机已关闭，自动关闭Web端摄像头
        if (this.cameraEnabled) {
          console.log(
            getTime(),
            `📷 [${this.deviceId}] 响应系统相机关闭，自动关闭Web摄像头...`
          );
          this.closeCamera();
          console.log(getTime(), `✅ [${this.deviceId}] Web摄像头已自动关闭`);
        } else {
          console.log(
            getTime(),
            `ℹ️ [${this.deviceId}] Web摄像头已经处于关闭状态`
          );
        }
      } else {
        console.warn(
          getTime(),
          `⚠️ [${this.deviceId}] 未知的相机控制操作: ${action}`
        );
      }
    } catch (error) {
      console.error(
        getTime(),
        `❌ [${this.deviceId}] 处理相机控制消息失败:`,
        error
      );
    }
  }

  // ==================== 摄像头功能 ====================

  /**
   * 添加占位符摄像头轨道
   * 在初始连接时调用，预留轨道避免后续重协商
   * 使用静默黑屏轨道（流量极小，约 1-5 Kbps）
   */
  addPlaceholderCameraTrack() {
    try {
      // 如果摄像头已开启，直接添加真实轨道
      if (this.cameraEnabled && this.cameraTrack) {
        this.cameraSender = this.pc.addTrack(
          this.cameraTrack,
          this.localCameraStream
        );
        console.log(
          getTime(),
          `📷 [${this.deviceId}] 已添加真实摄像头轨道（摄像头已开启）`
        );
        return;
      }

      // 创建一个静默的黑屏 Canvas
      const canvas = document.createElement("canvas");
      canvas.width = 16; // 极小分辨率
      canvas.height = 16;
      const ctx = canvas.getContext("2d");
      ctx.fillStyle = "black";
      ctx.fillRect(0, 0, 16, 16);

      // 获取 MediaStream（1fps，极低码率）
      const silentStream = canvas.captureStream(1);
      const silentTrack = silentStream.getVideoTracks()[0];

      // 保存占位符轨道，供后续关闭摄像头时使用
      this.placeholderCameraTrack = silentTrack;
      this.placeholderCameraStream = silentStream;

      // 添加到 PeerConnection
      this.cameraSender = this.pc.addTrack(silentTrack, silentStream);

      console.log(
        getTime(),
        `🎬 [${this.deviceId}] 已预留摄像头轨道（静默黑屏，流量约 1-5 Kbps）`
      );
      console.log(
        getTime(),
        `   这样 Android 端可以在初次连接时收到 onAddTrack 回调`
      );
    } catch (error) {
      console.error(
        getTime(),
        `❌ [${this.deviceId}] 添加占位符轨道失败:`,
        error
      );
    }
  }

  /**
   * 构建 binary_pcm 协议数据包
   * 协议：前缀10字节 + 头部24字节 + payload(s16le PCM)
   * @param {Float32Array} float32Samples - Float32 采样（-1~1），单声道为 [s0,s1,...]，立体声为交织 [L0,R0,L1,R1,...]
   * @param {number} sampleRate - 实际采样率（从 AudioContext.sampleRate 获取）
   * @param {number} channels - 实际通道数（从 input.numberOfChannels 获取，1=单声道 2=立体声）
   * @returns {ArrayBuffer} 完整数据包
   */
  buildBinaryPcmPacket(float32Samples, sampleRate, channels) {
    const PCM_PREFIX = "binary_pcm";
    const PCM_MAGIC = 0x50434d31; // "PCM1"
    const HEADER_LEN = 24;
    const SAMPLE_FORMAT = 1; // s16le

    const sampleCount = float32Samples.length;
    const payloadLen = sampleCount * 2; // s16le = 2 bytes per sample

    const packetSize = 10 + HEADER_LEN + payloadLen;
    const buffer = new ArrayBuffer(packetSize);
    const view = new DataView(buffer);
    let offset = 0;

    // 1. 前缀 10 字节 ASCII "binary_pcm"
    for (let i = 0; i < PCM_PREFIX.length; i++) {
      view.setUint8(offset++, PCM_PREFIX.charCodeAt(i));
    }

    // 2. 头部 24 字节（小端）
    view.setUint32(offset, PCM_MAGIC, true);
    offset += 4;
    view.setUint8(offset++, 1); // version
    view.setUint8(offset++, 0); // flags
    view.setUint16(offset, HEADER_LEN, true);
    offset += 2;
    view.setUint32(offset, this.audioPcmSeq++, true);
    offset += 4;
    view.setUint32(offset, Date.now() % 0xffffffff, true);
    offset += 4;
    view.setUint16(offset, sampleRate, true);
    offset += 2;
    view.setUint8(offset++, channels);
    view.setUint8(offset++, SAMPLE_FORMAT);
    view.setUint32(offset, payloadLen, true);
    offset += 4;

    // 3. Payload: Float32 -> s16le
    for (let i = 0; i < sampleCount; i++) {
      const s = Math.max(-1, Math.min(1, float32Samples[i]));
      const s16 = s < 0 ? s * 0x8000 : s * 0x7fff;
      view.setInt16(offset, s16, true);
      offset += 2;
    }

    return buffer;
  }

  /**
   * 启动音频采集并通过 DataChannel 发送 PCM（binary_pcm 协议）
   * @param {MediaStream} stream - 包含音频轨道的流
   */
  startAudioCapture(stream) {
    const audioTracks = stream.getAudioTracks();
    if (audioTracks.length === 0) {
      console.warn(
        getTime(),
        `⚠️ [${this.deviceId}] 流中无音频轨道，跳过音频采集`
      );
      return;
    }

    if (!this.dataChannel || this.dataChannel.readyState !== "open") {
      console.warn(
        getTime(),
        `⚠️ [${this.deviceId}] DataChannel 未就绪，无法发送音频`
      );
      return;
    }

    try {
      // 使用系统默认采样率，不写死 48000
      this.audioContext = new (window.AudioContext ||
        window.webkitAudioContext)();

      this.audioSourceNode = this.audioContext.createMediaStreamSource(stream);

      // 使用 ScriptProcessorNode 获取原始 PCM（bufferSize 1024 ≈ 21ms @ 48k）
      const bufferSize = 1024;
      this.audioProcessorNode = this.audioContext.createScriptProcessor(
        bufferSize,
        2, // 输入可能为立体声
        2 // 输出保持原通道数（1 或 2）
      );

      this.audioProcessorNode.onaudioprocess = e => {
        if (!this.dataChannel || this.dataChannel.readyState !== "open") return;
        if (!this.cameraEnabled) return;

        const input = e.inputBuffer;
        const inputChannels = input.numberOfChannels;
        const inputLength = input.length;

        // 根据真实通道数处理：单声道直接传递，立体声交织 L/R 传递
        let samples;
        if (inputChannels === 1) {
          samples = new Float32Array(inputLength);
          samples.set(input.getChannelData(0));
        } else {
          const L = input.getChannelData(0);
          const R = input.getChannelData(1);
          samples = new Float32Array(inputLength * 2);
          for (let i = 0; i < inputLength; i++) {
            samples[i * 2] = L[i];
            samples[i * 2 + 1] = R[i];
          }
        }

        // 使用真实的采样率和通道数
        const sampleRate = this.audioContext.sampleRate;
        const channels = inputChannels;
        const packet = this.buildBinaryPcmPacket(samples, sampleRate, channels);
        this.dataChannel.send(packet);
      };

      this.audioSourceNode.connect(this.audioProcessorNode);
      // 连接到静默目标，避免本地扬声器播放产生回声
      const silentDest = this.audioContext.createMediaStreamDestination();
      this.audioProcessorNode.connect(silentDest);

      const actualSampleRate = this.audioContext.sampleRate;
      let actualChannels = 2;
      try {
        const settings = stream.getAudioTracks()[0].getSettings();
        if (settings.channelCount) actualChannels = settings.channelCount;
      } catch (_) {}
      console.log(
        getTime(),
        `🎤 [${this.deviceId}] 音频采集已启动，通过 DataChannel 发送 PCM (${actualSampleRate}Hz ${actualChannels}ch s16le)`
      );
    } catch (error) {
      console.error(
        getTime(),
        `❌ [${this.deviceId}] 启动音频采集失败:`,
        error
      );
    }
  }

  /**
   * 停止音频采集
   */
  stopAudioCapture() {
    try {
      if (this.audioProcessorNode) {
        this.audioProcessorNode.disconnect();
        this.audioProcessorNode.onaudioprocess = null;
        this.audioProcessorNode = null;
      }
      if (this.audioSourceNode) {
        this.audioSourceNode.disconnect();
        this.audioSourceNode = null;
      }
      if (this.audioContext) {
        this.audioContext.close();
        this.audioContext = null;
      }
      console.log(getTime(), `🎤 [${this.deviceId}] 音频采集已停止`);
    } catch (error) {
      console.warn(
        getTime(),
        `⚠️ [${this.deviceId}] 停止音频采集时出错:`,
        error
      );
    }
  }

  /**
   * 获取摄像头设备列表
   * @returns {Promise<Array>} 摄像头设备列表
   */
  async getCameraList() {
    try {
      // 检查浏览器支持
      if (!navigator.mediaDevices || !navigator.mediaDevices.enumerateDevices) {
        throw new Error("浏览器不支持设备枚举功能");
      }

      // 先尝试不请求权限获取设备列表（避免摄像头短暂开启）
      let devices = await navigator.mediaDevices.enumerateDevices();
      const cameras = devices.filter(item => item.kind === "videoinput");

      // 检查是否有设备标签，如果没有标签则需要请求权限
      const hasLabels = cameras.some(
        device => device.label && device.label.trim() !== ""
      );

      let stream = null;
      // 只有在没有标签的情况下才请求权限
      if (!hasLabels && cameras.length > 0) {
        try {
          // 请求权限以获取设备标签（否则设备名可能是空的）
          stream = await navigator.mediaDevices.getUserMedia({
            video: true,
            audio: true
          });
          // 重新枚举设备以获取标签
          devices = await navigator.mediaDevices.enumerateDevices();
          const camerasWithLabels = devices.filter(
            item => item.kind === "videoinput"
          );

          // 停止媒体流（释放摄像头）
          if (stream) {
            stream.getTracks().forEach(track => track.stop());
            stream = null;
          }

          // 使用带标签的设备列表
          const cameraList = camerasWithLabels.map((device, index) => ({
            id: device.deviceId || `camera-${index}`,
            deviceId: device.deviceId,
            name: device.label || `摄像头 ${index + 1}`,
            label: device.label || "",
            kind: device.kind,
            groupId: device.groupId
          }));

          console.log(
            getTime(),
            `📷 [${this.deviceId}] 获取到 ${cameraList.length} 个摄像头设备（已获取标签）`
          );

          return cameraList;
        } catch (err) {
          // 即使权限被拒绝，也可以尝试获取设备列表（但可能没有标签）
          if (err.name === "NotFoundError") {
            throw new Error("未找到摄像头设备，请检查设备连接");
          }
          // 权限被拒绝时，继续使用之前获取的列表（可能没有标签）
          console.warn(
            getTime(),
            `⚠️ [${this.deviceId}] 无法获取设备标签（权限被拒绝），使用无标签的设备列表`
          );
        }
      }

      // 处理设备信息（使用已有标签或默认名称）
      const cameraList = cameras.map((device, index) => ({
        id: device.deviceId || `camera-${index}`,
        deviceId: device.deviceId,
        name: device.label || `摄像头 ${index + 1}`,
        label: device.label || "",
        kind: device.kind,
        groupId: device.groupId
      }));

      // 确保停止媒体流（释放摄像头）
      if (stream) {
        stream.getTracks().forEach(track => track.stop());
      }

      console.log(
        getTime(),
        `📷 [${this.deviceId}] 获取到 ${cameraList.length} 个摄像头设备`
      );

      return cameraList;
    } catch (error) {
      console.error(
        getTime(),
        `❌ [${this.deviceId}] 获取摄像头列表失败:`,
        error
      );
      throw error;
    }
  }

  /**
   * 创建镜像翻转的视频流
   * @param {MediaStream} originalStream - 原始视频流
   * @param {boolean} enableMirror - 是否启用镜像翻转（默认使用 this.cameraMirrorEnabled）
   * @returns {MediaStream} 翻转后的视频流（如果禁用翻转则返回原始流）
   */
  createMirroredStream(originalStream, enableMirror = null) {
    // 如果未指定 enableMirror，使用实例属性
    const shouldMirror =
      enableMirror !== null ? enableMirror : this.cameraMirrorEnabled;

    // 如果不需要翻转，直接返回原始流
    if (!shouldMirror) {
      console.log(
        getTime(),
        `📷 [${this.deviceId}] 摄像头镜像已禁用，使用原始视频流`
      );
      return originalStream;
    }
    const video = document.createElement("video");
    video.srcObject = originalStream;
    video.autoplay = true;
    video.playsInline = true;
    video.muted = true; // 静音以避免音频反馈

    const canvas = document.createElement("canvas");
    const ctx = canvas.getContext("2d");

    // 设置 canvas 尺寸并开始绘制
    const setupCanvas = () => {
      if (video.videoWidth > 0 && video.videoHeight > 0) {
        canvas.width = video.videoWidth;
        canvas.height = video.videoHeight;
        console.log(
          getTime(),
          `🪞 [${this.deviceId}] Canvas 尺寸设置为: ${canvas.width}x${canvas.height}`
        );

        // 开始绘制循环
        if (!this.mirrorDrawingActive) {
          this.mirrorDrawingActive = true;
          drawFrame();
        }
      }
    };

    // 绘制翻转后的视频帧
    const drawFrame = () => {
      if (
        this.mirrorDrawingActive &&
        video.readyState >= 2 &&
        canvas.width > 0 &&
        canvas.height > 0
      ) {
        // 保存当前上下文
        ctx.save();

        // 水平翻转：先移动到右侧，然后水平翻转
        ctx.translate(canvas.width, 0);
        ctx.scale(-1, 1);

        // 绘制视频帧
        ctx.drawImage(video, 0, 0, canvas.width, canvas.height);

        // 恢复上下文
        ctx.restore();
      }
      if (this.mirrorDrawingActive) {
        requestAnimationFrame(drawFrame);
      }
    };

    // 监听视频元数据加载
    video.addEventListener("loadedmetadata", setupCanvas);
    video.addEventListener("loadeddata", setupCanvas);

    // 从 canvas 捕获流（使用原始流的帧率）
    const videoTrack = originalStream.getVideoTracks()[0];
    const settings = videoTrack.getSettings();
    const frameRate = settings.frameRate || this.fps || 30;

    const mirroredStream = canvas.captureStream(frameRate);

    // 复制音频轨道（如果有）
    const audioTracks = originalStream.getAudioTracks();
    audioTracks.forEach(track => {
      mirroredStream.addTrack(track);
    });

    // 保存 video 和 canvas 引用以便后续清理
    this.mirrorVideoElement = video;
    this.mirrorCanvasElement = canvas;

    // 确保 video 开始播放
    video.play().catch(err => {
      console.warn(getTime(), `⚠️ [${this.deviceId}] 镜像视频播放失败:`, err);
    });

    console.log(
      getTime(),
      `🪞 [${this.deviceId}] 镜像流创建完成，帧率: ${frameRate}fps`
    );

    return mirroredStream;
  }

  /**
   * 打开摄像头（通过 dataChannel 发送消息控制 Android 端摄像头）
   * @param {string} deviceId - 可选的摄像头设备ID（保留参数以兼容现有调用）
   * @returns {Promise<boolean>} 是否成功发送打开指令
   */
  async openCamera(deviceId = null) {
    try {
      console.log(
        getTime(),
        `📷 [${this.deviceId}] 正在打开摄像头...${deviceId ? " (指定设备: " + deviceId.substring(0, 20) + "...)" : ""}`
      );

      // 检查 dataChannel 是否可用
      if (!this.dataChannel || this.dataChannel.readyState !== "open") {
        throw new Error("数据通道未连接，无法发送摄像头控制指令");
      }

      // 检查 PeerConnection 和摄像头发送器是否存在
      if (!this.pc || !this.cameraSender) {
        throw new Error("WebRTC 连接未建立或摄像头轨道未初始化");
      }

      // 确定要使用的摄像头设备ID：优先使用传入的 deviceId，否则使用已保存的 selectedCameraDeviceId
      const targetDeviceId = deviceId || this.selectedCameraDeviceId;
      if (targetDeviceId) {
        this.selectedCameraDeviceId = targetDeviceId;
        console.log(
          getTime(),
          `📷 [${this.deviceId}] 使用摄像头设备ID: ${targetDeviceId.substring(0, 20)}...`
        );
      } else {
        console.warn(
          getTime(),
          `⚠️ [${this.deviceId}] 未指定摄像头设备ID，将使用默认摄像头`
        );
      }

      // 如果摄像头已经打开，先关闭旧的流
      if (this.localCameraStream) {
        this.localCameraStream.getTracks().forEach(track => track.stop());
        this.localCameraStream = null;
        this.cameraTrack = null;
      }

      // 🔥 实际打开浏览器摄像头
      // 使用手机连接时传入的宽高参数
      // 根据 this.width 和 this.height 计算目标宽高
      let targetWidth = 352;
      let targetHeight = 640;

      // 如果宽高是 720*1544 或 1080*2316，使用 290x618
      if (
        (this.width === 720 && this.height === 1544) ||
        (this.width === 1080 && this.height === 2316)
      ) {
        targetWidth = 290;
        targetHeight = 618;
      }

      const videoConstraints = targetDeviceId
        ? {
            deviceId: { exact: targetDeviceId },
            width: { ideal: targetWidth },
            height: { ideal: targetHeight },
            frameRate: { ideal: this.fps }
            // resizeMode: "crop-and-scale",
            // aspectRatio: { ideal: 9 / 16 }
          }
        : {
            width: { ideal: targetWidth },
            height: { ideal: targetHeight },
            frameRate: { ideal: this.fps }
          };

      const constraints = {
        video: videoConstraints,
        audio: true
      };

      const cameraStream =
        await navigator.mediaDevices.getUserMedia(constraints);
      const cameraTrack = cameraStream.getVideoTracks()[0];
      const settings = cameraTrack.getSettings();
      const actualWidth = settings.width;
      const actualHeight = settings.height;
      console.log(`实际获取到的分辨率: ${actualWidth}x${actualHeight}`);
      if (!cameraTrack) {
        throw new Error("无法获取摄像头视频轨道");
      }

      console.log(
        getTime(),
        `📷 [${this.deviceId}] 已获取浏览器摄像头流: ${cameraTrack.label || "未知设备"}`
      );

      // 🔥 处理镜像问题：根据配置决定是否翻转视频流
      const processedStream = this.createMirroredStream(cameraStream);
      const processedTrack = processedStream.getVideoTracks()[0];

      // 保存原始流和翻转后的流
      this.localCameraStream = cameraStream; // 保存原始流用于清理
      this.cameraTrack = processedTrack; // 使用处理后的轨道（可能翻转也可能不翻转）

      if (this.cameraMirrorEnabled) {
        console.log(
          getTime(),
          `🪞 [${this.deviceId}] 已创建镜像翻转后的视频流`
        );
      } else {
        console.log(
          getTime(),
          `📷 [${this.deviceId}] 使用原始视频流（未翻转）`
        );
      }

      // 🔥 使用 replaceTrack 替换占位符轨道为真实摄像头轨道（避免重协商）
      if (this.cameraSender && this.placeholderCameraTrack) {
        await this.cameraSender.replaceTrack(processedTrack);
        console.log(
          getTime(),
          `🔄 [${this.deviceId}] 已替换为真实摄像头轨道（已镜像翻转），无需重协商`
        );
      } else {
        // 如果没有占位符轨道，直接添加轨道（这种情况应该很少见）
        this.cameraSender = this.pc.addTrack(processedTrack, processedStream);
        console.log(
          getTime(),
          `➕ [${this.deviceId}] 已添加摄像头轨道（新轨道，已镜像翻转）`
        );
      }

      // 通过 dataChannel 发送打开摄像头指令到 Android 端
      const cameraControlMessage = {
        type: "camera_control",
        action: "open"
      };

      this.dataChannel.send(JSON.stringify(cameraControlMessage));
      console.log(
        getTime(),
        `✅ [${this.deviceId}] 已发送打开摄像头指令到 Android 端`
      );

      // 启动音频采集，通过 DataChannel 发送 PCM（binary_pcm 协议）
      this.startAudioCapture(cameraStream);

      // 更新本地状态
      this.cameraEnabled = true;

      console.log(getTime(), `✅ [${this.deviceId}] Web摄像头已自动启用`);

      return true;
    } catch (error) {
      console.error(getTime(), `❌ [${this.deviceId}] 打开摄像头失败:`, error);

      // 清理失败的流
      if (this.localCameraStream) {
        this.localCameraStream.getTracks().forEach(track => track.stop());
        this.localCameraStream = null;
        this.cameraTrack = null;
      }

      // 清理镜像处理相关的元素
      if (this.mirrorVideoElement) {
        this.mirrorVideoElement.srcObject = null;
        this.mirrorVideoElement = null;
      }
      if (this.mirrorCanvasElement) {
        this.mirrorCanvasElement = null;
      }

      this.cameraEnabled = false;
      throw new Error(`打开摄像头失败: ${error.message}`);
    }
  }

  /**
   * 关闭摄像头
   */
  async closeCamera() {
    console.log(getTime(), `📷 [${this.deviceId}] 正在关闭摄像头...`);

    // 停止音频采集（DataChannel PCM 传输）
    this.stopAudioCapture();

    // 🔥 使用 replaceTrack 替换回黑屏轨道（避免重协商，流量极小）
    if (this.pc && this.cameraSender && this.placeholderCameraTrack) {
      try {
        await this.cameraSender.replaceTrack(this.placeholderCameraTrack);
        console.log(
          getTime(),
          `🔄 [${this.deviceId}] 已替换回静默黑屏轨道，无需重协商`
        );
      } catch (error) {
        console.error(
          getTime(),
          `❌ [${this.deviceId}] 替换黑屏轨道失败:`,
          error
        );
      }
    }

    // 停止真实摄像头轨道
    if (this.localCameraStream) {
      try {
        const tracks = this.localCameraStream.getTracks();
        tracks.forEach(track => {
          if (track.readyState !== "ended") {
            track.stop();
            console.log(
              getTime(),
              `🛑 [${this.deviceId}] 停止轨道: ${track.kind} - ${track.label || "未知"}`
            );
          } else {
            console.log(
              getTime(),
              `ℹ️ [${this.deviceId}] 轨道已结束: ${track.kind} - ${track.label || "未知"}`
            );
          }
        });
        this.localCameraStream = null;
      } catch (error) {
        console.error(
          getTime(),
          `❌ [${this.deviceId}] 停止摄像头轨道时出错:`,
          error
        );
        // 即使出错也清空引用
        this.localCameraStream = null;
      }
    } else {
      console.warn(
        getTime(),
        `⚠️ [${this.deviceId}] localCameraStream 为空，可能摄像头未打开或已被关闭`
      );
    }

    // 如果 cameraTrack 存在但不在 localCameraStream 中，也尝试停止它
    if (this.cameraTrack && this.cameraTrack.readyState !== "ended") {
      try {
        this.cameraTrack.stop();
        console.log(
          getTime(),
          `🛑 [${this.deviceId}] 停止 cameraTrack: ${this.cameraTrack.label || "未知"}`
        );
      } catch (error) {
        console.warn(
          getTime(),
          `⚠️ [${this.deviceId}] 停止 cameraTrack 失败:`,
          error
        );
      }
    }

    // 清理镜像处理相关的元素
    this.mirrorDrawingActive = false; // 停止绘制循环
    if (this.mirrorVideoElement) {
      try {
        this.mirrorVideoElement.pause();
        this.mirrorVideoElement.srcObject = null;
        this.mirrorVideoElement = null;
      } catch (error) {
        console.warn(
          getTime(),
          `⚠️ [${this.deviceId}] 清理 mirrorVideoElement 失败:`,
          error
        );
      }
    }
    if (this.mirrorCanvasElement) {
      try {
        const ctx = this.mirrorCanvasElement.getContext("2d");
        ctx.clearRect(
          0,
          0,
          this.mirrorCanvasElement.width,
          this.mirrorCanvasElement.height
        );
        this.mirrorCanvasElement = null;
      } catch (error) {
        console.warn(
          getTime(),
          `⚠️ [${this.deviceId}] 清理 mirrorCanvasElement 失败:`,
          error
        );
      }
    }

    this.cameraTrack = null;
    this.cameraEnabled = false;

    console.log(getTime(), `✅ [${this.deviceId}] 摄像头已关闭`);
  }

  /**
   * 切换摄像头镜像翻转状态
   * @param {boolean} enabled - 是否启用镜像翻转（可选，不传则切换当前状态）
   * @returns {Promise<boolean>} 是否成功切换
   */
  async toggleCameraMirror(enabled = null) {
    try {
      const newState = enabled !== null ? enabled : !this.cameraMirrorEnabled;

      // 如果状态没有变化，直接返回
      if (newState === this.cameraMirrorEnabled) {
        console.log(
          getTime(),
          `📷 [${this.deviceId}] 摄像头镜像状态已经是 ${newState ? "开启" : "关闭"}`
        );
        return true;
      }

      this.cameraMirrorEnabled = newState;
      console.log(
        getTime(),
        `🔄 [${this.deviceId}] 摄像头镜像已${newState ? "开启" : "关闭"}`
      );

      // 如果摄像头正在推流，需要重新打开以应用新的翻转设置
      if (this.cameraEnabled && this.localCameraStream) {
        console.log(
          getTime(),
          `🔄 [${this.deviceId}] 摄像头正在推流，重新打开以应用新的镜像设置...`
        );

        // 保存当前选中的摄像头设备ID
        const currentDeviceId = this.selectedCameraDeviceId;

        // 先关闭当前摄像头
        await this.closeCamera();

        // 重新打开摄像头（使用新的镜像设置）
        await this.openCamera(currentDeviceId);

        console.log(
          getTime(),
          `✅ [${this.deviceId}] 摄像头已重新打开，镜像设置已应用`
        );
      }

      return true;
    } catch (error) {
      console.error(
        getTime(),
        `❌ [${this.deviceId}] 切换摄像头镜像失败:`,
        error
      );
      throw error;
    }
  }

  /**
   * 获取摄像头镜像状态
   * @returns {boolean} 是否启用镜像翻转
   */
  getCameraMirrorStatus() {
    return this.cameraMirrorEnabled;
  }

  /**
   * 切换摄像头
   * @param {string} deviceId - 摄像头设备ID
   * @returns {Promise<boolean>} 是否成功切换
   */
  async switchToCamera(deviceId) {
    try {
      console.log(
        getTime(),
        `🔄 [${this.deviceId}] 切换到摄像头: ${deviceId.substring(0, 20)}...`
      );

      // 保存选中的摄像头设备ID
      this.selectedCameraDeviceId = deviceId;

      // 判断当前摄像头是否正在推流
      if (this.cameraEnabled) {
        // 如果正在推流，立即关闭旧流并打开新流
        console.log(
          getTime(),
          `📹 [${this.deviceId}] 摄像头正在推流，立即切换到新摄像头`
        );

        // 先关闭当前摄像头
        await this.closeCamera();

        // 打开新摄像头（使用刚保存的 deviceId）
        await this.openCamera(deviceId);

        console.log(
          getTime(),
          `✅ [${this.deviceId}] 已切换到新摄像头并保持推流状态`
        );
      } else {
        // 如果未推流，只保存选中的摄像头ID，等待 openCamera 调用时使用
        console.log(
          getTime(),
          `💾 [${this.deviceId}] 摄像头未推流，已保存选中的摄像头ID，等待开启时使用`
        );
      }

      return true;
    } catch (error) {
      console.error(getTime(), `❌ [${this.deviceId}] 切换摄像头失败:`, error);
      throw error;
    }
  }

  /**
   * 智能选择最佳摄像头（避免红外摄像头）
   * @returns {Promise<string|null>} 摄像头设备ID
   */
  async selectBestCamera() {
    try {
      // 先请求权限
      const tempStream = await navigator.mediaDevices.getUserMedia({
        video: true
      });
      tempStream.getTracks().forEach(t => t.stop()); // 立即停止

      // 获取所有设备
      const devices = await navigator.mediaDevices.enumerateDevices();
      const videoDevices = devices.filter(
        device => device.kind === "videoinput"
      );

      console.log(
        getTime(),
        `🔍 [${this.deviceId}] 检测到 ${videoDevices.length} 个摄像头设备`
      );

      if (videoDevices.length === 0) {
        console.warn(getTime(), `⚠️ [${this.deviceId}] 未检测到摄像头设备`);
        return null;
      }

      // 🔥 过滤掉红外摄像头
      const colorCameras = videoDevices.filter(device => {
        const label = device.label.toLowerCase();
        const isIR =
          label.includes("ir") ||
          label.includes("infrared") ||
          label.includes("depth");
        return !isIR;
      });

      if (colorCameras.length > 0) {
        const selected = colorCameras[0];
        console.log(
          getTime(),
          `✅ [${this.deviceId}] 自动选择彩色摄像头: ${selected.label}`
        );
        return selected.deviceId;
      } else {
        // 如果所有摄像头都是红外，选择第一个
        const selected = videoDevices[0];
        console.log(
          getTime(),
          `⚠️ [${this.deviceId}] 未找到彩色摄像头，使用默认: ${selected.label}`
        );
        return selected.deviceId;
      }
    } catch (error) {
      console.error(
        getTime(),
        `❌ [${this.deviceId}] 自动选择摄像头失败:`,
        error
      );
      return null;
    }
  }

  /**
   * 处理剪贴板内容
   */
  handleClipboardContent(content) {
    if (this.isMain && content && content.trim()) {
      console.log(
        getTime(),
        `📋 [${this.deviceId}] Received clipboard:`,
        content
      );

      // if (
      //   confirm(`获取到手机剪贴板内容:\n\n${content}\n\n是否复制到本地剪贴板?`)
      // ) {
      this.copyToLocalClipboard(content);
      // }
    }
  }

  /**
   * 处理剪贴板设置结果
   */
  handleClipboardSetResult(success, error) {
    if (success) {
      console.log(
        getTime(),
        `✅ [${this.deviceId}] Clipboard set successfully`
      );
    } else {
      console.error(`❌ [${this.deviceId}] Clipboard set failed:`, error);
    }
  }

  /**
   * 复制到本地剪贴板
   */
  async copyToLocalClipboard(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      try {
        await navigator.clipboard.writeText(text);
        console.log(
          getTime(),
          `✅ [${this.deviceId}] Copied to local clipboard`
        );
      } catch (error) {
        console.error(`❌ [${this.deviceId}] Clipboard API failed:`, error);
        this.fallbackCopyToClipboard(text);
      }
    } else {
      this.fallbackCopyToClipboard(text);
    }
  }

  /**
   * 备用复制方法
   */
  fallbackCopyToClipboard(text) {
    const textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.position = "fixed";
    textArea.style.left = "-999999px";
    textArea.style.top = "-999999px";
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
      document.execCommand("copy");
      console.log(getTime(), `✅ [${this.deviceId}] Fallback copy successful`);
    } catch (error) {
      console.error(`❌ [${this.deviceId}] Fallback copy failed:`, error);
    }

    document.body.removeChild(textArea);
  }

  /**
   * 处理连接状态变化
   */
  handleConnectionStateChange() {
    if (!this.pc) return;

    const state = this.pc.iceConnectionState;
    console.log(
      getTime(),
      `🔗 [${this.deviceId}] ICE connection state: ${state}`
    );

    switch (state) {
      case "connected":
      case "completed":
        this.updateConnectionStatus(true, "视频连接成功");
        break;
      case "disconnected":
      case "failed":
        this.updateConnectionStatus(false, "视频连接失败");
        this.handleConnectionFailed("视频连接失败");

        // 如果摄像头已开启，关闭摄像头
        if (this.cameraEnabled) {
          this.closeCamera().catch(err => {
            console.error(
              getTime(),
              `❌ [${this.deviceId}] 连接失败时关闭摄像头失败:`,
              err
            );
          });
        }

        //重置 pc 对象，下次连接时将创建新的
        // if (this.pc) {
        //   console.log(
        //     getTime(),
        //     `@[${this.deviceId}]重置 PeerConnection，下次连接将创建新的`
        //   );
        //   this.pc.close();
        //   this.pc = null;
        // }
        break;
      case "closed":
        this.updateConnectionStatus(false, "连接已关闭");

        // 如果摄像头已开启，关闭摄像头
        if (this.cameraEnabled) {
          this.closeCamera().catch(err => {
            console.error(
              getTime(),
              `❌ [${this.deviceId}] 连接关闭时关闭摄像头失败:`,
              err
            );
          });
        }
        break;
    }

    this.emit("onConnectionStateChange", {
      connected: state === "connected" || state === "completed",
      status: state
    });
  }

  /**
   * 更新连接状态显示
   */
  updateConnectionStatus(connected, status) {
    // 这个方法将由UI控制器实现
    // 或者通过事件回调通知外部组件
    console.log(
      getTime(),
      `📊 [${this.deviceId}] Connection status: ${
        connected ? "✅" : "❌"
      } ${status}`
    );
    if (connected) {
      window.CphoneWebRTC?.onConnectSuccess?.(
        this.deviceId,
        connected,
        status,
        this.cpId
      );
    }
  }
  /**
   * 抛出连接失败全局方法
   */
  handleConnectionFailed(message) {
    window.CphoneWebRTC?.onConnectFailed?.(this.deviceId, message, this.cpId);
  }

  /**
   * 更新DataChannel状态显示
   */
  updateDataChannelStatus(connected, status) {
    if (this.elements.dataChannelIcon) {
      this.elements.dataChannelIcon.textContent = connected ? "🟢" : "🔴";
    }
    if (this.elements.dataChannelStatus) {
      this.elements.dataChannelStatus.textContent = status;
    }
  }

  /**
   * 更新静音按钮状态
   */
  updateMuteButtonState() {
    if (this.elements.muteBtn) {
      this.elements.muteBtn.textContent = this.isMuted ? "🔇 静音" : "🔊 声音";
      this.elements.muteBtn.classList.toggle("muted", this.isMuted);
    }

    if (this.elements.remoteVideo) {
      this.elements.remoteVideo.muted = this.isMuted;
    }
  }

  /**
   * 清理视频显示
   */
  clearVideoDisplay() {
    this.stopVideoStats();

    // 如果摄像头已开启，先关闭
    if (this.cameraEnabled) {
      this.closeCamera().catch(err => {
        console.error(
          getTime(),
          `❌ [${this.deviceId}] 清理视频显示时关闭摄像头失败:`,
          err
        );
      });
    }

    if (this.elements.remoteVideo) {
      if (this.elements.remoteVideo.srcObject) {
        this.elements.remoteVideo.srcObject
          .getTracks()
          .forEach(track => track.stop());
      }
      this.elements.remoteVideo.srcObject = null;
    }

    this.resetStatsDisplay();
    this.remoteStream = null;

    // 清理DataChannel全局变量
    if (window[`dataChannel_${this.deviceId}`]) {
      window[`dataChannel_${this.deviceId}`] = null;
      console.log(
        getTime(),
        `🔧 [${this.deviceId}] DataChannel global variable cleared on video display cleanup`
      );
    }
  }

  /**
   * 应用视频旋转
   */
  /**
   * 统一的旋转和尺寸处理方法 - 集成视频监控逻辑
   */
  applyRotationAndSize() {
    if (!this.elements.remoteVideo) return;

    console.log(
      getTime(),
      `🔄 [${this.deviceId}] Applying rotation and size: ${this.rotation}°`
    );

    const videoContainer = this.elements.remoteVideo.parentElement;

    // 计算容器尺寸
    if (videoContainer) {
      const videoWidth = this.elements.remoteVideo.videoWidth || 0;
      const videoHeight = this.elements.remoteVideo.videoHeight || 0;
      const isVideoReady = videoWidth > 0 && videoHeight > 0;

      let containerWidth = "100%";
      let containerHeight = "500px";
      let containerMaxHeight = "600px";

      if (isVideoReady) {
        // 获取父容器的可用宽度
        const parentElement = videoContainer.parentElement;
        const parentRect = parentElement.getBoundingClientRect();
        const maxContainerWidth = parentRect.width || 400;

        console.log(
          getTime(),
          `📐 [${this.deviceId}] 容器计算 - 可用宽度: ${maxContainerWidth}px, 视频: ${videoWidth}x${videoHeight}`
        );

        if (
          this.rotation === 90 ||
          this.rotation === 270 ||
          this.rotation === -90
        ) {
          // 90°/270°旋转：实现宽高互换的视觉效果
          console.log(
            getTime(),
            `🔄 [${this.deviceId}] 处理90°/270°旋转 - 原始视频: ${videoWidth}x${videoHeight}`
          );

          // 计算0度时的实际显示尺寸
          const videoAspectRatio = videoWidth / videoHeight; // 720/1280 = 0.5625
          const displayHeight0deg = maxContainerWidth / videoAspectRatio; // 理论上0度时的显示高度

          // 旋转90度后，我们希望：
          // - 旋转后的视频宽度 ≈ 原来的显示高度
          // - 旋转后的视频高度 ≈ 原来的显示宽度

          // 为了实现这个效果，我们需要更大的容器高度
          // 旋转后视频的宽高比是 videoHeight/videoWidth = 1280/720 = 1.778
          // 要让旋转后的视频宽度接近原显示高度，容器高度需要更大

          // 方案：让旋转后的视频充分利用空间，接近宽高互换的效果
          const targetRotatedWidth = Math.min(
            displayHeight0deg * 0.8,
            maxContainerWidth * 0.9
          ); // 期望的旋转后宽度
          const rotatedAspectRatio = videoHeight / videoWidth; // 1.778
          let calculatedHeight = targetRotatedWidth / rotatedAspectRatio; // 反推需要的容器高度

          // 如果计算的高度太小，使用更大的值
          calculatedHeight = Math.max(
            calculatedHeight,
            maxContainerWidth * 0.6
          );
          calculatedHeight = Math.min(calculatedHeight, 800);

          containerHeight = `${calculatedHeight}px`;
          containerMaxHeight = `${calculatedHeight + 100}px`;

          console.log(
            getTime(),
            `🔄 [${
              this.deviceId
            }] 旋转90°/270°: 视频${videoWidth}x${videoHeight} -> 容器${maxContainerWidth}x${calculatedHeight} (目标旋转宽度${targetRotatedWidth.toFixed(
              0
            )})`
          );
        } else {
          // 0°/180°旋转：保持原有比例
          const videoAspectRatio = videoWidth / videoHeight;
          const calculatedHeight = Math.min(
            maxContainerWidth / videoAspectRatio,
            682
          );

          containerHeight = `${calculatedHeight}px`;
          containerMaxHeight = `${calculatedHeight + 50}px`;

          console.log(
            getTime(),
            `🔄 [${this.deviceId}] 旋转0°/180°: 视频${videoWidth}x${videoHeight} -> 容器高度${calculatedHeight}px`
          );
        }
      } else {
        // 视频尺寸未知时，使用默认值
        if (this.rotation === 90 || this.rotation === 270) {
          containerHeight = "600px";
          containerMaxHeight = "700px";
          console.log(getTime(), `🔄 [${this.deviceId}] 使用默认横屏旋转尺寸`);
        } else {
          containerHeight = "500px";
          containerMaxHeight = "550px";
          console.log(getTime(), `🔄 [${this.deviceId}] 使用默认竖屏尺寸`);
        }
      }

      // 应用计算出的容器尺寸
      videoContainer.style.width = containerWidth;
      // videoContainer.style.height = containerHeight;
      videoContainer.style.maxHeight = containerMaxHeight;

      // 确保容器样式
      videoContainer.style.display = "flex";
      videoContainer.style.alignItems = "center";
      videoContainer.style.justifyContent = "center";
      videoContainer.style.overflow = "visible";
      // videoContainer.style.background = "#000";
      videoContainer.style.position = "relative";

      console.log(
        getTime(),
        `🔄 [${this.deviceId}] Container size set to: ${containerWidth} x ${containerHeight}`
      );
      if (videoWidth > videoHeight) {
        this.rotation = 0;
      }
    }

    // 获取用户设置的缩放比例并合并到transform中
    let userScale = storage.getVideoScale(this.deviceId);

    // 也尝试从UI元素获取当前选择的缩放值
    // const num = this.deviceId.slice(-1);
    const num = this.deviceId.replace("device", "");
    const scaleSelect = document.getElementById(`scaleSelect${num}`);
    if (scaleSelect && scaleSelect.value) {
      const uiScale = parseFloat(scaleSelect.value);
      if (!isNaN(uiScale)) {
        userScale = uiScale;
        console.log(
          getTime(),
          `🔄 [${this.deviceId}] Using UI scale value: ${userScale}`
        );
      }
    }

    // 总是组合旋转和缩放变换（即使缩放为1.0）
    const transform = `rotate(${this.rotation}deg) scale(${userScale})`;
    console.log(
      getTime(),
      `🔄 [${this.deviceId}] Combined transform: rotate(${this.rotation}deg) scale(${userScale})`
    );

    // 应用旋转和缩放变换
    this.elements.remoteVideo.style.transform = transform;
    this.elements.remoteVideo.style.transformOrigin = "center center";
    this.elements.remoteVideo.style.transition = "transform 0.3s ease";

    // 设置视频元素的尺寸和适应方式
    this.elements.remoteVideo.style.visibility = "visible";
    this.elements.remoteVideo.style.opacity = "1";
    this.elements.remoteVideo.style.display = "block";

    // 保持鼠标控制兼容性：不改变video元素的尺寸计算方式
    this.elements.remoteVideo.style.width = "auto";
    this.elements.remoteVideo.style.height = "auto";
    this.elements.remoteVideo.style.objectFit = "contain";
    this.elements.remoteVideo.style.maxWidth = "100%";
    this.elements.remoteVideo.style.maxHeight = "100%";

    console.log(
      getTime(),
      `🔄 [${this.deviceId}] 保持video样式兼容鼠标控制，缩放通过transform处理: ${userScale}`
    );

    console.log(
      getTime(),
      `🔄 [${this.deviceId}] Transform applied: ${transform}`
    );
    console.log(
      getTime(),
      `🔄 [${this.deviceId}] Video dimensions: ${this.elements.remoteVideo.videoWidth}x${this.elements.remoteVideo.videoHeight}`
    );
  }

  /**
   * 兼容性方法：保持原有的applyRotation接口
   */
  applyRotation() {
    this.applyRotationAndSize();
  }

  // === 视频统计相关方法 ===

  /**
   * 开始视频统计
   */
  startVideoStats() {
    if (!this.pc || !this.remoteStream) return;

    // 使用webrtc-stats库
    if (window.WebRTCStats) {
      try {
        this.webrtcStats = new WebRTCStats({
          getStatsInterval: 1000,
          rawStats: false,
          statsObject: true,
          filteredStats: false,
          shouldWrapGetUserMedia: false
        });

        this.webrtcStats.on("stats", event => this.handleWebRTCStats(event));
        this.webrtcStats.addConnection({ pc: this.pc, peerId: this.deviceId });

        console.log(
          getTime(),
          `📊 [${this.deviceId}] WebRTC stats started (webrtc-stats library)`
        );
      } catch (error) {
        console.warn(
          `⚠️ [${this.deviceId}] WebRTC-Stats library failed, fallback to native:`,
          error
        );
        this.fallbackToNativeStats();
      }
    } else {
      this.fallbackToNativeStats();
    }
  }

  /**
   * 停止视频统计
   */
  stopVideoStats() {
    if (this.webrtcStats) {
      try {
        this.webrtcStats.removeConnection({
          pc: this.pc,
          peerId: this.deviceId
        });
        this.webrtcStats = null;
      } catch (error) {
        console.warn(`⚠️ [${this.deviceId}] Stop WebRTC stats failed:`, error);
      }
    }

    if (this.statsInterval) {
      clearInterval(this.statsInterval);
      this.statsInterval = null;
    }

    console.log(getTime(), `📊 [${this.deviceId}] Video stats stopped`);
  }

  /**
   * 处理WebRTC统计数据
   */
  handleWebRTCStats(event) {
    if (!event.data || !event.data[this.deviceId]) return;

    const stats = event.data[this.deviceId];
    const videoReceiver =
      stats.video && stats.video.inbound && stats.video.inbound[0];

    if (videoReceiver) {
      // 计算码率 (字节/秒)
      const bitsPerSecond = videoReceiver.bitrate || 0;
      const bytesPerSecond = bitsPerSecond / 8;
      let bitrate = 0;
      let bitrateUnit = "KB/s";

      if (bytesPerSecond >= 1024 * 1024) {
        bitrate = Math.round((bytesPerSecond / (1024 * 1024)) * 100) / 100; // MB/s
        bitrateUnit = "MB/s";
      } else {
        bitrate = Math.round((bytesPerSecond / 1024) * 100) / 100; // KB/s
        bitrateUnit = "KB/s";
      }

      const processedStats = {
        fps: Math.round(videoReceiver.framesPerSecond || 0),
        droppedFrames: videoReceiver.framesDropped || 0,
        totalFrames: videoReceiver.framesReceived || 0,
        bitrate: bitrate,
        bitrateUnit: bitrateUnit,
        resolution: `${videoReceiver.frameWidth || 0}x${
          videoReceiver.frameHeight || 0
        }`,
        width: videoReceiver.frameWidth || 0,
        height: videoReceiver.frameHeight || 0
      };

      this.updateStatsDisplay(processedStats);
      this.emit("onStatsUpdate", processedStats);
    }
  }

  /**
   * 备用统计方法
   */
  fallbackToNativeStats() {
    console.log(getTime(), `📊 [${this.deviceId}] Using native getStats API`);

    this.statsInterval = setInterval(async () => {
      if (!this.pc) return;

      try {
        const stats = await this.pc.getStats();
        const videoStats = {};
        const connectionStats = {};

        stats.forEach(report => {
          if (report.type === "inbound-rtp" && report.mediaType === "video") {
            videoStats.fps = report.framesPerSecond || 0;
            videoStats.droppedFrames = report.framesDropped || 0;
            videoStats.totalFrames = report.framesReceived || 0;
            videoStats.bytesReceived = report.bytesReceived || 0;
            videoStats.width = report.frameWidth || 0;
            videoStats.height = report.frameHeight || 0;
            videoStats.timestamp = report.timestamp;
          }
        });

        this.processVideoStats(videoStats, connectionStats);
      } catch (error) {
        console.error(`❌ [${this.deviceId}] Get stats failed:`, error);
      }
    }, 1000);
  }

  /**
   * 处理视频统计数据
   */
  processVideoStats(videoStats, connectionStats) {
    // 计算码率 (字节/秒)
    let bitrate = 0;
    let bitrateUnit = "KB/s";
    if (this.lastBytesReceived && this.lastTimestamp) {
      const bytesDelta = videoStats.bytesReceived - this.lastBytesReceived;
      const timeDelta = (videoStats.timestamp - this.lastTimestamp) / 1000;
      if (timeDelta > 0) {
        const bytesPerSecond = bytesDelta / timeDelta;
        // 转换为合适的单位
        if (bytesPerSecond >= 1024 * 1024) {
          bitrate = Math.round((bytesPerSecond / (1024 * 1024)) * 100) / 100; // MB/s
          bitrateUnit = "MB/s";
        } else {
          bitrate = Math.round((bytesPerSecond / 1024) * 100) / 100; // KB/s
          bitrateUnit = "KB/s";
        }
      }
    }

    this.lastBytesReceived = videoStats.bytesReceived;
    this.lastTimestamp = videoStats.timestamp;

    const processedStats = {
      fps: Math.round(videoStats.fps || 0),
      droppedFrames: videoStats.droppedFrames || 0,
      totalFrames: videoStats.totalFrames || 0,
      bitrate: bitrate,
      bitrateUnit: bitrateUnit,
      resolution: `${videoStats.width}x${videoStats.height}`,
      width: videoStats.width,
      height: videoStats.height
    };

    this.updateStatsDisplay(processedStats);
    this.emit("onStatsUpdate", processedStats);
  }

  /**
   * 更新统计显示
   */
  updateStatsDisplay(stats) {
    const { elements } = this;

    if (elements.fpsValue) {
      elements.fpsValue.textContent = stats.fps;
      elements.fpsValue.style.color = this.getFpsColor(stats.fps);
    }

    if (elements.videoResolution) {
      elements.videoResolution.textContent = stats.resolution;
    }

    if (elements.droppedFrames) {
      const dropRate =
        stats.totalFrames > 0
          ? ((stats.droppedFrames / stats.totalFrames) * 100).toFixed(1)
          : 0;
      elements.droppedFrames.textContent = `${stats.droppedFrames} (${dropRate}%)`;
      elements.droppedFrames.style.color = this.getDropRateColor(
        parseFloat(dropRate)
      );
    }

    if (elements.videoBitrate) {
      elements.videoBitrate.textContent = `${stats.bitrate} ${
        stats.bitrateUnit || "KB/s"
      }`;
    }
  }

  /**
   * 重置统计显示
   */
  resetStatsDisplay() {
    const elements = [
      this.elements.fpsValue,
      this.elements.videoResolution,
      this.elements.droppedFrames,
      this.elements.videoBitrate
    ];

    elements.forEach(element => {
      if (element) {
        element.textContent = "--";
        element.style.color = "";
      }
    });
  }

  /**
   * 获取FPS颜色
   */
  getFpsColor(fps) {
    if (fps >= 25) return "#4CAF50"; // 绿色 - 优秀
    if (fps >= 20) return "#8BC34A"; // 浅绿 - 良好
    if (fps >= 15) return "#FF9800"; // 橙色 - 一般
    if (fps >= 10) return "#FF5722"; // 橙红 - 较差
    return "#F44336"; // 红色 - 很差
  }

  /**
   * 获取丢帧率颜色
   */
  getDropRateColor(dropRate) {
    if (dropRate <= 1) return "#4CAF50"; // 绿色 - 优秀
    if (dropRate <= 3) return "#8BC34A"; // 浅绿 - 良好
    if (dropRate <= 5) return "#FF9800"; // 橙色 - 一般
    if (dropRate <= 10) return "#FF5722"; // 橙红 - 较差
    return "#F44336"; // 红色 - 很差
  }

  /**
   * 切换视频统计显示
   */
  toggleVideoStats() {
    this.statsEnabled = !this.statsEnabled;

    if (this.statsEnabled && this.remoteStream) {
      this.startVideoStats();
    } else {
      this.stopVideoStats();
      this.resetStatsDisplay();
    }
  }

  // === 设备配置相关方法 ===

  /**
   * 更新分辨率
   */
  updateResolution(width, height) {
    this.width = width;
    this.height = height;
    storage.setResolution(this.deviceId, width, height);
    console.log(
      getTime(),
      `📐 [${this.deviceId}] Resolution updated: ${width}x${height}`
    );

    // 清理矩阵缓存以确保使用新的分辨率变换
    if (
      window.multiDeviceMouseController &&
      window.multiDeviceMouseController.matrixCache
    ) {
      window.multiDeviceMouseController.matrixCache.clear();
      console.log(
        getTime(),
        `🔧 [${this.deviceId}] 清理矩阵缓存以应用新分辨率`
      );
    }

    // 重新连接以应用新分辨率配置
    if (this.isConnected) {
      console.log(
        getTime(),
        `🔄 [${this.deviceId}] Reconnecting to apply new resolution...`
      );
      this.disconnect();
      setTimeout(() => this.connect(), 1000);
    }
  }

  /**
   * 更新码率
   */
  updateBitrate(bitrate) {
    this.targetBitrate = bitrate;
    storage.setBitrate(this.deviceId, bitrate);
    console.log(
      getTime(),
      `📡 [${
        this.deviceId
      }] Bitrate updated: ${bitrate} MB/s (前端设置，编码器将收到 ${
        bitrate * 8
      } Mbps)`
    );

    // 重新连接以应用编码器配置
    if (this.isConnected) {
      console.log(
        getTime(),
        `🔄 [${this.deviceId}] Reconnecting to apply new bitrate...`
      );
      this.disconnect();
      setTimeout(() => this.connect(), 1000);
    }
  }

  /**
   * 更新关键帧间隔
   */
  updateKeyFrameInterval(interval) {
    this.keyFrameInterval = interval;
    storage.setKeyFrameInterval(this.deviceId, interval);
    console.log(
      getTime(),
      `🔑 [${this.deviceId}] KeyFrame interval updated: ${interval}s`
    );

    // 重新连接以应用编码器配置
    if (this.isConnected) {
      console.log(
        getTime(),
        `🔄 [${this.deviceId}] Reconnecting to apply new keyframe interval...`
      );
      this.disconnect();
      setTimeout(() => this.connect(), 1000);
    }
  }

  /**
   * 更新质量等级
   */
  updateQuality(quality) {
    this.qualityLevel = quality;
    storage.setQuality(this.deviceId, quality);
    console.log(
      getTime(),
      `🎨 [${this.deviceId}] Quality updated: ${quality}%`
    );

    // 重新连接以应用编码器配置
    if (this.isConnected) {
      console.log(
        getTime(),
        `🔄 [${this.deviceId}] Reconnecting to apply new quality level...`
      );
      this.disconnect();
      setTimeout(() => this.connect(), 1000);
    }
  }
  /**
   * 验证状态一致性（调试用）
   */
  validateState() {
    const videoWidth = this.elements.remoteVideo?.videoWidth || 0;
    const videoHeight = this.elements.remoteVideo?.videoHeight || 0;
    const videoIsLandscape = videoWidth > videoHeight;

    const hasRotatedClass =
      this.elements.remoteVideo?.classList.contains("rotated") || false;
    const hasLandscapeClass =
      this.elements.remoteVideo?.classList.contains("landscape") || false;

    console.log(`🔍 [${this.deviceId}] 状态验证:`);
    console.log(
      `  视频: ${videoWidth}x${videoHeight} (${
        videoIsLandscape ? "横屏" : "竖屏"
      })`
    );
    console.log(
      `  状态: rotation=${this.rotation}°, frontendRotated=${this.frontendRotated}`
    );
    console.log(
      `  CSS类: rotated=${hasRotatedClass}, landscape=${hasLandscapeClass}`
    );
    console.log(`  旋转允许: ${this.isRotationAllowed()}`);
    console.log(
      `  上次用户旋转: ${Date.now() - this.lastUserRotationTime}ms前`
    );
    console.log(`  记录分辨率: ${this.lastVideoWidth}x${this.lastVideoHeight}`);

    // 检查逻辑一致性
    if (videoIsLandscape && hasRotatedClass) {
      console.warn(`⚠️ [${this.deviceId}] 状态异常：横屏视频不应有rotated类`);
    }
    if (!videoIsLandscape && hasLandscapeClass) {
      console.warn(`⚠️ [${this.deviceId}] 状态异常：竖屏视频不应有landscape类`);
    }
    if (hasRotatedClass && hasLandscapeClass) {
      console.warn(
        `⚠️ [${this.deviceId}] 状态异常：不应同时有rotated和landscape类`
      );
    }
  }
  /**
   * 更新编码器FPS
   */
  updateEncoderFps(fps) {
    this.targetEncoderFps = fps;
    this.fps = fps; // FPS由编码器FPS控制
    storage.setEncoderFps(this.deviceId, fps);
    console.log(getTime(), `⚡ [${this.deviceId}] Encoder FPS updated: ${fps}`);

    // 重新连接以应用编码器配置
    if (this.isConnected) {
      console.log(
        getTime(),
        `🔄 [${this.deviceId}] Reconnecting to apply new encoder FPS...`
      );
      this.disconnect();
      setTimeout(() => this.connect(), 1000);
    }
  }
  /**
   * 更新旋转按钮状态
   */
  updateRotationButtonState() {
    const rotateButton = document.querySelector(
      `button[onclick="rotateDevice('${this.deviceId}')"]`
    );
    if (!rotateButton) {
      console.log(`⚠️ [${this.deviceId}] 未找到旋转按钮`);
      return;
    }

    // 始终启用旋转按钮
    rotateButton.disabled = false;
    rotateButton.style.opacity = "1";
    rotateButton.title = "横竖屏切换";
    console.log(`🔓 [${this.deviceId}] 旋转按钮已启用`);
  }

  /**
   * 检查当前是否允许旋转
   * 修改：允许在任何情况下都可以旋转，包括横屏分辨率
   */
  isRotationAllowed() {
    // 始终允许旋转，无论视频分辨率如何
    return true;
  }
  /**
   * 应用固定视频尺寸 - 根据视频分辨率切换横竖屏模式，结合前端旋转状态
   * @param {number} oldVideoWidth - 可选：变化前的视频宽度
   * @param {number} oldVideoHeight - 可选：变化前的视频高度
   */
  applyFixedVideoSize(oldVideoWidth = null, oldVideoHeight = null) {
    if (!this.elements.remoteVideo) return;

    const videoWidth = this.elements.remoteVideo.videoWidth || 0;
    const videoHeight = this.elements.remoteVideo.videoHeight || 0;

    if (videoWidth === 0 || videoHeight === 0) {
      console.log(`📐 [${this.deviceId}] 视频尺寸未知，跳过固定尺寸设置`);
      return;
    }

    // 检测分辨率是否发生变化
    // 如果传入了旧分辨率参数，使用传入的值；否则使用记录的值
    const lastWidth =
      oldVideoWidth !== null ? oldVideoWidth : this.lastVideoWidth;
    const lastHeight =
      oldVideoHeight !== null ? oldVideoHeight : this.lastVideoHeight;
    const videoResolutionChanged =
      lastWidth !== videoWidth || lastHeight !== videoHeight;
    const wasLandscape = lastWidth > lastHeight;
    const videoIsLandscape = videoWidth > videoHeight;

    // 调试信息：详细的分辨率检测状态
    console.log(`🔍 [${this.deviceId}] 分辨率检测详情:`);
    console.log(
      `  对比分辨率: ${lastWidth}x${lastHeight} (${
        wasLandscape ? "横屏" : "竖屏"
      })`
    );
    console.log(
      `  当前分辨率: ${videoWidth}x${videoHeight} (${
        videoIsLandscape ? "横屏" : "竖屏"
      })`
    );
    console.log(`  分辨率是否变化: ${videoResolutionChanged}`);
    if (oldVideoWidth !== null) {
      console.log(
        `  使用传入的旧分辨率参数: ${oldVideoWidth}x${oldVideoHeight}`
      );
    }

    // 特别检测横屏→竖屏的变化（如Home键场景）
    const isLandscapeToPortrait =
      videoResolutionChanged && wasLandscape && !videoIsLandscape;
    if (isLandscapeToPortrait && this.lastVideoWidth > 0) {
      console.log(
        `📱 [${this.deviceId}] 检测到关键的横屏→竖屏分辨率变化，可能是Home键或应用切换`
      );
    }

    console.log(
      `📐 [${this.deviceId}] 视频分辨率: ${videoWidth}x${videoHeight}`
    );
    console.log(
      `📐 [${this.deviceId}] 视频检测为: ${
        videoIsLandscape ? "横屏" : "竖屏"
      }模式`
    );
    console.log(
      `📐 [${this.deviceId}] 前端旋转状态: ${
        this.frontendRotated ? "已旋转" : "未旋转"
      }，角度: ${this.rotation}°`
    );

    if (videoResolutionChanged && this.lastVideoWidth > 0) {
      console.log(
        `📐 [${this.deviceId}] 检测到分辨率变化: ${this.lastVideoWidth}x${this.lastVideoHeight} → ${videoWidth}x${videoHeight}`
      );
      console.log(
        `📐 [${this.deviceId}] 方向变化: ${wasLandscape ? "横屏" : "竖屏"} → ${
          videoIsLandscape ? "横屏" : "竖屏"
        }`
      );
    }

    // 更新旋转按钮状态：如果是横屏分辨率，禁用旋转按钮
    this.updateRotationButtonState();

    // ===== 按照文档的完整逻辑判断流程 =====

    // 注意：在逻辑处理完成后再更新记录的分辨率，确保变化检测正确

    if (videoIsLandscape) {
      // 横屏分辨率分支
      console.log(`🎯 [${this.deviceId}] 执行横屏分辨率逻辑分支`);

      // 应用横屏模式 (.landscape)
      this.elements.remoteVideo.classList.remove("rotated");
      this.elements.remoteVideo.classList.add("landscape");
      console.log(`📐 [${this.deviceId}] 应用横屏模式: 640x360px，无旋转`);

      // 检测竖屏→横屏的变化
      const isPortraitToLandscape =
        videoResolutionChanged && !wasLandscape && videoIsLandscape;

      // 如果是从竖屏变为横屏，且前端处于旋转状态，则自动取消旋转
      if (
        isPortraitToLandscape &&
        this.frontendRotated &&
        this.lastVideoWidth > 0
      ) {
        console.log(`🔄 [${this.deviceId}] 检测到竖屏→横屏变化，取消前端旋转`);
        this.rotation = 0;
        this.frontendRotated = false;
        storage.setRotation(this.deviceId, 0);
        // 不发送旋转事件，因为分辨率变化本身就是由旋转事件触发的
      }
    } else {
      // 竖屏分辨率分支
      console.log(`🎯 [${this.deviceId}] 执行竖屏分辨率逻辑分支`);

      if (this.frontendRotated) {
        // 前端已旋转状态
        console.log(
          `🔍 [${this.deviceId}] 前端处于旋转状态，检查是否需要自动回正`
        );

        // 核心逻辑：只有在检测到明确的横屏→竖屏分辨率变化时才自动回正
        // 用户手动旋转时应该保持旋转状态，直到视频分辨率真正发生变化

        // 检查分辨率从横屏变回竖屏的情况（如Home键等场景）
        // 注意：只有明确的横屏→竖屏变化才触发自动回正，避免干扰用户刚旋转的操作
        const shouldAutoCorrect =
          videoResolutionChanged && wasLandscape && !videoIsLandscape;

        if (shouldAutoCorrect) {
          // 检测到横屏→竖屏的分辨率变化，执行自动回正
          console.log(
            `🔄 [${this.deviceId}] 检测到分辨率从横屏变回竖屏，执行自动回正`
          );
          console.log(
            `📱 [${this.deviceId}] 分辨率变化详情: ${lastWidth}x${lastHeight} → ${videoWidth}x${videoHeight}`
          );
          console.log(
            `🎯 [${this.deviceId}] 这可能是点击Home键或进入不支持横屏的应用导致的分辨率变化`
          );

          // 执行自动回正并发送旋转事件到后端
          this.rotation = 0;
          this.frontendRotated = false;
          storage.setRotation(this.deviceId, 0);

          // 注意：自动回正时不更新 lastUserRotationTime，保持原有的用户操作时间
          // 发送旋转事件给后端，确保后端也恢复到正常状态
          this.sendRotationEvent(0);

          // 立即移除所有CSS类
          this.elements.remoteVideo.classList.remove("rotated");
          this.elements.remoteVideo.classList.remove("landscape");

          console.log(`✅ [${this.deviceId}] 自动回正完成：恢复竖屏正常显示`);
        } else {
          // 保持旋转状态，检查用户操作时间
          const timeSinceLastRotation = Date.now() - this.lastUserRotationTime;
          const isRecentUserAction = timeSinceLastRotation < 5000;

          if (isRecentUserAction) {
            console.log(
              `🎯 [${this.deviceId}] 最近用户旋转操作(${Math.round(
                timeSinceLastRotation / 1000
              )}s前)，保持旋转`
            );
          } else {
            console.log(
              `🎯 [${this.deviceId}] 较久前的旋转操作，尊重用户选择继续保持旋转`
            );
          }

          // 应用旋转模式
          this.elements.remoteVideo.classList.remove("landscape");
          this.elements.remoteVideo.classList.add("rotated");
          console.log(
            `📐 [${this.deviceId}] 应用旋转模式: 360x640px + rotate(90deg)`
          );
        }
      } else {
        // 前端未旋转状态
        console.log(`🎯 [${this.deviceId}] 前端未旋转，应用正常竖屏显示`);

        // 正常竖屏显示 (无CSS类)
        this.elements.remoteVideo.classList.remove("landscape");
        this.elements.remoteVideo.classList.remove("rotated");
        console.log(`📐 [${this.deviceId}] 应用正常模式: 360x640px，无旋转`);
      }
    }

    // 禁用所有JavaScript动态尺寸设置，让CSS完全控制
    this.elements.remoteVideo.style.width = "";
    this.elements.remoteVideo.style.height = "";
    this.elements.remoteVideo.style.maxWidth = "";
    this.elements.remoteVideo.style.maxHeight = "";
    this.elements.remoteVideo.style.minWidth = "";
    this.elements.remoteVideo.style.minHeight = "";

    console.log(`✅ [${this.deviceId}] 智能旋转逻辑处理完成`);

    // 最后更新记录的分辨率（确保下次能正确检测变化）
    this.lastVideoWidth = videoWidth;
    this.lastVideoHeight = videoHeight;

    // 验证状态一致性（调试）
    this.validateState();
  }
  /**
   * 更新旋转角度
   */
  updateRotation(rotation) {
    // 检查是否允许旋转（横屏分辨率时不允许）
    if (!this.isRotationAllowed()) {
      console.log(
        `🚫 [${this.deviceId}] 旋转被禁止：当前为横屏分辨率，无需旋转`
      );
      return;
    }

    console.log(
      `🔄 [${this.deviceId}] 用户手动旋转操作: ${this.rotation}° → ${rotation}°`
    );

    // 更新状态
    this.rotation = rotation;
    storage.setRotation(this.deviceId, rotation);

    // 记录前端旋转状态（90度表示已旋转）
    this.frontendRotated = rotation === -90;

    // 记录用户旋转时间（用于区分用户操作和自动协同）
    this.lastUserRotationTime = Date.now();

    console.log(
      `📊 [${this.deviceId}] 状态更新完成: rotation=${rotation}°, frontendRotated=${this.frontendRotated}, lastUserRotationTime=${this.lastUserRotationTime}`
    );

    // 应用固定尺寸逻辑（这会处理CSS类的应用）
    this.applyFixedVideoSize();

    // 发送旋转事件给后端
    this.sendRotationEvent(rotation);

    console.log(`✅ [${this.deviceId}] 用户旋转操作处理完成`);
  }
  /**
   * 发送旋转事件给后端
   */
  sendRotationEvent(angle) {
    if (!this.dataChannel || this.dataChannel.readyState !== "open") {
      console.warn(
        `⚠️ [${this.deviceId}] DataChannel not ready, cannot send rotation event`
      );
      return;
    }

    try {
      // 更新状态
      this.rotation = angle;

      const rotationMessage = {
        type: "rotate_device",
        angle: angle
      };
      this.dataChannel.send(JSON.stringify(rotationMessage));
      console.log(
        `🔄🔄🔄✅✅✅ [${this.deviceId}] Rotation event sent: ${angle}°`
      );
    } catch (error) {
      console.error(
        `❌ [${this.deviceId}] Failed to send rotation event:`,
        error
      );
    }
  }

  /**
   * 切换静音状态
   */
  toggleMute() {
    this.isMuted = !this.isMuted;
    storage.setMuted(this.deviceId, this.isMuted);
    this.updateMuteButtonState();
    console.log(
      getTime(),
      `🔊 [${this.deviceId}] Mute toggled: ${this.isMuted}`
    );
    return this.isMuted;
  }

  /**
   * 更新房间ID
   */
  updateRoom(roomId) {
    const wasConnected = this.isConnected;

    // 如果已连接，先断开
    if (wasConnected) {
      this.disconnect();
    }

    this.roomId = roomId;
    console.log(getTime(), `🏠 [${this.deviceId}] Room updated: ${roomId}`);

    // 如果之前是连接状态，重新连接到新房间
    if (wasConnected) {
      setTimeout(() => this.connect(), 1000);
    }
  }

  /**
   * 获取设备状态
   */
  getDeviceStatus() {
    return {
      deviceId: this.deviceId,
      isConnected: this.isConnected,
      hasVideo: !!this.remoteStream,
      resolution: { width: this.width, height: this.height },
      encoderConfig: {
        bitrate: this.targetBitrate,
        keyFrame: this.keyFrameInterval,
        quality: this.qualityLevel,
        fps: this.targetEncoderFps
      },
      rotation: this.rotation,
      isMuted: this.isMuted,
      roomId: this.roomId
    };
  }

  /**
   * 销毁设备管理器
   */
  destroy() {
    this.disconnect();
    this.clearVideoDisplay();

    // 关闭摄像头
    if (this.cameraEnabled) {
      this.closeCamera().catch(err => {
        console.error(
          getTime(),
          `❌ [${this.deviceId}] 销毁时关闭摄像头失败:`,
          err
        );
      });
    }

    // 停止视频尺寸监控
    this.stopVideoSizeMonitoring();

    // 清理鼠标和键盘控制器
    MouseControllerFactory.destroyController(this.deviceId);

    // 清理DataChannel全局变量
    if (window[`dataChannel_${this.deviceId}`]) {
      window[`dataChannel_${this.deviceId}`] = null;
      console.log(
        getTime(),
        `🔧 [${this.deviceId}] DataChannel global variable cleared on destroy`
      );
    }

    // 清理事件回调
    Object.keys(this.callbacks).forEach(key => {
      this.callbacks[key] = null;
    });

    console.log(getTime(), `🗑️ [${this.deviceId}] DeviceManager destroyed`);
  }
}

/**
 * 设备管理器工厂
 */
export class DeviceManagerFactory {
  static managers = new Map();

  /**
   * 创建或获取设备管理器
   * @param {string} deviceId 设备ID
   * @returns {DeviceManager} 设备管理器实例
   */
  static getManager(deviceId) {
    if (!this.managers.has(deviceId)) {
      const item = window.CphoneWebRTCConfig.roomList.find(room => {
        if (room.deviceId == deviceId.replace("device", "")) {
          return room;
        }
      });

      const manager = new DeviceManager({ ...item, deviceId });
      this.managers.set(deviceId, manager);
      console.log(
        getTime(),
        `🏭 [Factory] Created DeviceManager for ${deviceId}`
      );
    }
    return this.managers.get(deviceId);
  }

  /**
   * 销毁设备管理器
   * @param {string} deviceId 设备ID
   */
  static destroyManager(deviceId) {
    const manager = this.managers.get(deviceId);
    if (manager) {
      manager.destroy();
      this.managers.delete(deviceId);
      console.log(
        getTime(),
        `🏭 [Factory] Destroyed DeviceManager for ${deviceId}`
      );
    }
  }

  /**
   * 获取所有设备管理器
   * @returns {Map} 所有设备管理器
   */
  static getAllManagers() {
    return this.managers;
  }

  /**
   * 清理所有设备管理器
   */
  static destroyAll() {
    this.managers.forEach((manager, deviceId) => {
      manager.disconnect();
      manager.destroy();
    });
    this.managers.clear();
    console.log(getTime(), `🏭 [Factory] Destroyed all DeviceManagers`);
  }

  /**
   * 🔧 获取连接统计信息（基于test1的监控）
   */
  static getStats() {
    return {
      activeConnections: window.activePeerConnections || 0,
      totalOperations: window.peerOperationCount || 0,
      managers: this.managers.size
    };
  }
}
