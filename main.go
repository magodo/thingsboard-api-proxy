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

func handleFunc(turl *url.URL) func(res http.ResponseWriter, req *http.Request) {
	return func(res http.ResponseWriter, req *http.Request) {
		proxy := httputil.NewSingleHostReverseProxy(turl)
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
				res.Write([]byte(fmt.Sprintf("failed to unmarshal the resource model for `id` field: %v", err)))
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
		case http.MethodPost:
			if req.URL.Path != "/api/auth/login" {
				proxy.ServeHTTP(res, req)
				return
			}
			// Turn the in param form (https://datatracker.ietf.org/doc/html/rfc6749#section-4.3.2) to in body form as is expected by thingsboard API.
			body, err := io.ReadAll(req.Body)
			if err != nil {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to read the request body: %v", err)))
				return
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to parse the request body as url encoded value: %v", err)))
				return
			}
			var username, password string
			if l, ok := values["username"]; ok && len(l) == 1 {
				username = l[0]
			}
			if l, ok := values["password"]; ok && len(l) == 1 {
				password = l[0]
			}
			if username == "" {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to identify `username` in the request")))
				return
			}
			if password == "" {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to identify `password` in the request")))
				return
			}

			resrec := httptest.ResponseRecorder{
				Body: bytes.NewBuffer([]byte{}),
			}
			nreq, _ := http.NewRequestWithContext(req.Context(), http.MethodPost, req.URL.String(), bytes.NewBuffer([]byte(fmt.Sprintf(`{"username": %q, "password": %q}`, username, password))))
			nreq.Header.Add("Content-Type", "application/json")
			proxy.ServeHTTP(&resrec, nreq)

			if resrec.Code/100 != 2 {
				res.WriteHeader(resrec.Code)
				res.Write(resrec.Body.Bytes())
				return
			}

			type token struct {
				Token        string `json:"token"`
				RefreshToken string `json:"refreshToken"`
			}
			var tk token
			if err := json.Unmarshal(resrec.Body.Bytes(), &tk); err != nil {
				res.WriteHeader(400)
				res.Write([]byte(fmt.Sprintf("failed to unmarshal the token: %v", err)))
				return
			}
			m := map[string]interface{}{}
			m["access_token"] = tk.Token
			m["refresh_token"] = tk.RefreshToken
			b, _ := json.Marshal(m)
			res.Header()["Content-Type"] = []string{"application/json"}
			res.Write(b)
		default:
			proxy.ServeHTTP(res, req)
		}
	}
}
