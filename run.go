package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	fmt.Println("Starting CloudFlare Flashlight Tests...")
	cf := &CloudflareApi{}

	for {
		loopThroughRecords(cf)
		time.Sleep(10 * time.Second)
	}

}

func loopThroughRecords(cf *CloudflareApi) {
	records, err := cf.loadAll("getiantem.org")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error retrieving record!", err)
		return
	}

	recs := records.Response.Recs.Records

	// Now loop through all the records and try to hit each peer to
	// see if it's up.
	for _, record := range recs {
		fmt.Println("RECORD: ", record)
		if record.Type != "A" {
			fmt.Println("NOT AN A RECORD: ", record.Type)
			continue
		}
		if len(record.Name) == 32 {
			fmt.Println("PEER: ", record.Name)

			client := &FlashlightClient{
				UpstreamHost: record.Name + ".getiantem.org"} //record.Name} //"roundrobin.getiantem.org"}

			httpClient := client.newClient()

			req, _ := http.NewRequest("GET", "http://www.google.com/humans.txt", nil)
			resp, err := httpClient.Do(req)
			log.Println("Finished http call")
			if err != nil {
				fmt.Errorf("HTTP Error: %s", resp)
				log.Println("Removing record")

				// If it's a peer, remove it.
				cf.remove(record.Domain, record.Id)
			} else {
				body, err := ioutil.ReadAll(resp.Body)
				defer resp.Body.Close()
				if err != nil {
					fmt.Errorf("HTTP Body Error: %s", body)
					log.Println("Returning 2")
					continue
				}

				log.Printf("RESPONSE: %s", body)
			}
		} else {
			fmt.Println("NOT A PEER: ", record.Name)
		}
	}
}
