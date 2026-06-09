# 同声传译 (TRSS)

轻量级 Windows 桌面应用，捕获系统音频，调用多模态 LLM API 实时翻译，以悬浮字幕形式显示。

## 特性

- 🎯 **系统音频捕获** — 自动截取电脑播放的声音，看视频无需外放
- 🌐 **完全开放式配置** — 不内置任何服务商，填 API 地址即用
- 📝 **悬浮字幕** — 半透明置顶窗口，不影响视频观看
- 🎨 **三种显示模式** — 仅翻译 / 双语 / 仅原文
- ⚡ **轻量小巧** — 安装包 4.3MB，运行内存约 50MB
- 🔌 **兼容主流 API** — 支持所有 OpenAI Chat Completions 协议的多模态模型

## 系统要求

- Windows 10/11
- 无需安装任何运行时（已内置 Go Runtime）

## 快速开始

1. 下载 `trss.exe`（或便携版 `trss-v1.0.0-portable.zip`）
2. 双击运行
3. 打开 **⚙ 设置** → **翻译配置**
4. 填写你的 API 信息：

| 字段 | 说明 | 示例 |
|------|------|------|
| 方案名称 | 自定义名称 | `英文→中文 (GPT-4o)` |
| API 地址 | API 服务地址 | `https://api.openai.com/v1` |
| API Key | 你的密钥 | `sk-xxxxxxxx` |
| 模型名称 | 支持音频的多模态模型 | `gpt-4o-audio-preview` |

5. 点击 **测试连接** 确认连通
6. 选择源语言和目标语言
7. 点击 **保存方案**
8. 回到主界面，下拉选择刚保存的方案
9. 点击 **▶ 开始**，播放你的视频
10. 字幕自动出现在屏幕底部

## 支持的模型

任何支持 **OpenAI Chat Completions 协议** 且能处理音频的多模态模型均可使用，例如：

| 模型 | API 地址 | 特点 |
|------|----------|------|
| GPT-4o | `https://api.openai.com/v1` | 翻译质量最高 |
| Gemini Flash | `https://generativelanguage.googleapis.com/v1beta` | 性价比高，延迟低 |
| 通义千问-Audio | 阿里云百炼平台 | 国内访问方便 |
| 自定义中转/代理 | 自填地址 | 兼容 OpenAI 协议即可 |

## 提示词自定义

提示词支持 `{source}` 和 `{target}` 变量，会自动替换为当前选择的语言：

```
将{source}实时翻译为{target}。要求简洁自然，适合字幕阅读。
保留原意，不添加解释。每次只输出翻译后的一句话。
```

## 开发

```bash
# 克隆项目
git clone git@github.com:xh1126xx/trss.git
cd trss

# 构建（需要 Go 1.25+ 和 Node.js）
go run github.com/wailsapp/wails/v2/cmd/wails@latest build

# 运行测试
go test ./...

# 开发模式
go run github.com/wailsapp/wails/v2/cmd/wails@latest dev
```

## 技术栈

- **框架**: Go + Wails v2
- **前端**: HTML/CSS/JS (Vanilla + Vite)
- **音频**: WASAPI Loopback (纯 Go，无 CGo)
- **打包**: 11MB 可执行文件

## License

MIT
