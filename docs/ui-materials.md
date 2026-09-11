# UI 材质分层

业务页面仍是一套 Vue UI，下载业务仍是一套 Go 服务。原生窗口效果和网页面板效果是两层不同的能力。

| 运行环境 | 原生窗口层 | 公共前端层 |
| --- | --- | --- |
| macOS 26+ | NSGlassEffectView | 半透明工作区、卡片、磨砂弹层 |
| 较早 macOS | NSVisualEffectView | 同一套公共面板 |
| Windows / Linux | 当前不启用原生桌面背景材质 | 检测 WebView 的 CSS blur 能力 |
| Docker / Web | 浏览器窗口，不使用桌面桥 | 检测浏览器的 CSS blur 能力 |

Windows 当前没有接入 Mica/Acrylic；Linux 没有要求 compositor 支持窗口透明。CSS 磨砂只能采样网页内的背景，不等于透过窗口看到桌面。

## 代码边界

1. `backend/internal/desktopappearance`：Go build tags 在编译期隔离平台。`appearance_darwin.go/.m` 处理 AppKit；`appearance_other.go` 是 Windows/Linux/no-cgo 的无原生材质适配。统一返回 platform、mode、reduce_transparency、reduce_motion。
2. `frontend/src/utils/appearancePolicy.js`：纯函数，把原生能力、CSS 支持和无障碍偏好转换为 windowMaterial、uiMaterial、reduceMotion。这里不操作 DOM。
3. `frontend/src/utils/desktopAppearance.js`：生命周期控制器，订阅系统偏好和主题变化，串行调用桥接，卸载时清理监听；桥接失败仍有可用的网页材质。
4. `frontend/src/assets/materials.css`：跨平台的工作区、panel、overlay、popover、control、table 材质变量。公共组件只声明 ui-panel 等语义类，不判断操作系统。
5. `frontend/src/assets/nativeWindow.css`：仅处理原生背景透明与窗口拖动区域，不定义字体、面板和业务页样式。

字体由皮肤决定，与操作系统及磨砂开关无关。`fonts.css` 从锁定版本的 Fontsource 包导入 ZCOOL KuaiLe 与 Nunito 本地字体，Vite 将字体写入 dist，再由 Wails 嵌入应用；Web/Docker 使用同一份静态资源，不再请求 Google Fonts。动森主题保持圆体，classic 保持系统字体。字体许可随 `public/font-licenses/` 一起发布。

后续若实现 Windows Mica，只需添加相应原生适配和窗口衔接规则，不要复制 Settings/Downloads 页面，也不要在每张卡片里添加平台判断。

### 最小代码约束

- 一套组件、一套布局、一套材质规则；平台文件优先只覆盖 CSS 变量，不重新定义卡片/弹层样式。
- 有真实差异才新增原生实现。Windows/Linux 当前行为相同，共用 fallback，不为了文件对称拆成两份。
- 不引入平台组件工厂、额外 Provider 层或平行主题树；纯策略函数 + 生命周期控制器已经足够。
- 公共边框、阴影、圆角合并声明；保留必要的无障碍、异步与卸载保护，不以压缩代码行数替代抽象。
- 架构回归测试限制平台样式进入公共组件与业务页面。

## 视觉与性能

- 工作区统一模糊一次；列表卡片只做分层半透明，不给每一行创建模糊层。
- Teleport 到 body 的弹窗、抽屉、下拉菜单使用根级 tokens，不依赖侧栏或父面板选择器。
- 输入框与表格保持更高的不透明度，防止背景干扰文字。
- 深色单独调整不透明度、高光与阴影，而非简单反相。
- 减少透明度、高对比度、CSS blur 不受支持时使用实底；减少动态效果时停止界面动画。
- WebView 对 prefers-reduced-transparency 的支持不同，不能宣称每个系统偏好均可被浏览器识别；macOS 额外使用原生无障碍通知。

## 验证

- `cd frontend && npm test`：平台/材质矩阵、无障碍策略、并发、桥接失败和卸载清理。
- `cd backend && go test -tags 'desktop nosqlite' ./cmd/anidog-desktop ./internal/desktopappearance`。
- macOS 原生冒烟测试见 `backend/internal/desktopappearance/testdata/material_smoke.m`；Release 工作流在 macOS 构建中执行。
- 本地完整打包使用 `wails build -skipbindings`。当前外置前端目录配置下，不要用 `-s` 跳过前端构建，否则可能只嵌入 dist 占位文件。
