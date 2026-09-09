"use client";

import { useRef } from "react";

function valueToEvenPct(value: number, markValues: readonly number[]) {
  if (markValues.length <= 1) return 0;
  if (value <= markValues[0]!) return 0;
  const last = markValues[markValues.length - 1]!;
  if (value >= last) return 100;
  for (let i = 0; i < markValues.length - 1; i += 1) {
    const a = markValues[i]!;
    const b = markValues[i + 1]!;
    if (value >= a && value <= b) {
      const t = (value - a) / (b - a);
      return ((i + t) / (markValues.length - 1)) * 100;
    }
  }
  return 100;
}

function evenPctToValue(pct: number, markValues: readonly number[]) {
  if (markValues.length === 0) return 0;
  if (markValues.length === 1) return markValues[0]!;
  const pos = (Math.min(100, Math.max(0, pct)) / 100) * (markValues.length - 1);
  const i = Math.min(markValues.length - 2, Math.floor(pos));
  const t = pos - i;
  const a = markValues[i]!;
  const b = markValues[i + 1]!;
  return a + t * (b - a);
}

export function DualRangeSlider({
  low,
  high,
  marks,
  onLowChange,
  onHighChange,
}: {
  low: number;
  high: number;
  marks: readonly { label: string; value: number }[];
  onLowChange: (value: number) => void;
  onHighChange: (value: number) => void;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<"low" | "high" | null>(null);
  const markValues = marks.map((mark) => mark.value);
  const min = markValues[0] ?? 0;
  const max = markValues[markValues.length - 1] ?? 100;

  const clampedLow = Math.min(Math.max(low, min), high);
  const clampedHigh = Math.max(Math.min(high, max), clampedLow);
  const lowPct = valueToEvenPct(clampedLow, markValues);
  const highPct = valueToEvenPct(clampedHigh, markValues);

  function pctFromClientX(clientX: number) {
    const rect = trackRef.current?.getBoundingClientRect();
    if (!rect || rect.width === 0) return 0;
    return Math.min(1, Math.max(0, (clientX - rect.left) / rect.width)) * 100;
  }

  function applyDrag(clientX: number) {
    const next = evenPctToValue(pctFromClientX(clientX), markValues);
    if (dragRef.current === "low") {
      onLowChange(Math.min(next, clampedHigh));
    } else if (dragRef.current === "high") {
      onHighChange(Math.max(next, clampedLow));
    }
  }

  function startDrag(which: "low" | "high") {
    dragRef.current = which;
    function onMove(event: PointerEvent) {
      applyDrag(event.clientX);
    }
    function onUp() {
      dragRef.current = null;
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    }
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  }

  return (
    <div className="project-dual-slider">
      <div
        ref={trackRef}
        className="project-dual-slider-track"
        onPointerDown={(event) => {
          const pct = pctFromClientX(event.clientX);
          const next = evenPctToValue(pct, markValues);
          const distLow = Math.abs(next - clampedLow);
          const distHigh = Math.abs(next - clampedHigh);
          dragRef.current = distLow <= distHigh ? "low" : "high";
          applyDrag(event.clientX);
          startDrag(dragRef.current);
        }}
      >
        <div
          className="project-dual-slider-bar"
          style={{ left: `${lowPct}%`, width: `${Math.max(0, highPct - lowPct)}%` }}
        />
        <button
          type="button"
          className="project-dual-slider-thumb"
          style={{ left: `${lowPct}%` }}
          aria-label="请求"
          onPointerDown={(event) => {
            event.stopPropagation();
            startDrag("low");
          }}
        />
        <button
          type="button"
          className="project-dual-slider-thumb"
          style={{ left: `${highPct}%` }}
          aria-label="限制"
          onPointerDown={(event) => {
            event.stopPropagation();
            startDrag("high");
          }}
        />
      </div>
      <div className="project-dual-slider-marks">
        {marks.map((mark, index) => {
          const pct = marks.length <= 1 ? 0 : (index / (marks.length - 1)) * 100;
          return (
            <button
              key={mark.label}
              type="button"
              className="project-resource-mark"
              style={{ left: `${pct}%` }}
              onClick={() => {
                const snapped = mark.value;
                if (Math.abs(snapped - clampedLow) <= Math.abs(snapped - clampedHigh)) {
                  onLowChange(Math.min(snapped, clampedHigh));
                } else {
                  onHighChange(Math.max(snapped, clampedLow));
                }
              }}
            >
              {mark.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
