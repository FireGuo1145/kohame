import { useEffect, useState, type FormEvent } from "react"
import {
  Database,
  Mail,
  Settings,
  ShieldCheck,
  KeyRound,
  Trash2,
  Plus,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Field } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { OIDCProvider, SiteSettings } from "@/lib/forge-types"

type Section = "general" | "security" | "mail" | "storage" | "oidc"

const sections: { id: Section; label: string; icon: typeof Settings }[] = [
  { id: "general", label: "常规设置", icon: Settings },
  { id: "security", label: "注册与安全", icon: ShieldCheck },
  { id: "mail", label: "邮件与通知", icon: Mail },
  { id: "storage", label: "存储与维护", icon: Database },
  { id: "oidc", label: "OIDC 登录", icon: KeyRound },
]

export default function SiteSettingsPage({
  site,
  onSaved,
}: {
  site: SiteSettings
  onSaved: (site: SiteSettings) => void
}) {
  const [value, setValue] = useState(site)
  const [active, setActive] = useState<Section>("general")
  const [message, setMessage] = useState("")
  const [providers, setProviders] = useState<OIDCProvider[]>([])
  const [provider, setProvider] = useState<OIDCProvider>({
    id: 0,
    slug: "",
    name: "",
    issuerUrl: "",
    clientId: "",
    clientSecret: "",
    scopes: "openid profile email",
    enabled: true,
  })
  useEffect(() => {
    void api<OIDCProvider[]>("/api/admin/oidc")
      .then(setProviders)
      .catch(() => setProviders([]))
  }, [])
  const saveProvider = async () => {
    try {
      const saved = await api<OIDCProvider>(
        provider.id ? `/api/admin/oidc/${provider.id}` : "/api/admin/oidc",
        {
          method: provider.id ? "PATCH" : "POST",
          body: JSON.stringify(provider),
        }
      )
      setProviders(
        provider.id
          ? providers.map((item) => (item.id === saved.id ? saved : item))
          : [saved, ...providers]
      )
      setProvider({
        id: 0,
        slug: "",
        name: "",
        issuerUrl: "",
        clientId: "",
        clientSecret: "",
        scopes: "openid profile email",
        enabled: true,
      })
      setMessage("OIDC 提供商已保存。")
    } catch (cause) {
      setMessage(
        cause instanceof Error ? cause.message : "无法保存 OIDC 提供商。"
      )
    }
  }
  const deleteProvider = async (id: number) => {
    await api(`/api/admin/oidc/${id}`, { method: "DELETE" })
    setProviders(providers.filter((item) => item.id !== id))
    setMessage("OIDC 提供商已删除。")
  }

  useEffect(() => {
    void api<SiteSettings>("/api/admin/settings")
      .then(setValue)
      .catch((cause: unknown) =>
        setMessage(cause instanceof Error ? cause.message : "无法加载设置。")
      )
  }, [])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    try {
      const saved = await api<SiteSettings>("/api/admin/settings", {
        method: "PATCH",
        body: JSON.stringify(value),
      })
      setValue(saved)
      onSaved(saved)
      setMessage("设置已保存。")
    } catch (cause) {
      setMessage(cause instanceof Error ? cause.message : "无法保存设置。")
    }
  }

  const heading =
    sections.find((item) => item.id === active)?.label || "站点设置"
  return (
    <div className="mx-auto grid max-w-6xl gap-8 px-5 py-10 md:grid-cols-[200px_1fr]">
      <aside className="h-fit rounded-lg border border-zinc-200 bg-white p-2 text-sm dark:border-zinc-800 dark:bg-zinc-900">
        <p className="px-3 py-2 text-xs font-semibold tracking-[0.12em] text-zinc-400 uppercase">
          站点管理
        </p>
        {sections.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => {
              setActive(id)
              setMessage("")
            }}
            className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left transition-colors ${active === id ? "bg-zinc-900 font-medium text-white dark:bg-zinc-100 dark:text-zinc-950" : "text-zinc-600 hover:bg-zinc-100 dark:text-zinc-400 dark:hover:bg-zinc-800"}`}
          >
            <Icon className="size-3.5" />
            {label}
          </button>
        ))}
      </aside>
      <form onSubmit={submit} className="max-w-2xl space-y-6">
        <div>
          <p className="text-sm font-medium text-emerald-700">管理员</p>
          <h1 className="mt-1 text-2xl font-semibold">{heading}</h1>
          <p className="mt-1 text-sm text-zinc-500">
            配置 Kohame 的运行方式与协作体验。
          </p>
        </div>
        {active === "general" && (
          <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <h2 className="font-semibold">基础信息</h2>
            <Field
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
            <Field
              label="Gravatar 镜像地址"
              value={value.gravatarMirror}
              onChange={(gravatarMirror) =>
                setValue({ ...value, gravatarMirror })
              }
              placeholder="https://www.gravatar.com/avatar/"
            />
          </section>
        )}
        {active === "security" && (
          <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <h2 className="font-semibold">注册与防滥用</h2>
            <Toggle
              label="允许用户注册"
              description="开启后，访客可自行创建 Kohame 账户。"
              checked={value.allowRegistration}
              onChange={(allowRegistration) =>
                setValue({ ...value, allowRegistration })
              }
            />
            <Toggle
              label="启用 hCaptcha"
              description="在注册页验证访客，降低自动注册风险。"
              checked={value.captchaEnabled}
              onChange={(captchaEnabled) =>
                setValue({ ...value, captchaEnabled })
              }
            />
            {value.captchaEnabled && (
              <div className="grid gap-3 sm:grid-cols-2">
                <Field
                  label="hCaptcha Site Key"
                  value={value.captchaSiteKey}
                  onChange={(captchaSiteKey) =>
                    setValue({ ...value, captchaSiteKey })
                  }
                  placeholder="10000000-ffff-ffff-ffff-000000000001"
                />
                <Field
                  label="hCaptcha Secret"
                  value={value.captchaSecret}
                  onChange={(captchaSecret) =>
                    setValue({ ...value, captchaSecret })
                  }
                  placeholder="密钥"
                  type="password"
                />
              </div>
            )}
          </section>
        )}
        {active === "mail" && (
          <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <div>
              <h2 className="font-semibold">SMTP 邮件服务</h2>
              <p className="mt-1 text-sm text-zinc-500">
                用于注册后的邮箱验证。支持 STARTTLS，通常使用 587 端口。
              </p>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field
                label="SMTP 主机"
                value={value.smtpHost}
                onChange={(smtpHost) => setValue({ ...value, smtpHost })}
                placeholder="smtp.example.com"
              />
              <Field
                label="SMTP 端口"
                value={value.smtpPort}
                onChange={(smtpPort) => setValue({ ...value, smtpPort })}
                placeholder="587"
              />
              <Field
                label="SMTP 用户名"
                value={value.smtpUsername}
                onChange={(smtpUsername) =>
                  setValue({ ...value, smtpUsername })
                }
                placeholder="mailer@example.com"
              />
              <Field
                label="SMTP 密码"
                value={value.smtpPassword}
                onChange={(smtpPassword) =>
                  setValue({ ...value, smtpPassword })
                }
                placeholder="应用专用密码"
                type="password"
              />
            </div>
            <Field
              label="发件人地址"
              value={value.smtpFrom}
              onChange={(smtpFrom) => setValue({ ...value, smtpFrom })}
              placeholder="noreply@example.com"
            />
          </section>
        )}
        {active === "storage" && (
          <section className="space-y-4 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <div>
              <h2 className="font-semibold">仓库存储</h2>
              <p className="mt-1 text-sm text-zinc-500">
                Kohame 在此位置保存裸 Git 仓库。现有仓库不会在保存时自动迁移。
              </p>
            </div>
            <Field
              label="仓库存储目录"
              value={value.repositoryRoot}
              onChange={(repositoryRoot) =>
                setValue({ ...value, repositoryRoot })
              }
              placeholder="data/repos"
            />
            <Field
              label="工作流目录"
              value={value.workflowDirectory}
              onChange={(workflowDirectory) =>
                setValue({ ...value, workflowDirectory })
              }
              placeholder="/.kohame/workflow"
            />
            <p className="text-sm text-zinc-500">
              所有仓库统一使用此仓库相对目录，遵循 GitHub Actions 的 YAML 工作流格式。
            </p>
            <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950/50 dark:text-amber-200">
              变更前请先迁移数据并做好备份，再重启服务使配置生效。
            </p>
          </section>
        )}
        {active === "oidc" && (
          <section className="space-y-5 rounded-lg border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900">
            <div>
              <h2 className="font-semibold">OIDC 登录提供商</h2>
              <p className="mt-1 text-sm text-zinc-500">
                可以配置多个兼容 OpenID Connect
                的身份提供商。首次登录会自动创建账号。
              </p>
            </div>
            <div className="space-y-2">
              {providers.map((item) => (
                <div
                  key={item.id}
                  className="flex items-center gap-3 rounded-lg border border-zinc-200 p-3 text-sm dark:border-zinc-700"
                >
                  <span className="min-w-0 flex-1">
                    <strong>{item.name}</strong>
                    <span className="ml-2 text-xs text-zinc-500">
                      {item.issuerUrl}
                    </span>
                  </span>
                  <button
                    onClick={() => setProvider(item)}
                    aria-label={`编辑 ${item.name}`}
                  >
                    <Settings className="size-4 text-zinc-500" />
                  </button>
                  <button
                    onClick={() => void deleteProvider(item.id)}
                    aria-label={`删除 ${item.name}`}
                  >
                    <Trash2 className="size-4 text-red-600" />
                  </button>
                </div>
              ))}
              {!providers.length && (
                <p className="text-sm text-zinc-500">尚未配置提供商。</p>
              )}
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field
                label="标识"
                value={provider.slug}
                onChange={(slug) => setProvider({ ...provider, slug })}
                placeholder="github"
              />
              <Field
                label="显示名称"
                value={provider.name}
                onChange={(name) => setProvider({ ...provider, name })}
                placeholder="GitHub"
              />
              <Field
                label="Issuer URL"
                value={provider.issuerUrl}
                onChange={(issuerUrl) =>
                  setProvider({ ...provider, issuerUrl })
                }
                placeholder="https://accounts.example.com"
              />
              <Field
                label="Client ID"
                value={provider.clientId}
                onChange={(clientId) => setProvider({ ...provider, clientId })}
                placeholder="客户端 ID"
              />
              <Field
                label="Client Secret"
                value={provider.clientSecret || ""}
                onChange={(clientSecret) =>
                  setProvider({ ...provider, clientSecret })
                }
                placeholder={provider.id ? "留空表示保持不变" : "客户端密钥"}
                type="password"
              />
              <Field
                label="Scopes"
                value={provider.scopes}
                onChange={(scopes) => setProvider({ ...provider, scopes })}
                placeholder="openid profile email"
              />
            </div>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={provider.enabled}
                onChange={(event) =>
                  setProvider({ ...provider, enabled: event.target.checked })
                }
              />
              启用此提供商
            </label>
            <div className="flex gap-2">
              <Button type="button" onPress={() => void saveProvider()}>
                <Plus />
                {provider.id ? "保存提供商" : "添加提供商"}
              </Button>
              {provider.id > 0 && (
                <Button
                  type="button"
                  variant="outline"
                  onPress={() =>
                    setProvider({
                      id: 0,
                      slug: "",
                      name: "",
                      issuerUrl: "",
                      clientId: "",
                      clientSecret: "",
                      scopes: "openid profile email",
                      enabled: true,
                    })
                  }
                >
                  取消编辑
                </Button>
              )}
            </div>
          </section>
        )}
        {message && (
          <p
            className={
              message === "设置已保存。"
                ? "text-sm text-emerald-700"
                : "text-sm text-red-600"
            }
          >
            {message}
          </p>
        )}
        <Button type="submit">
          <Settings />
          保存站点设置
        </Button>
      </form>
    </div>
  )
}

function Toggle({
  label,
  description,
  checked,
  onChange,
}: {
  label: string
  description: string
  checked: boolean
  onChange: (value: boolean) => void
}) {
  return (
    <label className="flex cursor-pointer items-center justify-between gap-5 rounded-md border border-zinc-200 p-4 text-sm dark:border-zinc-700">
      <span>
        <strong className="block">{label}</strong>
        <span className="mt-1 block text-zinc-500">{description}</span>
      </span>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="size-4 accent-zinc-900 dark:accent-zinc-100"
      />
    </label>
  )
}
