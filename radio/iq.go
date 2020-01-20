package radio

import (
	"context"
	"io"
)

type IQReader struct {
	r   io.Reader
	err error
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
	return &IQReader{r: r}
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
	ch := make(chan []complex64, 1)
	go func() {
		defer close(ch)
		iq8buf := make([]byte, batch*2)
		i := 0
		for {
			if limit > 0 && i >= limit {
				return
			}
			i++
			sumBytes := 0
			for sumBytes != len(iq8buf) {
				readBytes := 0
				if readBytes, iq.err = iq.r.Read(iq8buf[sumBytes:]); iq.err != nil {
					return
				}
				sumBytes += readBytes
			}

			samps := make([]complex64, batch)
			for i := 0; i < len(samps); i++ {
				samps[i] = complex(
					(float32(iq8buf[2*i])-127)/128.0,
					(float32(iq8buf[2*i+1])-127)/128.0)
			}
			select {
			case ch <- samps:
			case <-ctx.Done():
			}
		}
	}()
	return ch
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
