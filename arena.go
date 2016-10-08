// Copyright 2014 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Functions for controlling the arena and match play.

package main

import (
	"fmt"
	"log"
	"time"
)

const (
	arenaLoopPeriodMs     = 10
	dsPacketPeriodMs      = 250
	matchEndScoreDwellSec = 3
)

// Progression of match states.
const (
	PRE_MATCH      = 0
	START_MATCH    = 1
	AUTO_PERIOD    = 2
	PAUSE_PERIOD   = 3
	TELEOP_PERIOD  = 4
	ENDGAME_PERIOD = 5
	POST_MATCH     = 6
)

type AllianceStation struct {
	DsConn        *DriverStationConnection
	EmergencyStop bool
	Bypass        bool
	team          *Team
}

// Match period timings.
type MatchTiming struct {
	AutoDurationSec    int
	PauseDurationSec   int
	TeleopDurationSec  int
	EndgameTimeLeftSec int
}

type RealtimeScore struct {
	CurrentScore    Score
	Cards           map[string]string
	AutoCommitted   bool
	TeleopCommitted bool
	FoulsCommitted  bool
}

type Arena struct {
	AllianceStations               map[string]*AllianceStation
	MatchState                     int
	CanStartMatch                  bool
	matchTiming                    MatchTiming
	currentMatch                   *Match
	redRealtimeScore               *RealtimeScore
	blueRealtimeScore              *RealtimeScore
	matchStartTime                 time.Time
	lastDsPacketTime               time.Time
	matchStateNotifier             *Notifier
	matchTimeNotifier              *Notifier
	robotStatusNotifier            *Notifier
	matchLoadTeamsNotifier         *Notifier
	scoringStatusNotifier          *Notifier
	realtimeScoreNotifier          *Notifier
	scorePostedNotifier            *Notifier
	audienceDisplayNotifier        *Notifier
	playSoundNotifier              *Notifier
	allianceStationDisplayNotifier *Notifier
	allianceSelectionNotifier      *Notifier
	lowerThirdNotifier             *Notifier
	reloadDisplaysNotifier         *Notifier
	defenseSelectionNotifier       *Notifier
	audienceDisplayScreen          string
	allianceStationDisplays        map[string]string
	allianceStationDisplayScreen   string
	lastMatchState                 int
	lastMatchTimeSec               float64
	savedMatch                     *Match
	savedMatchResult               *MatchResult
	lights                         Lights
	muteMatchSounds                bool
	fieldReset                     bool
}

type RealtimeScoreFields struct {
	Score            int
	TowerStrength    int
	DefensesStrength [5]int
}

var mainArena Arena // Named thusly to avoid polluting the global namespace with something more generic.

func NewRealtimeScore() *RealtimeScore {
	realtimeScore := new(RealtimeScore)
	realtimeScore.Cards = make(map[string]string)
	return realtimeScore
}

// Sets the arena to its initial state.
func (arena *Arena) Setup() {
	arena.matchTiming.AutoDurationSec = 15
	arena.matchTiming.PauseDurationSec = 2
	arena.matchTiming.TeleopDurationSec = 135
	arena.matchTiming.EndgameTimeLeftSec = 20

	arena.AllianceStations = make(map[string]*AllianceStation)
	arena.AllianceStations["R1"] = new(AllianceStation)
	arena.AllianceStations["R2"] = new(AllianceStation)
	arena.AllianceStations["R3"] = new(AllianceStation)
	arena.AllianceStations["B1"] = new(AllianceStation)
	arena.AllianceStations["B2"] = new(AllianceStation)
	arena.AllianceStations["B3"] = new(AllianceStation)

	arena.matchStateNotifier = NewNotifier()
	arena.matchTimeNotifier = NewNotifier()
	arena.robotStatusNotifier = NewNotifier()
	arena.matchLoadTeamsNotifier = NewNotifier()
	arena.scoringStatusNotifier = NewNotifier()
	arena.realtimeScoreNotifier = NewNotifier()
	arena.scorePostedNotifier = NewNotifier()
	arena.audienceDisplayNotifier = NewNotifier()
	arena.playSoundNotifier = NewNotifier()
	arena.allianceStationDisplayNotifier = NewNotifier()
	arena.allianceSelectionNotifier = NewNotifier()
	arena.lowerThirdNotifier = NewNotifier()
	arena.reloadDisplaysNotifier = NewNotifier()
	arena.defenseSelectionNotifier = NewNotifier()

	arena.lights.Setup()

	// Load empty match as current.
	arena.MatchState = PRE_MATCH
	arena.LoadTestMatch()
	arena.lastMatchState = -1
	arena.lastMatchTimeSec = 0

	// Initialize display parameters.
	arena.audienceDisplayScreen = "blank"
	arena.savedMatch = &Match{}
	arena.savedMatchResult = &MatchResult{}
	arena.allianceStationDisplays = make(map[string]string)
	arena.allianceStationDisplayScreen = "match"
}

