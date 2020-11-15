# niceRX

Software defined radio tools and daemons.

## Build

### Third party dependencies

Libraries:
* [fftw3](https://www.fftw.org)
* [liquid-dsp](https://github.com/jgaeddert/liquid-dsp)
* [rtl\_tcp](https://osmocom.org/projects/rtl-sdr/wiki)
* [sdl](https://libsdl.org) (for scope)

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

View SDRs on system:
```sh
curl -v localhost:12000/api/sdr/
```

Read a radio stream:
```sh
curl -v localhost:12000/api/rx/ -d'{"center_hz" : 100000000, "width_hz" : 15000, "radio" : "123"}' -o out.dat
```

Read a radio stream with hinting to bind SDR to wider bandwidth:
```sh
curl -N -v localhost:12000/api/rx/ -d'{"hint_tune_hz" : 100000000, "center_hz" : 1009612000, "width_hz" : 30000, "radio" : "123"}' -o out.dat

```

## iqpipe

FM demodulate a pager signal:
```sh
curl ... -o - | cmd/iqpipe/iqpipe fmdemod - - -s 30000 -p 22050 -d 9600 | multimon-ng -
```

## iqscope

Stream sdrproxy channel to waterfall:
```sh
curl -N -v localhost:12000/api/rx/ -d'{"name" : "iqscope", "hint_tune_hz" : 100000000, "center_hz" : 100000000, "width_hz" : 1024000, "radio" : "123"}' -o - | ./iqscope fft -c 100000000 -s 1024000 -R -w 1024 -r 600 -
```

Open a radio stream and display spectrum with ffmpeg (super slow, use for sanity checking):
```sh
curl -N -v localhost:12000/api/rx/ -d'{"name" : "ffplay", "hint_tune_hz" : 100000000, "center_hz" : 100000000, "width_hz" : 1024000, "radio" : "123"}' -o - |  \
ffmpeg -f u8 -ar 1024000 -ac 2 -i - \
        -lavfi showspectrum="s=1024x480:mode=combined:color=rainbow:overlap=1:slide=scroll:orientation=horizontal" -an -f avi - | \
ffplay -
```

