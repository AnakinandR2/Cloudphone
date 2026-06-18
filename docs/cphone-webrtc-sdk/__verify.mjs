// SDK API 可用性验证 harness(Node 无浏览器环境,mock window/devices)
// 验证:① 15 方法结构存在 ② 纯方法逻辑实跑 ③ deviceId 归一化(1/"1"/"device1")
// 注:真实 WebRTC 连接(WebSocket/RTCPeerConnection/视频)在 device-manager.js,需浏览器+真实设备,本脚本不覆盖
globalThis.window = {};
globalThis.document = { addEventListener(){}, createElement(){return {style:{}};}, getElementById(){return null;} };

await import('./index.js');
const sdk = globalThis.window.CphoneWebRTC;

let pass=0, fail=0;
const P=(n,d='')=>{pass++; console.log(`  [PASS] ${n}${d?' — '+d:''}`);};
const F=(n,d='')=>{fail++; console.log(`  [FAIL] ${n}${d?' — '+d:''}`);};
const eq=(a,b)=>JSON.stringify(a)===JSON.stringify(b);

// ── ① 结构:15 方法全在且可调用 ──
console.log('=== ① 公开方法结构(15 个)===');
const METHODS=['init','sendControlMessage','setMain','destroy','startSync','stopSync','connectAll',
  'addDevice','updateConfig','getDeviceIsLandscape','getVideoResolution','checkVideoSizeChanges',
  'rotateDevice','getRotation','getStreamSize'];
for (const m of METHODS) (typeof sdk[m]==='function') ? P(m+'()') : F(m+'()','缺失或非函数');
// 摄像头方法结构(8 个,2026-06-11 从新版 xx-sdk 移植)
console.log('--- 摄像头方法(8 个)---');
const CAMERA_METHODS=['sendCameraMessage','getCameraList','openCamera','closeCamera',
  'switchToCamera','getCameraStatus','toggleCameraMirror','getCameraMirrorStatus'];
for (const m of CAMERA_METHODS) (typeof sdk[m]==='function') ? P(m+'()') : F(m+'()','缺失或非函数');

// ── mock 运行环境 ──
let sent=[];
let camCalls=[];
const mkDev=(rot=0,vw=720,vh=1280)=>({
  rotation:rot, lastVideoWidth:vw, lastVideoHeight:vh,
  elements:{remoteVideo:{videoWidth:vw, videoHeight:vh}},
  dataChannel:{readyState:'open', send:s=>sent.push(JSON.parse(s))},
  updateRotation(r){this.rotation=r;}, checkVideoSizeChanges(){return 'checked';},
  // 摄像头 mock(index.js 委托到 device.xxx,device-manager 真实实现需浏览器)
  cameraEnabled:false, selectedCameraDeviceId:'cam-abc', cameraMirrorEnabled:true,
  async getCameraList(){camCalls.push('getCameraList'); return [{deviceId:'cam-abc',label:'FaceTime'}];},
  async openCamera(id){camCalls.push('openCamera:'+id); this.cameraEnabled=true; return true;},
  async closeCamera(){camCalls.push('closeCamera'); this.cameraEnabled=false;},
  async switchToCamera(id){camCalls.push('switchToCamera:'+id); return true;},
  async toggleCameraMirror(e){camCalls.push('toggleCameraMirror:'+e); this.cameraMirrorEnabled=!this.cameraMirrorEnabled; return this.cameraMirrorEnabled;},
  getCameraMirrorStatus(){return this.cameraMirrorEnabled;},
});
const reset=()=>{
  sent=[];
  window.devices={ device1:mkDev(), device2:mkDev() };
  window.CphoneWebRTCConfig={ roomList:[{deviceId:1,isMain:true},{deviceId:2,isMain:false}] };
  window.getApp=()=>({ deviceControllers:[
    {deviceId:'device1', deviceManager:{dataChannel:{send:s=>sent.push(JSON.parse(s))}, width:720, height:1280}},
    {deviceId:'device2', deviceManager:{dataChannel:{send:s=>sent.push(JSON.parse(s))}, width:720, height:1280}},
  ]});
  window.connectDevice=k=>sent.push('connect:'+k);
  window.addDevice=d=>sent.push('add:'+d);
};

