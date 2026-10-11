// Command audiocmp compares two mono 16-bit WAV recordings of the same
// run: openMSX's audio of the original (record start -audioonly) and the
// port's (cmd/sndpreview -trace). It aligns them by loudness envelope,
// then compares per 1/60 s window whether each is sounding and its
// strongest frequency. Recordings are ROM-derived: keep them in /tmp.
//
//	go run ./cmd/audiocmp /tmp/ref_audio.wav /tmp/port_audio.wav
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/cmplx"
	"os"
)

const (
	rate = 44100
	hop  = 735  // samples per window (1/60 s)
	nfft = 4096 // FFT size, about 10.8 Hz per bin
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: audiocmp ref.wav port.wav")
		os.Exit(2)
	}
	ref, port := load(os.Args[1]), load(os.Args[2])
	er, ep := envelope(ref), envelope(port)
	off := align(er, ep)
	fmt.Printf("ref %.2fs, port %.2fs; port lags ref by %d windows (%.3fs)\n",
		float64(len(ref))/rate, float64(len(port))/rate, off, float64(off)/60)

	nw := len(er) - max(0, -off)
	if n := len(ep) - max(0, off); n < nw {
		nw = n
	}
	thr := 0.02 * maxOf(er)
	var both, refOnly, portOnly, pitchOK, pitchBad int
	var bad []string
	for w := 0; w < nw; w++ {
		ri, pi := w+max(0, -off), w+max(0, off)
		rOn, pOn := er[ri] > thr, ep[pi] > thr*maxOf(ep)/maxOf(er)
		switch {
		case rOn && pOn:
			both++
			sr, sp := spectrum(ref, ri*hop), spectrum(port, pi*hop)
			fr, fp := peakOf(sr), peakOf(sp)
			// A chord's loudest note depends on each output stage's
			// filtering, so require each side's strongest frequency to
			// be strongly present in the other's spectrum.
			if present(sp, fr) && present(sr, fp) {
				pitchOK++
			} else {
				pitchBad++
				if len(bad) < 15 {
					bad = append(bad, fmt.Sprintf("  %6.2fs: ref %7.1f Hz, port %7.1f Hz", float64(ri)/60, fr, fp))
				}
			}
		case rOn:
			refOnly++
			if len(bad) < 15 {
				bad = append(bad, fmt.Sprintf("  %6.2fs: ref sounding, port silent", float64(ri)/60))
			}
		case pOn:
			portOnly++
			if len(bad) < 15 {
				bad = append(bad, fmt.Sprintf("  %6.2fs: port sounding, ref silent", float64(ri)/60))
			}
		}
	}
	fmt.Printf("%d windows: both sounding %d (peak frequency agrees in %d, differs in %d), only ref %d, only port %d\n",
		nw, both, pitchOK, pitchBad, refOnly, portOnly)
	for _, b := range bad {
		fmt.Println(b)
	}
}

func load(path string) []float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Find the data chunk.
	i := 12
	for i+8 <= len(b) {
		id, n := string(b[i:i+4]), int(binary.LittleEndian.Uint32(b[i+4:]))
		if id == "data" {
			b = b[i+8 : min(len(b), i+8+n)]
			break
		}
		i += 8 + n
	}
	s := make([]float64, len(b)/2)
	for j := range s {
		s[j] = float64(int16(binary.LittleEndian.Uint16(b[2*j:]))) / 32768
	}
	return s
}

// envelope is the RMS of each window, with DC removed.
func envelope(s []float64) []float64 {
	var e []float64
	for i := 0; i+hop <= len(s); i += hop {
		var m, v float64
		for _, x := range s[i : i+hop] {
			m += x
		}
		m /= hop
		for _, x := range s[i : i+hop] {
			v += (x - m) * (x - m)
		}
		e = append(e, math.Sqrt(v/hop))
	}
	return e
}

// align returns the window offset of b relative to a that maximises the
// correlation of their loudness envelopes.
func align(a, b []float64) int {
	norm := func(s []float64) []float64 {
		var m, v float64
		for _, x := range s {
			m += x
		}
		m /= float64(len(s))
		out := make([]float64, len(s))
		for i, x := range s {
			out[i] = x - m
			v += out[i] * out[i]
		}
		v = math.Sqrt(v)
		for i := range out {
			out[i] /= v
		}
		return out
	}
	na, nb := norm(a), norm(b)
	best, bestScore := 0, math.Inf(-1)
	for off := -120; off <= 120; off++ {
		score := 0.0
		for w := range na {
			if j := w + off; j >= 0 && j < len(nb) {
				score += na[w] * nb[j]
			}
		}
		if score > bestScore {
			best, bestScore = off, score
		}
	}
	return best
}

// spectrum returns the magnitude spectrum of the nfft samples from i.
func spectrum(s []float64, i int) []float64 {
	x := make([]complex128, nfft)
	for k := 0; k < nfft && i+k < len(s); k++ {
		w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(k)/nfft) // Hann
		x[k] = complex(s[i+k]*w, 0)
	}
	fft(x)
	m := make([]float64, nfft/2)
	for k := 3; k < nfft/2; k++ { // skip DC
		m[k] = cmplx.Abs(x[k])
	}
	return m
}

func peakOf(m []float64) float64 {
	best := 0
	for k, v := range m {
		if v > m[best] {
			best = k
		}
	}
	return float64(best) * rate / nfft
}

// present reports whether frequency f (within 3%) carries at least a
// quarter of the spectrum's peak magnitude.
func present(m []float64, f float64) bool {
	peak := 0.0
	for _, v := range m {
		peak = math.Max(peak, v)
	}
	lo, hi := int(f*0.97*nfft/rate), int(f*1.03*nfft/rate)+1
	for k := max(lo, 0); k <= hi && k < len(m); k++ {
		if m[k] >= 0.25*peak {
			return true
		}
	}
	return false
}

func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for l := 2; l <= n; l <<= 1 {
		w := cmplx.Exp(complex(0, -2*math.Pi/float64(l)))
		for i := 0; i < n; i += l {
			wk := complex(1, 0)
			for k := 0; k < l/2; k++ {
				u, v := x[i+k], x[i+k+l/2]*wk
				x[i+k], x[i+k+l/2] = u+v, u-v
				wk *= w
			}
		}
	}
}

func maxOf(s []float64) float64 {
	m := 0.0
	for _, v := range s {
		m = math.Max(m, v)
	}
	return m
}
