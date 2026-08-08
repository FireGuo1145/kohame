import { useEffect, useState, type ReactNode } from "react"
import { Bell, ChevronLeft, ChevronRight, FolderGit2, GitBranch, House, LogOut, PanelLeftClose, PanelLeftOpen, Search, Settings, User as UserIcon } from "lucide-react"
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
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => typeof window !== "undefined" ? window.innerWidth < 768 : false)

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
      setError(cause instanceof Error ? cause.message : "无法启动 Kohame。")
    )
  }, [])
  useEffect(() => {
    document.title = `${site.title} · 自托管 Git`
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
    return <main className="grid min-h-svh place-items-center"><span className="animate-pulse text-sm text-zinc-500">正在启动 Kohame…</span></main>
  }

  const isActive = (path: string) => path === "/" ? location.pathname === "/" : location.pathname.startsWith(path)
  const pageName = location.pathname === "/" ? "探索" : location.pathname.startsWith("/work") ? "工作台" : location.pathname.startsWith("/notifications") ? "通知" : location.pathname.startsWith("/settings") ? "站点设置" : location.pathname.startsWith("/account") ? "个人设置" : "仓库"

  return (
    <main className="min-h-svh bg-[#f7f7f4] text-zinc-900 dark:bg-zinc-950 dark:text-zinc-100">
      <div className="min-h-svh">
        <aside className={`fixed inset-y-0 left-0 z-30 flex flex-col border-r border-zinc-200/80 bg-white px-3 py-4 transition-[width] duration-200 dark:border-zinc-800 dark:bg-zinc-950 ${sidebarCollapsed ? "w-[4.5rem]" : "w-60"}`}>
          <div className={`mb-8 flex items-center ${sidebarCollapsed ? "justify-center" : "justify-between"}`}>
            <button onClick={openHome} className={`flex min-w-0 items-center gap-3 text-left ${sidebarCollapsed ? "justify-center" : ""}`} aria-label="返回首页">
            <span className="grid size-8 place-items-center rounded-lg bg-zinc-950 text-white shadow-sm dark:bg-zinc-100 dark:text-zinc-950"><GitBranch className="size-4" /></span>
            {!sidebarCollapsed && <span className="leading-none"><strong className="block truncate text-sm tracking-[0.02em]">{site.title}</strong><small className="mt-1 block text-[10px] font-medium uppercase tracking-[0.14em] text-zinc-400">代码工作区</small></span>}
            </button>
            <button onClick={() => setSidebarCollapsed(!sidebarCollapsed)} className="hidden size-8 place-items-center rounded-lg text-zinc-500 hover:bg-zinc-100 hover:text-zinc-900 md:grid dark:hover:bg-zinc-900 dark:hover:text-zinc-100" aria-label={sidebarCollapsed ? "展开侧边栏" : "收起侧边栏"} title={sidebarCollapsed ? "展开侧边栏" : "收起侧边栏"}>{sidebarCollapsed ? <PanelLeftOpen className="size-4" /> : <PanelLeftClose className="size-4" />}</button>
          </div>
          <nav className="space-y-1">
            <SidebarItem icon={<House />} label="探索仓库" collapsed={sidebarCollapsed} active={isActive("/")} onPress={openHome} />
            <SidebarItem icon={<FolderGit2 />} label="我的工作台" collapsed={sidebarCollapsed} active={isActive("/work")} onPress={() => navigate("/work")} />
            <SidebarItem icon={<Bell />} label="通知" collapsed={sidebarCollapsed} active={isActive("/notifications")} onPress={() => navigate("/notifications")} />
          </nav>
          <div className="mt-7 border-t border-zinc-100 pt-5 dark:border-zinc-800">
            {!sidebarCollapsed && <p className="px-3 pb-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-400">工作区</p>}
            {user?.isAdmin && <SidebarItem icon={<Settings />} label="站点设置" collapsed={sidebarCollapsed} active={isActive("/settings")} onPress={() => navigate("/settings")} />}
            {user && <SidebarItem icon={<UserIcon />} label="个人设置" collapsed={sidebarCollapsed} active={isActive("/account")} onPress={() => navigate("/account")} />}
          </div>
          <div className="mt-auto border-t border-zinc-100 pt-3 dark:border-zinc-800">
            {user ? <button onClick={() => navigate(`/${user.username}`)} className={`flex w-full items-center gap-2 rounded-lg p-2 text-left text-sm hover:bg-zinc-100 dark:hover:bg-zinc-900 ${sidebarCollapsed ? "justify-center" : ""}`} title={sidebarCollapsed ? user.username : undefined}><span className="grid size-7 place-items-center rounded-full bg-emerald-100 text-xs font-semibold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">{user.username.slice(0, 1).toUpperCase()}</span>{!sidebarCollapsed && <><span className="min-w-0 flex-1 truncate font-medium">{user.username}</span><LogOut className="size-3.5 text-zinc-400" /></>}</button> : !sidebarCollapsed && <p className="px-2 text-xs leading-5 text-zinc-500">登录后可创建仓库并管理协作。</p>}
          </div>
        </aside>
        <div className={`min-w-0 transition-[margin] duration-200 ${sidebarCollapsed ? "ml-[4.5rem]" : "ml-60"}`}>
          <header className="sticky top-0 z-10 border-b border-zinc-200/80 bg-[#f7f7f4]/90 backdrop-blur dark:border-zinc-800 dark:bg-zinc-950/90">
            <div className="flex h-15 items-center gap-3 px-4 sm:px-6">
              <button onClick={() => setSidebarCollapsed(!sidebarCollapsed)} className="grid size-8 place-items-center rounded-lg text-zinc-500 hover:bg-zinc-100 md:hidden dark:hover:bg-zinc-900" aria-label={sidebarCollapsed ? "展开侧边栏" : "收起侧边栏"}>{sidebarCollapsed ? <ChevronRight className="size-4" /> : <ChevronLeft className="size-4" />}</button>
              <p className="hidden text-sm font-medium text-zinc-500 sm:block">{pageName}</p>
              <form className="ml-auto hidden md:block" onSubmit={(event) => { event.preventDefault(); const query = new FormData(event.currentTarget).get("q")?.toString().trim(); if (query) navigate(`/search?q=${encodeURIComponent(query)}`) }}>
                <label className="flex h-8 w-64 items-center gap-2 rounded-lg border border-zinc-200 bg-white px-2.5 text-zinc-400 shadow-sm focus-within:border-zinc-400 dark:border-zinc-800 dark:bg-zinc-900"><Search className="size-3.5" /><input name="q" defaultValue={location.pathname === "/search" ? new URLSearchParams(location.search).get("q") || "" : ""} placeholder="搜索仓库、用户..." className="w-full bg-transparent text-sm text-zinc-800 outline-none placeholder:text-zinc-400 dark:text-zinc-100" /></label>
              </form>
              <div className="flex items-center gap-1">
                {user?.isAdmin && <Button variant="ghost" size="icon" aria-label="站点设置" onPress={() => navigate("/settings")}><Settings /></Button>}
                {user ? <>
                  <Button variant="ghost" size="icon" aria-label="通知" onPress={() => navigate("/notifications")}><Bell /></Button>
                  <Button variant="ghost" size="icon" aria-label="退出登录" onPress={async () => { await api<void>("/api/auth/logout", { method: "POST" }); setUser(null); openHome() }}><LogOut /></Button>
                </> : <AuthMenu site={site} onUser={setUser} />}
              </div>
            </div>
          </header>
          {error && <div className="px-4 pt-4 sm:px-6"><p className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/50 dark:text-red-300">{error}</p></div>}
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
        </div>
      </div>
    </main>
  )
}

function SidebarItem({ icon, label, collapsed, active, onPress }: { icon: ReactNode; label: string; collapsed: boolean; active: boolean; onPress: () => void }) {
  return <button onClick={onPress} title={collapsed ? label : undefined} className={`flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors ${collapsed ? "justify-center px-2" : ""} ${active ? "bg-zinc-900 font-medium text-white shadow-sm dark:bg-zinc-100 dark:text-zinc-950" : "text-zinc-600 hover:bg-zinc-100 hover:text-zinc-950 dark:text-zinc-400 dark:hover:bg-zinc-900 dark:hover:text-zinc-100"}`}><span className="grid size-5 place-items-center [&_svg]:size-4">{icon}</span>{!collapsed && label}</button>
}
