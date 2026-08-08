import { useEffect, useState } from "react"
import { Check } from "lucide-react"
import { api } from "@/lib/forge-api"

export default function VerifyRoutePage({ onDone }: { onDone: () => void }) {
  const [message, setMessage] = useState("正在验证邮件地址…")
  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("token")
    if (!token) { setMessage("验证链接缺少令牌。"); return }
    void api<{ message: string }>(`/api/auth/verify?token=${encodeURIComponent(token)}`)
      .then((value) => { setMessage(value.message); setTimeout(onDone, 1800) })
      .catch((cause: unknown) => setMessage(cause instanceof Error ? cause.message : "邮件验证失败。"))
  }, [onDone])
  return <div className="mx-auto grid min-h-[60svh] max-w-xl place-items-center px-5 text-center"><div className="rounded-2xl border border-zinc-200 bg-white p-8 shadow-sm dark:border-zinc-800 dark:bg-zinc-900"><Check className="mx-auto mb-4 size-8 text-emerald-600" /><h1 className="text-xl font-semibold">邮箱验证</h1><p className="mt-2 text-sm text-zinc-500">{message}</p></div></div>
}

