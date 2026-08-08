import {
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react"
import {
  Check,
  Copy,
  FolderGit2,
  GitBranch,
  GitPullRequest,
  HeartHandshake,
  KeyRound,
  LogIn,
  LogOut,
  Megaphone,
  Plus,
  Settings,
  Tag,
  UserPlus,
  Users,
  X,
} from "lucide-react"

import { Button } from "@/components/ui/button"

type User = { id: number; username: string; email: string; isAdmin: boolean }
type SiteSettings = {
  title: string
  description: string
  allowRegistration: boolean
}
type Repository = {
  scope: string
  name: string
  fullName: string
  updatedAt: string
}
type TreeEntry = { name: string; path: string; type: string }
type Blob = { path: string; content: string; isText: boolean }
type GitRef = { name: string; hash: string }
type Commit = { hash: string; subject: string; author: string; date: string }
type RepositorySettings = {
  description: string
  visibility: "public" | "private"
  defaultBranch: string
  topics: string[]
}
type CaptchaConfig = { enabled: boolean; siteKey: string }
declare global {
  interface Window {
    hcaptcha?: {
      render: (element: HTMLElement, options: Record<string, unknown>) => void
    }
  }
}
type Issue = {
  id: number
  title: string
  body: string
  state: string
  author: string
  createdAt: string
}
type PullRequest = {
  id: number
  title: string
  body: string
  sourceBranch: string
  targetBranch: string
  state: string
  author: string
  createdAt: string
}
type Release = {
  id: number
  tagName: string
  title: string
  notes: string
  author: string
  createdAt: string
}
type Contributor = { username: string; contributions: number }

async function api<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error || "Request failed.")
  }
  return response.status === 204 ? (undefined as T) : response.json()
}

const when = (date: string) =>
  new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(
    new Date(date)
  )

export default function App() {
  const [site, setSite] = useState<SiteSettings>({
    title: "Kohame",
    description: "Self-hosted Git, kept simple",
    allowRegistration: true,
  })
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [repos, setRepos] = useState<Repository[]>([])
  const [selected, setSelected] = useState<string | null>(() => {
    const parts = window.location.pathname.split("/").filter(Boolean)
    return parts.length === 2 ? parts.join("/") : null
  })
  const [panel, setPanel] = useState<"home" | "settings">("home")
  const [error, setError] = useState("")

  const refresh = async () => {
    const [status, settings, repositories] = await Promise.all([
      api<{ needsSetup: boolean }>("/api/setup/status"),
      api<SiteSettings>("/api/settings"),
      api<Repository[]>("/api/repos"),
    ])
    setNeedsSetup(status.needsSetup)
    setSite(settings)
    setRepos(repositories)
    if (!status.needsSetup) {
      try {
        setUser(await api<User>("/api/auth/me"))
      } catch {
        setUser(null)
      }
    }
  }

  useEffect(() => {
    void refresh().catch((cause: unknown) =>
      setError(
        cause instanceof Error ? cause.message : "Could not start Kohame."
      )
    )
  }, [])
  useEffect(() => {
    document.title = `${site.title} · Self-hosted Git`
  }, [site.title])
  useEffect(() => {
    const handlePopState = () => {
      const parts = window.location.pathname.split("/").filter(Boolean)
      setSelected(parts.length === 2 ? parts.join("/") : null)
    }
    window.addEventListener("popstate", handlePopState)
    return () => window.removeEventListener("popstate", handlePopState)
  }, [])

  const openRepo = (fullName: string) => {
    window.history.pushState({}, "", `/${fullName}`)
    setSelected(fullName)
  }
  const closeRepo = () => {
    window.history.pushState({}, "", "/")
    setSelected(null)
  }

  if (needsSetup)
    return (
      <Setup
        onComplete={(nextUser) => {
          setUser(nextUser)
          setNeedsSetup(false)
          void refresh()
        }}
        error={error}
      />
    )
  if (needsSetup === null)
    return (
      <main className="grid min-h-svh place-items-center">
        <span className="animate-pulse text-sm text-zinc-500">
          Starting Kohame…
        </span>
      </main>
    )

  return (
    <main className="min-h-svh bg-[#fcfcfa] text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100">
      <header className="sticky top-0 z-10 border-b border-zinc-200/80 bg-white/90 backdrop-blur dark:border-zinc-800 dark:bg-zinc-950/90">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
          <button
            onClick={() => {
              setPanel("home")
              closeRepo()
            }}
            className="flex items-center gap-2.5 font-semibold tracking-tight"
          >
            <span className="grid size-8 place-items-center rounded-xl bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-950">
              <GitBranch className="size-4" />
            </span>
            {site.title}
          </button>
          <div className="flex items-center gap-2">
            {user?.isAdmin && (
              <Button
                variant="ghost"
                size="sm"
                onPress={() =>
                  setPanel(panel === "settings" ? "home" : "settings")
                }
              >
                <Settings />{" "}
                <span className="hidden sm:inline">Site settings</span>
              </Button>
            )}
            {user ? (
              <Button
                variant="outline"
                size="sm"
                onPress={async () => {
                  await api<void>("/api/auth/logout", { method: "POST" })
                  setUser(null)
                  setPanel("home")
                }}
              >
                <LogOut />{" "}
                <span className="hidden sm:inline">{user.username}</span>
              </Button>
            ) : (
              <Auth site={site} onUser={setUser} />
            )}
          </div>
        </div>
      </header>
      {error && (
        <div className="mx-auto max-w-6xl px-5 pt-4">
          <p className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/50 dark:text-red-300">
            {error}
          </p>
        </div>
      )}
      {panel === "settings" && user?.isAdmin ? (
        <AdminSettings site={site} onSaved={setSite} />
      ) : selected ? (
        <RepositoryView name={selected} user={user} onBack={closeRepo} />
      ) : (
        <Dashboard
          site={site}
          user={user}
          repos={repos}
          onRepos={setRepos}
          onOpen={openRepo}
        />
      )}
    </main>
  )
}

