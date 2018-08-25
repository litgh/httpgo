package main

import prompt "github.com/c-bata/go-prompt"

var (
	methodSuggestion = []prompt.Suggest{{Text: GET}, {Text: POST}, {Text: PUT}, {Text: DELETE}, {Text: PATCH}, {Text: HEAD}, {Text: OPTIONS}}
	emptySuggestion  = make([]prompt.Suggest, 0)
	suggestSet       = make(map[string]bool)
)

type Suggestion struct {
	methods []prompt.Suggest
	suggest []prompt.Suggest
}

func newSuggestion() *Suggestion {
	return &Suggestion{methods: methodSuggestion}
}

func (s *Suggestion) AddSuggest(param string) {
	if _, ok := suggestSet[param]; !ok {
		suggestSet[param] = true
		s.suggest = append(s.suggest, prompt.Suggest{Text: param})
	}
}

func (s *Suggestion) Suggest(req *Request) []prompt.Suggest {
	if req.Method == "" {
		return s.methods
	}
	return s.suggest
}
