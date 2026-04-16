# FFmpeg 视频转换指南

以下是针对你的需求（批量将 **AVI/MKV** 视频转成 **MP4**,使用 **H.264 NVENC** 快速编码,尽量保持原画质,但避免"一刀切"最高画质导致文件过大浪费空间）的推荐设置。

## H.264 NVENC 快速编码

### 核心思路

- **NVENC** 不支持 libx264 的 `-crf`,而是用 **Constant Quality (CQ)** 模式来近似实现"按内容自适应质量"。
- CQ 值范围 **0~51**（越小质量越高、文件越大）。**推荐从 23~28 开始测试**：
  - **23~24**：非常接近原画质（视觉上几乎无损），文件比原文件稍大或接近，适合追求高保真的视频。
  - **25~26**：优秀平衡点，画质损失极小（肉眼难辨），文件大小节省明显。
  - **27~28**：画质仍很好，但文件更小，适合大部分日常视频。
- **Preset**：用 `p6` 或 `p7`（高质量模式），速度比 CPU 快很多，同时质量优秀。不要用 `p1~p4`（太快，质量下降明显）。
- **Tune**：`hq`（High Quality）提升画质。
- **Rate Control**：`-rc vbr -b:v 0` + `-cq:v XX` 实现 CQ 模式（让编码器根据画面复杂度自动分配比特率）。
- **其他优化**：开启 lookahead 和 AQ（自适应量化），让复杂场景多分配比特率，平坦场景少用，达到“不浪费空间”的效果。
- **音频**：直接复制（`-c:a copy`），避免二次转码损失。
- **硬件加速**:用 `-hwaccel cuda` 加速解码(如果你的 NVIDIA 显卡支持)。

### 推荐单文件命令(先测试一个视频)

```bash
ffmpeg -hwaccel cuda -i "input.mkv" \
  -c:v h264_nvenc \
  -preset p6 \                  # 或 p7（质量更好，但稍慢）
  -tune hq \
  -rc vbr -b:v 0 \
  -cq:v 25 \                    # ← 这里改成 23~28 测试
  -rc-lookahead 32 \
  -spatial-aq 1 \
  -profile:v high \
  -c:a copy \
  "output.mp4"
```

- **测试方法**：
  1. 用 **CQ 25** 先转一个视频，对比原文件（用 VLC 或 PotPlayer 放大细节看噪点、纹理、边缘）。
  2. 如果觉得画质完美但文件太大 → 把 CQ 调到 26~27。
  3. 如果觉得还有轻微损失 → 调到 23~24。
  4. 不同视频内容（动作片 vs 静态纪录片）对 CQ 的敏感度不同，多试几个代表性文件。

**更高质量版本**(如果想更接近原画质,文件会稍大):

```bash
-preset p7 -tune hq -cq:v 23 -rc-lookahead 32 -spatial-aq 1 -temporal-aq 1
```

**更快版本**(牺牲一点点质量):

```bash
-preset p5 -cq:v 26
```

### 批量转换脚本(Windows)

在视频文件夹里新建一个 `convert.bat` 文件,用记事本编辑,粘贴下面内容:

```batch
@echo off
chcp 65001 >nul
echo 开始批量转换 AVI/MKV 到 MP4 (H.264 NVENC)...

for %%i in (*.avi *.mkv) do (
    echo 正在处理: %%i
    ffmpeg -hwaccel cuda -i "%%i" ^
      -c:v h264_nvenc ^
      -preset p6 ^
      -tune hq ^
      -rc vbr -b:v 0 ^
      -cq:v 25 ^
      -rc-lookahead 32 ^
      -spatial-aq 1 ^
      -profile:v high ^
      -c:a copy ^
      "%%~ni_converted.mp4"
)

echo 全部转换完成！
pause
```

- 把 `convert.bat` 放到你的视频文件夹，双击运行即可。
- 输出文件会是 `原文件名_converted.mp4`（避免覆盖原文件）。
- 如果想输出到子文件夹 `output`，可以修改脚本最后一行路径。

**Linux/macOS** 批量脚本（bash）：

```bash
for i in *.avi *.mkv; do
    ffmpeg -hwaccel cuda -i "$i" \
      -c:v h264_nvenc -preset p6 -tune hq \
      -rc vbr -b:v 0 -cq:v 25 -rc-lookahead 32 -spatial-aq 1 \
      -profile:v high -c:a copy "${i%.*}_converted.mp4"
done
```

### 注意事项

- **显卡要求**:NVIDIA GTX/RTX 系列(Pascal 及以后最好),确保驱动最新。
- **检查 FFmpeg 是否支持 NVENC**：运行 `ffmpeg -encoders | findstr nvenc`（Windows）或 `ffmpeg -encoders | grep nvenc`。
- **如果源视频是 10bit / HDR**：可能需要额外参数（如 `-pix_fmt yuv420p10le`），但 H.264 High Profile 通常处理 8bit 就够。
- **文件大小 vs 画质**：先用 CQ 25 转几个不同类型的视频，对比原文件大小和视觉效果，再决定最终值。
- **极端情况**：如果某个视频特别复杂（高动态、噪点多），可以单独用 CQ 23 处理；简单视频用 CQ 27。

先用上面推荐命令测试 1~2 个视频,找到你能接受的 CQ 值(大多数人 24~26 都很满意),然后批量跑就行。有什么具体视频类型(1080p/4K、动作/静态)或测试结果,可以再告诉我,我帮你进一步微调参数!

## H.265/HEVC 高质量压缩

