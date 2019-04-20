# niceRX

A software defined radio platform.

## Build

### Third party dependencies

Libraries:
* [fftw3](https://www.fftw.org)
* [liquid-dsp](https://github.com/jgaeddert/liquid-dsp)
* [rtl\_tcp](https://osmocom.org/projects/rtl-sdr/wiki)

External decoders:
* [multimon-ng](https://github.com/EliasOenal/multimon-ng)
* [dsd](https://github.com/szechyjs/dsd)

### Compile from sources

```sh
go get github.com/chzchzchz/nicerx/cmd/nicerx
```

## Run

``sh
nicerx serve
```