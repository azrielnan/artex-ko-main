"use client";
import * as React from "react";
import { api } from "@/lib/api";
import type { HoneypotAssessment } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function HoneypotsPage() {
  const [taskID, setTaskID] = React.useState("");
  const [items, setItems] = React.useState<HoneypotAssessment[]>([]);
  const [busy, setBusy] = React.useState(false);
  const [summary, setSummary] = React.useState("");
  const [error, setError] = React.useState("");
  const load = async () => {
    const id = Number(taskID);
    if (!Number.isSafeInteger(id) || id <= 0) { setError("请输入有效的任务 ID"); return; }
    setError(""); setBusy(true);
    try { const result = await api.honeypots(id); setItems(result.items ?? []); }
    catch { setError("读取蜜罐识别结果失败"); }
    finally { setBusy(false); }
  };
  const scan = async () => {
    const id = Number(taskID);
    if (!Number.isSafeInteger(id) || id <= 0) { setError("请输入有效的任务 ID"); return; }
    setError(""); setBusy(true);
    try { const result = await api.honeypotScan(id); setItems(result.items ?? []); setSummary(`已检查 ${result.scanned} 项资产，命中 ${result.matched} 项。`); }
    catch { setError("识别失败，请确认任务存在且已有资产记录"); }
    finally { setBusy(false); }
  };
  return <div className="space-y-4 p-4 md:p-6">
    <Card><CardHeader><CardTitle>蜜罐识别</CardTitle><p className="text-sm text-muted-foreground">使用内置及本地热更新签名，对任务已有资产的服务 banner、HTTP 标题/正文和 TLS 证书信息进行静态匹配；不会主动连接目标。</p></CardHeader><CardContent>
      <div className="flex flex-wrap gap-2"><input className="h-9 w-40 rounded-md border bg-background px-3" inputMode="numeric" placeholder="任务 ID" value={taskID} onChange={e=>setTaskID(e.target.value)} /><Button onClick={load} variant="outline" disabled={busy}>读取结果</Button><Button onClick={scan} disabled={busy}>{busy?"识别中…":"分析任务资产"}</Button></div>
      {summary&&<p className="mt-3 text-sm">{summary}</p>}{error&&<p role="alert" className="mt-3 text-sm text-destructive">{error}</p>}
    </CardContent></Card>
    <Card><CardHeader><CardTitle>识别结果</CardTitle></CardHeader><CardContent>
      {items.length===0?<p className="text-sm text-muted-foreground">暂无命中记录。请先输入任务 ID 并分析资产；评分表示签名匹配置信度，不是漏洞严重度。</p>:<div className="divide-y">{items.map(x=><div className="py-3" key={`${x.task_id}-${x.asset_id}`}><div className="flex flex-wrap items-center justify-between gap-2"><strong>资产 #{x.asset_id} · {(x.products??[]).join("、")||"未知产品"}</strong><span className="rounded bg-muted px-2 py-1 text-xs">置信度 {Math.round(x.score*100)}%</span></div><div className="mt-1 text-sm text-muted-foreground">{(x.evidence??[]).join("；")}</div></div>)}</div>}
    </CardContent></Card>
  </div>;
}
