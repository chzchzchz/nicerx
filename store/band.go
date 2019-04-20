package store

import (
	"encoding/csv"
	"encoding/gob"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chzchzchz/nicerx/radio"
)

type BandStore struct {
	bands map[float64]BandRecord
	rwmu  sync.RWMutex
}

type BandRecord struct {
	radio.FreqBand
	Date       time.Time
	Name       string
	Modulation string
}

func NewBandStore() *BandStore {
	return &BandStore{bands: make(map[float64]BandRecord)}
}

func (b *BandStore) ImportCSV(r io.Reader) error {
	csvr := csv.NewReader(r)
	csvr.Comma, csvr.Comment, csvr.FieldsPerRecord = ';', '#', -1
	records, err := csvr.ReadAll()
	if err != nil {
		return err
	}
	now := time.Now()
	for _, v := range records {
		if len(v) != 5 {
			continue
		}
		for i := range v {
			v[i] = strings.TrimSpace(v[i])
		}
		centerhzStr, name, mod, bwhzStr := v[0], v[1], v[2], v[3]
		centerhz, _ := strconv.ParseInt(centerhzStr, 10, 64)
		bwhz, _ := strconv.ParseInt(bwhzStr, 10, 64)
		fb := radio.FreqBand{Center: float64(centerhz) / 1e6, Width: float64(bwhz) / 1e6}
		rec := BandRecord{
			FreqBand:   fb,
			Name:       name,
			Modulation: mod,
			Date:       now,
		}
		if _, ok := b.bands[rec.Center]; !ok {
			b.bands[rec.Center] = rec
		}
	}
	return nil
}

func (b *BandStore) Load(fpath string) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	dec := gob.NewDecoder(f)
	err = dec.Decode(&b.bands)
	return err
}

func (b *BandStore) Save(fpath string) error {
	f, err := os.OpenFile(fpath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(f)
	return enc.Encode(&b.bands)
}

func (b BandStore) Bands() []BandRecord {
	b.rwmu.RLock()
	ret := make([]BandRecord, 0, len(b.bands))
	for _, v := range b.bands {
		ret = append(ret, v)
	}
	b.rwmu.RUnlock()
	return ret
}

func (b *BandStore) Range(fb radio.FreqBand) (ret []radio.FreqBand) {
	b.rwmu.RLock()
	defer b.rwmu.RUnlock()
	for _, v := range b.bands {
		if fb.Overlaps(v.FreqBand) {
			ret = append(ret, v.FreqBand)
		}
	}
	return ret
}

func (b *BandStore) Add(fbs []radio.FreqBand) {
	if len(fbs) == 0 {
		return
	}
	br := radio.BandRange(fbs)
	allOverlaps := b.Range(br)
	var overlaps []radio.FreqBand
	for _, fb := range allOverlaps {
		if rec, ok := b.bands[fb.Center]; !ok || len(rec.Name) == 0 {
			overlaps = append(overlaps, fb)
		}
	}
	b.rwmu.Lock()
	defer b.rwmu.Unlock()
	for _, fb := range overlaps {
		delete(b.bands, fb.Center)
	}
	overlaps = append(overlaps, fbs...)
	fbs = radio.BandMerge(overlaps)
	for _, v := range fbs {
		b.bands[v.Center] = BandRecord{FreqBand: v, Date: time.Now()}
	}
}
