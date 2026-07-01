# AC 组件库（动森风 / Animal Crossing UI）

本目录是 AniDog 前端的全自研组件库，零外部 UI 依赖（无 naive-ui / element-plus / vant）。所有组件遵循动森风视觉语言：

- **配色**：米白沙地 (`ac-cream`) + 草绿主色 (`ac-grass`) + 暖橙强调 (`ac-sun`) + 棕字 (`ac-earth/wood`)
- **形状**：18-32px 圆角、暖棕软阴影、凸起按钮（底部深色边模拟厚度）
- **字体**：ZCOOL KuaiLe（中文圆体）+ Nunito（英文 / 数字）

## 快速使用

```vue
<script setup>
import { AcButton, AcCard, AcModal, AcInput } from '@/components/ac'
import { useToast } from '@/composables/useToast'
import { useConfirm } from '@/composables/useConfirm'

const toast = useToast()
const { confirm } = useConfirm()

async function onDelete() {
  const ok = await confirm({ title: '确认删除', content: '此操作不可撤销', variant: 'danger' })
  if (ok) toast.success('已删除')
}
</script>
```

## 组件清单

| 组件 | 主要 props | 说明 |
|---|---|---|
| `AcButton` | `variant=primary\|secondary\|outline\|ghost\|danger\|sun`，`size=sm\|md\|lg`，`loading`，`disabled` | 凸起按钮，hover 抬升、active 下沉 |
| `AcCard` | `padding`，`rounded`，`hover` | 圆润卡片 |
| `AcInput` | `v-model`，`size`，`clearable`，`prefix/suffix slot` | 圆框输入 |
| `AcTextarea` | `v-model`，`rows`，`autosize` | 多行文本 |
| `AcSelect` | `v-model`，`options=[{label,value}]` | 自研 popper 下拉 |
| `AcCheckbox` | `v-model`，`label` | 叶子勾选 |
| `AcRadio` / `AcRadioGroup` | `v-model`，`value`，`options` | |
| `AcSwitch` | `v-model`，`size` | 圆滑开关 |
| `AcTag` | `variant=default\|grass\|sun\|sky\|heart\|leaf\|wood`，`size=sm\|md` | 圆药丸标签 |
| `AcModal` | `v-model:show`，`title`，`max-width`，`mask-closable` | 居中圆润 modal，焦点陷阱 + ESC 关闭 + body scroll lock |
| `AcDrawer` | `v-model:show`，`title`，`width`，`placement=right\|left` | 抽屉 |
| `AcDropdown` | `options`，`@select` | 点击触发 + 外部点击关闭 |
| `AcSpinner` | `size` | 旋转加载 |
| `AcTabs` | `v-model`，`tabs=[{label,key}]` | 木牌风 tab |
| `AcCollapse` | `title`，`open` | 简单折叠 |
| `AcTable` | `columns`，`data`，`row-key` | 朴素 table，含排序 |
| `AcToastContainer` | (单例) | 由 `useToast()` 调用 |
| `AcConfirmHost` | (单例) | 由 `useConfirm()` 调用 |
| `AcLoadingBar` | (单例) | 由 `useLoadingBar()` 调用 |
| `AcEmpty` | `title`，`description`，slot `actions` | 空态 |
| `AcSkeleton` | `lines`，`width` | 占位 |
| `AcProgress` | `value=0..100`，`status` | 圆条进度 |
| `AcPageHeader` | `title`，`subtitle`，slot `actions` | 页面头 |

## Composables

| 名称 | 返回 | 用途 |
|---|---|---|
| `useToast()` | `{ success, error, warning, info, loading }` | 自研单例 toast，替代 `useMessage()` |
| `useConfirm()` | `{ confirm({title, content, variant}) }` | 返回 `Promise<boolean>` |
| `useDialog()` | `{ warning, error, info, success }` | 兼容旧 naive-ui `useDialog().warning` 风格 |
| `useLoadingBar()` | `{ start, finish, error }` | 顶部加载条 |
| `useFocusTrap(el, opts)` | — | Modal 焦点陷阱 |
| `useClickOutside(el, fn)` | — | Dropdown 外部关闭 |
| `useTypewriter(text, speed)` | `{ display }` | AcEmpty 打字机文案 |

## 设计 Token

所有色彩通过 Tailwind 类直接使用，源自 `tailwind.config.js` 的 `colors.ac.*`：

```html
<div class="bg-ac-cream text-ac-earth border-2 border-ac-sand rounded-3xl">
  <button class="bg-ac-grass text-white shadow-ac-button">动森按钮</button>
</div>
```

阴影：`shadow-ac`（软棕）、`shadow-ac-button`（凸起按钮底边）。
动效：`animate-bounce-soft`、`animate-press-down`、`animate-typewriter`、`animate-fade-in`。

## 拓展约定

- 新增组件命名一律 `Ac<Name>.vue`，从 `index.js` 统一 export
- `<script setup>` + `defineProps/defineEmits`
- 颜色不要用 `text-blue-500` 这种硬编码，用 `text-ac-grass-dark` 等
- 圆角默认 `rounded-2xl`（18px），重要 modal/卡片用 `rounded-3xl`（24px）
