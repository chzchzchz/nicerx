package store

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chzchzchz/nicerx/radio"
)

type SignalStore struct {
	baseDir string
}

func NewSignalStore(dir string) (*SignalStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &SignalStore{dir}, nil
}

func (ss *SignalStore) OpenFile(fb radio.FreqBand) (*os.File, error) {
	fdir := filepath.Join(ss.baseDir, fmt.Sprintf("%.3f", fb.Center))
	if err := os.MkdirAll(fdir, 0755); err != nil {
		return nil, err
	}
	fn := filepath.Join(
		fdir,
		fmt.Sprintf("%d.%d.iq", time.Now().UnixNano(), int(fb.Width*1e6)))
	return os.OpenFile(fn, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
}

func (ss *SignalStore) HasBand(fb radio.FreqBand) bool {
	_, err := os.Stat(
		filepath.Join(ss.baseDir,
			fmt.Sprintf("%.3f", fb.Center)))
	return err == nil
}

type SpectrogramFile struct {
	Band radio.FreqBand
	Date time.Time
	Path string
}

func (ss *SignalStore) Spectrograms(fb radio.FreqBand) (ret []SpectrogramFile) {
	// TODO: cache
	files, err := ioutil.ReadDir(ss.baseDir)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		mhz, err := strconv.ParseFloat(file.Name(), 64)
		if err != nil {
			continue
		}
		if !fb.Overlaps(radio.FreqBand{Center: mhz, Width: 100.0 / 1e6}) {
			continue
		}
		fdir := filepath.Join(ss.baseDir, file.Name())
		ffiles, err := ioutil.ReadDir(fdir)
		if err != nil {
			continue
		}
		for _, ffile := range ffiles {
			if !strings.HasSuffix(ffile.Name(), ".jpg") {
				continue
			}
			spl := strings.Split(ffile.Name(), ".")
			ntime, bwhz := spl[0], spl[1]
			ntime64, err := strconv.ParseInt(ntime, 10, 64)
			if err != nil {
				continue
			}
			bwhz64, err := strconv.ParseUint(bwhz, 10, 64)
			if err != nil {
				continue
			}
			sf := SpectrogramFile{
				Band: radio.FreqBand{Center: mhz, Width: float64(bwhz64) / 1e6},
				Date: time.Unix(0, ntime64),
				Path: filepath.Join(fdir, ffile.Name()),
			}
			ret = append(ret, sf)
		}
	}
	return ret
}
