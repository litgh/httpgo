package main

import (
	"bytes"
	"strings"
	"text/scanner"
)

const (
	String = iota
	Flag
	Header
	Field
	Param
	File
	RawJSON
)

var EOF rune = scanner.EOF

type Token struct {
	Type rune
	Key  string
	Val  string
}

type Tokenizer struct {
	s      scanner.Scanner
	tokBuf bytes.Buffer
	err    string
}

func (t *Tokenizer) Init(str string) {
	t.s.Init(strings.NewReader(str))
}

func (t *Tokenizer) Next() Token {
	t.tokBuf.Reset()

	ch := t.s.Next()
	for isWhitespace(ch) {
		if t.tokBuf.Len() > 0 {
			return Token{Type: String, Val: t.tokBuf.String()}
		}
		ch = t.s.Next()
	}

	for ch != scanner.EOF {
		switch ch {
		case ' ':
			return Token{Type: String, Val: t.tokBuf.String()}
		case '$':
			t.tokBuf.WriteRune(ch)
			t.scanNext('=')
			return Token{Type: Flag, Key: t.Token(), Val: t.scanNext(' ')}
		case ':':
			s := t.tokBuf.String()
			if s == "http" || s == "https" || s == "" || strings.Contains(s, ".") {
				t.tokBuf.WriteRune(ch)
				return Token{Type: String, Val: t.scanNext(' ')}
			}
			return Token{Type: Header, Key: t.Token(), Val: t.scanNext(' ')}
		case '=':
			var token Token
			ch = t.s.Next()
			switch ch {
			case '=':
				token = Token{Type: Param, Key: t.Token(), Val: t.scanNext(' ')}
			case ':':
				token = Token{Type: RawJSON, Key: t.Token(), Val: t.scanNext(' ')}
			default:
				token = Token{Type: Field, Key: t.Token()}
				t.tokBuf.WriteRune(ch)
				token.Val = t.scanNext(' ')
			}

			return token
		case '@':
			return Token{Type: File, Key: t.Token(), Val: t.scanNext(' ')}
		case '\'', '"':
			return Token{Type: String, Val: t.scanNext(ch)}
		default:
			t.tokBuf.WriteRune(ch)
			ch = t.s.Next()
		}
	}

	if t.tokBuf.Len() > 0 {
		return Token{Type: String, Val: t.Token()}
	}
	return Token{Type: EOF}
}

func (t *Tokenizer) scanNext(c rune) string {
	ch := t.s.Next()
	for isWhitespace(ch) && t.tokBuf.Len() == 0 {
		ch = t.s.Next()
	}
	for ch != c {
		if ch == scanner.EOF {
			break
		}
		t.tokBuf.WriteRune(ch)
		ch = t.s.Next()
	}
	return t.tokBuf.String()
}

func (t *Tokenizer) Token() string {
	str := t.tokBuf.String()
	t.tokBuf.Reset()
	return str
}

func (t *Tokenizer) HasError() bool {
	return t.s.ErrorCount > 0
}

func isWhitespace(ch rune) bool {
	return ch == '\t' || ch == '\n' || ch == '\r' || ch == ' '
}
