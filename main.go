package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	autodoc "github.com/arpinfidel/autodoc/record"
	postman "github.com/rbretecher/go-postman-collection"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func getFiles() (paths []string) {
	filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		p := strings.Split(path, "/")
		folder := ""
		file := ""
		if len(p) > 1 {
			folder = p[len(p)-2]
			file = p[len(p)-1]
		}

		if folder != "autodoc" || !strings.HasPrefix(file, "autodoc-") {
			return nil
		}

		paths = append(paths, path)
		return nil
	})

	return paths
}

func fileToRecorder(path string) (r autodoc.Recorder, err error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return r, err
	}

	err = json.Unmarshal(f, &r)
	if err != nil {
		return r, err
	}

	return r, nil
}

func writeFile(b []byte, fname string) error {
	os.Mkdir("autodoc", os.ModePerm)
	f, err := os.Create("./autodoc/" + fname)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b)
	return err
}

func openAPI() error {
	all := autodoc.OpenAPI{
		OpenAPI: "3.0.3",
		Info: autodoc.OpenAPIInfo{
			Title:   "",
			Version: "1.0.0",
		},
		Paths: map[string]interface{}{},
	}

	paths := getFiles()
	for _, path := range paths {
		fmt.Println("found autodoc file:", path)

		recorder, err := fileToRecorder(path)
		if err != nil {
			return err
		}

		o := recorder.OpenAPI()

		for path, m := range o.Paths {
			m := m.(map[string]interface{})
			if all.Paths[path] == nil {
				all.Paths[path] = m
				continue
			}
			for method, m2 := range m {
				all.Paths[path].(map[string]interface{})[method] = m2
			}
		}
	}

	y, err := yaml.Marshal(all)
	if err != nil {
		return err
	}

	return writeFile(y, "openapi.yaml")
}

func postmanCollection() error {
	paths := getFiles()
	folders := map[string][]autodoc.Recorder{}
	for _, path := range paths {
		fmt.Println("found autodoc file:", path)

		recorder, err := fileToRecorder(path)
		if err != nil {
			return err
		}

		folders[recorder.Tag] = append(folders[recorder.Tag], recorder)
		// TODO: option for folder by url
	}

	// TODO: collection name
	c := postman.CreateCollection("Autodoc", "")
	for f, record := range folders {
		folder := c.AddItemGroup(f)
		if f == "" {
			f = "untagged"
		}

		for _, r := range record {
			req := autodoc.Record{}
			for _, r := range r.Records {
				if r.Options.UseAsRequestExample {
					req = r
					break
				}
			}
			h := []*postman.Header{}
			for k, v := range req.Request.Headers {
				if len(v) == 0 {
					continue
				}
				// TODO: handle arrays
				h = append(h, &postman.Header{
					Key:   k,
					Value: v[0],
				})
			}

			q := []*postman.QueryParam{}
			for k, v := range req.Request.QueryParams {
				if len(v) == 0 {
					continue
				}

				// TODO: handle arrays
				q = append(q, &postman.QueryParam{
					Key:   k,
					Value: v[0],
				})
			}

			println(string(req.Request.Body))
			item := postman.CreateItem(postman.Item{
				Name: fmt.Sprintf("[%s] %s", r.Method, r.Path),
				Request: &postman.Request{
					Description: "", // TODO:
					Method:      postman.Method(strings.ToUpper(r.Method)),
					URL: &postman.URL{
						Raw:   "http://localhost" + r.Path, //TODO:
						Query: q,
					},
					Header: h,
					Body: &postman.Body{
						Mode: "json", //TODO:
						Raw:  string(req.Request.Body),
					},
				},
			})
			folder.AddItem(item)
		}
	}

	b := bytes.NewBuffer([]byte{})
	err := c.Write(b, postman.V200)
	if err != nil {
		return err
	}

	println(b.String())

	return writeFile(b.Bytes(), "postman_collection.json")
}

func main() {
	var (
		format string
	)
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "format",
				Value:       "openapi",
				Aliases:     []string{"f"},
				Usage:       "openapi",
				Destination: &format,
			},
		},
		Name: "autodoc",
		Action: func(*cli.Context) error {
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

	switch format {
	case "openapi":
		err := openAPI()
		if err != nil {
			panic(err)
		}
		err = postmanCollection()
		if err != nil {
			panic(err)
		}
	}
}
