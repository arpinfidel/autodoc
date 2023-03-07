package autodoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type HarEntry struct {
	StartedDateTime string     `json:"startedDateTime"`
	Time            int        `json:"time"`
	Request         HarReqRes  `json:"request"`
	Response        HarReqRes  `json:"response"`
	Timings         HarTimings `json:"timings"`
}

type HarReqRes struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	HTTPVersion string            `json:"httpVersion"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body,omitempty"`
	Status      int               `json:"status,omitempty"`
	StatusText  string            `json:"statusText,omitempty"`
}

type HarTimings struct {
	Blocked int `json:"blocked"`
	DNS     int `json:"dns"`
	Connect int `json:"connect"`
	Send    int `json:"send"`
	Wait    int `json:"wait"`
	Receive int `json:"receive"`
}

func readRequestBody(req *http.Request) (string, error) {
	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return "", err
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	return string(bodyBytes), nil
}

func WrapHTTPHandlerFunc(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// create a new response recorder to capture the response
		rec := httptest.NewRecorder()

		// record the start time of the request
		start := time.Now()

		// call the original handler with the response recorder
		h(rec, r)

		// calculate the time taken for the request
		duration := time.Since(start)

		// create a new HAR entry
		entry := HarEntry{
			StartedDateTime: start.Format(time.RFC3339),
			Time:            int(duration / time.Millisecond),
			Request: HarReqRes{
				Method:      r.Method,
				URL:         getRequestURL(r),
				HTTPVersion: r.Proto,
				Headers:     r.Header,
			},
			Timings: HarTimings{},
		}

		// read request and response bodies
		if requestBody, err := readRequestBody(r); err == nil {
			entry.Request.Body = requestBody
		}

		if responseBody, err := readResponseBody(rec); err == nil {
			entry.Response.Body = responseBody
		}

		// update the HAR entry with the response values from the recorder
		for k, v := range rec.Header() {
			entry.Response.Headers[k] = strings.Join(v, ", ")
		}
		entry.Response.Status = rec.Code
		entry.Response.StatusText = http.StatusText(rec.Code)
		// read response body again if it's a Server-Sent Event (SSE)
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
			if responseBody, err := readResponseBody(rec); err == nil {
				// replace newlines with carriage returns to match SSE format
				entry.Response.Body = strings.ReplaceAll(responseBody, "\n", "\r\n")
			}
		}

		// calculate timings
		entry.Timings.Blocked = -1
		entry.Timings.DNS = -1
		entry.Timings.Connect = -1
		entry.Timings.Send = -1
		entry.Timings.Wait = int(duration / time.Millisecond)
		entry.Timings.Receive = -1

		// marshal the HAR entry to JSON
		jsonBytes, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			log.Println("error marshalling HAR entry to JSON:", err)
			return
		}

		// log the JSON to stdout
		fmt.Println(string(jsonBytes))

		// write the response from the recorder to the actual response writer
		for k, v := range rec.Header() {
			w.Header().Set(k, v[0])
		}
		w.WriteHeader(rec.Code)
		if _, err := w.Write(rec.Body.Bytes()); err != nil {
			log.Println("error writing response body:", err)
			return
		}
	}
}

func WrapGinHandlerFunc(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		// create a new response recorder to capture the response
		rec := httptest.NewRecorder()

		// record the start time of the request
		start := time.Now()

		// call the original handler with the response recorder
		h(rec, c)

		// calculate the time taken for the request
		duration := time.Since(start)

		// create a new HAR entry
		entry := HarEntry{
			StartedDateTime: start.Format(time.RFC3339),
			Time:            int(duration / time.Millisecond),
			Request: HarReqRes{
				Method:      c.Request.Method,
				URL:         getRequestURL(c.Request),
				HTTPVersion: c.Request.Proto,
				Headers:     c.Request.Header,
			},
			Timings: HarTimings{},
		}

		// read request and response bodies
		if requestBody, err := c.GetRawData(); err == nil {
			entry.Request.Body = string(requestBody)
			c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(requestBody))
		}

		if responseBody, err := readResponseBody(rec); err == nil {
			entry.Response.Body = responseBody
		}

		// update the HAR entry with the response values from the recorder
		for k, v := range rec.Header() {
			entry.Response.Headers[k] = strings.Join(v, ", ")
		}
		entry.Response.Status = rec.Code
		entry.Response.StatusText = http.StatusText(rec.Code)

		// read response body again if it's a Server-Sent Event (SSE)
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
			if responseBody, err := readResponseBody(rec); err == nil {
				// replace newlines with carriage returns to match SSE format
				entry.Response.Body = strings.ReplaceAll(responseBody, "\n", "\r\n")
			}
		}

		// calculate timings
		entry.Timings.Blocked = -1
		entry.Timings.DNS = -1
		entry.Timings.Connect = -1
		entry.Timings.Send = -1
		entry.Timings.Wait = int(duration / time.Millisecond)
		entry.Timings.Receive = -1

		// marshal the HAR entry to JSON
		jsonBytes, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			log.Println("error marshalling HAR entry to JSON:", err)
			return
		}

		// log the JSON to stdout
		fmt.Println(string(jsonBytes))

		// write the response from the recorder to the actual response writer
		for k, v := range rec.Header() {
			c.Writer.Header().Set(k, v[0])
		}
		c.Writer.WriteHeader(rec.Code)
		if _, err := c.Writer.Write(rec.Body.Bytes()); err != nil {
			log.Println("error writing response body:", err)
			return
		}
	}
}

// readResponseBody reads the response body from the recorder and returns it as a string.
func readResponseBody(rec *httptest.ResponseRecorder) (string, error) {
	if rec.Body == nil {
		return "", nil
	}
	bodyBytes, err := ioutil.ReadAll(rec.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

// getRequestURL returns the full URL of the request, including any query string parameters.
func getRequestURL(req *http.Request) string {
	url := *req.URL
	url.RawQuery = ""
	return url.String()
}
