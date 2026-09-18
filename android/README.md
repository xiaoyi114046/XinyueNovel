# 心阅小说 Android v6.1 Multi-AI

## 本次更新

### 多 AI 接入口

内置并可切换：

- DeepSeek
- 豆包 / 火山方舟
- 智谱 GLM
- 通义千问 / DashScope
- GPT / OpenAI（Responses API）
- Gemini
- Grok / xAI
- Claude / Anthropic
- 自定义 OpenAI Chat Completions 兼容接口

每个平台都支持单独保存 API Key、接口地址、模型名称。API Key 通过 Android Keystore + AES/GCM 保存，不进入 localStorage，也不会提交到 GitHub。

### 删除本章修复（v6.1）

这次同时修复了 **Android WebView 的 JavaScript 确认框问题**。旧版虽然有删除逻辑，但 MainActivity 没有配置 WebChromeClient，`confirm()` 在部分 Android WebView 上会表现为按钮无反应。

现在流程为：

1. 正常弹出“确定删除”确认框；
2. 删除当前章节；
3. 自动选中后一章；如果没有后一章则选中前一章；
4. 如果删的是唯一章节，自动创建一个新的空白章节；
5. 立即保存并刷新编辑器与章节列表。

### 当前默认接口

默认值按 2026-09 的官方接口整理，接口地址与模型仍可在 APP 内手工修改：DeepSeek、豆包、智谱、千问、OpenAI、Gemini、Grok、Claude。

## Android Studio

打开本仓库中的 `android` 目录。JDK 17，compileSdk 35。

```bash
gradle :app:assembleDebug
```

APK：`app/build/outputs/apk/debug/app-debug.apk`

## 测试

```bash
node --check app/src/main/assets/core.js
node --check app/src/main/assets/app.js
node tests/core_test.js
python tests/static_app_test.py
```

GitHub Actions 还会编译 APK，并在 Android 29 模拟器安装、启动，等待日志 `XINYUE_APP_READY`。