// Loads a team into an alliance station, cleaning up the previous team there if there is one.
func (arena *Arena) AssignTeam(teamId int, station string) error {
	// Reject invalid station values.
	if _, ok := arena.AllianceStations[station]; !ok {
		return fmt.Errorf("Invalid alliance station '%s'.", station)
	}

	// Do nothing if the station is already assigned to the requested team.
	dsConn := arena.AllianceStations[station].DsConn
	if dsConn != nil && dsConn.TeamId == teamId {
		return nil
	}
	if dsConn != nil {
		dsConn.Close()
		arena.AllianceStations[station].team = nil
		arena.AllianceStations[station].DsConn = nil
	}

	// Leave the station empty if the team number is zero.
	if teamId == 0 {
		return nil
	}

	// Load the team model. If it doesn't exist, enable anonymous operation.
	team, err := db.GetTeamById(teamId)
	if err != nil {
		return err
	}
	if team == nil {
		team = &Team{Id: teamId}
	}

	arena.AllianceStations[station].team = team
	return nil
}

// Sets up the arena for the given match.
func (arena *Arena) LoadMatch(match *Match) error {
	if arena.MatchState != PRE_MATCH {
		return fmt.Errorf("Cannot load match while there is a match still in progress or with results pending.")
	}

	arena.currentMatch = match
	err := arena.AssignTeam(match.Red1, "R1")
	if err != nil {
		return err
	}
	err = arena.AssignTeam(match.Red2, "R2")
	if err != nil {
		return err
	}
	err = arena.AssignTeam(match.Red3, "R3")
	if err != nil {
		return err
	}
	err = arena.AssignTeam(match.Blue1, "B1")
	if err != nil {
		return err
	}
	err = arena.AssignTeam(match.Blue2, "B2")
	if err != nil {
		return err
	}
	err = arena.AssignTeam(match.Blue3, "B3")
	if err != nil {
		return err
	}

	arena.SetupNetwork()

	// Reset the realtime scores.
	arena.redRealtimeScore = NewRealtimeScore()
	arena.blueRealtimeScore = NewRealtimeScore()
	arena.fieldReset = false
	arena.lights.ClearAll()

	// Notify any listeners about the new match.
	arena.matchLoadTeamsNotifier.Notify(nil)
	arena.realtimeScoreNotifier.Notify(nil)
	arena.allianceStationDisplayScreen = "match"
	arena.allianceStationDisplayNotifier.Notify(nil)
	arena.defenseSelectionNotifier.Notify(nil)

	return nil
}

// Sets a new test match containing no teams as the current match.
func (arena *Arena) LoadTestMatch() error {
	return arena.LoadMatch(&Match{Type: "test"})
}

