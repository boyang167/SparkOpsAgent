"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { productDetailTabIcons } from "./product-detail-icons";

export interface ProductMasterListItem {
  id: string;
  title: string;
  subtitle?: string;
  meta?: string;
  isGroup?: boolean;
  indent?: boolean;
}

interface ProductMasterDetailProps {
  sidebarTitle: string;
  searchPlaceholder?: string;
  searchQuery: string;
  onSearchChange: (query: string) => void;
  items: ProductMasterListItem[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onAdd?: () => void;
  addAriaLabel?: string;
  loading?: boolean;
  toolbarExtra?: ReactNode;
  listEmpty?: ReactNode;
  children: ReactNode;
  emptyMain?: ReactNode;
  showMain?: boolean;
}

export function ProductMasterDetail({
  sidebarTitle,
  searchPlaceholder = "搜索",
  searchQuery,
  onSearchChange,
  items,
  selectedId,
  onSelect,
  onAdd,
  addAriaLabel = "新建",
  loading = false,
  toolbarExtra,
  listEmpty,
  children,
  emptyMain,
  showMain = true,
}: ProductMasterDetailProps) {
  const groupIds = useMemo(
    () => items.filter((item) => item.isGroup).map((item) => item.id),
    [items],
  );

  const [collapsedIds, setCollapsedIds] = useState<Set<string>>(() => new Set());

  // 清理已删除的分组；保留用户折叠状态
  useEffect(() => {
    setCollapsedIds((prev) => {
      const next = new Set([...prev].filter((id) => groupIds.includes(id)));
      if (next.size === prev.size && [...next].every((id) => prev.has(id))) return prev;
      return next;
    });
  }, [groupIds]);

  // 选中资源所在组自动展开
  useEffect(() => {
    if (!selectedId || selectedId.startsWith("group:")) return;
    const index = items.findIndex((item) => item.id === selectedId);
    if (index < 0) return;
    for (let i = index; i >= 0; i -= 1) {
      if (!items[i].isGroup) continue;
      const groupId = items[i].id;
      setCollapsedIds((prev) => {
        if (!prev.has(groupId)) return prev;
        const next = new Set(prev);
        next.delete(groupId);
        return next;
      });
      break;
    }
  }, [selectedId, items]);

  const filtered = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return items;
    const matched = new Set<string>();
    items.forEach((item, index) => {
      const hit =
        item.title.toLowerCase().includes(q) ||
        item.subtitle?.toLowerCase().includes(q) ||
        item.meta?.toLowerCase().includes(q);
      if (item.isGroup) {
        if (!hit) return;
        matched.add(item.id);
        for (let next = index + 1; next < items.length && !items[next].isGroup; next += 1) {
          matched.add(items[next].id);
        }
        return;
      }
      if (!hit) return;
      matched.add(item.id);
      for (let prev = index - 1; prev >= 0; prev -= 1) {
        if (items[prev].isGroup) {
          matched.add(items[prev].id);
          break;
        }
      }
    });
    return items.filter((item) => matched.has(item.id));
  }, [items, searchQuery]);

  const searching = searchQuery.trim().length > 0;

  const visible = useMemo(() => {
    if (searching) return filtered;
    const rows: ProductMasterListItem[] = [];
    let hideChildren = false;
    for (const item of filtered) {
      if (item.isGroup) {
        hideChildren = collapsedIds.has(item.id);
        rows.push(item);
        continue;
      }
      if (!hideChildren) rows.push(item);
    }
    return rows;
  }, [filtered, collapsedIds, searching]);

  function toggleGroup(id: string) {
    setCollapsedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <div className="product-master-detail">
      <aside className="product-master-list" aria-label={sidebarTitle}>
        <div className="product-master-list-head">{sidebarTitle}</div>

        <div className="product-master-toolbar">
          <button type="button" className="product-master-tool is-active" aria-label="列表视图" title="列表视图">
            {productDetailTabIcons.iterations}
          </button>
          <button type="button" className="product-master-tool" aria-label="排序" title="排序">
            {productDetailTabIcons.sort}
          </button>
          {onAdd ? (
            <button type="button" className="product-master-tool is-accent" aria-label={addAriaLabel} title={addAriaLabel} onClick={onAdd}>
              {productDetailTabIcons.plus}
            </button>
          ) : null}
          {toolbarExtra}
        </div>

        <div className="product-master-items" role="listbox" aria-label={`${sidebarTitle}列表`}>
          {loading ? (
            <div className="product-master-list-status">加载中…</div>
          ) : visible.length === 0 ? (
            listEmpty ?? <div className="product-master-list-status">暂无数据</div>
          ) : (
            visible.map((item) => {
              const active = selectedId === item.id;
              if (item.isGroup) {
                const collapsed = !searching && collapsedIds.has(item.id);
                return (
                  <button
                    key={item.id}
                    type="button"
                    role="option"
                    aria-selected={false}
                    aria-expanded={!collapsed}
                    className={`product-master-item is-group${collapsed ? " is-collapsed" : ""}`}
                    onClick={() => toggleGroup(item.id)}
                  >
                    <span className="product-master-item-row">
                      <span className="product-master-chevron" aria-hidden="true">
                        {collapsed ? productDetailTabIcons.chevronRight : productDetailTabIcons.chevronDown}
                      </span>
                      <span className="product-master-item-title">{item.title}</span>
                    </span>
                    {item.subtitle ? <span className="product-master-item-sub">{item.subtitle}</span> : null}
                  </button>
                );
              }
              return (
                <button
                  key={item.id}
                  type="button"
                  role="option"
                  aria-selected={active}
                  className={`product-master-item${active ? " is-active" : ""}${item.indent ? " is-indent" : ""}`}
                  onClick={() => onSelect(item.id)}
                >
                  <span className="product-master-item-title">{item.title}</span>
                  {item.subtitle ? <span className="product-master-item-sub">{item.subtitle}</span> : null}
                  {item.meta ? <span className="product-master-item-meta">{item.meta}</span> : null}
                </button>
              );
            })
          )}
        </div>

        <label className="product-master-search">
          {productDetailTabIcons.search}
          <input
            type="search"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder={searchPlaceholder}
            aria-label={searchPlaceholder}
          />
        </label>
      </aside>

      <main className="product-master-main">{showMain ? children : emptyMain}</main>
    </div>
  );
}

export default ProductMasterDetail;
