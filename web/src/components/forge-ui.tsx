import type { ReactNode } from "react"
import { FolderGit2, Star } from "lucide-react"
import type { Repository } from "@/lib/forge-types"
import { when } from "@/lib/forge-api"

export function RepoCards({ repos, onOpen }: { repos: Repository[]; onOpen: (name: string) => void }) { return <div className="grid gap-3 md:grid-cols-2">{repos.length?repos.map(repo=><button key={repo.fullName} onClick={()=>onOpen(repo.fullName)} className="rounded-xl border border-zinc-200 bg-white p-4 text-left transition hover:border-zinc-400 hover:shadow-sm dark:border-zinc-800 dark:bg-zinc-900"><span className="flex items-center justify-between gap-3"><strong className="truncate text-sm text-sky-700 dark:text-sky-300"><FolderGit2 className="mr-1 inline size-4"/>{repo.fullName}</strong><span className="text-xs text-zinc-500"><Star className="mr-1 inline size-3"/>{repo.stars}</span></span><p className="mt-3 text-xs text-zinc-500">更新于 {when(repo.updatedAt)}{repo.forkedFrom?` · 派生自 ${repo.forkedFrom}`:""}</p></button>):<div className="col-span-full rounded-xl border border-dashed border-zinc-300 p-7 text-center text-sm text-zinc-500">暂无仓库。</div>}</div> }
export function Avatar({name,src,size="sm"}:{name:string;src?:string;size?:"sm"|"lg"}){return <span className={`${size==="lg"?"size-16 text-2xl":"size-7 text-xs"} grid shrink-0 place-items-center overflow-hidden rounded-full bg-zinc-900 font-semibold text-white dark:bg-zinc-100 dark:text-zinc-900`}>{src?<img src={src} alt="" className="size-full object-cover"/>:name.slice(0,1).toUpperCase()}</span>}
export function Stat({icon,label,value}:{icon:ReactNode;label:string;value:string}){return <div className="rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900"><span className="text-zinc-500">{icon}</span><strong className="mt-4 block text-2xl">{value}</strong><span className="text-sm text-zinc-500">{label}</span></div>}
export function Loading(){return <main className="grid min-h-[50svh] place-items-center text-sm text-zinc-500">加载中…</main>}
export function PageMessage({title,message}:{title:string;message:string}){return <div className="mx-auto max-w-3xl px-5 py-9"><h1 className="text-2xl font-semibold">{title}</h1><p className="mt-5 rounded-xl bg-red-50 p-4 text-sm text-red-700">{message}</p></div>}


export function Field({
  label,
  value,
  onChange,
  placeholder,
  type = "text",
}: {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder: string
  type?: string
}) {
  return (
    <label className="block text-sm font-medium">
      {label}
      <input
        required
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        type={type}
        className="mt-1.5 h-10 w-full rounded-xl border border-zinc-200 bg-transparent px-3 text-sm outline-none focus:border-zinc-500 dark:border-zinc-700"
      />
    </label>
  )
}
export function Badge({ state }: { state: string }) {
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${state === "open" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300" : state === "merged" ? "bg-purple-100 text-purple-700" : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"}`}
    >
      {state === "open" ? "开启" : state === "merged" ? "已合并" : state === "closed" ? "已关闭" : state}
    </span>
  )
}
export function Empty({
  icon,
  title,
  text,
}: {
  icon: ReactNode
  title: string
  text: string
}) {
  return (
    <div className="grid min-h-44 place-items-center p-6 text-center">
      <div>
        <span className="mx-auto mb-3 block text-zinc-300">{icon}</span>
        <p className="font-medium">{title}</p>
        <p className="mt-1 text-sm text-zinc-500">{text}</p>
      </div>
    </div>
  )
}

