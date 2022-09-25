# autodoc
Automatically generate OpenAPI documentation from unit tests

Currently only supports json request/response and path/query parameters

- import recorder
- call the record and generate functions in your test cases. this will generate temporary files
- run the test cases
- call `autodoc` in your root directory to generate the OpenAPI file containing all tests

# usage

```
go install github.com/arpinfidel/autodoc
```

```go
import autodoc "github.com/arpinfidel/autodoc/record"
```

```go
r := autodoc.Recorder{
  Path:   "/foo/bar",
  Method: "post",
  Tag:    "foo",
  ExpectedStatusCode: 200,
}

for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
    gin.SetMode(gin.TestMode)

    c, _ := gin.CreateTestContext(w)
    c.Request = &http.Request{
      URL:           &url.URL{},
      MultipartForm: &multipart.Form{},
      TLS:           &tls.ConnectionState{},
      Response:      &http.Response{},
    }

    // Foobar being a gin.HandlerFunc
    r.RecordGin(handler.FooBar)(c)
    r.GenerateFile()

    // Or for standard http handler
    // r.Record(handler.FooBar)(w, r)
  }
}
```

```bash
$ autodoc
```

# todo
- [ ] headers
- [ ] form body
- [ ] postman collection
- [ ] other body types
- [ ] description from go doc
- [ ] descriptions and other fields