以下是针对你的第二步需求(从第一步输出的 **H.264 MP4**,再用 **libx265** 转成高质量、低存储空间的 **H.265/HEVC** 文件)的推荐参数。

### H.265 核心思路

- H.265(HEVC)比 H.264 压缩效率高约 **25%~50%**,在相同视觉质量下文件会明显变小。
- 推荐使用 **CRF（Constant Rate Factor）** 模式：它能根据画面复杂度自动分配比特率，实现“保证质量的前提下尽量减少空间”的目标。
- **CRF 值建议**（x265 默认 28）：
  - **23~25**：高画质起点，视觉上接近原 H.264（或接近无损），文件大小通常能比第一步的 H.264 文件再节省 **30%~50%**。
  - **20~22**：更接近“近无损/透明”质量，适合追求极致保真的视频（文件会稍大一些）。
  - **26~28**：更激进压缩，质量仍很好，但节省空间更多（推荐先从 25 开始测试）。
- **Preset**：用 **slow** 或 **slower**（质量/压缩效率更好）。**veryslow** 能再小一点文件，但编码时间会显著增加（除非你有很强的 CPU 且时间充裕）。
- **其他优化**：开启 10-bit 编码（即使源是 8-bit，也能减少色带，提高压缩效率，几乎不增加文件大小）；添加 AQ（自适应量化）让复杂场景保留更多细节。
- **音频和容器**：直接复制音频（`-c:a copy`），输出 `.mkv` 或 `.mp4`（推荐 mkv 兼容性更好）。
- **重要提醒**:这是**二次转码**(H.264 → H.265),会有轻微一代损失。想尽量减少损失,就用较低的 CRF + 较慢 preset。

### 推荐单文件测试命令

先拿 1~2 个有代表性的视频测试(动作片、静态场景、暗部细节多的视频各一个),对比原 H.264 文件的视觉质量和文件大小。

**平衡推荐(大多数人最合适)**:

```bash
ffmpeg -i "input.mp4" \
  -c:v libx265 \
  -preset slow \              # 或 slower / veryslow
  -crf 24 \                   # ← 从 23~25 开始测试，调高=更小文件，调低=更好质量
  -pix_fmt yuv420p10le \      # 10-bit 编码，强烈推荐
  -x265-params "aq-mode=2:psy-rd=2" \   # 自适应量化 + 心理视觉优化
  -c:a copy \
  "output_h265.mkv"
```

**更高画质版本**(文件稍大,推荐追求保真时用):

```bash
-preset slower -crf 22 -pix_fmt yuv420p10le -x265-params "aq-mode=3:psy-rd=1.5:deblock=-1,-1"
```

**更节省空间版本**(质量仍优秀):

```bash
-preset slow -crf 26 -pix_fmt yuv420p10le
```

测试方法:

- 用相同 CRF + preset 转几个视频。
- 在 VLC / PotPlayer 中全屏或放大细节对比（看纹理、边缘、暗部噪点、色带）。
- 如果画质满意但文件还想再小 → 把 CRF 调高 1~2（例如 24 → 26）。
- 如果还有明显损失 → 把 CRF 调低 1~2(例如 24 → 22),或换成 slower preset。

### H.265 批量转换脚本(Windows)

在视频文件夹新建 `convert_to_h265.bat`,内容如下:

```batch
@echo off
chcp 65001 >nul
echo 开始批量 H.264 MP4 → H.265 MKV (libx265 高质量压缩)...

for %%i in (*.mp4) do (
    echo 正在处理: %%i
    ffmpeg -i "%%i" ^
      -c:v libx265 ^
      -preset slow ^
      -crf 24 ^
      -pix_fmt yuv420p10le ^
      -x265-params "aq-mode=2:psy-rd=2" ^
      -c:a copy ^
      "%%~ni_h265.mkv"
)

echo 全部转换完成！
pause
```

- 输出文件会是 `原文件名_h265.mkv`（避免覆盖）。
- 如果想输出到子文件夹 `h265_output`，可以修改路径。

**Linux/macOS bash 脚本**：

```bash
for i in *.mp4; do
    ffmpeg -i "$i" \
      -c:v libx265 -preset slow -crf 24 \
      -pix_fmt yuv420p10le -x265-params "aq-mode=2:psy-rd=2" \
      -c:a copy "${i%.*}_h265.mkv"
done
```

### H.265 注意事项

- **编码速度**:libx265 是 CPU 编码,**slow** preset 在现代多核 CPU 上还能接受;**veryslow** 会很慢(尤其是 4K)。建议晚上或用多线程工具分批跑。
- **10-bit 的好处**：几乎所有现代播放器（VLC、Plex、手机等）都支持，压缩效率更高，色带更少。
- **HDR / 10-bit 源视频**：如果你的第一步 H.264 已经是 10-bit 或 HDR，保留 `-pix_fmt yuv420p10le` 并可额外加 `-colorspace bt2020nc -color_trc smpte2084` 等参数（视情况）。
- **文件大小预期**：从 H.264 CRF 25 的文件再转 H.265 CRF 24，通常能再节省 30%~60%，具体取决于视频内容（动作多 vs 静态）。
- **如果想更快**：可以用 `preset medium`，但文件会稍大一点，质量略低。
- **最终建议**：先用 **-preset slow -crf 24** 测试几个视频，找到你能接受的平衡点（大多数人这个值都很满意）。不同视频类型可能需要微调（例如动作大片用 CRF 23，纪录片用 25）。

跑完测试后，告诉我你的视频分辨率（1080p/4K）、类型（动作/电影/纪录片）、测试结果（文件缩小比例 + 视觉感受），我可以帮你进一步精确调整参数！
