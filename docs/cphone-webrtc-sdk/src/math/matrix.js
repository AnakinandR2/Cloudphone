/**
 * 矩阵运算模块
 * 提供2D仿射变换相关的矩阵运算
 */

/**
 * 仿射变换矩阵类
 */
export class AffineMatrix {
  constructor(a = 1, b = 0, c = 0, d = 1, e = 0, f = 0) {
    this.a = a;
    this.b = b;
    this.c = c;
    this.d = d;
    this.e = e;
    this.f = f;
  }

  /**
   * 矩阵乘法
   * @param {AffineMatrix} other 另一个矩阵
   * @returns {AffineMatrix} 乘积矩阵
   */
  multiply(other) {
    if (!other) return this;

    const aa = this.a * other.a + this.c * other.b;
    const bb = this.b * other.a + this.d * other.b;
    const cc = this.a * other.c + this.c * other.d;
    const dd = this.b * other.c + this.d * other.d;
    const ee = this.a * other.e + this.c * other.f + this.e;
    const ff = this.b * other.e + this.d * other.f + this.f;

    return new AffineMatrix(aa, bb, cc, dd, ee, ff);
  }

  /**
   * 应用变换到点
   * @param {Object} point 点坐标 {x, y}
   * @returns {Object} 变换后的点坐标
   */
  apply(point) {
    const x = point.x * this.a + point.y * this.c + this.e;
    const y = point.x * this.b + point.y * this.d + this.f;
    return { x: Math.round(x), y: Math.round(y) };
  }

  /**
   * 创建平移矩阵
   * @param {number} x X轴平移量
   * @param {number} y Y轴平移量
   * @returns {AffineMatrix} 平移矩阵
   */
  static translate(x, y) {
    return new AffineMatrix(1, 0, 0, 1, x, y);
  }

  /**
   * 创建缩放矩阵
   * @param {number} x X轴缩放因子
   * @param {number} y Y轴缩放因子
   * @returns {AffineMatrix} 缩放矩阵
   */
  static scale(x, y) {
    return new AffineMatrix(x, 0, 0, y, 0, 0);
  }

  /**
   * 创建旋转矩阵
   * @param {number} degrees 旋转角度（度）
   * @returns {AffineMatrix} 旋转矩阵
   */
  static rotate(degrees) {
    const rad = (degrees * Math.PI) / 180;
    const cos = Math.cos(rad);
    const sin = Math.sin(rad);
    return new AffineMatrix(cos, sin, -sin, cos, 0, 0);
  }

  /**
   * NDC坐标转换 - 从像素到标准化设备坐标
   * @param {Object} size 尺寸 {width, height}
   * @returns {AffineMatrix} 转换矩阵
   */
  static ndcFromPixels(size) {
    const w = size.width;
    const h = size.height;
    return new AffineMatrix(1 / w, 0, 0, -1 / h, 0, 1);
  }

  /**
   * NDC坐标转换 - 从标准化设备坐标到像素
   * @param {Object} size 尺寸 {width, height}
   * @returns {AffineMatrix} 转换矩阵
   */
  static ndcToPixels(size) {
    const w = size.width;
    const h = size.height;
    return new AffineMatrix(w, 0, 0, -h, 0, h);
  }

  /**
   * 转换为CSS transform字符串
   * @returns {string} CSS matrix字符串
   */
  toCSSMatrix() {
    return `matrix(${this.a}, ${this.b}, ${this.c}, ${this.d}, ${this.e}, ${this.f})`;
  }

  /**
   * 单位矩阵
   * @returns {AffineMatrix} 单位矩阵
   */
  static identity() {
    return new AffineMatrix(1, 0, 0, 1, 0, 0);
  }

  /**
   * 创建旋转变换矩阵（针对特定角度优化）
   * @param {number} rotation 旋转角度 (0, 90, 180, 270)
   * @returns {AffineMatrix} 旋转变换矩阵
   */
  static createRotationTransform(rotation) {
    switch (rotation) {
      case -90:
        // 90度旋转：点(x,y) -> 点(y, width-x)
        // 在NDC空间中：(u,v) -> (v, 1-u)
        return new AffineMatrix(0, -1, 1, 0, 0, 1);
      case 90:
        // 90度旋转：点(x,y) -> 点(y, width-x)
        // 在NDC空间中：(u,v) -> (v, 1-u)
        return new AffineMatrix(0, 1, -1, 0, 1, 0);
      case 180:
        // 180度旋转：点(x,y) -> 点(width-x, height-y)
        // 在NDC空间中：(u,v) -> (1-u, 1-v)
        return new AffineMatrix(-1, 0, 0, -1, 1, 1);
      case 270:
        // 270度旋转：点(x,y) -> 点(height-y, x)
        // 在NDC空间中：(u,v) -> (1-v, u)
        return new AffineMatrix(0, -1, 1, 0, 0, 1);
      default:
        // 0度或其他角度
        return AffineMatrix.identity();
    }
  }

  /**
   * 克隆矩阵
   * @returns {AffineMatrix} 克隆的矩阵
   */
  clone() {
    return new AffineMatrix(this.a, this.b, this.c, this.d, this.e, this.f);
  }

  /**
   * 矩阵是否相等
   * @param {AffineMatrix} other 另一个矩阵
   * @returns {boolean} 是否相等
   */
  equals(other) {
    if (!other) return false;
    return (
      this.a === other.a &&
      this.b === other.b &&
      this.c === other.c &&
      this.d === other.d &&
      this.e === other.e &&
      this.f === other.f
    );
  }

