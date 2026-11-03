#!/usr/bin/env bash
# Gets what Zonda uses from others and does not travel in this repository: the WebRTC audio processing, libvpx and
# libyuv built into Zonda's two bridges (zapm.dll, zvid.dll), the runtime DLLs they need, and the typefaces.
# Everything comes from its own source: MSYS2's packages and Google Fonts.
#
# Run it from the repository's root, with MSYS2 installed in C:\msys64 (https://www.msys2.org):
#
#	bash herramientas/terceros.sh
#
# Then .\construir.ps1 builds the program into dist\.
set -euo pipefail

MSYS="${MSYS:-/c/msys64}"
UCRT="$MSYS/ucrt64"
export PATH="$UCRT/bin:$MSYS/usr/bin:$PATH"

echo "== paquetes de MSYS2 (UCRT64)"
pacman -S --needed --noconfirm \
	mingw-w64-ucrt-x86_64-gcc \
	mingw-w64-ucrt-x86_64-webrtc-audio-processing-1 \
	mingw-w64-ucrt-x86_64-abseil-cpp \
	mingw-w64-ucrt-x86_64-libvpx \
	mingw-w64-ucrt-x86_64-libyuv \
	mingw-w64-ucrt-x86_64-libjpeg-turbo

mkdir -p voz/apm_dlls

echo "== zapm.dll: el procesamiento de audio de WebRTC (cancelador de eco, ruido, ganancia)"
g++ -O2 -shared -std=c++17 -DWEBRTC_WIN -I"$UCRT/include/webrtc-audio-processing-1" \
	apm/zapm.cc -o voz/apm_dlls/zapm.dll \
	-lwebrtc-audio-processing-1 -static-libgcc -static-libstdc++

echo "== zvid.dll: VP8 de libvpx y conversión de color de libyuv, enlazados adentro"
g++ -O2 -shared -std=c++17 video/zvid.cc -o voz/apm_dlls/zvid.dll \
	-Wl,-Bstatic -lyuv -lvpx -ljpeg -Wl,-Bdynamic -static-libgcc -static-libstdc++
strip voz/apm_dlls/zvid.dll

echo "== las DLL que esas dos necesitan al correr"
for dll in libwebrtc-audio-processing-1-3.dll libstdc++-6.dll libgcc_s_seh-1.dll libwinpthread-1.dll; do
	cp "$UCRT/bin/$dll" voz/apm_dlls/
done

echo "== tipografías de Google Fonts (SIL Open Font License)"
mkdir -p interfaz/fuentes
# Each family in the weights the skins use. An old browser name makes Google answer with TrueType files.
for fam in "Chakra Petch:400;600;700" "Figtree:400;600;800" "Fredoka:400;600" "IM Fell English:400" "Sora:400;600;700"; do
	nombre="${fam%%:*}"
	pesos="${fam#*:}"
	css=$(curl -fsS -A "Mozilla/4.0" "https://fonts.googleapis.com/css2?family=${nombre// /+}:wght@${pesos}")
	archivo="${nombre// /}"
	# The CSS lists one @font-face per weight, in order: its weight and its url(...).
	paste <(grep -o "font-weight: [0-9]*" <<<"$css" | grep -o "[0-9]*") \
		<(grep -o "url([^)]*)" <<<"$css" | sed 's/url(//; s/)//') |
		while read -r peso url; do
			curl -fsS "$url" -o "interfaz/fuentes/${archivo}-${peso}.ttf"
			echo "   ${archivo}-${peso}.ttf"
		done
done

echo "listo: voz/apm_dlls y interfaz/fuentes completos"
