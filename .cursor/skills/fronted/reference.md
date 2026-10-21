# 前端 UI 参考

## K8s 企业级组件

路径：`fronted/apps/web/src/components/k8s/`

| 场景 | 组件 |
|------|------|
| 面板外壳 | `K8sPanelCard` |
| 资源页签 | `K8sResourceTabs` |
| 空态 | `K8sEmptyState`（variant: no-cluster / missing-def / empty） |
| 加载 | `K8sLoadingState`（表格内 compact） |
| 分页 | `K8sPagination` |
| 状态徽章 | `K8sStatusBadge` + `k8sPhaseTone()` |
| 危险确认 | `useK8sConfirm` |

K8s 页面布局：PanelCard → 工具栏（`.k8s-toolbar-row`）→ Tabs → 筛选 → 表格 → Pagination。

## 常用 globals 类速查

### 按钮
- `btn` / `btn-primary` — 主操作
- `btn-secondary` — 次要
- `btn-ghost` — 低优先级 / 表格行内

### 间距
- `stack-lg` — 1.75rem
- `stack-md` — 1.25rem
- `stack-sm` — 0.75rem

### 表单
- `input` / `select` / `textarea` — 全局表单控件样式（globals.css Forms 段）
- 字段组用 `stack-sm`，横向筛选用 `flex flex-wrap items-center gap-2`

### 列表示图
- `list-item` — 可 hover 的列表行
- `stat-card` — 统计数字展示

## Modal 用法模式

```tsx
<Modal
  open={open}
  onClose={close}
  title="新建产品"
  description="产品标识创建后不可修改"
  footer={
    <>
      <button type="button" className="btn btn-secondary" onClick={close}>取消</button>
      <button type="button" className="btn btn-primary" disabled={busy} onClick={submit}>
        {busy ? "创建中…" : "创建"}
      </button>
    </>
  }
>
  {/* form fields in stack-sm */}
</Modal>
```

## 品位感示例（Do / Don't）

**Do**
- 指标区 3–4 张 `MetricCard`，tone 按语义区分（sky 默认、emerald 成功、amber 警告）
- 空态：一句说明 + 主操作按钮
- 表格上方工具栏左筛右建

**Don't**
- 满屏 gradient 或大圆角卡片嵌套
- 每个字段不同 border 颜色
- emoji 作为唯一状态指示
- 5 个以上 primary 按钮同屏

## 模块目录约定

```
src/modules/<domain>/
├── components/     # Panel、Dialog、Tab
├── hooks/          # 可选
└── (无 pages — 页面在 app/)
```

BFF 调用走 `@sparkops/web-sdk`；错误优先 `BffError.message`。
