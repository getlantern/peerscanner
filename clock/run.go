package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
	"github.com/getlantern/cloudflare"

	"github.com/getlantern/peerscanner/common"
)

const (
	CF_DOMAIN = "getiantem.org"
)

type RecordTest struct {
	rec *cloudflare.Record
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

func loopThroughRecords(client *cloudflare.Client) {
	records, err := client.LoadAll("getiantem.org")
	if err != nil {
		log.Println("Error retrieving record!", err)
		return
	}

	recs := records.Response.Recs.Records

	if records.Response.Recs.HasMore {
		log.Println("Adding additional records")
		newrecords, err := client.LoadAllAtIndex("getiantem.org", records.Response.Recs.Count)
		if err != nil {
			log.Println("Error retrieving record!", err)
			return
		}
		log.Println("Loaded more records...", newrecords.Response.Recs.Count)
		recs = append(recs, newrecords.Response.Recs.Records...)
	} else {
		log.Println("Not loading additional records. Loaded: ", records.Response.Recs.Count)
	}

	// Sleep here instead to make sure records have propagated to CloudFlare internally.
	log.Println("Sleeping!")
	time.Sleep(10 * time.Second)

	// Loop through once to hit all the peers to see if they fail.

	// All failed peers.
	failed := make([]*cloudflare.Record, 0)

	// All successful peers.
	successful := make([]*cloudflare.Record, 0)

	// All peers.
	peers := make([]*cloudflare.Record, 0)

	// All entries in the round robin.
	roundrobin := make([]*cloudflare.Record, 0)

	c := make(chan RecordTest)

	var wg sync.WaitGroup
	for _, record := range recs {
		if len(record.Name) == 32 {
			log.Println("PEER: ", record.Value)
			peers = append(peers, &record)
			go testPeer(client, record, c)
		} else if (record.Name == "roundrobin") {
			roundrobin = append(roundrobin, &record)
		} else {
			log.Println("NON-PEER IP: ", record.Name, record.Value)
		}
	}
	//log.Println("Waiting for all peer tests to complete")
	//wg.Wait()

	for i := 0; i < len(peers); i++ {
		test := <-c
		if test.success {
			successful = append(successful, test.rec)
		} else {
			failed = append(failed, test.rec)
		}
	}



	log.Printf("RESULTS: SUCCESES: %v FAILURES: %v\n", len(successful), len(failed))

	log.Printf("IN ROUNDROBIN: %v", len(roundrobin))

	log.Println("FAILED IPS: ", failed)

	// Now loop through again and remove any entries for failed ips.
	// Note we need to both remove them directly as well as from
	// the roundrobin if they exist there.
	/*
	for _, f := range failed {
		log.Println("DELETING VALUE: ", f)

		go func() {
			wg.Add(1)
			defer wg.Done()
			// Look for the IP in the roundrobin and remove it if it's
			// there
			for _, rec := range roundrobin {
				if (rec.Value == f.Value) {
					client.DestroyRecord(rec.Domain, rec.Id)
					break
				}
			}
			client.DestroyRecord(f.Domain, f.Id)
		}()
	}

	log.Println("Waiting for removals")
	wg.Wait()
	*/

	// Now loop through and add any successful IPs that aren't 
	// already in the roundrobin.
	for _, record := range successful {
		log.Println("PEER: ", record.Name)
		rr := false
		for _, rec := range roundrobin {
			if (rec.Value == record.Value) {
				log.Println("Peer is already in round robin: ", record.Value)
				rr = true
				break
			}
		}
		if !rr {
			addToRoundRobin(client, record)
		}
	}

	log.Println("Waiting for additions")
	wg.Wait()
}

func addToRoundRobin(client *cloudflare.Client, record *cloudflare.Record) {
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

func testPeer(cf *cloudflare.Client, rec cloudflare.Record, c chan<- RecordTest) bool {

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
		c <- RecordTest{&rec, false}
		return false
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			fmt.Errorf("HTTP Body Error: %s", body)
			log.Println("Error reading body for peer: ", rec.Value)
			//cf.remove(domain, id)
			c <- RecordTest{&rec, false}
			return false
		} else {
			log.Printf("RESPONSE FOR PEER: %s, %s\n", rec.Value, body)
			c <- RecordTest{&rec, true}
			return true
		}
	}
}
