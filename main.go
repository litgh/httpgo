package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/c-bata/go-prompt"
	jsoniter "github.com/json-iterator/go"
	"github.com/tidwall/gjson"
)

var (
	json    = jsoniter.ConfigCompatibleWithStandardLibrary
	req     = newReq()
	scheme  = "http"
	suggest = newSuggestion()
)

var LivePrefixState struct {
	LivePrefix string
	IsEnable   bool
}

func main() {
	p := prompt.New(
		executor,
		func(in prompt.Document) []prompt.Suggest {
			if in.GetWordBeforeCursor() == "" {
				return []prompt.Suggest{}
			}
			return prompt.FilterContains(suggest.Suggest(req), in.GetWordBeforeCursor(), true)
		},
		prompt.OptionLivePrefix(func() (string, bool) {
			return LivePrefixState.LivePrefix, LivePrefixState.IsEnable
		}),
		prompt.OptionTitle("Httpgo"),
		prompt.OptionAddKeyBind(bindReset(), bindDoRequest(), bindQuit()),
		prompt.OptionPrefixTextColor(prompt.Blue),
	)
	fmt.Println("Welcome to the Httpgo!\nEnter 'h, help, ?' for help, Ctrl+D exit")
	usage()
	p.Run()
}

func executor(in string) {
	in = strings.TrimSpace(in)
	switch in {
	case "h", "help", "?":
		usage()
	case "p":
		dump()
	default:
		var tokenizer Tokenizer
		tokenizer.Init(in)
		var tok Token
	loop:
		for {
			tok = tokenizer.Next()
			switch tok.Type {
			case String:
				// 脚本
				if strings.HasPrefix(tok.Val, "!") {
					content := readFile(tok.Val[1:])
					if len(content) == 0 {
						continue loop
					}
					fmt.Printf("> Load file `%s`\n", tok.Val[1:])
					rd := bufio.NewReader(bytes.NewReader(content))
					var command []string
					for {
						l, err := rd.ReadString('\n')
						if err == io.EOF {
							break
						}
						l = strings.TrimSpace(l)
						if l != "" {
							fmt.Println(">", l)
							command = append(command, l)
						}
					}
					if len(command) > 0 {
						tokenizer.Init(strings.Join(command, " "))
						goto loop
					}
				} else if inSlice(HTTPMethods, tok.Val) {
					req.Method = strings.ToUpper(tok.Val)
				} else if strings.HasPrefix(tok.Val, "[") && strings.HasSuffix(tok.Val, "]") {
					if len(req.ResponseBody) == 0 {
						continue loop
					}
					jsonPath := tok.Val[1 : len(tok.Val)-1]
					v := gjson.GetBytes(req.ResponseBody, jsonPath)
					b, _ := json.MarshalIndent(v.Value(), "", " ")
					fmt.Println("json:", jsonPath, string(b))
					suggest.AddSuggest(tok.Val)
				} else {
					var URL = tok.Val
					var err error
					if req.URL != nil && strings.HasPrefix(URL, "/") {
						req.URL, err = req.URL.Parse(URL)
						if err != nil {
							req.errorf("parse `%s` %v\n", URL, err)
							return
						}
					} else if strings.HasPrefix(URL, ":") || strings.HasPrefix(URL, "/") {
						req.URL, err = url.Parse(scheme + "://localhost" + URL)
					} else if !strings.HasPrefix(URL, "http://") && !strings.HasPrefix(URL, "https://") {
						req.URL, err = url.Parse(scheme + "://" + URL)
					} else {
						req.URL, err = url.Parse(URL)
						scheme = req.URL.Scheme
					}

					if err != nil {
						req.error(err)
						return
					}

					if req.Method == "" {
						req.Method = GET
					}
					suggest.AddSuggest(req.URL.String())
				}
			case Header:
				req.Header.Set(tok.Key, tok.Val)
				suggest.AddSuggest(tok.Key)
				suggest.AddSuggest(tok.Key + ":" + tok.Val)
			case Field:
				req.Fields.Set(tok.Key, tok.Val)
				suggest.AddSuggest(tok.Key)
				suggest.AddSuggest(tok.Key + "=" + tok.Val)
			case Param:
				req.Values.Set(tok.Key, tok.Val)
				suggest.AddSuggest(tok.Key)
				suggest.AddSuggest(tok.Key + "==" + tok.Val)
			case RawJSON:
				rawJSON(tok.Key, tok.Val)
				suggest.AddSuggest(tok.Key)
				suggest.AddSuggest(tok.Key + "=:" + tok.Val)

				if req.Method == GET {
					req.Method = POST
				}
			case Flag:
				flag(tok.Key, tok.Val)
				suggest.AddSuggest(tok.Key)
				suggest.AddSuggest(tok.Key + "=" + tok.Val)
			case File:
				if tok.Key == "" {
					req.Body = readFile(tok.Val)
					suggest.AddSuggest("@" + tok.Val)
				} else {
					req.Files.Add(tok.Key, tok.Val)
					suggest.AddSuggest(tok.Key)
					suggest.AddSuggest("@" + tok.Val)
				}

				if req.Method == GET {
					req.Method = POST
				}
			case EOF:
				break loop
			}
		}

		LivePrefixState.IsEnable = true
		if req.URL != nil {
			LivePrefixState.LivePrefix = req.Method + " " + req.URL.String() + " > "
		} else if req.Method != "" {
			LivePrefixState.LivePrefix = req.Method + " > "
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

func dump() {
	httpReq, err := req.newHTTPRequest()
	if err != nil {
		fmt.Println(err)
		return
	}
	dump, _ := httputil.DumpRequestOut(httpReq, true)
	fmt.Println(colorize(dump))
	fmt.Println("")
}

func inSlice(slice []string, value string) bool {
	for i := range slice {
		if slice[i] == strings.ToUpper(value) {
			return true
		}
	}
	return false
}

func bindQuit() prompt.KeyBind {
	return prompt.KeyBind{Key: prompt.ControlB, Fn: func(buf *prompt.Buffer) {
		suggest.Save()
	}}
}

func bindReset() prompt.KeyBind {
	return prompt.KeyBind{Key: prompt.ControlC, Fn: func(buf *prompt.Buffer) {
		LivePrefixState.IsEnable = false
		r := newReq()
		r.Proxy = req.Proxy
		req = r
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
	if int64(req.Timeout) == 0 {
		req.Timeout = time.Second * 30
	}
	client := http.Client{Timeout: req.Timeout}
	if req.Proxy != "" {
		proxyURL, err := url.Parse(req.Proxy)
		if err != nil {
			fmt.Println(err)
			return
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client.Transport = transport
	}

	r, err := req.newHTTPRequest()
	if err != nil {
		fmt.Println(err)
		return
	}
	out, _ := httputil.DumpRequest(r, true)
	fmt.Printf("\n%s\n", colorize(out))
	resp, err := client.Do(r)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	out, _ = httputil.DumpResponse(resp, true)
	fmt.Printf("\n%s\n", colorize(out))
	req.ResponseBody, _ = ioutil.ReadAll(resp.Body)
}

func rawJSON(key, value string) {
	var j interface{}
	var err error
	if strings.HasPrefix(value, "@") {
		content := readFile(value[1:])
		err = json.Unmarshal(content, &j)
		if err != nil {
			req.error("Read from file", value[1:], "unmarshal", err)
			return
		}
	} else {
		err = json.UnmarshalFromString(value, &j)
		if err != nil {
			req.error("Unmarshal", "`"+value+"`", err)
			return
		}
	}
	req.Method = POST
	req.JSONMap[key] = append(req.JSONMap[key], j)
}

func readFile(filename string) []byte {
	f, err := os.Open(filename)
	if err != nil {
		req.error("Read file", filename, err)
		return nil
	}
	defer f.Close()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		req.error("ReadAll from file", filename, err)
		return nil
	}
	return content
}

func flag(key, value string) {
	switch key {
	case "$auth":
		pair := strings.Split(value, ":")
		if len(pair) > 1 {
			req.Username = pair[0]
			req.Password = strings.Join(pair[1:], "")
		}
	case "$json":
		req.JSON = true
		req.Form = false
	case "$form":
		req.JSON = false
		req.Form = true
	case "$scheme":
		scheme = value
	case "$proxy":
		req.Proxy = value
	case "$timeout":
		d, err := time.ParseDuration(value)
		if err == nil {
			req.Timeout = d
		} else {
			req.errorf("$timeout=%s %v/n", value, err)
		}
	case "$bench":
		pair := strings.Split(value, ",")
		if len(pair) != 2 {
			req.errorf("$bench={concurrency},{total_request or duration}, eg: $bench=10,10s $bench=10,1000")
			return
		}
		c, err := strconv.ParseUint(pair[0], 10, 64)
		if err == nil {
			req.Bench = true
			req.Concurrency = c
		} else {
			req.errorf("$bench=%s %v\n", value, err)
			return
		}
		if pair[1] != "" && !unicode.IsDigit(rune(pair[1][len(pair[1])-1])) {
			d, err := time.ParseDuration(pair[1])
			if err == nil {
				req.Bench = true
				req.Duration = d
			} else {
				req.errorf("$bench=%s %v\n", value, err)
			}
		} else if pair[1] != "" {
			n, err := strconv.ParseUint(pair[0], 10, 64)
			if err == nil {
				req.Bench = true
				req.NumberOfRequest = n
			} else {
				req.errorf("$bench=%s %v\n", value, err)
			}
		}

	default:
		req.error("unknown", key)
	}
}
