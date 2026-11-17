"use client";
import * as React from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { Tunnel } from "@/lib/types";
import type { IntranetSession, CredentialRef } from "@/lib/types";
import { Button } from "@/components/ui/button";

const modules = [
  ["会话管理", "登记受控 WebShell / 反弹 Shell 会话，保存语言、主机和凭证引用；明文凭证不进入平台数据库。"],
  ["session_recon", "通过已批准的会话执行只读 ip、route、邻居和连接信息采集，结果回写资产图。"],
  ["多层隧道", "按任务维护 SOCKS / port-forward 记录；内置 TCP 转发和已配置的 chisel、suo5 适配器可走真实部署、健康检查和拆除。"],
  ["凭据库", "记录 credential_ref、来源主机、用途和复用范围，凭证明文留在受控证据存储。"],
  ["内网拓扑", "按任务范围展示主机、网段、会话和隧道关系，避免跨任务资产串线。"],
];

export default function IntranetPage() {
  const [tunnels, setTunnels] = React.useState<Tunnel[]>([]);
  const [sessions, setSessions] = React.useState<IntranetSession[]>([]);
  const [footholds, setFootholds] = React.useState<{id:number; kind:string; url:string; lang:string; status:string; last_beat?:string}[]>([]);
  const [credentials, setCredentials] = React.useState<CredentialRef[]>([]);
  const [error, setError] = React.useState("");
  const [sessionForm, setSessionForm] = React.useState({name:"",kind:"webshell",target:"",language:"PHP",endpoint:"",credential_ref:""});
  const [credentialForm, setCredentialForm] = React.useState({credential_ref:"",kind:"token",source_host:"",scope:"",status:"unverified",secret_hint:""});
  const load = React.useCallback(() => Promise.all([api.tunnels(), api.intranetSessions(), api.credentials(), api.footholds()]).then(([t, s, c, f]) => { setTunnels(t.items ?? []); setSessions(s.items ?? []); setCredentials(c.items ?? []); setFootholds(f.items ?? []); }), []);
  React.useEffect(() => { load().catch(() => { setError("读取内网数据失败"); }); }, [load]);
  const createSession = async (e: React.FormEvent) => { e.preventDefault(); setError(""); try { await api.intranetSessionCreate(sessionForm); setSessionForm({...sessionForm,name:"",target:"",endpoint:"",credential_ref:""}); await load(); } catch { setError("会话登记失败，请检查必填字段和服务状态"); } };
  const createCredential = async (e: React.FormEvent) => { e.preventDefault(); setError(""); try { await api.credentialCreate(credentialForm); setCredentialForm({...credentialForm,credential_ref:"",source_host:"",scope:"",secret_hint:""}); await load(); } catch { setError("凭据引用登记失败"); } };
  const updateStatus = async (id:number,status:string) => { try { await api.intranetSessionUpdate(id,{status}); await load(); } catch { setError("会话状态更新失败"); } };
  const field = (label:string, key:string, value:string, set:(v:string)=>void, placeholder="") => <label className="grid gap-1 text-sm">{label}<input className="h-9 rounded-md border bg-background px-3" value={value} placeholder={placeholder} onChange={e=>set(e.target.value)} /></label>;
  return <div className="space-y-4 p-4 md:p-6">
    <Card><CardHeader><CardTitle>内网作战</CardTitle></CardHeader><CardContent><p className="text-sm text-muted-foreground">内网阶段必须绑定任务范围，并通过审批后才允许会话执行、凭据复用和隧道上线。</p></CardContent></Card>
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{modules.map(([title, desc]) => <Card key={title}><CardHeader><CardTitle className="text-base">{title}</CardTitle></CardHeader><CardContent><p className="text-sm text-muted-foreground">{desc}</p></CardContent></Card>)}</div>
    <div className="grid gap-4 md:grid-cols-4"><Card><CardHeader><CardTitle className="text-base">会话记录</CardTitle></CardHeader><CardContent><b>{sessions.length}</b><p className="text-sm text-muted-foreground">台账记录</p></CardContent></Card><Card><CardHeader><CardTitle className="text-base">真实立足点</CardTitle></CardHeader><CardContent><b>{footholds.length}</b><p className="text-sm text-muted-foreground">已接入 Registry 的会话</p></CardContent></Card><Card><CardHeader><CardTitle className="text-base">凭据引用</CardTitle></CardHeader><CardContent><b>{credentials.length}</b><p className="text-sm text-muted-foreground">只保存引用，不保存明文</p></CardContent></Card><Card><CardHeader><CardTitle className="text-base">隧道</CardTitle></CardHeader><CardContent><b>{tunnels.length}</b><p className="text-sm text-muted-foreground">计划、运行和关闭记录</p></CardContent></Card></div>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <div className="grid gap-4 xl:grid-cols-2">
      <Card><CardHeader><CardTitle>登记会话</CardTitle></CardHeader><CardContent><form onSubmit={createSession} className="grid gap-3 sm:grid-cols-2">{field("名称 *","name",sessionForm.name,v=>setSessionForm({...sessionForm,name:v}))}{field("类型","kind",sessionForm.kind,v=>setSessionForm({...sessionForm,kind:v}))}{field("目标主机","target",sessionForm.target,v=>setSessionForm({...sessionForm,target:v}))}{field("运行语言","language",sessionForm.language,v=>setSessionForm({...sessionForm,language:v}))}{field("受控端点","endpoint",sessionForm.endpoint,v=>setSessionForm({...sessionForm,endpoint:v}))}{field("凭据引用","credential_ref",sessionForm.credential_ref,v=>setSessionForm({...sessionForm,credential_ref:v}))}<div className="sm:col-span-2"><Button type="submit" disabled={!sessionForm.name.trim()}>保存会话记录</Button></div></form></CardContent></Card>
      <Card><CardHeader><CardTitle>登记凭据引用</CardTitle></CardHeader><CardContent><form onSubmit={createCredential} className="grid gap-3 sm:grid-cols-2">{field("凭据引用 *","credential_ref",credentialForm.credential_ref,v=>setCredentialForm({...credentialForm,credential_ref:v}))}{field("凭据类型","kind",credentialForm.kind,v=>setCredentialForm({...credentialForm,kind:v}))}{field("来源主机","source_host",credentialForm.source_host,v=>setCredentialForm({...credentialForm,source_host:v}))}{field("适用范围","scope",credentialForm.scope,v=>setCredentialForm({...credentialForm,scope:v}))}{field("状态","status",credentialForm.status,v=>setCredentialForm({...credentialForm,status:v}))}{field("脱敏提示（禁止填明文）","secret_hint",credentialForm.secret_hint,v=>setCredentialForm({...credentialForm,secret_hint:v}))}<div className="sm:col-span-2"><Button type="submit" disabled={!credentialForm.credential_ref.trim()}>保存凭据引用</Button></div></form></CardContent></Card>
    </div>
    <Card><CardHeader><CardTitle>会话清单</CardTitle></CardHeader><CardContent>{sessions.length===0?<p className="text-sm text-muted-foreground">暂无登记会话</p>:<div className="divide-y">{sessions.map(x=><div key={x.id} className="flex flex-wrap items-center justify-between gap-2 py-3 text-sm"><span>{x.name} · {x.kind} · {x.target||"未指定目标"} · {x.status}</span><span className="flex gap-2"><Button size="sm" variant="outline" onClick={()=>updateStatus(x.id,"active")}>标记活动</Button><Button size="sm" variant="outline" onClick={()=>updateStatus(x.id,"closed")}>关闭记录</Button></span></div>)}</div>}</CardContent></Card>
    <Card><CardHeader><CardTitle>真实立足点会话</CardTitle></CardHeader><CardContent>{footholds.length===0?<p className="text-sm text-muted-foreground">暂无已接入的 WebShell / 反弹 Shell。登记成功并通过探活后会显示在这里。</p>:<div className="divide-y">{footholds.map(x=><div key={x.id} className="flex flex-wrap justify-between gap-2 py-3 text-sm"><span>#{x.id} · {x.kind} · {x.lang || "未知语言"} · {x.url || "无 URL"}</span><span className={x.status === "alive" ? "text-green-600" : "text-muted-foreground"}>{x.status}{x.last_beat ? ` · 最近 ${new Date(x.last_beat).toLocaleString()}` : ""}</span></div>)}</div>}</CardContent></Card>
    <Card><CardHeader><CardTitle>当前隧道状态</CardTitle></CardHeader><CardContent>{tunnels.length === 0 ? <p className="text-sm text-muted-foreground">暂无隧道记录；请先在隧道表创建 planned 或 dry-run 记录。</p> : <div className="divide-y">{tunnels.map((x) => <div className="flex justify-between py-2 text-sm" key={x.id}><span>{x.name} · {x.kind}</span><span>{x.status}</span></div>)}</div>}</CardContent></Card>
  </div>;
}
