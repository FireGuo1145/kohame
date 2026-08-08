$ErrorActionPreference = "Stop"

Push-Location "$PSScriptRoot\web"
try {
  yarn build
} finally {
  Pop-Location
}

if (Test-Path "$PSScriptRoot\internal\web\dist") {
  Remove-Item -LiteralPath "$PSScriptRoot\internal\web\dist" -Recurse -Force
}
New-Item -ItemType Directory -Force "$PSScriptRoot\internal\web\dist" | Out-Null
Copy-Item -Path "$PSScriptRoot\web\dist\*" -Destination "$PSScriptRoot\internal\web\dist" -Recurse -Force
go build -o "$PSScriptRoot\kohame.exe" "$PSScriptRoot\cmd\kohame"
