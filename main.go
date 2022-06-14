package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path"
)

func main() {
	laddr := flag.String("laddr", "0.0.0.0:12345", "address to listen")
	taddr := flag.String("taddr", "", "thingsboard api address")
	flag.Parse()

	turl, err := url.Parse(*taddr)
	if err != nil {
		log.Fatalf("parsing thingsboard api address %q: %v", *taddr, err)
	}
	http.HandleFunc("/", handleFunc(turl))
	if err := http.ListenAndServe(*laddr, nil); err != nil {
		log.Fatal(err)
	}
}

func handleFunc(url *url.URL) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		proxy := httputil.NewSingleHostReverseProxy(url)
		switch req.Method {
		case http.MethodPut:
			// Convert a PUT request for updating to the expected POST request, with the `id` added up.
			obody := req.Body
			req.Body = nil
			req.ContentLength = 0
			req.Method = http.MethodGet
			resrec := httptest.ResponseRecorder{
				Body: bytes.NewBuffer([]byte{}),
			}
			proxy.ServeHTTP(&resrec, req)
			if resrec.Code/100 != 2 {
				res.WriteHeader(resrec.Code)
				res.Write(resrec.Body.Bytes())
				return
			}

			type object struct {
				Id map[string]interface{} `json:"id"`
			}
			var obj object
			if err := json.Unmarshal(resrec.Body.Bytes(), &obj); err != nil {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to parse the resource model for `id` field: %v", err)))
				return
			}

			var m map[string]interface{}
			body, err := io.ReadAll(obody)
			if err != nil {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to read the request body: %v", err)))
				return
			}
			if err := json.Unmarshal(body, &m); err != nil {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to unmarshal the request body as a map: %v", err)))
				return
			}
			m["id"] = obj.Id
			body, _ = json.Marshal(m)
			req.Body = io.NopCloser(bytes.NewBuffer(body))
			req.ContentLength = int64(len(body))
			req.URL.Path = path.Dir(req.URL.Path)
			req.Method = http.MethodPost
			proxy.ServeHTTP(res, req)
		default:
			proxy.ServeHTTP(res, req)
		}
	}
}
