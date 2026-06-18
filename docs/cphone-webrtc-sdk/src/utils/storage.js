/**
 * 本地存储管理模块
 * 提供设备设置的持久化存储功能
 */

import {
  ENCODER_CONFIG,
  ROTATION_CONFIG,
  SCALE_CONFIG,
  STORAGE_KEYS,
  VIDEO_CONFIG
} from "../core/config.js";

/**
 * 存储管理器类
 */
export class StorageManager {
  constructor() {
    this.prefix = "webscreen_";
  }

  /**
   * 生成完整的存储键名
   * @param {string} key 基础键名
   * @param {string} deviceId 设备ID
   * @returns {string} 完整键名
   */
  getKey(key, deviceId) {
    return `${this.prefix}${key}_${deviceId}`;
  }

  /**
   * 获取存储值
   * @param {string} key 键名
   * @param {string} deviceId 设备ID
   * @param {*} defaultValue 默认值
   * @returns {*} 存储的值或默认值
   */
  get(key, deviceId, defaultValue = null) {
    try {
      const fullKey = this.getKey(key, deviceId);
      const value = localStorage.getItem(fullKey);
      return value !== null ? JSON.parse(value) : defaultValue;
    } catch (error) {
      console.warn(`⚠️ [存储] 读取失败 ${key}:`, error);
      return defaultValue;
    }
  }

  /**
   * 设置存储值
   * @param {string} key 键名
   * @param {string} deviceId 设备ID
   * @param {*} value 值
   */
  set(key, deviceId, value) {
    try {
      const fullKey = this.getKey(key, deviceId);
      localStorage.setItem(fullKey, JSON.stringify(value));
    } catch (error) {
      console.warn(`⚠️ [存储] 写入失败 ${key}:`, error);
    }
  }

  /**
   * 删除存储值
   * @param {string} key 键名
   * @param {string} deviceId 设备ID
   */
  remove(key, deviceId) {
    try {
      const fullKey = this.getKey(key, deviceId);
      localStorage.removeItem(fullKey);
    } catch (error) {
      console.warn(`⚠️ [存储] 删除失败 ${key}:`, error);
    }
  }

  /**
   * 清除设备的所有设置
   * @param {string} deviceId 设备ID
   */
  clearDevice(deviceId) {
    const keys = Object.values(STORAGE_KEYS);
    keys.forEach(key => this.remove(key, deviceId));
    console.log(`🗑️ [存储] 清除设备 ${deviceId} 的所有设置`);
  }

