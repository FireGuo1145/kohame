import { useEffect, useMemo, useState } from "react"
import {
  ArrowUpRight,
  Check,
  ChevronRight,
  CircleDot,
  Copy,
  FolderGit2,
  GitBranch,
  Plus,
  Search,
  Terminal,
} from "lucide-react"

import { Button } from "@/components/ui/button"

type Repository = {
  name: string
  updatedAt: string
}

const relativeTime = (date: string) => {
  const seconds = Math.max(0, Math.floor((Date.now() - new Date(date).getTime()) / 1000))
  if (seconds < 60) return "just now"
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}

function App() {
  const [repos, setRepos] = useState<Repository[]>([])
  const [query, setQuery] = useState("")
  const [newName, setNewName] = useState("")
  const [error, setError] = useState("")
  const [creating, setCreating] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)

  useEffect(() => {
    void fetch("/api/repos")
      .then((response) => {
        if (!response.ok) throw new Error("Could not load repositories.")
        return response.json()
      })
      .then((data: Repository[]) => setRepos(data))
      .catch((cause: unknown) => setError(cause instanceof Error ? cause.message : "Could not load repositories."))
  }, [])

  const filteredRepos = useMemo(
    () => repos.filter((repo) => repo.name.includes(query.trim().toLowerCase())),
    [repos, query]
  )

  const createRepo = async (event: React.FormEvent) => {
    event.preventDefault()
    setError("")
    setCreating(true)
    try {
      const response = await fetch("/api/repos", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: newName.trim().toLowerCase() }),
      })
      const body = await response.json()
      if (!response.ok) throw new Error(body.error || "Could not create repository.")
      setRepos((current) => [body, ...current])
      setNewName("")
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not create repository.")
    } finally {
      setCreating(false)
    }
  }

  const copyClone = async (name: string) => {
    const command = `git clone ${window.location.origin}/git/${name}`
    await navigator.clipboard.writeText(command)
    setCopied(name)
    window.setTimeout(() => setCopied(null), 1800)
  }

  return (
    <main className="min-h-svh bg-[#fcfcfa] text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100">
      <header className="border-b border-zinc-200/80 bg-white/75 backdrop-blur dark:border-zinc-800 dark:bg-zinc-950/75">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-5">
          <a className="flex items-center gap-2.5 font-semibold tracking-tight" href="/">
            <span className="grid size-8 place-items-center rounded-xl bg-zinc-900 text-white shadow-sm dark:bg-zinc-100 dark:text-zinc-950"><GitBranch className="size-4" /></span>
            Kohame
          </a>
          <div className="flex items-center gap-1 text-sm text-zinc-500">
            <a className="rounded-lg px-3 py-2 hover:bg-zinc-100 dark:hover:bg-zinc-900" href="#repositories">Repositories</a>
            <a className="hidden rounded-lg px-3 py-2 hover:bg-zinc-100 sm:block dark:hover:bg-zinc-900" href="https://git-scm.com/docs/git-http-backend" target="_blank" rel="noreferrer">Git HTTP <ArrowUpRight className="ml-1 inline size-3" /></a>
          </div>
        </div>
      </header>

      <section className="border-b border-zinc-200/80 bg-[radial-gradient(circle_at_30%_0%,#e8f5ea_0,transparent_30%),linear-gradient(110deg,#f7faf7,#fbfaf8)] py-16 dark:border-zinc-800 dark:bg-zinc-950">
        <div className="mx-auto max-w-6xl px-5">
          <div className="max-w-2xl">
            <p className="mb-4 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300"><CircleDot className="size-3" /> Self-hosted Git, kept simple</p>
            <h1 className="text-4xl font-semibold tracking-[-0.04em] sm:text-5xl">Your code, at home.</h1>
            <p className="mt-5 max-w-xl text-base leading-7 text-zinc-600 dark:text-zinc-400">Create bare Git repositories, browse your workspace, and push with the Git client you already use. Everything runs from one small Go binary.</p>
          </div>
        </div>
      </section>

      <div className="mx-auto grid max-w-6xl gap-10 px-5 py-10 lg:grid-cols-[1fr_360px]">
        <section id="repositories" className="min-w-0">
          <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
            <div><h2 className="text-xl font-semibold tracking-tight">Repositories</h2><p className="mt-1 text-sm text-zinc-500">{repos.length} {repos.length === 1 ? "repository" : "repositories"}</p></div>
            <label className="relative block"><Search className="pointer-events-none absolute left-3 top-2.5 size-4 text-zinc-400" /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter repositories" className="h-9 w-52 rounded-xl border border-zinc-200 bg-white pl-9 pr-3 text-sm outline-none transition focus:border-zinc-400 focus:ring-3 focus:ring-zinc-200 dark:border-zinc-800 dark:bg-zinc-900 dark:focus:ring-zinc-800" /></label>
          </div>

          <div className="overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
            {filteredRepos.length ? filteredRepos.map((repo) => (
              <article key={repo.name} className="group flex items-center gap-3 border-b border-zinc-100 px-5 py-4 last:border-0 dark:border-zinc-800">
                <span className="grid size-9 shrink-0 place-items-center rounded-xl bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300"><FolderGit2 className="size-4" /></span>
                <div className="min-w-0 flex-1"><h3 className="truncate font-medium">{repo.name}</h3><p className="mt-0.5 text-xs text-zinc-500">Updated {relativeTime(repo.updatedAt)}</p></div>
                <Button variant="ghost" size="sm" aria-label={`Copy clone command for ${repo.name}`} onPress={() => copyClone(repo.name)}>{copied === repo.name ? <Check /> : <Copy />}<span className="hidden sm:inline">{copied === repo.name ? "Copied" : "Clone"}</span></Button>
                <ChevronRight className="size-4 text-zinc-300" />
              </article>
            )) : <div className="grid min-h-60 place-items-center px-6 text-center"><div><FolderGit2 className="mx-auto mb-3 size-8 text-zinc-300" /><p className="font-medium">{query ? "No matching repositories" : "No repositories yet"}</p><p className="mt-1 text-sm text-zinc-500">{query ? "Try a different filter." : "Create your first project to get started."}</p></div></div>}
          </div>
        </section>

        <aside className="h-fit rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
          <div className="mb-5 flex size-10 items-center justify-center rounded-xl bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-950"><Plus className="size-5" /></div>
          <h2 className="font-semibold">Create a repository</h2>
          <p className="mt-1 text-sm leading-6 text-zinc-500">A private bare repository will be created on this server.</p>
          <form className="mt-5 space-y-3" onSubmit={createRepo}>
            <label className="block text-sm font-medium">Repository name<input value={newName} onChange={(event) => setNewName(event.target.value)} required maxLength={80} placeholder="my-project" className="mt-1.5 h-10 w-full rounded-xl border border-zinc-200 bg-white px-3 text-sm outline-none transition placeholder:text-zinc-400 focus:border-zinc-400 focus:ring-3 focus:ring-zinc-200 dark:border-zinc-700 dark:bg-zinc-950 dark:focus:ring-zinc-800" /></label>
            {error && <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/50 dark:text-red-300">{error}</p>}
            <Button className="w-full" size="lg" type="submit" isPending={creating}><Plus />Create repository</Button>
          </form>
          <div className="mt-5 rounded-xl bg-zinc-50 p-3 text-xs leading-5 text-zinc-500 dark:bg-zinc-950"><Terminal className="mr-1 inline size-3" /> Names support lowercase letters, numbers, <code>-</code>, <code>_</code>, and <code>.</code></div>
        </aside>
      </div>

      <footer className="mx-auto max-w-6xl border-t border-zinc-200 px-5 py-7 text-xs text-zinc-500 dark:border-zinc-800"><span className="inline-flex items-center gap-1.5"><GitBranch className="size-3.5" /> Kohame · Git hosting for your local network</span></footer>
    </main>
  )
}

export default App
