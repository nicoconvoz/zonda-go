// zvid: a C bridge over the video components libwebrtc uses for screen sharing: libvpx (Google's VP8 codec) and
// libyuv (Google's color conversion and scaling), so Go can call them with syscall and no cgo.
//
// Frames go in and out as RGBA (4 bytes per pixel, R G B A in memory: libyuv calls it ABGR).
#include <cstdint>
#include <cstring>
#include <cstdlib>
#include <vpx/vpx_encoder.h>
#include <vpx/vpx_decoder.h>
#include <vpx/vp8cx.h>
#include <vpx/vp8dx.h>
#include <libyuv.h>

struct Codificador {
  vpx_codec_ctx_t ctx;
  vpx_image_t img;
  int w, h;
  int64_t pts;
};

struct Decodificador {
  vpx_codec_ctx_t ctx;
};

extern "C" {

// zvid_cod_crear: a VP8 encoder for w x h frames (even sizes) at about fps frames a second and kbps, tuned like
// libwebrtc's screen content (real time, low latency, no frame lag).
__declspec(dllexport) void* zvid_cod_crear(int w, int h, int fps, int kbps) {
  auto* c = new Codificador();
  vpx_codec_enc_cfg_t cfg;
  if (vpx_codec_enc_config_default(vpx_codec_vp8_cx(), &cfg, 0)) { delete c; return nullptr; }
  cfg.g_w = w;
  cfg.g_h = h;
  cfg.g_timebase.num = 1;
  cfg.g_timebase.den = 90000;
  cfg.rc_target_bitrate = kbps;
  cfg.rc_end_usage = VPX_CBR;
  cfg.g_lag_in_frames = 0;
  cfg.g_error_resilient = VPX_ERROR_RESILIENT_DEFAULT;
  cfg.kf_mode = VPX_KF_AUTO;
  cfg.kf_max_dist = fps * 10;
  cfg.g_threads = 2;
  cfg.rc_min_quantizer = 2;
  cfg.rc_max_quantizer = 56;
  if (vpx_codec_enc_init(&c->ctx, vpx_codec_vp8_cx(), &cfg, 0)) { delete c; return nullptr; }
  vpx_codec_control(&c->ctx, VP8E_SET_CPUUSED, -6);
  vpx_codec_control(&c->ctx, VP8E_SET_SCREEN_CONTENT_MODE, 1);
  vpx_codec_control(&c->ctx, VP8E_SET_STATIC_THRESHOLD, 100);
  vpx_img_alloc(&c->img, VPX_IMG_FMT_I420, w, h, 16);
  c->w = w;
  c->h = h;
  c->pts = 0;
  return c;
}

// zvid_cod_cuadro encodes one RGBA frame of sw x sh (scaled to the encoder's size) into out. Returns the encoded
// size (0 when the encoder skipped the frame, -1 on error); *clave tells whether it is a key frame.
__declspec(dllexport) int zvid_cod_cuadro(void* h, const uint8_t* rgba, int sw, int sh, int forzar, uint8_t* out,
                                          int cap, int* clave, int ms) {
  auto* c = static_cast<Codificador*>(h);
  if (sw == c->w && sh == c->h) {
    libyuv::ABGRToI420(rgba, sw * 4, c->img.planes[0], c->img.stride[0], c->img.planes[1], c->img.stride[1],
                       c->img.planes[2], c->img.stride[2], sw, sh);
  } else {
    // Converted at its own size, then scaled (libyuv's box filter, as libwebrtc scales screen frames).
    int cw = (sw + 1) / 2, ch = (sh + 1) / 2;
    uint8_t* y = (uint8_t*)malloc(sw * sh + 2 * cw * ch);
    uint8_t* u = y + sw * sh;
    uint8_t* v = u + cw * ch;
    libyuv::ABGRToI420(rgba, sw * 4, y, sw, u, cw, v, cw, sw, sh);
    libyuv::I420Scale(y, sw, u, cw, v, cw, sw, sh, c->img.planes[0], c->img.stride[0], c->img.planes[1],
                      c->img.stride[1], c->img.planes[2], c->img.stride[2], c->w, c->h, libyuv::kFilterBox);
    free(y);
  }
  c->pts += ms * 90;
  if (vpx_codec_encode(&c->ctx, &c->img, c->pts, ms * 90, forzar ? VPX_EFLAG_FORCE_KF : 0, VPX_DL_REALTIME)) return -1;
  vpx_codec_iter_t it = nullptr;
  const vpx_codec_cx_pkt_t* p;
  int n = 0;
  *clave = 0;
  while ((p = vpx_codec_get_cx_data(&c->ctx, &it))) {
    if (p->kind != VPX_CODEC_CX_FRAME_PKT) continue;
    if (n + (int)p->data.frame.sz > cap) return -1;
    memcpy(out + n, p->data.frame.buf, p->data.frame.sz);
    n += (int)p->data.frame.sz;
    if (p->data.frame.flags & VPX_FRAME_IS_KEY) *clave = 1;
  }
  return n;
}

__declspec(dllexport) void zvid_cod_cerrar(void* h) {
  auto* c = static_cast<Codificador*>(h);
  vpx_img_free(&c->img);
  vpx_codec_destroy(&c->ctx);
  delete c;
}

__declspec(dllexport) void* zvid_dec_crear() {
  auto* d = new Decodificador();
  vpx_codec_dec_cfg_t cfg = {2, 0, 0};
  if (vpx_codec_dec_init(&d->ctx, vpx_codec_vp8_dx(), &cfg, 0)) { delete d; return nullptr; }
  return d;
}

// zvid_dec_cuadro decodes one VP8 frame into out as RGBA. Returns 1 with *w, *h when a picture came out, 0 when the
// frame gave none, -1 on a decoding error (a key frame is needed), -2 when out is too small (*w, *h say the size).
__declspec(dllexport) int zvid_dec_cuadro(void* h, const uint8_t* data, int n, uint8_t* out, int cap, int* w, int* hh) {
  auto* d = static_cast<Decodificador*>(h);
  if (vpx_codec_decode(&d->ctx, data, n, nullptr, 0)) return -1;
  vpx_codec_iter_t it = nullptr;
  vpx_image_t* img = vpx_codec_get_frame(&d->ctx, &it);
  if (!img) return 0;
  *w = img->d_w;
  *hh = img->d_h;
  if ((int)(img->d_w * img->d_h * 4) > cap) return -2;
  libyuv::I420ToABGR(img->planes[0], img->stride[0], img->planes[1], img->stride[1], img->planes[2], img->stride[2],
                     out, img->d_w * 4, img->d_w, img->d_h);
  return 1;
}

__declspec(dllexport) void zvid_dec_cerrar(void* h) {
  auto* d = static_cast<Decodificador*>(h);
  vpx_codec_destroy(&d->ctx);
  delete d;
}
}
