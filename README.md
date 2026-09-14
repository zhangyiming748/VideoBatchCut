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

1. **将当前用户添加到 video 和 render 组**

```bash
sudo usermod -aG video,render $USER
```

2. **验证用户是否已加入这两个组**

```bash
getent group video; getent group render
# 应该看到类似输出：
# video:x:44:你的用户名
# render:x:105:你的用户名
```

3. **完全重启系统**（必须！）

```bash
sudo reboot
```

> **注意**：仅仅注销重新登录是不够的，必须完全重启系统才能使权限生效。

4. **重启后验证权限**

```bash
# 检查 DRI 设备权限（应能看到 card0 / renderD128）
ls -l /dev/dri/

# 确认当前 FFmpeg 编译时带了 QSV 支持（有输出即支持）
ffmpeg -hide_banner -encoders | grep qsv

# 测试 QSV 硬件编码是否可用（合成测试源，不会改动你的文件）
ffmpeg -hide_banner \
  -init_hw_device qsv=qsv0:hw,child_device=/dev/dri/renderD128 \
  -f lavfi -i testsrc=size=1280x720:duration=2 \
  -c:v h264_qsv -q 20 /tmp/qsv_test.mp4
```

> **设备节点说明**：QSV 在 Linux 上通过 VAAPI 子设备工作，必须指定 **render node**（`/dev/dri/renderD128`），不能用 primary node（`/dev/dri/card0`）。render node 的访问权限由 `render` 组控制，这也是第 1 步要同时加入 `render` 组的原因。

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

所有分支均按 **1080p60 + 质量优先** 定档，目标是消除平坦亮部/暗部可见的宏块效应（blocking artifacts）。

这里「质量优先」的含义是：**为源中真实存在的细节付出码率，但不通过极端参数去制造源中本不存在的信息**。因此只拉高与码率分配效率相关的开关（AQ、lookahead、RDO 等），而压低会凭空增强纹理的心理视觉优化。

**片源前提**：处理对象绝大多数是网站下载的二次压缩视频（部分已被发布者压过一次以上）。这类片源的细节在上一代编码时已永久丢失，因此档位取「对二手源透明」而非「视觉无损」：再往下压只会把源中已有的块效应、蚊式噪声、色带忠实地一起编码进去，体积暴涨而画质毫无提升。真正能改善这类片源的是编码前的轻度去噪。

### NVIDIA GPU 设备

- 视频编码：h264_nvenc
- 音频编码：aac（MP4）/ flac（MKV）
- 预设：p7（最高质量）+ tune hq
- 码率控制：VBR，`-b:v 0` 不设上限，CQ=19
- 自适应量化：`-spatial-aq 1 -temporal-aq 1 -aq-strength 11`
  - Spatial AQ 会把额外比特分配给低复杂度平坦区域，是抑制块效应的关键开关
  - AQ 强度范围 1-15，取 11 而不拉满：过高会从复杂纹理区抽走过量码率，片源中若无大面积平坦区就纯属浪费（驱动默认 8）

### Intel GPU 设备（VA-API/QSV）

- 视频编码：h264_qsv
- 音频编码：aac（MP4）/ flac（MKV）
- 质量参数：`-global_quality 18`（LA_ICQ 模式，范围 1-51，越小质量越高）
  - ⚠️ **不能用 `-q`**：`-q` 是 `-qscale` 的别名，会置位 qscale flag，使 QSV 落入 **CQP 恒定量化**模式 —— 全帧使用固定 QP、完全不做内容自适应，正是平坦区最容易出块的模式；而且会让 `-look_ahead` 彻底失效
  - 官方文档的判定顺序：指定 global_quality 时，若同时置位 qscale flag → CQP；否则若开启 look_ahead → LA_ICQ；否则 → ICQ
- Profile：high
- 硬件设备：`-init_hw_device qsv=qsv0:hw,child_device=/dev/dri/renderD128`，配合 `-hwaccel qsv -hwaccel_output_format qsv` 实现解码与编码全链路零拷贝
- 质量增强：`-look_ahead 1 -look_ahead_depth 40`（ICQ 升级为 LA_ICQ）、`-extbrc 1`、`-mbbrc 1`（宏块级码率控制，官方称可改善主观视觉质量）、`-rdo 1`、`-adaptive_i 1 -adaptive_b 1`、`-bf 4`