// ── ② 纯方法逻辑实跑 ──
console.log('\n=== ② 方法逻辑实跑(mock devices)===');

// sendControlMessage:字符串→{type},发到正确 device(isSync 默认 false 只发单机)
reset(); sdk.stopSync();
sdk.sendControlMessage(2,'button_home');
eq(sent,[{type:'button_home'}]) ? P('sendControlMessage 字符串→对象 + 单机下发') : F('sendControlMessage',JSON.stringify(sent));

// sendControlMessage 对象参数
reset(); sdk.stopSync();
sdk.sendControlMessage(2,{type:'key_char',char:'a',code:'KeyA'});
eq(sent,[{type:'key_char',char:'a',code:'KeyA'}]) ? P('sendControlMessage 对象参数') : F('sendControlMessage obj',JSON.stringify(sent));

// 群控:startSync + 主控设备 → 广播所有
reset(); sdk.startSync();
sdk.sendControlMessage(1,'button_back');   // device1 isMain
sent.length===2 ? P('startSync 主控广播(2 设备各收 1)') : F('群控广播',`收到 ${sent.length}`);
sdk.stopSync();

// getDeviceIsLandscape
reset();
window.devices.device1=mkDev(0,1280,720); // 横
let lsLand=sdk.getDeviceIsLandscape(1);
window.devices.device1=mkDev(0,720,1280); // 竖
let lsPort=sdk.getDeviceIsLandscape(1);
(lsLand===true && lsPort===false) ? P('getDeviceIsLandscape 横/竖判断') : F('getDeviceIsLandscape',`横=${lsLand} 竖=${lsPort}`);

// getRotation / getVideoResolution(都返回 rotation)
reset(); window.devices.device1=mkDev(-90);
(sdk.getRotation('device1')===-90 && sdk.getVideoResolution(1)===-90)
  ? P('getRotation / getVideoResolution 返回 rotation') : F('getRotation',`${sdk.getRotation('device1')}`);

// getStreamSize
reset();
eq(sdk.getStreamSize('device1'),{width:720,height:1280}) ? P('getStreamSize 返回宽高') : F('getStreamSize');

// checkVideoSizeChanges
reset();
sdk.checkVideoSizeChanges(1)==='checked' ? P('checkVideoSizeChanges 触发') : F('checkVideoSizeChanges');

// rotateDevice(无 globalRotateDevice → 走 updateRotation,0→-90,返回 true)
reset(); window.globalRotateDevice=undefined; window.devices.device1=mkDev(0);
let rr=sdk.rotateDevice(1);
(rr===true && window.devices.device1.rotation===-90) ? P('rotateDevice 0→-90 返回 true') : F('rotateDevice',`ret=${rr} rot=${window.devices.device1.rotation}`);

// updateConfig(id):发 encoder_config_update 到指定设备
reset();
sdk.updateConfig({width:1080,height:1920,qualityLevel:50,targetFps:30},1);
let ec=sent.find(x=>x.type==='encoder_config_update');
(ec && ec.config.resolution.width===1080 && ec.config.qualityLevel===50) ? P('updateConfig 发 encoder_config_update') : F('updateConfig',JSON.stringify(sent));

// connectAll:对每个 device 调 connectDevice
reset();
sdk.connectAll();
(sent.includes('connect:device1') && sent.includes('connect:device2')) ? P('connectAll 连接所有设备') : F('connectAll',JSON.stringify(sent));

// addDevice:委托 window.addDevice
reset();
sdk.addDevice(3);
sent.includes('add:3') ? P('addDevice 委托') : F('addDevice',JSON.stringify(sent));

// startSync/stopSync 状态
sdk.startSync(); let s1=sdk.isSync; sdk.stopSync(); let s2=sdk.isSync;
(s1===true && s2===false) ? P('startSync/stopSync 切换 isSync') : F('sync state');

