import { useEffect, useState, type FormEvent } from "react"
import { FolderGit2, Search, Star } from "lucide-react"
import { Button } from "@/components/ui/button"

type Repository = { fullName: string; updatedAt: string; stars: number; forkedFrom?: string }
type Profile = { username: string; createdAt: string }
type SearchResults = { repositories: Repository[]; users: Profile[] }

async function api<T>(url: string): Promise<T> {
  const response = await fetch(url)
  if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error || "搜索失败。") }
  return response.json()
}
const when = (date: string) => new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium" }).format(new Date(date))

export function SearchPage({ initialQuery, onOpen, onProfile }: { initialQuery: string; onOpen: (name: string) => void; onProfile: (username: string) => void }) {
  const [query, setQuery] = useState(initialQuery)
  const [result, setResult] = useState<SearchResults | null>(null)
  const [message, setMessage] = useState("")
  const search = async (event?: FormEvent) => {
    event?.preventDefault()
    const value = query.trim()
    if (!value) { setResult(null); return }
    try { setResult(await api<SearchResults>(`/api/search?q=${encodeURIComponent(value)}`)); setMessage("") }
    catch (cause) { setMessage(cause instanceof Error ? cause.message : "搜索失败。") }
  }
  useEffect(() => {
    setQuery(initialQuery)
    if (!initialQuery) return
    void api<SearchResults>(`/api/search?q=${encodeURIComponent(initialQuery)}`)
      .then((value) => { setResult(value); setMessage("") })
      .catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "搜索失败。"))
  }, [initialQuery])
  return <div className="mx-auto max-w-5xl px-5 py-9"><h1 className="text-2xl font-semibold">搜索</h1><p className="mt-1 text-sm text-zinc-500">查找仓库、开发者和组织中的公开项目。</p><form className="mt-6 flex gap-2" onSubmit={search}><input autoFocus value={query} onChange={(event) => setQuery(event.target.value)} placeholder="输入仓库名或用户名" className="h-10 min-w-0 flex-1 rounded-lg border border-zinc-200 bg-white px-3 text-sm outline-none focus:border-zinc-500 dark:border-zinc-700 dark:bg-zinc-900"/><Button type="submit"><Search/>搜索</Button></form>{message && <p className="mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-700">{message}</p>}{result && <div className="mt-8 grid gap-8 lg:grid-cols-[1fr_280px]"><section><h2 className="mb-3 font-semibold">仓库 <span className="text-sm font-normal text-zinc-500">{result.repositories.length} 个结果</span></h2><div className="grid gap-3">{result.repositories.length ? result.repositories.map((repo) => <button key={repo.fullName} onClick={() => onOpen(repo.fullName)} className="rounded-xl border border-zinc-200 bg-white p-4 text-left hover:border-zinc-400 dark:border-zinc-800 dark:bg-zinc-900"><strong className="text-sm text-sky-700 dark:text-sky-300"><FolderGit2 className="mr-1 inline size-4"/>{repo.fullName}</strong><p className="mt-2 text-xs text-zinc-500">更新于 {when(repo.updatedAt)} · <Star className="inline size-3"/> {repo.stars}{repo.forkedFrom ? ` · 派生自 ${repo.forkedFrom}` : ""}</p></button>) : <p className="rounded-xl border border-dashed border-zinc-300 p-6 text-sm text-zinc-500">未找到仓库。</p>}</div></section><aside><h2 className="mb-3 font-semibold">用户 <span className="text-sm font-normal text-zinc-500">{result.users.length} 个结果</span></h2><div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">{result.users.length ? result.users.map((profile) => <button key={profile.username} onClick={() => onProfile(profile.username)} className="flex w-full items-center gap-3 border-b border-zinc-100 p-3 text-left last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800"><span className="grid size-7 place-items-center rounded-full bg-zinc-900 text-xs font-semibold text-white dark:bg-zinc-100 dark:text-zinc-900">{profile.username.slice(0, 1).toUpperCase()}</span><span><strong className="block text-sm">{profile.username}</strong><small className="text-zinc-500">加入于 {when(profile.createdAt)}</small></span></button>) : <p className="p-4 text-sm text-zinc-500">未找到用户。</p>}</div></aside></div>}</div>
}
