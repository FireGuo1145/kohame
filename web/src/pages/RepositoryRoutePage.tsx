import { useEffect, useState, type FormEvent, type ReactNode } from "react"
import { useLocation, useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, Check, ChevronDown, CircleDot, Code2, Copy, FileCode2, FolderGit2, GitBranch, GitCompareArrows, GitCommitHorizontal, GitPullRequest, ListFilter, MessageSquare, Plus, Search, Settings, ShieldCheck, Star, Tag, Trash2, Upload, UserPlus, Users, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Avatar, Badge, Empty, Field, Loading, PageMessage } from "@/components/forge-ui"
import { api, when } from "@/lib/forge-api"
import type { Blob, Collaborator, Commit, CommitDetail, CommitFile, Contributor, GitRef, Issue, IssueComment, Label, Language, ProtectedBranch, PullRequest, PullRequestComment, Release, Repository, RepositorySettings, SSHInfo, TreeEntry, User } from "@/lib/forge-types"

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
  const commitMatch = subpath.match(/^commits\/([A-Za-z0-9._/-]+)$/)
  const commitHash = commitMatch ? commitMatch[1] : ""
  const releaseMatch = subpath.match(/^releases\/(\d+)$/)
  const releaseID = releaseMatch ? Number(releaseMatch[1]) : 0
  const pullMatch = subpath.match(/^pulls\/(\d+)$/)
  const pullID = pullMatch ? Number(pullMatch[1]) : 0
  const filePath = subpath.startsWith("blob/") ? decodeURIComponent(subpath.slice("blob/".length)) : ""
  const fileRef = new URLSearchParams(location.search).get("ref") || "HEAD"
  const treeMatch = subpath.match(/^tree\/([^/]+)(?:\/(.*))?$/)
  const treeRef = treeMatch ? decodeURIComponent(treeMatch[1]) : "HEAD"
  const treeDirectory = treeMatch?.[2] ? decodeURIComponent(treeMatch[2]) : ""
  const tab = subpath === "new" ? "file-new" : subpath === "issues/new" ? "issue-new" : issueMatch ? "issue-detail" : commitMatch ? "commit-detail" : releaseMatch ? "release-detail" : pullMatch ? "pull-detail" : treeMatch ? "code" : filePath ? "file" : subpath || "code"
  const [issues, setIssues] = useState<Issue[]>([])
  const [pulls, setPulls] = useState<PullRequest[]>([])
  const [releases, setReleases] = useState<Release[]>([])
  const [contributors, setContributors] = useState<Contributor[]>([])
  const [repository, setRepository] = useState<Repository | null>(null)
  const [repositorySettings, setRepositorySettings] = useState<RepositorySettings | null>(null)
  const [forkOpen, setForkOpen] = useState(false)
  const [forkName, setForkName] = useState("")
  const [forkScopes, setForkScopes] = useState<{ name: string }[]>([])
  const [forkScope, setForkScope] = useState("")
  const [forkDefaultOnly, setForkDefaultOnly] = useState(false)
  const [message, setMessage] = useState("")
  const load = async () => {
    const [a, b, c, d, repo, settings] = await Promise.all([
      api<Issue[]>(`/api/repos/${name}/issues`),
      api<PullRequest[]>(`/api/repos/${name}/pulls`),
      api<Release[]>(`/api/repos/${name}/releases`),
      api<Contributor[]>(`/api/repos/${name}/contributors`),
      api<Repository>(`/api/repos/${name}`),
      api<RepositorySettings>(`/api/repos/${name}/settings`),
    ])
    setIssues(a.map((item) => ({ ...item, labels: item.labels || [] })))
    setPulls(b)
    setReleases(c)
    setContributors(d)
    setRepository(repo)
    setRepositorySettings(settings)
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
  ].filter(([value]) => value === "issues" ? repositorySettings?.issuesEnabled !== false : value === "pulls" ? repositorySettings?.pullsEnabled !== false : value === "releases" ? repositorySettings?.releasesEnabled !== false : true) as [string, string, ReactNode][]
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
          <Button variant="outline" size="sm" onPress={async () => { setForkOpen(true); try { const scopes = await api<{ name: string }[]>("/api/scopes"); setForkScopes(scopes); setForkScope(scopes.find((scope) => scope.name === user?.username)?.name || scopes[0]?.name || "") } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法读取可用空间。") } }} isDisabled={!user || repositorySettings?.allowForks === false}><GitBranch /> 派生 {repository?.forks ?? 0}</Button>
          {repository?.forks ? <Button variant="ghost" size="sm" onPress={() => navigate(`/${name}/forks`)}>查看 Fork</Button> : null}
          {repository?.forkedFrom && <Button variant="outline" size="sm" onPress={() => navigate(`/${repository.forkedFrom}/compare?sourceRepo=${encodeURIComponent(name)}`)} isDisabled={!user}><GitPullRequest /> 向上游发起 PR</Button>}
          <Button variant={repository?.starred ? "secondary" : "outline"} size="sm" isDisabled={!user} onPress={async()=>{try{const result=await api<{starred:boolean;stars:number}>(`/api/repos/${name}/star`,{method:"POST"});setRepository(repository?{...repository,...result}:repository)}catch(cause){setMessage(cause instanceof Error?cause.message:"无法更新 Star。")}}}><Star className={repository?.starred?"fill-current":""} /> 收藏 {repository?.stars ?? 0}</Button>
          <Button variant="outline" size="sm" onPress={() => void navigator.clipboard.writeText(`git clone ${window.location.origin}/${name}.git`)}><Copy /> 克隆</Button>
        </div>
      </div>
      {(repository?.forkedFrom || repositorySettings?.description || repositorySettings?.homepageUrl) && <div className="mt-3 space-y-1 text-sm text-zinc-500">{repository?.forkedFrom && <p>派生自 <span className="font-medium text-sky-700 dark:text-sky-300">{repository.forkedFrom}</span></p>}{repositorySettings?.description && <p>{repositorySettings.description}</p>}{repositorySettings?.homepageUrl && <a className="inline-block text-sky-700 hover:underline dark:text-sky-300" href={repositorySettings.homepageUrl} target="_blank" rel="noreferrer">项目主页 ↗</a>}</div>}
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
        {tab === "code" && <CodeBrowser name={name} user={user} initialRef={treeRef} initialDirectory={treeDirectory} onBrowse={(ref, directory) => navigate(`/${name}/tree/${encodeURIComponent(ref)}${directory ? `/${directory.split("/").map(encodeURIComponent).join("/")}` : ""}`)} onCreate={() => navigate(`/${name}/new`)} onOpenFile={(path, ref) => navigate(`/${name}/blob/${path.split("/").map(encodeURIComponent).join("/")}?ref=${encodeURIComponent(ref)}`)} />}
        {tab === "file" && <FilePreview name={name} path={filePath} refName={fileRef} user={user} onBack={() => navigate(`/${name}`)} />}
        {tab === "file-new" && <FilePreview name={name} path="" refName={fileRef} user={user} onBack={() => navigate(`/${name}`)} />}
        {tab === "commits" && <RepositoryList name={name} kind="commits" onOpenCommit={(hash) => navigate(`/${name}/commits/${hash}`)} />}
        {tab === "commit-detail" && <CommitDetailPage name={name} hash={commitHash} onBack={() => navigate(`/${name}/commits`)} />}
        {tab === "branches" && <RepositoryList name={name} kind="branches" />}
        {tab === "tags" && <RepositoryList name={name} kind="tags" />}
        {tab === "issues" && (
          <WorkList
            title="议题"
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
                <LabelBadges labels={item.labels} />
                <p className="mt-1 text-sm text-zinc-500">由 {item.author} 创建于 {when(item.createdAt)}</p>
              </button>
            )}
          />
        )}
        {tab === "issue-new" && <IssueComposer name={name} user={user} onCancel={() => navigate(`/${name}/issues`)} onCreated={() => navigate(`/${name}/issues`)} />}
        {tab === "issue-detail" && <IssueDetail name={name} issueID={issueID} user={user} onBack={() => navigate(`/${name}/issues`)} onStateChange={(state) => changeState("issues", issueID, state)} />}
        {tab === "pulls" && <PullRequestList items={pulls} user={user} onNew={() => navigate(`/${name}/compare`)} onDelete={(id) => remove("pulls", id)} onStateChange={(id, state) => changeState("pulls", id, state)} onOpen={(id) => navigate(`/${name}/pulls/${id}`)} />}
        {tab === "pull-detail" && <PullRequestDetailPage name={name} pullID={pullID} user={user} onBack={() => navigate(`/${name}/pulls`)} />}
        {tab === "compare" && <CompareChanges name={name} user={user} onCancel={() => navigate(`/${name}/pulls`)} onCreated={async (value) => { if (await add("pull", value)) navigate(`/${name}/pulls`) }} />}
        {tab === "releases" && <ReleaseList name={name} user={user} releases={releases} onDelete={(id) => remove("releases", id)} onChanged={() => void load()} onOpen={(id) => navigate(`/${name}/releases/${id}`)} />}
        {tab === "forks" && <ForkList name={name} onOpen={onOpen} />}
        {tab === "release-detail" && <ReleaseDetailPage name={name} releaseID={releaseID} onBack={() => navigate(`/${name}/releases`)} />}
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
        {tab === "settings" && <RepositorySettingsPanel name={name} user={user} onDeleted={onBack} onTransferred={onOpen} />}
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
                  body: JSON.stringify({ name: forkName, scope: forkScope, defaultBranchOnly: forkDefaultOnly }),
                })
                setForkOpen(false)
                onOpen(repo.fullName)
              } catch (cause) {
                setMessage(cause instanceof Error ? cause.message : "派生失败。")
              }
            }}
          >
            <p className="text-sm text-zinc-500">创建一份独立的 bare Git 仓库副本到你的个人空间。</p>
            <label className="block text-sm font-medium">Fork 到<select value={forkScope} onChange={(event) => setForkScope(event.target.value)} className="mt-1.5 block h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700">{forkScopes.map((scope) => <option key={scope.name} value={scope.name}>{scope.name}</option>)}</select></label>
            <Field label="Fork 名称" value={forkName} onChange={setForkName} placeholder={name.split("/")[1]} />
            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={forkDefaultOnly} onChange={(event) => setForkDefaultOnly(event.target.checked)} />仅派生默认分支</label>
            <Button type="submit"><GitBranch /> 创建 Fork</Button>
          </form>
        </Modal>
      )}
    </div>
  )
}

