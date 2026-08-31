import { useEffect, useState, type FormEvent } from "react"
import {
  Bell,
  CircleUserRound,
  KeyRound,
  MailCheck,
  Shield,
  Terminal,
  Trash2,
  Upload,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Avatar } from "@/components/forge-ui"

type User = { username: string }
type PersonalSettings = {
  username: string
  email: string
  verified: boolean
  displayName: string
  bio: string
  location: string
  website: string
  avatarUrl: string
}
type SSHKey = { id: number; title: string; key: string; createdAt: string }
type Runner = {
  id: number
  name: string
  createdAt: string
  lastSeenAt?: string
}
type RunnerRegistration = Runner & { token: string }
type Section = "profile" | "security" | "ssh" | "runners" | "notifications"

async function api<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({}))
    throw new Error(body.error || "请求失败。")
  }
  return response.status === 204 ? (undefined as T) : response.json()
}

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder: string
}) {
  return (
    <label className="block text-sm font-medium">
      {label}
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="mt-1.5 h-10 w-full rounded-lg border border-zinc-200 bg-transparent px-3 text-sm outline-none focus:border-zinc-500 dark:border-zinc-700"
      />
    </label>
  )
}

export function AccountSettingsPage({ user }: { user: User | null }) {
  const [value, setValue] = useState<PersonalSettings | null>(null)
  const [keys, setKeys] = useState<SSHKey[]>([])
  const [keyTitle, setKeyTitle] = useState("")
  const [keyValue, setKeyValue] = useState("")
  const [runners, setRunners] = useState<Runner[]>([])
  const [runnerName, setRunnerName] = useState("")
  const [runnerToken, setRunnerToken] = useState("")
  const [message, setMessage] = useState("")
  const [saving, setSaving] = useState(false)
  const [active, setActive] = useState<Section>("profile")

  useEffect(() => {
    if (!user) return
    void api<PersonalSettings>("/api/user/settings")
      .then(setValue)
      .catch((cause: unknown) =>
        setMessage(
          cause instanceof Error ? cause.message : "无法加载个人设置。"
        )
      )
  }, [user])

  if (!user)
    return (
      <div className="mx-auto max-w-3xl px-5 py-9">
        <h1 className="text-2xl font-semibold">个人设置</h1>
        <p className="mt-5 rounded-lg bg-red-50 p-4 text-sm text-red-700">
          请先登录后再管理个人资料。
        </p>
      </div>
    )
  if (!value)
    return (
      <main className="grid min-h-[50svh] place-items-center text-sm text-zinc-500">
        加载中…
      </main>
    )

  const save = async (event: FormEvent) => {
    event.preventDefault()
    setSaving(true)
    try {
      setValue(
        await api<PersonalSettings>("/api/user/settings", {
          method: "PATCH",
          body: JSON.stringify(value),
        })
      )
      setMessage("个人资料已保存。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "保存失败。")
    } finally {
      setSaving(false)
    }
  }
  const uploadAvatar = async (file: File) => {
    const form = new FormData()
    form.append("avatar", file)
    setSaving(true)
    try {
      const response = await fetch("/api/user/avatar", {
        method: "POST",
        body: form,
      })
      const body = await response.json()
      if (!response.ok) throw new Error(body.error || "头像上传失败。")
      setValue(body)
      setMessage("头像已更新。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "头像上传失败。")
    } finally {
      setSaving(false)
    }
  }
  const loadKeys = async () => {
    try {
      setKeys(await api<SSHKey[]>("/api/user/ssh-keys"))
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "无法读取 SSH 密钥。")
    }
  }
  const addKey = async () => {
    setSaving(true)
    setMessage("")
    try {
      const item = await api<SSHKey>("/api/user/ssh-keys", {
        method: "POST",
        body: JSON.stringify({ title: keyTitle, key: keyValue }),
      })
      setKeys([item, ...keys])
      setKeyTitle("")
      setKeyValue("")
      setMessage("SSH 密钥已添加。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "无法添加 SSH 密钥。")
    } finally {
      setSaving(false)
    }
  }
  const removeKey = async (id: number) => {
    try {
      await api<void>(`/api/user/ssh-keys/${id}`, { method: "DELETE" })
      setKeys(keys.filter((item) => item.id !== id))
      setMessage("SSH 密钥已删除。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "无法删除 SSH 密钥。")
    }
  }
  const loadRunners = async () => {
    try {
      setRunners(await api<Runner[]>("/api/user/runners"))
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "无法读取 Runner。")
    }
  }
  const addRunner = async () => {
    setSaving(true)
    try {
      const item = await api<RunnerRegistration>("/api/user/runners", {
        method: "POST",
        body: JSON.stringify({ name: runnerName }),
      })
      setRunners([item, ...runners])
      setRunnerName("")
      setRunnerToken(item.token)
      setMessage("Runner 已创建。请立即保存令牌，它不会再次显示。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "无法创建 Runner。")
    } finally {
      setSaving(false)
    }
  }
  const removeRunner = async (id: number) => {
    try {
      await api<void>(`/api/user/runners/${id}`, { method: "DELETE" })
      setRunners(runners.filter((item) => item.id !== id))
      setMessage("Runner 已删除。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "无法删除 Runner。")
    }
  }

  const nav: { id: Section; label: string; icon: typeof CircleUserRound }[] = [
    { id: "profile", label: "公开资料", icon: CircleUserRound },
    { id: "security", label: "账户与安全", icon: Shield },
    { id: "ssh", label: "SSH 密钥", icon: KeyRound },
    { id: "runners", label: "Actions Runner", icon: Terminal },
    { id: "notifications", label: "通知", icon: Bell },
  ]
  const title = nav.find((item) => item.id === active)?.label || "个人设置"

  return (
    <div className="mx-auto grid max-w-6xl gap-8 px-5 py-9 md:grid-cols-[200px_1fr]">
      <aside className="h-fit rounded-lg border border-zinc-200 bg-white p-2 text-sm dark:border-zinc-800 dark:bg-zinc-900">
        <p className="px-3 py-2 text-xs font-semibold tracking-[0.12em] text-zinc-400 uppercase">
          个人设置
        </p>
        {nav.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => {
              setActive(id)
              setMessage("")
              if (id === "ssh") void loadKeys()
              if (id === "runners") void loadRunners()
            }}
            className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left ${active === id ? "bg-zinc-900 font-medium text-white dark:bg-zinc-100 dark:text-zinc-950" : "text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"}`}
          >
            <Icon className="size-3.5" />
            {label}
          </button>
        ))}
      </aside>
      <main className="max-w-2xl space-y-7">
        <div>
          <h1 className="text-2xl font-semibold">{title}</h1>
          <p className="mt-1 text-sm text-zinc-500">
            管理你在 Kohame 上显示的身份与账户信息。
          </p>
        </div>
        {active === "profile" && (
          <form
            onSubmit={save}
            className="space-y-5 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"
          >
            <div className="flex items-center gap-4">
              <Avatar name={user.username} src={value.avatarUrl} size="lg" />
              <label className="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-zinc-300 px-3 py-2 text-sm font-medium hover:bg-zinc-50 dark:border-zinc-700 dark:hover:bg-zinc-800">
                <Upload className="size-4" />
                上传头像
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/gif,image/webp"
                  className="sr-only"
                  onChange={(event) => {
                    const file = event.target.files?.[0]
                    if (file) void uploadAvatar(file)
                  }}
                />
              </label>
            </div>
            <Field
              label="显示名称"
              value={value.displayName}
              onChange={(displayName) => setValue({ ...value, displayName })}
              placeholder={user.username}
            />
            <label className="block text-sm font-medium">
              个人简介
              <textarea
                value={value.bio}
                onChange={(event) =>
                  setValue({ ...value, bio: event.target.value })
                }
                maxLength={500}
                placeholder="介绍一下你自己、正在做的事或擅长的方向"
                className="mt-1.5 block min-h-28 w-full rounded-lg border border-zinc-200 bg-transparent p-3 text-sm outline-none focus:border-zinc-500 dark:border-zinc-700"
              />
            </label>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field
                label="所在地"
                value={value.location}
                onChange={(location) => setValue({ ...value, location })}
                placeholder="重庆，中国"
              />
              <Field
                label="个人网站"
                value={value.website}
                onChange={(website) => setValue({ ...value, website })}
                placeholder="https://example.com"
              />
            </div>
            <Button type="submit" isPending={saving}>
              保存个人资料
            </Button>
          </form>
        )}
        {active === "security" && (
          <section className="rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <div className="flex items-start gap-3">
              <MailCheck className="mt-0.5 size-5 text-emerald-600" />
              <div>
                <h2 className="font-semibold">账户邮箱</h2>
                <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-400">
                  {value.email} · {value.verified ? "已验证" : "尚未验证"}
                </p>
                {!value.verified && (
                  <Button
                    className="mt-4"
                    variant="outline"
                    onPress={async () => {
                      try {
                        await api("/api/auth/verification", { method: "POST" })
                        setMessage("验证邮件已发送，请检查收件箱。")
                      } catch (cause) {
                        setMessage(
                          cause instanceof Error ? cause.message : "发送失败。"
                        )
                      }
                    }}
                  >
                    发送验证邮件
                  </Button>
                )}
              </div>
            </div>
          </section>
        )}
        {active === "ssh" && (
          <section className="space-y-5 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <div>
              <h2 className="font-semibold">SSH 密钥</h2>
              <p className="mt-1 text-sm leading-6 text-zinc-500">
                添加公钥后，可通过 SSH 克隆、拉取和推送仓库。
              </p>
            </div>
            <div className="space-y-3 border-b border-zinc-200 pb-5 dark:border-zinc-800">
              <Field
                label="密钥标题"
                value={keyTitle}
                onChange={setKeyTitle}
                placeholder="我的笔记本"
              />
              <label className="block text-sm font-medium">
                公钥
                <textarea
                  required
                  value={keyValue}
                  onChange={(event) => setKeyValue(event.target.value)}
                  placeholder="ssh-ed25519 AAAA..."
                  className="mt-1.5 block min-h-24 w-full rounded-lg border border-zinc-200 bg-transparent p-3 font-mono text-xs outline-none focus:border-zinc-500 dark:border-zinc-700"
                />
              </label>
              <Button
                type="button"
                isPending={saving}
                isDisabled={!keyTitle.trim() || !keyValue.trim()}
                onPress={() => void addKey()}
              >
                <KeyRound />
                添加 SSH 密钥
              </Button>
            </div>
            <div>
              {keys.length ? (
                keys.map((item) => (
                  <div
                    key={item.id}
                    className="flex items-start justify-between gap-4 border-b border-zinc-100 py-4 last:border-0 dark:border-zinc-800"
                  >
                    <div className="min-w-0">
                      <strong className="text-sm">{item.title}</strong>
                      <code className="mt-1 block truncate text-xs text-zinc-500">
                        {item.key}
                      </code>
                    </div>
                    <Button
                      type="button"
                      size="xs"
                      variant="destructive"
                      onPress={() => void removeKey(item.id)}
                      aria-label={`删除 ${item.title}`}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                ))
              ) : (
                <p className="py-3 text-sm text-zinc-500">
                  尚未添加 SSH 密钥。
                </p>
              )}
            </div>
          </section>
        )}
        {active === "runners" && (
          <section className="space-y-5 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <div>
              <h2 className="font-semibold">Actions Runner</h2>
              <p className="mt-1 text-sm leading-6 text-zinc-500">
                Runner 从你的网络主动连接 Kohame 领取任务，Kohame
                不会连接到你的机器。
              </p>
            </div>
            <div className="flex gap-2">
              <input
                value={runnerName}
                onChange={(event) => setRunnerName(event.target.value)}
                placeholder="Runner 名称"
                className="h-10 min-w-0 flex-1 rounded-lg border border-zinc-200 bg-transparent px-3 text-sm dark:border-zinc-700"
              />
              <Button
                type="button"
                isPending={saving}
                isDisabled={!runnerName.trim()}
                onPress={() => void addRunner()}
              >
                <Terminal />
                添加 Runner
              </Button>
            </div>
            {runnerToken && (
              <div className="space-y-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
                <strong>仅显示一次的令牌</strong>
                <code className="block rounded bg-white/70 p-2 text-xs break-all dark:bg-zinc-950">
                  {runnerToken}
                </code>
                <code className="block text-xs break-all">
                  kohame-runner -server {window.location.origin} -token{" "}
                  {runnerToken}
                </code>
              </div>
            )}
            <div>
              {runners.length ? (
                runners.map((item) => (
                  <div
                    key={item.id}
                    className="flex items-center gap-3 border-b border-zinc-100 py-3 last:border-0 dark:border-zinc-800"
                  >
                    <Terminal className="size-4 text-zinc-500" />
                    <div className="min-w-0 flex-1">
                      <strong className="text-sm">{item.name}</strong>
                      <p className="text-xs text-zinc-500">
                        {item.lastSeenAt
                          ? `上次在线 ${new Date(item.lastSeenAt).toLocaleString()}`
                          : "尚未连接"}
                      </p>
                    </div>
                    <Button
                      type="button"
                      size="xs"
                      variant="destructive"
                      onPress={() => void removeRunner(item.id)}
                      aria-label={`删除 ${item.name}`}
                    >
                      <Trash2 />
                    </Button>
                  </div>
                ))
              ) : (
                <p className="py-3 text-sm text-zinc-500">尚未注册 Runner。</p>
              )}
            </div>
          </section>
        )}
        {active === "notifications" && (
          <section className="rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <h2 className="font-semibold">通知收件箱</h2>
            <p className="mt-2 text-sm leading-6 text-zinc-500">
              议题、拉取请求、收藏和派生的站内通知会显示在顶部通知入口。邮件通知依赖管理员配置
              SMTP 服务。
            </p>
          </section>
        )}
        {message && (
          <p
            className={
              message.includes("已")
                ? "text-sm text-emerald-700"
                : "text-sm text-red-600"
            }
          >
            {message}
          </p>
        )}
      </main>
    </div>
  )
}
