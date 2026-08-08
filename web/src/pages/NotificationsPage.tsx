import { useEffect, useState } from "react"
import { Bell } from "lucide-react"
import { Empty } from "@/components/forge-ui"
import { api, when } from "@/lib/forge-api"
import type { Notification } from "@/lib/forge-types"

export default function NotificationsPage({ onOpen }: { onOpen: (link: string) => void }) {
  const [items,setItems]=useState<Notification[]>([]);const[message,setMessage]=useState("");useEffect(()=>{void api<Notification[]>("/api/notifications").then(setItems).catch((cause:unknown)=>setMessage(cause instanceof Error?cause.message:"请先登录以查看通知。"))},[])
  return <div className="mx-auto max-w-3xl px-5 py-9"><h1 className="text-2xl font-semibold">通知</h1><p className="mt-1 text-sm text-zinc-500">Star、Fork 和仓库协作的最新动态。</p>{message?<p className="mt-5 rounded-xl bg-red-50 p-4 text-sm text-red-700">{message}</p>:<div className="mt-6 overflow-hidden rounded-2xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">{items.length?items.map(item=><button key={item.id} onClick={async()=>{if(!item.isRead){await api<void>(`/api/notifications/${item.id}/read`,{method:"PATCH"});setItems(items.map(value=>value.id===item.id?{...value,isRead:true}:value))}onOpen(item.link)}} className={`block w-full border-b border-zinc-100 p-4 text-left last:border-0 dark:border-zinc-800 ${item.isRead?"":"bg-emerald-50/60 dark:bg-emerald-950/20"}`}><strong className="text-sm">{item.title}</strong><p className="mt-1 text-sm text-zinc-500">{item.body} · {when(item.createdAt)}</p></button>):<Empty icon={<Bell/>} title="还没有通知" text="仓库的新 Star、Fork 和协作动态会显示在这里。"/>}</div>}</div>
}

