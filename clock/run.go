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

	// All servers
	servers := make([]cloudflare.Record, 0)

	// All entries in the general round robin.
	mixedrr := make([]cloudflare.Record, 0)

	// All entries in the fallback round robin.
	fallbacksrr := make([]cloudflare.Record, 0)

	// All entries in the peer round robin.
	peersrr := make([]cloudflare.Record, 0)

	for _, record := range recs {
		// We just check the length of the subdomain here, which is the unique
		// peer GUID. While it's possible something else could have a subdomain
		// this long, it's unlikely.
		if isPeer(record) {
			log.Println("PEER: ", record.Value)
			peers = append(peers, record)
		} else if strings.HasPrefix(record.Name, "fl-") {
			log.Println("SERVER: ", record.Name, record.Value)
			servers = append(servers, record)
		} else if record.Name == ROUNDROBIN {
			log.Println("IN ROUNDROBIN: ", record.Name, record.Value)
			mixedrr = append(mixedrr, record)
		} else if record.Name == PEERS {
			log.Println("IN PEERS ROUNDROBIN: ", record.Name, record.Value)
			peersrr = append(peersrr, record)
		} else if record.Name == FALLBACKS {
			log.Println("IN FALLBACK ROUNDROBIN: ", record.Name, record.Value)
			fallbacksrr = append(fallbacksrr, record)
		} else {
			log.Println("UNKNOWN ENTRY: ", record.Name, record.Value)
		}
	}

	log.Printf("HOSTS IN PEERS: %v", len(peers))
	log.Printf("HOSTS IN ROUNDROBIN: %v", len(mixedrr))

	roundrobins := make(map[string][]cloudflare.Record)
	roundrobins[PEERS] = peersrr
	roundrobins[FALLBACKS] = fallbacksrr
	roundrobins[ROUNDROBIN] = mixedrr

	//removeAllPeersFromRoundRobin(client, roundrobin)

	//removeAllPeers(client, peers)

	testGroup(client, peers, 1, roundrobins, PEERS)


	// Now check to make sure all the servers are working as well. The danger
	// here is that the server start failing because they're overloaded, 
	// we start a cascading failure effect where we kill the most overloaded
	// servers and add their load to the remaining ones, thereby making it
	// much more likely those will fail as well. Our approach should take 
	// this into account and should only kill servers if their failure rates
	// are much higher than the others and likely leaving a reasonable number
	// of servers in the mix no matter what.
	testGroup(client, servers, 6, roundrobins, FALLBACKS)

	//close(complete)

	log.Println("Pass complete")
}

// testGroup: Runs tests against a group of DNS records in CloudFlare to see if they work. The group should
// not be a roundrobin group but rather a group of candidates servers, whether peers or not. If they work,
// they'll be added. If they don't, depending on the specified number of attempts signifying a failure, 
// they'll be removed.
func testGroup(client *cloudflare.Client, rr []cloudflare.Record, attempts int, 
	rrs map[string][]cloudflare.Record, group string) {
	successes := make(chan cloudflare.Record)
	failures := make(chan cloudflare.Record)

	for _, r := range rr {
		go testServer(client, r, successes, failures, attempts)
	}

	if len(rr) == 0 {
		log.Println("No records in group")
		return
	}
	responses := 0
	OuterLoop:
		for {
			select {
			case r := <-successes:
				log.Printf("%s was successful\n", r.Value)
				responses++

				addToRoundRobin(client, r, rrs[group], group)
				// Always add to the general roundrobin for now.
				addToRoundRobin(client, r, rrs[ROUNDROBIN], ROUNDROBIN)
				if responses == len(rr) {
					break OuterLoop
				}
			case r := <-failures:
				log.Printf("%s failed\n", r.Value)
				responses++
				removeFromRoundRobin(client, r, rrs[group])
				removeFromRoundRobin(client, r, rrs[ROUNDROBIN])

				// Only actually destroy the original record if it's for a peer.
				// Otherwise, we might restart the server or something so it will
				// work on a future pass.
				if isPeer(r) {
					client.DestroyRecord(r.Domain, r.Id)
				}
				if responses == len(rr) {
					break OuterLoop
				}
			case <-time.After(20 * time.Second):
				fmt.Printf(".")
				// TODO: We should also remove any that didn't explictly succeed.
				break OuterLoop
			}
		}
}

func addToRoundRobin(client *cloudflare.Client, r cloudflare.Record, rr []cloudflare.Record, group string) {
	// Check to see if the peer is already in the round robin before making a call
	// to the CloudFlare API.
	if !inRoundRobin(r, rr) {
		addToSubdomain(client, r, group)
	}
}

func inRoundRobin(r cloudflare.Record, rr []cloudflare.Record) bool {
	for _, rec := range rr {
		if rec.Value == r.Value {
			log.Println("Server is already in round robin: ", r.Value)
			return true
		}
	}
	return false
}

func removeFromRoundRobin(client *cloudflare.Client, r cloudflare.Record, rr []cloudflare.Record) {
	for _, rec := range rr {
		// Just look for the same IP.
		if rec.Value == r.Value {
			log.Println("Deleting server from round robin: ", r.Value)

			// Destroy the record in the roundrobin...
			client.DestroyRecord(rec.Domain, rec.Id)
			break
		}
	}
}

func isPeer(r cloudflare.Record) bool {
	// We just check the length of the subdomain here, which is the unique
	// peer GUID. While it's possible something else could have a subdomain
	// this long, it's unlikely.
	return len(r.Name) == 32 {
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
