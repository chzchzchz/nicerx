# niceRX

Software defined radio tools and daemons.

## Build

### Third party dependencies

Libraries:
* [fftw3](https://www.fftw.org)
* [liquid-dsp](https://github.com/jgaeddert/liquid-dsp)
* [rtl\_tcp](https://osmocom.org/projects/rtl-sdr/wiki)
* sdl (for scope)

### Compile from sources

```sh
go get github.com/chzchzchz/nicerx/cmd/sdrproxy
go get github.com/chzchzchz/nicerx/cmd/iqpipe
go get github.com/chzchzchz/nicerx/cmd/iqscope
```

## sdrproxy

Multiplexes and channelizes SDR data via a RESTful JSON interface.

### Run

```sh
sdrproxy --bind localhost:12000
```

### API

Read a radio stream:
```sh
curl -v localhost:12000/api/rx/ -d'{"center_hz" : 100330000, "width_hz" : 15000, "radio" : "3e78268d"}' -o out.dat
```

Read a radio stream with hinting to bind SDR to wider bandwidth:
```sh
curl -N -v localhost:12000/api/rx/ -d'{"hint_tune_hz" : 100800000, "center_hz" : 1009612000, "width_hz" : 30000, "radio" : "20000001"}' -o ~/radio.fifo

```

Current sdrs on system:
```sh
curl -v localhost:12000/api/sdr/
```

## iqscope

```sh
curl -N -v localhost:12000/api/rx/ -d'{"name" : "ffplay", "hint_tune_hz" : 929800000, "center_hz" : 929800000, "width_hz" : 1024000, "radio" : "90000001"}' -o - | ./iqscope fft -s 10240000 -w 480 -r 320 -
```

## Examples

Open a radio stream and display spectrum with ffmpeg (kind of slow, use for sanity checking):
```sh
curl -N -v localhost:12000/api/rx/ -d'{"name" : "ffplay", "hint_tune_hz" : 100800000, "center_hz" : 100800000, "width_hz" : 1024000, "radio" : "90000001"}' -o - |  \
ffmpeg -f u8 -ar 1024000 -ac 2 -i - \
        -lavfi showspectrum="s=1024x480:mode=combined:color=rainbow:overlap=1:slide=scroll:orientation=horizontal" -an -f avi - | \
ffplay -
```