  /**
   * 转换为字符串（用于调试）
   * @returns {string} 矩阵字符串表示
   */
  toString() {
    return `Matrix(${this.a}, ${this.b}, ${this.c}, ${this.d}, ${this.e}, ${this.f})`;
  }
}

/**
 * 矩阵缓存管理器
 * 缓存常用的变换矩阵以提高性能
 */
export class MatrixCache {
  constructor() {
    this.cache = new Map();
    this.maxSize = 100; // 最大缓存数量
  }

  /**
   * 获取变换矩阵（优先从缓存获取）
   * @param {Object} videoSize 视频尺寸 {width, height}
   * @param {Object} targetSize 目标尺寸 {width, height}
   * @param {number} rotation 旋转角度
   * @returns {PositionMapper} 位置映射器
   */
  getTransform(videoSize, targetSize, rotation) {
    const key = `${videoSize.width}x${videoSize.height}-${targetSize.width}x${targetSize.height}-${rotation}`;

    if (!this.cache.has(key)) {
      const transform = this.calculateTransform(
        videoSize,
        targetSize,
        rotation
      );
      this.setCache(key, transform);
      console.log(`🔧 [缓存] 创建变换矩阵: ${key}`);
    }

    return this.cache.get(key);
  }

  /**
   * 计算变换矩阵
   * @param {Object} videoSize 视频尺寸
   * @param {Object} targetSize 目标尺寸
   * @param {number} rotation 旋转角度
   * @returns {PositionMapper} 位置映射器
   */
  calculateTransform(videoSize, targetSize, rotation) {
    let filterTransform = null;

    // 如果有旋转，创建旋转变换
    if (rotation !== 0) {
      filterTransform = AffineMatrix.createRotationTransform(rotation);
    }

    return PositionMapper.create(videoSize, filterTransform, targetSize);
  }

  /**
   * 设置缓存（带大小限制）
   * @param {string} key 缓存键
   * @param {*} value 缓存值
   */
  setCache(key, value) {
    // 如果缓存已满，删除最老的条目
    if (this.cache.size >= this.maxSize) {
      const firstKey = this.cache.keys().next().value;
      this.cache.delete(firstKey);
    }

    this.cache.set(key, value);
  }

  /**
   * 清空缓存
   */
  clear() {
    this.cache.clear();
    console.log("🔧 [缓存] 清空变换矩阵缓存");
  }

  /**
   * 获取缓存统计信息
   * @returns {Object} 缓存统计
   */
  getStats() {
    return {
      size: this.cache.size,
      maxSize: this.maxSize,
      keys: Array.from(this.cache.keys())
    };
  }

  /**
   * 删除特定模式的缓存
   * @param {string} pattern 模式匹配字符串
   */
  deletePattern(pattern) {
    const keysToDelete = [];
    for (const key of this.cache.keys()) {
      if (key.includes(pattern)) {
        keysToDelete.push(key);
      }
    }

    keysToDelete.forEach(key => this.cache.delete(key));
    console.log(
      `🔧 [缓存] 删除模式 "${pattern}" 匹配的 ${keysToDelete.length} 个缓存条目`
    );
  }
}

/**
 * 位置映射器类
 * 处理不同分辨率之间的坐标映射
 */
export class PositionMapper {
  constructor(videoSize, transform) {
    this.videoSize = videoSize;
    this.transform = transform;
  }

  /**
   * 创建位置映射器
   * @param {Object} videoSize 视频尺寸
   * @param {AffineMatrix} filterTransform 过滤变换矩阵
   * @param {Object} targetSize 目标尺寸
   * @returns {PositionMapper} 位置映射器
   */
  static create(videoSize, filterTransform, targetSize) {
    const needsConversion =
      !this.sizeEquals(videoSize, targetSize) || filterTransform;

    let transform = filterTransform || AffineMatrix.identity();

    if (needsConversion) {
      const inputTransform = AffineMatrix.ndcFromPixels(videoSize);
      const outputTransform = AffineMatrix.ndcToPixels(targetSize);

      // 组合变换：output * filter * input
      transform = outputTransform
        .multiply(filterTransform || AffineMatrix.identity())
        .multiply(inputTransform);
    }

    return new PositionMapper(videoSize, transform);
  }

  /**
   * 映射位置
   * @param {Object} position 位置对象 {point: {x, y}, screenSize: {width, height}}
   * @returns {Object} 映射后的点坐标
   */
  map(position) {
    // 检查分辨率匹配
    if (!PositionMapper.sizeEquals(this.videoSize, position.screenSize)) {
      console.warn(
        `⚠️ 分辨率不匹配: 视频${this.videoSize.width}x${this.videoSize.height} vs 位置${position.screenSize.width}x${position.screenSize.height}`
      );
      return null;
    }

    let point = position.point;

    // 应用变换矩阵
    if (this.transform) {
      point = this.transform.apply(point);
    }

    return point;
  }

  /**
   * 比较两个尺寸是否相等
   * @param {Object} size1 尺寸1
   * @param {Object} size2 尺寸2
   * @returns {boolean} 是否相等
   */
  static sizeEquals(size1, size2) {
    return size1.width === size2.width && size1.height === size2.height;
  }
}

// 导出默认矩阵缓存实例
export const matrixCache = new MatrixCache();
