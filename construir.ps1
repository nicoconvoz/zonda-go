# Builds Zonda Go into dist\: the program and, next to it, the WebRTC libraries it loads (audio processing, video).
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force dist | Out-Null
go build -ldflags "-H windowsgui -s -w" -o dist\zondago.exe .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Copy-Item voz\apm_dlls\*.dll dist\ -Force
Get-ChildItem dist | Select-Object Name, Length