function Setup({
  onComplete,
  error,
}: {
  onComplete: (user: User) => void
  error: string
}) {
  const [username, setUsername] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [message, setMessage] = useState(error)
  const [busy, setBusy] = useState(false)
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    setMessage("")
    try {
      onComplete(
        await api<User>("/api/setup/admin", {
          method: "POST",
          body: JSON.stringify({ username, email, password }),
        })
      )
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "Setup failed.")
    } finally {
      setBusy(false)
    }
  }
  return (
    <main className="grid min-h-svh place-items-center bg-[radial-gradient(circle_at_top,#e4f4e8,transparent_44%),#fcfcfa] p-5 dark:bg-zinc-950">
      <form
        onSubmit={submit}
        className="w-full max-w-md rounded-3xl border border-zinc-200 bg-white p-7 shadow-xl shadow-zinc-200/40 dark:border-zinc-800 dark:bg-zinc-900 dark:shadow-none"
      >
        <span className="mb-5 grid size-11 place-items-center rounded-2xl bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-900">
          <GitBranch />
        </span>
        <p className="text-sm font-medium text-emerald-700 dark:text-emerald-300">
          Welcome to Kohame
        </p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight">
          Create the site administrator
        </h1>
        <p className="mt-2 text-sm leading-6 text-zinc-500">
          This one-time setup secures your new Git forge. You can change site
          settings after signing in.
        </p>
        <div className="mt-6 space-y-3">
          <Field
            label="Username"
            value={username}
            onChange={setUsername}
            placeholder="admin"
          />
          <Field
            label="Email"
            value={email}
            onChange={setEmail}
            placeholder="admin@example.com"
            type="email"
          />
          <Field
            label="Password"
            value={password}
            onChange={setPassword}
            placeholder="At least 8 characters"
            type="password"
          />
        </div>
        {message && <p className="mt-3 text-sm text-red-600">{message}</p>}
        <Button
          type="submit"
          size="lg"
          className="mt-6 w-full"
          isPending={busy}
        >
          <KeyRound />
          Finish setup
        </Button>
      </form>
    </main>
  )
}

