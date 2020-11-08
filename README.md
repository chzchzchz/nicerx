# niceRX

Software defined radio tools and daemons.

## Build

### Third party dependencies

Libraries:
* [fftw3](https://www.fftw.org)
* [liquid-dsp](https://github.com/jgaeddert/liquid-dsp)
* [rtl\_tcp](https://osmocom.org/projects/rtl-sdr/wiki)

### Compile from sources

```sh
go get github.com/chzchzchz/nicerx/cmd/nicerx
go get github.com/chzchzchz/nicerx/cmd/sdrproxy
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
curl -v localhost:12000/api/rx/ -d'{"center_hz" : 941330000, "width_hz" : 15000, "radio" : "3e78268d"}' -o out.dat
```

Read a radio stream with hinting to bind SDR to wider bandwidth:
```sh
curl -N -v localhost:12000/api/rx/ -d'{"hint_tune_hz" : 929800000, "center_hz" : 929612000, "width_hz" : 30000, "radio" : "20000001"}' -o ~/radio.fifo

```