function CodeBrowser({ name, user, initialRef, initialDirectory, onBrowse, onOpenFile, onCreate }: { name: string; user: User | null; initialRef: string; initialDirectory: string; onBrowse: (ref: string, directory: string) => void; onOpenFile: (path: string, ref: string) => void; onCreate: () => void }) {
  const [directory, setDirectory] = useState(initialDirectory)
  const [entries, setEntries] = useState<TreeEntry[]>([])
  const [branches, setBranches] = useState<GitRef[]>([])
  const [tags, setTags] = useState<GitRef[]>([])
  const [commits, setCommits] = useState<Commit[]>([])
  const [settings, setSettings] = useState<RepositorySettings | null>(null)
  const [releases, setReleases] = useState<Release[]>([])
  const [contributors, setContributors] = useState<Contributor[]>([])
  const [languages, setLanguages] = useState<Language[]>([])
  const [license, setLicense] = useState("")
  const [ref, setRef] = useState(initialRef)
  const [filter, setFilter] = useState("")
  const [copied, setCopied] = useState(false)
  const [branchMenuOpen, setBranchMenuOpen] = useState(false)
  const [codeMenuOpen, setCodeMenuOpen] = useState(false)
  const [cloneProtocol, setCloneProtocol] = useState<"https" | "ssh">("https")
  const [ssh, setSSH] = useState<SSHInfo | null>(null)
  const [branchQuery, setBranchQuery] = useState("")
  const [refTab, setRefTab] = useState<"branches" | "tags">("branches")
  const [message, setMessage] = useState("")
  useEffect(() => { setRef(initialRef); setDirectory(initialDirectory) }, [name, initialRef, initialDirectory])
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
      api<GitRef[]>(`/api/repos/${name}/tags`),
      api<Commit[]>(`/api/repos/${name}/commits?ref=${encodeURIComponent(ref)}`),
      api<RepositorySettings>(`/api/repos/${name}/settings`),
      api<Release[]>(`/api/repos/${name}/releases`),
      api<Contributor[]>(`/api/repos/${name}/contributors`),
    ]).then(([nextBranches, nextTags, nextCommits, nextSettings, nextReleases, nextContributors]) => {
      setBranches(nextBranches); setTags(nextTags); setCommits(nextCommits); setSettings(nextSettings); setReleases(nextReleases); setContributors(nextContributors)
      if (ref === "HEAD" && nextBranches[0]) { const nextRef = nextBranches.some((branch) => branch.name === nextSettings.defaultBranch) ? nextSettings.defaultBranch : nextBranches[0].name; setRef(nextRef) }
    }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载仓库概览。"))
  }, [name, ref])
  useEffect(() => { void api<SSHInfo>("/api/ssh").then(setSSH).catch(() => setSSH(null)) }, [])
  useEffect(() => { void api<{ license: string }>(`/api/repos/${name}/license?ref=${encodeURIComponent(ref)}`).then((value) => setLicense(value.license)).catch(() => setLicense("")) }, [name, ref])
  useEffect(() => { void api<Language[]>(`/api/repos/${name}/languages?ref=${encodeURIComponent(ref)}`).then(setLanguages).catch(() => setLanguages([])) }, [name, ref])
  const openEntry = (entry: TreeEntry) => {
    if (entry.type === "tree") {
      setDirectory(entry.path); onBrowse(ref, entry.path)
      return
    }
    onOpenFile(entry.path, ref)
  }
  const crumbs = directory ? directory.split("/") : []
  const visibleEntries = entries.filter((entry) => entry.name.toLowerCase().includes(filter.trim().toLowerCase()))
  const latestCommit = commits[0]
  const httpsCloneURL = `${window.location.origin}/${name}.git`
  const sshCloneURL = ssh ? (ssh.port === "22" ? `git@${ssh.host}:${name}.git` : `ssh://git@${ssh.host}:${ssh.port}/${name}.git`) : ""
  const cloneURL = cloneProtocol === "ssh" ? sshCloneURL : httpsCloneURL
  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_250px]">
      <div className="min-w-0">
        <div className="relative mb-4 flex flex-wrap items-center gap-2">
          <div className="relative">
            <Button variant="outline" size="sm" aria-expanded={branchMenuOpen} onPress={() => { setBranchMenuOpen(!branchMenuOpen); setCodeMenuOpen(false) }}><GitBranch /> {ref === "HEAD" ? "默认分支" : ref}<ChevronDown /></Button>
            {branchMenuOpen && <div className="absolute left-0 top-10 z-20 w-[min(21rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-zinc-700 bg-zinc-950 text-zinc-100 shadow-2xl"><div className="flex items-center justify-between border-b border-zinc-800 px-4 py-3"><strong>切换分支或标签</strong><button onClick={() => setBranchMenuOpen(false)} aria-label="关闭"><X className="size-4 text-zinc-400" /></button></div><label className="m-3 flex h-9 items-center gap-2 rounded-lg border border-zinc-700 px-2.5 text-zinc-400"><Search className="size-4" /><input value={branchQuery} onChange={(event) => setBranchQuery(event.target.value)} placeholder={refTab === "branches" ? "查找分支..." : "查找标签..."} className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-zinc-500" /></label><div className="flex border-y border-zinc-800"><button onClick={() => setRefTab("branches")} className={`px-4 py-2 text-sm ${refTab === "branches" ? "border-b-2 border-emerald-400 font-medium" : "text-zinc-400"}`}>分支</button><button onClick={() => setRefTab("tags")} className={`px-4 py-2 text-sm ${refTab === "tags" ? "border-b-2 border-emerald-400 font-medium" : "text-zinc-400"}`}>标签</button></div><div className="max-h-52 overflow-auto p-2">{(refTab === "branches" ? branches : tags).filter((item) => item.name.toLowerCase().includes(branchQuery.toLowerCase())).map((item) => <button key={item.name} onClick={() => { setRef(item.name); setDirectory(""); setBranchMenuOpen(false); onBrowse(item.name, "") }} className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm hover:bg-zinc-900"><Check className={`size-4 ${ref === item.name ? "text-emerald-400" : "text-transparent"}`} />{item.name}{refTab === "branches" && item.name === settings?.defaultBranch && <span className="ml-auto rounded-full border border-zinc-600 px-2 py-0.5 text-xs text-zinc-300">默认</span>}</button>)}{!(refTab === "branches" ? branches : tags).length && <p className="px-3 py-4 text-sm text-zinc-500">暂无{refTab === "branches" ? "分支" : "标签"}</p>}</div></div>}
          </div>
          <button className="inline-flex h-9 items-center gap-1 rounded-lg px-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-900"><GitBranch className="size-4" />{branches.length} 个分支</button>
          <button className="inline-flex h-9 items-center gap-1 rounded-lg px-2 text-sm text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-900"><Tag className="size-4" />{releases.length} 个发布</button>
          <label className="ml-auto flex h-9 min-w-48 flex-1 items-center gap-2 rounded-lg border border-zinc-300 bg-white px-3 text-zinc-400 shadow-sm sm:max-w-72 dark:border-zinc-700 dark:bg-zinc-900"><ListFilter className="size-4" /><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="转到文件" className="w-full bg-transparent text-sm text-zinc-900 outline-none dark:text-zinc-100" /></label>
          {user && <Button variant="outline" size="sm" onPress={onCreate}><Plus />添加文件</Button>}<div className="relative"><Button variant="outline" size="sm" aria-expanded={codeMenuOpen} onPress={() => { setCodeMenuOpen(!codeMenuOpen); setBranchMenuOpen(false) }}><Code2 />{copied ? "已复制" : "代码"}<ChevronDown /></Button>{codeMenuOpen && <div className="absolute right-0 top-10 z-20 w-80 overflow-hidden rounded-lg border border-zinc-700 bg-zinc-950 p-4 text-zinc-100 shadow-2xl"><div className="mb-3 flex items-center justify-between"><strong>克隆仓库</strong><button onClick={() => setCodeMenuOpen(false)} aria-label="关闭"><X className="size-4 text-zinc-400" /></button></div><div className="mb-3 flex gap-1 rounded-lg border border-zinc-700 p-1 text-sm"><button onClick={() => setCloneProtocol("https")} className={`rounded-md px-3 py-1.5 ${cloneProtocol === "https" ? "bg-zinc-700 font-medium" : "text-zinc-400"}`}>HTTPS</button><button onClick={() => setCloneProtocol("ssh")} className={`rounded-md px-3 py-1.5 ${cloneProtocol === "ssh" ? "bg-zinc-700 font-medium" : "text-zinc-400"}`}>SSH</button></div><div className="flex items-center gap-2 rounded-lg border border-zinc-700 bg-zinc-900 p-2"><code className="min-w-0 flex-1 truncate text-xs text-zinc-300">{cloneURL || "SSH 服务未配置"}</code><button disabled={!cloneURL} onClick={() => { void navigator.clipboard.writeText(`git clone ${cloneURL}`); setCopied(true) }} aria-label="复制克隆命令"><Copy className="size-4 text-zinc-400" /></button></div><p className="mt-3 text-xs text-zinc-500">{cloneProtocol === "ssh" ? "请先在个人设置中添加 SSH 公钥。" : "使用网页地址克隆此仓库。"}</p><div className="mt-4 space-y-1 border-t border-zinc-800 pt-3 text-sm"><button onClick={() => { window.location.href = `/api/repos/${name}/archive?ref=${encodeURIComponent(ref)}` }} className="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left hover:bg-zinc-900"><FolderGit2 className="size-4" />下载 ZIP</button></div></div>}</div>
        </div>
        {message && <p className="mb-4 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/50">{message}</p>}
        <div className="overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900">
          <div className="flex items-center gap-2 border-b border-zinc-200 bg-zinc-50 px-4 py-3 text-sm dark:border-zinc-800 dark:bg-zinc-950"><span className="grid size-6 place-items-center rounded-full bg-zinc-900 text-[10px] font-semibold text-white dark:bg-zinc-100 dark:text-zinc-950">{latestCommit?.author.slice(0, 1).toUpperCase() || "K"}</span><span className="min-w-0 flex-1 truncate"><strong className="font-medium">{latestCommit?.author || "暂无提交"}</strong>{latestCommit && <span className="ml-2 text-zinc-500">{latestCommit.subject}</span>}</span>{latestCommit && <span className="hidden text-xs text-zinc-500 sm:block">{latestCommit.hash} · {when(latestCommit.date)}</span>}</div>
          {directory && <div className="flex items-center gap-1 border-b border-zinc-100 px-4 py-2 text-sm text-zinc-500 dark:border-zinc-800"><button onClick={() => { setDirectory(""); onBrowse(ref, "") }} className="text-sky-700 hover:underline dark:text-sky-300">{name}</button>{crumbs.map((crumb, index) => <span key={`${crumb}-${index}`}><span className="mx-1 text-zinc-400">/</span><button onClick={() => { const path = crumbs.slice(0, index + 1).join("/"); setDirectory(path); onBrowse(ref, path) }} className="hover:underline">{crumb}</button></span>)}</div>}
          {visibleEntries.length ? visibleEntries.map((entry) => <button key={entry.path} onClick={() => openEntry(entry)} className="flex w-full items-center gap-3 border-b border-zinc-100 px-4 py-3 text-left text-sm last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800"><span className={`grid size-5 place-items-center ${entry.type === "tree" ? "text-amber-600" : "text-zinc-500"}`}>{entry.type === "tree" ? <FolderGit2 className="size-4" /> : <FileCode2 className="size-4" />}</span><span className="font-medium text-sky-700 dark:text-sky-300">{entry.name}</span><span className="ml-auto text-xs text-zinc-400">{entry.type === "tree" ? "目录" : "文件"}</span></button>) : <Empty icon={<FolderGit2 />} title="此分支暂无文件" text="推送一次提交后即可在这里浏览源代码。" />}
        </div>
      </div>
      <aside className="border-t border-zinc-200 pt-5 dark:border-zinc-800 xl:border-t-0 xl:border-l xl:pl-6 xl:pt-0"><h2 className="font-semibold">关于</h2><p className="mt-3 text-sm leading-6 text-zinc-500">{settings?.description || "暂无项目简介、网站或主题。"}</p>{settings?.topics.length ? <div className="mt-4 flex flex-wrap gap-1.5">{settings.topics.map((topic) => <span key={topic} className="rounded-full bg-sky-50 px-2 py-0.5 text-xs font-medium text-sky-700 dark:bg-sky-950 dark:text-sky-300">{topic}</span>)}</div> : null}<div className="mt-5 space-y-3 border-b border-zinc-200 pb-5 text-sm dark:border-zinc-800"><p className="flex items-center gap-2 text-zinc-600 dark:text-zinc-400"><GitCommitHorizontal className="size-4" />{commits.length} 次近期提交</p><p className="flex items-center gap-2 text-zinc-600 dark:text-zinc-400"><Users className="size-4" />{contributors.length} 位贡献者</p>{license && <p className="flex items-center gap-2 text-zinc-600 dark:text-zinc-400"><FileCode2 className="size-4" />许可证：{license}</p>}</div><LanguageStats languages={languages} />{releases.length ? <div className="pt-5"><h3 className="font-semibold">最新发布</h3>{releases.slice(0, 2).map((release) => <a href={`/${name}/releases/${release.id}`} key={release.id} className="mt-3 block"><p className="flex items-center gap-1 text-sm font-medium text-emerald-700 dark:text-emerald-300"><Tag className="size-3.5" />{release.tagName}</p><p className="mt-1 text-xs text-zinc-500">{release.title}</p></a>)}</div> : null}{contributors.length ? <div className="border-t border-zinc-200 pt-5 dark:border-zinc-800"><h3 className="font-semibold">贡献者</h3>{contributors.slice(0,5).map((item)=><a href={`/${item.username}`} key={item.username} className="mt-3 flex items-center gap-2 text-sm hover:text-sky-600"><Avatar name={item.username}/><span className="min-w-0 flex-1 truncate">{item.username}</span><span className="text-xs text-zinc-500">{item.contributions}</span></a>)}</div> : null}</aside>
    </div>
  )
}

