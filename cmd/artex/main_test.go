package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/agent"
)

func TestShutdownContextPreservesNamedCause(t *testing.T) {
	signalCtx, signalCancel := context.WithCancel(context.Background())
	ctx, shutdown := shutdownContext(signalCtx)
	defer shutdown(nil)
	signalCancel()
	<-ctx.Done()
	if code, _, _, ok := agent.AbortReason(ctx); !ok || code != "shutdown" {
		t.Fatalf("code=%q ok=%v, want shutdown", code, ok)
	}
}

// TestPrintBannerLocalized verifies that the startup banner printed to stdout is
// Chinese. This is the first thing a user sees when running ARTEX.
// The "[config] ..." log lines in run() are intentionally out of scope — they
// are logs, which the localization brief ranks lowest (backlog Z2).
func TestPrintBannerLocalized(t *testing.T) {
	out := captureStdout(t, func() { printBanner(":8787") })
	t.Logf("启动横幅样例:\n%s", out)

	if !strings.Contains(out, "自主渗透测试系统") {
		t.Fatalf("启动横幅不是中文: %q", out)
	}

	for _, want := range []string{"自主渗透测试系统", "版本", "监听地址", ":8787"} {
		if !strings.Contains(out, want) {
			t.Errorf("启动横幅缺少 %q: %q", want, out)
		}
	}

	for _, r := range out {
		_ = r
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns whatever
// fn wrote. printBanner uses fmt.Print*, which resolves os.Stdout at call time,
// so swapping it here captures the banner.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}
