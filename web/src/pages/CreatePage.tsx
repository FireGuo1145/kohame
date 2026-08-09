import { useEffect, useState, type FormEvent } from "react"
import { Building2, FolderGit2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Field } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { Organization, Repository, User } from "@/lib/forge-types"

export function CreateRepositoryPage({ user, onCreated }: { user: User | null; onCreated: (name: string) => void }) {
  const [scopes, setScopes] = useState<Organization[]>([])
  const [scope, setScope] = useState("")
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [visibility, setVisibility] = useState<"private" | "public">("private")
  const [license, setLicense] = useState("")
  const [createReadme, setCreateReadme] = useState(true)
  const [message, setMessage] = useState("")
  useEffect(() => { if (!user) return; void api<Organization[]>("/api/scopes").then((items) => { setScopes(items); setScope(items[0]?.name || user.username) }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载可用空间。")) }, [user])
  if (!user) return <div className="mx-auto max-w-xl px-5 py-10"><h1 className="text-2xl font-semibold">创建仓库</h1><p className="mt-4 text-sm text-zinc-500">请先登录后再创建仓库。</p></div>
  return <div className="mx-auto max-w-xl px-5 py-10"><div className="mb-7"><FolderGit2 className="mb-3 size-7 text-emerald-600" /><h1 className="text-2xl font-semibold">创建仓库</h1><p className="mt-2 text-sm text-zinc-500">设置项目的初始可见性和起始文件。</p></div><form onSubmit={async(event:FormEvent)=>{event.preventDefault();try{const repo=await api<Repository>("/api/repos",{method:"POST",body:JSON.stringify({scope,name,description,visibility,license,createReadme})});onCreated(repo.fullName)}catch(cause){setMessage(cause instanceof Error?cause.message:"无法创建仓库。")}}} className="space-y-5 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"><label className="block text-sm font-medium">归属空间<select value={scope} onChange={(event)=>setScope(event.target.value)} className="mt-1.5 block h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700">{scopes.map((item)=><option key={item.name} value={item.name}>{item.name}</option>)}</select></label><Field label="仓库名称" value={name} onChange={setName} placeholder="my-project" /><Field label="描述（可选）" value={description} onChange={setDescription} placeholder="简述这个项目" /><fieldset><legend className="text-sm font-medium">可见性</legend><div className="mt-2 grid grid-cols-2 gap-3"><label className={`cursor-pointer rounded-lg border p-3 text-sm ${visibility === "private" ? "border-emerald-600 bg-emerald-50 dark:bg-emerald-950/30" : "border-zinc-200 dark:border-zinc-700"}`}><input className="mr-2" type="radio" checked={visibility === "private"} onChange={() => setVisibility("private")} />私有</label><label className={`cursor-pointer rounded-lg border p-3 text-sm ${visibility === "public" ? "border-emerald-600 bg-emerald-50 dark:bg-emerald-950/30" : "border-zinc-200 dark:border-zinc-700"}`}><input className="mr-2" type="radio" checked={visibility === "public"} onChange={() => setVisibility("public")} />公开</label></div></fieldset><label className="block text-sm font-medium">许可证<select value={license} onChange={(event) => setLicense(event.target.value)} className="mt-1.5 block h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700"><option value="">不添加许可证</option><option value="MIT">MIT License</option><option value="Apache-2.0">Apache-2.0</option><option value="GPL-3.0">GPL-3.0</option><option value="BSD-3-Clause">BSD-3-Clause</option></select></label><label className="flex cursor-pointer items-center gap-3 rounded-lg border border-zinc-200 p-3 text-sm dark:border-zinc-700"><input type="checkbox" checked={createReadme} onChange={(event) => setCreateReadme(event.target.checked)} />初始化 README.md</label>{message&&<p className="text-sm text-red-600">{message}</p>}<Button type="submit"><FolderGit2/>创建仓库</Button></form></div>
}

export function CreateOrganizationPage({ user, onCreated }: { user: User | null; onCreated: (name: string) => void }) {
  const [name, setName] = useState(""); const [message, setMessage] = useState("")
  if (!user) return <div className="mx-auto max-w-xl px-5 py-10"><h1 className="text-2xl font-semibold">创建组织</h1><p className="mt-4 text-sm text-zinc-500">请先登录后再创建组织。</p></div>
  return <div className="mx-auto max-w-xl px-5 py-10"><div className="mb-7"><Building2 className="mb-3 size-7 text-violet-600" /><h1 className="text-2xl font-semibold">创建组织</h1><p className="mt-2 text-sm text-zinc-500">为团队成员和仓库建立独立的协作空间。</p></div><form onSubmit={async(event:FormEvent)=>{event.preventDefault();try{const org=await api<Organization>("/api/organizations",{method:"POST",body:JSON.stringify({name})});onCreated(org.name)}catch(cause){setMessage(cause instanceof Error?cause.message:"无法创建组织。")}}} className="space-y-5 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"><Field label="组织名称" value={name} onChange={setName} placeholder="my-team" />{message&&<p className="text-sm text-red-600">{message}</p>}<Button type="submit"><Building2/>创建组织</Button></form></div>
}
