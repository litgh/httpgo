package main

import (
	"fmt"
	"testing"
)

func TestString(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`get`)

	token := tokenizer.Next()
	if token.Type != String {
		t.Errorf("expected %v, actual %v", String, token.Type)
	}
}

func TestHeader(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`a:b`)

	token := tokenizer.Next()
	if token.Type != Header {
		t.Errorf("expected %v, actual %v", Header, token.Type)
	}
}

func TestField(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`a=b`)

	token := tokenizer.Next()
	if token.Type != Field {
		t.Errorf("expected %v, actual %v", Field, token.Type)
	}
}

func TestFile(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`a@/b/c/d.txt`)

	token := tokenizer.Next()
	if token.Type != File {
		t.Errorf("expected %v, actual %v", File, token.Type)
	}
}

func TestParam(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`a==b`)

	token := tokenizer.Next()
	if token.Type != Param {
		t.Errorf("expected %v, actual %v", Param, token.Type)
	}
}

func TestRawJSON(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`a=:{"foo":"bar"}`)

	token := tokenizer.Next()
	if token.Type != RawJSON {
		t.Errorf("expected %v, actual %v", RawJSON, token.Type)
	}
}

func TestRawJSONFile(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`a=:@/a/b/c.json`)

	token := tokenizer.Next()
	if token.Type != RawJSON {
		t.Errorf("expected %v, actual %v", RawJSON, token.Type)
	}
}

func TestFlag(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init("$a=b")

	token := tokenizer.Next()
	if token.Type != Flag {
		t.Errorf("expected %v, actual %v", RawJSON, token.Type)
	}
	if token.Key != "$a" {
		t.Errorf("expected %s, actual %s", "$a", token.Key)
	}
	if token.Val != "b" {
		t.Errorf("expected %s, actual %s", "b", token.Key)
	}
}

func TestInput(t *testing.T) {
	var tokenizer Tokenizer
	tokenizer.Init(`get http://baidu.com a:b c==d e=f g=:@/path/to/file h=:{"foo":"bar"} h=:["","",""] i@/path/to/j.txt $a=b`)

	typs := []rune{String, String, Header, Param, Field, RawJSON, RawJSON, RawJSON, File, Flag}
	token := tokenizer.Next()
	for idx, tt := range typs {
		fmt.Println(token.Type, token.Key, token.Val)
		if token.Type != tt {
			t.Errorf("%d => %s %s expected %v, actual %v", idx, token.Key, token.Val, tt, token.Type)
		}
		token = tokenizer.Next()
	}
}
