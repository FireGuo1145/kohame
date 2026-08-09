import { useEffect, useState } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { Building2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Avatar, Loading, PageMessage, RepoCards } from "@/components/forge-ui"
import { api } from "@/lib/forge-api"
import type { Organization, OrganizationMember, Repository, User } from "@/lib/forge-types"
import ProfileRoutePage from "@/pages/ProfileRoutePage"

function OrganizationPage({ name, onOpen, user, repositoriesOnly }: { name: string; onOpen: (name: string) => void; user: { username: string } | null; repositoriesOnly: boolean }) {
  const navigate = useNavigate()
  const [org,setOrg]=useState<Organization|null>(null);const[members,setMembers]=useState<OrganizationMember[]>([]);const[repos,setRepos]=useState<Repository[]>([]);const[message,setMessage]=useState("")
  useEffect(()=>{void Promise.all([api<Organization>(`/api/organizations/${name}`),api<OrganizationMember[]>(`/api/organizations/${name}/members`),api<Repository[]>(`/api/organizations/${name}/repos`)]).then(([o,m,r])=>{setOrg(o);setMembers(m);setRepos(r)}).catch((cause:unknown)=>setMessage(cause instanceof Error?cause.message:"无法加载组织主页。"))},[name])
  if(message)return <PageMessage title="组织主页" message={message}/>;if(!org)return <Loading/>;return <div className="mx-auto max-w-7xl px-5 py-9"><section className="flex items-center gap-4 rounded-2xl border border-zinc-200 bg-white p-6 dark:border-zinc-800 dark:bg-zinc-900"><span className="grid size-16 place-items-center rounded-2xl bg-violet-100 text-violet-700 dark:bg-violet-950"><Building2 /></span><div className="min-w-0 flex-1"><h1 className="text-2xl font-semibold">{org.name}</h1><p className="mt-1 text-sm text-zinc-500">组织主页 · {members.length} 位成员 · {org.followers} 位关注者</p></div>{user&&<Button variant={org.followed?"secondary":"outline"} onPress={async()=>{try{const result=await api<{followed:boolean;followers:number}>(`/api/organizations/${name}/follow`,{method:"POST"});setOrg({...org,...result})}catch(cause){setMessage(cause instanceof Error?cause.message:"操作失败。")}}}>{org.followed?"已关注":"关注"}</Button>}</section><nav className="mt-5 flex gap-1 border-b border-zinc-200 dark:border-zinc-800"><button onClick={() => navigate(`/${name}`)} className={`border-b-2 px-3 py-2 text-sm ${!repositoriesOnly ? "border-emerald-600 font-medium" : "border-transparent text-zinc-500"}`}>概览</button><button onClick={() => navigate(`/orgs/${name}/repositories`)} className={`border-b-2 px-3 py-2 text-sm ${repositoriesOnly ? "border-emerald-600 font-medium" : "border-transparent text-zinc-500"}`}>仓库 {repos.length}</button></nav><div className="mt-8 grid gap-8 lg:grid-cols-[1fr_280px]"><section><h2 className="mb-3 font-semibold">{repositoriesOnly ? "所有仓库" : "仓库"}</h2><RepoCards repos={repos} onOpen={onOpen}/></section>{!repositoriesOnly&&<aside><h2 className="mb-3 font-semibold">成员</h2><div className="overflow-hidden rounded-xl border border-zinc-200 bg-white dark:border-zinc-800 dark:bg-zinc-900">{members.map(member=><div key={member.username} className="flex items-center justify-between border-b border-zinc-100 p-3 text-sm last:border-0 dark:border-zinc-800"><span className="flex items-center gap-2"><Avatar name={member.username}/>{member.username}</span><span className="text-xs text-zinc-500">{member.role}</span></div>)}</div></aside>}</div></div>
}

export default function OrganizationRoutePage({ onOpen, user, repositoriesOnly = false }: { onOpen: (name: string) => void; user: User | null; repositoriesOnly?: boolean }) {
  const { name: routeName, username } = useParams()
  const name = routeName || username
  const [organization, setOrganization] = useState<boolean | null>(null)
  useEffect(() => { if (!name) return; void api<Organization>(`/api/organizations/${name}`).then(() => setOrganization(true)).catch(() => setOrganization(false)) }, [name])
  if (!name || organization === null) return <Loading />
  return organization ? <OrganizationPage name={name} onOpen={onOpen} user={user} repositoriesOnly={repositoriesOnly} /> : <ProfileRoutePage user={user} onOpen={onOpen} />
}
