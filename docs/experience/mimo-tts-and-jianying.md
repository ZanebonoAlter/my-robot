# MiMo TTS 与剪映制作说明

## 当前可自动化的范围

本机已检测到：

- `D:\tool\ffmpeg\ffmpeg.exe`
- `D:\tool\ffmpeg\ffprobe.exe`
- Python
- `uv`

因此可以自动完成：

- 按视频段落调用 MiMo V2.5 TTS；
- 生成独立 WAV 片段；
- 合并完整旁白；
- 根据真实音频时长生成 SRT 字幕；
- 使用 FFmpeg 制作图片版粗剪、转码和音量处理。

当前没有可控制 Windows 桌面应用的工具，因此不能可靠地替用户点击剪映界面、移动时间线或调整剪映工程。推荐的协作方式是：脚本准备音频、字幕、图片和镜头清单，用户在剪映中完成最终节奏、转场和动态录屏编排。

## MiMo API Key

不要把 Key 写进脚本或提交到仓库。仅在当前 PowerShell 窗口设置：

```powershell
$env:MIMO_API_KEY = "你的 API Key"
```

关闭终端后该环境变量会失效。

可选配置：

```powershell
$env:MIMO_TTS_VOICE = "白桦"
$env:MIMO_TTS_MODEL = "mimo-v2.5-tts"
```

预置中文音色：

- `冰糖`：女声
- `茉莉`：女声
- `苏打`：男声
- `白桦`：男声

## 先检查脚本

以下命令不会调用 API：

```powershell
uv run scripts/mimo_tts.py `
  docs/experience/product-video-tts.json `
  --dry-run
```

它会显示段落数、总字符数、模型和音色。

## 生成完整旁白

```powershell
$env:MIMO_API_KEY = "你的 API Key"

uv run scripts/mimo_tts.py `
  docs/experience/product-video-tts.json
```

默认输出到：

```text
artifacts/product-video/tts/
├── 01-hook.wav
├── 02-problem.wav
├── ...
├── 13-closing.wav
├── narration.wav
├── narration.srt
├── manifest.json
└── concat.txt
```

其中：

- `narration.wav`：合并后的完整旁白；
- `narration.srt`：按实际语音时长生成的字幕；
- `manifest.json`：每段文本、音色、风格和真实时长；
- 独立 WAV：便于在剪映中逐段调整停顿和镜头。

已生成的段落默认会复用。需要重新生成全部音频时：

```powershell
uv run scripts/mimo_tts.py `
  docs/experience/product-video-tts.json `
  --overwrite
```

## 更换音色与语气

更换为女声：

```powershell
uv run scripts/mimo_tts.py `
  docs/experience/product-video-tts.json `
  --voice 茉莉 `
  --overwrite
```

临时修改整体表达风格：

```powershell
uv run scripts/mimo_tts.py `
  docs/experience/product-video-tts.json `
  --instruction "使用自然清晰的中文普通话女声。语速中等偏快，表达克制但有好奇心，像产品作者在进行现场演示。" `
  --overwrite
```

也可以在 JSON 中为单独段落设置 `voice` 或 `instruction`。

## 导入剪映

1. 新建 1920×1080、30 FPS 项目。
2. 导入 `img/product-video-v2/` 下的图片以及需要的动态录屏。
3. 优先导入独立 WAV 片段，而不是完整 `narration.wav`，这样更容易调整镜头节奏。
4. 导入 `narration.srt` 作为字幕；如果逐段移动音频，需要同步调整字幕时间。
5. 按 `docs/experience/product-video-v1.md` 的分镜顺序铺设素材。
6. 静态截图使用缓慢的缩放或平移，不要连续使用强转场。
7. 旁白响度先保持一致，再添加低音量背景音乐。
8. 最终导出前检查 API Key、私人订阅地址和系统通知是否出现在画面中。

## 常见问题

### 提示没有 `MIMO_API_KEY`

确认环境变量是在执行脚本的同一个 PowerShell 窗口中设置。

### 某一段失败后是否需要全部重来

不需要。脚本会保留已经生成的 WAV，再次运行时只请求缺失段落。

### 想只生成音频片段，不合并

```powershell
uv run scripts/mimo_tts.py `
  docs/experience/product-video-tts.json `
  --no-merge
```

### 中文发音不符合预期

优先修改该段的 `instruction`，明确语速、停顿、情绪和需要强调的词。MiMo 的自然语言风格指令放在 `user` 消息中，真正需要合成的文本放在 `assistant` 消息中。

## 参考

- [MiMo V2.5 TTS 官方使用指南](https://mimo.mi.com/docs/zh-CN/quick-start/usage-guide/multimodal-understanding/speech-synthesis-v2.5)
