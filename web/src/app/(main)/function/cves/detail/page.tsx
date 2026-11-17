"use client";
import * as React from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { CVERecord } from "@/lib/types";

function pretty(value: unknown) { if (value == null) return "暂无"; if (typeof value === "string") return value; try { return JSON.stringify(value, null, 2); } catch { return String(value); } }
export default function CVEDetailPage() {
  const search = useSearchParams(); const id = search.get("id") || ""; const [item, setItem] = React.useState<CVERecord | null>(null); const [error, setError] = React.useState("");
  React.useEffect(() => { if (id) api.cve(id).then(setItem).catch(e => setError(e instanceof Error ? e.message : "读取 CVE 详情失败")); }, [id]);
  if (!id) return <div className="p-6 text-destructive">缺少 CVE 编号</div>; if (error) return <div className="p-6 text-destructive">{error}</div>; if (!item) return <div className="p-6 text-muted-foreground">正在读取 CVE 详情…</div>;
  return <div className="space-y-4 p-4 md:p-6"><Link href="/function/cves" className="text-sm text-muted-foreground hover:underline">← 返回 CVE 语料库</Link><Card><CardHeader><CardTitle>{item.cve_id}</CardTitle><div className="flex flex-wrap gap-3 text-sm text-muted-foreground"><span>严重度：{item.severity || "未标注"}</span><span>CVSS：{typeof item.cvss_score === "number" ? item.cvss_score.toFixed(1) : "未标注"}</span><span>来源：{item.source || "local"}</span><span>发布时间：{item.published_at ? new Date(item.published_at).toLocaleString("zh-CN") : "未知"}</span><span>修改时间：{item.modified_at ? new Date(item.modified_at).toLocaleString("zh-CN") : "未知"}</span></div></CardHeader><CardContent className="space-y-5"><section><h2 className="font-medium">漏洞描述</h2><p className="mt-1 whitespace-pre-wrap text-sm leading-6">{item.description || "暂无描述"}</p></section><section><h2 className="font-medium">CVSS 向量</h2><p className="mt-1 break-all font-mono text-sm">{item.cvss_vector || "暂无"}</p></section><section><h2 className="font-medium">CWE</h2><p className="mt-1 text-sm">{(item.cwes ?? []).join("、") || "暂无"}</p></section><section><h2 className="font-medium">受影响配置</h2><pre className="mt-1 max-h-96 overflow-auto rounded-md bg-muted p-3 text-xs">{pretty(item.affected)}</pre></section><section><h2 className="font-medium">参考链接</h2><div className="mt-1 space-y-1 text-sm">{(item.references ?? []).map((ref, i) => <a className="block break-all text-primary hover:underline" href={String(ref)} target="_blank" rel="noreferrer" key={`${ref}-${i}`}>{String(ref)}</a>)}{(item.references ?? []).length === 0 && <span className="text-muted-foreground">暂无</span>}</div></section><details className="rounded-md border p-3"><summary className="cursor-pointer text-sm font-medium">原始 NVD 条目（JSON）</summary><pre className="mt-3 max-h-[32rem] overflow-auto rounded-md bg-muted p-3 text-xs">{pretty(item.raw)}</pre></details></CardContent></Card></div>;
}
