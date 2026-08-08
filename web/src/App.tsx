import { useEffect, useState } from "react"
import { Bell, GitBranch, LogOut, Search, Settings, User as UserIcon } from "lucide-react"
import { Route, Routes, useLocation, useNavigate } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { api, defaultSiteSettings } from "@/lib/forge-api"
import type { Repository, SiteSettings, User } from "@/lib/forge-types"
import HomePage from "@/pages/HomePage"
import NotificationsPage from "@/pages/NotificationsPage"
import OrganizationRoutePage from "@/pages/OrganizationRoutePage"
import ProfileRoutePage from "@/pages/ProfileRoutePage"
import RepositoryRoutePage from "@/pages/RepositoryRoutePage"
import { SearchPage } from "@/pages/SearchPage"
import SiteSettingsPage from "@/pages/SiteSettingsPage"
import SetupPage, { AuthMenu } from "@/pages/SetupPage"
import VerifyRoutePage from "@/pages/VerifyRoutePage"
import WorkPage from "@/pages/WorkPage"
import { AccountSettingsPage } from "@/pages/AccountSettingsPage"

export default function App() {
  const location = useLocation()
  const navigate = useNavigate()
  const [site, setSite] = useState<SiteSettings>(defaultSiteSettings)
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null)
  const [user, setUser] = useState<User | null>(null)
  const [repos, setRepos] = useState<Repository[]>([])
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
      setError(cause instanceof Error ? cause.message : "Could not start Kohame.")
    )
  }, [])
  useEffect(() => {
    document.title = `${site.title} · Self-hosted Git`
  }, [site.title])

  const openRepo = (fullName: string) => navigate(`/${fullName}`)
  const openHome = () => navigate("/")
  const dashboard = (
    <HomePage
      site={site}
      user={user}
      repos={repos}
      onRepos={setRepos}
      onOpen={openRepo}
      onOrganization={(name) => navigate(`/orgs/${name}`)}
    />
  )

  if (needsSetup) {
    return (
      <SetupPage
        error={error}
        onComplete={(nextUser) => {
          setUser(nextUser)
          setNeedsSetup(false)
          void refresh()
        }}
      />
    )
  }
  if (needsSetup === null) {
    return <main className="grid min-h-svh place-items-center"><span className="animate-pulse text-sm text-zinc-500">Starting Kohame...</span></main>
  }

  return (
    <main className="min-h-svh bg-[#fcfcfa] text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100">
      <header className="sticky top-0 z-10 border-b border-zinc-200/80 bg-white/90 backdrop-blur dark:border-zinc-800 dark:bg-zinc-950/90">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-5">
          <button onClick={openHome} className="flex items-center gap-2.5 font-semibold tracking-tight">
            <span className="grid size-8 place-items-center rounded-xl bg-zinc-900 text-white dark:bg-zinc-100 dark:text-zinc-950"><GitBranch className="size-4" /></span>
            {site.title}
          </button>
          <nav className="hidden items-center gap-1 md:flex">
            <Button variant="ghost" size="sm" onPress={() => navigate("/work")}>工作台</Button>
            <Button variant="ghost" size="sm" onPress={openHome}>探索</Button>
          </nav>
          <div className="flex items-center gap-2">
            <form className="hidden lg:block" onSubmit={(event) => { event.preventDefault(); const query = new FormData(event.currentTarget).get("q")?.toString().trim(); if (query) navigate(`/search?q=${encodeURIComponent(query)}`) }}>
              <label className="flex h-8 w-56 items-center gap-2 rounded-lg border border-zinc-200 bg-zinc-50 px-2 text-zinc-500 dark:border-zinc-700 dark:bg-zinc-900"><Search className="size-4" /><input name="q" defaultValue={location.pathname === "/search" ? new URLSearchParams(location.search).get("q") || "" : ""} placeholder="搜索仓库和用户" className="w-full bg-transparent text-sm outline-none" /></label>
            </form>
            {user?.isAdmin && <Button variant="ghost" size="sm" onPress={() => navigate(location.pathname === "/settings" ? "/" : "/settings")}><Settings /><span className="hidden sm:inline">站点设置</span></Button>}
            {user ? <>
              <Button variant="ghost" size="icon" aria-label="通知" onPress={() => navigate("/notifications")}><Bell /></Button>
              <Button variant="outline" size="sm" onPress={() => navigate(`/${user.username}`)}><UserIcon /><span className="hidden sm:inline">{user.username}</span></Button>
              <Button variant="ghost" size="icon" aria-label="个人设置" onPress={() => navigate("/account")}><Settings /></Button>
              <Button variant="ghost" size="icon" aria-label="退出登录" onPress={async () => { await api<void>("/api/auth/logout", { method: "POST" }); setUser(null); openHome() }}><LogOut /></Button>
            </> : <AuthMenu site={site} onUser={setUser} />}
          </div>
        </div>
      </header>
      {error && <div className="mx-auto max-w-6xl px-5 pt-4"><p className="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700 dark:bg-red-950/50 dark:text-red-300">{error}</p></div>}
      <Routes>
        <Route path="/settings" element={user?.isAdmin ? <SiteSettingsPage site={site} onSaved={setSite} /> : dashboard} />
        <Route path="/work" element={<WorkPage user={user} repos={repos} onOpen={openRepo} />} />
        <Route path="/notifications" element={<NotificationsPage onOpen={(link) => { const parts = link.split("/").filter(Boolean); if (parts.length === 2) openRepo(parts.join("/")) }} />} />
        <Route path="/verify" element={<VerifyRoutePage onDone={() => { void refresh(); openHome() }} />} />
        <Route path="/search" element={<SearchPage initialQuery={new URLSearchParams(location.search).get("q") || ""} onOpen={openRepo} onProfile={(username) => navigate(`/${username}`)} />} />
        <Route path="/account" element={<AccountSettingsPage user={user} />} />
        <Route path="/orgs/:name" element={<OrganizationRoutePage onOpen={openRepo} />} />
        <Route path="/:scope/:name/*" element={<RepositoryRoutePage user={user} onBack={openHome} onOpen={openRepo} />} />
        <Route path="/:username" element={<ProfileRoutePage user={user} onOpen={openRepo} />} />
        <Route path="*" element={dashboard} />
      </Routes>
    </main>
  )
}
