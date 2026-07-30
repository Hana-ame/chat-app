# UI/UX 打磨 TODO

## High Priority

- [ ] **Loading skeleton** — 聊天列表和消息加载时显示占位骨架，避免白闪
- [ ] **移动端布局适配** — 侧栏/主区域在窄屏下响应式布局
- [ ] **公共频道搜索结果清理** — Join 后清空搜索状态和结果
- [ ] **Tab 键顺序** — 修 `column-reverse` 导致的焦点颠倒

## Medium Priority

- [ ] **图片预览** — 点击附件图片弹出大图预览
- [ ] **输入框自动高度** — textarea 随内容自动增高，替代固定 rows=1
- [ ] **emoji 选择器扩展** — 替换 10 个硬编码 emoji 为完整选择器
- [ ] **成员搜索防抖** — MemberPanel 搜索加 debounce，减少 API 请求
- [ ] **聊天创建选可见性** — 建群表单加 public/unlisted/private 选项

## Low Priority

- [ ] **消息 pin/置顶** — Pin 消息到群顶部
- [ ] **拖拽上传** — 拖拽文件到聊天区域直接上传
- [ ] **已读回执** — 显示消息已读状态
- [ ] **聊天置顶排序** — 置顶聊天在侧栏排序优化

