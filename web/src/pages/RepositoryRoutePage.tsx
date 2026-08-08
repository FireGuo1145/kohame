import { useEffect, useState, type FormEvent, type ReactNode } from "react"
import { useLocation, useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Check, ChevronDown, CircleDot, Code2, Copy, FileCode2, FolderGit2, GitBranch, GitCompareArrows, GitCommitHorizontal, GitPullRequest, ListFilter, MessageSquare, Plus, Search, Settings, Star, Tag, Users, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge, Empty, Field, Loading, PageMessage } from "@/components/forge-ui"
import { api, when } from "@/lib/forge-api"
import type { Blob, Commit, Contributor, GitRef, Issue, IssueComment, PullRequest, Release, Repository, RepositorySettings, TreeEntry, User } from "@/lib/forge-types"

function RepositoryView({
  name,
  user,
  onBack,
  onOpen,
}: {
  name: string
  user: User | null
  onBack: () => void
  onOpen: (name: string) => void
}) {
  const location = useLocation()
  const navigate = useNavigate()
  const subpath = location.pathname.slice(`/${name}`.length).replace(/^\//, "")
  const issueMatch = subpath.match(/^issues\/(\d+)$/)
  const issueID = issueMatch ? Number(issueMatch[1]) : 0
  const filePath = subpath.startsWith("blob/") ? decodeURIComponent(subpath.slice("blob/".length)) : ""
  const tab = subpath === "issues/new" ? "issue-new" : issueMatch ? "issue-detail" : filePath ? "file" : subpath || "code"
  const [issues, setIssues] = useState<Issue[]>([])
  const [pulls, setPulls] = useState<PullRequest[]>([])
  const [releases, setReleases] = useState<Release[]>([])
  const [contributors, setContributors] = useState<Contributor[]>([])
  const [repository, setRepository] = useState<Repository | null>(null)
  const [forkOpen, setForkOpen] = useState(false)
  const [forkName, setForkName] = useState("")
  const [message, setMessage] = useState("")
  const load = async () => {
    const [a, b, c, d, repo] = await Promise.all([
      api<Issue[]>(`/api/repos/${name}/issues`),
      api<PullRequest[]>(`/api/repos/${name}/pulls`),
      api<Release[]>(`/api/repos/${name}/releases`),
      api<Contributor[]>(`/api/repos/${name}/contributors`),
      api<Repository>(`/api/repos/${name}`),
    ])
    setIssues(a)
    setPulls(b)
    setReleases(c)
    setContributors(d)
    setRepository(repo)
  }
  useEffect(() => {
    void load().catch((cause: unknown) =>
      setMessage(
        cause instanceof Error ? cause.message : "无法加载仓库。"
      )
    )
  }, [name])
  const add = async (kind: string, value: Record<string, string>) => {
    if (!user) {
      setMessage("登录后即可参与协作。")
      return false
    }
    try {
      const target =
        kind === "issue" ? "issues" : kind === "pull" ? "pulls" : "releases"
      await api(`/api/repos/${name}/${target}`, {
        method: "POST",
        body: JSON.stringify(value),
      })
      await load()
      return true
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "无法保存内容。"
      )
      return false
    }
  }
  const remove = async (target: string, id: number) => {
    try {
      await api<void>(`/api/repos/${name}/${target}/${id}`, {
        method: "DELETE",
      })
      await load()
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "无法删除内容。"
      )
    }
  }
  const changeState = async (target: string, id: number, state: string) => {
    try {
      await api<void>(`/api/repos/${name}/${target}/${id}`, {
        method: "PATCH",
        body: JSON.stringify({ state }),
      })
      await load()
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "无法更新内容。"
      )
    }
  }
  const tabs = [
    ["code", "代码", <Code2 />],
    ["commits", "提交记录", <GitCommitHorizontal />],
    ["issues", "议题", <CircleDot />],
    ["pulls", "拉取请求", <GitPullRequest />],
    ["branches", "分支", <GitBranch />],
    ["tags", "标签", <Tag />],
    ["releases", "发布版本", <Tag />],
    ["settings", "设置", <Settings />],
  ] as const
  const openTab = (value: string) => navigate(value === "code" ? `/${name}` : `/${name}/${value}`)
  return (
    <div className="mx-auto max-w-7xl px-5 py-7">
      <button
        onClick={onBack}
        className="mb-5 text-sm text-zinc-500 hover:text-zinc-900"
      >
        ← 所有仓库
      </button>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="grid size-10 place-items-center rounded-xl bg-emerald-50 text-emerald-700 dark:bg-emerald-950">
            <FolderGit2 />
          </span>
          <div>
            <h1 className="text-xl font-semibold">{name}</h1>
            <p className="text-sm text-zinc-500">Git 仓库</p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onPress={() => setForkOpen(true)} isDisabled={!user}><GitBranch /> 派生 {repository?.forks ?? 0}</Button>
          <Button variant={repository?.starred ? "secondary" : "outline"} size="sm" isDisabled={!user} onPress={async()=>{try{const result=await api<{starred:boolean;stars:number}>(`/api/repos/${name}/star`,{method:"POST"});setRepository(repository?{...repository,...result}:repository)}catch(cause){setMessage(cause instanceof Error?cause.message:"无法更新 Star。")}}}><Star className={repository?.starred?"fill-current":""} /> 收藏 {repository?.stars ?? 0}</Button>
          <Button variant="outline" size="sm" onPress={() => void navigator.clipboard.writeText(`git clone ${window.location.origin}/${name}.git`)}><Copy /> 克隆</Button>
        </div>
      </div>
      {repository?.forkedFrom && <p className="mt-3 text-sm text-zinc-500">派生自 <span className="font-medium text-sky-700 dark:text-sky-300">{repository.forkedFrom}</span></p>}
      <div className="mt-6 flex gap-1 overflow-auto border-b border-zinc-200 dark:border-zinc-800">
        {tabs.map(([value, label, icon]) => (
          <button
            key={value}
            onClick={() => openTab(value)}
            className={`flex items-center gap-2 border-b-2 px-3 py-3 text-sm ${tab === value ? "border-zinc-900 font-medium dark:border-zinc-100" : "border-transparent text-zinc-500"}`}
          >
            {icon}
            {label}
          </button>
        ))}
      </div>
      {message && (
        <p className="mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950/50">
          {message}
        </p>
      )}
      <section className="mt-6">
        {tab === "code" && <CodeBrowser name={name} onOpenFile={(path) => navigate(`/${name}/blob/${path.split("/").map(encodeURIComponent).join("/")}`)} />}
        {tab === "file" && <FilePreview name={name} path={filePath} onBack={() => navigate(`/${name}`)} />}
        {tab === "commits" && <RepositoryList name={name} kind="commits" />}
        {tab === "branches" && <RepositoryList name={name} kind="branches" />}
        {tab === "tags" && <RepositoryList name={name} kind="tags" />}
        {tab === "issues" && (
          <WorkList
            title="Issues"
            items={issues}
            user={user}
            fields={["title", "body"]}
            button="新建议题"
            onNew={() => navigate(`/${name}/issues/new`)}
            onSubmit={(v) => add("issue", v)}
            onDelete={(id) => remove("issues", id)}
            onStateChange={(id, state) => changeState("issues", id, state)}
            render={(item) => (
              <button onClick={() => navigate(`/${name}/issues/${item.id}`)} className="block text-left hover:text-sky-700 dark:hover:text-sky-300">
                <Badge state={item.state} />
                <strong className="ml-2">
                  #{item.id} {item.title}
                </strong>
                <p className="mt-1 text-sm text-zinc-500">
                  Opened by {item.author} · {when(item.createdAt)}
                </p>
              </button>
            )}
          />
        )}
        {tab === "issue-new" && <IssueComposer user={user} onCancel={() => navigate(`/${name}/issues`)} onSubmit={async (value) => { if (await add("issue", value)) navigate(`/${name}/issues`) }} />}
        {tab === "issue-detail" && <IssueDetail name={name} issueID={issueID} user={user} onBack={() => navigate(`/${name}/issues`)} onStateChange={(state) => changeState("issues", issueID, state)} />}
        {tab === "pulls" && <PullRequestList items={pulls} user={user} onNew={() => navigate(`/${name}/compare`)} onDelete={(id) => remove("pulls", id)} onStateChange={(id, state) => changeState("pulls", id, state)} />}
        {tab === "compare" && <CompareChanges name={name} user={user} onCancel={() => navigate(`/${name}/pulls`)} onCreated={async (value) => { if (await add("pull", value)) navigate(`/${name}/pulls`) }} />}
        {tab === "releases" && (
          <WorkList
            title="发布版本"
            items={releases}
            user={user}
            fields={["tagName", "title", "notes"]}
            button="发布版本"
            onSubmit={(v) => add("release", v)}
            onDelete={(id) => remove("releases", id)}
            render={(item) => (
              <>
                <Tag className="inline size-3.5 text-emerald-600" />
                <strong className="ml-2">{item.title}</strong>
                <code className="ml-2 text-xs text-zinc-500">
                  {item.tagName}
                </code>
                <p className="mt-1 text-sm text-zinc-500">
                  Published by {item.author} · {when(item.createdAt)}
                </p>
              </>
            )}
          />
        )}
        {tab === "contributors" && (
          <div className="rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
            {contributors.length ? (
              contributors.map((item) => (
                <div
                  key={item.username}
                  className="flex items-center justify-between border-b border-zinc-100 px-5 py-4 last:border-0 dark:border-zinc-800"
                >
                  <span className="flex items-center gap-3">
                    <span className="grid size-8 place-items-center rounded-full bg-zinc-100 text-xs font-semibold dark:bg-zinc-800">
                      {item.username.slice(0, 1).toUpperCase()}
                    </span>
                    {item.username}
                  </span>
                  <span className="text-sm text-zinc-500">
                    {item.contributions} contributions
                  </span>
                </div>
              ))
            ) : (
              <Empty
                icon={<Users />}
                title="暂无贡献记录"
                text="议题、拉取请求和发布版本会显示在这里。"
              />
            )}
          </div>
        )}
        {tab === "settings" && <RepositorySettingsPanel name={name} />}
      </section>
      {forkOpen && (
        <Modal title="派生仓库" onClose={() => setForkOpen(false)}>
          <form
            className="space-y-4"
            onSubmit={async (event) => {
              event.preventDefault()
              try {
                const repo = await api<Repository>(`/api/repos/${name}/fork`, {
                  method: "POST",
                  body: JSON.stringify({ name: forkName, scope: user?.username }),
                })
                setForkOpen(false)
                onOpen(repo.fullName)
              } catch (cause) {
                setMessage(cause instanceof Error ? cause.message : "派生失败。")
              }
            }}
          >
            <p className="text-sm text-zinc-500">创建一份独立的 bare Git 仓库副本到你的个人空间。</p>
            <Field label="Fork 名称" value={forkName} onChange={setForkName} placeholder={name.split("/")[1]} />
            <Button type="submit"><GitBranch /> 创建 Fork</Button>
          </form>
        </Modal>
      )}
    </div>
  )
}

