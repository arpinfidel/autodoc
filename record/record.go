package autodoc

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/martian/har"
)

type Recorder struct {
	Path           string `json:"path"`
	Method         string `json:"method"`
	Tag            string `json:"tag"`
	APIDescription string `json:"api_description"`
	APISummary     string `json:"api_summary"`

	Options *RecorderOptions `json:"options"`

	Records []Entry `json:"records"`

	recordsLock *sync.RWMutex
}

type RecorderOptions struct {
	LogStartedDateTime bool `json:"log_started_date_time"`
}

type Entry struct {
	har.Entry
	Options *RecordOptions `json:"options"`
}

type RecordOptions struct {
	RequestName         string
	RequestSummary      string
	ResponseDescription string

	UseAsRequestExample          bool
	ExcludeFromOpenAPI           bool
	ExcludeFromPostmanCollection bool
}

// ResponseExample returns the response content for the entry
func (e *Entry) ResponseExample(desc string) map[string]interface{} {
	switch e.Response.Status {
	case 301, 302, 303, 307, 308:
		return map[string]interface{}{
			"description": desc,
		}
	default:
		return map[string]interface{}{
			"description": desc,
			"content": map[string]interface{}{
				getContentType(e.Response.Headers): map[string]interface{}{
					"schema": map[string]interface{}{
						"type":       "object",
						"properties": getJSONSchema(e.Response.Content.Text),
					},
				},
			},
		}
	}

}

func (re *Recorder) Record(h http.HandlerFunc, opts ...RecordOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// call actual handler
		ww := createResponseRecorder(w)
		req := r.Clone(context.Background())
		if req.Body != nil {
			body, _ := ioutil.ReadAll(req.Body)
			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
		h(ww, r)
		re.record(req, ww.recorder.Result(), opts...)
	}
}

func (re *Recorder) record(req *http.Request, res *http.Response, opts ...RecordOptions) {
	if re.recordsLock == nil {
		re.recordsLock = &sync.RWMutex{}
	}

	if re.Options == nil {
		re.Options = &RecorderOptions{}
	}

	rec := Entry{}
	if len(opts) > 0 {
		rec.Options = &opts[0]
	} else {
		// TODO: default options
		rec.Options = &RecordOptions{}
	}

	l := har.NewLogger()
	l.SetOption(har.BodyLogging(true))
	l.RecordRequest("", req)
	l.RecordResponse("", res)
	rec.Entry = *l.Export().Log.Entries[0]

	// har library doesn't read body if content length etc not set
	if rec.Entry.Request.PostData == nil && req.Body != nil {
		body, _ := ioutil.ReadAll(req.Body)
		rec.Entry.Request.PostData = &har.PostData{
			Text: string(body),
		}
	}

	// to prevent constant changes
	if !re.Options.LogStartedDateTime {
		rec.Entry.StartedDateTime = time.Time{}
		rec.Entry.Time = 0
	}

	// sort querystring
	if rec.Entry.Request.QueryString != nil {
		sort.Slice(rec.Entry.Request.QueryString, func(i, j int) bool {
			return rec.Entry.Request.QueryString[i].Name < rec.Entry.Request.QueryString[j].Name
		})
	}

	// sort headers
	if rec.Entry.Request.Headers != nil {
		sort.Slice(rec.Entry.Request.Headers, func(i, j int) bool {
			return rec.Entry.Request.Headers[i].Name < rec.Entry.Request.Headers[j].Name
		})
	}

	if rec.Entry.Response.Headers != nil {
		sort.Slice(rec.Entry.Response.Headers, func(i, j int) bool {
			return rec.Entry.Response.Headers[i].Name < rec.Entry.Response.Headers[j].Name
		})
	}

	if rec.Entry.Request.PostData != nil {
		sort.Slice(rec.Entry.Request.PostData.Params, func(i, j int) bool {
			return rec.Entry.Request.PostData.Params[i].Name < rec.Entry.Request.PostData.Params[j].Name
		})
	}

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

func (re *Recorder) JSON() []byte {
	j, _ := json.Marshal(re)
	return j
}

func (re *Recorder) JSONString() string {
	return string(re.JSON())
}

func (re *Recorder) GenerateFile() error {
	path := "./autodoc/autodoc-" + re.Method + "-" + strings.TrimLeft(strings.ReplaceAll(re.Path, "/", "_"), "_") + ".json"
	os.Mkdir("autodoc", os.ModePerm)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(re.JSON())
	return err
}