function Auth({
  site,
  onUser,
}: {
  site: SiteSettings
  onUser: (user: User) => void
}) {
  const [mode, setMode] = useState<"login" | "register" | null>(null)
  const [identity, setIdentity] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [captchaToken, setCaptchaToken] = useState("")
  const [message, setMessage] = useState("")
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!mode) return
    setMessage("")
    try {
      const user =
        mode === "login"
          ? await api<User>("/api/auth/login", {
              method: "POST",
              body: JSON.stringify({ identity, password }),
            })
          : await api<User>("/api/auth/register", {
              method: "POST",
              body: JSON.stringify({
                username: identity,
                email,
                password,
                captchaToken,
              }),
            })
      onUser(user)
      setMode(null)
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "Could not continue.")
    }
  }
  if (!mode)
    return (
      <div className="flex gap-1">
        <Button variant="ghost" size="sm" onPress={() => setMode("login")}>
          <LogIn />
          Sign in
        </Button>
        {site.allowRegistration && (
          <Button size="sm" onPress={() => setMode("register")}>
            <UserPlus />
            Register
          </Button>
        )}
      </div>
    )
  return (
    <div className="absolute top-14 right-5 z-20 w-80 rounded-2xl border border-zinc-200 bg-white p-5 shadow-xl dark:border-zinc-700 dark:bg-zinc-900">
      <div className="mb-4 flex items-center justify-between">
        <h2 className="font-semibold">
          {mode === "login" ? "Sign in" : "Create account"}
        </h2>
        <button onClick={() => setMode(null)}>
          <X className="size-4 text-zinc-500" />
        </button>
      </div>
      <form onSubmit={submit} className="space-y-3">
        <Field
          label={mode === "login" ? "Username or email" : "Username"}
          value={identity}
          onChange={setIdentity}
          placeholder={mode === "login" ? "you@example.com" : "octocat"}
        />
        {mode === "register" && (
          <Field
            label="Email"
            value={email}
            onChange={setEmail}
            type="email"
            placeholder="you@example.com"
          />
        )}
        <Field
          label="Password"
          value={password}
          onChange={setPassword}
          type="password"
          placeholder="••••••••"
        />
        {mode === "register" && <HCaptcha onToken={setCaptchaToken} />}
        {message && <p className="text-sm text-red-600">{message}</p>}
        <Button type="submit" className="w-full">
          {mode === "login" ? "Sign in" : "Create account"}
        </Button>
      </form>
    </div>
  )
}

function HCaptcha({ onToken }: { onToken: (token: string) => void }) {
  const host = useRef<HTMLDivElement>(null)
  const [config, setConfig] = useState<CaptchaConfig | null>(null)
  useEffect(() => {
    void api<CaptchaConfig>("/api/captcha")
      .then(setConfig)
      .catch(() => setConfig({ enabled: false, siteKey: "" }))
  }, [])
  useEffect(() => {
    if (!config?.enabled || !host.current) return
    const render = () => {
      if (window.hcaptcha && host.current)
        window.hcaptcha.render(host.current, {
          sitekey: config.siteKey,
          callback: onToken,
        })
    }
    const existing = document.querySelector<HTMLScriptElement>(
      'script[src^="https://js.hcaptcha.com"]'
    )
    if (existing) {
      existing.addEventListener("load", render, { once: true })
      render()
      return
    }
    const script = document.createElement("script")
    script.src = "https://js.hcaptcha.com/1/api.js?render=explicit"
    script.async = true
    script.defer = true
    script.addEventListener("load", render, { once: true })
    document.head.appendChild(script)
  }, [config, onToken])
  return config?.enabled ? <div ref={host} className="pt-1" /> : null
}

