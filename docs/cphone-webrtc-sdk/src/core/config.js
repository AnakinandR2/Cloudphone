/**
 * 应用配置常量
 */

// 服务器配置
export const SERVER_CONFIG = {
  HTTP_PORT: 3000,
  HTTPS_PORT: 3443,
  MAX_WINDOWS: 200,
  DEFAULT_WINDOWS: 4
};

// 存储键名配置
export const STORAGE_KEYS = {
  RESOLUTION: "resolution",
  BITRATE: "bitrate",
  KEYFRAME: "keyframe",
  QUALITY: "quality",
  ENCODER_FPS: "encoderFps",
  ROTATION: "rotation",
  VIDEO_SCALE: "videoScale",
  MUTED: "muted"
};

// WebRTC配置
export const WEBRTC_CONFIG = {
  WS_URL: "ws://192.168.9.1:9551/ws",
  DEFAULT_ROOM: "vm:mobile_screen_2",
  ICE_CONFIG: {
    iceCandidatePoolSize: 1,
    iceTransportPolicy: "relay",
    iceConnectionReceivingTimeout: 60000,
    iceCheckMinInterval: 2000,
    iceBackupCandidatePairPingInterval: 25000,
    continualGatheringPolicy: "gather_continually",
    sdpSemantics: "unified-plan"
  }
};

// 视频配置
export const VIDEO_CONFIG = {
  DEFAULT_RESOLUTION: {
    width: 360,
    height: 640
  },
  DEFAULT_FPS: 5,
  SUPPORTED_RESOLUTIONS: [
    { width: 1080, height: 1920, label: "1080x1920 (FHD)" },
    { width: 720, height: 1280, label: "720x1280 (HD)" },
    { width: 480, height: 854, label: "480x854 (FWVGA)" },
    { width: 1440, height: 2560, label: "1440x2560 (QHD)" },
    { width: 540, height: 960, label: "540x960 (qHD)" }
  ]
};

// 编码器配置
export const ENCODER_CONFIG = {
  DEFAULT_BITRATE: 5, // 目标码率 (单位：MB/s，前端显示用)
  DEFAULT_KEYFRAME_INTERVAL: 10, // seconds
  DEFAULT_QUALITY: 20, // %
  DEFAULT_ENCODER_FPS: 5,

  // 码率选项：以MB/s为单位，对应用户期望的字节传输速率
  BITRATE_OPTIONS: [0.5, 1, 2, 3, 4, 5, 6, 8, 10, 12, 15],
  KEYFRAME_OPTIONS: [5, 10, 15, 20, 30],
  QUALITY_OPTIONS: [10, 20, 30, 40, 50, 60, 70, 80, 90, 100],
  FPS_OPTIONS: [3, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60]
};

// 旋转配置
export const ROTATION_CONFIG = {
  OPTIONS: [-90, 0, 90, 180, 270],
  DEFAULT: 0
};

// 缩放配置
export const SCALE_CONFIG = {
  OPTIONS: [0.5, 0.6, 0.7, 0.8, 0.9, 1.0],
  DEFAULT: 1.0,
  MIN: 0.1,
  MAX: 2.0
};

// 统计配置
export const STATS_CONFIG = {
  UPDATE_INTERVAL: 1000, // ms
  FPS_COLORS: {
    EXCELLENT: { min: 25, color: "#4CAF50" },
    GOOD: { min: 20, color: "#8BC34A" },
    FAIR: { min: 15, color: "#FF9800" },
    POOR: { min: 10, color: "#FF5722" },
    BAD: { min: 0, color: "#F44336" }
  },
  DROP_RATE_COLORS: {
    EXCELLENT: { max: 1, color: "#4CAF50" },
    GOOD: { max: 3, color: "#8BC34A" },
    FAIR: { max: 5, color: "#FF9800" },
    POOR: { max: 10, color: "#FF5722" },
    BAD: { max: 100, color: "#F44336" }
  }
};

// 输入配置
export const INPUT_CONFIG = {
  DRAG_THRESHOLD: 5, // 像素
  SCROLL_COOLDOWN: 50, // ms
  TOUCH_TIMEOUT: 300 // ms
};

// 调试配置
export const DEBUG_CONFIG = {
  ENABLE_CONSOLE_LOGS: true,
  ENABLE_PERFORMANCE_LOGS: false,
  MATRIX_CACHE_STATS: true
};
