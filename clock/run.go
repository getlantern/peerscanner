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
)

const (
	CF_DOMAIN     = "getiantem.org"
	MASQUERADE_AS = "cdnjs.com"
	ROOT_CA       = "-----BEGIN CERTIFICATE-----\nMIIDdTCCAl2gAwIBAgILBAAAAAABFUtaw5QwDQYJKoZIhvcNAQEFBQAwVzELMAkG\nA1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsTB1Jv\nb3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw05ODA5MDExMjAw\nMDBaFw0yODAxMjgxMjAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9i\nYWxTaWduIG52LXNhMRAwDgYDVQQLEwdSb290IENBMRswGQYDVQQDExJHbG9iYWxT\naWduIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDaDuaZ\njc6j40+Kfvvxi4Mla+pIH/EqsLmVEQS98GPR4mdmzxzdzxtIK+6NiY6arymAZavp\nxy0Sy6scTHAHoT0KMM0VjU/43dSMUBUc71DuxC73/OlS8pF94G3VNTCOXkNz8kHp\n1Wrjsok6Vjk4bwY8iGlbKk3Fp1S4bInMm/k8yuX9ifUSPJJ4ltbcdG6TRGHRjcdG\nsnUOhugZitVtbNV4FpWi6cgKOOvyJBNPc1STE4U6G7weNLWLBYy5d4ux2x8gkasJ\nU26Qzns3dLlwR5EiUWMWea6xrkEmCMgZK9FGqkjWZCrXgzT/LCrBbBlDSgeF59N8\n9iFo7+ryUp9/k5DPAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8E\nBTADAQH/MB0GA1UdDgQWBBRge2YaRQ2XyolQL30EzTSo//z9SzANBgkqhkiG9w0B\nAQUFAAOCAQEA1nPnfE920I2/7LqivjTFKDK1fPxsnCwrvQmeU79rXqoRSLblCKOz\nyj1hTdNGCbM+w6DjY1Ub8rrvrTnhQ7k4o+YviiY776BQVvnGCv04zcQLcFGUl5gE\n38NflNUVyRRBnMRddWQVDf9VMOyGj/8N7yy5Y0b2qvzfvGn9LhJIZJrglfCm7ymP\nAbEVtQwdpf5pLGkkeB6zpxxxYu7KyJesF12KwvhHhm4qxFYxldBniYUr+WymXUad\nDKqC5JlR3XC321Y9YeRq4VzW9v493kHMB65jUr9TU/Qr6cf9tveCX4XSQRjbgbME\nHMUfpIBvFSDJ3gyICh3WZlXi/EjJKSZp4A==\n-----END CERTIFICATE-----\n"
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

	log.Printf("HOSTS IN PEERS: %v", len(peers))
	log.Printf("HOSTS IN ROUNDROBIN: %v", len(roundrobin))

	//removeAllPeers(client, peers)

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
				for _, rec := range roundrobin {
					if rec.Value == r.Value {
						log.Println("Deleting peer from round robin: ", r.Value)
						client.DestroyRecord(CF_DOMAIN, rec.Id)
						client.DestroyRecord(rec.Domain, rec.Id)
					}
				}
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

	// Sleep here instead to make sure records have propagated to CloudFlare internally.
	log.Println("Sleeping!")
	time.Sleep(10 * time.Second)
	//close(complete)

	log.Println("Waiting for additions")
}

func removeAllPeers(client *cloudflare.Client, peers []cloudflare.Record) {
	for _, r := range peers {
		log.Println("Removing peer: ", r.Value)
		client.DestroyRecord(r.Domain, r.Id)
		client.DestroyRecord(CF_DOMAIN, r.Id)
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

	httpClient := clientFor(rec.Name+".getiantem.org", MASQUERADE_AS, ROOT_CA)

	req, _ := http.NewRequest("GET", "http://www.google.com/humans.txt", nil)
	resp, err := httpClient.Do(req)
	log.Println("Finished http call for ", rec.Value)
	if err != nil {
		fmt.Errorf("HTTP Error: %s", resp)
		log.Println("HTTP ERROR HITTING PEER: ", rec.Value, err)
		//cf.DestroyRecord(CF_DOMAIN, rec.Id)
		//cf.DestroyRecord(rec.Domain, rec.Id)
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

func clientFor(upstreamHost string, masqueradeHost string, rootCA string) *http.Client {
	flashlightClient := &proxy.Client{
		UpstreamHost:   upstreamHost,
		UpstreamPort:   443,
		MasqueradeAs: masqueradeHost,
		DialTimeout:    5 * time.Second,
		RootCA:         rootCA,
	}

	if rootCA == "" {
		flashlightClient.InsecureSkipVerify = true
	}

	flashlightClient.BuildEnproxyConfig()

	return &http.Client{
		Transport: &http.Transport{
			Dial: flashlightClient.DialWithEnproxy,
		},
	}
}
