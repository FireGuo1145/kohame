# Kohame

Kohame is a compact self-hosted Git platform written in Go. It has a React + shadcn frontend embedded into the Go binary.

## Run

```powershell
.\build.ps1
.\kohame.exe
```

Open http://localhost:3000. The default [config.yml](D:\dev\kohame\config.yml) stores bare repositories in `./data/repos` and metadata in SQLite at `./data/kohame.db`.

On the first launch, complete the OOBE screen to create the administrator. Users can register when the administrator enables registration in **Site settings**.

Git HTTP requires an authenticated Kohame account. Git will prompt for the account password:

```powershell
git clone http://localhost:3000/git/my-repository
```

To build without starting it:

```powershell
.\build.ps1
```

Use `-config D:\path\to\config.yml` to select a configuration file.

## Configuration

`config.yml` is the source of runtime configuration. Relative SQLite paths and repository paths are resolved from the directory containing it.

```yaml
server:
  addr: ":3000"
storage:
  repository_root: data/repos
database:
  driver: sqlite # sqlite or pgsql
  dsn: data/kohame.db
```

For PostgreSQL, set `driver: pgsql` and use a PostgreSQL DSN, for example `postgres://kohame:secret@localhost:5432/kohame?sslmode=disable`.

## Frontend development

Start the Go API in one terminal, then the Vite server in another:

```powershell
.\kohame.exe
cd web
yarn dev
```

Vite proxies `/api` and `/git` to `http://localhost:3000`, so the frontend behaves as it does in the embedded production build.

## Forge features

- One-time OOBE administrator creation, registration, login, and HttpOnly sessions.
- OIDC login with multiple configurable identity providers and automatic account linking.
- Administrator-controlled title, description, and registration policy.
- GitHub-style repository actions: Star, bare-Git Fork, fork attribution, and in-app notifications for stars, forks, issues, and pull requests.
- Work dashboard plus user and organization home pages.
- Email verification with one-time, 24-hour links. Configure SMTP under **Site settings → SMTP 邮件服务** (host, port, username, password, and From address); STARTTLS is negotiated when the mail server advertises it.
- Repository creation plus Git smart HTTP with browser session or HTTP Basic authentication.
- Repository workflows with JSON event triggers (`push`, `issues`, `pull_request`, `release`, and `workflow_dispatch`) and run history.
- Repository-scoped issues, pull requests, releases, and activity-based contributor summaries.

This is a compact forge, not a full Gitea replacement yet: pull requests provide file diffs, comments, and server-side merge commits, while workflows provide lightweight repository automation rather than a full CI runner. Use HTTPS when serving it beyond a trusted local network.
