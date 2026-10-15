"use client";

import * as React from "react";

type PollingOptions<T> = {
  load: (signal: AbortSignal) => Promise<T>;
  onData: (value: T) => void;
  intervalMs: number;
  enabled?: boolean;
};

/**
 * 生命周期感知的轮询：隐藏页面暂停，恢复可见立即刷新；请求可取消且失败指数退避。
 * load/onData 通过 ref 读取，避免调用方每次渲染都重建定时器。
 */
export function usePolling<T>({ load, onData, intervalMs, enabled = true }: PollingOptions<T>) {
  const loadRef = React.useRef(load);
  const onDataRef = React.useRef(onData);
  React.useEffect(() => {
    loadRef.current = load;
  }, [load]);
  React.useEffect(() => {
    onDataRef.current = onData;
  }, [onData]);

  React.useEffect(() => {
    if (!enabled) return;
    let active = true;
    let hidden = typeof document !== "undefined" && document.visibilityState === "hidden";
    let timer: ReturnType<typeof setTimeout> | undefined;
    let controller: AbortController | undefined;
    let running = false;
    let failures = 0;

    const clearTimer = () => {
      if (timer !== undefined) clearTimeout(timer);
      timer = undefined;
    };

    const nextDelay = () => Math.min(30_000, intervalMs * 2 ** Math.min(failures, 3));

    const run = async () => {
      if (!active || hidden || running) return;
      running = true;
      controller = new AbortController();
      try {
        const value = await loadRef.current(controller.signal);
        if (active && !controller.signal.aborted) {
          failures = 0;
          onDataRef.current(value);
        }
      } catch (error) {
        if (active && !(error instanceof DOMException && error.name === "AbortError")) failures += 1;
      } finally {
        running = false;
        controller = undefined;
        if (active && !hidden) {
          clearTimer();
          timer = setTimeout(() => void run(), nextDelay());
        }
      }
    };

    const onVisibility = () => {
      hidden = document.visibilityState === "hidden";
      if (hidden) {
        clearTimer();
        controller?.abort();
      } else {
        failures = 0;
        void run();
      }
    };

    document.addEventListener("visibilitychange", onVisibility);
    void run();
    return () => {
      active = false;
      clearTimer();
      controller?.abort();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [enabled, intervalMs]);
}
   