  /**
   * 清除所有设备的所有设置
   */
  clearAll() {
    const keysToRemove = [];
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.startsWith(this.prefix)) {
        keysToRemove.push(key);
      }
    }

    keysToRemove.forEach(key => localStorage.removeItem(key));
    console.log(`🗑️ [存储] 清除所有设备设置，共 ${keysToRemove.length} 个条目`);
  }

  /**
   * 获取设备的所有设置
   * @param {string} deviceId 设备ID
   * @returns {Object} 设备设置对象
   */
  getDeviceSettings(deviceId) {
    return {
      resolution: this.getResolution(deviceId),
      bitrate: this.getBitrate(deviceId),
      keyFrame: this.getKeyFrameInterval(deviceId),
      quality: this.getQuality(deviceId),
      encoderFps: this.getEncoderFps(deviceId),
      rotation: this.getRotation(deviceId),
      videoScale: this.getVideoScale(deviceId),
      muted: this.getMuted(deviceId)
    };
  }

  /**
   * 设置设备的所有设置
   * @param {string} deviceId 设备ID
   * @param {Object} settings 设置对象
   */
  setDeviceSettings(deviceId, settings) {
    Object.entries(settings).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        this.set(key, deviceId, value);
      }
    });
  }

  // === 分辨率相关 ===
  getResolution(deviceId) {
    const saved = this.get(STORAGE_KEYS.RESOLUTION, deviceId);
    if (saved && saved !== "custom") {
      const [width, height] = saved.split("x").map(Number);
      return { width, height, label: saved };
    }
    return VIDEO_CONFIG.DEFAULT_RESOLUTION;
  }

  setResolution(deviceId, width, height) {
    const resolution = `${width}x${height}`;
    this.set(STORAGE_KEYS.RESOLUTION, deviceId, resolution);
  }

  // === 编码器相关 ===
  getBitrate(deviceId) {
    return this.get(
      STORAGE_KEYS.BITRATE,
      deviceId,
      ENCODER_CONFIG.DEFAULT_BITRATE
    );
  }

  setBitrate(deviceId, bitrate) {
    this.set(STORAGE_KEYS.BITRATE, deviceId, parseFloat(bitrate));
  }

  getKeyFrameInterval(deviceId) {
    return this.get(
      STORAGE_KEYS.KEYFRAME,
      deviceId,
      ENCODER_CONFIG.DEFAULT_KEYFRAME_INTERVAL
    );
  }

  setKeyFrameInterval(deviceId, interval) {
    this.set(STORAGE_KEYS.KEYFRAME, deviceId, parseInt(interval));
  }

  getQuality(deviceId) {
    return this.get(
      STORAGE_KEYS.QUALITY,
      deviceId,
      ENCODER_CONFIG.DEFAULT_QUALITY
    );
  }

  setQuality(deviceId, quality) {
    this.set(STORAGE_KEYS.QUALITY, deviceId, parseInt(quality));
  }

  getEncoderFps(deviceId) {
    return this.get(
      STORAGE_KEYS.ENCODER_FPS,
      deviceId,
      ENCODER_CONFIG.DEFAULT_ENCODER_FPS
    );
  }

  setEncoderFps(deviceId, fps) {
    this.set(STORAGE_KEYS.ENCODER_FPS, deviceId, parseInt(fps));
  }

  // === 视频变换相关 ===
  getRotation(deviceId) {
    return this.get(STORAGE_KEYS.ROTATION, deviceId, ROTATION_CONFIG.DEFAULT);
  }

  setRotation(deviceId, rotation) {
    this.set(STORAGE_KEYS.ROTATION, deviceId, parseInt(rotation));
  }

  getVideoScale(deviceId) {
    const scale = this.get(
      STORAGE_KEYS.VIDEO_SCALE,
      deviceId,
      SCALE_CONFIG.DEFAULT
    );
    return Math.min(Math.max(scale, SCALE_CONFIG.MIN), SCALE_CONFIG.MAX);
  }

  setVideoScale(deviceId, scale) {
    const clampedScale = Math.min(
      Math.max(parseFloat(scale), SCALE_CONFIG.MIN),
      SCALE_CONFIG.MAX
    );
    this.set(STORAGE_KEYS.VIDEO_SCALE, deviceId, clampedScale);
  }

  // === 音频相关 ===
  getMuted(deviceId) {
    return this.get(STORAGE_KEYS.MUTED, deviceId, false);
  }

  setMuted(deviceId, muted) {
    this.set(STORAGE_KEYS.MUTED, deviceId, Boolean(muted));
  }

  // === 设备设置迁移和备份 ===

  /**
   * 导出设备设置
   * @param {string} deviceId 设备ID
   * @returns {string} JSON格式的设备设置
   */
  exportDeviceSettings(deviceId) {
    const settings = this.getDeviceSettings(deviceId);
    return JSON.stringify(settings, null, 2);
  }

  /**
   * 导入设备设置
   * @param {string} deviceId 设备ID
   * @param {string} settingsJson JSON格式的设备设置
   * @returns {boolean} 是否导入成功
   */
  importDeviceSettings(deviceId, settingsJson) {
    try {
      const settings = JSON.parse(settingsJson);
      this.setDeviceSettings(deviceId, settings);
      console.log(`✅ [存储] 成功导入设备 ${deviceId} 的设置`);
      return true;
    } catch (error) {
      console.error(`❌ [存储] 导入设备 ${deviceId} 设置失败:`, error);
      return false;
    }
  }

  /**
   * 复制设备设置到另一个设备
   * @param {string} sourceDeviceId 源设备ID
   * @param {string} targetDeviceId 目标设备ID
   */
  copyDeviceSettings(sourceDeviceId, targetDeviceId) {
    const settings = this.getDeviceSettings(sourceDeviceId);
    this.setDeviceSettings(targetDeviceId, settings);
    console.log(
      `📋 [存储] 将设备 ${sourceDeviceId} 的设置复制到设备 ${targetDeviceId}`
    );
  }

  /**
   * 重置设备设置为默认值
   * @param {string} deviceId 设备ID
   */
  resetDeviceSettings(deviceId) {
    this.clearDevice(deviceId);
    console.log(`🔄 [存储] 重置设备 ${deviceId} 设置为默认值`);
  }

  /**
   * 获取存储使用情况统计
   * @returns {Object} 存储统计信息
   */
  getStorageStats() {
    let totalItems = 0;
    let webscreenItems = 0;
    let totalSize = 0;

    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      const value = localStorage.getItem(key);
      totalItems++;
      totalSize += key.length + value.length;

      if (key.startsWith(this.prefix)) {
        webscreenItems++;
      }
    }

    return {
      totalItems,
      webscreenItems,
      totalSize,
      sizeInKB: (totalSize / 1024).toFixed(2),
      percentageUsed: ((totalSize / (5 * 1024 * 1024)) * 100).toFixed(2) // 假设5MB限制
    };
  }
}

/**
 * 默认存储管理器实例
 */
export const storage = new StorageManager();

/**
 * 存储工具函数
 */
export const StorageUtils = {
  /**
   * 快速获取设备设置
   */
  getSettings: deviceId => storage.getDeviceSettings(deviceId),

  /**
   * 快速设置设备设置
   */
  setSettings: (deviceId, settings) =>
    storage.setDeviceSettings(deviceId, settings),

  /**
   * 快速获取视频缩放
   */
  getScale: deviceId => storage.getVideoScale(deviceId),

  /**
   * 快速设置视频缩放
   */
  setScale: (deviceId, scale) => storage.setVideoScale(deviceId, scale),

  /**
   * 快速获取旋转角度
   */
  getRotation: deviceId => storage.getRotation(deviceId),

  /**
   * 快速设置旋转角度
   */
  setRotation: (deviceId, rotation) => storage.setRotation(deviceId, rotation),

  /**
   * 快速重置设备
   */
  reset: deviceId => storage.resetDeviceSettings(deviceId),

  /**
   * 快速清理所有设备
   */
  clearAll: () => storage.clearAll()
};
/**
 * 获取时间，格式：2025-10-16 03:38:25.078
 */
export const getTime = () => {
  const now = new Date();
  const month = now.getMonth() + 1;
  const day = now.getDate();
  const hours = now.getHours();
  const minutes = now.getMinutes();
  const seconds = now.getSeconds();
  const milliseconds = now.getMilliseconds();
  return `${month}-${day} ${hours}:${minutes}:${seconds}.${milliseconds} `;
};
