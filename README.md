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
go get github.com/chzchzchz/nicerx/cmd/sdrproxy
```

## sdrproxy

### Run

```sh
sdrproxy --bind localhost:12000
```

### API

```sh
curl -v localhost:12000/api/rx/ -d'{"center_hz" : 941330000, "width_hz" : 15000, "radio" : "3e78268d"}' -o out.dat
```


## nicerx

### Run

```sh
nicerx serve
```

### API


#### Tuning a radio

```sh
curl -f http://localhost:8080/api/sdr/tune  -XPOST -D'{"id" : "radio1", "center_hz" : 433000000, "width_hz" : 2048000}'
```

##### Receivers

Add a scanner:
```sh
curl  -f -v http://localhost:8080/api/rx/  -XPOST -d'{"user_name": "my_scanner", "type_name" : "scan"}'
```

Stream scan data:
```sh
curl -v http://localhost:8080/api/rx/my_scanner
```

Delete it:
```sh
curl  -f -v http://localhost:8080/api/rx/my_scanner  -XDELETE
```

