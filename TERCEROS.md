# De terceros

Zonda usa trabajo de otros autores. Ninguno viaja en este repositorio: las bibliotecas de Go las baja `go build`, y
el resto lo baja la receta de [CONSTRUIR.md](CONSTRUIR.md). Cada uno conserva su licencia, y todas permiten usarlos
dentro de un programa AGPL.

## Bibliotecas de Go (las baja `go build`)

| Módulo | Licencia |
|---|---|
| `gioui.org`, `gioui.org/x`, `gioui.org/shader` — la ventana | Unlicense o MIT |
| `github.com/go-text/typesetting` — el texto | Unlicense o BSD-3-Clause |
| `github.com/pion/*` — WebRTC (enlaces, voz, video) | MIT |
| `github.com/tphakala/go-opus` — Opus | BSD-3-Clause |
| `github.com/tphakala/simd` | MIT |
| `github.com/coder/websocket` — el tablero Nostr | ISC |
| `github.com/btcsuite/btcd/btcec/v2`, `.../chainhash/v2` — firmas de Nostr | ISC |
| `github.com/decred/dcrd/dcrec/secp256k1/v4`, `.../crypto/blake256` | ISC |
| `filippo.io/edwards25519` | BSD-3-Clause |
| `golang.org/x/crypto`, `x/image`, `x/net`, `x/sys`, `x/text`, `x/time`, `x/exp/shiny` | BSD-3-Clause |
| `github.com/google/uuid` | BSD-3-Clause |
| `github.com/wlynxg/anet` | BSD-3-Clause |
| `github.com/go-ole/go-ole` | MIT |

## Bibliotecas nativas (las baja la receta, de MSYS2)

| Biblioteca | Licencia |
|---|---|
| webrtc-audio-processing | BSD-3-Clause |
| libvpx (The WebM Project) | BSD-3-Clause, con su concesión de patentes |
| libyuv | BSD-3-Clause |
| libjpeg-turbo | IJG, BSD-3-Clause y zlib |
| abseil-cpp | Apache-2.0 |
| libstdc++ y libgcc (GCC) | GPL-3.0 con la GCC Runtime Library Exception |
| winpthreads (mingw-w64) | MIT y ZPL-2.1 |

Quien reparta Zonda ya compilado tiene que acompañar las licencias de estas bibliotecas: MSYS2 deja cada texto en
`C:\msys64\ucrt64\share\licenses\`.

## Tipografías (las baja la receta, de Google Fonts)

Todas bajo la SIL Open Font License 1.1:

| Familia | Copyright |
|---|---|
| Chakra Petch | 2018 The Chakra Petch Project Authors |
| Figtree | 2022 The Figtree Project Authors |
| Fredoka | 2016 The Fredoka Project Authors |
| IM Fell English | 2007 Igino Marini, con nombre de fuente reservado «IM FELL English Roman» |
| Sora | 2019 The Sora Project Authors |
