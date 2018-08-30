package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	prompt "github.com/c-bata/go-prompt"
)

var (
	methodSuggestion = []prompt.Suggest{{Text: GET}, {Text: POST}, {Text: PUT}, {Text: DELETE}, {Text: PATCH}, {Text: HEAD}, {Text: OPTIONS}}
	emptySuggestion  = make([]prompt.Suggest, 0)
	suggestSet       = make(map[string]bool)
)

type Suggestion struct {
	suggest []prompt.Suggest
}

func (s *Suggestion) Len() int {
	return len(s.suggest)
}

func (s *Suggestion) Less(i, j int) bool {
	return s.suggest[i].Text < s.suggest[j].Text
}

func (s *Suggestion) Swap(i, j int) {
	s.suggest[i], s.suggest[j] = s.suggest[j], s.suggest[i]
}

func newSuggestion() *Suggestion {
	s := &Suggestion{}
	f, err := os.OpenFile(os.TempDir()+"httpgo_suggest", os.O_CREATE|os.O_RDONLY, os.ModePerm)
	for _, m := range methodSuggestion {
		s.AddSuggest(m.Text)
	}
	if err == nil {
		r := bufio.NewReader(f)
		for {
			l, e := r.ReadString('\n')
			if e == io.EOF {
				break
			}
			s.AddSuggest(strings.TrimSpace(l))
		}
		f.Close()
	}

	return s
}

func (s *Suggestion) AddSuggest(param string) {
	if _, ok := suggestSet[param]; !ok {
		suggestSet[param] = true
		s.suggest = append(s.suggest, prompt.Suggest{Text: param})
	}
}

func (s *Suggestion) Suggest(req *Request) []prompt.Suggest {
	sort.Sort(s)
	return s.suggest
}

func (s *Suggestion) Save() {
	f, err := os.OpenFile(os.TempDir()+"httpgo_suggest", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, s := range s.suggest {
		if inSlice(HTTPMethods, s.Text) {
			continue
		}
		f.WriteString(s.Text)
		f.WriteString("\n")
	}
	f.Close()
	fmt.Println("")
	fmt.Println("Save suggestion to", f.Name())
}