// Loads the first unplayed match of the current match type.
func (arena *Arena) LoadNextMatch() error {
	if arena.currentMatch.Type == "test" {
		return arena.LoadTestMatch()
	}

	matches, err := db.GetMatchesByType(arena.currentMatch.Type)
	if err != nil {
		return err
	}
	for _, match := range matches {
		if match.Status != "complete" {
			err = arena.LoadMatch(&match)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// Assigns the given team to the given station, also substituting it into the match record.
func (arena *Arena) SubstituteTeam(teamId int, station string) error {
	if arena.currentMatch.Type == "qualification" {
		return fmt.Errorf("Can't substitute teams for qualification matches.")
	}
	err := arena.AssignTeam(teamId, station)
	if err != nil {
		return err
	}
	switch station {
	case "R1":
		arena.currentMatch.Red1 = teamId
	case "R2":
		arena.currentMatch.Red2 = teamId
	case "R3":
		arena.currentMatch.Red3 = teamId
	case "B1":
		arena.currentMatch.Blue1 = teamId
	case "B2":
		arena.currentMatch.Blue2 = teamId
	case "B3":
		arena.currentMatch.Blue3 = teamId
	}
	arena.SetupNetwork()
	arena.matchLoadTeamsNotifier.Notify(nil)
	return nil
}

// Asynchronously reconfigures the networking hardware for the new set of teams.
func (arena *Arena) SetupNetwork() {
	if eventSettings.NetworkSecurityEnabled {
		go func() {
			err := ConfigureTeamWifi(arena.AllianceStations["R1"].team, arena.AllianceStations["R2"].team,
				arena.AllianceStations["R3"].team, arena.AllianceStations["B1"].team,
				arena.AllianceStations["B2"].team, arena.AllianceStations["B3"].team)
			if err != nil {
				log.Printf("Failed to configure team WiFi: %s", err.Error())
			}
		}()
		go func() {
			err := ConfigureTeamEthernet(arena.AllianceStations["R1"].team, arena.AllianceStations["R2"].team,
				arena.AllianceStations["R3"].team, arena.AllianceStations["B1"].team,
				arena.AllianceStations["B2"].team, arena.AllianceStations["B3"].team)
			if err != nil {
				log.Printf("Failed to configure team Ethernet: %s", err.Error())
			}
		}()
	}
}

// Returns nil if the match can be started, and an error otherwise.
func (arena *Arena) CheckCanStartMatch() error {
	if arena.MatchState != PRE_MATCH {
		return fmt.Errorf("Cannot start match while there is a match still in progress or with results pending.")
	}
	for _, allianceStation := range arena.AllianceStations {
		if allianceStation.EmergencyStop {
			return fmt.Errorf("Cannot start match while an emergency stop is active.")
		}
		if !allianceStation.Bypass {
			if allianceStation.DsConn == nil || !allianceStation.DsConn.RobotLinked {
				return fmt.Errorf("Cannot start match until all robots are connected or bypassed.")
			}
		}
	}
	return nil
}

// Starts the match if all conditions are met.
func (arena *Arena) StartMatch() error {
	err := arena.CheckCanStartMatch()
	if err == nil {
		// Save the match start time to the database for posterity.
		arena.currentMatch.StartedAt = time.Now()
		if arena.currentMatch.Type != "test" {
			db.SaveMatch(arena.currentMatch)
		}

		// Save the missed packet count to subtract it from the running count.
		for _, allianceStation := range arena.AllianceStations {
			if allianceStation.DsConn != nil {
				err = allianceStation.DsConn.signalMatchStart(arena.currentMatch)
				if err != nil {
					log.Println(err)
				}
			}
		}

		arena.MatchState = START_MATCH
	}
	return err
}

// Kills the current match if it is underway.
func (arena *Arena) AbortMatch() error {
	if arena.MatchState == PRE_MATCH || arena.MatchState == POST_MATCH {
		return fmt.Errorf("Cannot abort match when it is not in progress.")
	}
	arena.MatchState = POST_MATCH
	arena.audienceDisplayScreen = "blank"
	arena.audienceDisplayNotifier.Notify(nil)
	if !arena.muteMatchSounds {
		arena.playSoundNotifier.Notify("match-abort")
	}
	return nil
}

// Clears out the match and resets the arena state unless there is a match underway.
func (arena *Arena) ResetMatch() error {
	if arena.MatchState != POST_MATCH && arena.MatchState != PRE_MATCH {
		return fmt.Errorf("Cannot reset match while it is in progress.")
	}
	arena.MatchState = PRE_MATCH
	arena.AllianceStations["R1"].Bypass = false
	arena.AllianceStations["R2"].Bypass = false
	arena.AllianceStations["R3"].Bypass = false
	arena.AllianceStations["B1"].Bypass = false
	arena.AllianceStations["B2"].Bypass = false
	arena.AllianceStations["B3"].Bypass = false
	arena.muteMatchSounds = false
	return nil
}

// Returns the fractional number of seconds since the start of the match.
func (arena *Arena) MatchTimeSec() float64 {
	if arena.MatchState == PRE_MATCH || arena.MatchState == START_MATCH || arena.MatchState == POST_MATCH {
		return 0
	} else {
		return time.Since(arena.matchStartTime).Seconds()
	}
}

// Performs a single iteration of checking inputs and timers and setting outputs accordingly to control the
// flow of a match.
func (arena *Arena) Update() {
	arena.CanStartMatch = arena.CheckCanStartMatch() == nil

	// Decide what state the robots need to be in, depending on where we are in the match.
	auto := false
	enabled := false
	sendDsPacket := false
	matchTimeSec := arena.MatchTimeSec()
	switch arena.MatchState {
	case PRE_MATCH:
		auto = true
		enabled = false
	case START_MATCH:
		arena.MatchState = AUTO_PERIOD
		arena.matchStartTime = time.Now()
		arena.lastMatchTimeSec = -1
		auto = true
		enabled = true
		sendDsPacket = true
		arena.audienceDisplayScreen = "match"
		arena.audienceDisplayNotifier.Notify(nil)
		if !arena.muteMatchSounds {
			arena.playSoundNotifier.Notify("match-start")
		}
	case AUTO_PERIOD:
		auto = true
		enabled = true
		if matchTimeSec >= float64(arena.matchTiming.AutoDurationSec) {
			arena.MatchState = PAUSE_PERIOD
			auto = false
			enabled = false
			sendDsPacket = true
			if !arena.muteMatchSounds {
				arena.playSoundNotifier.Notify("match-end")
			}
		}
	case PAUSE_PERIOD:
		auto = false
		enabled = false
		if matchTimeSec >= float64(arena.matchTiming.AutoDurationSec+arena.matchTiming.PauseDurationSec) {
			arena.MatchState = TELEOP_PERIOD
			auto = false
			enabled = true
			sendDsPacket = true
			if !arena.muteMatchSounds {
				arena.playSoundNotifier.Notify("match-resume")
			}
		}
	case TELEOP_PERIOD:
		auto = false
		enabled = true
		if matchTimeSec >= float64(arena.matchTiming.AutoDurationSec+arena.matchTiming.PauseDurationSec+
			arena.matchTiming.TeleopDurationSec-arena.matchTiming.EndgameTimeLeftSec) {
			arena.MatchState = ENDGAME_PERIOD
			sendDsPacket = false
			if !arena.muteMatchSounds {
				arena.playSoundNotifier.Notify("match-endgame")
			}
		}
	case ENDGAME_PERIOD:
		auto = false
		enabled = true
		if matchTimeSec >= float64(arena.matchTiming.AutoDurationSec+arena.matchTiming.PauseDurationSec+
			arena.matchTiming.TeleopDurationSec) {
			arena.MatchState = POST_MATCH
			auto = false
			enabled = false
			sendDsPacket = true
			go func() {
				// Leave the scores on the screen briefly at the end of the match.
				time.Sleep(time.Second * matchEndScoreDwellSec)
				arena.audienceDisplayScreen = "blank"
				arena.audienceDisplayNotifier.Notify(nil)
				arena.allianceStationDisplayScreen = "logo"
				arena.allianceStationDisplayNotifier.Notify(nil)
			}()
			if !arena.muteMatchSounds {
				arena.playSoundNotifier.Notify("match-end")
			}
		}
	}

	// Send a notification if the match state has changed.
	if arena.MatchState != arena.lastMatchState {
		arena.matchStateNotifier.Notify(arena.MatchState)
	}
	arena.lastMatchState = arena.MatchState

	// Send a match tick notification if passing an integer second threshold.
	if int(matchTimeSec) != int(arena.lastMatchTimeSec) {
		arena.matchTimeNotifier.Notify(int(matchTimeSec))
	}
	arena.lastMatchTimeSec = matchTimeSec

	// Send a packet if at a period transition point or if it's been long enough since the last one.
	if sendDsPacket || time.Since(arena.lastDsPacketTime).Seconds()*1000 >= dsPacketPeriodMs {
		arena.sendDsPacket(auto, enabled)
		arena.robotStatusNotifier.Notify(nil)
	}

	arena.handleLighting()
}

// Loops indefinitely to track and update the arena components.
func (arena *Arena) Run() {
	for {
		arena.Update()
		time.Sleep(time.Millisecond * arenaLoopPeriodMs)
	}
}

func (arena *Arena) sendDsPacket(auto bool, enabled bool) {
	for _, allianceStation := range arena.AllianceStations {
		dsConn := allianceStation.DsConn
		if dsConn != nil {
			dsConn.Auto = auto
			dsConn.Enabled = enabled && !allianceStation.EmergencyStop && !allianceStation.Bypass
			err := dsConn.Update()
			if err != nil {
				log.Printf("Unable to send driver station packet for team %d.", allianceStation.team.Id)
			}
		}
	}
	arena.lastDsPacketTime = time.Now()
}

// Calculates the integer score value for the given realtime snapshot.
func (realtimeScore *RealtimeScore) Score(opponentFouls []Foul) int {
	return scoreSummary(&realtimeScore.CurrentScore, opponentFouls, mainArena.currentMatch.Type).Score
}

// Calculates the integer score, tower strength, and defenses strength for the given realtime snapshot.
func (realtimeScore *RealtimeScore) ScoreFields(opponentFouls []Foul) *RealtimeScoreFields {
	scoreSummary := scoreSummary(&realtimeScore.CurrentScore, opponentFouls, mainArena.currentMatch.Type)
	var defensesStrength [5]int
	for i := 0; i < 5; i++ {
		defensesStrength[i] = 2 - realtimeScore.CurrentScore.AutoDefensesCrossed[i] -
			realtimeScore.CurrentScore.DefensesCrossed[i]
	}
	return &RealtimeScoreFields{scoreSummary.Score, scoreSummary.TowerStrength, defensesStrength}
}

// Manipulates the arena LED lighting based on the current state of the match.
func (arena *Arena) handleLighting() {
	switch arena.MatchState {
	case AUTO_PERIOD:
		fallthrough
	case PAUSE_PERIOD:
		fallthrough
	case TELEOP_PERIOD:
		fallthrough
	case ENDGAME_PERIOD:
		redScoreFields := arena.redRealtimeScore.ScoreFields(arena.blueRealtimeScore.CurrentScore.Fouls)
		blueScoreFields := arena.blueRealtimeScore.ScoreFields(arena.redRealtimeScore.CurrentScore.Fouls)
		arena.lights.SetDefenses(redScoreFields.DefensesStrength, blueScoreFields.DefensesStrength)
	case POST_MATCH:
		if mainArena.fieldReset {
			arena.lights.SetFieldReset()
		} else {
			arena.lights.ClearAll()
		}
	}
}

// Returns the alliance station identifier for the given team, or the empty string if the team is not present
// in the current match.
func (arena *Arena) getAssignedAllianceStation(teamId int) string {
	for station, allianceStation := range arena.AllianceStations {
		if allianceStation.team != nil && allianceStation.team.Id == teamId {
			return station
		}
	}

	return ""
}
