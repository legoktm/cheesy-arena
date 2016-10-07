// Copyright 2014 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Configuration and functions for the event server web interface.

package main

import (
	"bitbucket.org/rj/httpauth-go"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"text/template"
)

const httpPort = 8080
const adminUser = "admin"
const readerUser = "reader"

var websocketUpgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 2014}
var adminAuth = httpauth.NewBasic("Cheesy Arena", checkAdminPassword, nil)
var readerAuth = httpauth.NewBasic("Cheesy Arena", checkReaderPassword, nil)

// Helper functions that can be used inside templates.
var templateHelpers = template.FuncMap{
	// Allows sub-templates to be invoked with multiple arguments.
	"dict": func(values ...interface{}) (map[string]interface{}, error) {
		if len(values)%2 != 0 {
			return nil, fmt.Errorf("Invalid dict call.")
		}
		dict := make(map[string]interface{}, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil, fmt.Errorf("Dict keys must be strings.")
			}
			dict[key] = values[i+1]
		}
		return dict, nil
	},
}

// Wraps the Gorilla Websocket module so that we can define additional functions on it.
type Websocket struct {
	conn       *websocket.Conn
	writeMutex *sync.Mutex
}

type WebsocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Upgrades the given HTTP request to a websocket connection.
func NewWebsocket(w http.ResponseWriter, r *http.Request) (*Websocket, error) {
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return &Websocket{conn, new(sync.Mutex)}, nil
}

func (websocket *Websocket) Close() {
	websocket.conn.Close()
}

func (websocket *Websocket) Read() (string, interface{}, error) {
	var message WebsocketMessage
	err := websocket.conn.ReadJSON(&message)
	return message.Type, message.Data, err
}

func (websocket *Websocket) Write(messageType string, data interface{}) error {
	websocket.writeMutex.Lock()
	defer websocket.writeMutex.Unlock()
	return websocket.conn.WriteJSON(WebsocketMessage{messageType, data})
}

func (websocket *Websocket) WriteError(errorMessage string) error {
	websocket.writeMutex.Lock()
	defer websocket.writeMutex.Unlock()
	return websocket.conn.WriteJSON(WebsocketMessage{"error", errorMessage})
}

func (websocket *Websocket) ShowDialog(message string) error {
	websocket.writeMutex.Lock()
	defer websocket.writeMutex.Unlock()
	return websocket.conn.WriteJSON(WebsocketMessage{"dialog", message})
}

// Serves the root page of Cheesy Arena.
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	template, err := template.ParseFiles("templates/index.html", "templates/base.html")
	if err != nil {
		handleWebErr(w, err)
		return
	}
	data := struct {
		*EventSettings
	}{eventSettings}
	err = template.ExecuteTemplate(w, "base", data)
	if err != nil {
		handleWebErr(w, err)
		return
	}
}

// Starts the webserver and blocks, waiting on requests. Does not return until the application exits.
func ServeWebInterface() {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	http.Handle("/", newHandler())
	log.Printf("Serving HTTP requests on port %d", httpPort)

	// Start Server
	http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil)
}

// Returns true if the given user is authorized for admin operations. Used for HTTP Basic Auth.
func UserIsAdmin(w http.ResponseWriter, r *http.Request) bool {
	if eventSettings.AdminPassword == "" {
		// Disable auth if there is no password configured.
		return true
	}
	if adminAuth.Authorize(r) == "" {
		adminAuth.NotifyAuthRequired(w, r)
		return false
	}
	return true
}

// Returns true if the given user is authorized for read-only operations. Used for HTTP Basic Auth.
func UserIsReader(w http.ResponseWriter, r *http.Request) bool {
	if eventSettings.ReaderPassword == "" {
		// Disable auth if there is no password configured.
		return true
	}
	if readerAuth.Authorize(r) == "" {
		readerAuth.NotifyAuthRequired(w, r)
		return false
	}
	return true
}

func checkAdminPassword(user, password string) bool {
	return user == adminUser && password == eventSettings.AdminPassword
}

func checkReaderPassword(user, password string) bool {
	if user == readerUser {
		return password == eventSettings.ReaderPassword
	}

	// The admin role also has read permissions.
	return checkAdminPassword(user, password)
}

