import { FolderGit2, GitPullRequest, Star } from "lucide-react"
import { RepoCards, Stat } from "@/components/forge-ui"
import type { Repository, User } from "@/lib/forge-types"

export default function WorkPage({ user, repos, onOpen }: { user: User | null; repos: Repository[]; onOpen: (name: string) => void }) {
  const mine = user ? repos.filter((repo) => repo.scope === user.username) : []
  return <div className="mx-auto max-w-7xl px-5 py-9"><div className="mb-8 flex items-end justify-between"><div><p className="text-sm font-medium text-emerald-700">你的工作主页</p><h1 className="text-3xl font-semibold tracking-tight">{user ? `欢迎回来，${user.username}` : "登录后查看工作台"}</h1><p className="mt-2 text-sm text-zinc-500">跟进自己拥有的项目、通知和组织协作。</p></div></div>{user ? <div className="grid gap-5 md:grid-cols-3"><Stat icon={<FolderGit2 />} label="个人仓库" value={String(mine.length)} /><Stat icon={<Star />} label="获得 Star" value={String(mine.reduce((total, repo) => total + repo.stars, 0))} /><Stat icon={<GitPullRequest />} label="可继续协作" value="议题与拉取请求" /></div> : null}<section className="mt-8"><h2 className="mb-3 font-semibold">最近仓库</h2><RepoCards repos={user ? mine : repos.slice(0, 6)} onOpen={onOpen} /></section></div>
}

