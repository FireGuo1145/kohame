# Kohame Runner

Create a Runner in **Personal settings -> Actions Runner** and copy the one-time token. The Runner initiates every connection to Kohame; the server never needs network access to the Runner machine.

```powershell
go run ./runner -server https://kohame.example.com -token your-one-time-runner-token
```

The Runner polls for jobs, receives only work for repositories owned by its registering user, and reports the result back over the same HTTPS connection. It supports shell steps plus composite and Node-based `uses` Actions referenced as `owner/repository/path@ref`. Run it in an isolated environment with the tools and network access required by your Actions.