**前置条件**：
- 系统需安装 Intel VA-API 驱动（Alder Lake 及更新平台使用 `iHD`）
- 用户需同时属于 `video` 和 `render` 组（见上方飞牛 OS 配置说明）
- FFmpeg 需编译时启用 `--enable-vaapi` 和 `--enable-libvpl`（旧版本为 `--enable-libmfx`）
- `rdo` / `adaptive_i` / `adaptive_b` / `mbbrc` 需较新的 FFmpeg；若报 `Option xxx not found`，用 `ffmpeg -h encoder=h264_qsv` 查看当前构建支持哪些选项，并删掉不支持的行

### AMD GPU 设备（AMF）

- 视频编码：h264_amf
- 音频编码：aac（MP4）/ flac（MKV）
- 使用场景：`-usage high_quality`（影视制作级；注意 `transcoding` 是面向低码率网络传输的预设，会关闭部分 AQ 与 deblock 优化）
- 质量偏好：`-quality quality`
- 量化参数：qp_i=18 / qp_p=20 / qp_b=22（CQP 模式）
- 质量增强：`-vbaq true`（方差自适应量化，优先给平坦区域分配码率）、`-preanalysis true`
- Profile：high

### CPU 软件编码（兜底）

- 视频编码：libx264
- 音频编码：aac（MP4）/ flac（MKV）
- 预设：slow，CRF=19
- Profile：high，像素格式 yuv420p
- `-x264-params aq-strength=1.2`：加强平坦区域自适应量化（x264 默认 1.0）
- `-psy-rd 0.6:0.0`：压低心理视觉优化强度（`preset slow` 默认 1.0:0.0）。psy-rd 会主动增强高频纹理让画面「看起来更锐」，但被增强的往往是噪声或压缩残留而非真实内容，属于凭空制造信息并抬高码率；调低后编码更忠实于源。设为 `0.0:0.0` 则完全关闭，但画面可能显得过度平滑
  - 注意：`psy-rd` 的值本身包含冒号（`psy-rd:psy-trellis` 格式），而 `-x264-params` 内部也用冒号分隔多个参数，因此**必须使用 FFmpeg 暴露的独立 `-psy-rd` 选项**，不能写进 `-x264-params`
- **不指定 `-level`**：1080p 每帧 8160 个宏块，60fps 下需 MaxMBPS ≥ 489,600，而 Level 4.1 上限仅 245,760（官方标注仅支持 1920×1080@30.1），硬写会导致限流降质；交由 x264 按输入自动选择（Level 4.2 起才满足）

## 编码参数设计依据

本节记录上一节中每一项取值的推导过程，包括画质目标的量化定义、片源特性分析、各编码器码率控制模式的语义差异，以及已经踩过的坑。**修改参数前请先读完本节。**

### 1. 画质目标的量化定义

主观要求：**1080p 分辨率下，在 50 英寸电视的正常观看距离上，纯亮部与纯暗部不出现肉眼可辨的大块状伪影。**

这在编码层面对应两个具体对象：

- **宏块效应（blocking artifact）**：H.264 以 16×16 宏块为单位做 DCT 变换与量化，块与块之间独立量化会产生边界不连续。量化步长过大时，边界在屏幕上表现为规则的方格状纹理。
- **色带（banding）**：大面积平滑渐变（天空、暗部、纯色背景）的色阶被量化成有限几档，出现阶梯状断层。

两者都集中出现在**低空间复杂度的平坦区域**，原因有二：

1. 平坦区的 DCT 高频系数接近零，量化后容易被整体清零，块内失去梯度支撑；
2. 人眼的对比敏感度函数（CSF）峰值落在 2~5 周/度的低频段，平坦区的任何不连续都无处遮掩；而在高纹理区，同等强度的失真会被纹理本身掩蔽（masking effect）。

NVIDIA 的编码器文档对此有直接表述：

> *"the low complexity flat regions are visually more perceptible to quality differences than high complexity detailed regions"*

这决定了本项目的参数策略核心：**不是单纯拉高整体码率，而是通过自适应量化把码率重新分配到平坦区**。

### 2. 片源特性：二次压缩视频

