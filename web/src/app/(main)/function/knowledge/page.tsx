"use client";
import * as React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import type { KnowledgeEntry } from "@/lib/types";

export default function KnowledgePage() {
  const [items, setItems] = React.useState<KnowledgeEntry[]>([]); const [q, setQ] = React.useState("");
  const load = React.useCallback(() => api.knowledge(q ? `q=${encodeURIComponent(q)}` : "").then(r => setItems(r.items ?? [])), [q]);
  React.useEffect(() => { load().catch(() => setItems([])); }, [load]);
  return <div className="p-4 md:p-6"><Card><CardHeader><CardTitle>知识库</CardTitle><div className="flex gap-2"><Input placeholder="搜索标题、标签、CVE" value={q} onChange={e => setQ(e.target.value)} onKeyDown={e => e.key === "Enter" && load()} /><Button onClick={load}>搜索</Button></div></CardHeader><CardContent><div className="divide-y">{items.map(x => <div key={x.id} className="py-3"><div className="font-medium">{x.title}</div><div className="text-sm text-muted-foreground">{x.entry_type} · {x.status ?? "draft"} · {(x.tags ?? []).join(", ")}</div><p className="mt-1 text-sm">{x.summary || x.content || ""}</p></div>)}{items.length === 0 && <div className="py-8 text-center text-muted-foreground">暂无条目</div>}</div></CardContent></Card></div>;
}
