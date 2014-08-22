package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
	"github.com/getlantern/cloudflare"

	"github.com/getlantern/peerscanner/common"
)

const (
	CF_DOMAIN = "getiantem.org"
)

var failedips = make([]string, 2)
var sucessfulips = make([]string, 20)

func main() {
	log.Println("Starting CloudFlare Flashlight Tests...")
	client, err := cloudflare.NewClient("", "")
	if err != nil {
		log.Println("Could not create CloudFlare client:", err)
		return
	}

	for {
		log.Println("Starting pass!")
		failedips = make([]string, 2)
		sucessfulips = make([]string, 20)
		loopThroughRecords(client)

		log.Println("Sleeping!")
		time.Sleep(6 * time.Second)
	}

}

func loopThroughRecords(client *cloudflare.Client) {
	records, err := client.LoadAll("getiantem.org")
	if err != nil {
		log.Println("Error retrieving record!", err)
		return
	}

	recs := records.Response.Recs.Records

	// Sleep to make sure records have propagated to CloudFlare internally.
	time.Sleep(10 * time.Second)

	// Loop through once to hit all the peers to see if they fail.
	c := make(chan bool)
	numpeers := 0
	for _, record := range recs {
		if len(record.Name) == 32 {
			log.Println("PEER: ", record.Name)

			go testPeer(record.Domain, record.Id, record.Name, record.Value, c)
			numpeers++

		} else {
			log.Println("VALUE: ", record.Value)
		}
	}

	successes := 0
	failures := 0
	for i := 0; i < numpeers; i++ {
		result := <-c
		if result {
			successes++
		} else {
			failures++
		}
		fmt.Println(result)
	}
	log.Printf("RESULTS:\nSUCCESES: %i\nFAILURES: %i\n", successes, failures)

	log.Println("FAILED IPS: ", failedips)

	// Now loop through again and remove any entries for failed ips
	for _, record := range recs {
		if record.Type != "A" {
			log.Println("NOT AN A RECORD: ", record.Type)
			continue
		}
		for _, ip := range failedips {
			if record.Value == ip {
				log.Println("DELETING VALUE: ", record.Value)
				client.DestroyRecord(record.Domain, record.Id)
			}
		}
	}

	for _, record := range recs {
		if len(record.Name) == 32 {
			log.Println("PEER: ", record.Name)
			for _, ip := range sucessfulips {
				if record.Value == ip {
					log.Println("ADDING IP TO ROUNDROBIN!!: ", record.Value)
					cr := cloudflare.CreateRecord{Type: "A", Name: "roundrobin", Content: record.Value}
					rec, err := client.CreateRecord(CF_DOMAIN, &cr)

					if err != nil {
						log.Println("Could not create record? ", err)
						break
					}

					log.Println("Successfully created record for: ", rec.FullName, rec.Value)

					// Note for some reason CloudFlare seems to ignore the TTL here.
					ur := cloudflare.UpdateRecord{Type: "A", Name: rec.Name, Content: rec.Value, Ttl: "360", ServiceMode: "1"}	

					err = client.UpdateRecord(CF_DOMAIN, rec.Id, &ur)

					if err != nil {
						log.Println("Could not update record? ", err)
						break
					} else {
						log.Println("Successfully updated record for ", ip)
					}
				}
			}

		} else {
			log.Println("VALUE: ", record.Value)
		}
	}
}

func testPeer(domain string, id string, name string, ip string, c chan<- bool) {

	client := &common.FlashlightClient{
		UpstreamHost: name + ".getiantem.org"} //record.Name} //"roundrobin.getiantem.org"}

	httpClient := client.NewClient()

	req, _ := http.NewRequest("GET", "http://www.google.com/humans.txt", nil)
	resp, err := httpClient.Do(req)
	log.Println("Finished http call for ", ip)
	if err != nil {
		fmt.Errorf("HTTP Error: %s", resp)
		log.Println("REMOVING RECORD FOR PEER: ", ip, err)

		// If it's a peer, remove it.
		//cf.remove(domain, id)
		failedips = append(failedips, ip)
		c <- false
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			fmt.Errorf("HTTP Body Error: %s", body)
			log.Println("Error reading body for peer: ", ip)
			//cf.remove(domain, id)
			failedips = append(failedips, ip)
			c <- false
		} else {
			log.Printf("RESPONSE FOR PEER: %s, %s\n", name, body)
			sucessfulips = append(sucessfulips, ip)
			c <- true
		}
	}
}
