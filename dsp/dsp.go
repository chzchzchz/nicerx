package dsp

/*
#cgo LDFLAGS: -lliquid
#include <liquid/liquid.h>
static void firfilt_crcf_block(
	firfilt_crcf q,
	complex float *in, complex float *out,
	unsigned n, unsigned dec)
{
	// remove DC bias by subtracting mean
	complex double mean = 0.0;
	// for (unsigned i = 0; i < n; i++)
	// 	mean += in[i];
	// mean /= (double)n;

	unsigned j = 0, k = 0;
	for (unsigned i = 0; i < n; i++) {
		firfilt_crcf_push(q, in[i] - mean);
		k++;
		firfilt_crcf_execute(q, &out[j]);
		if (k == dec) {
			k = 0;
			j++;
		}
	}
}
*/
import "C"

import (
	"math"
	"unsafe"
)

func MixDown(mixHz float64, sampHz int, sigc <-chan []complex64) <-chan []complex64 {
	q := C.nco_crcf_create(C.LIQUID_NCO)
	C.nco_crcf_set_phase(q, C.float(0))
	outc := make(chan []complex64, 1)
	go func() {
		defer func() {
			C.nco_crcf_destroy(q)
			close(outc)
		}()
		radiansPerSample := mixHz * (2.0 * math.Pi / float64(sampHz))
		C.nco_crcf_set_frequency(q, C.float(radiansPerSample))
		for samp := range sigc {
			outsamp := make([]complex64, len(samp))
			C.nco_crcf_mix_block_down(
				q,
				(*C.complexfloat)(unsafe.Pointer(&samp[0])),
				(*C.complexfloat)(unsafe.Pointer(&outsamp[0])),
				C.uint(len(samp)))
			outc <- outsamp
		}
	}()
	return outc
}

func Lowpass(cutoffHz float64,
	sampHz int,
	decRate int,
	sigc <-chan []complex64) <-chan []complex64 {
	As := 70.0
	cutoffFreq := cutoffHz / float64(sampHz)

	if decRate <= 0 {
		panic("bad decimation")
	}

	q := C.firfilt_crcf_create_kaiser(
		64,
		C.float(cutoffFreq),
		C.float(As),
		C.float(0.0))
	C.firfilt_crcf_set_scale(q, C.float(2.0*cutoffFreq))
	outc := make(chan []complex64, 1)
	go func() {
		defer func() {
			C.firfilt_crcf_destroy(q)
			close(outc)
		}()
		for samp := range sigc {
			outsamp := make([]complex64, len(samp)/decRate)
			C.firfilt_crcf_block(q,
				(*C.complexfloat)(unsafe.Pointer(&samp[0])),
				(*C.complexfloat)(unsafe.Pointer(&outsamp[0])),
				C.uint(len(samp)),
				C.uint(decRate))
			outc <- outsamp
		}
	}()
	return outc
}

func ResampleComplex64(r float32, sigc <-chan []complex64) <-chan []complex64 {
	outc := make(chan []complex64, 1)
	q := C.resamp_crcf_create_default(C.float(r))
	go func() {
		defer func() {
			close(outc)
			C.resamp_crcf_destroy(q)
		}()
		for samps := range sigc {
			outsamp := make([]complex64, int(math.Ceil(float64(r)*float64(len(samps)))))
			var outlen uint
			C.resamp_crcf_execute_block(q,
				(*C.complexfloat)(unsafe.Pointer(&samps[0])),
				C.uint(len(samps)),
				(*C.complexfloat)(unsafe.Pointer(&outsamp[0])),
				(*C.uint)(unsafe.Pointer(&outlen)))
			outsamp = outsamp[:outlen]
			outc <- outsamp
		}
	}()
	return outc
}

func Resample(r float32, sigc <-chan []float32) <-chan []float32 {
	outc := make(chan []float32, 1)
	q := C.resamp_rrrf_create_default(C.float(r))
	go func() {
		defer func() {
			close(outc)
			C.resamp_rrrf_destroy(q)
		}()
		for samps := range sigc {
			outsamp := make([]float32, int(math.Ceil(float64(r)*float64(len(samps)))))
			var outlen uint
			C.resamp_rrrf_execute_block(q,
				(*C.float)(unsafe.Pointer(&samps[0])),
				C.uint(len(samps)),
				(*C.float)(unsafe.Pointer(&outsamp[0])),
				(*C.uint)(unsafe.Pointer(&outlen)))
			outsamp = outsamp[:outlen]
			outc <- outsamp

		}
	}()
	return outc
}

func DemodFM(h float32, sigc <-chan []complex64) <-chan []float32 {
	// h = modulation index = (delta f)/(delta modulation)
	outc := make(chan []float32, 1)
	q := C.freqdem_create(C.float(h))
	go func() {
		defer func() {
			close(outc)
			C.freqdem_destroy(q)
		}()
		for samps := range sigc {
			outsamp := make([]float32, len(samps))
			C.freqdem_demodulate_block(
				q,
				(*C.complexfloat)(unsafe.Pointer(&samps[0])),
				C.uint(len(samps)),
				(*C.float)(unsafe.Pointer(&outsamp[0])))
			outc <- outsamp
		}
	}()
	return outc
}
