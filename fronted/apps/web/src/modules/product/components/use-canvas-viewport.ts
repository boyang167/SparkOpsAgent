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

export function computeFitViewport(
  containerWidth: number,
  containerHeight: number,
  contentWidth: number,
  contentHeight: number,
  padding = 40,
): CanvasViewport {
  if (containerWidth <= 0 || containerHeight <= 0) {
    return { scale: 1, offsetX: 0, offsetY: 0 };
  }
  if (contentWidth <= 0 || contentHeight <= 0) {
    return { scale: 1, offsetX: padding, offsetY: padding };
  }

  const availableWidth = Math.max(containerWidth - padding * 2, 1);
  const availableHeight = Math.max(containerHeight - padding * 2, 1);
  const scale = clampScale(Math.min(1, availableWidth / contentWidth, availableHeight / contentHeight));
  return {
    scale,
    offsetX: padding + (availableWidth - contentWidth * scale) / 2,
    offsetY: padding + (availableHeight - contentHeight * scale) / 2,
  };
}

interface UseCanvasViewportOptions {
  panBlockSelector?: string;
}

export function useCanvasViewport(
  initial: CanvasViewport = { scale: 1, offsetX: 0, offsetY: 0 },
  options: UseCanvasViewportOptions = {},
) {
  const [viewport, setViewport] = useState<CanvasViewport>(initial);
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<{ startX: number; startY: number; originX: number; originY: number } | null>(null);

  const zoomBy = useCallback((delta: number) => {
    setViewport((prev) => ({ ...prev, scale: clampScale(prev.scale + delta) }));
  }, []);

  const reset = useCallback(() => {
    setViewport({ scale: 1, offsetX: 0, offsetY: 0 });
  }, []);

  const fitToView = useCallback((container: HTMLElement | null, content: HTMLElement | null, padding = 40) => {
    if (!container || !content) return;
    setViewport(
      computeFitViewport(
        container.clientWidth,
        container.clientHeight,
        content.offsetWidth,
        content.offsetHeight,
        padding,
      ),
    );
  }, []);

  const panBlockSelector = options.panBlockSelector ?? ".wf-run-live-task";

  const onPointerDown = useCallback((event: PointerEvent<HTMLElement>) => {
    if (event.button !== 0) return;
    const target = event.target as HTMLElement;
    if (panBlockSelector && target.closest(panBlockSelector)) return;
    dragRef.current = {
      startX: event.clientX,
      startY: event.clientY,
      originX: viewport.offsetX,
      originY: viewport.offsetY,
    };
    setDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  }, [viewport.offsetX, viewport.offsetY, panBlockSelector]);

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
    fitToView,
    zoomIn: () => zoomBy(SCALE_STEP),
    zoomOut: () => zoomBy(-SCALE_STEP),
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onWheel,
    dragging,
  };
}
