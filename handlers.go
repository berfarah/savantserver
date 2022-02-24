package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type State struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func ReadState(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stateNames := strings.Split(vars["names"], ",")

	stateValues, err := scliClient.Run("readstate", stateNames...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	out := make([]State, 0, len(stateNames))
	for i, stateName := range stateNames {
		out = append(out, State{Name: stateName, Value: stateValues[i]})
	}

	serveJson(w, http.StatusOK, out)
}

func WriteState(w http.ResponseWriter, r *http.Request) {
	var states []State
	if err := json.NewDecoder(r.Body).Decode(&states); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	statePairs := make([]string, 0, len(states)*2)
	for _, state := range states {
		statePairs = append(statePairs, state.Name, state.Value)
	}

	if _, err := scliClient.Run("writeState", statePairs...); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func StateNames(w http.ResponseWriter, r *http.Request) {
	stateNames, err := scliClient.Run("statenames")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	serveJson(w, http.StatusOK, stateNames)
}

type ServiceRequestInput struct {
	Zone                   string            `json:"zone"`
	SourceComponent        string            `json:"sourceComponent"`
	SourceLogicalComponent string            `json:"sourceLogicalComponent"`
	ServiceVariant         string            `json:"serviceVariant"`
	ServiceType            string            `json:"serviceType"`
	Request                string            `json:"request"`
	RequestArgs            map[string]string `json:"args"`
}

// servicerequest <zone> <source component> <source logical component> <service variant>
// <service type> <request> [<key> <value>]...
func ServiceRequest(w http.ResponseWriter, r *http.Request) {
	var body ServiceRequestInput
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	args := []string{
		body.Zone,
		body.SourceComponent,
		body.SourceLogicalComponent,
		body.ServiceVariant,
		body.ServiceType,
		body.Request,
	}
	for key, value := range body.RequestArgs {
		args = append(args, key, value)
	}

	if _, err := scliClient.Run("servicerequest", args...); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func UserZones(w http.ResponseWriter, r *http.Request) {
	zones, err := scliClient.Run("userzones")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	serveJson(w, http.StatusOK, zones)
}

func ServicesForZone(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	services, err := scliClient.Run("servicesforzone", vars["name"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	serveJson(w, http.StatusOK, services)
}

type Scene struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	User string `json:"user"`
}

func GetSceneNames(w http.ResponseWriter, r *http.Request) {
	sceneRows, err := scliClient.Run("getSceneNames")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scenes := make([]Scene, 0, len(sceneRows))
	for _, sceneRow := range sceneRows {
		if sceneRow == "" {
			continue
		}
		elements := strings.Split(sceneRow, ",")
		scenes = append(scenes, Scene{
			Name: elements[0],
			ID:   elements[1],
			User: elements[2],
		})
	}

	serveJson(w, http.StatusOK, scenes)
}

func ActivateScene(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if _, err := scliClient.Run("activateScene", vars["name"], vars["id"], vars["user"]); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func RemoveScene(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if _, err := scliClient.Run("removeScene", vars["name"], vars["id"], vars["user"]); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
