/**
 * 坐标转换模块
 * 提供各种坐标系统之间的转换功能
 */

import { matrixCache } from "../math/matrix.js";

/**
 * 坐标转换器类
 * 负责在不同坐标系统之间进行转换
 */
export class CoordinateTransformer {
  constructor(matrixCacheInstance = matrixCache) {
    this.matrixCache = matrixCacheInstance;
  }

  /**
   * 将显示坐标转换为设备坐标
   * @param {number} displayX 显示X坐标
   * @param {number} displayY 显示Y坐标
   * @param {Object} displayRect 显示区域信息 {width, height}
   * @param {number} targetWidth 目标设备宽度
   * @param {number} targetHeight 目标设备高度
   * @param {number} rotation 旋转角度
   * @param {Object} videoElement 视频元素（可选，用于获取实际视频尺寸）
   * @returns {Object} 转换后的设备坐标 {x, y}
   */
  convertDisplayToDeviceCoords(
    displayX,
    displayY,
    displayRect,
    targetWidth,
    targetHeight,
    rotation,
    videoElement = null
  ) {
    // 第一步：将显示坐标转换为视频实际尺寸坐标（处理缩放）
    const videoWidth = videoElement?.videoWidth || targetWidth;
    const videoHeight = videoElement?.videoHeight || targetHeight;

    // 计算缩放比例
    const scaleX = videoWidth / displayRect.width;
    const scaleY = videoHeight / displayRect.height;

    // 应用缩放得到视频坐标
    const videoX = displayX * scaleX;
    const videoY = displayY * scaleY;

    console.log(
      `🔍 缩放转换: 显示(${displayX}, ${displayY}) -> 视频(${videoX}, ${videoY}) 缩放比例(${scaleX.toFixed(
        3
      )}, ${scaleY.toFixed(3)})`
    );

    // 第二步：使用矩阵变换处理旋转
    const videoSize = { width: videoWidth, height: videoHeight };
    const targetSize = { width: targetWidth, height: targetHeight };

    // 获取缓存的变换矩阵
    const positionMapper = this.matrixCache.getTransform(
      videoSize,
      targetSize,
      rotation
    );

    // 创建位置对象
    const position = {
      point: { x: videoX, y: videoY },
      screenSize: videoSize
    };

    // 应用矩阵变换
    const transformedPoint = positionMapper.map(position);

    if (!transformedPoint) {
      console.warn(`⚠️ 矩阵变换失败，使用原始坐标`);
      return { x: Math.round(videoX), y: Math.round(videoY) };
    }

    // 边界检查
    const result = {
      x: Math.max(0, Math.min(transformedPoint.x, targetWidth - 1)),
      y: Math.max(0, Math.min(transformedPoint.y, targetHeight - 1))
    };

    console.log(
      `🎯 矩阵变换: 视频(${videoX}, ${videoY}) -> 旋转${rotation}度 -> 最终(${result.x}, ${result.y})`
    );
    console.log(
      `📏 尺寸信息: 视频${videoWidth}x${videoHeight} -> 目标${targetWidth}x${targetHeight}`
    );

    return result;
  }

  /**
   * 简化的坐标转换（仅处理旋转，不处理缩放）
   * @param {number} x X坐标
   * @param {number} y Y坐标
   * @param {number} width 宽度
   * @param {number} height 高度
   * @param {number} rotation 旋转角度
   * @returns {Object} 转换后的坐标 {x, y}
   */
  simpleTransform(x, y, width, height, rotation) {
    switch (rotation) {
      case 90:
        return { x: y, y: width - x };
      case 180:
        return { x: width - x, y: height - y };
      case 270:
        return { x: height - y, y: x };
      default:
        return { x, y };
    }
  }

  /**
   * 获取事件坐标（支持鼠标和触摸）
   * @param {Event} event 事件对象
   * @returns {Object} 坐标 {clientX, clientY}
   */
  getEventCoordinates(event) {
    if (event.touches && event.touches.length > 0) {
      return {
        clientX: event.touches[0].clientX,
        clientY: event.touches[0].clientY
      };
    } else if (event.changedTouches && event.changedTouches.length > 0) {
      return {
        clientX: event.changedTouches[0].clientX,
        clientY: event.changedTouches[0].clientY
      };
    } else {
      return {
        clientX: event.clientX,
        clientY: event.clientY
      };
    }
  }