// ── ③ deviceId 归一化:三种写法等价 ──
console.log('\n=== ③ deviceId 归一化(1 / "1" / "device1" 等价)===');
for (const form of [2,'2','device2']) {
  reset(); sdk.stopSync();
  sdk.sendControlMessage(form,'button_recent');
  eq(sent,[{type:'button_recent'}]) ? P(`sendControlMessage(${JSON.stringify(form)})`) : F(`norm ${form}`,JSON.stringify(sent));
}
reset(); window.devices.device2=mkDev(90);
[2,'2','device2'].every(f=>sdk.getRotation(f)===90) ? P('getRotation 三种写法一致') : F('getRotation norm');

// ── ④ 摄像头方法逻辑(mock device,真实采集/注入需浏览器)──
console.log('\n=== ④ 摄像头方法逻辑实跑(mock device)===');
// sendCameraMessage:发 camera_control 信令(底层,不采集)
reset(); camCalls=[]; sdk.stopSync();
sdk.sendCameraMessage(2,'open');
eq(sent,[{type:'camera_control',action:'open'}]) ? P('sendCameraMessage 发 camera_control open') : F('sendCameraMessage',JSON.stringify(sent));
// sendCameraMessage 非法 action 拒绝
reset(); sdk.sendCameraMessage(2,'xxx'); eq(sent,[]) ? P('sendCameraMessage 拒绝非法 action') : F('sendCameraMessage 非法',JSON.stringify(sent));
// openCamera / closeCamera / getCameraList / switchToCamera / toggleCameraMirror 委托
reset(); camCalls=[];
await sdk.openCamera(2,'cam-abc'); await sdk.getCameraList(2); await sdk.switchToCamera(2,'cam-x'); await sdk.toggleCameraMirror(2,true); await sdk.closeCamera(2);
(camCalls.includes('openCamera:cam-abc') && camCalls.includes('getCameraList') && camCalls.includes('switchToCamera:cam-x') && camCalls.includes('toggleCameraMirror:true') && camCalls.includes('closeCamera'))
  ? P('openCamera/getCameraList/switchToCamera/toggleCameraMirror/closeCamera 委托设备') : F('camera 委托',JSON.stringify(camCalls));
// getCameraStatus:存在设备返回状态
reset(); window.devices.device2.cameraEnabled=true;
const st=sdk.getCameraStatus(2);
(st.enabled===true && st.selectedDeviceId==='cam-abc' && st.mirrorEnabled===true) ? P('getCameraStatus 返回状态') : F('getCameraStatus',JSON.stringify(st));
// getCameraStatus / getCameraMirrorStatus:缺设备优雅降级
reset();
(sdk.getCameraStatus(99).enabled===false && sdk.getCameraMirrorStatus(99)===true) ? P('摄像头查询 缺设备优雅降级') : F('camera 降级');
// switchToCamera 无 cameraDeviceId 抛错
reset(); let threw=false; try{ await sdk.switchToCamera(2); }catch(e){ threw=true; }
threw ? P('switchToCamera 无 ID 抛错') : F('switchToCamera 校验');
// 摄像头方法 deviceId 归一化
reset(); camCalls=[]; await sdk.openCamera('device2'); camCalls.includes('openCamera:null') ? P('openCamera deviceId 归一化(device2)') : F('camera norm',JSON.stringify(camCalls));

// ── 需 live/浏览器 的方法(标注,不算失败)──
console.log('\n=== ⑤ 需浏览器/main.js 才能验(已标注)===');
console.log('  [SKIP] init() — 调 initApp(来自 main.js),真实启动需浏览器');
console.log('  [SKIP] destroy() — 依赖 MouseControllerFactory 等(main.js 工厂)');
console.log('  [SKIP] setMain() — 走 updateConfig,逻辑同 updateConfig(已验)');
console.log('  [INFO] 真实 WebRTC 连接(WS/RTCPeerConnection/视频/DataChannel)在 device-manager.js,需浏览器+真实设备');
console.log('  [INFO] 摄像头真实采集/注入(getUserMedia/replaceTrack/PCM)在 device-manager.js,需浏览器(下一步用 fake camera 实测)');

console.log(`\n${'='.repeat(56)}`);
console.log(`SDK API 验证: PASS=${pass} FAIL=${fail}`);
process.exit(fail?1:0);
