"use client";
import * as React from "react";
import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import type { CVERecord } from "@/lib/types";

export default function CVEsPage() {
  const [items, setItems] = React.useState<CVERecord[]>([]); const [q, setQ] = React.useState(""); const [busy, setBusy] = React.useState(false); const [error, setError] = React.useState("");
  const load = React.useCallback(() => api.cves(q ? `q=${encodeURIComponent(q)}` : "").then(r => setItems(r.items ?? [])), [q]);
  React.useEffect(() => { load().catch(() => setError("读取 CVE 数据失败")); }, [load]);
  const update = (days: number, feed = false) => { setBusy(true); setError(""); api.cveUpdate({ days, keyword: q, feed }).then(load).catch(e => setError(e instanceof Error ? e.message : "更新失败")).finally(() => setBusy(false)); };
  return <div className="space-y-4 p-4 md:p-6"><Card><CardHeader><CardTitle>CVE 语料库</CardTitle><div className="flex flex-wrap gap-2"><Input placeholder="CVE-2021-44228、产品或关键词" value={q} onChange={e => setQ(e.target.value)} onKeyDown={e => e.key === "Enter" && load()} /><Button onClick={load}>搜索</Button><Button variant="outline" onClick={() => update(30)} disabled={busy}>{busy ? "更新中…" : "更新近 30 天"}</Button><Button variant="outline" onClick={() => update(3650, true)} disabled={busy}>{busy ? "导入中…" : "导入近十年"}</Button></div><p className="text-xs text-muted-foreground">点击 CVE 编号可查看完整描述、CVSS、CWE、受影响配置和参考链接。</p>{error && <p role="alert" className="text-sm text-destructive">{error}</p>}</CardHeader><CardContent><div className="divide-y">{items.map(x => <Link href={`/function/cves/detail?id=${encodeURIComponent(x.cve_id)}`} key={x.cve_id} className="block py-3 transition-colors hover:bg-muted/40"><div className="font-medium">{x.cve_id} {x.severity && <span className="text-sm text-muted-foreground">· {x.severity}</span>} {typeof x.cvss_score === "number" && <span className="text-sm text-muted-foreground">· CVSS {x.cvss_score.toFixed(1)}</span>}</div><p className="text-sm line-clamp-3">{x.description || "暂无描述"}</p><div className="mt-1 text-xs text-muted-foreground">{(x.cwes ?? []).join("、") || "未标注 CWE"} · 查看详情 →</div></Link>)}{items.length === 0 && <div className="py-8 text-center text-muted-foreground">暂无 CVE 数据</div>}</div></CardContent></Card></div>;
}