处理对象绝大多数是网站下载的二手视频，部分已被发布者转码过一次以上。这类片源有三个必须正视的事实：

1. **细节不可逆丢失**：上一代编码时被量化掉的高频信息无法恢复，任何参数都救不回来。
2. **伪影已是像素的一部分**：块效应、蚊式噪声（mosquito noise）、色带在解码后就是实实在在的信号，编码器会忠实地把它们当作内容去编。
3. **伪影极难压缩**：块边界与蚊噪本质是高频、无规律信号，压缩效率远低于自然纹理。

由此得出两条结论：

- **档位不能盲目压低**。把 CRF/ICQ 拉到「视觉无损」级别，对干净源是保真，对二手源只是花大量码率去精确保留 artifact —— 体积暴涨而观感零提升。因此档位定在「对二手源透明」（不新增可见的代际损失）而非「对原始拍摄无损」。
- **真正能改善观感的是编码前处理（去噪/去色带），而不是编码参数**。参数只能保证「不再变差」，不能「变好」。

### 3. 各编码器码率控制模式的语义差异

这是本项目最容易出错、也确实错过一次的地方。**同一个数字在不同编码器的不同模式下含义完全不同。**

#### 3.1 QSV：CQP / ICQ / LA_ICQ 的判定规则

FFmpeg 官方文档对 QSV 的码率控制选择有明确的判定顺序：

> *"When global_quality is specified, a quality-based mode is used. Specifically this means either*
> - *CQP - constant quantizer scale, when the **qscale codec flag** is also set (the `-qscale` ffmpeg option).*
> - *LA_ICQ - intelligent constant quality with lookahead, when the `look_ahead` option is also set.*
> - *ICQ – intelligent constant quality otherwise."*

关键在于 **`-q` 是 `-qscale` 的别名**，它会置位 qscale flag。因此：

| 写法 | 实际模式 | 行为 |
|---|---|---|
| `-q 20` | **CQP 20** | 全帧、全区域使用固定量化步长，完全不做内容自适应 |
| `-global_quality 18` | **ICQ 18** | 按帧复杂度动态调整 QP |
| `-global_quality 18 -look_ahead 1` | **LA_ICQ 18** | 在 ICQ 基础上加入前瞻分析，使码率分配可以在多帧之间统筹优化 |

**CQP 正是平坦区最容易出块的模式**：固定 QP 意味着天空和树林用同一个量化步长，平坦区拿不到额外比特。项目早期使用的 `-q 20` 实际上一直运行在 CQP 模式下，这是「亮部暗部有方块」的直接技术成因之一。

同时要注意，在 CQP 模式下 `-look_ahead` **会被完全忽略**（判定顺序中 qscale flag 优先级最高），所以「加了 look_ahead 就升级成 LA_ICQ」这个直觉在用 `-q` 时是不成立的。

#### 3.2 NVENC：CQ 与 AQ 的分工

- `-rc vbr -b:v 0 -cq:v N`：VBR 模式下不设码率上限，由 CQ 值决定质量目标（等效于 x264 的 CRF）。
- `-spatial-aq`：空间自适应量化。NVIDIA 文档描述其作用为 *"extra bits are allocated to flat regions of the frame at the cost of the regions having high spatial detail"* —— 与本项目需求完全对口。
- `-temporal-aq`：时间自适应量化，改善帧间静止但高细节区域的参考帧质量。
- `-aq-strength`：范围 1（最弱）~ 15（最强），**不指定时由驱动自选，默认 8**。

`aq-strength` 取 **11** 而非拉满 15 的理由：AQ 是零和的重新分配，强度越高，从复杂纹理区抽走的码率越多。对本身没有大面积平坦区的素材，拉满只会造成纹理区细节损失与体积浪费，属于典型的「为参数付费」而非「为内容付费」。

#### 3.3 AMF：usage 预设会连带改写一大批参数

AMD 文档明确说明 `usage` 不只是个标签，它会预设包括 *"Encoding profile and level / GOP size and structure / Rate control mode and strategy / **Deblocking filter strength** / **Adaptive quantization and rate distortion optimization**"* 在内的多项参数。

各取值的设计场景：

