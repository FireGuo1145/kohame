# Kohame Runner

The runner is a separate Go service for executing repository workflows. Configure the Kohame administrator settings with the runner URL and the same token, then start:

```powershell
go run ./runner -addr :8090 -token change-me
```

It accepts repository snapshots from Kohame and supports shell steps plus composite and Node-based `uses` Actions referenced as `owner/repository/path@ref`. The runner executes jobs in temporary directories; deploy it in an isolated environment with the tools and network access required by your Actions.
