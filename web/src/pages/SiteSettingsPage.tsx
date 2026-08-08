import { useEffect, useState, type FormEvent } from "react"
import { Settings } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Field } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { SiteSettings } from "@/lib/forge-types"

export default function SiteSettingsPage({
  site,
  onSaved,
}: {
  site: SiteSettings
  onSaved: (site: SiteSettings) => void
}) {
  const [value, setValue] = useState(site)
  const [message, setMessage] = useState("")
  useEffect(() => {
    void api<SiteSettings>("/api/admin/settings")
      .then(setValue)
      .catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "Could not load settings."))
  }, [])
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
    <div className="mx-auto grid max-w-6xl gap-8 px-5 py-10 md:grid-cols-[200px_1fr]">
      <aside className="h-fit rounded-xl border border-zinc-200 bg-white p-3 text-sm dark:border-zinc-800 dark:bg-zinc-900"><p className="px-3 py-2 font-semibold">站点管理</p><p className="rounded-lg bg-zinc-100 px-3 py-2 font-medium dark:bg-zinc-800">常规设置</p><p className="px-3 py-2 text-zinc-500">注册与安全</p><p className="px-3 py-2 text-zinc-500">邮件与通知</p><p className="px-3 py-2 text-zinc-500">存储与维护</p></aside>
      <div>
      <div className="mb-6">
        <p className="text-sm font-medium text-emerald-700">管理员</p>
        <h1 className="text-2xl font-semibold">站点设置</h1>
        <p className="mt-1 text-sm text-zinc-500">
          配置站点品牌、账户注册、邮件投递与防滥用策略。
        </p>
      </div>
      <form
        onSubmit={submit}
        className="space-y-5 rounded-2xl border border-zinc-200 bg-white p-6 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"
      >
        <section className="space-y-4 rounded-xl border border-zinc-200 p-5 dark:border-zinc-700"><h2 className="font-semibold">基础信息</h2><Field
          label="站点名称"
          value={value.title}
          onChange={(title) => setValue({ ...value, title })}
          placeholder="Kohame"
        />
        <Field
          label="站点说明"
          value={value.description}
          onChange={(description) => setValue({ ...value, description })}
          placeholder="代码协作的家园"
        />
        <Field label="仓库存储目录" value={value.repositoryRoot} onChange={(repositoryRoot)=>setValue({...value,repositoryRoot})} placeholder="data/repos" />
        <p className="text-xs text-zinc-500">修改存储目录不会自动迁移已存在仓库；迁移前请先备份数据。</p></section>
        <section className="space-y-4 rounded-xl border border-zinc-200 p-5 dark:border-zinc-700"><h2 className="font-semibold">注册与防滥用</h2>
        <label className="flex cursor-pointer items-center justify-between gap-5 rounded-xl bg-zinc-50 p-4 text-sm dark:bg-zinc-950">
          <span>
            <strong className="block">允许用户注册</strong>
            <span className="text-zinc-500">
              开启后，访客可自行创建 Kohame 账户。
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
        <label className="flex cursor-pointer items-center justify-between gap-5 rounded-xl bg-zinc-50 p-4 text-sm dark:bg-zinc-950"><span><strong className="block">启用 hCaptcha</strong><span className="text-zinc-500">在注册页验证访客，降低机器人注册风险。</span></span><input type="checkbox" checked={value.captchaEnabled} onChange={(event)=>setValue({...value,captchaEnabled:event.target.checked})} className="size-4" /></label>
        {value.captchaEnabled&&<div className="grid gap-3 sm:grid-cols-2"><Field label="hCaptcha Site Key" value={value.captchaSiteKey} onChange={(captchaSiteKey)=>setValue({...value,captchaSiteKey})} placeholder="10000000-ffff-ffff-ffff-000000000001"/><Field label="hCaptcha Secret" value={value.captchaSecret} onChange={(captchaSecret)=>setValue({...value,captchaSecret})} placeholder="密钥" type="password"/></div>}</section>
        <div className="rounded-xl border border-zinc-200 p-4 dark:border-zinc-700">
          <div className="mb-4"><h3 className="font-medium">SMTP 邮件服务</h3><p className="mt-1 text-sm text-zinc-500">用于注册后的邮箱验证。支持 STARTTLS（通常为 587 端口）。</p></div>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="SMTP 主机" value={value.smtpHost} onChange={(smtpHost) => setValue({ ...value, smtpHost })} placeholder="smtp.example.com" />
            <Field label="SMTP 端口" value={value.smtpPort} onChange={(smtpPort) => setValue({ ...value, smtpPort })} placeholder="587" />
            <Field label="SMTP 用户名" value={value.smtpUsername} onChange={(smtpUsername) => setValue({ ...value, smtpUsername })} placeholder="mailer@example.com" />
            <Field label="SMTP 密码" value={value.smtpPassword} onChange={(smtpPassword) => setValue({ ...value, smtpPassword })} placeholder="应用专用密码" type="password" />
          </div>
          <div className="mt-3"><Field label="发件人地址" value={value.smtpFrom} onChange={(smtpFrom) => setValue({ ...value, smtpFrom })} placeholder="noreply@example.com" /></div>
        </div>
        {message && <p className="text-sm text-emerald-700">{message}</p>}
        <Button type="submit">
          <Settings />
          保存站点设置
        </Button>
      </form>
      </div>
    </div>
  )
}