| usage | 官方定义的场景 |
|---|---|
| `transcoding` | *"Convert high-resolution or high-bitrate videos to **low-resolution or low-bitrate** videos for transmission or storage in **bandwidth-limited network environments"* |
| `high_quality` | *"Suitable for scenarios that require **outputting high-quality videos**, such as film and television production"* |

项目早期使用的 `transcoding` 面向的是低码率网络传输，会关闭部分 AQ 与 deblock 优化 —— 与本项目目标相反，已改为 `high_quality`。

另外 `-vbaq`（Variance Based Adaptive Quantization）默认 **false**，其作用是 *"Prioritize bits to parts of the image humans care about"*，需显式开启；`-preanalysis` 同理，AMD 官方推荐配置中包含 `-preanalysis true`。

#### 3.4 x264：AQ 默认开启，psy-rd 也默认开启

x264 与硬件编码器最大的区别是**默认就开启自适应量化**。FFmpeg 的 libx264 wrapper 把 `aq-mode` 与 `aq-strength` 的默认值都设为 `-1`（意为不干预），实际生效的是 x264 内部默认值：**`aq-mode 1`（Variance AQ）+ `aq-strength 1.0`**。而 NVENC 的 `-spatial-aq`、AMF 的 `-vbaq` 默认都是关闭的，必须显式开启。

FFmpeg 对 `aq-strength` 的描述是 *"Reduces blocking and blurring in flat and textured areas"* —— 字面对应本项目需求，已通过 `-x264-params aq-strength=1.2` 轻度加强。

`aq-mode` 另有两个可选档位，**目前未启用**：

| 值 | 名称 | 行为 |
|---|---|---|
| 1（当前） | `variance` | 基于局部方差的 AQ（complexity mask） |
| 2 | `autovariance` | Auto-variance AQ，令帧内平均 QP 偏移趋于 0，避免 mode 1 有时过度削减弱纹理区 |
| 3 | `autovariance-biased` | Auto-variance AQ **with bias to dark scenes**，在 mode 2 基础上对暗场景额外加权 |

其中 **`3` 直接针对暗部**，与本项目「纯暗部不出现方块」的要求高度契合：给暗场景分配更多码率意味着量化更细、色阶档位更多，理论上既能压住暗部块效应，也能减轻新增的 banding。代价是亮部与高复杂度区域的码率被相对削减，可能在那里产生新的损失，因此需实测对比后再决定是否启用（写法：`-x264-params aq-mode=3:aq-strength=1.2`）。

注意 `aq-mode` 是 x264 专属选项，**只影响 CPU 兜底分支**。三家硬件编码器没有等价开关，各自的区域级调节手段为：QSV → `-mbbrc`（宏块级码率控制）与 `-extbrc`；NVENC → `-spatial-aq`；AMF → `-vbaq`。

而 `psy-rd` 需要特别压制，见下节。

### 4. 为何压低 psy-rd：区分「保真」与「造细节」

`preset slow` 默认带 `psy-rd 1.0:0.0`。心理视觉优化的工作方式是**主动增强高频纹理**，让画面「看起来更锐、更有质感」，代价是码率上升。

问题在于：被增强的对象往往是**噪声与压缩残留，而不是真实内容**。对二次压缩片源，这意味着把 mosquito noise 进一步放大，并为这些本不存在于原始素材的信号付费。

按「为源中真实存在的细节付出码率，但不制造源中本不存在的信息」这一原则，已将 `psy-rd` 压到 `0.6:0.0`：保留一部分纹理感避免画面过度平滑（塑料感），但不再主动增强伪影。设为 `0.0:0.0` 则完全关闭，或加 `-psy 0` 彻底停用心理视觉优化。

> **语法陷阱**：`psy-rd` 的值本身是 `psy-rd:psy-trellis` 冒号格式，而 `-x264-params` 内部同样用冒号分隔多个参数。写成 `-x264-params "aq-strength=1.2:psy-rd=0.6:0.0"` 会被切分为 `aq-strength=1.2`、`psy-rd=0.6` 和一个无 key 的孤立 `0.0`，导致 psy-trellis 丢失或解析失败。必须使用 FFmpeg libx264 wrapper 暴露的独立 `-psy-rd` 选项。

### 5. 档位数值的定标依据

统一取「对二手压缩源透明」这一标准：

