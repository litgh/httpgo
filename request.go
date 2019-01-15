package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
)

// Http methods
const (
	GET     = "GET"
	POST    = "POST"
	PUT     = "PUT"
	DELETE  = "DELETE"
	HEAD    = "HEAD"
	OPTIONS = "OPTIONS"
	PATCH   = "PATCH"
)

var (
	//HTTPMethods is this http methods list
	HTTPMethods = []string{GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS}
	regHeader   = regexp.MustCompile(`([a-zA-Z0-9-_]+:\s)(.+)`)
	regStatus   = regexp.MustCompile(`(HTTP/1\.1) (([2345])\d{2})`)
)

// Request is the http request
type Request struct {
	Method          string
	URL             *url.URL
	Username        string
	Password        string
	Proxy           string
	JSON            bool
	Form            bool
	Bench           bool
	Call            bool
	NumberOfRequest uint64
	Concurrency     uint64
	Duration        time.Duration
	Timeout         time.Duration
	Header          http.Header
	Values          url.Values
	Fields          url.Values
	Files           url.Values
	JSONMap         map[string][]interface{}
	Body            bytes.Buffer
	ResponseBody    []byte
}

func (r Request) String() string {
	return r.Method + " " + r.URL.String()
}

func newReq() *Request {
	return &Request{Header: make(http.Header), Values: make(url.Values), Files: make(url.Values), Fields: make(url.Values), JSON: true, JSONMap: make(map[string][]interface{})}
}

func (r *Request) reset() {
	r.Body.Reset()
	r.ResponseBody = nil
	r.Fields = make(url.Values)
	r.Files = make(url.Values)
	r.Values = make(url.Values)
	r.JSONMap = make(map[string][]interface{})

}

func (r *Request) newHTTPRequest() (httpReq *http.Request, err error) {
	if r.URL == nil {
		return nil, fmt.Errorf("URL not set")
	}
	var _URL = r.URL.String()
	if len(r.Values) != 0 {
		_URL += "?" + r.Values.Encode()
	}
	r.Header.Set("Content-Type", "application/json; charset=UTF-8")
	r.Header.Set("Accept", "application/json")

	if r.Method == POST || r.Method == PUT || r.Method == PATCH {
		var body io.Reader
		if req.Body.Len() > 0 {
			body = bytes.NewReader(req.Body.Bytes())
		} else if len(r.JSONMap) > 0 && r.JSON {
			body = r.jsonBody()
		} else if len(r.Files) > 0 {
			pipeReader, pipeWriter := io.Pipe()
			bodyWriter := multipart.NewWriter(pipeWriter)
			go func() {
				for param, filename := range r.Files {
					for _, file := range filename {
						fileWriter, err := bodyWriter.CreateFormFile(param, file)
						if err != nil {
							req.error(err)
						}
						f, err := os.Open(file)
						if err != nil {
							req.error(err)
						}
						_, err = io.Copy(fileWriter, f)
						f.Close()
						if err != nil {
							req.error(err)
						}
					}
				}
				for k, v := range r.Fields {
					for _, l := range v {
						bodyWriter.WriteField(k, l)
					}
				}
				bodyWriter.Close()
				pipeWriter.Close()
			}()
			r.Header.Set("Content-Type", bodyWriter.FormDataContentType())
			body = ioutil.NopCloser(pipeReader)
		} else if len(r.Fields) > 0 {
			if r.JSON {
				body = r.jsonBody()
			} else {
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				body = strings.NewReader(r.Fields.Encode())
			}
		}
		httpReq, err = http.NewRequest(r.Method, _URL, body)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	if r.Method == GET || r.Method == DELETE || r.Method == OPTIONS {
		httpReq, _ = http.NewRequest(r.Method, _URL, nil)
	}
	if r.Username != "" {
		httpReq.SetBasicAuth(r.Username, r.Password)
	}
	httpReq.Header = r.Header
	return
}

func (r *Request) jsonBody() io.Reader {
	js := make(map[string]interface{})

	if len(r.Fields) > 0 {
		for k, v := range r.Fields {
			for _, x := range v {
				r.JSONMap[k] = append(r.JSONMap[k], x)
			}
		}
	}

	for k, v := range r.JSONMap {
		if len(v) == 1 {
			js[k] = v[0]
		} else if len(v) > 1 {
			js[k] = v
		}
	}
	b, _ := json.Marshal(js)
	r.JSONMap = make(map[string][]interface{})
	return bytes.NewReader(b)
}

func (r *Request) errorf(format string, a ...interface{}) {
	r.Call = false
	fmt.Printf(format, a...)
}

func (r *Request) error(err ...interface{}) {
	r.Call = false
	fmt.Println(err...)
}

func colorize(dump []byte) string {
	b := regHeader.ReplaceAllFunc(dump, func(src []byte) []byte {
		var buf bytes.Buffer
		k := color.New(color.FgGreen)
		v := color.New(color.FgCyan)
		sub := regHeader.FindAllSubmatch(src, -1)
		k.Fprint(&buf, string(sub[0][1]))
		v.Fprint(&buf, string(sub[0][2]))
		return buf.Bytes()
	})
	b = regStatus.ReplaceAllFunc(b, func(src []byte) []byte {
		var buf bytes.Buffer
		m := regStatus.FindAllSubmatch(src, -1)
		s := string(m[0][3])
		buf.Write(m[0][1])
		buf.WriteString(" ")
		var c *color.Color
		switch s {
		case "2":
			c = color.New(color.FgGreen)
		case "3":
			c = color.New(color.FgYellow)
		case "4":
			c = color.New(color.FgBlue)
		case "5":
			c = color.New(color.FgHiRed)
		}
		c.Fprint(&buf, string(m[0][2]))
		return buf.Bytes()
	})
	return string(b)
}

func (r *Request) dumpRequest() {
	httpReq, err := r.newHTTPRequest()
	if err != nil {
		fmt.Println(err)
		return
	}
	dump, _ := httputil.DumpRequestOut(httpReq, true)
	fmt.Println(colorize(dump))
	fmt.Println("")
}
