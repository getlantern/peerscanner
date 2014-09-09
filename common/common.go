package common

import (
	"log"
	"github.com/getlantern/cloudflare"
)

const (
	CF_DOMAIN     = "getiantem.org"
	MASQUERADE_AS = "cdnjs.com"
	ROOT_CA       = "-----BEGIN CERTIFICATE-----\nMIIDdTCCAl2gAwIBAgILBAAAAAABFUtaw5QwDQYJKoZIhvcNAQEFBQAwVzELMAkG\nA1UEBhMCQkUxGTAXBgNVBAoTEEdsb2JhbFNpZ24gbnYtc2ExEDAOBgNVBAsTB1Jv\nb3QgQ0ExGzAZBgNVBAMTEkdsb2JhbFNpZ24gUm9vdCBDQTAeFw05ODA5MDExMjAw\nMDBaFw0yODAxMjgxMjAwMDBaMFcxCzAJBgNVBAYTAkJFMRkwFwYDVQQKExBHbG9i\nYWxTaWduIG52LXNhMRAwDgYDVQQLEwdSb290IENBMRswGQYDVQQDExJHbG9iYWxT\naWduIFJvb3QgQ0EwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDaDuaZ\njc6j40+Kfvvxi4Mla+pIH/EqsLmVEQS98GPR4mdmzxzdzxtIK+6NiY6arymAZavp\nxy0Sy6scTHAHoT0KMM0VjU/43dSMUBUc71DuxC73/OlS8pF94G3VNTCOXkNz8kHp\n1Wrjsok6Vjk4bwY8iGlbKk3Fp1S4bInMm/k8yuX9ifUSPJJ4ltbcdG6TRGHRjcdG\nsnUOhugZitVtbNV4FpWi6cgKOOvyJBNPc1STE4U6G7weNLWLBYy5d4ux2x8gkasJ\nU26Qzns3dLlwR5EiUWMWea6xrkEmCMgZK9FGqkjWZCrXgzT/LCrBbBlDSgeF59N8\n9iFo7+ryUp9/k5DPAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8E\nBTADAQH/MB0GA1UdDgQWBBRge2YaRQ2XyolQL30EzTSo//z9SzANBgkqhkiG9w0B\nAQUFAAOCAQEA1nPnfE920I2/7LqivjTFKDK1fPxsnCwrvQmeU79rXqoRSLblCKOz\nyj1hTdNGCbM+w6DjY1Ub8rrvrTnhQ7k4o+YviiY776BQVvnGCv04zcQLcFGUl5gE\n38NflNUVyRRBnMRddWQVDf9VMOyGj/8N7yy5Y0b2qvzfvGn9LhJIZJrglfCm7ymP\nAbEVtQwdpf5pLGkkeB6zpxxxYu7KyJesF12KwvhHhm4qxFYxldBniYUr+WymXUad\nDKqC5JlR3XC321Y9YeRq4VzW9v493kHMB65jUr9TU/Qr6cf9tveCX4XSQRjbgbME\nHMUfpIBvFSDJ3gyICh3WZlXi/EjJKSZp4A==\n-----END CERTIFICATE-----\n"
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