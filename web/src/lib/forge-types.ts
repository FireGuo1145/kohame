export type User = {
  id: number
  username: string
  email: string
  isAdmin: boolean
  emailVerified: boolean
}
export type OIDCProvider = {
  id: number
  slug: string
  name: string
  issuerUrl: string
  clientId: string
  clientSecret?: string
  scopes: string
  enabled: boolean
  createdAt?: string
  updatedAt?: string
}
export type Workflow = {
  id: number
  repositoryName: string
  name: string
  config: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}
export type WorkflowRun = {
  id: number
  workflowId: number
  repositoryName: string
  event: string
  status: string
  output: string
  startedAt: string
  finishedAt?: string
}
export type SiteSettings = {
  title: string
  description: string
  allowRegistration: boolean
  smtpHost: string
  smtpPort: string
  smtpUsername: string
  smtpPassword: string
  smtpFrom: string
  captchaEnabled: boolean
  captchaSiteKey: string
  captchaSecret: string
  repositoryRoot: string
  gravatarMirror: string
}
export type Repository = {
  scope: string
  name: string
  fullName: string
  updatedAt: string
  forkedFrom?: string
  stars: number
  forks: number
  starred?: boolean
}
export type TreeEntry = { name: string; path: string; type: string }
export type Language = { name: string; color: string; percentage: number }
export type Blob = { path: string; content: string; isText: boolean }
export type GitRef = { name: string; hash: string }
export type Commit = {
  hash: string
  subject: string
  author: string
  date: string
}
export type CommitFile = {
  path: string
  status: "added" | "modified" | "deleted" | "renamed"
  patch: string
}
export type CommitDetail = Commit & {
  body: string
  changes: string
  files: CommitFile[]
}
export type RepositorySettings = {
  description: string
  homepageUrl: string
  visibility: "public" | "private"
  defaultBranch: string
  topics: string[]
  issuesEnabled: boolean
  pullsEnabled: boolean
  releasesEnabled: boolean
  wikiEnabled: boolean
  autoCloseIssues: boolean
  allowForks: boolean
  archived: boolean
}
export type Collaborator = {
  username: string
  permission: "read" | "write" | "maintain" | "admin"
}
export type ProtectedBranch = {
  branch: string
  requirePullRequest: boolean
  requireApprovals: number
}
export type WikiPage = {
  slug: string
  title: string
  content?: string
  author: string
  createdAt: string
  updatedAt: string
}

export type CaptchaConfig = { enabled: boolean; siteKey: string }
export type Label = {
  id: number
  name: string
  color: string
  description: string
}
export type Issue = {
  id: number
  title: string
  body: string
  state: string
  author: string
  createdAt: string
  labels: Label[]
}
export type IssueComment = {
  id: number
  author: string
  body: string
  createdAt: string
}
export type PullRequest = {
  id: number
  title: string
  body: string
  sourceBranch: string
  targetBranch: string
  state: string
  author: string
  createdAt: string
}
export type PullRequestComment = {
  id: number
  author: string
  body: string
  createdAt: string
}
export type ReleaseAsset = {
  id: number
  fileName: string
  size: number
  url: string
}
export type Release = {
  id: number
  tagName: string
  title: string
  notes: string
  author: string
  createdAt: string
  assets: ReleaseAsset[]
}
export type SSHKey = {
  id: number
  title: string
  key: string
  createdAt: string
}
export type SSHInfo = { host: string; port: string }
export type Contributor = { username: string; contributions: number }
export type Notification = {
  id: number
  kind: string
  title: string
  body: string
  link: string
  isRead: boolean
  createdAt: string
}
export type Profile = {
  username: string
  displayName: string
  bio: string
  location: string
  website: string
  avatarUrl: string
  createdAt: string
  repositories: number
  stars: number
  followers: number
  following: number
  followed: boolean
}
export type Organization = {
  name: string
  role?: string
  followers: number
  followed: boolean
}
export type OrganizationMember = { username: string; role: string }
export type FollowTarget = { name: string; type: "user" | "organization" }
