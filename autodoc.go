package autodoc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type Recorder struct {
	Path     string
	Method   string
	Tag      string
	request  payload
	response response
}

type payload struct {
	headers map[string][]string
	body    []byte
}

type response struct {
	payload
	statusCode int
}

func (p *payload) contentType() string {
	for k, v := range p.headers {
		if strings.ToLower(k) == "content-type" && len(v) > 0 {
			return v[0]
		}
	}
	return "application/json"
}

func getType(i interface{}) map[string]interface{} {
	m := map[string]interface{}{}
	switch i := i.(type) {
	case json.Number:
		if n, err := i.Int64(); err == nil {
			m = map[string]interface{}{
				"type":    "integer",
				"example": n,
			}
		} else if n, err := i.Float64(); err == nil {
			m = map[string]interface{}{
				"type":    "float",
				"example": n,
			}
		} else {
			panic("unexpected type")
		}
	case string:
		m = map[string]interface{}{
			"type":    "string",
			"example": i,
		}
	case bool:
		m = map[string]interface{}{
			"type":    "boolean",
			"example": i,
		}
	case map[string]interface{}:
		m = map[string]interface{}{
			"type": "object",
		}
		p := map[string]interface{}{}
		for k, v := range i {
			p[k] = getType(v)
		}
		m["properties"] = p
	default:
		panic("unexpected type")
	}
	return m
}

func (p *payload) getJSON() interface{} {
	m := map[string]interface{}{}
	j := map[string]interface{}{}
	println(string(p.body))
	d := json.NewDecoder(bytes.NewReader(p.body))
	d.UseNumber()
	d.Decode(&j)
	fmt.Printf("j: %#v\n", j)
	for k, v := range j {
		m[k] = getType(v)
	}
	return m
}

type writerRecorder struct {
	w          http.ResponseWriter
	body       []byte
	statusCode int
}

func (w *writerRecorder) Header() http.Header {
	return w.w.Header()
}

func (w *writerRecorder) Write(b []byte) (int, error) {
	w.body = append(make([]byte, 0, len(b)), b...)
	return w.w.Write(b)
}

func (w *writerRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.w.WriteHeader(statusCode)
}

type ginWriterRecorder struct {
	writerRecorder
	w gin.ResponseWriter
}

func (w *ginWriterRecorder) CloseNotify() <-chan bool {
	return w.w.CloseNotify()
}

func (w *ginWriterRecorder) Flush() {
	w.w.Flush()
}

func (w *ginWriterRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.w.Hijack()
}

func (w *ginWriterRecorder) Pusher() http.Pusher {
	return w.w.Pusher()
}

func (w *ginWriterRecorder) Status() int {
	return w.w.Status()
}
func (w *ginWriterRecorder) Size() int {
	return w.w.Size()
}
func (w *ginWriterRecorder) WriteString(s string) (int, error) {
	return w.w.WriteString(s)
}
func (w *ginWriterRecorder) Written() bool {
	return w.w.Written()
}
func (w *ginWriterRecorder) WriteHeaderNow() {
	w.w.WriteHeaderNow()
}

func newGinWriterRecorder(w gin.ResponseWriter) ginWriterRecorder {
	return ginWriterRecorder{
		w: w,
		writerRecorder: writerRecorder{
			w: w,
		},
	}
}

func (re *Recorder) Record(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			body, _ := ioutil.ReadAll(r.Body)
			re.request.body = body
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
		}
		re.request.headers = r.Header.Clone()

		ww := writerRecorder{
			w: w,
		}
		h(&ww, r)

		re.response.body = ww.body
		re.response.headers = ww.Header().Clone()
		re.response.statusCode = ww.statusCode
	}
}

func (r *Recorder) RecordGin(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// cc := c.Copy()
		ww := newGinWriterRecorder(c.Writer)
		r.Record(func(w http.ResponseWriter, r *http.Request) {
			c.Writer = &ww
			h(c)
		})(&ww, c.Request)
		r.response.statusCode = ww.Status()
		r.response.body = ww.body
	}
}

func (r *Recorder) Print() {
	requestBody := map[string]interface{}{}
	{
		content := map[string]interface{}{
			r.request.contentType(): map[string]interface{}{
				"schema": map[string]interface{}{
					"type":       "object",
					"properties": r.request.getJSON(),
				},
			},
		}
		requestBody["content"] = content
	}
	responses := map[string]interface{}{
		strconv.Itoa(r.response.statusCode): map[string]interface{}{
			"description": "",
			"content": map[string]interface{}{
				r.response.contentType(): map[string]interface{}{
					"schema": map[string]interface{}{
						"type":       "object",
						"properties": r.response.getJSON(),
					},
				},
			},
		},
	}
	yml := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			r.Path: map[string]interface{}{
				r.Method: map[string]interface{}{
					"tags":        []string{r.Tag},
					"requestBody": requestBody,
					"responses":   responses,
				},
			},
		},
	}

	y, _ := yaml.Marshal(yml)
	println(string(y))
}
