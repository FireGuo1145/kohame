import { useState } from "react"
import { ArrowUpRight, Check, Copy, FolderGit2, GitPullRequest, Sparkles } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Empty } from "@/components/forge-ui"
import { when } from "@/lib/forge-api"
import type { Repository, SiteSettings } from "@/lib/forge-types"

export default function HomePage({
  site,
  repos,
  onOpen,
}: {
  site: SiteSettings
  repos: Repository[]
  onOpen: (name: string) => void
}) {
  const siteDescription = site.description === "Self-hosted Git, kept simple" ? "简洁自托管的 Git 代码协作平台" : site.description
  const [copied, setCopied] = useState("")
  return (
    <div className="mx-auto max-w-7xl px-4 py-7 sm:px-6 lg:px-8">
      <section className="grid gap-6 border-b border-zinc-200 pb-7 dark:border-zinc-800 lg:grid-cols-[minmax(0,1fr)_280px]">
        <div>
          <p className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.14em] text-emerald-700 dark:text-emerald-300"><span className="size-1.5 rounded-full bg-emerald-500" />代码协作空间</p>
          <h1 className="text-3xl font-semibold tracking-tight sm:text-4xl">{siteDescription}</h1>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-zinc-500">所有仓库、议题与发布流程汇聚在一个安静、清晰的工作区里。</p>
        </div>
        <div className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-zinc-200 bg-zinc-200 text-sm dark:border-zinc-800 dark:bg-zinc-800">
          <div className="bg-white p-4 dark:bg-zinc-900"><span className="text-xs text-zinc-500">项目</span><strong className="mt-1 block text-2xl">{repos.length}</strong></div>
          <div className="bg-white p-4 dark:bg-zinc-900"><span className="text-xs text-zinc-500">协作</span><strong className="mt-1 block text-2xl">Git</strong></div>
          <div className="col-span-2 flex items-center gap-2 bg-white px-4 py-3 text-xs text-zinc-500 dark:bg-zinc-900"><Sparkles className="size-3.5 text-emerald-600" />从一个仓库开始构建。</div>
        </div>
      </section>
      <div className="py-8">
        <section>
          <div className="mb-4 flex items-end justify-between gap-4">
            <div><h2 className="text-base font-semibold">仓库</h2><p className="mt-1 text-sm text-zinc-500">已托管 {repos.length} 个项目</p></div>
            <span className="hidden items-center gap-1 text-xs text-zinc-400 sm:flex">按最后更新排序 <ArrowUpRight className="size-3" /></span>
          </div>
          <div className="overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
            {repos.length ? (
              repos.map((repo) => (
                <div key={repo.fullName} className="flex items-center gap-3 border-b border-zinc-100 px-4 py-3.5 last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800">
                  <button onClick={() => onOpen(repo.fullName)} className="flex min-w-0 flex-1 items-center gap-3 text-left">
                    <span className="grid size-9 place-items-center rounded-lg bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                      <FolderGit2 className="size-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <strong className="block truncate text-sm">
                        {repo.fullName}
                      </strong>
                      <small className="flex items-center gap-1 text-zinc-500"><GitPullRequest className="size-3" />更新于 {when(repo.updatedAt)}</small>
                    </span>
                  </button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onPress={() => {
                      const command = `git clone ${window.location.origin}/${repo.fullName}.git`
                      void navigator.clipboard.writeText(command)
                      setCopied(repo.fullName)
                    }}
                  >
                    {copied === repo.fullName ? <Check /> : <Copy />}
                    <span className="hidden sm:inline">
                      {copied === repo.fullName ? "已复制" : "克隆"}
                    </span>
                  </Button>
                </div>
              ))
            ) : (
              <Empty
                icon={<FolderGit2 />}
                title="还没有仓库"
                text="登录后创建第一个项目。"
              />
            )}
          </div>
        </section>
      </div>
    </div>
  )
}
