package main

import (
	"bytes"
	"encoding/json"
	"errors"
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

type config struct {
	OutputDir string `yaml:"output_dir"`

	GeneratePostmanCollection bool   `yaml:"generate_postman_collection"`
	GenerateOpenAPI           bool   `yaml:"generate_openapi"`
	OpenAPIFileType           string `yaml:"openapi_file_type"`

	OpenAPIConfig autodoc.OpenAPIConfig `yaml:"openapi_config"`
}

var defaultConfig = config{
	OutputDir: "autodoc",

	GeneratePostmanCollection: true,
	GenerateOpenAPI:           true,
	OpenAPIFileType:           "yaml",
}

type instance struct {
	config config
}

func (inst *instance) getFiles() (paths []string) {
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

func (inst *instance) fileToRecorder(path string) (r autodoc.Recorder, err error) {
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

func (inst *instance) writeFile(b []byte, fname string) error {
	os.MkdirAll("autodoc", os.ModePerm)
	f, err := os.Create("./autodoc/" + fname)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b)
	return err
}

func (inst *instance) openAPI() error {
	all := autodoc.OpenAPI{
		OpenAPI:       "3.0.3",
		OpenAPIConfig: inst.config.OpenAPIConfig,
		Paths:         map[string]interface{}{},
	}

	paths := inst.getFiles()
	for _, path := range paths {
		fmt.Println("found autodoc file:", path)

		recorder, err := inst.fileToRecorder(path)
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

	return inst.writeFile(y, "openapi.yaml")
}

func (inst *instance) postmanCollection() error {
	paths := inst.getFiles()
	folders := map[string][]autodoc.Recorder{}
	for _, path := range paths {
		fmt.Println("found autodoc file:", path)

		recorder, err := inst.fileToRecorder(path)
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
			req := autodoc.Entry{}
			found := false
			for _, r := range r.Records {
				if r.Options.UseAsRequestExample {
					req = r
					found = true
					break
				}
			}

			if !found {
				// TODO:
				return errors.New("no request example found")
			}

			h := []*postman.Header{}
			for _, rh := range req.Request.Headers {
				h = append(h, &postman.Header{
					Key:   rh.Name,
					Value: rh.Value,
				})
			}

			q := []*postman.QueryParam{}
			for _, rq := range req.Request.QueryString {
				q = append(q, &postman.QueryParam{
					Key:   rq.Name,
					Value: rq.Value,
				})
			}

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
						Raw:  string(req.Request.PostData.Text),
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

	return inst.writeFile(b.Bytes(), "postman_collection.json")
}

func writeDefaultConfig(path string) error {
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := yaml.Marshal(defaultConfig)
	if err != nil {
		return err
	}

	_, err = f.Write(b)
	return err
}

func getConfig(path string) (config, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := writeDefaultConfig("./autodoc/config.yaml")
		if err != nil {
			return defaultConfig, err
		}
	}

	f, err := ioutil.ReadFile(path)
	if err != nil {
		return config{}, err
	}

	c := defaultConfig
	err = yaml.Unmarshal(f, &c)
	if err != nil {
		return config{}, err
	}

	return c, nil
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			// &cli.StringFlag{
			// 	Name:        "format",
			// 	Value:       "openapi",
			// 	Aliases:     []string{"f"},
			// 	Usage:       "openapi",
			// 	Destination: &format,
			// },
		},
		Name: "autodoc",
		Action: func(*cli.Context) error {
			return nil
		},
	}

	cfg, err := getConfig("./autodoc/config.yaml")
	if err != nil {
		panic(err)
	}

	inst := &instance{
		config: cfg,
	}

	if err = app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

	if inst.config.GenerateOpenAPI {
		err := inst.openAPI()
		if err != nil {
			panic(err)
		}
	}

	if inst.config.GeneratePostmanCollection {
		err = inst.postmanCollection()
		if err != nil {
			panic(err)
		}
	}
}
