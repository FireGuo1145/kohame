import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Building2, UserRound } from "lucide-react"
import { Loading, PageMessage } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { FollowTarget, Organization } from "@/lib/forge-types"

export default function FollowListRoutePage({ kind }: { kind: "followers" | "following" }) {
  const { name } = useParams()
  const navigate = useNavigate()
  const [items, setItems] = useState<FollowTarget[]>([])
  const [message, setMessage] = useState("")
  const [isOrganization, setIsOrganization] = useState<boolean | null>(null)
  useEffect(() => {
    if (!name) return
    void api<Organization>(`/api/organizations/${name}`).then(() => setIsOrganization(true)).catch(() => setIsOrganization(false))
  }, [name])
  useEffect(() => {
    if (!name) return
    if (isOrganization === null) return
    const endpoint = isOrganization ? `/api/organizations/${name}/followers` : `/api/users/${name}/${kind}`
    if (isOrganization && kind === "following") { setItems([]); return }
    void api<FollowTarget[]>(endpoint).then(setItems).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取关注列表。"))
  }, [name, kind, isOrganization])
  if (!name) return null
  if (message) return <PageMessage title="关注" message={message} />
  if (isOrganization === null) return <Loading />
  return <div className="mx-auto max-w-3xl px-5 py-9"><button onClick={() => navigate(`/${name}`)} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />返回主页</button><h1 className="text-2xl font-semibold">{name} 的{kind === "followers" ? "关注者" : "正在关注"}</h1><div className="mt-6 overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">{items.length ? items.map((item) => <button key={`${item.type}-${item.name}`} onClick={() => navigate(`/${item.name}`)} className="flex w-full items-center gap-3 border-b border-zinc-100 px-5 py-4 text-left text-sm last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800">{item.type === "organization" ? <Building2 className="size-5 text-violet-600" /> : <UserRound className="size-5 text-emerald-600" />}<span className="font-medium">{item.name}</span><span className="text-xs text-zinc-500">{item.type === "organization" ? "组织" : "用户"}</span></button>) : <p className="px-5 py-10 text-center text-sm text-zinc-500">暂无{kind === "followers" ? "关注者" : "关注对象"}。</p>}</div></div>
}
