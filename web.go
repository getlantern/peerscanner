package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/getlantern/peerscanner/common"

	"github.com/iron-io/iron_go/mq"
)

func register(w http.ResponseWriter, r *http.Request) {
	json, err := requestToReg(r)
	if err != nil {
		fmt.Println("Error converting request ", err)
	} else {
		postToQueue("peer-register", json)
	}
}

func unregister(w http.ResponseWriter, r *http.Request) {
	json, err := requestToReg(r)
	if err != nil {
		fmt.Println("Error converting request ", err)
	} else {
		postToQueue("peer-unregister", json)
	}
}

func requestToReg(r *http.Request) (string, error) {
	name := r.FormValue("name")
	fmt.Println("Read name: ", name)
	ip := r.FormValue("ip")
	portString := r.FormValue("port")

	port := 0
	if portString == "" {
		// Could be an unregister call
		port = 0
	} else {
		converted, err := strconv.Atoi(portString)
		if err != nil {
			// handle error
			fmt.Println(err)
			return "", err
		}
		port = converted
	}

	// If they're actually reporting an IP (it's a register call), make
	// sure the port is 443
	if len(ip) > 0 && port != 443 {
		fmt.Println("Ignoring port not on 443")
		return "", fmt.Errorf("Bad port: %d", port)
	}
	reg := &common.Reg{name, ip, port}

	json, err := json.Marshal(reg)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	jsonStr := string(json)
	fmt.Println(jsonStr)
	return jsonStr, nil
}

func postToQueue(queueName string, data string) {
	queue := mq.New(queueName)

	id, err := queue.PushString(data)

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("ID is ", id)
	}
}

func main() {
	http.HandleFunc("/register", register)
	http.HandleFunc("/unregister", unregister)
	http.ListenAndServe(getPort(), nil)
}

// Get the Port from the environment so we can run on Heroku
func getPort() string {
	var port = os.Getenv("PORT")
	// Set a default port if there is nothing in the environment
	if port == "" {
		port = "7777"
		fmt.Println("INFO: No PORT environment variable detected, defaulting to " + port)
	}
	return ":" + port
}
