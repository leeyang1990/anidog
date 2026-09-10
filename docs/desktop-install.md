# AniDog 桌面版安装

桌面版使用本机 SQLite 与内嵌 BT，不依赖 Docker、PostgreSQL 或 qBittorrent。桌面与服务器的数据相互独立。

| 系统 | 下载附件 | 安装 |
| --- | --- | --- |
| macOS Apple Silicon | macos-arm64.zip | 解压，将 AniDog.app 拖到 Applications |
| macOS Intel | macos-amd64.zip | 解压，将 AniDog.app 拖到 Applications |
| Windows 10/11 x64 | windows-amd64.zip | 解压后运行 AniDog.exe，需要 Microsoft Edge WebView2 Runtime |
| Linux x64 | linux-amd64.tar.gz | 解压后运行 `./AniDog`；在 Ubuntu 24.04 构建，需要桌面图形会话 |

Linux 在 Ubuntu 24.04 上安装运行库：

```bash
sudo apt-get install libgtk-3-0t64 libwebkit2gtk-4.1-0
tar -xzf AniDog-vX.Y.Z-linux-amd64.tar.gz
./AniDog
```

Linux 包不是静态可执行程序，也不是 AppImage；其他发行版需要兼容的 glibc、GTK3、WebKitGTK 4.1。无图形界面的服务器请使用 Docker 版。

macOS 包为自签名，尚未 Apple 公证。Windows 包尚未 Authenticode 签名。仅在确认下载来源和校验和后，按系统的应用安全提示允许运行。

首次启动注册本地管理员。默认下载目录为用户主目录下的 `Downloads/AniDog`，可在设置中修改。配置和数据库分别存储在 macOS 的 `~/Library/Application Support/AniDog`、Windows 的 `%AppData%\AniDog`、Linux 的 `~/.config/AniDog`（或 `$XDG_CONFIG_HOME/AniDog`）。

BT 下载开箱可用；流媒体抓取还需要本机安装 `ffmpeg` / `ffprobe` 并使其可在 PATH 中找到，浏览器由 Rod 管理，首次使用可能需要联网下载。压缩包不附带这些外部程序。

Release 附带 `SHA256SUMS`，可用 `sha256sum --check SHA256SUMS`（Linux）或 `shasum -a 256`（macOS）校验附件。
