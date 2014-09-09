package main

import (
    //"common"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	//"sync"
	"strings"
	"github.com/getlantern/cloudflare"
	"time"

	"github.com/getlantern/peerscanner/common"
	"github.com/getlantern/flashlight/client"
)

const (
	CF_DOMAIN     = "getiantem.org"
	ROUNDROBIN    = "roundrobin"
	PEERS         = "peers"
	FALLBACKS     = "fallbacks"
)

func main() {
	log.Println("Starting CloudFlare Flashlight Tests...")
	client, err := cloudflare.NewClient("", "")
	if err != nil {
		log.Println("Could not create CloudFlare client:", err)
		return
	}

	for {
		log.Println("Starting pass!")
		loopThroughRecords(client)
	}

}

func loopThroughRecords(client *cloudflare.Client) {
	records, err := common.GetAllRecords(client)
	if err != nil {
		log.Println("Error retrieving record!", err)
		return
	}
	log.Println("Loaded all records...", records.Response.Recs.Count)

	// Sleep here instead to make sure records have propagated to CloudFlare internally.
	log.Println("Sleeping!")
	time.Sleep(10 * time.Second)

	recs := records.Response.Recs.Records

	log.Println("Total records loaded: ", len(recs))

	// Loop through everything to do some bookkeeping and to put
	// records in their appropriate categories.

	// All peers.
	peers := make([]cloudflare.Record, 0)

	// All entries in the round robin.
	roundrobin := make([]cloudflare.Record, 0)

	for _, record := range recs {
		if len(record.Name) == 32 {
			log.Println("PEER: ", record.Value)
			peers = append(peers, record)
		} else if record.Name == ROUNDROBIN {
			log.Println("IN ROUNDROBIN IP: ", record.Name, record.Value)
			roundrobin = append(roundrobin, record)
		} else {
			log.Println("NON-PEER IP: ", record.Name, record.Value)
		}
	}

	log.Printf("HOSTS IN PEERS: %v", len(peers))
	log.Printf("HOSTS IN ROUNDROBIN: %v", len(roundrobin))

	//removeAllPeersFromRoundRobin(client, roundrobin)

	//removeAllPeers(client, peers)

	successes := make(chan cloudflare.Record)
	failures := make(chan cloudflare.Record)

	for _, r := range peers {
		go testServer(client, r, successes, failures, 1)
	}



	if len(peers) > 0 {
		responses := 0
		OuterLoop:
			for {
				select {
				case r := <-successes:
					log.Printf("%s was successful\n", r.Value)
					responses++

					// Check to see if the peer is already in the round robin
					rr := false
					for _, rec := range roundrobin {
						if rec.Value == r.Value {
							log.Println("Server is already in round robin: ", r.Value)
							rr = true
							break
						}
					}
					if !rr {
						addToSubdomain(client, r, ROUNDROBIN)
						addToSubdomain(client, r, PEERS)
					}
					if responses == len(peers) {
						break OuterLoop
					}
				case r := <-failures:
					log.Printf("%s failed\n", r.Value)
					responses++
					for _, rec := range roundrobin {
						if rec.Value == r.Value {
							log.Println("Deleting server from round robin: ", r.Value)

							// Destroy the peer in the roundrobin...
							client.DestroyRecord(rec.Domain, rec.Id)
							break
						}
					}
					client.DestroyRecord(r.Domain, r.Id)
					if responses == len(peers) {
						break OuterLoop
					}
				case <-time.After(20 * time.Second):
					fmt.Printf(".")
					break OuterLoop
				}
			}
	}

	// Now check to make sure all the servers are working as well. The danger
	// here is that the server start failing because they're overloaded, 
	// we start a cascading failure effect where we kill the most overloaded
	// servers and add their load to the remaining ones, thereby making it
	// much more likely those will fail as well. Our approach should take 
	// this into account and should only kill servers if their failure rates
	// are much higher than the others and likely leaving a reasonable number
	// of servers in the mix no matter what.
	successes = make(chan cloudflare.Record)
	failures = make(chan cloudflare.Record)

	for _, r := range roundrobin {
		// Require many failures in a row for peers in the roundrobin, as those are
		// servers of last resort (if we delete them all, we have a problem!)
		// Only test non-peers since peers are already handled in the above 
		// function.
		if !isPeer(r, peers) {
			go testServer(client, r, successes, failures, 4)
		}
	}

	if len(roundrobin) > 0 {
		responses := 0
		OutOfFor:
			for {
				select {
				case r := <-successes:
					log.Printf("%s was successful\n", r.Value)
					responses++
					if responses == len(peers) {
						break OutOfFor
					}
				case r := <-failures:
					log.Printf("%s failed\n", r.Value)
					responses++
					log.Println("Deleting server from roundrobin - disabled for now: ", r.Value)

					// Destroy the server in the roundrobin...
					client.DestroyRecord(r.Domain, r.Id)

					if responses == len(peers) {
						break OutOfFor
					}
				case <-time.After(20 * time.Second):
					fmt.Printf(".")
					break OutOfFor
				}
			}
	}

	//close(complete)

	log.Println("Pass complete")
}

