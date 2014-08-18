package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getlantern/peerscanner/common"
)

var failedips = make([]string, 2)

func main() {
	fmt.Println("Starting CloudFlare Flashlight Tests...")
	cf := &common.CloudflareApi{}

	for {
		fmt.Println("Starting pass!")
		failedips = make([]string, 2)
		loopThroughRecords(cf)

		fmt.Println("Sleeping!")
		time.Sleep(6 * time.Second)
	}

}

func loopThroughRecords(cf *CloudflareApi) {
	records, err := cf.loadAll("getiantem.org")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error retrieving record!", err)
		return
	}

	recs := records.Response.Recs.Records

	// Loop through once to hit all the peers to see if they fail.
	c := make(chan bool)
	numpeers := 0
	for _, record := range recs {
		if len(record.Name) == 32 {
			fmt.Println("PEER: ", record.Name)

			go testPeer(cf, record.Domain, record.Id, record.Name, record.Value, c)
			numpeers++

		} else {
			fmt.Println("VALUE: ", record.Value)
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
	fmt.Printf("RESULTS:\nSUCCESES: %i\nFAILURES: %i\n", successes, failures)

	fmt.Println("FAILED IPS: ", failedips)

	// Now loop through again and remove any entries for failed ips
	for _, record := range recs {
		if record.Type != "A" {
			fmt.Println("NOT AN A RECORD: ", record.Type)
			continue
		}
		for _, ip := range failedips {
			if record.Value == ip {
				fmt.Println("DELETING VALUE: ", record.Value)
				cf.remove(record.Domain, record.Id)
			}
		}
	}
}

func testPeer(cf *CloudflareApi, domain string, id string, name string, value string, c chan<- bool) {

	client := &common.FlashlightClient{
		UpstreamHost: name + ".getiantem.org"} //record.Name} //"roundrobin.getiantem.org"}

	httpClient := client.newClient()

	req, _ := http.NewRequest("GET", "http://www.google.com/humans.txt", nil)
	resp, err := httpClient.Do(req)
	log.Println("Finished http call")
	if err != nil {
		fmt.Errorf("HTTP Error: %s", resp)
		log.Println("REMOVING RECORD FOR PEER: %s", name)

		// If it's a peer, remove it.
		//cf.remove(domain, id)
		failedips = append(failedips, value)
		c <- false
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			fmt.Errorf("HTTP Body Error: %s", body)
			log.Println("Error reading body")
			//cf.remove(domain, id)
			failedips = append(failedips, value)
			c <- false
		} else {
			log.Printf("RESPONSE FOR PEER: %s, %s", name, body)
			c <- true
		}
	}
}
