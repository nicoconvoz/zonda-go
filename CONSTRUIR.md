# Cómo construir Zonda

Este repositorio trae **solo el código de Zonda**. Lo que es de otros —las bibliotecas de WebRTC, libvpx y libyuv, sus
DLL de ejecución y las tipografías— no viaja acá: cada uno lo baja de su fuente original con la receta de abajo.

## Qué hace falta

- **Windows 10 u 11**, 64 bits.
- **Go 1.27 o más nuevo** — <https://go.dev/dl/>
- **MSYS2** instalado en `C:\msys64` — <https://www.msys2.org>. Da el compilador de C++ y las bibliotecas de terceros.
- **Git Bash** (viene con Git para Windows), para correr la receta.

## Los tres pasos

Desde la carpeta del repositorio:

```bash
bash herramientas/terceros.sh
```

Instala con `pacman` los paquetes de MSYS2 que hacen falta, compila los dos puentes propios de Zonda
(`apm/zapm.cc` → `zapm.dll`, `video/zvid.cc` → `zvid.dll`), copia las DLL de ejecución a `voz/apm_dlls/` y baja las
tipografías de Google Fonts a `interfaz/fuentes/`, con las licencias de todo eso en `licencias/terceros/`.

```powershell
.\construir.ps1
```

Compila el programa en `dist\`: `zondago.exe` con sus DLL al lado y la carpeta `licencias\` (la de Zonda, la de
cada módulo de Go y las de terceros), que viaja con el programa.

```powershell
go test ./...
```

Corre las pruebas (las de video usan las DLL de `voz/apm_dlls/`).

## Qué baja la receta, y de dónde

| Qué | De dónde | Para qué |
|---|---|---|
| `webrtc-audio-processing-1` | MSYS2 (UCRT64) | cancelador de eco, supresor de ruido y ganancia de WebRTC, dentro de `zapm.dll` |
| `libvpx`, `libyuv`, `libjpeg-turbo` | MSYS2 (UCRT64) | video VP8 de la pantalla compartida, enlazados dentro de `zvid.dll` |
| `abseil-cpp` | MSYS2 (UCRT64) | encabezados que pide WebRTC para compilar |
| `gcc` y sus DLL de ejecución | MSYS2 (UCRT64) | el compilador, y `libstdc++-6.dll`, `libgcc_s_seh-1.dll`, `libwinpthread-1.dll` |
| Chakra Petch, Figtree, Fredoka, IM Fell English, Sora | Google Fonts | las tipografías de los estilos |

Las bibliotecas de Go las baja solo `go build` desde `go.mod`.

Cada pieza sigue con su propia licencia: el detalle está en [TERCEROS.md](TERCEROS.md).
