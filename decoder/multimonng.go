package decoder

import (
	"encoding/binary"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"

	"github.com/chzchzchz/nicerx/dsp"

	"github.com/kr/pty"
)

func FlexDecode(rate float32, sigc <-chan []complex64) (string, error) {
	syscall.Mkfifo("flex.fifo", 0644)
	h := 22050.0 / rate
	cmd, fpty, err := newMultimonngProcess("flex.fifo")
	if err != nil {
		return "", err
	}
	defer fpty.Close()
	go func() {
		ffifo, err := os.OpenFile("flex.fifo", os.O_RDWR, 0644)
		if err != nil {
			cmd.Process.Kill()
			return
		}
		defer ffifo.Close()
		demodc := dsp.DemodFM(float32(h), sigc)
		for rsamps := range dsp.Resample(float32(h), demodc) {
			// 16-bit signed le
			outsamps := make([]int16, len(rsamps))
			for i, v := range rsamps {
				outsamps[i] = int16(v * 65536)
			}
			if err := binary.Write(ffifo, binary.LittleEndian, outsamps); err != nil {
				panic(err)
			}
		}
	}()
	b, _ := ioutil.ReadAll(fpty)
	return string(b), cmd.Wait()
}

func newMultimonngProcess(fifoPath string) (*exec.Cmd, *os.File, error) {
	cmd := exec.Command("multimon-ng", "-v9", "-c", "-a", "FLEX", "-t", "raw", fifoPath)
	fpty, err := pty.Start(cmd)
	return cmd, fpty, err
}
