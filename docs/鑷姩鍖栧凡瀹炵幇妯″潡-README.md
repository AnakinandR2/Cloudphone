# Lua 模块化脚本系统

## 目录结构

```
lua_script/
├── modules/                    # 公共模块目录
│   ├── tiktok/                # TikTok 相关模块
│   │   ├── constants.lua      # 常量定义
│   │   ├── permissions.lua    # 权限授予模块（旧版）
│   │   ├── permissions_enhanced.lua  # 权限授予模块（增强版）
│   │   ├── widget.lua         # Widget 处理模块
│   │   ├── download.lua       # 文件下载模块
│   │   ├── media.lua          # 媒体库操作模块
│   │   ├── ui_utils.lua       # UI 操作工具
│   │   ├── create_flow.lua    # 创作流程模块
│   │   ├── share_link.lua     # 分享链接获取模块
│   │   ├── follow.lua         # 关注用户模块
│   │   └── home.lua           # 主页弹窗处理模块
│   └── common/                # 公共工具模块
│       ├── shell_utils.lua    # Shell 工具函数
│       ├── file_utils.lua      # 文件操作工具
│       └── utils.lua           # 通用工具函数（字符串处理、表格操作等）
└── modules_test/              # 模块化测试脚本
    ├── create_test.lua        # 模块化测试主脚本
    └── create_test.py         # 测试 Python 脚本
```

## 模块说明

### TikTok 模块

#### `tiktok/constants.lua`
- 定义 TikTok 相关的常量（包名、UI 元素 ID、文本等）

#### `tiktok/permissions.lua`
- `grant_tiktok_permissions()`: 授予 TikTok 应用必要的权限

#### `tiktok/widget.lua`
- `disable_completely()`: 彻底禁用 Widget 功能
- `handle_dialog(step_map)`: 处理 Widget 弹窗

#### `tiktok/download.lua`
- `download_materials(img_url, video_url, download_dir)`: 下载图片和视频素材
- `clear_gallery(download_dir)`: 清空相册目录

#### `tiktok/media.lua`
- `refresh_media_library(img_paths, video_path, download_dir)`: 刷新媒体库
- `verify_files_exist(img_paths, video_path, checkpoint)`: 验证文件是否存在

#### `tiktok/ui_utils.lua`
- `skip_page(step_map, tag, filter, filter_value, timeout)`: 跳过页面/点击元素
- `extract_button_positions(json_str, button_id)`: 提取按钮位置
- `select_multiple_items(count)`: 选择多个图片/视频
- `handle_verify()`: 处理滑块验证
- `handle_swipe_mask()`: 处理滑动遮罩验证
- `checkBlocked()`: 检查账号是否被封禁
- `parse_follower_count(count_str)`: 解析粉丝数格式

#### `tiktok/create_flow.lua`
- `create(step_map, add_music, add_location, desc)`: 执行创作流程
- `open_tiktok(step_map, img_paths, video_path, download_dir)`: 打开 TikTok 应用
- `return_to_home()`: 返回主页

#### `tiktok/share_link.lua`
- `get_share_link(step_map)`: 获取视频分享链接
- `try_get_share_link(step_map, create_success)`: 尝试获取分享链接（带日志）

#### `tiktok/follow.lua`
- `search_and_enter_user_profile(username)`: 搜索并进入用户主页
- `get_username()`: 获取用户名
- `get_follower_count()`: 获取粉丝数
- `get_following_count()`: 获取关注数
- `follow_user()`: 执行关注操作
- `set_skip_page(func)`: 注入 skip_page 依赖

#### `tiktok/home.lua`
- `handle_popups_and_init(step_map)`: 处理各种弹窗和初始设置
- `enter_profile_page(step_map)`: 进入个人主页
- `set_skip_page(func)`: 注入 skip_page 依赖
- `set_dependencies(funcs)`: 批量注入依赖函数

### 公共模块

#### `common/shell_utils.lua`
- `safe_shell(cmd)`: 安全执行 shell 命令
- `shell_success(cmd)`: 检查 shell 命令是否成功

#### `common/file_utils.lua`
- `get_file_size(file_path)`: 获取文件大小
- `file_exists(file_path)`: 检查文件是否存在
- `extract_extension(url)`: 从 URL 提取文件扩展名

#### `common/utils.lua`
- `split(str, sep)`: 分割字符串为数字数组
- `split_string(str, sep)`: 分割字符串为字符串数组
- `trim(str)`: 去除字符串两端空格
- `starts_with(str, prefix)`: 检查字符串是否以指定前缀开头
- `ends_with(str, suffix)`: 检查字符串是否以指定后缀结尾
- `is_empty_table(tbl)`: 判断表格是否为空
- `safe_call(func, ...)`: 安全地执行函数，捕获异常
- `log_format(msg, level)`: 日志格式化输出
- `tap_screen_center()`: 点击屏幕中心点
- `get_screen_resolution()`: 获取屏幕分辨率
- `tap_by_ratio(x_ratio, y_ratio, duration)`: 点击指定比例的坐标

#### `tiktok/permissions_enhanced.lua`
- `grant_tiktok_permissions(package_name)`: 授予 TikTok 完整权限
- `grant_additional_permissions(package_name)`: 补充授予额外权限
- `check_permission_granted(package_name, permission)`: 检查权限是否已授予
- `deny_permissions(package_name, permissions_to_deny)`: 禁用指定权限

## 使用方法

### 在 Lua 脚本中使用模块

```lua
-- 加载模块
local constants = require("tiktok.constants")
local permissions = require("tiktok.permissions")
local download = require("tiktok.download")

-- 使用模块功能
permissions.grant_tiktok_permissions()
local img_paths, video_path = download.download_materials(img_url, video_url, download_dir)
```

### 模块路径配置

**注意**：当前实现需要在 Native 层配置 `package.path`，指向模块目录。例如：

```cpp
// 在 lua_native.cpp 的 luaInit() 中
const char* modulePath = 
    "/data/data/com.cloudphone.agent/files/lua_modules/?.lua;"
    "/data/data/com.cloudphone.agent/files/lua_modules/?/init.lua;"
    "./?.lua;./?/init.lua";

lua_getglobal(g_luaState, "package");
lua_pushstring(g_luaState, modulePath);
lua_setfield(g_luaState, -2, "path");
lua_pop(g_luaState, 1);
```

### 测试脚本

运行测试脚本：

```bash
cd lua_script/modules_test
python create_test.py
```

## 模块化优势

1. **代码复用**：公共功能可以在多个脚本中复用
2. **易于维护**：功能模块化，修改时只需关注特定模块
3. **清晰结构**：主脚本只负责流程编排，逻辑清晰
4. **独立测试**：每个模块可以独立测试和验证

## 与原有脚本的对比

### 原有脚本 (`create_temp.lua`)
- 单文件，1680 行
- 所有功能混在一起
- 难以复用和维护

### 模块化脚本 (`create_test.lua`)
- 主脚本约 200 行
- 功能模块化，职责清晰
- 易于扩展和维护

## 后续改进

1. **接口增强**：在 `LuaController.java` 中添加 `exec_with_modules()` 接口
2. **模块热更新**：支持动态加载和更新模块
3. **依赖管理**：自动解析和管理模块依赖关系
4. **模块缓存**：利用 Lua 的 `package.loaded` 缓存已加载模块
