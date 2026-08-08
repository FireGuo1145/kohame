import { useState, type FormEvent } from "react"
import { Building2, Check, Copy, FolderGit2, HeartHandshake, Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Empty, Field } from "@/components/forge-ui"
import { api, when } from "@/lib/forge-api"
import type { Organization, Repository, SiteSettings, User } from "@/lib/forge-types"

export default function HomePage({
  site,
  user,
  repos,
  onRepos,
  onOpen,
  onOrganization,
}: {
  site: SiteSettings
  user: User | null
  repos: Repository[]
  onRepos: (repos: Repository[]) => void
  onOpen: (name: string) => void
  onOrganization: (name: string) => void
}) {
  const [scope, setScope] = useState(user?.username ?? "")
  const [name, setName] = useState("")
  const [organizationName, setOrganizationName] = useState("")
  const [message, setMessage] = useState("")
  const [copied, setCopied] = useState("")
  const create = async (event: FormEvent) => {
    event.preventDefault()
    try {
      const repo = await api<Repository>("/api/repos", {
        method: "POST",
        body: JSON.stringify({ scope, name }),
      })
      onRepos([repo, ...repos])
      setName("")
      onOpen(repo.fullName)
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "Could not create repository."
      )
    }
  }
  const createOrganization = async (event: FormEvent) => {
    event.preventDefault()
    try {
      const organization = await api<Organization>("/api/organizations", {
        method: "POST",
        body: JSON.stringify({ name: organizationName }),
      })
      onOrganization(organization.name)
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "Could not create organization.")
    }
  }
  return (
    <>
      <section className="border-b border-zinc-200 bg-[radial-gradient(circle_at_20%_0%,#e4f4e8,transparent_34%),#fafaf8] py-14 dark:border-zinc-800 dark:bg-zinc-950">
        <div className="mx-auto max-w-6xl px-5">
          <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300">
            <HeartHandshake className="size-3" />
            你的代码协作平台
          </p>
          <h1 className="text-4xl font-semibold tracking-[-.04em] sm:text-5xl">
            {site.description}
          </h1>
          <p className="mt-4 max-w-xl text-zinc-600 dark:text-zinc-400">
            托管 Git 仓库、用 Issue 规划工作、通过 Pull Request 审查改动，并在同一处发布版本。
          </p>
        </div>
      </section>
      <div className="mx-auto grid max-w-6xl gap-8 px-5 py-9 lg:grid-cols-[1fr_340px]">
        <section>
          <div className="mb-4 flex items-end justify-between">
            <div>
              <h2 className="text-xl font-semibold">仓库</h2>
              <p className="text-sm text-zinc-500">
                已托管 {repos.length} 个项目
              </p>
            </div>
          </div>
          <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
            {repos.length ? (
              repos.map((repo) => (
                <div key={repo.fullName} className="flex items-center gap-3 border-b border-zinc-100 px-5 py-4 last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800">
                  <button onClick={() => onOpen(repo.fullName)} className="flex min-w-0 flex-1 items-center gap-3 text-left">
                    <span className="grid size-9 place-items-center rounded-xl bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                      <FolderGit2 className="size-4" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <strong className="block truncate text-sm">
                        {repo.fullName}
                      </strong>
                      <small className="text-zinc-500">
                        更新于 {when(repo.updatedAt)}
                      </small>
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
        <aside className="h-fit rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <Plus className="mb-4 size-5" />
          <h2 className="font-semibold">创建仓库</h2>
          {user ? (
            <form className="mt-4 space-y-3" onSubmit={create}>
              <Field
                label="所有者 / 命名空间"
                value={scope}
                onChange={setScope}
                placeholder={user.username}
              />
              <Field
                label="仓库名称"
                value={name}
                onChange={setName}
                placeholder="my-project"
              />
              {message && <p className="text-sm text-red-600">{message}</p>}
              <Button type="submit" className="w-full">
                <Plus />
                创建仓库
              </Button>
            </form>
          ) : (
            <p className="mt-2 text-sm leading-6 text-zinc-500">
              登录或注册后即可创建仓库并与你的团队协作。
            </p>
          )}
          {user && (
            <form className="mt-5 border-t border-zinc-100 pt-5 dark:border-zinc-800" onSubmit={createOrganization}>
              <div className="mb-3 flex items-center gap-2"><Building2 className="size-4" /><h3 className="text-sm font-semibold">创建组织</h3></div>
              <Field label="组织名称" value={organizationName} onChange={setOrganizationName} placeholder="my-team" />
              <Button type="submit" variant="outline" className="mt-3 w-full"><Building2 /> 创建组织主页</Button>
            </form>
          )}
        </aside>
      </div>
    </>
  )
}
