# Builds Zonda Go into dist\: the program and, next to it, the WebRTC libraries it loads (audio processing, video).
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force dist | Out-Null
# A running program cannot be overwritten, but it can be renamed: the open window keeps working and the next start
# opens the new build.
Remove-Item dist\zondago.viejo.exe -ErrorAction SilentlyContinue
if (Test-Path dist\zondago.exe) { Rename-Item dist\zondago.exe zondago.viejo.exe }
go build -ldflags "-H windowsgui -s -w" -o dist\zondago.exe .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
# Only the libraries that changed are copied: one the open window has loaded cannot be overwritten.
foreach ($dll in Get-ChildItem voz\apm_dlls\*.dll) {
    $destino = Join-Path dist $dll.Name
    if (-not (Test-Path $destino) -or (Get-FileHash $destino).Hash -ne (Get-FileHash $dll.FullName).Hash) {
        Copy-Item $dll.FullName $destino -Force
    }
}
Get-ChildItem dist | Select-Object Name, Length
