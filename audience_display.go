// Copyright 2014 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Web handlers for audience screen display.

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"text/template"
)

type RankingAlliance struct {
	Team       AllianceTeam
	AllianceId int

	Team1  int
	Team2  int
	Team3  int
	Score  int
	Played int
}

// Renders the audience display to be chroma keyed over the video feed.
func AudienceDisplayHandler(w http.ResponseWriter, r *http.Request) {
	if !UserIsReader(w, r) {
		return
	}

	template := template.New("").Funcs(templateHelpers)
	_, err := template.ParseFiles("templates/audience_display.html")
	if err != nil {
		handleWebErr(w, err)
		return
	}

	data := struct {
		*EventSettings
	}{eventSettings}
	err = template.ExecuteTemplate(w, "audience_display.html", data)
	if err != nil {
		handleWebErr(w, err)
		return
	}
}

func updateRankingAllianceScore(alliances []RankingAlliance, teamId int, team2 int, team3 int, score int) {
	theRealIndex := 0
	for index, alliance := range alliances {
		if alliance.Team.TeamId == teamId || alliance.Team.TeamId == team2 || alliance.Team.TeamId == team3 {
			theRealIndex = index
			// alliance.Score = alliance.Score + score
			// return
		}
	}
	fmt.Println(alliances[theRealIndex])
	fmt.Println("old")
	fmt.Println(alliances[theRealIndex].Score)
	alliances[theRealIndex].Score += score
	alliances[theRealIndex].Played++
	fmt.Println("new")
	fmt.Println(alliances[theRealIndex].Score)
}

func GetLatestRankings(database *Database) ([]RankingAlliance, Match) {
	matches, _ := database.GetMatchesByType("elimination")
	var last Match
	for _, match := range matches {
		if match.Status == "complete" {
			last = match
		}
	}

	if strings.HasPrefix(last.DisplayName, "SF") {
		return GetRankingsForRound("SF", database), last
	} else if strings.HasPrefix(last.DisplayName, "QF") {
		return GetRankingsForRound("QF", database), last
	} else {
		return []RankingAlliance{}, last
	}
}

func GetRankingsForRound(round string, database *Database) []RankingAlliance {
	var alliances = make([]RankingAlliance, 8)
	matches, _ := database.GetMatchesByType("elimination")
	allAlliances, _ := database.GetAllAlliances()
	for _, at := range allAlliances {
		for _, allianceTeam := range at {
			alliTeams := database.GetTeamsByAlliance(allianceTeam.AllianceId)
			alliances[allianceTeam.AllianceId-1] = RankingAlliance{
				allianceTeam,
				allianceTeam.AllianceId,
				alliTeams[0].TeamId,
				alliTeams[1].TeamId,
				alliTeams[2].TeamId,
				0,
				0}
		}
	}

	for _, match := range matches {
		if strings.HasPrefix(match.DisplayName, round) && match.Status == "complete" {
			result, _ := database.GetMatchResultForMatch(match.Id)
			//fmt.Println(match.Id)
			result.CorrectEliminationScore()
			fmt.Println("red: " + strconv.Itoa(match.Red1) + "," + strconv.Itoa(match.Red2) + "," + strconv.Itoa(match.Red3))
			updateRankingAllianceScore(alliances, match.Red1, match.Red2, match.Red3, result.RedScoreSummary().Score)
			//fmt.Println(result.RedScoreSummary().Score)
			fmt.Println("blue: " + strconv.Itoa(match.Blue1) + "," + strconv.Itoa(match.Blue2) + "," + strconv.Itoa(match.Blue3))
			updateRankingAllianceScore(alliances, match.Blue1, match.Blue2, match.Blue3, result.BlueScoreSummary().Score)
			//fmt.Println(result.BlueScoreSummary().Score)

		}
	}
	return alliances
}

