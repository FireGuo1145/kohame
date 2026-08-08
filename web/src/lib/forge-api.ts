import type { SiteSettings } from "@/lib/forge-types"

export async function api<T>(url: string, init?: RequestInit): Promise<T> {
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

export const when = (date: string) =>
  new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium" }).format(new Date(date))

export const defaultSiteSettings: SiteSettings = {
  title: "Kohame", description: "Self-hosted Git, kept simple", allowRegistration: true,
  smtpHost: "", smtpPort: "587", smtpUsername: "", smtpPassword: "", smtpFrom: "",
  captchaEnabled: false, captchaSiteKey: "", captchaSecret: "", repositoryRoot: "",
}