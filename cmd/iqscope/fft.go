package main

import (
	"context"
	"fmt"
	"image/jpeg"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/veandco/go-sdl2/sdl"

	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
)

type fftWindow struct {
	win *sdl.Window
	r   *sdl.Renderer
	ft  *fftTexture
	w   int
	h   int

	sampc   <-chan []complex64
	iqr     *radio.MixerIQReader
	iqrPath string

	pause      bool
	lines      []int32
	muteCenter bool

	wcancel context.CancelFunc
	wdonec  <-chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewFFTWindow(iqr *radio.MixerIQReader, w, h int) (fw *fftWindow, err error) {
	sampc := iqr.Batch64(w, 0)
	log.Println("waiting for first row")
	if row0 := <-sampc; row0 == nil {
		return nil, fmt.Errorf("failed reading first row of samples")
	}
	log.Println("got first samples")

	winFlags := uint32(sdl.WINDOW_SHOWN)
	if resizable {
		winFlags |= sdl.WINDOW_RESIZABLE | sdl.WINDOW_OPENGL | sdl.WINDOW_UTILITY
	}
	if popup {
		winFlags |= sdl.WINDOW_UTILITY
	}
	mhz := iqr.ToMHz()
	winName := fmt.Sprintf("iqscope FFT @ [%0.5g,%0.5g]MHz", mhz.BeginMHz(), mhz.EndMHz())
	win, e := sdl.CreateWindow(
		winName,
		sdl.WINDOWPOS_UNDEFINED,
		sdl.WINDOWPOS_UNDEFINED,
		int32(w),
		int32(h),
		winFlags)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			win.Destroy()
		}
	}()

	// Disable letterboxing.
	sdl.SetHint(sdl.HINT_RENDER_LOGICAL_SIZE_MODE, "1")

	r, e := sdl.CreateRenderer(win, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_TARGETTEXTURE)
	if e != nil {
		return nil, e
	}
	defer func() {
		if err != nil {
			r.Destroy()
		}
	}()

	info, err := r.GetInfo()
	if err != nil {
		return nil, err
	}
	if (info.Flags & sdl.RENDERER_ACCELERATED) == 0 {
		log.Println("no hw acceleration")
	}
	if err := r.SetLogicalSize(int32(w), int32(h)); err != nil {
		return nil, err
	}
	if err := r.SetIntegerScale(false); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &fftWindow{
		win:    win,
		r:      r,
		w:      w,
		h:      h,
		pause:  false,
		ft:     newFFTTexture(r, w, h),
		sampc:  sampc,
		iqr:    iqr,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (fw *fftWindow) Close() {
	fw.cancel()
	if fw.wcancel != nil {
		fw.wcancel()
	}
	fw.ft.Destroy()
	fw.r.Destroy()
	fw.win.Destroy()
	fw.wg.Wait()
}

func (fw *fftWindow) redraw() {
	// Draw waterfall.
	fw.ft.blit()

	// Draw selection.
	fw.r.SetDrawColor(0xff, 0xd3, 0, 0xff)
	for _, x := range fw.lines {
		fw.r.DrawLine(x, 0, x, int32(fw.h))
	}

	if err := fw.r.Flush(); err != nil {
		panic(err)
	}
	fw.r.Present()
}

func (fw *fftWindow) Run() {
	// Cope with too much data by dropping rows to render.
	// This will work even if the sample rate is totally wrong.
	fps := float64(30)
	if maxfps := float64(fw.iqr.Width) / float64(fw.w); maxfps < fps {
		fps = maxfps
	}
	framec := make(chan []complex64, int(2*fps))
	fw.wg.Add(1)
	go func() {
		defer func() {
			close(framec)
			fw.wg.Done()
		}()
		for row := range fw.sampc {
			select {
			case framec <- row:
			default:
			}
		}
	}()

	// FFT uses frame rate limited channel to avoid processing dropped rows.
	fftc := nicerx.SpectrogramChan(framec, fw.w)

	fpsDur := time.Duration(float64(time.Second) / fps)
	ticker := time.NewTicker(fpsDur)
	defer ticker.Stop()

	for fw.processEvents() {
		<-ticker.C
		if fw.pause {
			continue
		}
		// TODO: add more than one row per tick
		if row := <-fftc; row != nil {
			fw.ft.add(row)
		} else {
			log.Println("stream terminated")
			return
		}
		fw.redraw()

		select {
		case <-ticker.C:
		default:
		}
		ticker.Reset(fpsDur)
	}
}

func (fw *fftWindow) x2hz(x int32) int64 {
	ww := float64(fw.w)
	centerOffset := (float64(x) - (ww / 2.0)) / ww
	offHz := int64(centerOffset * float64(fw.iqr.Width))
	return int64(fw.iqr.Center) + offHz
}

func (fw *fftWindow) handleEvent(event sdl.Event) bool {
	switch ev := event.(type) {
	case *sdl.QuitEvent:
		return false
	case *sdl.MouseButtonEvent:
		if ev.Type != sdl.MOUSEBUTTONDOWN {
			break
		}
		if ev.Button == sdl.BUTTON_LEFT {
			if len(fw.lines) > 1 {
				fw.lines = nil
			}
			fw.lines = append(fw.lines, ev.X)
		} else if ev.Button == sdl.BUTTON_RIGHT {
			fw.lines = nil
		}
		if fw.pause {
			fw.redraw()
		}
	case *sdl.MouseMotionEvent:
		cursorMHz := float64(fw.x2hz(ev.X)) / 1.0e6
		mhz := fw.iqr.ToMHz()
		if !fw.muteCenter {
			log.Printf("center: %0.7gMHz of [%g,%g]", cursorMHz, mhz.BeginMHz(), mhz.EndMHz())
		}
	case *sdl.WindowEvent:
		if fw.pause {
			fw.redraw()
		}
	case *sdl.KeyboardEvent:
		if ev.Type == sdl.KEYDOWN {
			switch ev.Keysym.Sym {
			case sdl.K_SPACE:
				// TODO: disconnect stream if paused for too long.
				fw.pause = !fw.pause
			case sdl.K_l:
				fw.launchWindow()
			case sdl.K_m:
				fw.muteCenter = !fw.muteCenter
			case sdl.K_r:
				// Reset window size.
				fw.win.SetSize(int32(fw.w), int32(fw.h))
			}
		} else if ev.Type == sdl.KEYUP {
			switch ev.Keysym.Sym {
			case sdl.K_ESCAPE:
				return false
			case sdl.K_w:
				fw.toggleWrite(false)
			case sdl.K_f:
				fw.toggleWrite(true)
			case sdl.K_s:
				go fw.screenshot()
			}
		}

	}
	return true
}

func (fw *fftWindow) screenshot() error {
	img := fw.ft.blitImage()
	t := time.Now()
	outfn := fmt.Sprintf("%02d%02d%02dT%02d%02d%02d-%d[%d].jpg",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(),
		fw.iqr.Center, fw.iqr.Width)
	outf, err := os.OpenFile(outfn, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		log.Println(err)
		return err
	}
	defer outf.Close()
	if err := jpeg.Encode(outf, img, nil); err != nil {
		log.Println(err)
		return err
	}
	log.Println("saved screenshot", outfn)
	return nil
}

func (fw *fftWindow) toggleWrite(useFifo bool) {
	path := fmt.Sprintf("%d[%d].%s", fw.iqr.Center, fw.iqr.Width, outputExt)
	if fw.wdonec != nil {
		fw.wcancel()
		<-fw.wdonec
		fw.wdonec = nil
		return
	}

	if useFifo {
		if err := syscall.Mkfifo(path, 0644); err != nil {
			if !os.IsExist(err) {
				panic(err)
			}
			fi, err := os.Stat(path)
			if err != nil {
				panic(err)
			}
			if fi.Mode()&os.ModeNamedPipe != os.ModeNamedPipe {
				panic("expected named pipe on " + path)
			}
		}
	}

	wdonec := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	fw.wdonec, fw.wcancel = wdonec, cancel
	fw.wg.Add(1)
	go func() {
		defer func() {
			fw.wcancel()
			close(wdonec)
			log.Println("finished recording", path)
			fw.wg.Done()
		}()
		log.Println("start recording", path)
		iqw, closer, err := nicerx.OpenIQW(path, fw.iqr.HzBand)
		if err != nil {
			log.Println("failed to open", path, err)
			return
		}

		defer closer()
		fw.wg.Add(1)
		go func() {
			// FIFO case needs file closed to terminate Write64.
			defer fw.wg.Done()
			<-ctx.Done()
			closer()
		}()

		// TODO: this needs a way to stop the write if sampc closes.

		sampc := fw.iqr.BatchStream64(ctx, fw.w, 0)
		for samps := range sampc {
			if err := iqw.Write64(samps); err != nil {
				log.Println("failed to write", path, err)
				return
			}
		}
	}()
}

func (fw *fftWindow) launchWindow() {
	if len(fw.lines) != 2 {
		return
	}
	a, b := fw.lines[0], fw.lines[1]
	if a == b {
		return
	}
	if fw.iqrPath == "" {
		log.Println("cannot launch without path")
		return
	}
	fw.lines = nil
	band := radio.HzBandRange(fw.x2hz(a), fw.x2hz(b))

	// Set fft window based on 30 fps.
	w := band.Width / 30
	if w < 240 {
		w = 240
	} else if w > uint64(fw.w) {
		w = uint64(fw.w)
	}

	args := []string{
		"fft",
		"-w",
		fmt.Sprintf("%v", w),
		"-s",
		fmt.Sprintf("%v", band.Width),
		"-c",
		fmt.Sprintf("%v", band.Center),
		fw.iqrPath}
	if popup {
		args = append(args, "-p")
	}
	log.Printf("launching iqscope %v", args)
	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		log.Println("failed to launch window:", err)
	}

	fw.wg.Add(1)
	go func() {
		defer fw.wg.Done()
		if err := cmd.Wait(); err != nil {
			log.Println("failed to wait:", err)
		}
	}()
}

func (fw *fftWindow) processEvents() bool {
	for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
		if !fw.handleEvent(event) {
			return false
		}
	}
	return true
}
