// zapm: a C bridge over WebRTC's audio processing module (webrtc-audio-processing, the APM that libwebrtc runs on
// every call), so Go can call it with syscall and no cgo. Mono, 48 kHz, 10 ms frames of int16.
//
// The configuration is WebRTC's voice engine's: high-pass filter, echo canceller (AEC3), noise suppression at its
// high level, and AGC2's adaptive digital gain (there is no microphone volume to drive, so no analog AGC1).
#include <cstdint>
#include <modules/audio_processing/include/audio_processing.h>

using webrtc::AudioProcessing;
using webrtc::StreamConfig;

extern "C" {

__declspec(dllexport) void* zapm_crear(int retardo_ms) {
  AudioProcessing* apm = webrtc::AudioProcessingBuilder().Create();
  if (!apm) return nullptr;
  AudioProcessing::Config c;
  c.high_pass_filter.enabled = true;
  c.echo_canceller.enabled = true;
  c.echo_canceller.mobile_mode = false;
  c.noise_suppression.enabled = true;
  c.noise_suppression.level = AudioProcessing::Config::NoiseSuppression::kHigh;
  c.gain_controller1.enabled = false;
  c.gain_controller2.enabled = true;
  c.gain_controller2.adaptive_digital.enabled = true;
  apm->ApplyConfig(c);
  apm->Initialize();
  apm->set_stream_delay_ms(retardo_ms);
  return apm;
}

// The microphone: 480 samples in place. Returns 0 when fine.
__declspec(dllexport) int zapm_microfono(void* h, int16_t* x, int retardo_ms) {
  auto* apm = static_cast<AudioProcessing*>(h);
  StreamConfig sc(48000, 1);
  apm->set_stream_delay_ms(retardo_ms);
  return apm->ProcessStream(x, sc, sc, x);
}

// What the speaker plays (the echo canceller's reference): 480 samples, left untouched.
__declspec(dllexport) int zapm_parlante(void* h, int16_t* x) {
  auto* apm = static_cast<AudioProcessing*>(h);
  StreamConfig sc(48000, 1);
  int16_t copia[480];
  return apm->ProcessReverseStream(x, sc, sc, copia);
}

__declspec(dllexport) void zapm_destruir(void* h) { delete static_cast<AudioProcessing*>(h); }
}
