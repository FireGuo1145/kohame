import { useEffect, useRef, useState, type FormEvent } from "react"
import { GitBranch, KeyRound, LogIn, UserPlus, X } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { CaptchaConfig, SiteSettings, User } from "@/lib/forge-types"

declare global { interface Window { hcaptcha?: { render: (element: HTMLElement, options: Record<string, unknown>) => void } } }

export default function SetupPage({
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
            label="用户名"
            value={username}
            onChange={setUsername}
            placeholder="admin"
          />
          <Field
            label="邮箱"
            value={email}
            onChange={setEmail}
            placeholder="admin@example.com"
            type="email"
          />
          <Field
            label="密码"
            value={password}
            onChange={setPassword}
            placeholder="至少 8 个字符"
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

export function AuthMenu({
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
          {mode === "login" ? "登录" : "注册账号"}
        </h2>
        <button onClick={() => setMode(null)}>
          <X className="size-4 text-zinc-500" />
        </button>
      </div>
      <form onSubmit={submit} className="space-y-3">
        <Field
          label={mode === "login" ? "用户名或邮箱" : "用户名"}
          value={identity}
          onChange={setIdentity}
          placeholder={mode === "login" ? "you@example.com" : "octocat"}
        />
        {mode === "register" && (
          <Field
            label="邮箱"
            value={email}
            onChange={setEmail}
            type="email"
            placeholder="you@example.com"
          />
        )}
        <Field
          label="密码"
          value={password}
          onChange={setPassword}
          type="password"
          placeholder="••••••••"
        />
        {mode === "register" && <HCaptcha onToken={setCaptchaToken} />}
        {message && <p className="text-sm text-red-600">{message}</p>}
        <Button type="submit" className="w-full">
          {mode === "login" ? "登录" : "注册账号"}
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

