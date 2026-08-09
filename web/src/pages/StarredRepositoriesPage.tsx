import { useEffect, useState } from "react"
import { Star } from "lucide-react"
import { Loading, PageMessage, RepoCards } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { Repository } from "@/lib/forge-types"

export default function StarredRepositoriesPage({ onOpen }: { onOpen: (name: string) => void }) {
  const [repos, setRepos] = useState<Repository[] | null>(null)
  const [message, setMessage] = useState("")
  useEffect(() => { void api<Repository[]>("/api/user/starred-repos").then(setRepos).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取收藏仓库。")) }, [])
  if (message) return <PageMessage title="收藏" message={message} />
  if (!repos) return <Loading />
  return <div className="mx-auto max-w-7xl px-5 py-9"><div className="mb-6 flex items-center gap-3"><Star className="size-6 text-amber-500" /><div><h1 className="text-2xl font-semibold">收藏的仓库</h1><p className="mt-1 text-sm text-zinc-500">你收藏的所有项目。</p></div></div><RepoCards repos={repos} onOpen={onOpen} /></div>
}