function LanguageStats({ languages }: { languages: Language[] }) {
  if (!languages.length) return null
  return <section className="border-b border-zinc-200 py-5 dark:border-zinc-800"><h3 className="font-semibold">语言</h3><div className="mt-3 flex h-2 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800">{languages.map((language) => <span key={language.name} style={{ width: `${language.percentage}%`, backgroundColor: language.color }} title={`${language.name} ${language.percentage.toFixed(1)}%`} />)}</div><div className="mt-3 flex flex-wrap gap-x-3 gap-y-2 text-xs">{languages.map((language) => <span key={language.name} className="inline-flex items-center gap-1.5"><i className="size-2.5 rounded-full" style={{ backgroundColor: language.color }} />{language.name} {language.percentage.toFixed(1)}%</span>)}</div></section>
}

function FilePreview({ name, path, refName, user, onBack }: { name: string; path: string; refName: string; user: User | null; onBack: () => void }) {
  const navigate = useNavigate()
  const [file, setFile] = useState<Blob | null>(null)
  const [recent, setRecent] = useState<Commit | null>(null)
  const [message, setMessage] = useState("")
  const [editing, setEditing] = useState(path === "")
  const [preview, setPreview] = useState(path.toLowerCase().endsWith(".md"))
  const [filePath, setFilePath] = useState(path)
  const [content, setContent] = useState("")
  const [commitMessage, setCommitMessage] = useState("")
  useEffect(() => { if (!path) return; void Promise.all([api<Blob>(`/api/repos/${name}/blob?ref=${encodeURIComponent(refName)}&path=${encodeURIComponent(path)}`), api<Commit>(`/api/repos/${name}/file-commit?ref=${encodeURIComponent(refName)}&path=${encodeURIComponent(path)}`)]).then(([value, commit]) => { setFile(value); setContent(value.content); setRecent(commit) }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取文件。")) }, [name, path, refName])
  if (path && !file && !message) return <Loading />
  const save = async () => { try { await api(`/api/repos/${name}/blob`, { method:"PUT", body:JSON.stringify({path:filePath,content,branch:refName,message:commitMessage}) }); onBack() } catch(cause) { setMessage(cause instanceof Error?cause.message:"无法保存文件。") } }
  const rawURL = `/api/repos/${name}/raw?ref=${encodeURIComponent(refName)}&path=${encodeURIComponent(path)}`
  return <div className="mx-auto max-w-7xl"><button onClick={onBack} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />返回文件列表</button><div className="grid gap-6 lg:grid-cols-[230px_minmax(0,1fr)]"><FileTreeSidebar name={name} refName={refName} currentPath={path} onOpen={(nextPath) => navigate(`/${name}/blob/${nextPath.split("/").map(encodeURIComponent).join("/")}?ref=${encodeURIComponent(refName)}`)} /><div className="min-w-0 space-y-4">{recent && <div className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border border-zinc-200 bg-white px-4 py-3 text-sm shadow-sm dark:border-zinc-800 dark:bg-zinc-900"><span className="grid size-6 place-items-center rounded-full bg-emerald-100 text-xs font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">{recent.author.slice(0,1).toUpperCase()}</span><span className="min-w-0 flex-1 truncate"><strong>{recent.author}</strong> · {recent.subject}</span><span className="text-xs text-zinc-500">{recent.hash} · {when(recent.date)}</span></div>}<div className="overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900"><header className="flex flex-wrap items-center gap-2 border-b border-zinc-200 bg-zinc-50 px-4 py-3 text-sm dark:border-zinc-800 dark:bg-zinc-950"><FileCode2 className="size-4 text-zinc-500" />{editing && !path ? <input value={filePath} onChange={(event)=>setFilePath(event.target.value)} placeholder="文件路径，例如 README.md" className="h-8 flex-1 rounded border border-zinc-300 bg-white px-2 outline-none dark:border-zinc-700 dark:bg-zinc-900"/> : <span className="min-w-0 flex-1 truncate font-medium">{path}</span>}<span className="text-xs text-zinc-500">{refName}</span>{path && <><Button size="sm" variant="outline" onPress={() => { void navigator.clipboard.writeText(content) }}><Copy />复制</Button><Button size="sm" variant="outline" onPress={() => { window.location.href = rawURL }}><Upload />下载</Button></>}{user && file?.isText && <Button size="sm" variant="outline" onPress={()=>setEditing(!editing)}>{editing?"取消编辑":"编辑"}</Button>}</header>{file?.isText && path.toLowerCase().endsWith(".md") && !editing && <div className="flex gap-2 border-b border-zinc-200 px-4 py-2 text-sm dark:border-zinc-800"><button onClick={() => setPreview(true)} className={preview ? "font-medium text-emerald-700" : "text-zinc-500"}>预览</button><button onClick={() => setPreview(false)} className={!preview ? "font-medium text-emerald-700" : "text-zinc-500"}>代码</button></div>}{message ? <p className="p-4 text-sm text-red-600">{message}</p> : editing ? <div className="p-4"><textarea value={content} onChange={(event)=>setContent(event.target.value)} className="min-h-[24rem] w-full rounded-md border border-zinc-300 bg-zinc-950 p-4 font-mono text-xs leading-6 text-zinc-100 outline-none dark:border-zinc-700"/><input value={commitMessage} onChange={(event)=>setCommitMessage(event.target.value)} placeholder="提交说明（可选）" className="mt-3 h-9 w-full rounded-md border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700"/>{user?<div className="mt-3 flex justify-end"><Button onPress={save}>提交更改</Button></div>:<p className="mt-3 text-sm text-zinc-500">登录后可编辑文件。</p>}</div> : file?.isText ? preview && path.toLowerCase().endsWith(".md") ? <MarkdownPreview value={file.content} /> : <pre className="max-h-[calc(100svh-15rem)] overflow-auto bg-zinc-950 p-5 text-xs leading-6 text-zinc-100"><code>{file.content}</code></pre> : <div className="grid min-h-64 place-items-center p-6 text-sm text-zinc-500">暂不支持预览二进制文件。</div>}</div></div></div></div>
}

function FileTreeSidebar({ name, refName, currentPath, onOpen }: { name: string; refName: string; currentPath: string; onOpen: (path: string) => void }) {
  const [items, setItems] = useState<TreeEntry[]>([])
  const [filter, setFilter] = useState("")
  useEffect(() => { void api<TreeEntry[]>(`/api/repos/${name}/file-tree?ref=${encodeURIComponent(refName)}`).then(setItems).catch(() => setItems([])) }, [name, refName])
  const visible = items.filter((item) => item.path.toLowerCase().includes(filter.toLowerCase()))
  return <aside className="min-w-0 rounded-lg border border-zinc-200 bg-white p-3 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"><div className="mb-3 flex items-center gap-2"><GitBranch className="size-4 text-zinc-500" /><strong className="text-sm">{refName}</strong></div><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="查找文件" className="mb-3 h-9 w-full rounded-md border border-zinc-300 bg-transparent px-2 text-sm dark:border-zinc-700" /><div className="max-h-[calc(100svh-17rem)] overflow-auto">{visible.map((item) => <button key={item.path} onClick={() => onOpen(item.path)} className={`flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm ${item.path === currentPath ? "bg-emerald-50 font-medium text-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300" : "hover:bg-zinc-100 dark:hover:bg-zinc-800"}`} style={{ paddingLeft: `${8 + (item.path.split("/").length - 1) * 12}px` }}><FileCode2 className="size-3.5 shrink-0 text-zinc-500" /><span className="truncate">{item.name}</span></button>)}</div></aside>
}

function MarkdownPreview({ value }: { value: string }) {
  const lines = value.split("\n")
  return <article className="prose prose-zinc max-w-none p-6 dark:prose-invert">{lines.map((line, index) => line.startsWith("### ") ? <h3 key={index}>{line.slice(4)}</h3> : line.startsWith("## ") ? <h2 key={index}>{line.slice(3)}</h2> : line.startsWith("# ") ? <h1 key={index}>{line.slice(2)}</h1> : line.startsWith("> ") ? <blockquote key={index}>{line.slice(2)}</blockquote> : line.startsWith("- ") ? <li key={index}>{line.slice(2)}</li> : line ? <p key={index}>{line}</p> : <br key={index} />)}</article>
}

function RepositoryList({
  name,
  kind,
  onOpenCommit,
}: {
  name: string
  kind: "commits" | "branches" | "tags"
  onOpenCommit?: (hash: string) => void
}) {
  const [items, setItems] = useState<(Commit | GitRef)[]>([])
  const [branches, setBranches] = useState<GitRef[]>([])
  const [ref, setRef] = useState("HEAD")
  const [message, setMessage] = useState("")
  useEffect(() => {
    void api<(Commit | GitRef)[]>(`/api/repos/${name}/${kind}${kind === "commits" ? `?ref=${encodeURIComponent(ref)}` : ""}`)
      .then(setItems)
      .catch((cause: unknown) =>
        setMessage(cause instanceof Error ? cause.message : "加载失败")
      )
  }, [name, kind, ref])
  useEffect(() => { if (kind === "commits") void api<GitRef[]>(`/api/repos/${name}/branches`).then((values) => { setBranches(values); if (ref === "HEAD" && values[0]) setRef(values.find((item) => item.name === "main")?.name || values[0].name) }).catch(() => setBranches([])) }, [name, kind])
  const title =
    kind === "commits" ? "提交记录" : kind === "branches" ? "分支" : "标签"
  return (
    <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-100 px-5 py-4 dark:border-zinc-800">
        <h2 className="text-xl font-semibold">{title}</h2>
        {kind === "commits" && <label className="flex items-center gap-2 text-sm"><GitBranch className="size-4 text-zinc-500" /><select value={ref} onChange={(event) => setRef(event.target.value)} className="h-9 rounded-lg border border-zinc-300 bg-transparent px-2 text-sm dark:border-zinc-700">{branches.map((branch) => <option key={branch.name} value={branch.name}>{branch.name}</option>)}</select></label>}
      </div>
      {message && <p className="p-4 text-sm text-red-600">{message}</p>}
      {items.length ? (
        items.map((item) =>
          "subject" in item ? (
            <button onClick={() => onOpenCommit?.(item.hash)} key={item.hash} className="relative block w-full border-b border-zinc-100 px-5 py-4 pl-10 text-left last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800">
              <span className="absolute left-5 top-6 size-2.5 rounded-full border-2 border-emerald-500 bg-white dark:bg-zinc-900" />
              <p className="font-medium">{item.subject}</p>
              <p className="mt-1 text-sm text-zinc-500">{item.author} 提交于 {when(item.date)}</p>
              <code className="absolute right-5 top-5 rounded bg-zinc-100 px-2 py-1 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">{item.hash}</code>
            </button>
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

function CommitDetailPage({ name, hash, onBack }: { name: string; hash: string; onBack: () => void }) {
  const [detail, setDetail] = useState<CommitDetail | null>(null)
  const [message, setMessage] = useState("")
  useEffect(() => { void api<CommitDetail>(`/api/repos/${name}/commits/${encodeURIComponent(hash)}`).then(setDetail).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取提交详情。")) }, [name, hash])
  if (message) return <PageMessage title="提交详情" message={message} />
  if (!detail) return <Loading />
  return <div className="mx-auto max-w-6xl"><button onClick={onBack} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />所有提交</button><section className="rounded-lg border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"><p className="font-mono text-xs text-zinc-500">{detail.hash}</p><h1 className="mt-2 text-2xl font-semibold">{detail.subject}</h1><p className="mt-2 text-sm text-zinc-500">{detail.author} · {when(detail.date)} · {detail.files.length} 个文件</p>{detail.body && <pre className="mt-4 whitespace-pre-wrap rounded-md bg-zinc-950 p-3 text-xs text-zinc-100">{detail.body}</pre>}</section><div className="mt-6 space-y-4">{detail.files.map((file) => <CommitFileDiff key={`${file.path}-${file.status}`} file={file} />)}</div></div>
}

function CommitFileDiff({ file }: { file: CommitFile }) {
  const color = file.status === "added" ? "text-emerald-700 dark:text-emerald-300" : file.status === "deleted" ? "text-red-700 dark:text-red-300" : "text-zinc-700 dark:text-zinc-300"
  const lines = file.patch.split("\n").filter((line) => (line.startsWith("+") && !line.startsWith("+++")) || (line.startsWith("-") && !line.startsWith("---")))
  return <section className="overflow-hidden rounded-lg border border-zinc-300 bg-white dark:border-zinc-700 dark:bg-zinc-900"><header className="flex items-center justify-between border-b border-zinc-200 bg-zinc-50 px-4 py-2.5 text-sm dark:border-zinc-800 dark:bg-zinc-950"><code className="font-medium">{file.path}</code><span className={`text-xs font-medium ${color}`}>{file.status}</span></header><pre className="max-h-[34rem] overflow-auto bg-zinc-950 p-3 text-xs leading-5 text-zinc-200">{lines.length ? lines.map((line, index) => <code key={index} className={`block px-1 ${line.startsWith("+") ? "bg-emerald-950/80 text-emerald-200" : "bg-red-950/80 text-red-200"}`}>{line}</code>) : <code className="text-zinc-500">二进制文件或无可显示的行级变更。</code>}</pre></section>
}

function ForkList({ name, onOpen }: { name: string; onOpen: (name: string) => void }) {
  const [items, setItems] = useState<Repository[]>([])
  const [message, setMessage] = useState("")
  useEffect(() => { void api<Repository[]>(`/api/repos/${name}/forks`).then(setItems).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取 Fork 列表。")) }, [name])
  if (message) return <PageMessage title="Fork" message={message} />
  return <div className="mx-auto max-w-4xl"><h1 className="mb-5 text-2xl font-semibold">Fork</h1><div className="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">{items.length ? items.map((item) => <button key={item.fullName} onClick={() => onOpen(item.fullName)} className="block w-full border-b border-zinc-100 px-5 py-4 text-left last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800"><strong>{item.fullName}</strong><p className="mt-1 text-sm text-zinc-500">更新于 {when(item.updatedAt)}</p></button>) : <Empty icon={<GitBranch />} title="暂无 Fork" text="还没有从此仓库创建派生。" />}</div></div>
}

function RepositorySettingsPanel({ name, user, onDeleted, onTransferred }: { name: string; user: User | null; onDeleted: () => void; onTransferred: (name: string) => void }) {
  const emptySettings: RepositorySettings = { description: "", homepageUrl: "", visibility: "private", defaultBranch: "main", topics: [], issuesEnabled: true, pullsEnabled: true, releasesEnabled: true, wikiEnabled: false, autoCloseIssues: false, allowForks: true, archived: false }
  const [value, setValue] = useState<RepositorySettings>(emptySettings)
  const [topics, setTopics] = useState("")
  const [labels, setLabels] = useState<Label[]>([])
  const [collaborators, setCollaborators] = useState<Collaborator[]>([])
  const [protections, setProtections] = useState<ProtectedBranch[]>([])
  const [scopes, setScopes] = useState<{ name: string }[]>([])
  const [section, setSection] = useState("general")
  const [message, setMessage] = useState("")
  const [confirm, setConfirm] = useState<"visibility" | "delete" | null>(null)
  const [confirmName, setConfirmName] = useState("")
  const [labelForm, setLabelForm] = useState({ name: "", color: "#0e8a16", description: "" })
  const [collaborator, setCollaborator] = useState({ username: "", permission: "write" })
  const [protection, setProtection] = useState({ branch: "", requirePullRequest: true, requireApprovals: 1 })
  const [targetScope, setTargetScope] = useState("")
  const [newName, setNewName] = useState("")
  const [renameConfirmation, setRenameConfirmation] = useState("")
  const load = async () => {
    const [settings, nextLabels, nextCollaborators, nextProtections, nextScopes] = await Promise.all([
      api<RepositorySettings>(`/api/repos/${name}/settings`), api<Label[]>(`/api/repos/${name}/labels`), api<Collaborator[]>(`/api/repos/${name}/collaborators`), api<ProtectedBranch[]>(`/api/repos/${name}/branch-protections`), user ? api<{ name: string }[]>("/api/scopes") : Promise.resolve([]),
    ])
    setValue(settings); setTopics(settings.topics.join(", ")); setLabels(nextLabels); setCollaborators(nextCollaborators); setProtections(nextProtections); setScopes(nextScopes)
    setTargetScope(nextScopes.find((scope) => scope.name !== name.split("/")[0])?.name || "")
  }
  useEffect(() => { void load().catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载仓库设置。")) }, [name, user])
  const save = async (next = value) => {
    try {
      const saved = await api<RepositorySettings>(`/api/repos/${name}/settings`, { method: "PATCH", body: JSON.stringify({ ...next, topics: topics.split(",").map((item) => item.trim()).filter(Boolean) }) })
      setValue(saved); setMessage("设置已保存。")
    } catch (cause) { setMessage(cause instanceof Error ? cause.message : "保存失败。") }
  }
  const setFeature = (key: keyof RepositorySettings, checked: boolean) => { const next = { ...value, [key]: checked }; setValue(next); void save(next) }
  const nav = [["general", "常规"], ["features", "仓库功能"], ["issues", "议题标签"], ["access", "协作者"], ["branches", "分支保护"], ["lifecycle", "归档与转移"], ["danger", "危险操作"]]
  return <div className="grid gap-7 lg:grid-cols-[190px_minmax(0,1fr)]">
    <nav className="flex gap-1 overflow-x-auto border-b border-zinc-200 pb-3 text-sm lg:block lg:border-b-0 lg:border-r lg:pb-0 lg:pr-5 dark:border-zinc-800">
      {nav.map(([id, label]) => <button key={id} onClick={() => setSection(id)} className={`whitespace-nowrap rounded-lg px-3 py-2 text-left lg:mb-1 lg:block lg:w-full ${section === id ? "bg-zinc-900 font-medium text-white dark:bg-zinc-100 dark:text-zinc-950" : "text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-900"}`}>{label}</button>)}
    </nav>
    <div className="min-w-0">
      {message && <p className="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{message}</p>}
      {section === "general" && <SettingsSection title="常规" description="管理项目展示信息和默认分支。"><form className="space-y-5" onSubmit={(event) => { event.preventDefault(); void save() }}><Field label="仓库简介" value={value.description} onChange={(description) => setValue({ ...value, description })} placeholder="描述这个项目" /><Field label="项目主页" value={value.homepageUrl} onChange={(homepageUrl) => setValue({ ...value, homepageUrl })} placeholder="https://example.com" /><Field label="默认分支" value={value.defaultBranch} onChange={(defaultBranch) => setValue({ ...value, defaultBranch })} placeholder="main" /><Field label="Topics（逗号分隔）" value={topics} onChange={setTopics} placeholder="go, git, forge" /><div className="flex justify-end"><Button type="submit"><Settings />保存更改</Button></div></form></SettingsSection>}
      {section === "features" && <SettingsSection title="仓库功能" description="按需开启工作流模块；关闭后保留历史数据。"><div className="divide-y divide-zinc-100 dark:divide-zinc-800"><SettingToggle label="议题" description="允许创建、讨论和管理议题。" checked={value.issuesEnabled} onChange={(checked) => setFeature("issuesEnabled", checked)} /><SettingToggle label="拉取请求" description="允许创建和审阅拉取请求。" checked={value.pullsEnabled} onChange={(checked) => setFeature("pullsEnabled", checked)} /><SettingToggle label="发布版本" description="允许发布标签说明和附加文件。" checked={value.releasesEnabled} onChange={(checked) => setFeature("releasesEnabled", checked)} /><SettingToggle label="允许派生" description="关闭后，其他用户无法从此仓库创建派生。" checked={value.allowForks} onChange={(checked) => setFeature("allowForks", checked)} /><SettingToggle label="Wiki" description="为仓库预留知识库功能。" checked={value.wikiEnabled} onChange={(checked) => setFeature("wikiEnabled", checked)} /></div></SettingsSection>}
      {section === "issues" && <SettingsSection title="议题" description="维护议题标签，并控制合并拉取请求时的自动关闭行为。"><div className="mb-5 rounded-lg border border-zinc-200 px-4 dark:border-zinc-700"><SettingToggle label="自动关闭关联议题" description="拉取请求合并后，自动关闭描述中引用的议题。" checked={value.autoCloseIssues} onChange={(checked) => setFeature("autoCloseIssues", checked)} /></div><form className="grid gap-2 rounded-lg border border-zinc-200 p-3 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto] dark:border-zinc-700" onSubmit={async (event) => { event.preventDefault(); try { const item = await api<Label>(`/api/repos/${name}/labels`, { method: "POST", body: JSON.stringify(labelForm) }); setLabels([...labels, item]); setLabelForm({ name: "", color: "#0e8a16", description: "" }) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法新建标签。") } }}><input required value={labelForm.name} onChange={(event) => setLabelForm({ ...labelForm, name: event.target.value })} placeholder="标签名称" className="h-9 min-w-0 rounded-md border border-zinc-300 bg-transparent px-2 text-sm dark:border-zinc-700" /><input value={labelForm.color} onChange={(event) => setLabelForm({ ...labelForm, color: event.target.value })} type="color" aria-label="标签颜色" className="size-9 rounded border border-zinc-300 p-1 dark:border-zinc-700" /><input value={labelForm.description} onChange={(event) => setLabelForm({ ...labelForm, description: event.target.value })} placeholder="说明（可选）" className="h-9 min-w-0 rounded-md border border-zinc-300 bg-transparent px-2 text-sm dark:border-zinc-700" /><Button size="sm" type="submit"><Plus />添加</Button></form><div className="mt-4 divide-y divide-zinc-100 rounded-lg border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-700">{labels.length ? labels.map((label) => <div key={label.id} className="flex items-center gap-3 px-4 py-3"><span className="h-3 w-3 rounded-full" style={{ backgroundColor: label.color }} /><div className="min-w-0 flex-1"><p className="font-medium text-sm">{label.name}</p><p className="truncate text-xs text-zinc-500">{label.description || "无说明"}</p></div><Button size="xs" variant="destructive" aria-label={`删除 ${label.name}`} onPress={async () => { try { await api<void>(`/api/repos/${name}/labels/${label.id}`, { method: "DELETE" }); setLabels(labels.filter((item) => item.id !== label.id)) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法删除标签。") } }}><Trash2 /></Button></div>) : <p className="px-4 py-7 text-sm text-zinc-500">还没有议题标签。</p>}</div></SettingsSection>}
      {section === "access" && <SettingsSection title="协作者" description="添加成员并分配只读、写入、维护或管理员权限。"><form className="flex flex-wrap gap-2" onSubmit={async (event) => { event.preventDefault(); try { await api<void>(`/api/repos/${name}/collaborators/${encodeURIComponent(collaborator.username)}`, { method: "PUT", body: JSON.stringify({ permission: collaborator.permission }) }); await load(); setCollaborator({ username: "", permission: "write" }) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法添加协作者。") } }}><input required value={collaborator.username} onChange={(event) => setCollaborator({ ...collaborator, username: event.target.value })} placeholder="用户名" className="h-10 min-w-40 flex-1 rounded-lg border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700" /><select value={collaborator.permission} onChange={(event) => setCollaborator({ ...collaborator, permission: event.target.value })} className="h-10 rounded-lg border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700"><option value="read">只读</option><option value="write">写入</option><option value="maintain">维护</option><option value="admin">管理员</option></select><Button type="submit"><UserPlus />添加</Button></form><div className="mt-5 divide-y divide-zinc-100 rounded-lg border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-700">{collaborators.length ? collaborators.map((item) => <div key={item.username} className="flex items-center gap-3 px-4 py-3"><Avatar name={item.username} /><strong className="min-w-0 flex-1 truncate text-sm">{item.username}</strong><select value={item.permission} onChange={async (event) => { try { await api<void>(`/api/repos/${name}/collaborators/${encodeURIComponent(item.username)}`, { method: "PUT", body: JSON.stringify({ permission: event.target.value }) }); await load() } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法更新权限。") } }} className="h-8 rounded-md border border-zinc-300 bg-transparent px-2 text-xs dark:border-zinc-700"><option value="read">只读</option><option value="write">写入</option><option value="maintain">维护</option><option value="admin">管理员</option></select><Button size="xs" variant="outline" aria-label={`移除 ${item.username}`} onPress={async () => { await api<void>(`/api/repos/${name}/collaborators/${encodeURIComponent(item.username)}`, { method: "DELETE" }); await load() }}><X /></Button></div>) : <p className="px-4 py-7 text-sm text-zinc-500">尚未添加协作者。</p>}</div></SettingsSection>}
      {section === "branches" && <SettingsSection title="分支保护" description="限制关键分支的直接合并，并设置审批要求。"><form className="grid gap-3 rounded-lg border border-zinc-200 p-4 sm:grid-cols-[minmax(0,1fr)_auto_auto] dark:border-zinc-700" onSubmit={async (event) => { event.preventDefault(); try { await api<ProtectedBranch>(`/api/repos/${name}/branch-protections/${encodeURIComponent(protection.branch)}`, { method: "PUT", body: JSON.stringify(protection) }); await load(); setProtection({ branch: "", requirePullRequest: true, requireApprovals: 1 }) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法保护分支。") } }}><input required value={protection.branch} onChange={(event) => setProtection({ ...protection, branch: event.target.value })} placeholder="分支名称，例如 main" className="h-10 rounded-lg border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700" /><label className="flex items-center gap-2 text-sm"><input checked={protection.requirePullRequest} onChange={(event) => setProtection({ ...protection, requirePullRequest: event.target.checked })} type="checkbox" />需要拉取请求</label><label className="flex items-center gap-2 text-sm">审批 <input value={protection.requireApprovals} onChange={(event) => setProtection({ ...protection, requireApprovals: Number(event.target.value) })} type="number" min="0" max="10" className="h-8 w-14 rounded border border-zinc-300 bg-transparent px-2 dark:border-zinc-700" /></label><Button className="sm:col-span-3 sm:justify-self-end" type="submit"><ShieldCheck />保护分支</Button></form><div className="mt-5 divide-y divide-zinc-100 rounded-lg border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-700">{protections.length ? protections.map((item) => <div key={item.branch} className="flex items-center gap-3 px-4 py-3"><ShieldCheck className="size-4 text-emerald-600" /><strong className="min-w-0 flex-1 text-sm">{item.branch}</strong><span className="text-xs text-zinc-500">{item.requirePullRequest ? "需 PR" : "允许直推"} · {item.requireApprovals} 个审批</span><Button size="xs" variant="outline" aria-label={`移除 ${item.branch} 的保护`} onPress={async () => { await api<void>(`/api/repos/${name}/branch-protections/${encodeURIComponent(item.branch)}`, { method: "DELETE" }); await load() }}><X /></Button></div>) : <p className="px-4 py-7 text-sm text-zinc-500">暂无受保护分支。</p>}</div></SettingsSection>}
      {section === "lifecycle" && <SettingsSection title="归档、重命名与转移" description="变更仓库地址前请告知协作者更新远程地址。"><div className="space-y-6"><div className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700"><SettingToggle label="归档仓库" description="归档后保留内容与设置，适用于停止维护的项目。" checked={value.archived} onChange={(checked) => setFeature("archived", checked)} /></div><form className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700" onSubmit={async (event) => { event.preventDefault(); try { const renamed = await api<Repository>(`/api/repos/${name}/rename`, { method: "POST", body: JSON.stringify({ newName, confirmName: renameConfirmation }) }); onTransferred(renamed.fullName) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法重命名仓库。") } }}><h3 className="font-medium">重命名仓库</h3><p className="mt-1 text-sm text-zinc-500">这会迁移 Git 数据、议题、发布版本和所有仓库设置。</p><div className="mt-4 grid gap-2 sm:grid-cols-2"><input required value={newName} onChange={(event) => setNewName(event.target.value)} placeholder="新的仓库名称" className="h-10 rounded-lg border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700" /><input required value={renameConfirmation} onChange={(event) => setRenameConfirmation(event.target.value)} placeholder={`输入 ${name} 确认`} className="h-10 rounded-lg border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700" /></div><div className="mt-3 flex justify-end"><Button type="submit" isDisabled={!newName || renameConfirmation !== name}>重命名仓库</Button></div></form><form className="rounded-lg border border-zinc-200 p-4 dark:border-zinc-700" onSubmit={async (event) => { event.preventDefault(); try { const moved = await api<Repository>(`/api/repos/${name}/transfer`, { method: "POST", body: JSON.stringify({ targetScope }) }); onTransferred(moved.fullName) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法转移仓库。") } }}><h3 className="font-medium">转移仓库</h3><p className="mt-1 text-sm text-zinc-500">目标空间会成为仓库的新归属。</p><div className="mt-4 flex flex-wrap gap-2"><select value={targetScope} onChange={(event) => setTargetScope(event.target.value)} className="h-10 min-w-40 flex-1 rounded-lg border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700">{scopes.filter((scope) => scope.name !== name.split("/")[0]).map((scope) => <option key={scope.name} value={scope.name}>{scope.name}</option>)}</select><Button type="submit" isDisabled={!targetScope}>转移仓库</Button></div></form></div></SettingsSection>}
      {section === "danger" && <SettingsSection title="危险操作" description="这些操作会影响仓库的访问范围或永久移除数据。"><div className="space-y-4"><div className="rounded-lg border border-amber-200 bg-amber-50/50 p-4 dark:border-amber-900 dark:bg-amber-950/20"><h3 className="font-medium">修改可见性</h3><p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">当前为{value.visibility === "public" ? "公开" : "私有"}仓库。变更前需要确认仓库名称。</p><Button className="mt-4" variant="outline" onPress={() => { setConfirm("visibility"); setConfirmName("") }}>改为{value.visibility === "public" ? "私有" : "公开"}</Button></div><div className="rounded-lg border border-red-200 bg-red-50/50 p-4 dark:border-red-900 dark:bg-red-950/20"><h3 className="font-medium text-red-800 dark:text-red-300">删除仓库</h3><p className="mt-1 text-sm text-zinc-600 dark:text-zinc-400">永久删除 Git 数据、议题、发布版本与配置，无法恢复。</p><Button className="mt-4" variant="destructive" onPress={() => { setConfirm("delete"); setConfirmName("") }}><Trash2 />删除仓库</Button></div></div></SettingsSection>}
    </div>
    {confirm && <Modal title={confirm === "delete" ? "删除仓库" : "修改仓库可见性"} onClose={() => setConfirm(null)}><p className="text-sm text-zinc-500">请输入 <strong className="text-zinc-800 dark:text-zinc-200">{name}</strong> 以确认此操作。</p><input autoFocus value={confirmName} onChange={(event) => setConfirmName(event.target.value)} className="mt-4 h-10 w-full rounded-lg border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700" placeholder={name} /><div className="mt-5 flex justify-end gap-2"><Button variant="outline" onPress={() => setConfirm(null)}>取消</Button><Button variant={confirm === "delete" ? "destructive" : "default"} isDisabled={confirmName !== name} onPress={async () => { try { if (confirm === "delete") { await api<void>(`/api/repos/${name}`, { method: "DELETE", body: JSON.stringify({ confirmName }) }); onDeleted() } else { const next = await api<RepositorySettings>(`/api/repos/${name}/visibility`, { method: "PUT", body: JSON.stringify({ visibility: value.visibility === "public" ? "private" : "public", confirmName }) }); setValue(next); setConfirm(null); setMessage("仓库可见性已更新。") } } catch (cause) { setMessage(cause instanceof Error ? cause.message : "操作失败。") } }}>{confirm === "delete" ? "永久删除" : "确认修改"}</Button></div></Modal>}
  </div>
}

function SettingsSection({ title, description, children }: { title: string; description: string; children: ReactNode }) { return <section className="max-w-3xl rounded-lg border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"><header className="mb-5 border-b border-zinc-100 pb-4 dark:border-zinc-800"><h2 className="text-lg font-semibold">{title}</h2><p className="mt-1 text-sm text-zinc-500">{description}</p></header>{children}</section> }
function SettingToggle({ label, description, checked, onChange }: { label: string; description: string; checked: boolean; onChange: (checked: boolean) => void }) { return <label className="flex cursor-pointer items-center gap-4 py-4"><span className="min-w-0 flex-1"><span className="block text-sm font-medium">{label}</span><span className="mt-1 block text-sm text-zinc-500">{description}</span></span><input className="size-4 accent-emerald-600" type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} /></label> }

function LabelBadges({ labels = [] }: { labels?: Label[] }) {
  if (!labels.length) return null
  return <span className="ml-2 inline-flex flex-wrap gap-1 align-middle">{labels.map((label) => <span key={label.id} title={label.description} className="rounded-full px-2 py-0.5 text-xs font-medium" style={{ backgroundColor: `${label.color}22`, color: label.color }}>{label.name}</span>)}</span>
}

function IssueComposer({ name, user, onCancel, onCreated }: { name: string; user: User | null; onCancel: () => void; onCreated: () => void }) {
  const [title, setTitle] = useState("")
  const [body, setBody] = useState("")
  const [labels, setLabels] = useState<Label[]>([])
  const [selected, setSelected] = useState<number[]>([])
  const [preview, setPreview] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState("")
  useEffect(() => { void api<Label[]>(`/api/repos/${name}/labels`).then(setLabels).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载标签。")) }, [name])
  const toggle = (id: number) => setSelected(selected.includes(id) ? selected.filter((value) => value !== id) : [...selected, id])
  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_250px]">
      <form className="min-w-0" onSubmit={async (event) => {
        event.preventDefault()
        if (!user) { setMessage("请先登录再创建议题。 "); return }
        setSaving(true); setMessage("")
        try { await api(`/api/repos/${name}/issues`, { method: "POST", body: JSON.stringify({ title, body, labelIds: selected }) }); onCreated() } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法创建议题。") } finally { setSaving(false) }
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
        <div className="py-5"><p className="font-medium">标签</p><div className="mt-3 space-y-2">{labels.length ? labels.map((label) => <label key={label.id} className="flex cursor-pointer items-center gap-2"><input type="checkbox" checked={selected.includes(label.id)} onChange={() => toggle(label.id)} /><span className="rounded-full px-2 py-0.5 text-xs font-medium" style={{ backgroundColor: `${label.color}22`, color: label.color }}>{label.name}</span></label>) : <p className="text-zinc-500">暂无可选标签</p>}</div></div>
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
  const [labels, setLabels] = useState<Label[]>([])
  const [editingLabels, setEditingLabels] = useState(false)
  const [newLabelName, setNewLabelName] = useState("")
  const [newLabelColor, setNewLabelColor] = useState("#0e8a16")
  const [body, setBody] = useState("")
  const [message, setMessage] = useState("")
  const [saving, setSaving] = useState(false)
  const load = async () => {
    const [nextIssue, nextComments, nextLabels] = await Promise.all([
      api<Issue>(`/api/repos/${name}/issues/${issueID}`),
      api<IssueComment[]>(`/api/repos/${name}/issues/${issueID}/comments`),
      api<Label[]>(`/api/repos/${name}/labels`),
    ])
    setIssue({ ...nextIssue, labels: nextIssue.labels || [] })
    setComments(nextComments)
    setLabels(nextLabels)
  }
  useEffect(() => { void load().catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载议题。")) }, [name, issueID])
  if (!issue && !message) return <Loading />
  if (!issue) return <PageMessage title="议题" message={message} />
  const updateState = (state: string) => { onStateChange(state); setIssue({ ...issue, state }) }
  const saveLabels = async (ids: number[]) => { try { await api<void>(`/api/repos/${name}/issues/${issueID}/labels`, { method: "PUT", body: JSON.stringify({ labelIds: ids }) }); setIssue({ ...issue, labels: labels.filter((label) => ids.includes(label.id)) }); setEditingLabels(false) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法更新标签。") } }
  const createLabel = async () => { try { const item = await api<Label>(`/api/repos/${name}/labels`, { method: "POST", body: JSON.stringify({ name: newLabelName, color: newLabelColor, description: "" }) }); setLabels([...labels, item]); setNewLabelName(""); await saveLabels([...issue.labels.map((label) => label.id), item.id]) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法创建标签。") } }
  return <div className="mx-auto max-w-5xl">
    <button onClick={onBack} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />所有议题</button>
    <div className="border-b border-zinc-200 pb-5 dark:border-zinc-800"><h1 className="text-2xl font-semibold leading-tight">{issue.title} <span className="font-normal text-zinc-400">#{issue.id}</span></h1><div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-zinc-500"><Badge state={issue.state} /><LabelBadges labels={issue.labels} /><span><strong className="font-medium text-zinc-700 dark:text-zinc-300">{issue.author}</strong> 创建于 {when(issue.createdAt)}</span></div></div>
    <div className="grid gap-6 py-6 lg:grid-cols-[minmax(0,1fr)_210px]">
      <div className="space-y-5">
        <article className="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"><header className="flex items-center gap-2 border-b border-zinc-100 bg-zinc-50 px-4 py-2.5 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950"><span className="grid size-6 place-items-center rounded-full bg-zinc-900 text-[10px] font-semibold text-white dark:bg-zinc-100 dark:text-zinc-950">{issue.author.slice(0, 1).toUpperCase()}</span><strong className="font-medium text-zinc-700 dark:text-zinc-300">{issue.author}</strong><span>创建于 {when(issue.createdAt)}</span></header><div className="min-h-24 whitespace-pre-wrap p-4 text-sm leading-6">{issue.body || <span className="text-zinc-400">未提供描述。</span>}</div></article>
        {comments.map((comment) => <article key={comment.id} className="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"><header className="flex items-center gap-2 border-b border-zinc-100 bg-zinc-50 px-4 py-2.5 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950"><span className="grid size-6 place-items-center rounded-full bg-emerald-100 text-[10px] font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">{comment.author.slice(0, 1).toUpperCase()}</span><strong className="font-medium text-zinc-700 dark:text-zinc-300">{comment.author}</strong><span>commented {when(comment.createdAt)}</span></header><div className="whitespace-pre-wrap p-4 text-sm leading-6">{comment.body}</div></article>)}
        {user ? <form className="rounded-lg border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900" onSubmit={async (event) => { event.preventDefault(); if (!body.trim()) return; setSaving(true); setMessage(""); try { const comment = await api<IssueComment>(`/api/repos/${name}/issues/${issueID}/comments`, { method: "POST", body: JSON.stringify({ body }) }); setComments([...comments, comment]); setBody("") } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法发布评论。") } finally { setSaving(false) } }}><textarea value={body} onChange={(event) => setBody(event.target.value)} placeholder="留下评论" className="min-h-28 w-full resize-y bg-transparent p-2 text-sm leading-6 outline-none" />{message && <p className="px-2 pb-2 text-sm text-red-600">{message}</p>}<div className="flex justify-end border-t border-zinc-100 pt-3 dark:border-zinc-800"><Button type="submit" isDisabled={saving || !body.trim()}><MessageSquare />{saving ? "正在评论..." : "评论"}</Button></div></form> : <p className="rounded-lg border border-dashed border-zinc-300 p-4 text-sm text-zinc-500 dark:border-zinc-700">登录后即可参与讨论。</p>}
      </div>
      <aside className="border-t border-zinc-200 text-sm dark:border-zinc-800 lg:border-t-0 lg:border-l lg:pl-5"><IssueMeta label="受理人" value="未分配" /><div className="py-5"><div className="flex items-center justify-between"><p className="font-medium">标签</p>{user && <button onClick={() => setEditingLabels(!editingLabels)} className="text-xs text-sky-700 hover:underline dark:text-sky-300">{editingLabels ? "取消" : "编辑"}</button>}</div>{editingLabels ? <div className="mt-3 space-y-2">{labels.map((label) => <label key={label.id} className="flex items-center gap-2"><input type="checkbox" checked={issue.labels.some((item) => item.id === label.id)} onChange={(event) => { const ids = issue.labels.map((item) => item.id); void saveLabels(event.target.checked ? [...ids, label.id] : ids.filter((id) => id !== label.id)) }} /><span className="rounded-full px-2 py-0.5 text-xs font-medium" style={{ backgroundColor: `${label.color}22`, color: label.color }}>{label.name}</span></label>)}<div className="flex gap-2 pt-2"><input value={newLabelName} onChange={(event) => setNewLabelName(event.target.value)} placeholder="新标签" className="h-8 min-w-0 flex-1 rounded-md border border-zinc-300 bg-transparent px-2 text-xs dark:border-zinc-700"/><input value={newLabelColor} onChange={(event) => setNewLabelColor(event.target.value)} type="color" aria-label="标签颜色" className="size-8 rounded border border-zinc-300 p-1 dark:border-zinc-700"/><Button size="xs" variant="outline" isDisabled={!newLabelName.trim()} onPress={() => void createLabel()}>新建</Button></div></div> : <div className="mt-2"><LabelBadges labels={issue.labels} />{!issue.labels.length && <p className="text-zinc-500">暂无标签</p>}</div>}</div><IssueMeta label="项目" value="暂无项目" />{user && <div className="py-5"><p className="font-medium">状态</p><Button size="sm" variant="outline" className="mt-3" onPress={() => updateState(issue.state === "open" ? "closed" : "open")}>{issue.state === "open" ? "关闭议题" : "重新打开议题"}</Button></div>}</aside>
    </div>
  </div>
}

function fileSize(size: number) {
  return size < 1024 * 1024 ? `${Math.max(1, Math.round(size / 1024))} KB` : `${(size / 1024 / 1024).toFixed(1)} MB`
}

function ReleaseList({ name, user, releases, onDelete, onChanged, onOpen }: { name: string; user: User | null; releases: Release[]; onDelete: (id: number) => void; onChanged: () => void; onOpen: (id: number) => void }) {
  const [open, setOpen] = useState(false)
  const [tags, setTags] = useState<GitRef[]>([])
  const [branches, setBranches] = useState<GitRef[]>([])
  const [existingTag, setExistingTag] = useState("")
  const [createTag, setCreateTag] = useState(false)
  const [tagName, setTagName] = useState("")
  const [targetRef, setTargetRef] = useState("")
  const [title, setTitle] = useState("")
  const [notes, setNotes] = useState("")
  const [files, setFiles] = useState<File[]>([])
  const [message, setMessage] = useState("")
  const [saving, setSaving] = useState(false)
  useEffect(() => { void Promise.all([api<GitRef[]>(`/api/repos/${name}/tags`), api<GitRef[]>(`/api/repos/${name}/branches`)]).then(([nextTags, nextBranches]) => { setTags(nextTags); setBranches(nextBranches); setExistingTag(nextTags[0]?.name || ""); setTargetRef(nextBranches[0]?.name || "HEAD") }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取标签和分支。")) }, [name])
  const publish = async (event: FormEvent) => {
    event.preventDefault()
    const selectedTag = createTag ? tagName.trim() : existingTag
    if (!selectedTag) { setMessage("请选择已有标签，或输入新标签名称。 "); return }
    setSaving(true); setMessage("")
    try {
      const release = await api<Release>(`/api/repos/${name}/releases`, { method: "POST", body: JSON.stringify({ tagName: selectedTag, title: title.trim() || selectedTag, notes, createTag, targetRef }) })
      for (const file of files) {
        const form = new FormData(); form.append("asset", file)
        const response = await fetch(`/api/repos/${name}/releases/${release.id}/assets`, { method: "POST", body: form })
        if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error || `无法上传 ${file.name}`) }
      }
      setOpen(false); setTitle(""); setNotes(""); setTagName(""); setFiles([]); onChanged()
    } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法发布版本。") }
    finally { setSaving(false) }
  }
  return <>
    <div className="mb-5 flex flex-wrap items-center justify-between gap-3"><div><h1 className="text-2xl font-semibold">发布版本</h1><p className="mt-1 text-sm text-zinc-500">为已有标签发布说明和可下载文件。</p></div>{user && <Button onPress={() => setOpen(true)}><Tag />新建发布版本</Button>}</div>
    {message && <p className="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950/50">{message}</p>}
    <div className="overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900">{releases.length ? releases.map((release) => <article key={release.id} className="border-b border-zinc-200 px-5 py-5 last:border-0 dark:border-zinc-800"><button onClick={() => onOpen(release.id)} className="block text-left hover:text-sky-700 dark:hover:text-sky-300"><div className="flex flex-wrap items-center gap-2"><Tag className="size-4 text-emerald-600" /><strong>{release.title}</strong><code className="rounded bg-zinc-100 px-1.5 py-0.5 text-xs text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300">{release.tagName}</code></div>{release.notes && <p className="mt-3 line-clamp-2 whitespace-pre-wrap text-sm leading-6 text-zinc-700 dark:text-zinc-300">{release.notes}</p>}</button><p className="mt-3 text-xs text-zinc-500">由 {release.author} 发布于 {when(release.createdAt)}</p>{user && <div className="mt-4"><Button size="xs" variant="destructive" onPress={() => onDelete(release.id)}>删除发布版本</Button></div>}</article>) : <Empty icon={<Tag />} title="暂无发布版本" text={user ? "选择标签后发布第一个版本。" : "登录后可发布版本。"} />}</div>
    {open && <Modal title="新建发布版本" onClose={() => setOpen(false)}><form className="space-y-4" onSubmit={publish}><div className="flex gap-4 text-sm"><label className="flex items-center gap-2"><input type="radio" checked={!createTag} onChange={() => setCreateTag(false)} />已有标签</label><label className="flex items-center gap-2"><input type="radio" checked={createTag} onChange={() => setCreateTag(true)} />新建标签</label></div>{createTag ? <><label className="block text-sm font-medium">标签名称<input required value={tagName} onChange={(event) => setTagName(event.target.value)} placeholder="v1.0.0" className="mt-1.5 h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700" /></label><label className="block text-sm font-medium">目标分支或提交<select value={targetRef} onChange={(event) => setTargetRef(event.target.value)} className="mt-1.5 h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700">{branches.length ? branches.map((branch) => <option key={branch.name} value={branch.name}>{branch.name}</option>) : <option value="HEAD">当前 HEAD</option>}</select></label></> : <label className="block text-sm font-medium">已有标签<select required value={existingTag} onChange={(event) => setExistingTag(event.target.value)} className="mt-1.5 h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700">{tags.length ? tags.map((tag) => <option key={tag.name} value={tag.name}>{tag.name}</option>) : <option value="">暂无标签，请创建新标签</option>}</select></label>}<label className="block text-sm font-medium">发布标题<input value={title} onChange={(event) => setTitle(event.target.value)} placeholder={createTag ? tagName || "v1.0.0" : existingTag} className="mt-1.5 h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700" /></label><label className="block text-sm font-medium">发布说明<textarea value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="说明此版本的更新内容" className="mt-1.5 min-h-28 w-full rounded-lg border border-zinc-200 bg-transparent p-3 text-sm dark:border-zinc-700" /></label><label className="block text-sm font-medium">发布文件<input type="file" multiple onChange={(event) => setFiles(Array.from(event.target.files || []))} className="mt-1.5 block w-full text-sm" /></label>{files.length > 0 && <p className="text-xs text-zinc-500">已选择 {files.length} 个文件</p>}<div className="flex justify-end gap-2"><Button type="button" variant="outline" onPress={() => setOpen(false)}>取消</Button><Button type="submit" isPending={saving}>{saving ? "正在发布..." : "发布版本"}</Button></div></form></Modal>}
  </>
}

function ReleaseDetailPage({ name, releaseID, onBack }: { name: string; releaseID: number; onBack: () => void }) {
  const [release, setRelease] = useState<Release | null>(null)
  const [message, setMessage] = useState("")
  useEffect(() => { void api<Release[]>(`/api/repos/${name}/releases`).then((items) => { const item = items.find((value) => value.id === releaseID); if (!item) throw new Error("未找到发布版本。"); setRelease(item) }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取发布版本。")) }, [name, releaseID])
  if (message) return <PageMessage title="发布版本" message={message} />
  if (!release) return <Loading />
  const sourceURL = (format: "zip" | "tar.gz") => `/api/repos/${name}/archive?ref=${encodeURIComponent(release.tagName)}&format=${format}`
  return <div className="mx-auto max-w-5xl"><button onClick={onBack} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />所有发布版本</button><article className="rounded-lg border border-zinc-200 bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"><div className="flex flex-wrap items-center gap-2"><Tag className="size-5 text-emerald-600" /><h1 className="text-2xl font-semibold">{release.title}</h1><code className="rounded bg-zinc-100 px-2 py-1 text-xs dark:bg-zinc-800">{release.tagName}</code></div><p className="mt-3 text-sm text-zinc-500">由 {release.author} 发布于 {when(release.createdAt)}</p>{release.notes && <div className="mt-6 whitespace-pre-wrap text-sm leading-7 text-zinc-700 dark:text-zinc-300">{release.notes}</div>}<div className="mt-7 border-t border-zinc-200 pt-5 dark:border-zinc-800"><h2 className="font-semibold">Assets</h2><div className="mt-3 space-y-2">{release.assets.map((asset) => <a key={asset.id} href={asset.url} className="flex items-center gap-2 text-sm text-sky-700 hover:underline dark:text-sky-300"><Upload className="size-4" />{asset.fileName}<span className="text-xs text-zinc-500">{fileSize(asset.size)}</span></a>)}<a href={sourceURL("zip")} className="flex items-center gap-2 text-sm text-sky-700 hover:underline dark:text-sky-300"><Upload className="size-4" />Source code (zip)</a><a href={sourceURL("tar.gz")} className="flex items-center gap-2 text-sm text-sky-700 hover:underline dark:text-sky-300"><Upload className="size-4" />Source code (tar.gz)</a></div></div></article></div>
}

function PullRequestList({ items, user, onNew, onDelete, onStateChange, onOpen }: { items: PullRequest[]; user: User | null; onNew: () => void; onDelete: (id: number) => void; onStateChange: (id: number, state: string) => void; onOpen: (id: number) => void }) {
  const [filter, setFilter] = useState("")
  const [stateFilter, setStateFilter] = useState<"all" | "open" | "closed" | "merged">("all")
  const visible = items.filter((item) => `${item.title} ${item.author} ${item.state}`.toLowerCase().includes(filter.toLowerCase()) && (stateFilter === "all" || item.state === stateFilter))
  return <>
    <div className="mb-5 flex flex-wrap items-center justify-between gap-3"><h1 className="text-2xl font-semibold">拉取请求</h1><Button onPress={onNew} isDisabled={!user}><GitPullRequest /> 新建拉取请求</Button></div>
    <div className="mb-4 space-y-3 rounded-lg border border-zinc-300 bg-white p-3 shadow-sm dark:border-zinc-700 dark:bg-zinc-900"><div className="flex items-center"><span className="px-1 text-zinc-400"><ListFilter className="size-4" /></span><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="筛选拉取请求" className="h-8 min-w-0 flex-1 bg-transparent px-2 text-sm outline-none" /><span className="text-xs text-zinc-500">{visible.length} 项结果</span></div><div className="flex flex-wrap gap-2">{(["all", "open", "closed", "merged"] as const).map((state) => <button key={state} onClick={() => setStateFilter(state)} className={`rounded-full border px-3 py-1 text-xs ${stateFilter === state ? "border-emerald-600 bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300" : "border-zinc-200 text-zinc-600 dark:border-zinc-700 dark:text-zinc-400"}`}>{state === "all" ? "全部" : state === "open" ? "开启" : state === "closed" ? "已关闭" : "已合并"}</button>)}</div></div>
    <div className="overflow-hidden rounded-lg border border-zinc-300 bg-white shadow-sm dark:border-zinc-700 dark:bg-zinc-900">{visible.length ? visible.map((item) => <article key={item.id} className="border-b border-zinc-200 px-5 py-4 last:border-0 dark:border-zinc-800"><button onClick={() => onOpen(item.id)} className="block text-left hover:text-sky-700 dark:hover:text-sky-300"><div className="flex flex-wrap items-center gap-x-2 gap-y-1"><Badge state={item.state} /><strong>#{item.id} {item.title}</strong></div><p className="mt-2 text-sm text-zinc-500"><code>{item.sourceBranch}</code> 合并到 <code>{item.targetBranch}</code> · {item.author} 创建于 {when(item.createdAt)}</p></button>{user && <div className="mt-3 flex gap-2"><Button size="xs" variant="outline" onPress={() => onStateChange(item.id, item.state === "open" ? "closed" : "open")}>{item.state === "open" ? "关闭" : "重新打开"}</Button><Button size="xs" variant="destructive" onPress={() => onDelete(item.id)}>删除</Button></div>}</article>) : <Empty icon={<GitPullRequest />} title="欢迎使用拉取请求" text={user ? "比较两个分支以开始创建拉取请求。" : "登录后即可创建拉取请求。"} />}</div>
  </>
}

function PullRequestDetailPage({ name, pullID, user, onBack }: { name: string; pullID: number; user: User | null; onBack: () => void }) {
  const [pull, setPull] = useState<PullRequest | null>(null)
  const [comments, setComments] = useState<PullRequestComment[]>([])
  const [files, setFiles] = useState<CommitFile[]>([])
  const [section, setSection] = useState<"conversation" | "files">("conversation")
  const [body, setBody] = useState("")
  const [message, setMessage] = useState("")
  const [saving, setSaving] = useState(false)
  const load = async () => { const [nextPull, nextComments, nextFiles] = await Promise.all([api<PullRequest>(`/api/repos/${name}/pulls/${pullID}`), api<PullRequestComment[]>(`/api/repos/${name}/pulls/${pullID}/comments`), api<CommitFile[]>(`/api/repos/${name}/pulls/${pullID}/files`)]); setPull(nextPull); setComments(nextComments); setFiles(nextFiles) }
  useEffect(() => { void load().catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法读取拉取请求。")) }, [name, pullID])
  if (message && !pull) return <PageMessage title="拉取请求" message={message} />
  if (!pull) return <Loading />
  const merge = async () => { setSaving(true); setMessage(""); try { await api(`/api/repos/${name}/pulls/${pullID}/merge`, { method: "POST" }); await load(); setMessage("拉取请求已合并。") } catch (cause) { setMessage(cause instanceof Error ? cause.message : "合并失败。") } finally { setSaving(false) } }
  return <div className="mx-auto max-w-6xl"><button onClick={onBack} className="mb-5 inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-900 dark:hover:text-zinc-100"><ArrowLeft className="size-4" />所有拉取请求</button><header className="border-b border-zinc-200 pb-5 dark:border-zinc-800"><h1 className="text-2xl font-semibold">{pull.title} <span className="font-normal text-zinc-400">#{pull.id}</span></h1><div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-zinc-500"><Badge state={pull.state} /><span><code className="rounded bg-sky-50 px-1.5 py-0.5 text-sky-700 dark:bg-sky-950 dark:text-sky-300">{pull.sourceBranch}</code> 请求合并到 <code className="rounded bg-sky-50 px-1.5 py-0.5 text-sky-700 dark:bg-sky-950 dark:text-sky-300">{pull.targetBranch}</code></span><span>由 {pull.author} 创建于 {when(pull.createdAt)}</span></div></header><div className="mt-5 flex gap-1 border-b border-zinc-200 dark:border-zinc-800"><button onClick={() => setSection("conversation")} className={`border-b-2 px-4 py-3 text-sm ${section === "conversation" ? "border-emerald-600 font-medium" : "border-transparent text-zinc-500"}`}>会话 {comments.length}</button><button onClick={() => setSection("files")} className={`border-b-2 px-4 py-3 text-sm ${section === "files" ? "border-emerald-600 font-medium" : "border-transparent text-zinc-500"}`}>文件变更 {files.length}</button></div>{message && <p className="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{message}</p>}{section === "conversation" ? <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,1fr)_230px]"><div className="space-y-5"><article className="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"><header className="border-b border-zinc-100 bg-zinc-50 px-4 py-2.5 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950"><strong className="text-zinc-700 dark:text-zinc-300">{pull.author}</strong> 创建了此拉取请求</header><div className="min-h-24 whitespace-pre-wrap p-4 text-sm leading-6">{pull.body || <span className="text-zinc-400">未提供说明。</span>}</div></article>{comments.map((comment) => <article key={comment.id} className="overflow-hidden rounded-lg border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900"><header className="border-b border-zinc-100 bg-zinc-50 px-4 py-2.5 text-sm text-zinc-500 dark:border-zinc-800 dark:bg-zinc-950"><strong className="text-zinc-700 dark:text-zinc-300">{comment.author}</strong> 评论于 {when(comment.createdAt)}</header><div className="whitespace-pre-wrap p-4 text-sm leading-6">{comment.body}</div></article>)}{user && pull.state === "open" && <form className="rounded-lg border border-zinc-200 bg-white p-3 dark:border-zinc-800 dark:bg-zinc-900" onSubmit={async (event) => { event.preventDefault(); if (!body.trim()) return; setSaving(true); try { const comment = await api<PullRequestComment>(`/api/repos/${name}/pulls/${pullID}/comments`, { method: "POST", body: JSON.stringify({ body }) }); setComments([...comments, comment]); setBody("") } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法发布评论。") } finally { setSaving(false) } }}><textarea value={body} onChange={(event) => setBody(event.target.value)} placeholder="留下评论" className="min-h-28 w-full resize-y bg-transparent p-2 text-sm outline-none" /><div className="flex justify-end border-t border-zinc-100 pt-3 dark:border-zinc-800"><Button type="submit" isDisabled={saving || !body.trim()}><MessageSquare />评论</Button></div></form>}</div><aside className="rounded-lg border border-zinc-200 bg-white p-4 dark:border-zinc-800 dark:bg-zinc-900"><h2 className="font-semibold">合并</h2><p className="mt-2 text-sm leading-6 text-zinc-500">将变更以合并提交写入 <code>{pull.targetBranch}</code>。</p>{pull.state === "open" ? <Button className="mt-4 w-full" isDisabled={!user || saving} onPress={merge}><GitPullRequest />合并拉取请求</Button> : <p className="mt-4 text-sm text-zinc-500">此拉取请求已{pull.state === "merged" ? "合并" : "关闭"}。</p>}</aside></div> : <div className="mt-6 space-y-4">{files.length ? files.map((file) => <CommitFileDiff key={`${file.path}-${file.status}`} file={file} />) : <Empty icon={<GitPullRequest />} title="没有文件变更" text="两个分支之间没有可显示的变更。" />}</div>}</div>
}

function CompareChanges({ name, user, onCancel, onCreated }: { name: string; user: User | null; onCancel: () => void; onCreated: (value: Record<string, string>) => Promise<void> }) {
  const location = useLocation()
  const sourceRepo = new URLSearchParams(location.search).get("sourceRepo") || ""
  const [branches, setBranches] = useState<GitRef[]>([])
  const [sourceBranches, setSourceBranches] = useState<GitRef[]>([])
  const [base, setBase] = useState("main")
  const [compare, setCompare] = useState("")
  const [title, setTitle] = useState("")
  const [body, setBody] = useState("")
  const [message, setMessage] = useState("")
  const [saving, setSaving] = useState(false)
  useEffect(() => { void Promise.all([api<GitRef[]>(`/api/repos/${name}/branches`), sourceRepo ? api<GitRef[]>(`/api/repos/${sourceRepo}/branches`) : Promise.resolve([])]).then(([items, sourceItems]) => { setBranches(items); setSourceBranches(sourceItems); if (items[0]) setBase(items.find((item) => item.name === "main")?.name || items[0].name); const compareItems = sourceRepo ? sourceItems : items; if (compareItems[0]) setCompare(compareItems.find((item) => item.name === "main")?.name || compareItems[1]?.name || compareItems[0].name) }).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "无法加载分支。")) }, [name, sourceRepo])
  const canCreate = Boolean(user && compare && (sourceRepo || compare !== base) && title.trim())
  return <div className="mx-auto max-w-5xl"><div className="mb-2 flex items-center gap-2 text-sm text-zinc-500"><GitCompareArrows className="size-4" /> 比较变更</div><h1 className="text-2xl font-semibold">{sourceRepo ? "向上游发起拉取请求" : "比较分支"}</h1><p className="mt-2 text-sm text-zinc-500">{sourceRepo ? `将 ${sourceRepo} 的变更提交到 ${name}。` : "选择基准分支和包含变更的分支。"}</p><div className="mt-6 flex flex-wrap items-center gap-3 rounded-lg border border-zinc-300 bg-zinc-50 p-4 dark:border-zinc-700 dark:bg-zinc-900"><BranchSelect label="基准" value={base} branches={branches} onChange={setBase} /><ArrowLeft className="size-4 text-zinc-400" /><BranchSelect label={sourceRepo ? `${sourceRepo} 分支` : "比较"} value={compare} branches={sourceRepo ? sourceBranches : branches} onChange={setCompare} /></div>{message && <p className="mt-4 text-sm text-red-600">{message}</p>}{compare && compare === base && !sourceRepo && <p className="mt-5 rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-200">请选择不同的分支或派生仓库进行比较。</p>}<form className="mt-6 space-y-4" onSubmit={async (event) => { event.preventDefault(); if (!canCreate) return; setSaving(true); setMessage(""); try { await onCreated({ title, body, sourceBranch: sourceRepo ? `${sourceRepo}:${compare}` : compare, targetBranch: base }) } catch (cause) { setMessage(cause instanceof Error ? cause.message : "无法创建拉取请求。") } finally { setSaving(false) } }}><input required value={title} onChange={(event) => setTitle(event.target.value)} placeholder="拉取请求标题" className="h-11 w-full rounded-lg border border-zinc-300 bg-transparent px-3 text-sm outline-none focus:border-sky-600 dark:border-zinc-700" /><textarea required value={body} onChange={(event) => setBody(event.target.value)} placeholder="描述你的变更" className="min-h-36 w-full rounded-lg border border-zinc-300 bg-transparent p-3 text-sm outline-none focus:border-sky-600 dark:border-zinc-700" /><div className="flex justify-end gap-3"><Button type="button" variant="outline" onPress={onCancel}>取消</Button><Button type="submit" isDisabled={!canCreate || saving}>{saving ? "正在创建..." : "创建拉取请求"}</Button></div></form></div>
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
  const [stateFilter, setStateFilter] = useState("all")
  const [filter, setFilter] = useState("")
  const visibleItems = items.filter((item) => {
    const value = item as T & { title?: string; author?: string; state?: string }
    const needle = filter.trim().toLowerCase()
    return (!needle || `${value.title || ""} ${value.author || ""} ${value.state || ""}`.toLowerCase().includes(needle)) && (stateFilter === "all" || value.state === stateFilter)
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
      <div className="mb-3 rounded-xl border border-zinc-200 bg-zinc-50 p-3 dark:border-zinc-800 dark:bg-zinc-950"><div className="flex flex-wrap items-center gap-3"><input value={filter} onChange={(event) => setFilter(event.target.value)} placeholder={`筛选${title}（标题、作者或状态）`} className="h-8 min-w-52 flex-1 bg-transparent px-2 text-sm outline-none"/><span className="text-xs text-zinc-500">{visibleItems.length} 项结果</span></div>{items.some((item) => "state" in item) && <div className="mt-3 flex flex-wrap gap-2">{["all", "open", "closed"].map((value) => <button key={value} onClick={() => setStateFilter(value)} className={`rounded-full border px-3 py-1 text-xs ${stateFilter === value ? "border-emerald-600 bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300" : "border-zinc-200 text-zinc-600 dark:border-zinc-700 dark:text-zinc-400"}`}>{value === "all" ? "全部" : value === "open" ? "开启" : "已关闭"}</button>)}</div>}</div>
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
