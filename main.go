package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hashicorp/memberlist"
)

func WebsocketConfig(transport *Transport, d *delegate) *memberlist.Config {
	transport.logger = log.New(os.Stderr, "", log.LstdFlags)
	d.logger = transport.logger

	conf := memberlist.DefaultLocalConfig()
	conf.Transport = transport
	conf.BindPort = transport.Port
	conf.Delegate = d
	conf.Events = d
	conf.Logger = transport.logger

	return conf
}

func main() {
	me, err := os.Hostname()
	if err != nil {
		panic("Failed to get hostname: " + err.Error())
	}

	log.Printf("[INFO] app: hostname=%v", me)

	d := new(delegate)
	transport := NewTransport("0.0.0.0", 8080)

	conf := WebsocketConfig(transport, d)

	list, err := memberlist.Create(conf)
	if err != nil {
		panic(err)
	}
	d.Start(list)

	http.HandleFunc(transport.Route, transport.Handler)

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "<h1>%s (%d)</h1>\n", me, list.GetHealthScore())

		fmt.Fprintf(w, "<ul>\n")
		for _, member := range list.Members() {
			fmt.Fprintf(w, "<li>%s @ %s</li>", member.Name, member.Addr)
		}
		fmt.Fprintf(w, "</ul>\n")
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
