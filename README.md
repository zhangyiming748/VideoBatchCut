# VideoBatchCut

基于 FFmpeg 的视频批量处理工具，支持视频切割和快速MP4转换。

## 功能特点

- ✅ 支持读取 LosslessCut 的 `-proj.llc` 项目文件进行精确视频切割
- ✅ 自动按序号生成分割后的视频文件 (01.mp4, 02.mp4, ...)
- ✅ 支持 NVIDIA GPU 硬件加速编码
- ✅ 毫秒级精确切割，避免黑屏和音画不同步
- ✅ 快速MP4转换功能，优化视频格式和编码
- ✅ 自动清理原始文件和项目文件
- ✅ 完整的日志记录和 SQLite 数据库存储

## 系统要求

- FFmpeg 命令行工具（必须在系统PATH中）
- Go 1.25.6 或更高版本
- （可选）NVIDIA GPU 及相关驱动
- （可选）Intel GPU 及 VA-API/QSV 支持

### ⚠️ 飞牛 OS (FnOS) 特别配置

**重要**：飞牛 OS 默认普通用户没有访问 GPU 硬件设备的权限，必须先完成以下配置才能使用硬件加速功能。

#### 配置步骤

1. **将当前用户添加到 video 组**

```bash
sudo usermod -aG video $USER
```

2. **验证用户是否已加入 video 组**

```bash
getent group video
# 应该看到类似输出：video:x:44:你的用户名
```

3. **完全重启系统**（必须！）

```bash
sudo reboot
```

> **注意**：仅仅注销重新登录是不够的，必须完全重启系统才能使权限生效。

4. **重启后验证权限**

```bash
# 检查 DRI 设备权限
ls -la /dev/dri/

# 测试硬件加速是否可用
ffmpeg -hwaccel vaapi -hwaccel_device /dev/dri/card1 -i input.avi -c:v h264_vaapi -qp 20 output.mp4
```

#### 替代方案：永久开放 DRI 设备权限

如果不想每次重启都依赖用户组权限，可以创建 udev 规则让所有用户都能访问：

```bash
# 创建 udev 规则文件
sudo tee /etc/udev/rules.d/99-dri-permissions.rules << 'EOF'
# 设置 DRI 设备权限为所有用户可读写
KERNEL=="card*", SUBSYSTEM=="drm", MODE="0666"
KERNEL=="renderD*", SUBSYSTEM=="drm", MODE="0666"
EOF

# 重新加载 udev 规则
sudo udevadm control --reload-rules
sudo udevadm trigger

# 验证权限（应立即生效，无需重启）
ls -la /dev/dri/
```

## 安装

### 方式一：从 Release 下载（推荐）

#### macOS (Apple Silicon)

```bash
wget https://github.com/zhangyiming748/VideoBatchCut/releases/latest/download/vbc_darwin_arm64 -O vbc
chmod +x vbc
./vbc --help
```

#### macOS (Intel)

```bash
wget https://github.com/zhangyiming748/VideoBatchCut/releases/latest/download/vbc_darwin_amd64 -O vbc
chmod +x vbc
./vbc --help
```

#### Linux (AMD64)

```bash
wget https://github.com/zhangyiming748/VideoBatchCut/releases/latest/download/vbc_linux_amd64 -O vbc
chmod +x vbc
./vbc --help
```

#### Linux (ARM64)

```bash
wget https://github.com/zhangyiming748/VideoBatchCut/releases/latest/download/vbc_linux_arm64 -O vbc
chmod +x vbc
./vbc --help
```

#### Windows (AMD64)

```powershell
# PowerShell
Invoke-WebRequest -Uri "https://github.com/zhangyiming748/VideoBatchCut/releases/latest/download/vbc_windows_amd64.exe" -OutFile "vbc.exe"
.\vbc.exe --help
```

#### Windows (ARM64)

```powershell
# PowerShell
Invoke-WebRequest -Uri "https://github.com/zhangyiming748/VideoBatchCut/releases/latest/download/vbc_windows_arm64.exe" -OutFile "vbc.exe"
.\vbc.exe --help
```

