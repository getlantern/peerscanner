package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	//"sync"
	"github.com/getlantern/cloudflare"
	"time"

	"github.com/getlantern/flashlight/proxy"
	"github.com/getlantern/peerscanner/common"
)

const (
	CF_DOMAIN = "getiantem.org"
)

type RecordTest struct {
	rec     cloudflare.Record
	success bool
}

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

func getAllRecords(client *cloudflare.Client) (*cloudflare.RecordsResponse, error) {
	records, err := client.LoadAll("getiantem.org")
	if err != nil {
		log.Println("Error retrieving record!", err)
		return nil, err
	}

	log.Println("Loaded original records...", records.Response.Recs.Count)

	//recs := records.Response.Recs.Records

	if records.Response.Recs.HasMore {
		return getAllRecordsByIndex(client, records.Response.Recs.Count, records)
	} 
	return records, nil
}

func getAllRecordsByIndex(client *cloudflare.Client, index int, response *cloudflare.RecordsResponse) (*cloudflare.RecordsResponse, error) {

	records, err := client.LoadAllAtIndex("getiantem.org", index)
	if err != nil {
		log.Println("Error retrieving record!", err)
		return nil, err
	}

	log.Println("Loaded original records...", records.Response.Recs.Count)

	response.Response.Recs.Records = append(response.Response.Recs.Records, records.Response.Recs.Records...)

	response.Response.Recs.Count = response.Response.Recs.Count + records.Response.Recs.Count

	if records.Response.Recs.HasMore {
		log.Println("Adding additional records")
		return getAllRecordsByIndex(client, response.Response.Recs.Count, response)
	} else {
		log.Println("Not loading additional records. Loaded: ", records.Response.Recs.Count)
		return response, nil
	}

}

func loopThroughRecords(client *cloudflare.Client) {
	records, err := getAllRecords(client)
	if err != nil {
		log.Println("Error retrieving record!", err)
		return
	}
	log.Println("Loaded all records...", records.Response.Recs.Count)

	recs := records.Response.Recs.Records

	log.Println("Total records loaded: ", len(recs))

	// Loop through everything to do some bookkeeping and to put 
	// records in their appropriate categories.
	
	// All peers.
	peers := make([]cloudflare.Record, 0)

	// All entries in the round robin.
	roundrobin := make([]cloudflare.Record, 0)

	//var wg sync.WaitGroup
	for _, record := range recs {
		if len(record.Name) == 32 {
			log.Println("PEER: ", record.Value)
			peers = append(peers, record)
		} else if record.Name == "roundrobin" {
			log.Println("IN ROUNDROBIN IP: ", record.Name, record.Value)
			roundrobin = append(roundrobin, record)
		} else {
			log.Println("NON-PEER IP: ", record.Name, record.Value)
		}
	}

	log.Printf("IN ROUNDROBIN: %v", len(roundrobin))

	successes := make(chan cloudflare.Record)
	failures := make(chan cloudflare.Record)
	for _, r := range peers {
		go testPeer(client, r, successes, failures)
	}

	if len(peers) > 0 {
		responses := 0
		for {
			select {
			case r := <-successes:
				log.Printf("%s was successful\n", r.Value)
				responses++

				// Check to see if the peer is already in the round robin
				rr := false
				for _, rec := range roundrobin {
					if rec.Value == r.Value {
						log.Println("Peer is already in round robin: ", r.Value)
						rr = true
						break
					}
				}
				if !rr {
					addToRoundRobin(client, r)
				}
				if responses == len(peers) {
					return
				}
			case r := <-failures:
				log.Printf("%s failed\n", r.Value)
				responses++
				client.DestroyRecord(r.Domain, r.Id)
				if responses == len(peers) {
					return
				}
			case <-time.After(20 * time.Second):
				fmt.Printf(".")
				return
			}
		}
	}

	/*
		for _, r := range roundrobin {
			//log.Println("Testing roundrobin entry: ", r.Value)

				go func() {
					log.Println("CALLING BACK TO roundrobin entry: ", r.Value)
					success := callbackToPeer(r.Value)
					if !success {
						log.Println("Destroying roundrobin record: ", r.Value)
						go client.DestroyRecord(r.Domain, r.Id)
					} else {
						log.Println("Ignoring successful roundrobin record: ", r.Value)
					}
				}()

			//pending <- r
		}
	*/
	//log.Println("Checking complete records...")

	/*
		go func() {
			for r := range complete {
				if !r.success {
					log.Println("Destroying roundrobin record: ", r.rec.Value)
					go client.DestroyRecord(r.rec.Domain, r.rec.Id)
				} else {
					log.Println("Ignoring successful roundrobin record: ", r.rec.Value)
				}
			}
		}()
	*/

	//log.Println("Destroyed all failing records")

	// Now loop through and add any successful IPs that aren't
	// already in the roundrobin.
	/*
	for _, record := range successful {
		log.Println("PEER: ", record.Name)
		rr := false
		for _, rec := range roundrobin {
			if rec.Value == record.Value {
				log.Println("Peer is already in round robin: ", record.Value)
				rr = true
				break
			}
		}
		if !rr {
			// Disabled for now.
			//addToRoundRobin(client, record)
		}
	}
	*/

	// Sleep here instead to make sure records have propagated to CloudFlare internally.
	log.Println("Sleeping!")
	time.Sleep(10 * time.Second)
	//close(complete)

	log.Println("Waiting for additions")
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

func addToRoundRobin(client *cloudflare.Client, record cloudflare.Record) {
	log.Println("ADDING IP TO ROUNDROBIN!!: ", record.Value)
	cr := cloudflare.CreateRecord{Type: "A", Name: "roundrobin", Content: record.Value}
	rec, err := client.CreateRecord(CF_DOMAIN, &cr)

	if err != nil {
		log.Println("Could not create record? ", err)
		return
	}

	log.Println("Successfully created record for: ", rec.FullName, rec.Value)

	// Note for some reason CloudFlare seems to ignore the TTL here.
	ur := cloudflare.UpdateRecord{Type: "A", Name: rec.Name, Content: rec.Value, Ttl: "360", ServiceMode: "1"}

	err = client.UpdateRecord(CF_DOMAIN, rec.Id, &ur)

	if err != nil {
		log.Println("Could not update record? ", err)
	} else {
		log.Println("Successfully updated record for ", record.Value)
	}
}

func testPeer(cf *cloudflare.Client, rec cloudflare.Record, successes chan<- cloudflare.Record,
	failures chan<- cloudflare.Record) bool {

	client := &common.FlashlightClient{
		UpstreamHost: rec.Name + ".getiantem.org"} //record.Name} //"roundrobin.getiantem.org"}

	httpClient := client.NewClient()

	req, _ := http.NewRequest("GET", "http://www.google.com/humans.txt", nil)
	resp, err := httpClient.Do(req)
	log.Println("Finished http call for ", rec.Value)
	if err != nil {
		fmt.Errorf("HTTP Error: %s", resp)
		log.Println("HTTP ERROR HITTING PEER: ", rec.Value, err)
		cf.DestroyRecord(CF_DOMAIN, rec.Id)
		cf.DestroyRecord(rec.Domain, rec.Id)
		failures <- rec
		return false
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			fmt.Errorf("HTTP Body Error: %s", body)
			log.Println("Error reading body for peer: ", rec.Value)
			//cf.remove(domain, id)
			//c <- RecordTest{rec, false}
			failures <- rec
			return false
		} else {
			log.Printf("RESPONSE FOR PEER: %s, %s\n", rec.Value, body)
			//c <- RecordTest{rec, true}
			successes <- rec
			return true
		}
	}
}
