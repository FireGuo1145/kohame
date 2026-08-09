import { useEffect, useState } from "react"
import { useLocation, useNavigate, useParams } from "react-router-dom"
import { Building2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Avatar,
  Empty,
  Loading,
  PageMessage,
  RepoCards,
} from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type {
  Organization,
  OrganizationMember,
  Repository,
  User,
} from "@/lib/forge-types"
import ProfileRoutePage from "@/pages/ProfileRoutePage"

const tabs = [
  { id: "overview", label: "概览" },
  { id: "repositories", label: "仓库" },
  { id: "projects", label: "项目" },
  { id: "packages", label: "软件包" },
  { id: "teams", label: "团队" },
  { id: "members", label: "成员" },
  { id: "insights", label: "洞察" },
  { id: "settings", label: "设置" },
] as const

type OrganizationTab = (typeof tabs)[number]["id"]

function getTab(
  value: string | null,
  repositoriesOnly: boolean
): OrganizationTab {
  if (repositoriesOnly) return "repositories"
  return tabs.some((tab) => tab.id === value)
    ? (value as OrganizationTab)
    : "overview"
}

function OrganizationPage({
  name,
  onOpen,
  user,
  repositoriesOnly,
}: {
  name: string
  onOpen: (name: string) => void
  user: { username: string } | null
  repositoriesOnly: boolean
}) {
  const location = useLocation()
  const navigate = useNavigate()
  const [org, setOrg] = useState<Organization | null>(null)
  const [members, setMembers] = useState<OrganizationMember[]>([])
  const [repos, setRepos] = useState<Repository[]>([])
  const [message, setMessage] = useState("")
  const tab = getTab(
    new URLSearchParams(location.search).get("tab"),
    repositoriesOnly
  )

  useEffect(() => {
    void Promise.all([
      api<Organization>(`/api/organizations/${name}`),
      api<OrganizationMember[]>(`/api/organizations/${name}/members`),
      api<Repository[]>(`/api/organizations/${name}/repos`),
    ])
      .then(([organization, organizationMembers, organizationRepos]) => {
        setOrg(organization)
        setMembers(organizationMembers)
        setRepos(organizationRepos)
      })
      .catch((cause: unknown) =>
        setMessage(
          cause instanceof Error ? cause.message : "无法加载组织主页。"
        )
      )
  }, [name])

  if (message) return <PageMessage title="组织主页" message={message} />
  if (!org) return <Loading />

  const openTab = (nextTab: OrganizationTab) => {
    navigate(nextTab === "overview" ? `/${name}` : `/${name}?tab=${nextTab}`)
  }

  return (
    <div className="mx-auto max-w-7xl px-5 py-9">
      <section className="flex flex-wrap items-center gap-4 rounded-2xl border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
        <span className="grid size-16 place-items-center rounded-2xl bg-violet-100 text-violet-700 dark:bg-violet-950">
          <Building2 />
        </span>
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-semibold">{org.name}</h1>
          <p className="mt-1 text-sm text-zinc-500">
            组织主页 · {members.length} 位成员 · {org.followers} 位关注者
          </p>
        </div>
        {user && (
          <Button
            variant={org.followed ? "secondary" : "outline"}
            onPress={async () => {
              try {
                const result = await api<{
                  followed: boolean
                  followers: number
                }>(`/api/organizations/${name}/follow`, { method: "POST" })
                setOrg({ ...org, ...result })
              } catch (cause) {
                setMessage(
                  cause instanceof Error ? cause.message : "操作失败。"
                )
              }
            }}
          >
            {org.followed ? "已关注" : "关注"}
          </Button>
        )}
      </section>

      <nav
        aria-label="组织导航"
        className="-mx-5 mt-6 overflow-x-auto border-b border-zinc-200 px-5 dark:border-zinc-800"
      >
        <div className="flex min-w-max gap-1">
          {tabs.map((item) => {
            const count =
              item.id === "repositories"
                ? repos.length
                : item.id === "members"
                  ? members.length
                  : undefined
            return (
              <button
                key={item.id}
                onClick={() => openTab(item.id)}
                className={`border-b-2 px-3 py-2.5 text-sm transition-colors ${tab === item.id ? "border-emerald-600 font-medium text-zinc-950 dark:text-zinc-100" : "border-transparent text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"}`}
              >
                {item.label}
                {count !== undefined && (
                  <span className="ml-1.5 rounded-full bg-zinc-100 px-1.5 py-0.5 text-xs dark:bg-zinc-800">
                    {count}
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </nav>

      <OrganizationTabContent
        tab={tab}
        followers={org.followers}
        members={members}
        repos={repos}
        onOpen={onOpen}
        onOpenMember={(username) => navigate(`/${username}`)}
      />
    </div>
  )
}

function OrganizationTabContent({
  tab,
  followers,
  members,
  repos,
  onOpen,
  onOpenMember,
}: {
  tab: OrganizationTab
  followers: number
  members: OrganizationMember[]
  repos: Repository[]
  onOpen: (name: string) => void
  onOpenMember: (username: string) => void
}) {
  if (tab === "overview") {
    return (
      <div className="mt-8 grid gap-8 lg:grid-cols-[minmax(0,1fr)_280px]">
        <section>
          <div className="mb-3 flex items-center justify-between gap-3">
            <h2 className="font-semibold">仓库</h2>
            <span className="text-sm text-zinc-500">共 {repos.length} 个</span>
          </div>
          <RepoCards repos={repos.slice(0, 6)} onOpen={onOpen} />
        </section>
        <aside className="rounded-xl border border-zinc-200 bg-white p-5 dark:border-zinc-800 dark:bg-zinc-900">
          <h2 className="font-semibold">组织概况</h2>
          <dl className="mt-4 space-y-3 text-sm">
            <div className="flex items-center justify-between">
              <dt className="text-zinc-500">仓库</dt>
              <dd className="font-medium">{repos.length}</dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-zinc-500">成员</dt>
              <dd className="font-medium">{members.length}</dd>
            </div>
            <div className="flex items-center justify-between">
              <dt className="text-zinc-500">关注者</dt>
              <dd className="font-medium">{followers}</dd>
            </div>
          </dl>
        </aside>
      </div>
    )
  }

  if (tab === "repositories") {
    return (
      <section className="mt-8">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="font-semibold">所有仓库</h2>
          <span className="text-sm text-zinc-500">共 {repos.length} 个</span>
        </div>
        <RepoCards repos={repos} onOpen={onOpen} />
      </section>
    )
  }

  if (tab === "members") {
    return (
      <section className="mt-8 max-w-3xl">
        <div className="mb-3 flex items-center justify-between">
          <h2 className="font-semibold">成员</h2>
          <span className="text-sm text-zinc-500">共 {members.length} 位</span>
        </div>
        <div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
          {members.length ? (
            members.map((member) => (
              <button
                key={member.username}
                onClick={() => onOpenMember(member.username)}
                className="flex w-full items-center gap-3 border-b border-zinc-100 p-4 text-left last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800"
              >
                <Avatar name={member.username} />
                <span className="min-w-0 flex-1 truncate text-sm font-medium">
                  {member.username}
                </span>
                <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">
                  {member.role}
                </span>
              </button>
            ))
          ) : (
            <p className="p-8 text-center text-sm text-zinc-500">
              该组织暂无成员。
            </p>
          )}
        </div>
      </section>
    )
  }

  const labels: Record<
    Exclude<OrganizationTab, "overview" | "repositories" | "members">,
    [string, string]
  > = {
    projects: ["项目", "项目功能正在准备中。"],
    packages: ["软件包", "软件包发布与管理功能正在准备中。"],
    teams: ["团队", "团队分组功能正在准备中。"],
    insights: ["洞察", "组织活动与统计功能正在准备中。"],
    settings: ["设置", "组织设置功能正在准备中。"],
  }
  const [title, text] = labels[tab]
  return (
    <section className="mt-8 rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <Empty
        icon={<Building2 className="size-8" />}
        title={title}
        text={text}
      />
    </section>
  )
}

export default function OrganizationRoutePage({
  onOpen,
  user,
  repositoriesOnly = false,
}: {
  onOpen: (name: string) => void
  user: User | null
  repositoriesOnly?: boolean
}) {
  const { name: routeName, username } = useParams()
  const name = routeName || username
  const [organization, setOrganization] = useState<boolean | null>(null)

  useEffect(() => {
    if (!name) return
    void api<Organization>(`/api/organizations/${name}`)
      .then(() => setOrganization(true))
      .catch(() => setOrganization(false))
  }, [name])

  if (!name || organization === null) return <Loading />
  return organization ? (
    <OrganizationPage
      name={name}
      onOpen={onOpen}
      user={user}
      repositoriesOnly={repositoriesOnly}
    />
  ) : (
    <ProfileRoutePage user={user} onOpen={onOpen} />
  )
}