> **提示**：也可以访问 [Releases 页面](https://github.com/zhangyiming748/VideoBatchCut/releases) 查看所有版本和手动下载。

### 方式二：从源码编译

```bash
git clone https://github.com/zhangyiming748/VideoBatchCut.git
cd VideoBatchCut
go mod download
go build -o vbc main.go
```

## 使用方法

### 命令行使用

#### 1. 视频切割命令 (cut)

```bash
# 基本用法
./vbc cut --root "/path/to/video/folder"

# 示例
./vbc cut --root "/Users/username/videos"
```

#### 2. 快速MP4转换命令 (fastmp4)

```bash
# 基本用法
./vbc fastmp4 --root "/path/to/video/folder"

# 示例
./vbc fastmp4 --root "/Users/username/videos"
```

#### 3. 查看版本信息

```bash
./vbc version
```

#### 4. 获取帮助信息

```bash
# 总体帮助
./vbc --help

# 子命令帮助
./vbc cut --help
./vbc fastmp4 --help
```

### 项目文件准备

在视频文件所在的目录中创建 `-proj.llc` 文件，格式如下：

```json
{
    "cutSegments": [
        {
            "start": 0,
            "end": 90.5,
            "name": "segment1"
        },
        {
            "start": 90.5,
            "end": 180.3,
            "name": "segment2"
        }
    ]
}
```

或者使用简化格式：

```txt
start: 0
end: 90.5
name: segment1

start: 90.5
end: 180.3
name: segment2
```

### 目录结构示例

```txt
/videos/
├── myvideo.mp4
├── myvideo-proj.llc
└── another-video.mp4
    └── another-video-proj.llc
```

## 命令详解

### cut 命令

- **功能**：根据LosslessCut项目文件进行精确视频切割
- **参数**：`--root` 指定要处理的根目录
- **输出**：在原目录生成编号的视频片段文件

### fastmp4 命令

- **功能**：快速转换视频为优化的MP4格式
- **参数**：`--root` 指定要处理的根目录
- **特点**：使用高效的编码参数，快速完成格式转换

## 输出说明

- 程序会在视频所在目录生成编号的片段文件：`01.mp4`, `02.mp4`, ...
- 处理完成后自动删除原始视频文件和 `.llc` 项目文件
- 所有操作日志记录在 `BitchCut.log` 文件中
- 处理记录保存在 SQLite 数据库中

## 编码参数

### NVIDIA GPU 设备

- 视频编码：h264_nvenc
- 音频编码：aac
- 预设：slow
- CQ：18

### Intel GPU 设备（VA-API/QSV）

- 视频编码：h264_qsv（推荐）或 h264_vaapi
- 音频编码：aac
- 质量参数：q=20（范围1-51，越小质量越高）
- Profile：high

**前置条件**：
- 系统需安装 Intel VA-API 驱动
- 用户需有访问 `/dev/dri` 设备的权限（见上方飞牛 OS 配置说明）
- FFmpeg 需编译时启用 `--enable-vaapi` 和 `--enable-libvpl`

## 技术特点

### 精确切割技术

采用两步法确保毫秒级精度：

1. 使用 `-ss` 参数在输入端精确定位
2. 重新编码确保帧级别精确度
3. 使用 `-avoid_negative_ts make_zero` 消除黑屏
4. 应用 `aresample=async=1` 确保音画同步

### 关键FFmpeg参数

```shell
-ss [开始时间]           # 输入端seek，提高精度
-to [结束时间]           # 指定结束时间点
-avoid_negative_ts make_zero  # 消除负时间戳
-fflags +genpts+igndts   # 重新生成时间戳
-af "adelay=0|0, aresample=async=1"  # 音频同步处理
-copyts                  # 保持时间戳连续性
```

## 注意事项

1. 建议先用小视频文件测试功能
2. 确保 `.llc` 文件与对应视频在同一目录
3. 处理过程不可逆，原始文件会被自动删除
4. Linux/macOS 环境下运行效果最佳
5. 支持断点续传和优雅退出

## 常见问题

### Q: 如何创建 `.llc` 项目文件？

A: 可以使用 LosslessCut 软件导出，或手动按照上述格式创建

### Q: 切割精度如何保证？

A: 通过重新编码和精确的时间戳处理，可实现毫秒级精度

### Q: 为什么需要NVIDIA GPU？

A: GPU加速可显著提高编码速度，非必需但推荐

### Q: 处理过程中如何停止？

A: 使用 Ctrl+C 发送中断信号，程序会完成当前任务后优雅退出

### Q: fastmp4和cut命令有什么区别？

A: `cut`命令用于根据项目文件精确切割视频，`fastmp4`命令用于快速转换视频格式

## 许可证

查看 LICENSE 文件了解详细信息

## 贡献

欢迎提交 Issue 和 Pull Request 来改进项目功能。
