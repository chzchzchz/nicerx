package wav

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrBadFormat = errors.New("bad format")
)

type riffHeader struct {
	ChunkId   [4]byte
	ChunkSize uint32
	Format    [4]byte
}

// WaveHeader is wave header struct
type fmtHeader struct {
	ChunkId       [4]byte /* "fmt " */
	ChunkSize     uint32
	AudioFormat   uint16 /* 1 */
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
}

type metaHeader struct {
	ChunkId   [4]byte /* "meta" */
	ChunkSize uint32
	Key       []uint8
	Val       []uint8
}

type dataHeader struct {
	ChunkId   [4]byte /* "data" */
	ChunkSize uint32
}

type Reader struct {
	io.Reader
	rh riffHeader
	fh fmtHeader
	dh dataHeader
}

func NewReader(r io.Reader) (*Reader, error) {
	rr := &Reader{Reader: r}
	if err := binary.Read(r, binary.LittleEndian, &rr.rh); err != nil {
		return nil, err
	}
	if string(rr.rh.ChunkId[:]) != "RIFF" || string(rr.rh.Format[:]) != "WAVE" {
		return nil, ErrBadFormat
	}
	if err := binary.Read(r, binary.LittleEndian, &rr.fh); err != nil {
		return nil, err
	}
	if string(rr.fh.ChunkId[:]) != "fmt " || rr.fh.AudioFormat != 1 {
		return nil, ErrBadFormat
	}
	if err := binary.Read(r, binary.LittleEndian, &rr.dh); err != nil {
		return nil, err
	}
	if string(rr.dh.ChunkId[:]) != "data" {
		return nil, ErrBadFormat
	}
	return rr, nil
}

func (r *Reader) Channels() int {
	return int(r.fh.NumChannels)
}

func (r *Reader) SampleRate() int {
	return int(r.fh.SampleRate)
}

func (r *Reader) BitDepth() int {
	return int(r.fh.BitsPerSample / r.fh.NumChannels)
}

type Writer struct {
	w io.Writer

	SampleRate    uint32
	BitsPerSample uint16
	NumChannels   uint16

	dataLen uint32
	// TODO: metadata
}

func NewWriter(w io.Writer, rate, depth, channels int) (*Writer, error) {
	if rate == 0 || depth == 0 || channels == 0 {
		return nil, ErrBadFormat
	}
	ww := &Writer{
		w:             w,
		SampleRate:    uint32(rate),
		BitsPerSample: uint16(depth),
		NumChannels:   uint16(channels),
	}
	if err := ww.writeHeader(0); err != nil {
		return nil, err
	}
	return ww, nil
}

func (w *Writer) Write(p []byte) (int, error) {
	w.dataLen += uint32(len(p))
	return w.w.Write(p)
}

func (w *Writer) Close() error {
	if ws, ok := w.w.(io.WriteSeeker); ok {
		if _, err := ws.Seek(0, 0); err != nil {
			return err
		}
		if err := w.writeHeader(w.dataLen); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeHeader(dataLen uint32) error {
	if dataLen == 0 {
		dataLen = 1 << 31
	}
	rh := &riffHeader{
		ChunkId:   [4]byte{'R', 'I', 'F', 'F'},
		ChunkSize: dataLen + 32,
		Format:    [4]byte{'W', 'A', 'V', 'E'},
	}
	if err := binary.Write(w.w, binary.LittleEndian, rh); err != nil {
		return err
	}

	fh := &fmtHeader{
		ChunkId:       [4]byte{'f', 'm', 't', ' '},
		ChunkSize:     16,
		AudioFormat:   1,
		NumChannels:   w.NumChannels,
		SampleRate:    w.SampleRate,
		ByteRate:      w.SampleRate * uint32(w.NumChannels) * uint32(w.BitsPerSample) / 8,
		BlockAlign:    uint16((uint32(w.NumChannels) * uint32(w.BitsPerSample)) / 8),
		BitsPerSample: w.BitsPerSample,
	}
	if err := binary.Write(w.w, binary.LittleEndian, fh); err != nil {
		return err
	}

	dh := &dataHeader{
		ChunkId:   [4]byte{'d', 'a', 't', 'a'},
		ChunkSize: dataLen,
	}
	return binary.Write(w.w, binary.LittleEndian, dh)
}
