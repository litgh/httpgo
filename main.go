package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/c-bata/go-prompt"
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
)

// Request is the http request
type Request struct {
	Method string
	URL    string
	Header http.Header
	Values url.Values
	Files  url.Values
	JSON   map[string][]interface{}
	Body   []byte
}

var LivePrefixState struct {
	LivePrefix string
	IsEnable   bool
}

func changeLivePrefix() (string, bool) {
	return LivePrefixState.LivePrefix, LivePrefixState.IsEnable
}

func executor(in string) {
	in = strings.TrimSpace(in)
	switch in {
	case "h", "help", "?":
		usage()
	case "p":
		httpReq := req.newHTTPRequest()
		dump, _ := httputil.DumpRequestOut(httpReq, true)
		fmt.Println(string(dump))
	default:
		tokens := strings.Fields(in)
		for i := 0; i < len(tokens); {
			token := tokens[i]
			call = strings.HasSuffix(token, ";")
			if call {
				token = strings.TrimSuffix(token, ";")
			}

			if strings.HasPrefix(token, ":") {
				req.URL = scheme + "://localhost" + token
			}

			if strings.HasPrefix(token, scheme+"://") {
				req.URL = token
			}

			if inSlice(HTTPMethods, token) {
				req.Method = strings.ToUpper(token)
			} else if strings.Contains(token, "==") {
				pair := strings.Split(in, "==")
				if len(pair) == 2 {
					req.Values.Add(pair[0], strings.TrimSpace(pair[1]))
				}
				suggest.AddSuggest(pair[0])
			} else if strings.Contains(token, ":=") {
				rawJSON(token)
			} else if strings.HasSuffix(token, ":") {
				req.Header.Add(token, tokens[i+1])
				suggest.AddSuggest(token)
				i++
			} else if strings.Contains(token, ":") {
				pair := strings.Split(in, ":")
				if len(pair) == 2 {
					req.Header.Add(pair[0], pair[1])
				}
			} else if token != "" {
				if !strings.HasPrefix(token, scheme+"://") {
					req.URL = scheme + "://" + token
				}
				suggest.AddSuggest(token)
			}
			if call {
				do()
			}
			i++

			LivePrefixState.IsEnable = true
			if req.URL != "" {
				LivePrefixState.LivePrefix = req.Method + " " + req.URL + " > "
			} else {
				LivePrefixState.LivePrefix = req.Method + " > "
			}
		}
	}

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
		do()
	}}
}

func do() {
	client := http.Client{Timeout: time.Second * 10}
	r := req.newHTTPRequest()
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
	return &Request{Header: make(http.Header), Values: make(url.Values), Files: make(url.Values), JSON: make(map[string][]interface{})}
}

func rawJSON(value string) {
	pair := strings.Split(value, ":=")
	if len(pair) != 2 {
		return
	}
	var j interface{}
	if strings.HasPrefix(pair[1], "@") {
		filename := pair[1][1:]
		f, err := os.Open(pair[1][1:])
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
		err := json.UnmarshalFromString(pair[1], &j)
		if err != nil {
			fmt.Println("Unmarshal", "`"+pair[1]+"`", err)
			call = false
			return
		}
	}
	req.Method = POST
	req.JSON[pair[0]] = append(req.JSON[pair[0]], j)
}

func (r *Request) newHTTPRequest() (httpReq *http.Request) {
	if len(r.Values) != 0 {
		r.URL += "?" + r.Values.Encode()
	}
	if r.Method == POST || r.Method == PUT || r.Method == PATCH {
		var body io.Reader
		if len(r.Body) > 0 {
			body = bytes.NewReader(r.Body)
		} else if len(r.JSON) > 0 {
			r.Header.Set("Content-Type", "application/json; charset=UTF-8")
			r.Header.Set("Accept", "application/json")
			js := make(map[string]interface{})
			for k, v := range r.JSON {
				if len(v) == 1 {
					js[k] = v[0]
				} else if len(v) > 1 {
					js[k] = v
				}
			}
			b, _ := json.Marshal(js)
			body = bytes.NewReader(b)
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
				for k, v := range r.Values {
					bodyWriter.WriteField(k, strings.Join(v, ","))
				}
				bodyWriter.Close()
				pipeWriter.Close()
			}()
			r.Header.Set("Content-Type", bodyWriter.FormDataContentType())
			body = ioutil.NopCloser(pipeReader)
		} else if len(r.Values) > 0 {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			body = strings.NewReader(r.Values.Encode())
		}
		httpReq, _ = http.NewRequest(r.Method, r.URL, body)
	}
	if r.Method == GET || r.Method == DELETE || r.Method == OPTIONS {
		httpReq, _ = http.NewRequest(r.Method, r.URL, nil)
	}
	httpReq.Header = r.Header
	return
}
