package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type ProxyHandler struct {
	db *sql.DB
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	curTime := time.Now().Unix()
	uri, _ := url.Parse(r.URL.Path)
	method := strings.ToUpper(r.Method)
	userID := r.Header.Get("X-LX-UserId")

	var appKey, appSecret string
	row := p.db.QueryRow("select app_access_key, app_access_secret from users where id = ?", userID)
	row.Scan(&appKey, &appSecret)
	fmt.Println("db =>", userID, appKey, appSecret)
	if appKey == "" {
		appKey, appSecret = "s1jg2ddvqsl3", "64B61D33A750BC0F6BDA5B258791AA69"
	}

	strToSign := fmt.Sprintf("%s\n%d\n%s", method, curTime, uri.RequestURI())
	mac := hmac.New(sha1.New, []byte(appSecret))
	mac.Write([]byte(strToSign))

	encode := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	r.Header.Add("Time", fmt.Sprintf("%d", curTime))
	r.Header.Add("Authorization", fmt.Sprintf("LIANXIN %s:%s", appKey, encode))
	dump, _ := httputil.DumpRequest(r, false)
	fmt.Println(strToSign)
	fmt.Println(string(dump))

	transport := http.DefaultTransport
	outReq := new(http.Request)
	*outReq = *r

	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	for key, value := range resp.Header {
		for _, v := range value {
			w.Header().Add(key, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	resp.Body.Close()
}

func main() {
	db, err := sql.Open("mysql", "root:Lianxin2017@@tcp(172.16.0.240:3306)/lx_loan?autocommit=true")
	//db, err := sql.Open("mysql", "lx_loan:v9N92FmB2Ky3uxZ4@tcp(127.0.0.1:4040)/lx_loan?autocommit=true")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	p := &ProxyHandler{db: db}
	fmt.Println("Listen :9088")
	http.ListenAndServe(":9088", p)
}