func isPeer(r cloudflare.Record, peers []cloudflare.Record) bool {
	for _, rec := range peers {
		if rec.Value == r.Value {
			log.Println("Found peer: ", r.Value)
			return true
		}
	}
	return false
}

/*
func removeAllPeers(client *cloudflare.Client, peers []cloudflare.Record) {
	for _, r := range peers {
		log.Println("Removing peer: ", r.Value)
		client.DestroyRecord(r.Domain, r.Id)

		// TODO: THIS IS WRONG -- SHOULD SEARCH FOR THE IP IN THE ROUNDROBIN
		client.DestroyRecord(CF_DOMAIN, r.Id)
	}
}
*/

func removeAllPeersFromRoundRobin(client *cloudflare.Client, roundrobin []cloudflare.Record) {
	for _, r := range roundrobin {
		if !strings.HasPrefix(r.Value, "128") {
			client.DestroyRecord(CF_DOMAIN, r.Id)
		}
	}
}

func callbackToPeer(upstreamHost string) bool {
	client := clientFor(upstreamHost, "", "")

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

func addToSubdomain(client *cloudflare.Client, record cloudflare.Record, subdomain string) {
	log.Println("ADDING IP TO ROUNDROBIN!!: ", record.Value)
	cr := cloudflare.CreateRecord{Type: "A", Name: subdomain, Content: record.Value}
	rec, err := client.CreateRecord(CF_DOMAIN, &cr)

	if err != nil {
		log.Println("Could not create record? ", err)
		return
	}

	log.Println("Successfully created record for: ", rec.FullName, rec.Value)

	// Note for some reason CloudFlare seems to ignore the TTL here.
	ur := cloudflare.UpdateRecord{Type: "A", Name: subdomain, Content: rec.Value, Ttl: "360", ServiceMode: "1"}

	err = client.UpdateRecord(CF_DOMAIN, rec.Id, &ur)

	if err != nil {
		log.Println("Could not update record? ", err)
	} else {
		log.Println("Successfully updated record for ", record.Value)
	}
}

func testServer(cf *cloudflare.Client, rec cloudflare.Record, successes chan<- cloudflare.Record,
	failures chan<- cloudflare.Record, attempts int) {

	for i := 0; i < attempts; i++ {
		success := runTest(cf, rec)

		if success {
			// If we get a single success we can exit the loop and consider it a success.
			successes <- rec
			break
		} else if i == (attempts - 1) {
			// If we get consecutive failures up to our threshhold, consider it a failure.
			failures <- rec
		}
	}
}

func runTest(cf *cloudflare.Client, rec cloudflare.Record) bool {
	httpClient := clientFor(rec.Name+".getiantem.org", common.MASQUERADE_AS, common.ROOT_CA)

	req, _ := http.NewRequest("GET", "http://www.google.com/humans.txt", nil)
	resp, err := httpClient.Do(req)
	log.Println("Finished http call for ", rec.Value)
	if err != nil {
		fmt.Errorf("HTTP Error: %s", resp)
		log.Println("HTTP ERROR HITTING PEER: ", rec.Value, err)
		return false
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			fmt.Errorf("HTTP Body Error: %s", body)
			log.Println("Error reading body for peer: ", rec.Value)
			return false
		} else {
			log.Printf("RESPONSE FOR PEER: %s, %s\n", rec.Value, body)
			return true
		}
	}
}


func clientFor(upstreamHost string, masqueradeHost string, rootCA string) *http.Client {

	serverInfo := &client.ServerInfo{
		Host: upstreamHost,
		Port: 443,
		DialTimeoutMillis: 5000,
	}
	masquerade := &client.Masquerade{masqueradeHost, rootCA}
	httpClient := client.HttpClient(serverInfo, masquerade)

	return httpClient
}
