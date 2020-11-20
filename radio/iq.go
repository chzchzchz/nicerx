package radio

import (
	"context"
	"io"
	"log"
	"sync"
	"time"
)

type IQReader struct {
	r     io.Reader
	err   error
	mu    sync.Mutex
	batch int
	chans map[*iqChannel]struct{}
}

type iqChannel struct {
	batch int
	limit int
	sent  int
	c     chan []complex64
	ctx   context.Context
}

type MixerIQReader struct {
	HzBand
	*IQReader
}

// NewIQReader takes a reader that uses u8 I/Q samples.
func NewIQReader(r io.Reader) *IQReader {
	if r == nil {
		panic("nil reader")
	}
	return &IQReader{r: r, chans: make(map[*iqChannel]struct{})}
}

func (iqr *IQReader) ToMixer(hzb HzBand) *MixerIQReader {
	return &MixerIQReader{HzBand: hzb, IQReader: iqr}
}

func NewMixerIQReader(r io.Reader, hzb HzBand) *MixerIQReader {
	return &MixerIQReader{
		HzBand:   hzb,
		IQReader: NewIQReader(r),
	}
}

func (iq *IQReader) Batch64(batch, limit int) <-chan []complex64 {
	return iq.BatchStream64(context.Background(), batch, limit)
}

func (iq *IQReader) BatchStream64(ctx context.Context, batch, limit int) <-chan []complex64 {
	iqc := &iqChannel{limit: limit, c: make(chan []complex64, 4), ctx: ctx}
	iq.mu.Lock()
	defer iq.mu.Unlock()
	iq.chans[iqc] = struct{}{}
	if len(iq.chans) == 1 {
		iq.batch = batch
		go iq.dispatch()
	} else if iq.batch != batch {
		panic("bad batch")
	}
	return iqc.c
}

func (iq *IQReader) dispatch() error {
	iq8buf := make([]byte, iq.batch*2)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		sumBytes := 0
		for sumBytes != len(iq8buf) {
			readBytes := 0
			if readBytes, iq.err = iq.r.Read(iq8buf[sumBytes:]); iq.err != nil {
				iq.mu.Lock()
				defer iq.mu.Unlock()
				for iqc := range iq.chans {
					close(iqc.c)
				}
				iq.chans = make(map[*iqChannel]struct{})
				return iq.err
			}
			sumBytes += readBytes
		}

		samps := make([]complex64, iq.batch)
		for i := 0; i < len(samps); i++ {
			samps[i] = complex(
				(float32(iq8buf[2*i])-127)/128.0,
				(float32(iq8buf[2*i+1])-127)/128.0)
		}

		ticker.Reset(time.Second)
		for len(ticker.C) > 0 {
			<-ticker.C
		}

		iq.mu.Lock()

		tc := ticker.C
		if len(iq.chans) == 1 {
			tc = nil
		}

		// Broadcast.
		for iqc := range iq.chans {
			ok := false
			select {
			case iqc.c <- samps:
				iqc.sent++
				ok = iqc.limit == 0 || iqc.sent < iqc.limit
			case <-iqc.ctx.Done():
				log.Println("canceled channel")
			case <-tc:
				log.Println("channel too slow")
			}
			if !ok {
				delete(iq.chans, iqc)
				close(iqc.c)
			}
		}
		if len(iq.chans) == 0 {
			// TODO: close reader entirely, reopen when ready
			iq.mu.Unlock()
			return nil
		}
		iq.mu.Unlock()
	}
}

type IQWriter struct{ w io.Writer }

func NewIQWriter(w io.Writer) *IQWriter { return &IQWriter{w} }

func (iq *IQWriter) Write64(out []complex64) error {
	buf := make([]byte, 2*len(out))
	for i := range out {
		buf[2*i] = byte((real(out[i]) * 128.0) + 127.0)
		buf[2*i+1] = byte((imag(out[i]) * 128.0) + 127.0)
	}
	_, err := iq.w.Write(buf)
	return err
}