function Dashboard({
  site,
  user,
  repos,
  onRepos,
  onOpen,
}: {
  site: SiteSettings
  user: User | null
  repos: Repository[]
  onRepos: (repos: Repository[]) => void
  onOpen: (name: string) => void
}) {
  const [scope, setScope] = useState(user?.username ?? "")
  const [name, setName] = useState("")
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
  return (
    <>
      <section className="border-b border-zinc-200 bg-[radial-gradient(circle_at_20%_0%,#e4f4e8,transparent_34%),#fafaf8] py-14 dark:border-zinc-800 dark:bg-zinc-950">
        <div className="mx-auto max-w-6xl px-5">
          <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300">
            <HeartHandshake className="size-3" />
            Your collaborative code forge
          </p>
          <h1 className="text-4xl font-semibold tracking-[-.04em] sm:text-5xl">
            {site.description}
          </h1>
          <p className="mt-4 max-w-xl text-zinc-600 dark:text-zinc-400">
            Host Git repositories, plan work with issues, review branches with
            pull requests, and share releases from one place.
          </p>
        </div>
      </section>
      <div className="mx-auto grid max-w-6xl gap-8 px-5 py-9 lg:grid-cols-[1fr_340px]">
        <section>
          <div className="mb-4 flex items-end justify-between">
            <div>
              <h2 className="text-xl font-semibold">Repositories</h2>
              <p className="text-sm text-zinc-500">
                {repos.length} hosted project{repos.length === 1 ? "" : "s"}
              </p>
            </div>
          </div>
          <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
            {repos.length ? (
              repos.map((repo) => (
                <button
                  key={repo.fullName}
                  onClick={() => onOpen(repo.fullName)}
                  className="flex w-full items-center gap-3 border-b border-zinc-100 px-5 py-4 text-left last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800"
                >
                  <span className="grid size-9 place-items-center rounded-xl bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                    <FolderGit2 className="size-4" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <strong className="block truncate text-sm">
                      {repo.fullName}
                    </strong>
                    <small className="text-zinc-500">
                      Updated {when(repo.updatedAt)}
                    </small>
                  </span>
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
                      {copied === repo.fullName ? "Copied" : "Clone"}
                    </span>
                  </Button>
                </button>
              ))
            ) : (
              <Empty
                icon={<FolderGit2 />}
                title="No repositories yet"
                text="Create the first project after signing in."
              />
            )}
          </div>
        </section>
        <aside className="h-fit rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <Plus className="mb-4 size-5" />
          <h2 className="font-semibold">Create repository</h2>
          {user ? (
            <form className="mt-4 space-y-3" onSubmit={create}>
              <Field
                label="Scope / owner"
                value={scope}
                onChange={setScope}
                placeholder={user.username}
              />
              <Field
                label="Repository name"
                value={name}
                onChange={setName}
                placeholder="my-project"
              />
              {message && <p className="text-sm text-red-600">{message}</p>}
              <Button type="submit" className="w-full">
                <Plus />
                Create repository
              </Button>
            </form>
          ) : (
            <p className="mt-2 text-sm leading-6 text-zinc-500">
              Sign in or register to create a repository and collaborate with
              your team.
            </p>
          )}
        </aside>
      </div>
    </>
  )
}

