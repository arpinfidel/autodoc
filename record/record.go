package autodoc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"

	"github.com/google/martian/har"
)

type Recorder struct {
	Path               string  `json:"path"`
	Method             string  `json:"method"`
	Tag                string  `json:"tag"`
	APIDescription     string  `json:"api_description"`
	APISummary         string  `json:"api_summary"`
	ExpectedStatusCode int     `json:"expected_status_code"`
	Records            []Entry `json:"records"`
	recordsLock        *sync.RWMutex
}

type Entry struct {
	har.Entry
	Options *RecordOptions `json:"options"`
}

type RecordOptions struct {
	RecordDescription string

	UseAsRequestExample          bool
	ExcludeFromOpenAPI           bool
	ExcludeFromPostmanCollection bool
}

// func (p *Payload) getJSON() interface{} {
// 	m := map[string]interface{}{}
// 	j := map[string]interface{}{}
// 	d := json.NewDecoder(bytes.NewReader(p.Body))
// 	d.UseNumber()
// 	d.Decode(&j)
// 	for k, v := range j {
// 		m[k] = getType(v)
// 	}
// 	return m
// }

func (re *Recorder) Record(h http.HandlerFunc, opts ...RecordOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// call actual handler
		ww := createResponseRecorder(w)
		h(ww, r)
		re.record(r, ww.recorder.Result(), opts...)
	}
}

func (re *Recorder) record(req *http.Request, res *http.Response, opts ...RecordOptions) {
	if re.recordsLock == nil {
		re.recordsLock = &sync.RWMutex{}
	}

	rec := Entry{}
	if len(opts) > 0 {
		rec.Options = &opts[0]
	} else {
		// TODO: default options
		rec.Options = &RecordOptions{}
	}

	l := har.NewLogger()
	l.RecordRequest("", req)
	l.RecordResponse("", res)
	rec.Entry = *l.Export().Log.Entries[0]

	re.recordsLock.Lock()
	re.Records = append(re.Records, rec)
	re.recordsLock.Unlock()
}

// responseRecorder writes to both a responseRecorder and the original ResponseWriter
type responseRecorder struct {
	http.ResponseWriter
	recorder     *httptest.ResponseRecorder
	closeChannel chan bool
}

func createResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		recorder:       httptest.NewRecorder(),
		closeChannel:   make(chan bool, 1),
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.recorder.Header()
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.recorder.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	// TODO: temp fix for sse
	if statusCode == -1 {
		statusCode = 200
	}
	r.recorder.WriteHeader(statusCode)
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func (r *Recorder) JSON() []byte {
	j, _ := json.Marshal(r)
	return j
}

func (r *Recorder) JSONString() string {
	return string(r.JSON())
}

func (r *Recorder) GenerateFile() error {
	path := "./autodoc/autodoc-" + r.Method + "-" + strings.TrimLeft(strings.ReplaceAll(r.Path, "/", "_"), "_") + ".json"
	os.Mkdir("autodoc", os.ModePerm)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(r.JSON())
	return err
}
