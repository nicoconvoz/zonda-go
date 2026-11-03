# Builds Zonda Go into dist\: the program and, next to it, the WebRTC libraries it loads (audio processing, video).
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
New-Item -ItemType Directory -Force dist | Out-Null
# The program keeps its name: when it is open it cannot be overwritten, so the build stops and asks to close it.
if (Get-Process zondago -ErrorAction SilentlyContinue | Where-Object { $_.Path -like "$PSScriptRoot\dist\*" }) {
    Write-Host "Zonda Go está abierto desde dist: cerralo y volvé a compilar."
    exit 1
}
Get-ChildItem dist\zondago.viejo*.exe -ErrorAction SilentlyContinue | Remove-Item -ErrorAction SilentlyContinue
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