// Sets up the mapping between URLs and handlers.
func newHandler() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/setup/settings", SettingsGetHandler).Methods("GET")
	router.HandleFunc("/setup/settings", SettingsPostHandler).Methods("POST")
	router.HandleFunc("/setup/db/save", SaveDbHandler).Methods("GET")
	router.HandleFunc("/setup/db/restore", RestoreDbHandler).Methods("POST")
	router.HandleFunc("/setup/db/clear", ClearDbHandler).Methods("POST")
	router.HandleFunc("/setup/teams", TeamsGetHandler).Methods("GET")
	router.HandleFunc("/setup/teams", TeamsPostHandler).Methods("POST")
	router.HandleFunc("/setup/teams/clear", TeamsClearHandler).Methods("POST")
	router.HandleFunc("/setup/teams/{id}/edit", TeamEditGetHandler).Methods("GET")
	router.HandleFunc("/setup/teams/{id}/edit", TeamEditPostHandler).Methods("POST")
	router.HandleFunc("/setup/teams/{id}/delete", TeamDeletePostHandler).Methods("POST")
	router.HandleFunc("/setup/teams/publish", TeamsPublishHandler).Methods("POST")
	router.HandleFunc("/setup/teams/generate_wpa_keys", TeamsGenerateWpaKeysHandler).Methods("GET")
	router.HandleFunc("/setup/schedule", ScheduleGetHandler).Methods("GET")
	router.HandleFunc("/setup/schedule/generate", ScheduleGeneratePostHandler).Methods("POST")
	router.HandleFunc("/setup/schedule/republish", ScheduleRepublishPostHandler).Methods("POST")
	router.HandleFunc("/setup/schedule/save", ScheduleSavePostHandler).Methods("POST")
	router.HandleFunc("/setup/alliance_selection", AllianceSelectionGetHandler).Methods("GET")
	router.HandleFunc("/setup/alliance_selection", AllianceSelectionPostHandler).Methods("POST")
	router.HandleFunc("/setup/alliance_selection/start", AllianceSelectionStartHandler).Methods("POST")
	router.HandleFunc("/setup/alliance_selection/reset", AllianceSelectionResetHandler).Methods("POST")
	router.HandleFunc("/setup/alliance_selection/finalize", AllianceSelectionFinalizeHandler).Methods("POST")
	router.HandleFunc("/setup/field", FieldGetHandler).Methods("GET")
	router.HandleFunc("/setup/field", FieldPostHandler).Methods("POST")
	router.HandleFunc("/setup/field/reload_displays", FieldReloadDisplaysHandler).Methods("GET")
	router.HandleFunc("/setup/field/lights", FieldLightsPostHandler).Methods("POST")
	router.HandleFunc("/setup/lower_thirds", LowerThirdsGetHandler).Methods("GET")
	router.HandleFunc("/setup/lower_thirds/websocket", LowerThirdsWebsocketHandler).Methods("GET")
	router.HandleFunc("/setup/sponsor_slides", SponsorSlidesGetHandler).Methods("GET")
	router.HandleFunc("/setup/sponsor_slides", SponsorSlidesPostHandler).Methods("POST")
	router.HandleFunc("/api/sponsor_slides", SponsorSlidesApiHandler).Methods("GET")
	router.HandleFunc("/setup/defense_selection", DefenseSelectionGetHandler).Methods("GET")
	router.HandleFunc("/setup/defense_selection", DefenseSelectionPostHandler).Methods("POST")
	router.HandleFunc("/match_play", MatchPlayHandler).Methods("GET")
	router.HandleFunc("/match_play/{matchId}/load", MatchPlayLoadHandler).Methods("GET")
	router.HandleFunc("/match_play/{matchId}/show_result", MatchPlayShowResultHandler).Methods("GET")
	router.HandleFunc("/match_play/websocket", MatchPlayWebsocketHandler).Methods("GET")
	router.HandleFunc("/match_review", MatchReviewHandler).Methods("GET")
	router.HandleFunc("/match_review/{matchId}/edit", MatchReviewEditGetHandler).Methods("GET")
	router.HandleFunc("/match_review/{matchId}/edit", MatchReviewEditPostHandler).Methods("POST")
	router.HandleFunc("/reports/csv/rankings", RankingsCsvReportHandler).Methods("GET")
	router.HandleFunc("/reports/pdf/rankings", RankingsPdfReportHandler).Methods("GET")
	router.HandleFunc("/reports/csv/schedule/{type}", ScheduleCsvReportHandler).Methods("GET")
	router.HandleFunc("/reports/pdf/schedule/{type}", SchedulePdfReportHandler).Methods("GET")
	router.HandleFunc("/reports/pdf/defenses/{type}", DefensesPdfReportHandler).Methods("GET")
	router.HandleFunc("/reports/csv/teams", TeamsCsvReportHandler).Methods("GET")
	router.HandleFunc("/reports/pdf/teams", TeamsPdfReportHandler).Methods("GET")
	router.HandleFunc("/reports/csv/wpa_keys", WpaKeysCsvReportHandler).Methods("GET")
	router.HandleFunc("/displays/audience", AudienceDisplayHandler).Methods("GET")
	router.HandleFunc("/displays/audience/websocket", AudienceDisplayWebsocketHandler).Methods("GET")
	router.HandleFunc("/displays/pit", PitDisplayHandler).Methods("GET")
	router.HandleFunc("/displays/pit/websocket", PitDisplayWebsocketHandler).Methods("GET")
	router.HandleFunc("/displays/announcer", AnnouncerDisplayHandler).Methods("GET")
	router.HandleFunc("/displays/announcer/websocket", AnnouncerDisplayWebsocketHandler).Methods("GET")
	router.HandleFunc("/displays/scoring/{alliance}", ScoringDisplayHandler).Methods("GET")
	router.HandleFunc("/displays/scoring/{alliance}/websocket", ScoringDisplayWebsocketHandler).Methods("GET")
	router.HandleFunc("/displays/referee", RefereeDisplayHandler).Methods("GET")
	router.HandleFunc("/displays/referee/websocket", RefereeDisplayWebsocketHandler).Methods("GET")
	router.HandleFunc("/displays/alliance_station", AllianceStationDisplayHandler).Methods("GET")
	router.HandleFunc("/displays/alliance_station/websocket", AllianceStationDisplayWebsocketHandler).Methods("GET")
	router.HandleFunc("/displays/fta", FtaDisplayHandler).Methods("GET")
	router.HandleFunc("/displays/fta/websocket", FtaDisplayWebsocketHandler).Methods("GET")
	router.HandleFunc("/api/matches/{type}", MatchesApiHandler).Methods("GET")
	router.HandleFunc("/api/rankings", RankingsApiHandler).Methods("GET")
	router.HandleFunc("/", IndexHandler).Methods("GET")
	return router
}

// Writes the given error out as plain text with a status code of 500.
func handleWebErr(w http.ResponseWriter, err error) {
	http.Error(w, "Internal server error: "+err.Error(), 500)
}