function RepositoryView({
  name,
  user,
  onBack,
}: {
  name: string
  user: User | null
  onBack: () => void
}) {
  const [tab, setTab] = useState<
    | "code"
    | "commits"
    | "branches"
    | "tags"
    | "issues"
    | "pulls"
    | "releases"
    | "contributors"
    | "settings"
  >("code")
  const [issues, setIssues] = useState<Issue[]>([])
  const [pulls, setPulls] = useState<PullRequest[]>([])
  const [releases, setReleases] = useState<Release[]>([])
  const [contributors, setContributors] = useState<Contributor[]>([])
  const [message, setMessage] = useState("")
  const load = async () => {
    const [a, b, c, d] = await Promise.all([
      api<Issue[]>(`/api/repos/${name}/issues`),
      api<PullRequest[]>(`/api/repos/${name}/pulls`),
      api<Release[]>(`/api/repos/${name}/releases`),
      api<Contributor[]>(`/api/repos/${name}/contributors`),
    ])
    setIssues(a)
    setPulls(b)
    setReleases(c)
    setContributors(d)
  }
  useEffect(() => {
    void load().catch((cause: unknown) =>
      setMessage(
        cause instanceof Error ? cause.message : "Could not load repository."
      )
    )
  }, [name])
  const add = async (kind: string, value: Record<string, string>) => {
    if (!user) {
      setMessage("Sign in to contribute.")
      return
    }
    try {
      const target =
        kind === "issue" ? "issues" : kind === "pull" ? "pulls" : "releases"
      await api(`/api/repos/${name}/${target}`, {
        method: "POST",
        body: JSON.stringify(value),
      })
      await load()
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "Could not save item."
      )
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
        cause instanceof Error ? cause.message : "Could not delete item."
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
        cause instanceof Error ? cause.message : "Could not update item."
      )
    }
  }
  const tabs = [
    ["code", "Code", <FolderGit2 />],
    ["commits", "提交", <GitBranch />],
    ["branches", "分支", <GitBranch />],
    ["tags", "标签", <Tag />],
    ["issues", "Issues", <Megaphone />],
    ["pulls", "Pull requests", <GitPullRequest />],
    ["releases", "Releases", <Tag />],
    ["contributors", "Contributors", <Users />],
    ["settings", "设置", <Settings />],
  ] as const
  return (
    <div className="mx-auto max-w-6xl px-5 py-8">
      <button
        onClick={onBack}
        className="mb-5 text-sm text-zinc-500 hover:text-zinc-900"
      >
        ← All repositories
      </button>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <span className="grid size-10 place-items-center rounded-xl bg-emerald-50 text-emerald-700 dark:bg-emerald-950">
            <FolderGit2 />
          </span>
          <div>
            <h1 className="text-2xl font-semibold">{name}</h1>
            <p className="text-sm text-zinc-500">Git repository workspace</p>
          </div>
        </div>
        <code className="rounded-lg bg-zinc-100 px-3 py-2 text-xs text-zinc-600 dark:bg-zinc-900 dark:text-zinc-300">
          git clone {window.location.origin}/{name}.git
        </code>
      </div>
      <div className="mt-7 flex gap-1 overflow-auto border-b border-zinc-200 dark:border-zinc-800">
        {tabs.map(([value, label, icon]) => (
          <button
            key={value}
            onClick={() => setTab(value)}
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
        {tab === "code" && <CodeBrowser name={name} />}
        {tab === "commits" && <RepositoryList name={name} kind="commits" />}
        {tab === "branches" && <RepositoryList name={name} kind="branches" />}
        {tab === "tags" && <RepositoryList name={name} kind="tags" />}
        {tab === "issues" && (
          <WorkList
            title="Issues"
            items={issues}
            user={user}
            fields={["title", "body"]}
            button="New issue"
            onSubmit={(v) => add("issue", v)}
            onDelete={(id) => remove("issues", id)}
            onStateChange={(id, state) => changeState("issues", id, state)}
            render={(item) => (
              <>
                <Badge state={item.state} />
                <strong className="ml-2">
                  #{item.id} {item.title}
                </strong>
                <p className="mt-1 text-sm text-zinc-500">
                  Opened by {item.author} · {when(item.createdAt)}
                </p>
              </>
            )}
          />
        )}
        {tab === "pulls" && (
          <WorkList
            title="Pull requests"
            items={pulls}
            user={user}
            fields={["title", "sourceBranch", "targetBranch", "body"]}
            button="New pull request"
            onSubmit={(v) => add("pull", v)}
            onDelete={(id) => remove("pulls", id)}
            onStateChange={(id, state) => changeState("pulls", id, state)}
            render={(item) => (
              <>
                <Badge state={item.state} />
                <strong className="ml-2">
                  #{item.id} {item.title}
                </strong>
                <p className="mt-1 text-sm text-zinc-500">
                  {item.sourceBranch} → {item.targetBranch} · {item.author}
                </p>
              </>
            )}
          />
        )}
        {tab === "releases" && (
          <WorkList
            title="Releases"
            items={releases}
            user={user}
            fields={["tagName", "title", "notes"]}
            button="Publish release"
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
                title="No contributions yet"
                text="Issues, pull requests, and releases will appear here."
              />
            )}
          </div>
        )}
        {tab === "settings" && <RepositorySettingsPanel name={name} />}
      </section>
    </div>
  )
}

