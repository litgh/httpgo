package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/fatih/color"
	"github.com/json-iterator/go"
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
	json = jsoniter.ConfigCompatibleWithStandardLibrary
	//HTTPMethods is this http methods list
	HTTPMethods = []string{GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS}
	// Current http request
	req     = newReq()
	scheme  = "http"
	suggest = newSuggestion()
	call    = false

	regHeader = regexp.MustCompile(`(\w+\s*:)(.+)`)
)

// Request is the http request
type Request struct {
	Method          string
	URL             string
	Username        string
	Password        string
	JSON            bool
	Form            bool
	Bench           bool
	NumberOfRequest uint64
	Concurrency     uint64
	Duration        time.Duration
	Timeout         time.Duration
	Header          http.Header
	Values          url.Values
	Fields          url.Values
	Files           url.Values
	JSONMap         map[string][]interface{}
	Body            []byte
}

func executor(in string) {
	in = strings.TrimSpace(in)
	switch in {
	case "h", "help", "?":
		usage()
	case "p":
		httpReq, _ := req.newHTTPRequest()
		dump, _ := httputil.DumpRequestOut(httpReq, true)
		dump = regHeader.ReplaceAllFunc(dump, func(src []byte) []byte {
			var buf bytes.Buffer
			c := color.New(color.FgCyan)
			sub := regHeader.FindAllSubmatch(src, -1)
			buf.Write(sub[0][1])
			c.Fprint(&buf, string(sub[0][2]))
			return buf.Bytes()
		})
		fmt.Println(string(dump))
	default:
		var tokenizer Tokenizer
		tokenizer.Init(in)
	loop:
		for {
			tok := tokenizer.Next()
			switch tok.Type {
			case String:
				if inSlice(HTTPMethods, tok.Val) {
					req.Method = tok.Val
				} else {
					req.URL = tok.Val
					if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
						req.URL = scheme + "://" + req.URL
					} else if strings.HasPrefix(req.URL, ":") {
						req.URL = scheme + "://localhost" + req.URL
					}
					if req.Method == "" {
						req.Method = GET
					}
				}
			case Header:
				req.Header.Set(tok.Key, tok.Val)
			case Field:
				req.Fields.Add(tok.Key, tok.Val)
			case Param:
				req.Values.Add(tok.Key, tok.Val)
			case RawJSON:
				rawJSON(tok.Key, tok.Val)
			case Flag:
				flag(tok.Key, tok.Val)
			case EOF:
				break loop
			}
		}

		LivePrefixState.IsEnable = true
		if req.URL != "" {
			LivePrefixState.LivePrefix = req.Method + " " + req.URL + " > "
		} else {
			LivePrefixState.LivePrefix = req.Method + " > "
		}
		// }
	}

}

var LivePrefixState struct {
	LivePrefix string
	IsEnable   bool
}

func changeLivePrefix() (string, bool) {
	return LivePrefixState.LivePrefix, LivePrefixState.IsEnable
}

func usage() {
	fmt.Println(`
  p print current request info
  Ctrl + c reset current state
  Ctrl + r do request
	`)
}

func main() {
	p := prompt.New(
		executor,
		func(in prompt.Document) []prompt.Suggest {
			return prompt.FilterContains(suggest.Suggest(req), in.GetWordBeforeCursor(), true)
		},
		prompt.OptionLivePrefix(changeLivePrefix),
		prompt.OptionTitle("GO-http-prompt"),
		prompt.OptionAddKeyBind(bindReset(), bindDoRequest()),
	)
	fmt.Println("Welcome to the Go-http-Prompt!\nEnter 'h, help, ?' for help, Ctrl+D exit")
	usage()
	p.Run()
}

func inSlice(slice []string, value string) bool {
	for i := range slice {
		if slice[i] == strings.ToUpper(value) {
			return true
		}
	}
	return false
}

func bindReset() prompt.KeyBind {
	return prompt.KeyBind{Key: prompt.ControlC, Fn: func(buf *prompt.Buffer) {
		LivePrefixState.IsEnable = false
		req = newReq()
	}}
}

func bindDoRequest() prompt.KeyBind {
	return prompt.KeyBind{Key: prompt.ControlR, Fn: func(buf *prompt.Buffer) {
		if req.Bench {
			client := http.Client{Timeout: req.Timeout}
			r, err := req.newHTTPRequest()
			if err != nil {
				fmt.Println(err)
				return
			}
			c := make(chan struct{}, req.Concurrency)
			var wg sync.WaitGroup

			var do = func() {
				defer func() {
					<-c
					wg.Done()
				}()
				client.Do(r)
			}

			if int64(req.Duration) != 0 {
				ctx, cancel := context.WithTimeout(context.Background(), req.Duration)
				defer cancel()
			bench:
				for {
					select {
					case <-ctx.Done():
						break bench
					default:
						c <- struct{}{}
						wg.Add(1)
						go do()

					}
				}
			} else {
				if req.NumberOfRequest == 0 {
					req.NumberOfRequest = 1
				}
				for ; req.NumberOfRequest > 0; req.NumberOfRequest-- {
					c <- struct{}{}
					wg.Add(1)
					go do()
				}
			}
			wg.Wait()

		} else {
			httpCall()
		}
	}}
}