function CodeBrowser({ name, onOpenFile }: { name: string; onOpenFile: (path: string) => void }) {
  const [directory, setDirectory] = useState("")
  const [entries, setEntries] = useState<TreeEntry[]>([])
  const [branches, setBranches] = useState<GitRef[]>([])
  const [commits, setCommits] = useState<Commit[]>([])
  const [settings, setSettings] = useState<RepositorySettings | null>(null)
  const [releases, setReleases] = useState<Release[]>([])
  const [contributors, setContributors] = useState<Contributor[]>([])
  const [ref, setRef] = useState("HEAD")
  const [filter, setFilter] = useState("")
  const [copied, setCopied] = useState(false)
  const [branchMenuOpen, setBranchMenuOpen] = useState(false)
  const [codeMenuOpen, setCodeMenuOpen] = useState(false)
  const [branchQuery, setBranchQuery] = useState("")
  const [message, setMessage] = useState("")
  useEffect(() => {
    void api<TreeEntry[]>(
      `/api/repos/${name}/tree?ref=${encodeURIComponent(ref)}&path=${encodeURIComponent(directory)}`
    )
      .then((items) => {
        setEntries(items)
      })
      .catch((cause: unknown) =>
        setMessage(
            cause instanceof Error
            ? cause.message
            : "无法读取仓库目录。"
        )
      )
  }, [name, directory, ref])
  useEffect(() => {
    void Promise.all([
      api<GitRef[]>(`/api/repos/${name}/branches`),
      api<Commit[]>(`/api/repos/${name}/commits?ref=${encodeURIComponent(ref)}`),
      api<RepositorySettings>(`/api/repos/${name}/settings`),
      api<Release[]>(`/api/repos/${name}/releases`),
      api<Contributor[]>(`/api/repos/${name}/contributors`),
    ]).then(([nextBranches, nextCommits, nextSettings, nextReleases, nextContributors]) => {
      setBranches(nextBranches); setCommits(nextCommits); setSettings(nextSettings); setReleases(nextReleases); setContributors(nextContributors)
      if (ref === "HEAD" && nextBranches[0]) setRef(nextBranches.some((branch) => branch.name === nextSettings.defaultBranch) ? nextSettings.defaultBranch : nextBranches[0].name)
    }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载仓库概览。"))
  }, [name, ref])
  const openEntry = (entry: TreeEntry) => {
    if (entry.type === "tree") {
      setDirectory(entry.path)
      return
    }
    onOpenFile(entry.path)
  }
  const crumbs = directory ? directory.split("/") : []
  const visibleEntries = entries.filter((entry) => entry.name.toLowerCase().includes(filter.trim().toLowerCase()))
  const latestCommit = commits[0]
  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_250px]">
      <div className="min-w-0">
        <div className="relative mb-4 flex flex-wrap items-center gap-2">
          <div className="relative">
            <Button variant="outline" size="sm" aria-expanded={branchMenuOpen} onPress={() => { setBranchMenuOpen(!branchMenuOpen); setCodeMenuOpen(false) }}><GitBranch /> {ref === "HEAD" ? "默认分支" : ref}<ChevronDown /></Button>
            {branchMenuOpen && <div className="absolute left-0 top-10 z-20 w-[min(21rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 text-zinc-100 shadow-2xl"><div className="flex items-center justify-between border-b border-zinc-800 px-4 py-3"><strong>切换分支或标签</strong><button onClick={() => setBranchMenuOpen(false)} aria-label="关闭"><X className="size-4 text-zinc-400" /></button></div><label className="m-3 flex h-9 items-center gap-2 rounded-lg border border-zinc-700 px-2.5 text-zinc-400"><Search className="size-4" /><input value={branchQuery} onChange={(event) => setBranchQuery(event.target.value)} placeholder="查找或创建分支..." className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-zinc-500" /></label><div className="flex border-y border-zinc-800"><span className="border-b-2 border-emerald-400 px-4 py-2 text-sm font-medium">分支</span><span className="px-4 py-2 text-sm text-zinc-400">标签</span></div><div className="max-h-52 overflow-auto p-2">{branches.filter((branch) => branch.name.toLowerCase().includes(branchQuery.toLowerCase())).map((branch) => <button key={branch.name} onClick={() => { setRef(branch.name); setDirectory(""); setBranchMenuOpen(false) }} className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm hover:bg-zinc-900"><Check className={`size-4 ${ref === branch.name ? "text-emerald-400" : "text-transparent"}`} />{branch.name}{branch.name === settings?.defaultBranch && <span className="ml-auto rounded-full border border-zinc-600 px-2 py-0.5 text-xs text-zinc-300">默认</span>}</button>)}{!branches.length && <p className="px-3 py-4 text-sm text-zinc-500">暂无分支</p>}</div><button onClick={() => setBranchMenuOpen(false)} className="w-full border-t border-zinc-800 px-4 py-3 text-left text-sm text-sky-400 hover:bg-zinc-900">查看所有分支</button></div>}
          </div>
          <button className="inline-flex h-9 items-center gap-1 rounded-lg px-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-900"><GitBranch className="size-4" />{branches.length} 个分支</button>
          <button className="inline-flex h-9 items-center gap-1 rounded-lg px-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-900"><Tag className="size-4" />{releases.length} 个发布</button>
          <label className="ml-auto flex h-9 min-w-48 flex-1 items-center gap-2 rounded-lg border border-zinc-300 bg-white px-3 text-zinc-400 shadow-sm sm:max-w-72 dark:border-zinc-700 dark:bg-zinc-900"><ListFilter className="size-4" /><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="转到文件" className="w-full bg-transparent text-sm text-zinc-900 outline-none dark:text-zinc-100" /></label>
          <div className="relative"><Button variant="outline" size="sm" aria-expanded={codeMenuOpen} onPress={() => { setCodeMenuOpen(!codeMenuOpen); setBranchMenuOpen(false) }}><Code2 />{copied ? "已复制" : "代码"}<ChevronDown /></Button>{codeMenuOpen && <div className="absolute right-0 top-10 z-20 w-80 overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 p-4 text-zinc-100 shadow-2xl"><div className="mb-3 flex items-center justify-between"><strong>克隆仓库</strong><button onClick={() => setCodeMenuOpen(false)} aria-label="关闭"><X className="size-4 text-zinc-400" /></button></div><div className="mb-3 flex gap-1 rounded-lg border border-zinc-700 p-1 text-sm"><span className="rounded-md bg-zinc-700 px-3 py-1.5 font-medium">HTTPS</span><span className="px-3 py-1.5 text-zinc-400">SSH</span></div><div className="flex items-center gap-2 rounded-lg border border-zinc-700 bg-zinc-900 p-2"><code className="min-w-0 flex-1 truncate text-xs text-zinc-300">{window.location.origin}/{name}.git</code><button onClick={() => { void navigator.clipboard.writeText(`git clone ${window.location.origin}/${name}.git`); setCopied(true) }} aria-label="复制克隆地址"><Copy className="size-4 text-zinc-400" /></button></div><p className="mt-3 text-xs text-zinc-500">使用网页地址克隆此仓库。</p><div className="mt-4 space-y-1 border-t border-zinc-800 pt-3 text-sm"><button className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left hover:bg-zinc-900"><GitBranch className="size-4" />在桌面客户端中打开</button><button className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left hover:bg-zinc-900"><FolderGit2 className="size-4" />下载 ZIP</button></div></div>}</div>
        </div>
        {message && <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/50">{message}</p>}
        <div className="overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900">
          <div className="flex items-center gap-2 border-b border-zinc-200 bg-zinc-50 px-4 py-3 text-sm dark:border-zinc-800 dark:bg-zinc-950"><span className="grid size-6 place-items-center rounded-full bg-zinc-900 text-[10px] font-semibold text-white dark:bg-zinc-100 dark:text-zinc-950">{latestCommit?.author.slice(0, 1).toUpperCase() || "K"}</span><span className="min-w-0 flex-1 truncate"><strong className="font-medium">{latestCommit?.author || "暂无提交"}</strong>{latestCommit && <span className="ml-2 text-zinc-500">{latestCommit.subject}</span>}</span>{latestCommit && <span className="hidden text-xs text-zinc-500 sm:block">{latestCommit.hash} · {when(latestCommit.date)}</span>}</div>
          {directory && <div className="flex items-center gap-1 border-b border-zinc-100 px-4 py-2 text-sm text-zinc-500 dark:border-zinc-800"><button onClick={() => setDirectory("")} className="text-sky-700 hover:underline dark:text-sky-300">{name}</button>{crumbs.map((crumb, index) => <span key={`${crumb}-${index}`}><span className="mx-1 text-zinc-400">/</span><button onClick={() => setDirectory(crumbs.slice(0, index + 1).join("/"))} className="hover:underline">{crumb}</button></span>)}</div>}
          {visibleEntries.length ? visibleEntries.map((entry) => <button key={entry.path} onClick={() => openEntry(entry)} className="flex w-full items-center gap-3 border-b border-zinc-100 px-4 py-3 text-left text-sm last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800"><span className={`grid size-5 place-items-center ${entry.type === "tree" ? "text-amber-600" : "text-zinc-500"}`}>{entry.type === "tree" ? <FolderGit2 className="size-4" /> : <FileCode2 className="size-4" />}</span><span className="font-medium text-sky-700 dark:text-sky-300">{entry.name}</span><span className="ml-auto text-xs text-zinc-400">{entry.type === "tree" ? "目录" : "文件"}</span></button>) : <Empty icon={<FolderGit2 />} title="此分支暂无文件" text="推送一次提交后即可在这里浏览源代码。" />}
        </div>
      </div>
      <aside className="border-t border-zinc-200 pt-5 dark:border-zinc-800 xl:border-t-0 xl:border-l xl:pl-6 xl:pt-0"><h2 className="font-semibold">关于</h2><p className="mt-3 text-sm leading-6 text-zinc-500">{settings?.description || "暂无项目简介、网站或主题。"}</p>{settings?.topics.length ? <div className="mt-4 flex flex-wrap gap-1.5">{settings.topics.map((topic) => <span key={topic} className="rounded-full bg-sky-50 px-2 py-0.5 text-xs font-medium text-sky-700 dark:bg-sky-950 dark:text-sky-300">{topic}</span>)}</div> : null}<div className="mt-5 space-y-3 border-b border-zinc-200 pb-5 text-sm dark:border-zinc-800"><p className="flex items-center gap-2 text-zinc-600 dark:text-zinc-400"><GitCommitHorizontal className="size-4" />{commits.length} 次近期提交</p><p className="flex items-center gap-2 text-zinc-600 dark:text-zinc-400"><Users className="size-4" />{contributors.length} 位贡献者</p></div>{releases.length ? <div className="pt-5"><h3 className="font-semibold">发布版本</h3>{releases.slice(0, 2).map((release) => <div key={release.id} className="mt-3"><p className="flex items-center gap-1 text-sm font-medium text-emerald-700 dark:text-emerald-300"><Tag className="size-3.5" />{release.tagName}</p><p className="mt-1 text-xs text-zinc-500">{release.title}</p></div>)}</div> : null}</aside>
    </div>
  )
}