| 编码器 | 参数 | 取值 | 范围与方向 |
|---|---|---|---|
| QSV | `-global_quality` | 18 | 1~51，越小越好 |
| NVENC | `-cq:v` | 19 | 0~51，越小越好 |
| AMF | `-qp_i/p/b` | 18/20/22 | 0~51，分帧型固定量化 |
| x264 | `-crf` | 19 | 0~51，越小越好 |

这几个值在干净源上已接近视觉无损，在二手源上则足以避免引入新的可见代际损失。继续往下压（如 CRF 14~16）对二手源的观感提升趋近于零，而体积呈非线性增长。

### 6. 硬件设备节点：必须用 render node

QSV 在 Linux 上通过 VAAPI 子设备工作，FFmpeg 文档对 `-init_hw_device qsv` 的 `child_device` 选项的定义是：

> *"Specify a DRM **render node** on Linux or DirectX adapter on Windows."*

因此必须使用 `/dev/dri/renderD128`，**不能用 primary node `/dev/dri/card0`**：primary node 通常需要 DRM master 权限或活动的显示会话，无头转码场景下会打开失败；render node 才是为计算/转码设计的入口，其访问权限由 `render` 用户组控制（这也是权限配置要同时加入 `video` 和 `render` 组的原因）。

`-hwaccel_device` 传的是 `-init_hw_device` 创建的**设备名**（此处为 `qsv0`）而非路径，文档原文：*"It can either refer to an existing device created with `-init_hw_device` by name, or it can create a new device"*。

另需注意：DRM 的 `cardN` 编号**按驱动 probe 完成的先后顺序分配，而非 PCI 地址顺序**。纯核显机器上核显是 `card0`，有独显时可能排到 `card1`。项目早期曾硬编码 `/dev/dri/card1`（当时调试机的实际编号），换机后直接失效 —— 这是调试残留写进代码的典型例子。

### 7. H.264 Level 与 1080p60

x264 分支**不再硬写 `-level`**，原因是 Level 4.1 无法容纳 1080p60。

宏块数计算：`ceil(1920/16) × ceil(1080/16) = 120 × 68 = 8,160 宏块/帧`

| Level | MaxMBPS | MaxFS | MaxDpbMbs | 官方标注的 1080p 支持 |
|---|---|---|---|---|
| 4.1 | 245,760 | 8,192 | 32,768 | 1920×1080@**30.1** |
| 4.2 | 522,240 | 8,704 | 34,816 | 1920×1080@**64.0** |

- 1080p**30**：8,160 × 30 = 244,800 ≤ 245,760，勉强够用（余量仅 0.4%）
- 1080p**60**：8,160 × 60 = **489,600 > 245,760**，超出近一倍，x264 会告警 `frame MB size > level limit` 并被迫限流降质

还有一个隐性损失：Level 4.1 的 MaxDpbMbs 为 32,768，除以每帧 8,160 宏块 = **最多 4 个参考帧**，而 `preset slow` 默认希望使用 5 个，会被 level 卡掉，压缩效率下降 —— 同等 CRF 下反而更容易出现块效应。

删除 `-level` 后由 x264 依据实际输入自动选择，是最稳妥的做法。

### 8. 尚未落地的一项：去噪预处理

针对第 2 节所述的片源特性，**编码前的轻度去噪是唯一能真正提升二手源观感的手段**，且能同时减小体积：压缩噪声不可压缩，清掉后码率才能用在真实内容上。目前尚未加入，因为存在两个待定因素：

- **GPU 路径兼容性**：`vpp_qsv=denoise=N` 可在保持零拷贝的前提下于 GPU 去噪，但需确认具体 FFmpeg 构建与 iHD 驱动是否支持（`ffmpeg -h filter=vpp_qsv | grep -i denoise`）。
- **去噪与色带的对冲**：去除噪声后，原本被噪声「打散」掩盖的色阶断层会暴露出来，因此需要配套 `gradfun` 做抖动补偿；而 `gradfun` 是 CPU 滤镜，加入 QSV 全硬件链路需要 `hwdownload`/`hwupload`，会破坏零拷贝并显著拖慢 N100 这类低功耗平台。

在确定方案前，**请勿仅靠继续压低档位来追求「干净」** —— 那条路对二手源无效。

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
