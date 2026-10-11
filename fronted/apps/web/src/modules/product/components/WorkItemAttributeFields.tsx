"use client";

import type { WorkItemPriority, WorkItemType } from "@sparkops/contracts/product";
import { WORK_ITEM_PRIORITY_OPTIONS, WORK_ITEM_TYPE_OPTIONS } from "./work-item-attributes";

interface ChipGroupProps<T extends string> {
  label: string;
  value: T;
  options: ReadonlyArray<{ value: T; label: string }>;
  disabled?: boolean;
  onChange: (value: T) => void;
}

function ChipGroup<T extends string>({ label, value, options, disabled, onChange }: ChipGroupProps<T>) {
  return (
    <div className="wi-attr-field">
      <span className="wi-attr-label">{label}</span>
      <div className="wi-attr-chips" role="radiogroup" aria-label={label}>
        {options.map((option) => {
          const selected = option.value === value;
          return (
            <button
              key={option.value}
              type="button"
              role="radio"
              aria-checked={selected}
              disabled={disabled}
              className={`wi-attr-chip is-${option.value}${selected ? " is-selected" : ""}`}
              onClick={() => {
                if (!selected) onChange(option.value);
              }}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

interface WorkItemAttributeFieldsProps {
  type: WorkItemType;
  priority: WorkItemPriority;
  disabled?: boolean;
  onTypeChange: (value: WorkItemType) => void;
  onPriorityChange: (value: WorkItemPriority) => void;
}

export function WorkItemAttributeFields({
  type,
  priority,
  disabled,
  onTypeChange,
  onPriorityChange,
}: WorkItemAttributeFieldsProps) {
  return (
    <div className="wi-attr">
      <ChipGroup label="类型" value={type} options={WORK_ITEM_TYPE_OPTIONS} disabled={disabled} onChange={onTypeChange} />
      <ChipGroup
        label="优先级"
        value={priority}
        options={WORK_ITEM_PRIORITY_OPTIONS}
        disabled={disabled}
        onChange={onPriorityChange}
      />
    </div>
  );
}
