/**
 * CommonJS兼容的页面生成器包装器
 * 为了在server-new.js中使用ES6模块的page-generator功能
 */

const fs = require('fs');
const path = require('path');

// 动态导入ES6模块
let pageGenerator = null;
let initError = null;

async function initializePageGenerator() {
    if (initError) {
        throw initError;
    }
    
    if (!pageGenerator) {
        try {
            console.log('🔧 Initializing page generator...');
            const module = await import('./page-generator.js');
            pageGenerator = module.pageGenerator;
            console.log('✅ Page generator initialized successfully');
        } catch (error) {
            console.error('❌ Failed to initialize page generator:', error);
            initError = error;
            throw error;
        }
    }
    return pageGenerator;
}

/**
 * 生成单个设备窗口HTML（简化版回退）
 * @param {number} deviceNum 设备编号
 * @returns {string} 设备窗口HTML
 */
function generateFallbackDeviceWindow(deviceNum) {
    return `
        <!-- 设备${deviceNum} (Fallback) -->
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
                    </select>
                    <select id="resolutionSelect${deviceNum}" onchange="changeResolution('device${deviceNum}')" class="resolution_select">
                        <option value="1080x1920">1080x1920 (FHD)</option>
                        <option value="720x1280">720x1280 (HD)</option>
                        <option value="480x854">480x854 (FWVGA)</option>
                    </select>
                </div>
            </div>
            
            <div class="encoder_config_panel">
                <div class="encoder_config_title">🔧 编码器配置</div>
                <div class="encoder_controls">
                    <select id="bitrateSelect${deviceNum}" onchange="changeBitrate('device${deviceNum}')" class="bitrate_select" title="目标码率">
                        <option value="4000">4M</option>
                        <option value="5000" selected>5M</option>
                        <option value="8000">8M</option>
                        <option value="10000">10M</option>
                    </select>
                    
                    <select id="keyFrameSelect${deviceNum}" onchange="changeKeyFrame('device${deviceNum}')" class="keyframe_select" title="关键帧间隔">
                        <option value="5">5s间隔</option>
                        <option value="10" selected>10s间隔</option>
                        <option value="20">20s间隔</option>
                    </select>
                    
                    <select id="qualitySelect${deviceNum}" onchange="changeQuality('device${deviceNum}')" class="quality_select" title="编码质量">
                        <option value="50">质量50%</option>
                        <option value="70">质量70%</option>
                        <option value="80" selected>质量80%</option>
                        <option value="90">质量90%</option>
                    </select>
                    
                    <select id="encoderFpsSelect${deviceNum}" onchange="changeEncoderFps('device${deviceNum}')" class="encoder_fps_select" title="目标帧率">
                        <option value="15">15 FPS</option>
                        <option value="30" selected>30 FPS</option>
                        <option value="60">60 FPS</option>
                    </select>
                    
                    <select id="rotationSelect${deviceNum}" onchange="changeRotation('device${deviceNum}')" class="rotation_select" title="画面旋转">
                        <option value="0" selected>0°</option>
                        <option value="90">90°</option>
                        <option value="180">180°</option>
                        <option value="270">270°</option>
                    </select>
                    
                    <select id="scaleSelect${deviceNum}" class="scale_select" title="视频缩放比例">
                        <option value="0.5">50%</option>
                        <option value="0.8">80%</option>
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

/**
 * 生成单个设备窗口HTML
 * @param {number} deviceNum 设备编号
 * @returns {Promise<string>} 设备窗口HTML
 */
async function generateDeviceWindow(deviceNum) {
    try {
        const generator = await initializePageGenerator();
        return generator.generateDeviceWindow(deviceNum);
    } catch (error) {
        console.warn('⚠️ Using fallback device window generator:', error.message);
        return generateFallbackDeviceWindow(deviceNum);
    }
}

/**
 * 生成多个设备窗口HTML
 * @param {number} windowCount 窗口数量
 * @returns {Promise<string>} 设备窗口HTML字符串
 */
async function generateDeviceWindows(windowCount) {
    const validWindowCount = Math.min(Math.max(1, parseInt(windowCount) || 1), 12);
    let deviceWindowsHtml = '';
    
    for (let i = 1; i <= validWindowCount; i++) {
        deviceWindowsHtml += await generateDeviceWindow(i);
    }
    
    return deviceWindowsHtml;
}

/**
 * 生成完整的多窗口页面
 * @param {number} windowCount 窗口数量
 * @returns {Promise<string>} 完整HTML页面
 */
async function generateMultiWindowPage(windowCount) {
    try {
        const generator = await initializePageGenerator();
        return generator.generateMultiWindowPage(windowCount);
    } catch (error) {
        console.warn('⚠️ Page generator not available, using simplified version:', error.message);
        // 如果页面生成器不可用，返回简化版本
        const deviceWindows = await generateDeviceWindows(windowCount);
        return `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>WebScreen - ${windowCount}个窗口</title>
    <link rel="stylesheet" href="css/main.css">
</head>
<body>
    <div class="page_header">
        <h1>📱 多设备投屏监控 - ${windowCount}个窗口</h1>
    </div>
    <div class="multi_window_container">
        ${deviceWindows}
    </div>
    <script src="js/adapter-latest.js"></script>
    <script src="js/video-stats-enhanced.js"></script>
</body>
</html>`;
    }
}

/**
 * 生成新架构页面HTML
 * @param {number} windowCount 窗口数量
 * @returns {Promise<string>} 完整的HTML页面
 */
async function generateNewArchitecturePage(windowCount) {
    try {
        const generator = await initializePageGenerator();
        
        // 在Node.js环境中读取CSS文件内容
        let cssContent = '';
        try {
            const cssPath = path.join(process.cwd(), 'public/css/main.css');
            cssContent = fs.readFileSync(cssPath, 'utf8');
        } catch (err) {
            console.warn('⚠️ Failed to read CSS file:', err.message);
        }
        
        return await generator.generateNewArchitecturePage(windowCount, cssContent);
    } catch (error) {
        console.warn('⚠️ New architecture page generator not available, using fallback:', error.message);
        // 如果新架构页面生成器不可用，回退到普通多窗口页面
        return await generateMultiWindowPage(windowCount);
    }
}

module.exports = {
    generateDeviceWindow,
    generateDeviceWindows,
    generateMultiWindowPage,
    generateNewArchitecturePage
}; 