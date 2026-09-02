"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { bucketRecordFrames, coalesceRecordFrames, inferTerminalSize, parseRecordFrames } from "../records-view";

const PLAYBACK_FRAME_MS = 120;

export function RecordPlayer({ ndjson }: { ndjson: string }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const timersRef = useRef<number[]>([]);
  const indexRef = useRef(0);
  const [ready, setReady] = useState(false);
  const [playing, setPlaying] = useState(false);

  const playbackFrames = useMemo(() => {
    const parsed = parseRecordFrames(ndjson);
    const coalesced = coalesceRecordFrames(parsed);
    return bucketRecordFrames(coalesced, 5);
  }, [ndjson]);

  const terminalSize = useMemo(() => inferTerminalSize(parseRecordFrames(ndjson)), [ndjson]);

  const clearTimers = useCallback(() => {
    timersRef.current.forEach((timer) => window.clearTimeout(timer));
    timersRef.current = [];
  }, []);

  const stop = useCallback(() => {
    clearTimers();
    setPlaying(false);
  }, [clearTimers]);

  const resetTerminal = useCallback(() => {
    stop();
    indexRef.current = 0;
    termRef.current?.reset();
  }, [stop]);

  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;

    const term = new Terminal({
      cols: terminalSize.width,
      rows: terminalSize.height,
      scrollback: 10000,
      convertEol: true,
      fontFamily: "Consolas, \"Courier New\", monospace",
      fontSize: 14,
      theme: { background: "#0a0a0a", foreground: "#e5e5e5", cursor: "#e5e5e5" },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(host);
    fit.fit();
    termRef.current = term;
    fitRef.current = fit;
    setReady(true);

    const observer = new ResizeObserver(() => fit.fit());
    observer.observe(host);

    return () => {
      observer.disconnect();
      stop();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
      setReady(false);
    };
  }, [stop, terminalSize.height, terminalSize.width]);

  const play = useCallback(() => {
    if (!termRef.current || playbackFrames.length === 0) return;
    clearTimers();
    if (indexRef.current >= playbackFrames.length) {
      termRef.current.reset();
      indexRef.current = 0;
    }
    setPlaying(true);

    const schedule = () => {
      const term = termRef.current;
      if (!term) return;
      const index = indexRef.current;
      if (index >= playbackFrames.length) {
        setPlaying(false);
        return;
      }
      term.write(playbackFrames[index].data);
      indexRef.current = index + 1;
      if (indexRef.current >= playbackFrames.length) {
        setPlaying(false);
        return;
      }
      timersRef.current.push(window.setTimeout(schedule, PLAYBACK_FRAME_MS));
    };

    schedule();
  }, [clearTimers, playbackFrames]);

  const pause = useCallback(() => {
    stop();
  }, [stop]);

  return (
    <div className="stack-md">
      <p className="text-sm text-muted">提示：按固定帧率回放终端输出，空格可播放/暂停（播放器按钮）。</p>
      <div ref={hostRef} className="h-[420px] rounded border border-border bg-[#0a0a0a] p-1" />
      <div className="flex flex-wrap justify-center gap-3">
        <button type="button" className="btn-secondary text-sm" disabled={!ready} onClick={() => void play()}>
          播放
        </button>
        <button type="button" className="btn-secondary text-sm" disabled={!ready || !playing} onClick={pause}>
          暂停
        </button>
        <button type="button" className="btn-secondary text-sm" disabled={!ready} onClick={resetTerminal}>
          重播
        </button>
      </div>
    </div>
  );
}
