# Android Agent

Android 端自动化 Agent，通过 Lua 脚本驱动 TikTok 等 App 完成注册、登录、发布视频等自动化任务。

---

## 目录

- [项目结构概览](#项目结构概览)
- [Lua 脚本执行原理](#lua-脚本执行原理)
  - [整体调用链](#整体调用链)
  - [第一层：RPC 入口 — LuaController](#第一层rpc-入口--luacontroller)
  - [第二层：任务队列 — LuaTaskQueue](#第二层任务队列--luataskqueue)
  - [第三层：执行引擎 — LuaExecutor](#第三层执行引擎--luaexecutor)
  - [第四层：Native 层 — lua_native.cpp](#第四层native-层--lua_nativecpp)
  - [第五层：Java 能力层 — LuaBridge](#第五层java-能力层--luabridge)
- [Lua 可用函数速查](#lua-可用函数速查)
- [Lua 模块系统（require）](#lua-模块系统require)
- [日志与结果系统](#日志与结果系统)
- [脚本中断机制](#脚本中断机制)
- [UI 元素获取原理](#ui-元素获取原理)
- [文本输入原理](#文本输入原理)
- [上传进度监控原理](#上传进度监控原理)

---

## 项目结构概览

```
android_agent/
├── lua_script/                  # Lua 业务脚本
│   ├── temp/                    # 当前使用的脚本版本
│   │   ├── create_temp.lua      # 发布视频流程
│   │   ├── login_temp.lua       # 登录流程
│   │   └── regist_temp.lua      # 注册流程
│   └── ...
├── protocollib/
│   └── src/main/
│       ├── java/xiaoxi/
│       │   ├── lua/
│       │   │   ├── LuaController.java   # RPC 入口
│       │   │   ├── LuaTaskQueue.java    # 任务队列
│       │   │   ├── LuaExecutor.java     # 执行引擎（JNI 封装）
│       │   │   ├── LuaBridge.java       # Java 能力层
│       │   │   └── LuaLogManager.java   # 日志管理
│       │   ├── manager/
│       │   │   ├── UIElementsManager.java        # UI 元素获取
│       │   │   └── NotificationListenerManager.java  # 通知监听
│       │   └── service/
│       │       └── UIAccessibilityService.java   # 无障碍服务
│       ├── cpp/
│       │   └── lua_native.cpp           # JNI 桥接 + Lua 函数注册
│       └── assets/
│           └── lua_modules/             # 可 require 的 Lua 模块
```

---

## Lua 脚本执行原理

### 整体调用链

```
外部调用方（RPC）
    │  HTTP/RPC 请求，携带 script + taskId
    ▼
LuaController.exec()               ← RPC 入口，解压脚本，委托给队列
    │
    ▼
LuaTaskQueue.submitTask()          ← 放入 LinkedBlockingQueue，立即返回 taskId
    │  （单线程队列处理器阻塞取任务）
    ▼
LuaTaskQueue.executeTask()         ← 串行执行，保证同时只有一个脚本运行
    │
    ▼
LuaExecutor.executeLuaScriptWithTracking()   ← 启动日志会话，提交到执行线程池
    │
    ▼
LuaExecutor.executeLuaScript()     ← native 方法，进入 C++ 层
    │  (JNI)
    ▼
lua_native.cpp :: Java_xiaoxi_lua_LuaExecutor_executeLuaScript()
    │  luaInit() 初始化 Lua VM，注册所有全局函数
    │  用 xpcall 包裹脚本，lua_pcall() 执行
    ▼
Lua 脚本运行
    │  调用全局函数，如 input_text("xxx")
    ▼
lua_native.cpp :: input_text()     ← C++ 函数，从 Lua 栈取参数
    │  callJavaVoidMethod(env, L, "inputText", ...)
    │  (JNI 反射)
    ▼
LuaBridge.inputText(String text)   ← Java 静态方法，执行 Android 操作
    │
    ▼
AccessibilityService / ClipboardManager / Shell 等
```

---

### 第一层：RPC 入口 — `LuaController`

**文件**：`protocollib/src/main/java/xiaoxi/controller/LuaController.java`

对外暴露以下 RPC 接口（通过 `@RPC` 注解注册）：

| 接口 | 说明 |
|------|------|
| `exec` | 提交脚本到队列执行（主要接口），支持 gzip+Base64 压缩传输 |
| `cancel` | 取消任务：若在队列中直接移除，若正在执行则发送中断信号 |
| `status` | 查询当前执行状态、队列大小、线程 ID 等 |
| `queue_status` | 查询队列详情 |
| `queue_clear` | 清空待执行队列（不影响正在执行的任务） |
| `task_result` | 查询指定 taskId 的执行结果（读日志文件） |

脚本传输支持 **gzip 压缩 + Base64 编码**，减少网络传输体积：

```java
// LuaController.exec()
if (compressed) {
    script = decompressScript(script);  // Base64 解码 → gzip 解压
}
return LuaTaskQueue.getInstance().submitTask(taskId, script, timeout);
```

---

### 第二层：任务队列 — `LuaTaskQueue`

**文件**：`protocollib/src/main/java/xiaoxi/lua/LuaTaskQueue.java`

**核心设计**：使用 `LinkedBlockingQueue` + 单线程处理器，**保证同时只有一个脚本在执行**。

```
taskQueue (LinkedBlockingQueue)
    ┌──────────────────────────────┐
    │  ScriptTask(taskId, script)  │  ← submitTask() 放入
    │  ScriptTask(taskId, script)  │
    │  ...                         │
    └──────────────────────────────┘
              │ take()（阻塞）
              ▼
    LuaTaskQueueProcessor（单线程）
              │
              ▼
         executeTask()  →  LuaExecutor（同步阻塞）
```

**任务生命周期**：

```
提交 → queued（在队列中）
     → executing（正在执行）
     → success / failed（执行完成，写入日志文件）
```

**任务结果查询**（`getTaskResult`）优先级：
1. 检查是否是 `currentTask`（正在执行）→ 返回 `executing`
2. 检查是否在 `taskQueue` 中 → 返回 `queued`
3. 读取日志文件 `/data/local/tmp/log/lua_logs/lua_{taskId}.log` → 解析成功/失败标记

**线程安全**：队列处理器线程意外退出后会**自动重启**（`submitQueueProcessorLoop` 的 `finally` 块）。

---

### 第三层：执行引擎 — `LuaExecutor`

**文件**：`protocollib/src/main/java/xiaoxi/lua/LuaExecutor.java`

**职责**：
- 加载 `lua_native` so 库
- 从 `assets/lua_modules/` 递归读取所有 `.lua` 模块，通过 `setLuaModulesNative` 注册到 Lua 的 `package.preload`（只在首次执行时加载一次）
- 在独立线程池（`LuaExecutor-Thread`）中执行脚本，支持超时监控
- 提供 `interruptLuaScriptExecution()` 中断接口

**执行流程**：

```java
// executeLuaScriptWithTracking()
logManager.startSession(taskId);          // 开启日志会话
currentExecutionFuture = luaExecutorService.submit(() -> {
    return executeLuaScript(script);      // 调用 native 方法（阻塞）
});
return currentExecutionFuture.get();      // 等待完成
// native 执行完成后回调 onNativeExecutionCompleted()
// → logManager.endSession(success)
```

**超时机制**：启动独立监控线程，每 1000ms 检查一次是否超时，超时则调用 `interruptLuaScriptExecution()`。

---

### 第四层：Native 层 — `lua_native.cpp`

**文件**：`protocollib/src/main/cpp/lua_native.cpp`

这是整个调用链的核心桥接层，完成 **Lua ↔ Java** 的双向通信。

#### 初始化（`JNI_OnLoad`）

so 库被加载时执行，保存全局引用：

```cpp
// 保存 JavaVM 指针（用于后续在任意线程获取 JNIEnv）
g_vm = vm;
// 缓存 LuaBridge class 的全局引用（避免每次调用都 FindClass）
g_luaBridgeClass = (jclass)env->NewGlobalRef(env->FindClass("xiaoxi/lua/LuaBridge"));
g_luaLogManagerClass = (jclass)env->NewGlobalRef(env->FindClass("xiaoxi/lua/LuaLogManager"));
```

#### Lua VM 初始化（`luaInit`）

首次执行脚本时调用，只初始化一次（全局单例 `g_luaState`）：

```cpp
g_luaState = luaL_newstate();
luaL_openlibs(g_luaState);                          // 开启标准库
lua_sethook(g_luaState, lua_hook_check_interrupt,   // 注册中断钩子
            LUA_MASKCOUNT, 50);                      // 每执行50条指令检查一次

// 注册所有全局函数
const luaL_Reg functions[] = {
    {"input_text",   input_text},
    {"wait",         wait},
    {"log",          log_message},
    {"tap_element",  tap_element},
    // ... 共30+个函数
};
for (const luaL_Reg* func = functions; func->name; ++func) {
    lua_register(g_luaState, func->name, func->func);
}
```

#### 脚本执行

脚本被包裹在 `xpcall` 中执行，确保 Lua 错误被捕获并返回堆栈信息：

```lua
-- C++ 自动包裹的外层代码
local status, result = xpcall(
  function()
    -- 用户脚本内容
  end,
  function(err)
    return debug.traceback(tostring(err), 2)  -- 捕获错误堆栈
  end
)
if not status then
  return 'Error: ' .. tostring(result)
end
return result
```

#### C++ 函数的固定模式

每个注册到 Lua 的 C++ 函数都遵循相同的模式：

```cpp
static int input_text(lua_State* L) {
    // 1. 从 Lua 栈取参数
    const char* text = luaL_checkstring(L, 1);

    // 2. 获取 JNI 环境（RAII，自动 Attach/Detach 线程）
    JNIEnvGuard env;
    if (!env) return 0;

    // 3. 转换参数类型
    LocalRefGuard<jstring> jText(env.get(), env->NewStringUTF(text));

    // 4. 反射调用 LuaBridge 的静态方法
    callJavaVoidMethod(env.get(), L,
        "inputText",              // 方法名
        "(Ljava/lang/String;)V",  // JNI 签名
        jText.get());             // 参数

    return 0;  // 无返回值
}
```

三个通用调用模板根据返回类型选择：

| 模板 | 对应 Java 返回类型 |
|------|-------------------|
| `callJavaVoidMethod` | `void` |
| `callJavaStringMethod` | `String` |
| `callJavaBooleanMethod` | `boolean` |
| `callJavaIntMethod` | `int` |

#### 中断机制

通过 Lua Hook 实现：每执行 50 条 Lua 指令检查一次中断标志位：

```cpp
static void lua_hook_check_interrupt(lua_State* L, lua_Debug* ar) {
    if (g_lua_interrupt_flag) {
        g_lua_interrupt_flag = 0;
        luaL_error(L, "Script interrupted");  // 抛出 Lua 错误，终止执行
    }
}

// Java 调用 interruptLuaScript() 时设置标志位
g_lua_interrupt_flag = 1;
```

---

### 第五层：Java 能力层 — `LuaBridge`

**文件**：`protocollib/src/main/java/xiaoxi/lua/LuaBridge.java`

所有方法均为 `public static`，由 C++ 通过 JNI 反射调用。封装了所有 Android 系统能力：

| 方法 | 能力 |
|------|------|
| `inputText(String)` | 文本输入（优先 AccessibilityService，回退剪贴板） |
| `tapCoordinate(x, y, duration)` | 坐标点击 |
| `humanTap(x, y, duration)` | 模拟人类点击（加随机抖动和压力） |
| `humanMove(x1,y1,x2,y2,...)` | 模拟人类滑动 |
| `waitByElementShown(filter, value, timeout)` | 等待 UI 元素出现 |
| `tapElement(filter, value)` | 查找并点击 UI 元素 |
| `getTextByElement(filter, value)` | 获取 UI 元素文本 |
| `getElementPosition(filter, value)` | 获取 UI 元素坐标 |
| `getUIHierarchy()` | 获取当前页面 UI 树（JSON） |
| `openApp(packageName)` | 启动 App |
| `closeApp(packageName)` | 关闭 App |
| `doShell(command)` | 执行 Shell 命令 |
| `sendHttp(method, url, headers, body)` | 发送 HTTP 请求 |
| `getClipBoardText()` | 读取剪贴板 |
| `getNotificationUploadStatus()` | 获取通知栏上传进度 |
| `reportResult(json)` | 上报脚本执行结果 |
| `log(message)` | 写入日志 |

---

## Lua 可用函数速查

以下函数在 Lua 脚本中直接作为全局函数调用：

```lua
-- 应用控制
open_app("com.zhiliaoapp.musically")
close_app("com.zhiliaoapp.musically")

-- 等待
wait(2000)                                          -- 等待毫秒数

-- 点击
tap_coordinate(540, 960, 100)                       -- 坐标点击
human_tap(540, 960, 120)                            -- 模拟人类点击
tap_element("text", "Next")                         -- 按文本点击元素
tap_element("id", "com.xxx:id/button")              -- 按 ID 点击元素

-- 滑动
move(540, 1200, 100, 300, 300, 600, 1, 50)
human_move(540, 1500, 540, 500)                     -- 模拟人类上滑

-- 等待元素
wait_element_shown("text", "Copy link", 5000)       -- 等待元素出现，超时5秒
wait_element_shown_or("text", "A", "text", "B", 5000)

-- 文本输入
input_text("hello@example.com")
input_keycode(4)                                    -- 发送按键（4=返回键）

-- UI 信息
get_text_by_element("text", "Email or username")   -- 获取元素文本
get_element_position("id", "com.xxx:id/btn")       -- 获取元素位置
get_ui_hierarchy()                                  -- 获取完整 UI 树 JSON

-- 剪贴板
get_clipboard_text()

-- 上传状态
get_notification_upload_status()                    -- 返回 "54%" 或 "complete"
reset_notification_upload_status()

-- 网络
send_http("POST", url, headers, body)

-- 文件
read_excel("/path/to/file.xlsx")
write_to_file("/path/to/file.txt", "content", true)

-- 工具
get_str_by_regex(text, pattern)                    -- 正则提取
get_screen_width()
get_screen_height()
do_shell("command")                                 -- 执行 Shell 命令

-- 日志与结果
log("消息内容")                                     -- 写入日志（同时输出 logcat）
report_result('{"key": "value"}')                  -- 上报结果 JSON
```

---

## Lua 模块系统（require）

`assets/lua_modules/` 目录下的 `.lua` 文件会在**首次执行脚本前**被自动加载，注册到 Lua 的 `package.preload`。

**加载流程**：

```
LuaExecutor.executeLuaScriptWithTracking()
    │  首次执行时
    ▼
loadModulesFromAssets(context)
    │  递归读取 assets/lua_modules/**/*.lua
    │  文件路径转换为模块名：tiktok/constants.lua → "tiktok.constants"
    ▼
setLuaModulesNative(modulesArray)
    │  (JNI)
    ▼
lua_native.cpp :: setLuaModulesNative()
    │  对每个模块执行：
    │  package.preload["tiktok.constants"] = function()
    │      local func = load(module_code)
    │      return func()
    │  end
    ▼
脚本中 require("tiktok.constants") 即可使用
```

---

## 日志与结果系统

**文件**：`protocollib/src/main/java/xiaoxi/lua/LuaLogManager.java`

每次脚本执行对应一个独立日志文件，路径为：

```
/data/local/tmp/log/lua_logs/lua_{taskId}.log
```

**日志文件结构**：

```
#-1710000000000-#                    ← 开始时间戳
[2026-03-12 10:00:00.000] [INFO] ... ← 普通日志行
[2026-03-12 10:00:01.000] [INFO] ...
#RESULT#{"key":"value"}#RESULT#      ← 脚本通过 report_result() 上报的结果
#-1710000060000-#                    ← 结束时间戳
##SUCCESS##                          ← 成功标记（或 ##FAILED##）
```

**结束时的自动操作**（`endSession`）：
1. 截图保存
2. 自动关闭 TikTok（`com.zhiliaoapp.musically` / `com.ss.android.ugc.trill`）
3. 如果失败且设置了 `clearProxyIfFailed`，自动清除代理
4. 写入结果和结束标记

---

## 脚本中断机制

支持两种中断方式，`cancel` 接口会同时触发：

**1. Native 中断标志位**（推荐）

```
cancel RPC → LuaExecutor.interruptLuaScriptExecution()
    → native interruptLuaScript()
    → g_lua_interrupt_flag = 1
    → Lua Hook 每50条指令检测到标志位
    → luaL_error(L, "Script interrupted")
    → xpcall 捕获，脚本终止
```

**2. Java Future 取消**

```
cancel RPC → currentExecutionFuture.cancel(true)
    → 向 LuaExecutor-Thread 发送 interrupt 信号
    → 线程中断，Future 抛出 CancellationException
```

**队列中的任务**：若任务还未开始执行，`cancel` 会直接从 `LinkedBlockingQueue` 中移除，无需中断。

---

## UI 元素获取原理

**文件**：`protocollib/src/main/java/xiaoxi/manager/UIElementsManager.java`

优先使用 `UIAccessibilityService`（无障碍服务），失败时回退到 `uiautomator dump`：

```
getUIElements()
    │  最多重试5次，间隔800ms
    ├─ UIAccessibilityService.getRootNode()
    │      → getRootInActiveWindow()
    │      → 递归遍历节点树，构建 JSON
    │
    └─ 全部失败 → uiautomator dump（备用，会导致 AccessibilityService 重启）
```

元素过滤支持以下 `filterName`：

| filterName | 说明 |
|------------|------|
| `text` | 元素显示文本 |
| `contentDesc` | 无障碍描述 |
| `id` | 资源 ID（如 `com.xxx:id/button`） |
| `focused` | 是否获得焦点 |
| `class` | 控件类名 |

---

## 文本输入原理

**文件**：`protocollib/src/main/java/xiaoxi/lua/LuaBridge.java` — `inputText()`

采用**两阶段策略**，解决 Android 10+ 剪贴板访问限制问题：

**阶段一（优先）：AccessibilityService `ACTION_SET_TEXT`**

```java
// 找到当前获得输入焦点的可编辑节点
AccessibilityNodeInfo focusedNode = rootNode.findFocus(FOCUS_INPUT);
// 直接通过无障碍服务写入文本，完全绕过剪贴板
Bundle args = new Bundle();
args.putCharSequence(ACTION_ARGUMENT_SET_TEXT_CHARSEQUENCE, text);
focusedNode.performAction(ACTION_SET_TEXT, args);
```

**阶段二（回退）：剪贴板 + `KEYCODE_PASTE`**

当 AccessibilityService 不可用或 `ACTION_SET_TEXT` 失败时使用：

```java
clipboardManager.setPrimaryClip(ClipData.newPlainText("text", text));
RootCmd.execNonRootCmd("input keyevent KEYCODE_PASTE");
```

> **为什么需要两阶段？**
> Android 10+ 引入剪贴板安全限制：只有当前处于前台焦点的 App 才能读取剪贴板。
> 当 Agent 进程执行 Shell 命令时，目标 App 可能短暂失去焦点，导致粘贴失败。
> `ACTION_SET_TEXT` 是无障碍服务的特权操作，不受此限制。

---

## 上传进度监控原理

**文件**：`protocollib/src/main/java/xiaoxi/manager/NotificationListenerManager.java`

TikTok 上传视频时会在通知栏显示进度，`NotificationListenerManager` 监听系统通知，将进度写入文件：

```
通知栏: "Uploading... 54%"
    ↓
NotificationListenerManager.onNotificationPosted()
    ↓
setUploadStatus("54%")  →  写入 /data/local/tmp/tiktok_upload_status.txt
```

Lua 脚本通过 `get_notification_upload_status()` 轮询该文件，返回值为：
- `"0%"` ～ `"100%"`：上传中
- `"complete"`：上传完成

**动态超时延长策略**（`create_temp.lua`）：

```lua
-- 根据已用时间和当前进度，估算剩余时间
local ms_per_pct = elapsed_ms / progress
local estimated_remaining = ms_per_pct * (100 - progress) * 1.3  -- 加30%缓冲

-- 如果剩余超时不足，自动延长
if estimated_remaining > remaining_timeout then
    upload_timeout = elapsed_ms + estimated_remaining
end
```

**检测间隔自适应**：

| 进度 | 检测间隔 |
|------|---------|
| 0–10% | 4000ms |
| 10–40% | 3000ms |
| 40–70% | 2000ms |
| 70–90% | 1500ms |
| 90%+ | 1000ms |
