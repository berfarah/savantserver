package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

var port string

func init() {
	flag.StringVar(&port, "port", "12000", "")
	flag.Parse()
}

func main() {
	fmt.Println("Starting up")

	r := mux.NewRouter()
	// readstate
	r.HandleFunc("/states", ReadState).Methods("GET").Queries("names", "{names}")
	// writestate
	r.HandleFunc("/states", WriteState).Methods("POST")
	// statenames <filter>
	r.HandleFunc("/states/names", StateNames).Methods("GET")
	// servicerequest
	r.HandleFunc("/servicerequest", ServiceRequest).Methods("POST")
	// userzones <zone>
	r.HandleFunc("/zones", UserZones).Methods("GET")
	// servicesforzone <service>
	r.HandleFunc("/zones/{name}/services", ServicesForZone).Methods("GET")

	// TODO:
	// // settrigger <triggername> <transitionCount> <Match Entry Type> <Match Entry State Scope>
	// // <Match Entry State Name> <Match Logic> <Match Data> <prematchCount> <Match Entry Type> <Match
	// // Entry State Scope> <Match Entry State Name> <Match Logic> <Match Data> <serviceZone>
	// // <serviceSourceComponent> <serviceLogicalName> <serviceVariantID> <serviceType> <serviceReq>
	// // [<serviceReqFirstArg> <serviceReqSecondArg>]...
	// r.HandleFunc("/trigger", ServiceRequest).Methods("POST")
	// // removetrigger <triggername>
	// r.HandleFunc("/trigger/{name}", ServiceRequest).Methods("DELETE")

	// // getSceneNames <scene name>,<scene id>,<scene user>
	r.HandleFunc("/scenes", GetSceneNames).Methods("GET")
	// // activateScene <scene name> <scene id> <scene user>
	r.HandleFunc("/scenes/{name}-{id}-{user}/activate", ActivateScene).Methods("POST")
	// // removeScene <scene name> <scene id> <scene user>
	r.HandleFunc("/scenes/{name}-{id}-{user}", RemoveScene).Methods("DELETE")

	if err := http.ListenAndServe(":"+port, r); err != nil {
		panic(err)
	}
}
