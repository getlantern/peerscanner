package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/iron-io/iron_go/mq"
)

type Reg struct {
	Name string
	Ip   string
	Port int
}

func register(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	fmt.Println("Read name: ", name)
	ip := r.FormValue("ip")
	portString := r.FormValue("port")

	port, err := strconv.Atoi(portString)
	if err != nil {
		// handle error
		fmt.Println(err)
		return
	}
	if port != 443 {
		fmt.Println("Ignoring port not on 443")
		return
	}
	reg := &Reg{name, ip, port}

	json, err := json.Marshal(reg)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(json))
	queue := mq.New("peers")

	id, err := queue.PushString(string(json))

	fmt.Println("ID is ", id)
}

func unregister(w http.ResponseWriter, r *http.Request) {

}

func main() {
	http.HandleFunc("/register/", register)
	http.HandleFunc("/unregister/", unregister)
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
