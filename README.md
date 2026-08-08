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
- Administrator-controlled title, description, and registration policy.
- Repository creation plus Git smart HTTP with browser session or HTTP Basic authentication.
- Repository-scoped issues, pull requests, releases, and activity-based contributor summaries.

This is a compact forge, not a full Gitea replacement yet: pull requests track review metadata but do not yet calculate diffs, comments, or perform server-side merges. Use HTTPS when serving it beyond a trusted local network.
