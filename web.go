// main simply contains the primary web serving code that allows peers to
// register and unregister as give mode peers running within the Lantern
// network
package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	XForwardedFor = "X-Forwarded-For"
)

func main() {
	http.HandleFunc("/register", register)
	http.HandleFunc("/unregister", unregister)
	err := http.ListenAndServe(getPort(), nil)
	if err != nil {
		log.Fatal(err.Error())
	}
}

// register is the entry point for peers registering themselves with the service.
// If peers are successfully vetted, they'll be added to the DNS round robin.
func register(w http.ResponseWriter, request *http.Request) {
	vals := url.Values{
		"name": []string{request.FormValue("name")},
		"port": []string{request.FormValue("port")},
	}
	forward(w, request, fmt.Sprintf("https://peerscanner.getiantem.org/register?%s", vals.Encode()))
}

// unregister is the HTTP endpoint for removing peers from DNS.
func unregister(w http.ResponseWriter, request *http.Request) {
	forward(w, request, "https://peerscanner.getiantem.org/unregister")
}

func forward(w http.ResponseWriter, request *http.Request, dest string) {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 60 * time.Second,
	}
	req, err := http.NewRequest("POST", dest, nil)
	if err != nil {
		log.Printf("Unexpected error creating POST request: %s", err)
		w.WriteHeader(500)
		return
	}
	req.Header.Set(XForwardedFor, clientIpFor(request))
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Unexpected error forwarding request: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func clientIpFor(req *http.Request) string {
	// Client requested their info
	clientIp := req.Header.Get(XForwardedFor)
	if clientIp == "" {
		clientIp = strings.Split(req.RemoteAddr, ":")[0]
	}
	// clientIp may contain multiple ips, use the first
	ips := strings.Split(clientIp, ",")
	return strings.TrimSpace(ips[0])
}

// Get the Port from the environment so we can run on Heroku
func getPort() string {
	var port = os.Getenv("PORT")
	// Set a default port if there is nothing in the environment
	if port == "" {
		port = "7777"
		fmt.Println("INFO: No PORT environment variable detected, defaulting to " + port)
	}
	return ":" + port
}
