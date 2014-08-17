package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	fmt.Println("Starting CloudFlare Flashlight Tests...")
	cf := &CloudflareApi{}

	for {
		loopThroughRecords(cf)
		//time.Sleep(10 * time.Second)
	}

}

func loopThroughRecords(cf *CloudflareApi) {
	records, err := cf.loadAll("getiantem.org")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error retrieving record!", err)
		return
	}

	recs := records.Response.Recs.Records

	c := make(chan string)

	// Now loop through all the records and try to hit each peer to
	// see if it's up.
	numpeers := 0
	for _, record := range recs {
		fmt.Println("RECORD: ", record)
		if record.Type != "A" {
			fmt.Println("NOT AN A RECORD: ", record.Type)
			continue
		}
		if len(record.Name) == 32 {
			fmt.Println("PEER: ", record.Name)

			go testPeer(cf, record.Domain, record.Id, record.Name, c)
			numpeers++
			//queue := mq.New("peers")

			//queue.PushString()

		} else {
			fmt.Println("NOT A PEER: ", record.Name)
		}
	}

	for i := 0; i < numpeers; i++ {
		fmt.Print(<-c)
	}

}

func testPeer(cf *CloudflareApi, domain string, id string, name string, c chan<- string) {

	client := &FlashlightClient{
		UpstreamHost: name + ".getiantem.org"} //record.Name} //"roundrobin.getiantem.org"}

	httpClient := client.newClient()

	req, _ := http.NewRequest("GET", "http://www.google.com/humans.txt", nil)
	resp, err := httpClient.Do(req)
	log.Println("Finished http call")
	if err != nil {
		fmt.Errorf("HTTP Error: %s", resp)
		log.Println("REMOVING RECORD FOR PEER: %s", name)

		// If it's a peer, remove it.
		cf.remove(domain, id)
		c <- "failure in HTTP"
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			fmt.Errorf("HTTP Body Error: %s", body)
			log.Println("Error reading body")
			cf.remove(domain, id)
			c <- "failure reading body"
		} else {
			log.Printf("RESPONSE FOR PEER: %s, %s", name, body)
			c <- "success"
		}
	}
}
