"use client";

import { useCallback, useRef, useState, type PointerEvent, type WheelEvent } from "react";

const MIN_SCALE = 0.5;
const MAX_SCALE = 1.5;
const SCALE_STEP = 0.1;

export interface CanvasViewport {
  scale: number;
  offsetX: number;
  offsetY: number;
}

export function clampScale(value: number): number {
  return Math.min(MAX_SCALE, Math.max(MIN_SCALE, Number(value.toFixed(2))));
}

export function useCanvasViewport(initial: CanvasViewport = { scale: 1, offsetX: 0, offsetY: 0 }) {
  const [viewport, setViewport] = useState<CanvasViewport>(initial);
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<{ startX: number; startY: number; originX: number; originY: number } | null>(null);

  const zoomBy = useCallback((delta: number) => {
    setViewport((prev) => ({ ...prev, scale: clampScale(prev.scale + delta) }));
  }, []);

  const reset = useCallback(() => {
    setViewport({ scale: 1, offsetX: 0, offsetY: 0 });
  }, []);

  const onPointerDown = useCallback((event: PointerEvent<HTMLElement>) => {
    if (event.button !== 0) return;
    const target = event.target as HTMLElement;
    if (target.closest(".wf-run-live-task")) return;
    dragRef.current = {
      startX: event.clientX,
      startY: event.clientY,
      originX: viewport.offsetX,
      originY: viewport.offsetY,
    };
    setDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  }, [viewport.offsetX, viewport.offsetY]);

  const onPointerMove = useCallback((event: PointerEvent<HTMLElement>) => {
    const drag = dragRef.current;
    if (!drag) return;
    setViewport((prev) => ({
      ...prev,
      offsetX: drag.originX + (event.clientX - drag.startX),
      offsetY: drag.originY + (event.clientY - drag.startY),
    }));
  }, []);

  const onPointerUp = useCallback((event: PointerEvent<HTMLElement>) => {
    dragRef.current = null;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }, []);

  const onWheel = useCallback((event: WheelEvent<HTMLElement>) => {
    if (!event.ctrlKey && !event.metaKey) return;
    event.preventDefault();
    const delta = event.deltaY > 0 ? -SCALE_STEP : SCALE_STEP;
    setViewport((prev) => ({ ...prev, scale: clampScale(prev.scale + delta) }));
  }, []);

  return {
    viewport,
    zoomBy,
    reset,
    zoomIn: () => zoomBy(SCALE_STEP),
    zoomOut: () => zoomBy(-SCALE_STEP),
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onWheel,
    dragging,
  };
}
