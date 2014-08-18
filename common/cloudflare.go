package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/pearkes/cloudflare"
)

type CloudflareApi struct {
}

func (api *CloudflareApi) loadAll(domain string) (*cloudflare.RecordsResponse, error) {

	client, err := cloudflare.NewClient("", "")
	if err != nil {
		return nil, fmt.Errorf("Error creating clouflare client: %s", err)
	}

	params := make(map[string]string)
	// The zone we want
	params["z"] = domain

	req, err := client.NewRequest(params, "GET", "rec_load_all")

	if err != nil {
		return nil, fmt.Errorf("Error creating request: %s", err)
	}

	resp, err := api.checkResp(client.Http.Do(req))
	if err != nil {
		return nil, fmt.Errorf("Error destroying record: %s", err)
	}

	records := new(cloudflare.RecordsResponse)

	err = api.decodeBody(resp, records)

	if err != nil {
		return nil, fmt.Errorf("Error decoding record response: %s", err)
	}

	// The request was successful
	return records, nil
}

func (api *CloudflareApi) remove(domain string, id string) error {
	client, err := cloudflare.NewClient("", "")
	if err != nil {
		return fmt.Errorf("Error creating clouflare client: %s", err)
	}

	err = client.DestroyRecord(domain, id)

	if err != nil {
		log.Println("Error destroying record")
		return fmt.Errorf("Error creating request: %s", err)
	}
	return nil
}

// decodeBody is used to JSON decode a body
func (api *CloudflareApi) decodeBody(resp *http.Response, out interface{}) error {
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	if err = json.Unmarshal(body, &out); err != nil {
		return err
	}

	return nil
}

// checkResp wraps http.Client.Do() and verifies that the
// request was successful. A non-200 request returns an error
// formatted to included any validation problems or otherwise
func (api *CloudflareApi) checkResp(resp *http.Response, err error) (*http.Response, error) {
	// If the err is already there, there was an error higher
	// up the chain, so just return that
	if err != nil {
		return resp, err
	}

	switch i := resp.StatusCode; {
	case i == 200:
		return resp, nil
	default:
		return nil, fmt.Errorf("API Error: %s", resp.Status)
	}
}