func httpCall() {
	client := http.Client{Timeout: time.Second * 10}
	r, err := req.newHTTPRequest()
	if err != nil {
		fmt.Println(err)
		return
	}
	out, _ := httputil.DumpRequest(r, true)
	fmt.Printf("%s\n", out)
	resp, err := client.Do(r)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	out, _ = httputil.DumpResponse(resp, true)
	fmt.Printf("%s\n", out)

	call = false
}

func newReq() *Request {
	return &Request{Header: make(http.Header), Values: make(url.Values), Files: make(url.Values), Fields: make(url.Values), JSON: true, JSONMap: make(map[string][]interface{})}
}

func rawJSON(key, value string) {
	var j interface{}
	if strings.HasPrefix(value, "@") {
		filename := value[1:]
		f, err := os.Open(filename)
		if err != nil {
			fmt.Println("Read file", filename, err)
			call = false
			return
		}
		content, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Println("ReadAll from file", filename, err)
			call = false
			return
		}
		err = json.Unmarshal(content, &j)
		if err != nil {
			fmt.Println("Read from file", filename, "unmarshal", err)
			call = false
			return
		}
	} else {
		err := json.UnmarshalFromString(value, &j)
		if err != nil {
			fmt.Println("Unmarshal", "`"+value+"`", err)
			call = false
			return
		}
	}
	req.Method = POST
	req.JSONMap[key] = append(req.JSONMap[key], j)
}

func flag(key, value string) {
	switch key {
	case "$auth":
		pair := strings.Split(value, ":")
		if len(pair) > 1 {
			req.Username = pair[0]
			req.Password = pair[1]
		}
	case "$json":
		req.JSON = true
		req.Form = false
	case "$form":
		req.JSON = false
		req.Form = true
	case "$scheme":
		scheme = value
	case "$timeout":
		d, err := time.ParseDuration(value)
		if err == nil {
			req.Timeout = d
		} else {
			fmt.Printf("$timeout=%s %v/n", value, err)
			call = false
		}
	case "$n":
		n, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			req.Bench = true
			req.NumberOfRequest = n
		} else {
			fmt.Printf("$n=%s %v/n", value, err)
			call = false
		}
	case "$d":
		d, err := time.ParseDuration(value)
		if err == nil {
			req.Bench = true
			req.Duration = d
		} else {
			fmt.Printf("$d=%s %v/n", value, err)
			call = false
		}
	case "$c":
		c, err := strconv.ParseUint(value, 10, 64)
		if err == nil {
			req.Bench = true
			req.Concurrency = c
		} else {
			fmt.Printf("$n=%s %v/n", value, err)
			call = false
		}
	default:
		fmt.Println("unknown", key)
		call = false
	}
}

func (r *Request) newHTTPRequest() (httpReq *http.Request, err error) {
	if len(r.Values) != 0 {
		r.URL += "?" + r.Values.Encode()
	}
	if r.Method == POST || r.Method == PUT || r.Method == PATCH {
		var body io.Reader
		if len(r.Body) > 0 {
			body = bytes.NewReader(r.Body)
		} else if len(r.JSONMap) > 0 {
			body = r.jsonBody()
		} else if len(r.Files) > 0 {
			pipeReader, pipeWriter := io.Pipe()
			bodyWriter := multipart.NewWriter(pipeWriter)
			go func() {
				for param, filename := range r.Files {
					for _, file := range filename {
						fileWriter, err := bodyWriter.CreateFormFile(param, file)
						if err != nil {
							log.Fatal(err)
						}
						f, err := os.Open(file)
						if err != nil {
							log.Fatal(err)
						}
						_, err = io.Copy(fileWriter, f)
						f.Close()
						if err != nil {
							log.Fatal(err)
						}
					}
				}
				for k, v := range r.Fields {
					bodyWriter.WriteField(k, strings.Join(v, ","))
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
				body = strings.NewReader(r.Values.Encode())
			}
		}
		httpReq, err = http.NewRequest(r.Method, r.URL, body)
		if err != nil {
			return
		}
	}
	if r.Method == GET || r.Method == DELETE || r.Method == OPTIONS {
		httpReq, _ = http.NewRequest(r.Method, r.URL, nil)
	}
	if r.Username != "" {
		httpReq.SetBasicAuth(r.Username, r.Password)
	}
	httpReq.Header = r.Header
	return
}

func (r *Request) jsonBody() io.Reader {
	r.Header.Set("Content-Type", "application/json; charset=UTF-8")
	r.Header.Set("Accept", "application/json")
	js := make(map[string]interface{})

	if len(r.Fields) > 0 {
		for k, v := range r.Fields {
			for _, x := range v {
				r.JSONMap[k] = append(r.JSONMap[k], x)
			}
		}
		r.Fields = make(url.Values)
	}

	for k, v := range r.JSONMap {
		if len(v) == 1 {
			js[k] = v[0]
		} else if len(v) > 1 {
			js[k] = v
		}
	}
	b, _ := json.Marshal(js)
	return bytes.NewReader(b)
}
