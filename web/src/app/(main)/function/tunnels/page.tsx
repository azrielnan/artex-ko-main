"use client";
import * as React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { Tunnel } from "@/lib/types";
import { Button } from "@/components/ui/button";

export default function TunnelsPage() {
  const [items, setItems] = React.useState<Tunnel[]>([]);
  const [name, setName] = React.useState("");
  const [target, setTarget] = React.useState("");
  const [preferred, setPreferred] = React.useState("tcp_forward");
  const [listenAddr, setListenAddr] = React.useState("");
  const [command, setCommand] = React.useState("");
  const [busy, setBusy] = React.useState<number | null>(null);
  const [error, setError] = React.useState("");
  const load = React.useCallback(() => api.tunnels().then((r) => setItems(r.items ?? [])), []);
  React.useEffect(() => { load().catch(() => setError("读取隧道记录失败")); }, [load]);
  const plan = async (e: React.FormEvent) => {
    e.preventDefault(); setError("");
    try { await api.tunnelPlan({ name, target, preferred, listen_addr: listenAddr, command }); setName(""); setTarget(""); setListenAddr(""); setCommand(""); await load(); }
    catch (e) { setError(e instanceof Error ? e.message : "计划生成失败"); }
  };
  const action = async (id: number, fn: () => Promise<unknown>) => {
    setBusy(id); setError("");
    try { await fn(); await load(); } catch (e) { setError(e instanceof Error ? e.message : "隧道操作失败"); }
    finally { setBusy(null); }
  };
  return <div className="space-y-4 p-4 md:p-6">
    <Card><CardHeader><CardTitle>隧道表与受控部署</CardTitle><p className="text-sm text-muted-foreground">tcp_forward、已配置二进制的 Chisel / suo5 都会真实启动并记录进程；外部适配器必须填写回环监听地址和完整 command。</p></CardHeader><CardContent>
      <form onSubmit={plan} className="grid gap-3 md:grid-cols-3"><label className="grid gap-1 text-sm">计划名称<input className="h-9 rounded-md border bg-background px-3" value={name} onChange={e=>setName(e.target.value)} required /></label><label className="grid gap-1 text-sm">目标地址<input className="h-9 rounded-md border bg-background px-3" value={target} onChange={e=>setTarget(e.target.value)} placeholder="host:port" required /></label><label className="grid gap-1 text-sm">适配器<select className="h-9 rounded-md border bg-background px-3" value={preferred} onChange={e=>setPreferred(e.target.value)}><option value="tcp_forward">内置 TCP 转发（可实际部署）</option><option value="chisel">Chisel（需二进制和回连端点）</option><option value="suo5">suo5（需二进制和回连端点）</option></select></label><label className="grid gap-1 text-sm">本机监听地址（外部适配器必填）<input className="h-9 rounded-md border bg-background px-3" value={listenAddr} onChange={e=>setListenAddr(e.target.value)} placeholder="127.0.0.1:1080" /></label><label className="grid gap-1 text-sm md:col-span-2">完整启动命令（外部适配器必填）<input className="h-9 rounded-md border bg-background px-3" value={command} onChange={e=>setCommand(e.target.value)} placeholder="chisel client ..." /></label><div className="flex items-end"><Button>生成计划</Button></div></form>
      {error && <p role="alert" className="mt-3 text-sm text-destructive">{error}</p>}
    </CardContent></Card>
    <Card><CardHeader><CardTitle>隧道记录</CardTitle></CardHeader><CardContent><div className="divide-y">{items.map(x => <div key={x.id} className="py-3"><div className="flex flex-wrap items-center justify-between gap-3"><div><div className="font-medium">{x.name}</div><div className="text-sm text-muted-foreground">{x.kind} · {x.target || "未指定"} · {x.listen_addr || "未监听"} · {x.approved ? "已批准" : "待批准"}</div></div><div className="flex flex-wrap gap-2"><span className="rounded bg-muted px-2 py-1 text-xs">{x.status}</span>{x.status === "dry_run" || x.status === "planned" ? <><Button size="sm" variant="outline" onClick={() => action(x.id, () => api.tunnelUpdate(x.id, { status: "dry_run", approved: true }))}>批准</Button>{(x.kind === "tcp_forward" || x.kind === "chisel" || x.kind === "suo5") && <Button size="sm" onClick={() => action(x.id, () => api.tunnelDeploy(x.id))}>部署</Button>}</> : null}{x.status === "active" && <><Button size="sm" variant="outline" onClick={() => action(x.id, () => api.tunnelHealth(x.id))}>健康检查</Button><Button size="sm" variant="destructive" onClick={() => action(x.id, () => api.tunnelTeardown(x.id))}>拆除</Button></>}</div></div>{x.command && <pre className="mt-2 overflow-auto rounded bg-muted p-2 text-xs">启动命令：{x.command}</pre>}{x.dry_run_script && <pre className="mt-2 max-h-48 overflow-auto rounded bg-muted p-2 text-xs">{x.dry_run_script}</pre>}</div>)}{items.length===0&&<div className="py-8 text-center text-muted-foreground">暂无隧道记录</div>}</div></CardContent></Card>
  </div>;
}