function CodeBrowser({ name }: { name: string }) {
  const [directory, setDirectory] = useState("")
  const [entries, setEntries] = useState<TreeEntry[]>([])
  const [file, setFile] = useState<Blob | null>(null)
  const [message, setMessage] = useState("")
  useEffect(() => {
    void api<TreeEntry[]>(
      `/api/repos/${name}/tree?path=${encodeURIComponent(directory)}`
    )
      .then((items) => {
        setEntries(items)
        setFile(null)
      })
      .catch((cause: unknown) =>
        setMessage(
          cause instanceof Error
            ? cause.message
            : "Could not read repository tree."
        )
      )
  }, [name, directory])
  const openEntry = async (entry: TreeEntry) => {
    if (entry.type === "tree") {
      setDirectory(entry.path)
      return
    }
    try {
      setFile(
        await api<Blob>(
          `/api/repos/${name}/blob?path=${encodeURIComponent(entry.path)}`
        )
      )
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "Could not read file."
      )
    }
  }
  const crumbs = directory ? directory.split("/") : []
  return (
    <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
      <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
        <div className="flex items-center gap-1 border-b border-zinc-100 px-4 py-3 text-sm dark:border-zinc-800">
          <button
            className="text-emerald-700 hover:underline dark:text-emerald-300"
            onClick={() => setDirectory("")}
          >
            {name}
          </button>
          {crumbs.map((crumb, index) => (
            <span key={`${crumb}-${index}`}>
              <span className="mx-1 text-zinc-300">/</span>
              <button
                onClick={() =>
                  setDirectory(crumbs.slice(0, index + 1).join("/"))
                }
                className="hover:underline"
              >
                {crumb}
              </button>
            </span>
          ))}
        </div>
        {message && <p className="px-4 pt-3 text-sm text-red-600">{message}</p>}
        {entries.length ? (
          entries.map((entry) => (
            <button
              key={entry.path}
              onClick={() => void openEntry(entry)}
              className="flex w-full items-center gap-3 border-b border-zinc-100 px-4 py-3 text-left text-sm last:border-0 hover:bg-zinc-50 dark:border-zinc-800 dark:hover:bg-zinc-800"
            >
              <FolderGit2
                className={`size-4 ${entry.type === "tree" ? "text-amber-500" : "text-zinc-400"}`}
              />
              <span>{entry.name}</span>
              <small className="ml-auto text-zinc-400">
                {entry.type === "tree" ? "directory" : "file"}
              </small>
            </button>
          ))
        ) : (
          <Empty
            icon={<FolderGit2 />}
            title="No files on the default branch"
            text="Push a commit to browse its source code here."
          />
        )}
      </div>
      <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-zinc-950 text-zinc-100 dark:border-zinc-800">
        {file ? (
          <>
            <div className="border-b border-white/10 px-4 py-3 text-sm text-zinc-300">
              {file.path}
            </div>
            {file.isText ? (
              <pre className="max-h-[32rem] overflow-auto p-4 text-xs leading-6">
                <code>{file.content}</code>
              </pre>
            ) : (
              <p className="p-5 text-sm text-zinc-400">
                Binary file preview is unavailable.
              </p>
            )}
          </>
        ) : (
          <div className="grid min-h-64 place-items-center p-6 text-center text-sm text-zinc-400">
            Select a file to preview its source code.
          </div>
        )}
      </div>
    </div>
  )
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

function WorkList<T extends { id: number }>({
  title,
  items,
  user,
  fields,
  button,
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
  return (
    <>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="text-lg font-semibold">{title}</h2>
        {user && (
          <Button size="sm" onPress={() => setOpen(!open)}>
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
      <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">
        {items.length ? (
          items.map((item) => (
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
            title={`No ${title.toLowerCase()} yet`}
            text={
              user
                ? `Create the first ${title.slice(0, -1).toLowerCase()}.`
                : "Sign in to contribute."
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

function AdminSettings({
  site,
  onSaved,
}: {
  site: SiteSettings
  onSaved: (site: SiteSettings) => void
}) {
  const [value, setValue] = useState(site)
  const [message, setMessage] = useState("")
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      onSaved(
        await api<SiteSettings>("/api/admin/settings", {
          method: "PATCH",
          body: JSON.stringify(value),
        })
      )
      setMessage("Settings saved.")
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "Could not save settings."
      )
    }
  }
  return (
    <div className="mx-auto max-w-2xl px-5 py-10">
      <div className="mb-6">
        <p className="text-sm font-medium text-emerald-700">Administrator</p>
        <h1 className="text-2xl font-semibold">Site settings</h1>
        <p className="mt-1 text-sm text-zinc-500">
          Customize the forge and control who can create an account.
        </p>
      </div>
      <form
        onSubmit={submit}
        className="space-y-5 rounded-2xl border border-zinc-200 bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
      >
        <Field
          label="Site title"
          value={value.title}
          onChange={(title) => setValue({ ...value, title })}
          placeholder="Kohame"
        />
        <Field
          label="Site description"
          value={value.description}
          onChange={(description) => setValue({ ...value, description })}
          placeholder="A home for code"
        />
        <label className="flex cursor-pointer items-center justify-between gap-5 rounded-xl bg-zinc-50 p-4 text-sm dark:bg-zinc-950">
          <span>
            <strong className="block">Allow registration</strong>
            <span className="text-zinc-500">
              Permit new users to create accounts.
            </span>
          </span>
          <input
            type="checkbox"
            checked={value.allowRegistration}
            onChange={(e) =>
              setValue({ ...value, allowRegistration: e.target.checked })
            }
            className="size-4"
          />
        </label>
        {message && <p className="text-sm text-emerald-700">{message}</p>}
        <Button type="submit">
          <Settings />
          Save settings
        </Button>
      </form>
    </div>
  )
}

function Field({
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
function Badge({ state }: { state: string }) {
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${state === "open" ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300" : state === "merged" ? "bg-purple-100 text-purple-700" : "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300"}`}
    >
      {state}
    </span>
  )
}
function Empty({
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