// The websocket endpoint for the audience display client to receive status updates.
func AudienceDisplayWebsocketHandler(w http.ResponseWriter, r *http.Request) {
	if !UserIsReader(w, r) {
		return
	}

	websocket, err := NewWebsocket(w, r)
	if err != nil {
		handleWebErr(w, err)
		return
	}
	defer websocket.Close()

	audienceDisplayListener := mainArena.audienceDisplayNotifier.Listen()
	defer close(audienceDisplayListener)
	matchLoadTeamsListener := mainArena.matchLoadTeamsNotifier.Listen()
	defer close(matchLoadTeamsListener)
	matchTimeListener := mainArena.matchTimeNotifier.Listen()
	defer close(matchTimeListener)
	realtimeScoreListener := mainArena.realtimeScoreNotifier.Listen()
	defer close(realtimeScoreListener)
	scorePostedListener := mainArena.scorePostedNotifier.Listen()
	defer close(scorePostedListener)
	elimRankingsUpdatedListener := mainArena.elimRankingsUpdatedNotifier.Listen()
	defer close(elimRankingsUpdatedListener)
	playSoundListener := mainArena.playSoundNotifier.Listen()
	defer close(playSoundListener)
	allianceSelectionListener := mainArena.allianceSelectionNotifier.Listen()
	defer close(allianceSelectionListener)
	lowerThirdListener := mainArena.lowerThirdNotifier.Listen()
	defer close(lowerThirdListener)
	reloadDisplaysListener := mainArena.reloadDisplaysNotifier.Listen()
	defer close(reloadDisplaysListener)

	// Send the various notifications immediately upon connection.
	var data interface{}
	err = websocket.Write("matchTiming", mainArena.matchTiming)
	if err != nil {
		log.Printf("Websocket error: %s", err)
		return
	}
	err = websocket.Write("matchTime", MatchTimeMessage{mainArena.MatchState, int(mainArena.lastMatchTimeSec)})
	if err != nil {
		log.Printf("Websocket error: %s", err)
		return
	}
	err = websocket.Write("setAudienceDisplay", mainArena.audienceDisplayScreen)
	if err != nil {
		log.Printf("Websocket error: %s", err)
		return
	}
	data = struct {
		Match     *Match
		MatchName string
	}{mainArena.currentMatch, mainArena.currentMatch.CapitalizedType()}
	err = websocket.Write("setMatch", data)
	if err != nil {
		log.Printf("Websocket error: %s", err)
		return
	}
	data = struct {
		RedScore  int
		BlueScore int
	}{mainArena.redRealtimeScore.Score(), mainArena.blueRealtimeScore.Score()}
	err = websocket.Write("realtimeScore", data)
	if err != nil {
		log.Printf("Websocket error: %s", err)
		return
	}
	data = struct {
		Match     *Match
		MatchName string
		RedScore  *ScoreSummary
		BlueScore *ScoreSummary
	}{mainArena.savedMatch, mainArena.savedMatch.CapitalizedType(),
		mainArena.savedMatchResult.RedScoreSummary(), mainArena.savedMatchResult.BlueScoreSummary()}
	fmt.Println(mainArena.savedMatch)
	fmt.Println(data)
	err = websocket.Write("setFinalScore", data)
	if err != nil {
		log.Printf("Websocket error: %s", err)
		return
	}
	latestRankings, lastMatch := GetLatestRankings(db)
	data = struct {
		LastRoundName string
		Rankings []RankingAlliance
	}{lastMatch.DisplayName, latestRankings}
	err = websocket.Write("elimRankingsUpdated", data)
	if err != nil {
		log.Printf("Websocket error: %s", err)
	}
	err = websocket.Write("allianceSelection", cachedAlliances)
	if err != nil {
		log.Printf("Websocket error: %s", err)
		return
	}

	// Spin off a goroutine to listen for notifications and pass them on through the websocket.
	go func() {
		for {
			var messageType string
			var message interface{}
			select {
			case _, ok := <-audienceDisplayListener:
				if !ok {
					return
				}
				messageType = "setAudienceDisplay"
				message = mainArena.audienceDisplayScreen
			case _, ok := <-matchLoadTeamsListener:
				if !ok {
					return
				}
				messageType = "setMatch"
				message = struct {
					Match     *Match
					MatchName string
				}{mainArena.currentMatch, mainArena.currentMatch.CapitalizedType()}
			case matchTimeSec, ok := <-matchTimeListener:
				if !ok {
					return
				}
				messageType = "matchTime"
				message = MatchTimeMessage{mainArena.MatchState, matchTimeSec.(int)}
			case _, ok := <-realtimeScoreListener:
				if !ok {
					return
				}
				messageType = "realtimeScore"
				message = struct {
					RedScore  int
					BlueScore int
				}{mainArena.redRealtimeScore.Score(), mainArena.blueRealtimeScore.Score()}
			case _, ok := <-elimRankingsUpdatedListener:
				if !ok {
					return
				}
				messageType = "elimRankingsUpdated"
				latestRankings, lastMatch := GetLatestRankings(db)
				message = struct {
					LastRoundName string
					Rankings []RankingAlliance
				}{lastMatch.DisplayName, latestRankings}
				fmt.Println(message)
			case _, ok := <-scorePostedListener:
				if !ok {
					return
				}
				messageType = "setFinalScore"
				message = struct {
					Match     *Match
					MatchName string
					RedScore  *ScoreSummary
					BlueScore *ScoreSummary
				}{mainArena.savedMatch, mainArena.savedMatch.CapitalizedType(),
					mainArena.savedMatchResult.RedScoreSummary(), mainArena.savedMatchResult.BlueScoreSummary()}
			case sound, ok := <-playSoundListener:
				if !ok {
					return
				}
				messageType = "playSound"
				message = sound
			case _, ok := <-allianceSelectionListener:
				if !ok {
					return
				}
				messageType = "allianceSelection"
				message = cachedAlliances
			case lowerThird, ok := <-lowerThirdListener:
				if !ok {
					return
				}
				messageType = "lowerThird"
				message = lowerThird
			case _, ok := <-reloadDisplaysListener:
				if !ok {
					return
				}
				messageType = "reload"
				message = nil
			}
			err = websocket.Write(messageType, message)
			if err != nil {
				// The client has probably closed the connection; nothing to do here.
				return
			}
		}
	}()

	// Loop, waiting for commands and responding to them, until the client closes the connection.
	for {
		_, _, err := websocket.Read()
		if err != nil {
			if err == io.EOF {
				// Client has closed the connection; nothing to do here.
				return
			}
			log.Printf("Websocket error: %s", err)
			return
		}
	}
}
