package http

import (
	"html/template"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/store"
)

type httpHandler struct {
	s          *nicerx.Server
	serverTmpl *template.Template
	bandTmpl   *template.Template
}

const serverTmplStr = `<!DOCTYPE html>
<html>
<head>
<title>SDR!!</title>
<style>
table, th, td {
  border: 1px solid black;
  text-align: right;
}
</style>
</head>
<body>
<h1>welcome to nicerx</h1>
<hr/>

<h2>SDR status &#x1F4FB;</h2>
<ul>
<li>Current frequency: {{printf "%.3f" .SDR.Band.BeginMHz}}--{{printf "%.3f" .SDR.Band.EndMHz}}MHz</li>
<li>Frequency correction: {{.SDR.PPM}}ppm</li>
</ul>

<h2>Scheduled tasks &#x1F552;</h2>
<table>
<tr><th>TaskID</th><th>Name</th><th>Band</th><th>Duration</th><th>Control</th></tr>
{{range $_, $t := .Tasks.Running}}
<tr>
<td>{{$t.Id}}</td>
<td>{{$t.Task.Name}}</td>
<td><a href="band?f={{$t.Band.Center}}">{{printf "%.3f" $t.Band.Center}}</a></td>
<td>{{$t.Duration}}</td>
<td>
<a href="?pause={{$t.Id}}">&#x23F8;&#xfe0f;</a>
<a href="?stop={{$t.Id}}">&#x23F9;&#xfe0f;</a>
</td>
</tr>
{{end}}
</table>


{{$length := len .Tasks.Paused}} {{if gt $length 0}}
<h2>Paused tasks</h2>
<table>
<tr><th>TaskID</th><th>Name</th><th>Runtime</th><th>Control</th></tr>
{{range $_, $t := .Tasks.Paused}}
<tr><td>{{$t.Id}}</td><td>{{$t.Task.Name}}</td><td>{{$t.Duration}}</td>
<td>
<a href="?resume={{$t.Id}}">&#x25B6;&#xFE0F;</a>
<a href="?stop={{$t.Id}}">&#x23F9;&#xFE0F;</a>
</td>
</tr>
{{end}}
</table>
{{end}}


<h2>Scanned frequencies &#x1F4D6;</h2>
<p>Detected bands: {{len .Bands.Bands}}</p>
<table>
<tr><th>Name</th><th>Center MHz</th><th>Bandwidth kHz</th><th>Status</th></tr>
{{range $_, $sb := .SignalBands}}
<tr>
<td>{{$sb.Name}}</td>
<td><a href="band?f={{$sb.Center}}">{{printf "%.3f" $sb.Center}}</a></td>
<td>{{printf "%.2f" $sb.BandwidthKHz}}</td>
<td>
{{if $sb.HasSignal}} &#x1F48C; {{end}}
{{if $sb.HasCapture}} &#x1F3A4; {{end}}
</td>
</tr>
{{end}}
</table>
</body>
</html>
`

const bandTmplStr = `<!DOCTYPE html>
<html>
<head>
<title>Frequency Info -- {{printf "%.3f" .Band.Center}} </title>
<style>
table, td {
  border: 1px solid black;
  text-align: right;
}
th {
  text-align: center;
}
</style>
</head>
<body>
<h1>Frequency Information</h1>
<hr/>

<h2>Band Information</h2>
<ul>
<li>Center: {{printf "%.3fMHz" .Band.Center }}</li>
<li>Bandwidth: {{printf "%.1fKHz" .Band.BandwidthKHz }}</li>
<li>Range: {{printf "%.3f" .Band.BeginMHz}}&mdash;{{printf "%.3f MHz" .Band.EndMHz}}</li>
</ul>

{{$length := len .Spectrograms}} {{if gt $length 0}}
<h2>Signals</h2>
<table>
<tr><th>Date</th><th>Spectogram</th></tr>
{{range $_, $s := .Spectrograms}}
<tr>
<td>{{$s.Date}}</td>
<td><img src="{{ $s.Path }}" /></td>
</tr>
{{end}}
</table>
{{end}}

<h2>Controls</h2>
<ul>
<li><a href="/?capture={{printf "%f" .Band.Center}}">Capture</a></li>
</ul>

</body>
</html>
`

func ServeHttp(s *nicerx.Server, serv string) error {
	h := &httpHandler{
		s:          s,
		serverTmpl: template.Must(template.New("server").Parse(serverTmplStr)),
		bandTmpl:   template.Must(template.New("freq").Parse(bandTmplStr)),
	}
	return http.ListenAndServe(serv, h)
}

type freqInfo struct {
	Band         radio.FreqBand
	Spectrograms []store.SpectrogramFile
}

func (h *httpHandler) handleBand(w http.ResponseWriter, mhzStr string) {
	mhz, err := strconv.ParseFloat(mhzStr, 64)
	if err != nil {
		io.WriteString(w, err.Error())
		return
	}
	fb := radio.FreqBand{Center: mhz, Width: 0.001}
	bands := h.s.Bands.Range(fb)
	if len(bands) == 0 {
		io.WriteString(w, "band never scanned")
		return
	}
	fi := &freqInfo{Band: bands[0], Spectrograms: h.s.Signals.Spectrograms(fb)}
	if err := h.bandTmpl.Execute(w, fi); err != nil {
		io.WriteString(w, err.Error())
	}
}

func (h *httpHandler) handleCapture(mhzStr string) {
	mhz, err := strconv.ParseFloat(mhzStr, 64)
	if err != nil {
		return
	}
	h.s.Capture(mhz)
}

func (h *httpHandler) handleGetIndex(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if captureStr := q.Get("capture"); len(captureStr) > 0 {
		h.handleCapture(captureStr)
	} else if s := q.Get("resume"); len(s) > 0 {
		tid, _ := strconv.ParseInt(s, 10, 64)
		h.s.Resume(nicerx.TaskId(tid))
	} else if s := q.Get("pause"); len(s) > 0 {
		tid, _ := strconv.ParseInt(s, 10, 64)
		h.s.Pause(nicerx.TaskId(tid))
	} else if s := q.Get("stop"); len(s) > 0 {
		tid, _ := strconv.ParseInt(s, 10, 64)
		h.s.Stop(nicerx.TaskId(tid))
	} else {
		if err := h.serverTmpl.Execute(w, h.s); err != nil {
			io.WriteString(w, err.Error())
		}
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		base := path.Base(r.URL.Path)
		if strings.HasSuffix(base, ".jpg") {
			w.Header().Set("Content-Type", "image/jpeg")
			http.ServeFile(w, r, r.URL.Path[1:])
			return
		} else if base == "band" {
			h.handleBand(w, r.URL.Query().Get("f"))
		} else {
			h.handleGetIndex(w, r)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
