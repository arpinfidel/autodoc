package autodoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type Recorder struct {
	Path               string
	Method             string
	Tag                string
	ExpectedStatusCode int
	records            []record
	recordsLock        sync.RWMutex
}

type record struct {
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
			panic(fmt.Sprintf("unexpected type %T", i))
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
	case []interface{}:
		m = map[string]interface{}{
			"type": "array",
		}
		if len(i) > 0 {
			m["items"] = getType(i[0])
		}
	default:
		panic(fmt.Sprintf("unexpected type %T %#v", i, i))
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
	http.ResponseWriter
	body       []byte
	statusCode int
}

func (w *writerRecorder) Body() []byte {
	return w.body
}

func (w *writerRecorder) StatusCode() int {
	return w.statusCode
}

func (w *writerRecorder) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w *writerRecorder) Write(b []byte) (int, error) {
	w.body = append(make([]byte, 0, len(b)), b...)
	return w.ResponseWriter.Write(b)
}

func (w *writerRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (re *Recorder) Record(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := record{}
		if r.Body != nil {
			body, _ := ioutil.ReadAll(r.Body)
			rec.request.body = body
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
		}
		rec.request.headers = r.Header.Clone()

		ww := writerRecorder{
			ResponseWriter: w,
		}
		h(&ww, r)

		rec.response.body = ww.body
		rec.response.headers = ww.Header().Clone()
		rec.response.statusCode = ww.statusCode
		re.recordsLock.Lock()
		re.records = append(re.records, rec)
		re.recordsLock.Unlock()
	}
}

func (r *Recorder) RecordGin(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		r.Record(func(w http.ResponseWriter, r *http.Request) {
			cc, _ := gin.CreateTestContext(w)
			c.Writer = cc.Writer
			h(c)
		})(c.Writer, c.Request)
	}
}

type OpenAPI map[string]interface{}

func (o *OpenAPI) Bytes() []byte {
	y, _ := yaml.Marshal(o)
	return y
}

func (o *OpenAPI) String() string {
	return string(o.Bytes())
}

func (r *Recorder) OpenAPI() OpenAPI {
	req := payload{}
	for _, rec := range r.records {
		if rec.response.statusCode == r.ExpectedStatusCode {
			req = rec.request
		}
	}
	requestBody := map[string]interface{}{}
	{
		content := map[string]interface{}{
			req.contentType(): map[string]interface{}{
				"schema": map[string]interface{}{
					"type":       "object",
					"properties": req.getJSON(),
				},
			},
		}
		requestBody["content"] = content
	}

	responses := map[string]interface{}{}
	for _, rec := range r.records {
		responses[strconv.Itoa(rec.response.statusCode)] = map[string]interface{}{
			"description": "",
			"content": map[string]interface{}{
				rec.response.contentType(): map[string]interface{}{
					"schema": map[string]interface{}{
						"type":       "object",
						"properties": rec.response.getJSON(),
					},
				},
			},
		}
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
	return yml
}
