# 心阅小说 Android v4.1.3

这是根据 Windows v4.1.3 Go 源码迁移的 Android 手机版本工程。

## 已迁移功能

- DeepSeek 小说续写；目标字数、特殊要求、最近章节上下文
- 多小说项目、本地持久化、项目切换/新建/重命名/删除
- 小说类型、网文流派、文风、叙事视角、节奏、主角、人物关系、剧情脉络
- 世界观、境界体系、当前境界、总纲、卷纲、本章小纲
- AI 生成境界、AI 生成总纲/卷纲
- 历史章节查看、修改、重命名、删除、作为续写上下文
- 完整小说 TXT 导出；Android 10+ 导出到 下载/XinyueNovel
- DeepSeek API 地址/模型/思考模式；连接测试
- API Key 使用 Android Keystore (AES/GCM) 加密保存
- 生成请求可停止

## 构建

需要 JDK 17、Android SDK（compileSdk 35）和 Gradle 8.9：

```bash
gradle :app:assembleDebug
```

APK：`app/build/outputs/apk/debug/app-debug.apk`

也可把整个工程推送到 GitHub，仓库自带 `.github/workflows/android.yml`，Actions 会运行核心测试并构建 APK Artifact。

## 测试

```bash
node tests/core_test.js
```

原 Windows Go 核心测试由源仓库 `go test ./...` 验证。

## 自动验证

GitHub Actions 会依次执行：

1. 业务核心 JavaScript 单元测试；
2. HTML/JS/Android Bridge 静态一致性检查；
3. `assembleDebug` 编译 APK；
4. Android 29 模拟器安装并启动 `MainActivity`；
5. 上传 `XinyueNovel-Android-debug` APK Artifact。

详细状态见 `BUILD_STATUS.md`。
