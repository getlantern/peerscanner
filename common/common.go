package common

import (
	"log"
	"github.com/getlantern/cloudflare"
)

func RemoveIpFromRoundRobin(client *cloudflare.Client, ip string) error {
	client, err := cloudflare.NewClient("", "")
	if err != nil {
		log.Println("Could not create CloudFlare client:", err)
		return err
	}

	roundrobin, err := GetRoundRobinRecords(client)
	if err != nil {
		return err
	}
	return RemoveIpFromRoundRobinRecords(client, ip, roundrobin)

}

func RemoveIpFromRoundRobinRecords(client *cloudflare.Client, ip string, roundrobin []cloudflare.Record) error {
	for _, rec := range roundrobin {
		if rec.Value == ip {
			log.Println("Destroying record ", rec.Value)
			err := client.DestroyRecord(rec.Domain, rec.Id)
			return err
		}
	}
	return nil
}

func GetRoundRobinRecords(client *cloudflare.Client) ([]cloudflare.Record, error) {
	records, err := GetAllRecords(client)

	if err != nil {
		log.Println("Could not get records:", err)
		return nil, err
	}

	recs := records.Response.Recs.Records

	roundrobin := make([]cloudflare.Record, 0)
	for _, record := range recs {
		if record.Name == "roundrobin" {
			log.Println("IN ROUNDROBIN IP: ", record.Name, record.Value)
			roundrobin = append(roundrobin, record)
		} else {
			log.Println("NON ROUNDROBIN IP: ", record.Name, record.Value)
		}
	}
	return roundrobin, nil
}

func GetAllRecords(client *cloudflare.Client) (*cloudflare.RecordsResponse, error) {
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