  /**
   * 计算相对于元素的坐标
   * @param {Event} event 事件对象
   * @param {HTMLElement} element 目标元素
   * @returns {Object} 相对坐标 {x, y, rect}
   */
  getRelativeCoordinates(event, element) {
    const rect = element.getBoundingClientRect();
    const coords = this.getEventCoordinates(event);

    return {
      x: coords.clientX - rect.left,
      y: coords.clientY - rect.top,
      rect: {
        width: rect.width,
        height: rect.height,
        left: rect.left,
        top: rect.top
      }
    };
  }

  /**
   * 检查坐标是否在边界内
   * @param {number} x X坐标
   * @param {number} y Y坐标
   * @param {number} width 宽度
   * @param {number} height 高度
   * @returns {boolean} 是否在边界内
   */
  isWithinBounds(x, y, width, height) {
    return x >= 0 && x < width && y >= 0 && y < height;
  }

  /**
   * 限制坐标在边界内
   * @param {number} x X坐标
   * @param {number} y Y坐标
   * @param {number} width 宽度
   * @param {number} height 高度
   * @returns {Object} 限制后的坐标 {x, y}
   */
  clampCoordinates(x, y, width, height) {
    return {
      x: Math.max(0, Math.min(x, width - 1)),
      y: Math.max(0, Math.min(y, height - 1))
    };
  }

  /**
   * 计算两点之间的距离
   * @param {Object} point1 点1 {x, y}
   * @param {Object} point2 点2 {x, y}
   * @returns {number} 距离
   */
  calculateDistance(point1, point2) {
    const dx = point2.x - point1.x;
    const dy = point2.y - point1.y;
    return Math.sqrt(dx * dx + dy * dy);
  }

  /**
   * 判断当前视频是否为横屏模式
   * @param {HTMLVideoElement} videoElement 视频元素
   * @returns {boolean} 是否为横屏
   */
  isVideoLandscape(videoElement) {
    if (
      !videoElement ||
      !videoElement.videoWidth ||
      !videoElement.videoHeight
    ) {
      return false; // 默认竖屏
    }

    return videoElement.videoWidth > videoElement.videoHeight;
  }

  /**
   * 根据视频当前状态计算正确的分辨率
   * @param {HTMLVideoElement} videoElement 视频元素
   * @param {number} selectedWidth 选择的宽度
   * @param {number} selectedHeight 选择的高度
   * @returns {Object} 当前分辨率 {width, height}
   */
  getCurrentResolution(videoElement, selectedWidth, selectedHeight) {
    const isLandscape = this.isVideoLandscape(videoElement);

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

    return { width: currentWidth, height: currentHeight };
  }

  /**
   * 清理矩阵缓存
   */
  clearCache() {
    this.matrixCache.clear();
  }

  /**
   * 获取缓存统计信息
   * @returns {Object} 缓存统计
   */
  getCacheStats() {
    return this.matrixCache.getStats();
  }
}

/**
 * 默认坐标转换器实例
 */
export const coordinateTransformer = new CoordinateTransformer();

/**
 * 坐标转换工具函数
 */
export const CoordinateUtils = {
  /**
   * 快速转换显示坐标到设备坐标
   */
  displayToDevice: (
    displayX,
    displayY,
    displayRect,
    targetWidth,
    targetHeight,
    rotation,
    videoElement
  ) => {
    return coordinateTransformer.convertDisplayToDeviceCoords(
      displayX,
      displayY,
      displayRect,
      targetWidth,
      targetHeight,
      rotation,
      videoElement
    );
  },

  /**
   * 快速获取相对坐标
   */
  getRelative: (event, element) => {
    return coordinateTransformer.getRelativeCoordinates(event, element);
  },

  /**
   * 快速计算距离
   */
  distance: (point1, point2) => {
    return coordinateTransformer.calculateDistance(point1, point2);
  },

  /**
   * 快速边界检查
   */
  clamp: (x, y, width, height) => {
    return coordinateTransformer.clampCoordinates(x, y, width, height);
  },

  /**
   * 快速旋转变换
   */
  rotate: (x, y, width, height, rotation) => {
    return coordinateTransformer.simpleTransform(x, y, width, height, rotation);
  }
};
