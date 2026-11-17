"use client";

import * as React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { ChainGuide } from "@/lib/types";

export default function ChainsPage() {
  const [items, setItems] = React.useState<ChainGuide[]>([]);
  const [error, setError] = React.useState("");
  React.useEffect(() => {
    api.chains().then((result) => setItems(result.items ?? [])).catch((e) => {
      setError(e instanceof Error ? e.message : "读取场景检查清单失败");
    });
  }, []);

  return <div className="space-y-4 p-4 md:p-6">
    <Card>
      <CardHeader>
        <CardTitle>Chains 场景化</CardTitle>
        <p className="text-sm text-muted-foreground">以下清单来自 Agent 实际使用的规则表。Worker 会根据任务意图自动注入最多两个匹配场景的反问和转向建议；展开卡片可查看完整检查项。</p>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      </CardHeader>
      <CardContent>
        {items.length === 0 && !error && <p className="py-6 text-center text-sm text-muted-foreground">正在读取场景规则…</p>}
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {items.map((guide) => <details className="group rounded-lg border p-3 open:border-primary/40 open:bg-muted/20" key={guide.tag}>
            <summary className="cursor-pointer list-none">
              <span className="flex items-center justify-between gap-3 font-medium"><span>{guide.name}</span><span className="text-xs text-muted-foreground group-open:hidden">查看清单 ↓</span><span className="hidden text-xs text-muted-foreground group-open:inline">收起 ↑</span></span>
              <span className="mt-1 block text-sm text-muted-foreground">{guide.summary}</span>
            </summary>
            <div className="mt-3 space-y-3 border-t pt-3 text-sm">
              <section><h2 className="font-medium">核验问题</h2><ul className="mt-1 list-disc space-y-1 pl-5 text-muted-foreground">{guide.questions.map((q) => <li key={q}>{q}</li>)}</ul></section>
              <section><h2 className="font-medium">转向建议</h2><ul className="mt-1 list-disc space-y-1 pl-5 text-muted-foreground">{guide.turns.map((turn) => <li key={turn}>{turn}</li>)}</ul></section>
            </div>
          </details>)}
        </div>
        <p className="mt-4 rounded-md bg-muted p-3 text-xs text-muted-foreground">工作流判定规则：没有决定性证据时记录为 inferred / unverified；只有经过独立复现的漏洞才提交 finding。此页面用于查看 Agent 使用的场景清单，不会单独改变运行中的任务配置。</p>
      </CardContent>
    </Card>
  </div>;
}
