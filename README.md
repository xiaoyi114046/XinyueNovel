# 心阅小说

心阅小说 v4.1.3 Windows 源码。

主要功能：DeepSeek 小说续写、本地多项目、世界观/境界/大纲、历史章节管理、章节修改/重命名/删除、完整 TXT 导出、本地加密保存 API Key、情感细腻风等。

> 注意：API Key 和用户本地小说数据不应提交到仓库。

## 构建

Windows 下可运行：

    build_windows.bat

或使用 Go：

    set GOOS=windows
    set GOARCH=amd64
    go build -trimpath -ldflags="-s -w -H windowsgui" -o XinyueNovel.exe .
