# Zonda

[![DOI](https://zenodo.org/badge/DOI/10.5281/zenodo.23921404.svg)](https://doi.org/10.5281/zenodo.23921404)

Salas, chat, llamadas y pantalla compartida, **de dispositivo a dispositivo**, sin servidores, sin cuentas y sin
publicidad. Este es Zonda para Windows, escrito entero en Go: la ventana la dibuja [Gio](https://gioui.org), los
enlaces son WebRTC ([pion](https://github.com/pion/webrtc)) y la voz pasa por el procesamiento de audio de WebRTC.

**Descargar el instalador:** <https://nicoconvoz.github.io/zonda-web/>

## Qué hay acá

| Carpeta | Qué es |
|---|---|
| `nucleo/` | identidad, firmas y sellos, salas, chats, la malla y el tablero Nostr |
| `voz/` | audio de Windows, Opus, el colchón de llegada, el árbol de reenvío y la pantalla compartida |
| `interfaz/` | las pantallas, los estilos y los íconos |
| `bandeja/` | el ícono junto al reloj |
| `apm/`, `video/` | los dos puentes en C++ hacia WebRTC, libvpx y libyuv |
| `diag/` | el diagnóstico (`ZONDAGO_DIAG=1`) |

## Construirlo

La receta completa está en [CONSTRUIR.md](CONSTRUIR.md). En corto: `bash herramientas/terceros.sh` baja lo que es de
terceros, y `.\construir.ps1` compila.

## Licencia

- **El código:** [AGPL-3.0-or-later](LICENSE). Libre de usar, estudiar y modificar; si lo distribuís o otros se
  conectan por red con tu versión modificada, tenés que ofrecer su fuente.
- **Para uso cerrado:** [licencia comercial](LICENCIA-COMERCIAL.md).
- **De terceros:** cada biblioteca y tipografía con la suya, en [TERCEROS.md](TERCEROS.md).

Copyright © 2026 Jesús Nicolás Astorga y RESOURCES OPEN DOORS S.A.S — ver [NOTICE](NOTICE) y
[LICENCIAS.md](LICENCIAS.md). Para citarlo, [CITATION.cff](CITATION.cff).
