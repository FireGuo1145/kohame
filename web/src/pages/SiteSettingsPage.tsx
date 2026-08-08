import { useEffect, useState, type FormEvent } from "react"
import { Database, Mail, Settings, ShieldCheck } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Field } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { SiteSettings } from "@/lib/forge-types"

type Section = "general" | "security" | "mail" | "storage"

const sections: { id: Section; label: string; icon: typeof Settings }[] = [
  { id: "general", label: "常规设置", icon: Settings },
  { id: "security", label: "注册与安全", icon: ShieldCheck },
  { id: "mail", label: "邮件与通知", icon: Mail },
  { id: "storage", label: "存储与维护", icon: Database },
]

export default function SiteSettingsPage({ site, onSaved }: { site: SiteSettings; onSaved: (site: SiteSettings) => void }) {
  const [value, setValue] = useState(site)
  const [active, setActive] = useState<Section>("general")
  const [message, setMessage] = useState("")

  useEffect(() => {
    void api<SiteSettings>("/api/admin/settings").then(setValue).catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "Could not load settings."))
  }, [])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      const saved = await api<SiteSettings>("/api/admin/settings", { method: "PATCH", body: JSON.stringify(value) })
      setValue(saved)
      onSaved(saved)
      setMessage("设置已保存。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "Could not save settings.")
    }
  }

  const heading = sections.find((item) => item.id === active)?.label || "站点设置"
  return <div className="mx-auto grid max-w-6xl gap-8 px-5 py-10 md:grid-cols-[200px_1fr]">
    <aside className="h-fit rounded-lg border border-zinc-200 bg-white p-2 text-sm dark:border-zinc-800 dark:bg-zinc-900"><p className="px-3 py-2 text-xs font-semibold uppercase tracking-[0.12em] text-zinc-400">站点管理</p>{sections.map(({ id, label, icon: Icon }) => <button key={id} onClick={() => { setActive(id); setMessage("") }} className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left transition-colors ${active === id ? "bg-zinc-900 font-medium text-white dark:bg-zinc-100 dark:text-zinc-950" : "text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"}`}><Icon className="size-3.5" />{label}</button>)}</aside>
    <form onSubmit={submit} className="max-w-2xl space-y-6"><div><p className="text-sm font-medium text-emerald-700">管理员</p><h1 className="mt-1 text-2xl font-semibold">{heading}</h1><p className="mt-1 text-sm text-zinc-500">配置 Kohame 的运行方式与协作体验。</p></div>
      {active === "general" && <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"><h2 className="font-semibold">基础信息</h2><Field label="站点名称" value={value.title} onChange={(title) => setValue({ ...value, title })} placeholder="Kohame" /><Field label="站点说明" value={value.description} onChange={(description) => setValue({ ...value, description })} placeholder="代码协作的家园" /></section>}
      {active === "security" && <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"><h2 className="font-semibold">注册与防滥用</h2><Toggle label="允许用户注册" description="开启后，访客可自行创建 Kohame 账户。" checked={value.allowRegistration} onChange={(allowRegistration) => setValue({ ...value, allowRegistration })} /><Toggle label="启用 hCaptcha" description="在注册页验证访客，降低自动注册风险。" checked={value.captchaEnabled} onChange={(captchaEnabled) => setValue({ ...value, captchaEnabled })} />{value.captchaEnabled && <div className="grid gap-3 sm:grid-cols-2"><Field label="hCaptcha Site Key" value={value.captchaSiteKey} onChange={(captchaSiteKey) => setValue({ ...value, captchaSiteKey })} placeholder="10000000-ffff-ffff-ffff-000000000001" /><Field label="hCaptcha Secret" value={value.captchaSecret} onChange={(captchaSecret) => setValue({ ...value, captchaSecret })} placeholder="密钥" type="password" /></div>}</section>}
      {active === "mail" && <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"><div><h2 className="font-semibold">SMTP 邮件服务</h2><p className="mt-1 text-sm text-zinc-500">用于注册后的邮箱验证。支持 STARTTLS，通常使用 587 端口。</p></div><div className="grid gap-3 sm:grid-cols-2"><Field label="SMTP 主机" value={value.smtpHost} onChange={(smtpHost) => setValue({ ...value, smtpHost })} placeholder="smtp.example.com" /><Field label="SMTP 端口" value={value.smtpPort} onChange={(smtpPort) => setValue({ ...value, smtpPort })} placeholder="587" /><Field label="SMTP 用户名" value={value.smtpUsername} onChange={(smtpUsername) => setValue({ ...value, smtpUsername })} placeholder="mailer@example.com" /><Field label="SMTP 密码" value={value.smtpPassword} onChange={(smtpPassword) => setValue({ ...value, smtpPassword })} placeholder="应用专用密码" type="password" /></div><Field label="发件人地址" value={value.smtpFrom} onChange={(smtpFrom) => setValue({ ...value, smtpFrom })} placeholder="noreply@example.com" /></section>}
      {active === "storage" && <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"><div><h2 className="font-semibold">仓库存储</h2><p className="mt-1 text-sm text-zinc-500">Kohame 在此位置保存裸 Git 仓库。现有仓库不会在保存时自动迁移。</p></div><Field label="仓库存储目录" value={value.repositoryRoot} onChange={(repositoryRoot) => setValue({ ...value, repositoryRoot })} placeholder="data/repos" /><p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950/50 dark:text-amber-200">变更前请先迁移数据并做好备份，再重启服务使配置生效。</p></section>}
      {message && <p className={message === "设置已保存。" ? "text-sm text-emerald-700" : "text-sm text-red-600"}>{message}</p>}<Button type="submit"><Settings />保存站点设置</Button>
    </form>
  </div>
}

function Toggle({ label, description, checked, onChange }: { label: string; description: string; checked: boolean; onChange: (value: boolean) => void }) {
  return <label className="flex cursor-pointer items-center justify-between gap-5 rounded-md border border-zinc-200 p-4 text-sm dark:border-zinc-700"><span><strong className="block">{label}</strong><span className="mt-1 block text-zinc-500">{description}</span></span><input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} className="size-4 accent-zinc-900 dark:accent-zinc-100" /></label>
}
