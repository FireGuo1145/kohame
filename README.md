# Kohame

Kohame is a compact self-hosted Git platform written in Go. It has a React + shadcn frontend embedded into the Go binary.

## Run

```powershell
.\build.ps1
.\kohame.exe
```

Open http://localhost:3000. The default [config.yml](D:\dev\kohame\config.yml) stores bare repositories in `./data/repos` and metadata in SQLite at `./data/kohame.db`.

Create a repository in the UI, then use standard Git HTTP:

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

## Current scope

This first version provides local repository creation, listing, and standard Git smart HTTP clone/fetch/push. It intentionally does not yet include accounts, authorization, pull requests, issues, or a repository file browser; do not expose it to an untrusted network until authentication is added.
