"use client";

import { STEP_PALETTE_MIME } from "../../lib/flow-adapter";
import { STEP_CATALOG } from "../../mock/data";

interface StepPaletteProps {
  onAdd: (uses: string) => void;
}

export function StepPalette({ onAdd }: StepPaletteProps) {
  const groups = STEP_CATALOG.reduce<Record<string, typeof STEP_CATALOG>>((acc, item) => {
    (acc[item.category] ??= []).push(item);
    return acc;
  }, {});

  return (
    <aside className="cd-catalog">
      <h3 className="cd-side-title">Step 库</h3>
      <p className="muted text-sm cd-catalog-hint">拖到画布，或点击添加</p>
      {Object.entries(groups).map(([category, items]) => (
        <div key={category} className="cd-catalog-group">
          <p className="cd-catalog-group-title">{category}</p>
          {items.map((item) => (
            <button
              key={item.uses}
              type="button"
              className="cd-catalog-item"
              draggable
              onDragStart={(event) => {
                event.dataTransfer.setData(STEP_PALETTE_MIME, item.uses);
                event.dataTransfer.effectAllowed = "move";
              }}
              onClick={() => onAdd(item.uses)}
            >
              <span>{item.label}</span>
              {item.pauses ? <span className="badge badge-warning">暂停</span> : null}
            </button>
          ))}
        </div>
      ))}
    </aside>
  );
}
