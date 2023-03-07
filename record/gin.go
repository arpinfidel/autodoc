package autodoc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/martian/har"
)

// ginResponseRecorder writes to both a ResponseRecorder and the original ResponseWriter
type ginResponseRecorder struct {
	gin.ResponseWriter
	recorder     *httptest.ResponseRecorder
	closeChannel chan bool
}

func (r *ginResponseRecorder) Header() http.Header {
	return r.recorder.Header()
}

func (r *ginResponseRecorder) Write(b []byte) (int, error) {
	r.recorder.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *ginResponseRecorder) WriteHeader(statusCode int) {
	// TODO: temp fix for sse
	if statusCode == -1 {
		statusCode = 200
	}
	r.recorder.WriteHeader(statusCode)
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *ginResponseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func createGinResponseRecorder(w gin.ResponseWriter) *ginResponseRecorder {
	return &ginResponseRecorder{
		ResponseWriter: w,
		recorder:       httptest.NewRecorder(),
		closeChannel:   make(chan bool, 1),
	}
}

func createTestGinContext(c *gin.Context) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := createGinResponseRecorder(c.Writer)
	c.Writer = rec
	return c, rec.recorder
}

func (r *Recorder) RecordGin(h gin.HandlerFunc, opts ...RecordOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		c, rec := createTestGinContext(c)

		if c.Request.URL.Path == "" {
			p := r.Path
			re := regexp.MustCompile(`{(.*)}`)
			matches := re.FindAllString(r.Path, -1)
			for _, m := range matches {
				p = strings.ReplaceAll(p, m, c.Param(strings.Trim(m, "{}")))
			}
			c.Request.URL.Path = p
		}

		h(c)

		fmt.Printf(">> debug >> c.Request.URL.String(): %#v\n", c.Request.URL.String())

		l := har.NewLogger()
		l.SetOption(har.BodyLogging(true))
		l.RecordRequest("a", c.Request)
		l.RecordResponse("a", rec.Result())
		h := l.Export()
		j, _ := json.Marshal(h)
		fmt.Printf(">> debug >> j: %#v\n", string(j))
	}
}