function FilePreview({ name, path, onBack }: { name: string; path: string; onBack: () => void }) {
  const [file, setFile] = useState<Blob | null>(null)
  const [message, setMessage] = useState("")
  useEffect(() => { void api<Blob>(`/api/repos/${name}/blob?path=${encodeURIComponent(path)}`).then(setFile).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取文件。")) }, [name, path])
  if (!file && !message) return <Loading />
  return <div className="mx-auto max-w-6xl"><button onClick={onBack} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />返回文件列表</button><div className="overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900"><header className="flex items-center gap-2 border-b border-zinc-200 bg-zinc-50 px-4 py-3 text-sm dark:border-zinc-800 dark:bg-zinc-950"><FileCode2 className="size-4 text-zinc-500" /><span className="font-medium">{path}</span></header>{message ? <p className="p-4 text-sm text-red-600">{message}</p> : file?.isText ? <pre className="max-h-[calc(100svh-15rem)] overflow-auto bg-zinc-950 p-5 text-xs leading-6 text-zinc-100"><code>{file.content}</code></pre> : <div className="grid min-h-64 place-items-center p-6 text-sm text-zinc-500">暂不支持预览二进制文件。</div>}</div></div>
}

function RepositoryList({
  name,
  kind,
}: {
  name: string
  kind: "commits" | "branches" | "tags"
}) {
  const [items, setItems] = useState<(Commit | GitRef)[]>([])
  const [message, setMessage] = useState("")
  useEffect(() => {
    void api<(Commit | GitRef)[]>(`/api/repos/${name}/${kind}`)
      .then(setItems)
      .catch((cause: unknown) =>
        setMessage(cause instanceof Error ? cause.message : "加载失败")
      )
  }, [name, kind])
  const title =
    kind === "commits" ? "提交记录" : kind === "branches" ? "分支" : "标签"
  return (
    <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <div className="border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
        <h2 className="font-semibold">{title}</h2>
      </div>
      {message && <p className="p-4 text-sm text-red-600">{message}</p>}
      {items.length ? (
        items.map((item) =>
          "subject" in item ? (
            <div
              key={item.hash}
              className="border-b border-zinc-100 px-5 py-4 last:border-0 dark:border-zinc-800"
            >
              <p className="font-medium">{item.subject}</p>
              <p className="mt-1 text-sm text-zinc-500">
                {item.author} · {when(item.date)} · <code>{item.hash}</code>
              </p>
            </div>
          ) : (
            <div
              key={item.name}
              className="flex items-center justify-between border-b border-zinc-100 px-5 py-4 last:border-0 dark:border-zinc-800"
            >
              <span className="font-medium">{item.name}</span>
              <code className="text-xs text-zinc-500">{item.hash}</code>
            </div>
          )
        )
      ) : (
        <Empty
          icon={<GitBranch />}
          title={`暂无${title}`}
          text="推送提交后将在此显示。"
        />
      )}
    </div>
  )
}

function RepositorySettingsPanel({ name }: { name: string }) {
  const [value, setValue] = useState<RepositorySettings>({
    description: "",
    visibility: "private",
    defaultBranch: "main",
    topics: [],
  })
  const [topics, setTopics] = useState("")
  const [message, setMessage] = useState("")
  useEffect(() => {
    void api<RepositorySettings>(`/api/repos/${name}/settings`)
      .then((settings) => {
        setValue(settings)
        setTopics(settings.topics.join(", "))
      })
      .catch((cause: unknown) =>
        setMessage(cause instanceof Error ? cause.message : "加载失败")
      )
  }, [name])
  const save = async (event: FormEvent) => {
    event.preventDefault()
    try {
      const next = {
        ...value,
        topics: topics
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean),
      }
      setValue(
        await api<RepositorySettings>(`/api/repos/${name}/settings`, {
          method: "PATCH",
          body: JSON.stringify(next),
        })
      )
      setMessage("仓库设置已保存。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "保存失败")
    }
  }
  return (
    <form
      onSubmit={save}
      className="max-w-2xl space-y-5 rounded-2xl border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"
    >
      <div>
        <h2 className="text-lg font-semibold">仓库设置</h2>
        <p className="mt-1 text-sm text-zinc-500">
          管理仓库简介、可见性、默认分支和 Topics。
        </p>
      </div>
      <Field
        label="仓库简介"
        value={value.description}
        onChange={(description) => setValue({ ...value, description })}
        placeholder="描述这个项目"
      />
      <Field
        label="默认分支"
        value={value.defaultBranch}
        onChange={(defaultBranch) => setValue({ ...value, defaultBranch })}
        placeholder="main"
      />
      <Field
        label="Topics（逗号分隔）"
        value={topics}
        onChange={setTopics}
        placeholder="go, git, forge"
      />
      <label className="block text-sm font-medium">
        可见性
        <select
          value={value.visibility}
          onChange={(event) =>
            setValue({
              ...value,
              visibility: event.target.value as "public" | "private",
            })
          }
          className="mt-1.5 block h-10 w-full rounded-xl border border-zinc-200 bg-transparent px-3 dark:border-zinc-700"
        >
          <option value="private">私有</option>
          <option value="public">公开</option>
        </select>
      </label>
      {message && <p className="text-sm text-emerald-700">{message}</p>}
      <Button type="submit">
        <Settings />
        保存仓库设置
      </Button>
    </form>
  )
}

function IssueComposer({
  user,
  onCancel,
  onSubmit,
}: {
  user: User | null
  onCancel: () => void
  onSubmit: (value: Record<string, string>) => Promise<void>
}) {
  const [title, setTitle] = useState("")
  const [body, setBody] = useState("")
  const [preview, setPreview] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState("")
  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_250px]">
      <form className="min-w-0" onSubmit={async (event) => {
        event.preventDefault()
        if (!user) { setMessage("请先登录再创建议题。 "); return }
        setSaving(true); setMessage("")
        try { await onSubmit({ title, body }) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法创建议题。") } finally { setSaving(false) }
      }}>
        <h1 className="mb-5 text-2xl font-semibold">新建议题</h1>
        <input autoFocus required value={title} onChange={(event) => setTitle(event.target.value)} placeholder="标题" className="h-11 w-full rounded-lg border border-zinc-300 bg-transparent px-3 text-sm shadow-sm outline-none focus:border-sky-600 focus:ring-2 focus:ring-sky-100 dark:border-zinc-700 dark:focus:ring-sky-950" />
        <div className="mt-4 overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900">
          <div className="flex items-center gap-4 border-b border-zinc-200 px-4 dark:border-zinc-800">
            <button type="button" onClick={() => setPreview(false)} className={`border-b-2 py-3 text-sm ${!preview ? "border-sky-600 font-medium" : "border-transparent text-zinc-500"}`}>编辑</button>
            <button type="button" onClick={() => setPreview(true)} className={`border-b-2 py-3 text-sm ${preview ? "border-sky-600 font-medium" : "border-transparent text-zinc-500"}`}>预览</button>
          </div>
          {preview ? <div className="min-h-72 whitespace-pre-wrap p-4 text-sm leading-6 text-zinc-700 dark:text-zinc-300">{body || <span className="text-zinc-400">暂无可预览内容</span>}</div> : <textarea required value={body} onChange={(event) => setBody(event.target.value)} placeholder="在这里输入描述..." className="min-h-72 w-full resize-y bg-transparent p-4 text-sm leading-6 outline-none" />}
        </div>
        {message && <p className="mt-3 text-sm text-red-600">{message}</p>}
        <div className="mt-5 flex justify-end gap-3"><Button type="button" variant="outline" onPress={onCancel}>取消</Button><Button type="submit" isDisabled={saving || !user}>{saving ? "正在创建..." : "创建议题"}</Button></div>
      </form>
      <aside className="divide-y divide-zinc-200 border-t border-zinc-200 text-sm dark:divide-zinc-800 dark:border-zinc-800 xl:border-t-0 xl:border-l xl:pl-6">
        <IssueMeta label="受理人" value="未分配" />
        <IssueMeta label="标签" value="暂无标签" />
        <IssueMeta label="项目" value="暂无项目" />
        <IssueMeta label="里程碑" value="暂无里程碑" />
      </aside>
    </div>
  )
}

function IssueMeta({ label, value }: { label: string; value: string }) {
  return <div className="py-5"><p className="font-medium">{label}</p><p className="mt-2 text-zinc-500">{value}</p></div>
}

function IssueDetail({ name, issueID, user, onBack, onStateChange }: { name: string; issueID: number; user: User | null; onBack: () => void; onStateChange: (state: string) => void }) {
  const [issue, setIssue] = useState<Issue | null>(null)
  const [comments, setComments] = useState<IssueComment[]>([])
  const [body, setBody] = useState("")
  const [message, setMessage] = useState("")
  const [saving, setSaving] = useState(false)
  const load = async () => {
    const [nextIssue, nextComments] = await Promise.all([
      api<Issue>(`/api/repos/${name}/issues/${issueID}`),
      api<IssueComment[]>(`/api/repos/${name}/issues/${issueID}/comments`),
    ])
    setIssue(nextIssue)
    setComments(nextComments)
  }
  useEffect(() => { void load().catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载议题。")) }, [name, issueID])
  if (!issue && !message) return <Loading />
  if (!issue) return <PageMessage title="议题" message={message} />
  const updateState = (state: string) => { onStateChange(state); setIssue({ ...issue, state }) }
  return <div className="mx-auto max-w-5xl">
    <button onClick={onBack} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />所有议题</button>
    <div className="border-b border-zinc-200 pb-5 dark:border-zinc-800"><h1 className="text-2xl font-semibold leading-tight">{issue.title} <span className="font-normal text-zinc-400">#{issue.id}</span></h1><div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-zinc-500"><Badge state={issue.state} /><span><strong className="font-medium text-zinc-700 dark:text-zinc-300">{issue.author}</strong> opened this issue {when(issue.createdAt)}</span></div></div>
    <div className="grid gap-6 py-6 lg:grid-cols-[minmax(0,1fr)_210px]">
      <div className="space-y-5">
        <article className="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"><header className="flex items-center gap-2 border-b border-zinc-100 bg-zinc-50 px-4 py-2.5 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950"><span className="grid size-6 place-items-center rounded-full bg-zinc-900 text-[10px] font-semibold text-white dark:bg-zinc-100 dark:text-zinc-950">{issue.author.slice(0, 1).toUpperCase()}</span><strong className="font-medium text-zinc-700 dark:text-zinc-300">{issue.author}</strong><span>创建于 {when(issue.createdAt)}</span></header><div className="min-h-24 whitespace-pre-wrap p-4 text-sm leading-6">{issue.body || <span className="text-zinc-400">未提供描述。</span>}</div></article>
        {comments.map((comment) => <article key={comment.id} className="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"><header className="flex items-center gap-2 border-b border-zinc-100 bg-zinc-50 px-4 py-2.5 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950"><span className="grid size-6 place-items-center rounded-full bg-emerald-100 text-[10px] font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">{comment.author.slice(0, 1).toUpperCase()}</span><strong className="font-medium text-zinc-700 dark:text-zinc-300">{comment.author}</strong><span>commented {when(comment.createdAt)}</span></header><div className="whitespace-pre-wrap p-4 text-sm leading-6">{comment.body}</div></article>)}
        {user ? <form className="rounded-lg border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900" onSubmit={async (event) => { event.preventDefault(); if (!body.trim()) return; setSaving(true); setMessage(""); try { const comment = await api<IssueComment>(`/api/repos/${name}/issues/${issueID}/comments`, { method: "POST", body: JSON.stringify({ body }) }); setComments([...comments, comment]); setBody("") } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法发布评论。") } finally { setSaving(false) } }}><textarea value={body} onChange={(event) => setBody(event.target.value)} placeholder="留下评论" className="min-h-28 w-full resize-y bg-transparent p-2 text-sm leading-6 outline-none" />{message && <p className="px-2 pb-2 text-sm text-red-600">{message}</p>}<div className="flex justify-end border-t border-zinc-100 pt-3 dark:border-zinc-800"><Button type="submit" isDisabled={saving || !body.trim()}><MessageSquare />{saving ? "正在评论..." : "评论"}</Button></div></form> : <p className="rounded-lg border border-dashed border-zinc-300 p-4 text-sm text-zinc-500 dark:border-zinc-700">登录后即可参与讨论。</p>}
      </div>
      <aside className="border-t border-zinc-200 text-sm dark:border-zinc-800 lg:border-t-0 lg:border-l lg:pl-5"><IssueMeta label="受理人" value="未分配" /><IssueMeta label="标签" value="暂无标签" /><IssueMeta label="项目" value="暂无项目" />{user && <div className="py-5"><p className="font-medium">状态</p><Button size="sm" variant="outline" className="mt-3" onPress={() => updateState(issue.state === "open" ? "closed" : "open")}>{issue.state === "open" ? "关闭议题" : "重新打开议题"}</Button></div>}</aside>
    </div>
  </div>
}

function PullRequestList({ items, user, onNew, onDelete, onStateChange }: { items: PullRequest[]; user: User | null; onNew: () => void; onDelete: (id: number) => void; onStateChange: (id: number, state: string) => void }) {
  const [filter, setFilter] = useState("")
  const visible = items.filter((item) => `${item.title} ${item.author} ${item.state}`.toLowerCase().includes(filter.toLowerCase()))
  return <>
    <div className="mb-5 flex flex-wrap items-center justify-between gap-3"><h1 className="text-2xl font-semibold">拉取请求</h1><Button onPress={onNew} isDisabled={!user}><GitPullRequest /> 新建拉取请求</Button></div>
    <div className="mb-4 flex items-center rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900"><span className="px-3 text-zinc-400"><ListFilter className="size-4" /></span><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="筛选拉取请求" className="h-10 min-w-0 flex-1 bg-transparent text-sm outline-none" /><span className="mr-3 text-xs text-zinc-500">{visible.length} 项结果</span></div>
    <div className="overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900">{visible.length ? visible.map((item) => <article key={item.id} className="border-b border-zinc-200 px-5 py-4 last:border-0 dark:border-zinc-800"><div className="flex flex-wrap items-center gap-x-2 gap-y-1"><Badge state={item.state} /><strong>#{item.id} {item.title}</strong></div><p className="mt-2 text-sm text-zinc-500"><code>{item.sourceBranch}</code> 合并到 <code>{item.targetBranch}</code> · {item.author} 创建于 {when(item.createdAt)}</p>{user && <div className="mt-3 flex gap-2"><Button size="xs" variant="outline" onPress={() => onStateChange(item.id, item.state === "open" ? "closed" : "open")}>{item.state === "open" ? "关闭" : "重新打开"}</Button><Button size="xs" variant="destructive" onPress={() => onDelete(item.id)}>删除</Button></div>}</article>) : <Empty icon={<GitPullRequest />} title="欢迎使用拉取请求" text={user ? "比较两个分支以开始创建拉取请求。" : "登录后即可创建拉取请求。"} />}</div>
  </>
}

function CompareChanges({ name, user, onCancel, onCreated }: { name: string; user: User | null; onCancel: () => void; onCreated: (value: Record<string, string>) => Promise<void> }) {
  const [branches, setBranches] = useState<GitRef[]>([])
  const [base, setBase] = useState("main")
  const [compare, setCompare] = useState("")
  const [title, setTitle] = useState("")
  const [body, setBody] = useState("")
  const [message, setMessage] = useState("")
  const [saving, setSaving] = useState(false)
  useEffect(() => { void api<GitRef[]>(`/api/repos/${name}/branches`).then((items) => { setBranches(items); if (items[0]) setBase(items.find((item) => item.name === "main")?.name || items[0].name); if (items[1]) setCompare(items[1].name) }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载分支。")) }, [name])
  const canCreate = Boolean(user && compare && compare !== base && title.trim())
  return <div className="mx-auto max-w-5xl"><div className="mb-2 flex items-center gap-2 text-sm text-zinc-500"><GitCompareArrows className="size-4" /> 比较变更</div><h1 className="text-2xl font-semibold">比较分支</h1><p className="mt-2 text-sm text-zinc-500">选择基准分支和包含变更的分支。</p><div className="mt-6 flex flex-wrap items-center gap-3 rounded-lg border border-zinc-300 bg-zinc-50 p-4 dark:border-zinc-700 dark:bg-zinc-900"><BranchSelect label="基准" value={base} branches={branches} onChange={setBase} /><ArrowLeft className="size-4 text-zinc-400" /><BranchSelect label="比较" value={compare} branches={branches} onChange={setCompare} /></div>{message && <p className="mt-4 text-sm text-red-600">{message}</p>}{compare && compare === base && <p className="mt-5 rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200">请选择不同的分支或派生仓库进行比较。</p>}<form className="mt-6 space-y-4" onSubmit={async (event) => { event.preventDefault(); if (!canCreate) return; setSaving(true); setMessage(""); try { await onCreated({ title, body, sourceBranch: compare, targetBranch: base }) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法创建拉取请求。") } finally { setSaving(false) } }}><input required value={title} onChange={(event) => setTitle(event.target.value)} placeholder="拉取请求标题" className="h-11 w-full rounded-lg border border-zinc-300 bg-transparent px-3 text-sm outline-none focus:border-sky-600 dark:border-zinc-700" /><textarea required value={body} onChange={(event) => setBody(event.target.value)} placeholder="描述你的变更" className="min-h-36 w-full rounded-lg border border-zinc-300 bg-transparent p-3 text-sm outline-none focus:border-sky-600 dark:border-zinc-700" /><div className="flex justify-end gap-3"><Button type="button" variant="outline" onPress={onCancel}>取消</Button><Button type="submit" isDisabled={!canCreate || saving}>{saving ? "正在创建..." : "创建拉取请求"}</Button></div></form></div>
}

function BranchSelect({ label, value, branches, onChange }: { label: string; value: string; branches: GitRef[]; onChange: (value: string) => void }) {
  return <label className="flex items-center gap-2 text-sm font-medium"><span className="text-zinc-500">{label}：</span><select value={value} onChange={(event) => onChange(event.target.value)} className="h-8 rounded-md border border-zinc-300 bg-white px-2 text-sm dark:border-zinc-700 dark:bg-zinc-950">{label === "比较" && <option value="">选择分支</option>}{branches.length ? branches.map((branch) => <option key={branch.name} value={branch.name}>{branch.name}</option>) : <option value="">暂无分支</option>}</select><ChevronDown className="-ml-7 size-3.5 pointer-events-none text-zinc-500" /></label>
}

function WorkList<T extends { id: number }>({
  title,
  items,
  user,
  fields,
  button,
  onNew,
  onSubmit,
  onDelete,
  onStateChange,
  render,
}: {
  title: string
  items: T[]
  user: User | null
  fields: string[]
  button: string
  onNew?: () => void
  onSubmit: (value: Record<string, string>) => void
  onDelete?: (id: number) => void
  onStateChange?: (id: number, state: string) => void
  render: (item: T) => ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [values, setValues] = useState<Record<string, string>>({})
  const [deleting, setDeleting] = useState<number | null>(null)
  const [editing, setEditing] = useState<number | null>(null)
  const [state, setState] = useState("open")
  const [filter, setFilter] = useState("")
  const visibleItems = items.filter((item) => {
    const value = item as T & { title?: string; author?: string; state?: string }
    const needle = filter.trim().toLowerCase()
    return !needle || `${value.title || ""} ${value.author || ""} ${value.state || ""}`.toLowerCase().includes(needle)
  })
  return (
    <>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold">{title}</h2>
        {user && (
          <Button size="sm" onPress={() => onNew ? onNew() : setOpen(!open)}>
            <Plus />
            {button}
          </Button>
        )}
      </div>
      {open && (
        <Modal title={button} onClose={() => setOpen(false)}>
          <form
            className="grid gap-3"
            onSubmit={(e) => {
              e.preventDefault()
              onSubmit(values)
              setValues({})
              setOpen(false)
            }}
          >
            {fields.map((field) => (
              <label key={field} className="text-sm font-medium">
                {field === "sourceBranch"
                  ? "Source branch"
                  : field === "targetBranch"
                    ? "Target branch"
                    : field === "tagName"
                      ? "Tag"
                      : field[0].toUpperCase() + field.slice(1)}
                {field === "body" || field === "notes" ? (
                  <textarea
                    required={field === "body"}
                    value={values[field] || ""}
                    onChange={(e) =>
                      setValues({ ...values, [field]: e.target.value })
                    }
                    className="mt-1 block min-h-20 w-full rounded-xl border border-zinc-200 bg-transparent p-2 text-sm dark:border-zinc-700"
                  />
                ) : (
                  <input
                    required={field !== "body"}
                    value={values[field] || ""}
                    onChange={(e) =>
                      setValues({ ...values, [field]: e.target.value })
                    }
                    className="mt-1 block h-9 w-full rounded-xl border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700"
                  />
                )}
              </label>
            ))}
            <Button type="submit" className="w-fit">
              Save
            </Button>
          </form>
        </Modal>
      )}
      <div className="mb-3 flex flex-wrap items-center gap-3 rounded-xl border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950"><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder={`筛选${title}（标题、作者或状态）`} className="h-8 min-w-52 flex-1 bg-transparent px-2 text-sm outline-none"/><span className="text-xs text-zinc-500">{visibleItems.length} 项结果</span></div>
      <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
        {visibleItems.length ? (
          visibleItems.map((item) => (
            <article
              key={item.id}
              className="border-b border-zinc-100 px-5 py-4 last:border-0 dark:border-zinc-800"
            >
              {render(item)}
              {user && (onDelete || onStateChange) && (
                <div className="mt-3 flex gap-2">
                  {onStateChange && (
                    <Button
                      variant="outline"
                      size="xs"
                      onPress={() => {
                        setEditing(item.id)
                        setState(
                          (item as T & { state?: string }).state || "open"
                        )
                      }}
                    >
                      Edit status
                    </Button>
                  )}
                  {onDelete && (
                    <Button
                      variant="destructive"
                      size="xs"
                      onPress={() => setDeleting(item.id)}
                    >
                      Delete
                    </Button>
                  )}
                </div>
              )}
            </article>
          ))
        ) : (
          <Empty
            icon={<GitPullRequest />}
            title={`暂无${title}`}
            text={
              user
                ? `创建第一个${title.slice(0, -1)}。`
                : "登录后参与协作。"
            }
          />
        )}
      </div>
      {editing !== null && (
        <Modal title="Edit status" onClose={() => setEditing(null)}>
          <div className="space-y-4">
            <label className="block text-sm font-medium">
              Status
              <select
                value={state}
                onChange={(event) => setState(event.target.value)}
                className="mt-1 block h-10 w-full rounded-xl border border-zinc-200 bg-transparent px-3 dark:border-zinc-700"
              >
                <option value="open">Open</option>
                <option value="closed">Closed</option>
                <option value="merged">Merged</option>
              </select>
            </label>
            <Button
              onPress={() => {
                onStateChange?.(editing, state)
                setEditing(null)
              }}
            >
              Save changes
            </Button>
          </div>
        </Modal>
      )}
      {deleting !== null && (
        <Modal
          title={`Delete ${title.slice(0, -1)}`}
          onClose={() => setDeleting(null)}
        >
          <p className="text-sm text-zinc-500">
            This action permanently removes the item.
          </p>
          <div className="mt-5 flex justify-end gap-2">
            <Button variant="outline" onPress={() => setDeleting(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onPress={() => {
                onDelete?.(deleting)
                setDeleting(null)
              }}
            >
              Delete
            </Button>
          </div>
        </Modal>
      )}
    </>
  )
}

function Modal({
  title,
  onClose,
  children,
}: {
  title: string
  onClose: () => void
  children: ReactNode
}) {
  return (
    <div
      className="fixed inset-0 z-30 grid place-items-center bg-zinc-950/45 p-5"
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <div className="w-full max-w-lg rounded-2xl bg-white p-5 shadow-2xl dark:bg-zinc-900">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="font-semibold">{title}</h2>
          <button onClick={onClose} aria-label="Close dialog">
            <X className="size-4 text-zinc-500" />
          </button>
        </div>
        {children}
      </div>
    </div>
  )
}

export default function RepositoryRoutePage({ user, onBack, onOpen }: { user: User | null; onBack: () => void; onOpen: (name: string) => void }) {
  const { scope, name } = useParams()
  return scope && name ? <RepositoryView name={`${scope}/${name}`} user={user} onBack={onBack} onOpen={onOpen} /> : null
}
