package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	//"io/ioutil"
	"strings"

	"github.com/getlantern/cloudflare"
	"github.com/getlantern/flashlight/proxy"
)

const (
	CF_DOMAIN = "getiantem.org"
	STATUS_GATEWAY_TIMEOUT = 504
)

type Reg struct {
	Name string
	Ip   string
	Port int
}

func register(w http.ResponseWriter, request *http.Request) {
	reg, err := requestToReg(request)
	if err != nil {
		log.Println("Error converting request ", err)
	} else {
		// We make a flashlight callback directly to
		// the peer. If that works, then we register
		// it in DNS. Our periodic worker process
		// will find it there and will test it again
		// end-to-end with the DNS provider before
		// entering it into the round robin.
		if callbackToPeer(reg.Ip) {
			go func() {
				//if (reg.Ip == "23.243.192.92") {
				log.Println("Registering peer: ", reg.Ip)	
				registerPeer(reg)
				//}
			}()
		} else {
			w.WriteHeader(STATUS_GATEWAY_TIMEOUT)
		}
	}
}

func unregister(w http.ResponseWriter, r *http.Request) {
	reg, err := requestToReg(r)
	if err != nil {
		fmt.Println("Error converting request ", err)
	} else {
		removeFromDns(reg)
	}
}

func removeFromDns(reg *Reg) {

	client, err := cloudflare.NewClient("", "")
	if err != nil {
		log.Println("Could not create CloudFlare client:", err)
		return
	}

	rec, err := client.RetrieveRecordByName(CF_DOMAIN, reg.Name)
	if err != nil {
		log.Println("Error retrieving record! ", err)
		return
	}

	// Make sure we destroy the record on CloudFlare if it
	// didn't work.
	log.Println("Destroying record for: ", reg.Name)
	err = client.DestroyRecord(CF_DOMAIN, rec.Id)
	if err != nil {
		log.Println("Error deleting peer record! ", err)
	} else {
		log.Println("Removed DNS record for ", reg.Ip)
	}
}

func callbackToPeer(upstreamHost string) bool {
	flashlightClient := &proxy.Client{
		UpstreamHost:       upstreamHost,
		UpstreamPort:       443,
		InsecureSkipVerify: true,
	}

	flashlightClient.BuildEnproxyConfig()

	client := &http.Client{
		Transport: &http.Transport{
			Dial: flashlightClient.DialWithEnproxy,
		},
	}

	resp, err := client.Head("http://www.google.com/humans.txt")
	if err != nil {
		log.Println("Direct HEAD request failed for IP ", upstreamHost)
		return false
	} else {
		log.Println("Direct HEAD request succeeded ", upstreamHost)
		defer resp.Body.Close()
		return true
	}
}

func registerPeer(reg *Reg) (*cloudflare.Record, error) {
	client, err := cloudflare.NewClient("", "")
	if err != nil {
		log.Println("Could not create CloudFlare client:", err)
		return nil, err
	}

	cr := cloudflare.CreateRecord{Type: "A", Name: reg.Name, Content: reg.Ip}
	rec, err := client.CreateRecord(CF_DOMAIN, &cr)

	if err != nil {
		log.Println("Could not create record? ", err)
		return nil, err
	}

	log.Println("Successfully created record for: ", rec.FullName, rec.Id)

	// Note for some reason CloudFlare seems to ignore the TTL here.
	ur := cloudflare.UpdateRecord{Type: "A", Name: reg.Name, Content: reg.Ip, Ttl: "360", ServiceMode: "1"}

	err = client.UpdateRecord(CF_DOMAIN, rec.Id, &ur)

	if err != nil {
		log.Println("Could not update record? ", err)
		return nil, err
	}

	log.Println("Successfully updated record to use CloudFlare service mode")
	return rec, err
}

func requestToReg(r *http.Request) (*Reg, error) {
	name := r.FormValue("name")
	log.Println("Read name: ", name)
	ip := clientIpFor(r)
	portString := r.FormValue("port")

	port := 0
	if portString == "" {
		// Could be an unregister call
		port = 0
	} else {
		converted, err := strconv.Atoi(portString)
		if err != nil {
			// handle error
			fmt.Println(err)
			return nil, err
		}
		port = converted
	}

	// If they're actually reporting an IP (it's a register call), make
	// sure the port is 443
	if len(ip) > 0 && port != 443 {
		log.Println("Ignoring port not on 443")
		return nil, fmt.Errorf("Bad port: %d", port)
	}
	reg := &Reg{name, ip, port}

	return reg, nil
}

func clientIpFor(req *http.Request) string {
	// If we're running in production on Heroku, use the IP of the client.
	// Otherwise use whatever IP is passed to the API.
	if onHeroku() {
		// Client requested their info
		clientIp := req.Header.Get("X-Forwarded-For")
		if clientIp == "" {
			clientIp = strings.Split(req.RemoteAddr, ":")[0]
		}
		// clientIp may contain multiple ips, use the first
		ips := strings.Split(clientIp, ",")
		return strings.TrimSpace(ips[0])
	} else {
		return req.FormValue("ip")
	}
}

func main() {
	http.HandleFunc("/register", register)
	http.HandleFunc("/unregister", unregister)
	http.ListenAndServe(getPort(), nil)
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

func onHeroku() bool {
	var dyno = os.Getenv("DYNO")
	return dyno != ""
}
