package webrtcns

import "math"

/* ---------- three_band_filter_bank.cc: 48 kHz into three bands of 16 kHz, and back ---------- */

const (
	kSparsity          = 4
	kStrideLog2        = 2
	kStride            = 1 << kStrideLog2
	kNumZeroFilters    = 2
	kFilterSize        = 4
	kMemorySize        = kFilterSize*kStride - 1
	kNumBands          = 3
	KFullBandSize      = 480
	kSplitBandSize     = KFullBandSize / kNumBands
	kNumNonZeroFilters = kSparsity*kNumBands - kNumZeroFilters
	kSubSampling       = kNumBands
	kDctSize           = kNumBands
	kZeroFilterIndex1  = 3
	kZeroFilterIndex2  = 9
)

var kFilterCoeffs = [kNumNonZeroFilters][kFilterSize]float32{
	{-0.00047749, -0.00496888, +0.16547118, +0.00425496},
	{-0.00173287, -0.01585778, +0.14989004, +0.00994113},
	{-0.00304815, -0.02536082, +0.12154542, +0.01157993},
	{-0.00346946, -0.02587886, +0.04760441, +0.00607594},
	{-0.00154717, -0.01136076, +0.01387458, +0.00186353},
	{+0.00186353, +0.01387458, -0.01136076, -0.00154717},
	{+0.00607594, +0.04760441, -0.02587886, -0.00346946},
	{+0.00983212, +0.08543175, -0.02982767, -0.00383509},
	{+0.00994113, +0.14989004, -0.01585778, -0.00173287},
	{+0.00425496, +0.16547118, -0.00496888, -0.00047749},
}

var kSqrt3 = float32(math.Sqrt(3))

var kDctModulation = [kNumNonZeroFilters][kDctSize]float32{
	{2, 2, 2},
	{kSqrt3, 0, -kSqrt3},
	{1, -2, 1},
	{-1, 2, -1},
	{-kSqrt3, 0, kSqrt3},
	{-2, -2, -2},
	{-kSqrt3, 0, kSqrt3},
	{-1, 2, -1},
	{1, -2, 1},
	{kSqrt3, 0, -kSqrt3},
}

func filterCore(filter *[kFilterSize]float32, in *[kSplitBandSize]float32, inShift int, out *[kSplitBandSize]float32, state *[kMemorySize]float32) {
	*out = [kSplitBandSize]float32{}
	for k := 0; k < inShift; k++ {
		for i, j := 0, kMemorySize+k-inShift; i < kFilterSize; i, j = i+1, j-kStride {
			out[k] += state[j] * filter[i]
		}
	}
	for k, shift := inShift, 0; k < kFilterSize*kStride; k, shift = k+1, shift+1 {
		loopLimit := min(kFilterSize, 1+(shift>>kStrideLog2))
		for i, j := 0, shift; i < loopLimit; i, j = i+1, j-kStride {
			out[k] += in[j] * filter[i]
		}
		for i, j := loopLimit, kMemorySize+shift-loopLimit*kStride; i < kFilterSize; i, j = i+1, j-kStride {
			out[k] += state[j] * filter[i]
		}
	}
	for k, shift := kFilterSize*kStride, kFilterSize*kStride-inShift; k < kSplitBandSize; k, shift = k+1, shift+1 {
		for i, j := 0, shift; i < kFilterSize; i, j = i+1, j-kStride {
			out[k] += in[j] * filter[i]
		}
	}
	copy(state[:], in[kSplitBandSize-kMemorySize:])
}

func filterIndex(index int) int {
	if index < kZeroFilterIndex1 {
		return index
	}
	if index < kZeroFilterIndex2 {
		return index - 1
	}
	return index - 2
}

type threeBandFilterBank struct {
	stateAnalysis, stateSynthesis [kNumNonZeroFilters][kMemorySize]float32
}

func (f *threeBandFilterBank) analysis(in *[KFullBandSize]float32, out *[kNumBands][kSplitBandSize]float32) {
	*out = [kNumBands][kSplitBandSize]float32{}
	for di := 0; di < kSubSampling; di++ {
		var inSub [kSplitBandSize]float32
		for k := 0; k < kSplitBandSize; k++ {
			inSub[k] = in[(kSubSampling-1)-di+kSubSampling*k]
		}
		for inShift := 0; inShift < kStride; inShift++ {
			index := di + inShift*kSubSampling
			if index == kZeroFilterIndex1 || index == kZeroFilterIndex2 {
				continue
			}
			fi := filterIndex(index)
			var outSub [kSplitBandSize]float32
			filterCore(&kFilterCoeffs[fi], &inSub, inShift, &outSub, &f.stateAnalysis[fi])
			for band := 0; band < kNumBands; band++ {
				for n := 0; n < kSplitBandSize; n++ {
					out[band][n] += kDctModulation[fi][band] * outSub[n]
				}
			}
		}
	}
}

func (f *threeBandFilterBank) synthesis(in *[kNumBands][kSplitBandSize]float32, out *[KFullBandSize]float32) {
	*out = [KFullBandSize]float32{}
	for ui := 0; ui < kSubSampling; ui++ {
		for inShift := 0; inShift < kStride; inShift++ {
			index := ui + inShift*kSubSampling
			if index == kZeroFilterIndex1 || index == kZeroFilterIndex2 {
				continue
			}
			fi := filterIndex(index)
			var inSub [kSplitBandSize]float32
			for band := 0; band < kNumBands; band++ {
				for n := 0; n < kSplitBandSize; n++ {
					inSub[n] += kDctModulation[fi][band] * in[band][n]
				}
			}
			var outSub [kSplitBandSize]float32
			filterCore(&kFilterCoeffs[fi], &inSub, inShift, &outSub, &f.stateSynthesis[fi])
			const kUpsamplingScaling = float32(kSubSampling)
			for k := 0; k < kSplitBandSize; k++ {
				out[ui+kSubSampling*k] += kUpsamplingScaling * outSub[k]
			}
		}
	}
}
