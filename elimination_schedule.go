// Copyright 2014 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Functions for creating and updating the elimination match schedule.

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ElimAlliance struct {
	Team       AllianceTeam
	AllianceId int
	Score      int
}

const elimMatchSpacingSec = 600

// Incrementally creates any elimination matches that can be created, based on the results of alliance
// selection or prior elimination rounds. Returns the winning alliance once it has been determined.
func (database *Database) UpdateEliminationSchedule(startTime time.Time) ([]AllianceTeam, error) {
	winner, err := database.buildEliminationMatchesFifteen()
	if err != nil {
		return []AllianceTeam{}, err
	}

	// Update the scheduled time for all matches that have yet to be run.
	matches, err := database.GetMatchesByType("elimination")
	if err != nil {
		return []AllianceTeam{}, err
	}
	matchIndex := 0
	for _, match := range matches {
		if match.Status == "complete" {
			continue
		}
		match.Time = startTime.Add(time.Duration(matchIndex*elimMatchSpacingSec) * time.Second)
		database.SaveMatch(&match)
		matchIndex++
	}

	return winner, err
}

func (database *Database) buildEliminationMatchesFifteen() ([]AllianceTeam, error) {
	fmt.Println("yo building 2015 matches")
	completed := 0
	matches, err := database.GetMatchesByType("elimination")
	if err != nil {
		return []AllianceTeam{}, err
	}
	for _, match := range matches {
		if match.Status == "complete" {
			completed++
		}
	}
	// So we could have:
	// 0 matches - nothing yet
	// 8 matches - QFs
	// 8+6 matches - SFs
	// > 14 matches - Fs (W-L-T)
	if len(matches) == 0 {
		fmt.Println("yo building 2015 qf")
		// do we need the shuffle teams stuff?
		match1 := createMatch("QF", 4, 1, 1, database.GetTeamsByAlliance(4), database.GetTeamsByAlliance(5))
		err = database.CreateMatch(match1)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match2 := createMatch("QF", 4, 1, 2, database.GetTeamsByAlliance(3), database.GetTeamsByAlliance(6))
		err = database.CreateMatch(match2)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match3 := createMatch("QF", 4, 1, 3, database.GetTeamsByAlliance(2), database.GetTeamsByAlliance(7))
		err = database.CreateMatch(match3)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match4 := createMatch("QF", 4, 1, 4, database.GetTeamsByAlliance(1), database.GetTeamsByAlliance(8))
		err = database.CreateMatch(match4)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match5 := createMatch("QF", 4, 1, 5, database.GetTeamsByAlliance(4), database.GetTeamsByAlliance(6))
		err = database.CreateMatch(match5)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match6 := createMatch("QF", 4, 1, 6, database.GetTeamsByAlliance(3), database.GetTeamsByAlliance(5))
		err = database.CreateMatch(match6)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match7 := createMatch("QF", 4, 1, 7, database.GetTeamsByAlliance(2), database.GetTeamsByAlliance(8))
		err = database.CreateMatch(match7)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match8 := createMatch("QF", 4, 1, 8, database.GetTeamsByAlliance(1), database.GetTeamsByAlliance(7))
		err = database.CreateMatch(match8)
		if err != nil {
			return []AllianceTeam{}, err
		}

	} else if completed == 8 && len(matches) == 8 {
		fmt.Println("yo building 2015 sf")
		var alliances = make([]ElimAlliance, 8)
		allAlliances, _ := database.GetAllAlliances()
		for _, at := range allAlliances {
			for _, allianceTeam := range at {
				alliances[allianceTeam.AllianceId-1] = ElimAlliance{allianceTeam, allianceTeam.AllianceId, 0}
			}
		}

		for _, match := range matches {
			result, _ := database.GetMatchResultForMatch(match.Id)
			//fmt.Println(match.Id)
			result.CorrectEliminationScore()
			fmt.Println("red: " + strconv.Itoa(match.Red1) + "," + strconv.Itoa(match.Red2) + "," + strconv.Itoa(match.Red3))
			updateAllianceScore(alliances, match.Red1, match.Red2, match.Red3, result.RedScoreSummary().Score)
			//fmt.Println(result.RedScoreSummary().Score)
			fmt.Println("blue: " + strconv.Itoa(match.Blue1) + "," + strconv.Itoa(match.Blue2) + "," + strconv.Itoa(match.Blue3))
			updateAllianceScore(alliances, match.Blue1, match.Blue2, match.Blue3, result.BlueScoreSummary().Score)
			//fmt.Println(result.BlueScoreSummary().Score)
		}

		fmt.Println(alliances)

		sort.Sort(ByScore(alliances))
		fmt.Println(alliances)
		alliances = alliances[0:4]

		match9 := createMatch("SF", 4, 1, 9, database.GetTeamsByAlliance(alliances[1].AllianceId), database.GetTeamsByAlliance(alliances[3].AllianceId))
		err = database.CreateMatch(match9)
		if err != nil {
			return []AllianceTeam{}, err
		}

		match10 := createMatch("SF", 4, 1, 10, database.GetTeamsByAlliance(alliances[0].AllianceId), database.GetTeamsByAlliance(alliances[2].AllianceId))
		err = database.CreateMatch(match10)
		if err != nil {
			return []AllianceTeam{}, err
		}

		match11 := createMatch("SF", 4, 2, 11, database.GetTeamsByAlliance(alliances[1].AllianceId), database.GetTeamsByAlliance(alliances[2].AllianceId))
		err = database.CreateMatch(match11)
		if err != nil {
			return []AllianceTeam{}, err
		}

		match12 := createMatch("SF", 4, 2, 12, database.GetTeamsByAlliance(alliances[0].AllianceId), database.GetTeamsByAlliance(alliances[3].AllianceId))
		err = database.CreateMatch(match12)
		if err != nil {
			return []AllianceTeam{}, err
		}

		match13 := createMatch("SF", 4, 3, 13, database.GetTeamsByAlliance(alliances[2].AllianceId), database.GetTeamsByAlliance(alliances[3].AllianceId))
		err = database.CreateMatch(match13)
		if err != nil {
			return []AllianceTeam{}, err
		}

		match14 := createMatch("SF", 4, 3, 14, database.GetTeamsByAlliance(alliances[0].AllianceId), database.GetTeamsByAlliance(alliances[1].AllianceId))
		err = database.CreateMatch(match14)
		if err != nil {
			return []AllianceTeam{}, err
		}
	} else if completed == 14 && len(matches) == 14 {
		fmt.Println("yo building 2015 f")
		// The finals!
		var semiAlliances = make([]ElimAlliance, 8)
		allAlliances, _ := database.GetAllAlliances()
		for _, at := range allAlliances {
			for _, allianceTeam := range at {
				semiAlliances[allianceTeam.TeamId] = ElimAlliance{allianceTeam, allianceTeam.AllianceId, 0}
			}
		}

		for _, match := range matches {
			if strings.HasPrefix(match.DisplayName, "SF") {
				result, _ := database.GetMatchResultForMatch(match.Id)
				result.CorrectEliminationScore()
				updateAllianceScore(semiAlliances, match.Red1, match.Red2, match.Red3, result.RedScoreSummary().Score)
				updateAllianceScore(semiAlliances, match.Blue1, match.Blue2, match.Blue3, result.BlueScoreSummary().Score)

			}
		}

		sort.Sort(ByScore(semiAlliances))
		redFinalsAlliance := semiAlliances[0]
		blueFinalsAlliance := semiAlliances[1]
		match15 := createMatch("F", 4, 1, 15, database.GetTeamsByAlliance(redFinalsAlliance.AllianceId), database.GetTeamsByAlliance(blueFinalsAlliance.AllianceId))
		err = database.CreateMatch(match15)
		if err != nil {
			return []AllianceTeam{}, err
		}
		match16 := createMatch("F", 4, 1, 16, database.GetTeamsByAlliance(redFinalsAlliance.AllianceId), database.GetTeamsByAlliance(blueFinalsAlliance.AllianceId))
		err = database.CreateMatch(match16)
		if err != nil {
			return []AllianceTeam{}, err
		}

	} else if completed > 14 {
		fmt.Println("yo we're in 2015 finals")
		finalsPlayed := 0
		redWins := 0
		blueWins := 0
		// TODO: Don't copy this
		var semiAlliances = make([]ElimAlliance, 8)
		allAlliances, _ := database.GetAllAlliances()
		for _, at := range allAlliances {
			for _, allianceTeam := range at {
				semiAlliances[allianceTeam.TeamId] = ElimAlliance{allianceTeam, allianceTeam.AllianceId, 0}
			}
		}

		for _, match := range matches {
			if match.Status == "complete" && strings.HasPrefix(match.DisplayName, "F") {
				finalsPlayed += 1
				// Check who won.
				switch match.Winner {
				case "R":
					redWins += 1
				case "B":
					blueWins += 1
				case "T":
					// ?
				default:
					return []AllianceTeam{}, fmt.Errorf("Completed match %d has invalid winner '%s'", match.Id, match.Winner)
				}

			} else if strings.HasPrefix(match.DisplayName, "SF") {
				result, _ := database.GetMatchResultForMatch(match.Id)
				result.CorrectEliminationScore()
				updateAllianceScore(semiAlliances, match.Red1, match.Red2, match.Red3, result.RedScoreSummary().Score)
				updateAllianceScore(semiAlliances, match.Blue1, match.Blue2, match.Blue3, result.BlueScoreSummary().Score)

			}

		}

		sort.Sort(ByScore(semiAlliances))
		redFinalsAlliance := semiAlliances[0]
		blueFinalsAlliance := semiAlliances[1]

		if finalsPlayed >= 2 {
			if redWins == 2 {
				return database.GetTeamsByAlliance(redFinalsAlliance.AllianceId), nil
			} else if blueWins == 2 {
				return database.GetTeamsByAlliance(blueFinalsAlliance.AllianceId), nil
			}
			// No one has won 2 yet, add another match
			matchFinalsNext := createMatch("F", 4, 1, 14+finalsPlayed, database.GetTeamsByAlliance(redFinalsAlliance.AllianceId), database.GetTeamsByAlliance(blueFinalsAlliance.AllianceId))
			err = database.CreateMatch(matchFinalsNext)
			if err != nil {
				return []AllianceTeam{}, err
			}

		}
	}

	return []AllianceTeam{}, err

}

type ByScore []ElimAlliance

func (a ByScore) Len() int           { return len(a) }
func (a ByScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByScore) Less(i, j int) bool { return a[i].Score > a[j].Score }

func updateAllianceScore(alliances []ElimAlliance, teamId int, team2 int, team3 int, score int) {
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
	fmt.Println("new")
	fmt.Println(alliances[theRealIndex].Score)
}

// Recursively traverses the elimination bracket downwards, creating matches as necessary. Returns the winner
// of the given round if known.
func (database *Database) buildEliminationMatchSet(round int, group int, numAlliances int) ([]AllianceTeam, error) {
	if numAlliances < 2 {
		return []AllianceTeam{}, fmt.Errorf("Must have at least 2 alliances")
	}
	roundName, ok := map[int]string{1: "F", 2: "SF", 4: "QF", 8: "EF"}[round]
	if !ok {
		return []AllianceTeam{}, fmt.Errorf("Round of depth %d is not supported", round*2)
	}
	if round != 1 {
		roundName += strconv.Itoa(group)
	}

	// Recurse to figure out who the involved alliances are.
	var redAlliance, blueAlliance []AllianceTeam
	var err error
	if numAlliances < 4*round {
		// This is the first round for some or all alliances and will be at least partially populated from the
		// alliance selection results.
		matchups := []int{1, 16, 8, 9, 4, 13, 5, 12, 2, 15, 7, 10, 3, 14, 6, 11}
		factor := len(matchups) / round
		redAllianceNumber := matchups[(group-1)*factor]
		blueAllianceNumber := matchups[(group-1)*factor+factor/2]
		numDirectAlliances := 4*round - numAlliances
		if redAllianceNumber <= numDirectAlliances {
			// The red alliance has a bye or the number of alliances is a power of 2; get from alliance selection.
			redAlliance = database.GetTeamsByAlliance(redAllianceNumber)
			if err != nil {
				return []AllianceTeam{}, err
			}
		}
		if blueAllianceNumber <= numDirectAlliances {
			// The blue alliance has a bye or the number of alliances is a power of 2; get from alliance selection.
			blueAlliance = database.GetTeamsByAlliance(blueAllianceNumber)
			if err != nil {
				return []AllianceTeam{}, err
			}
		}
	}

	// If the alliances aren't known yet, get them from one round down in the bracket.
	if len(redAlliance) == 0 {
		redAlliance, err = database.buildEliminationMatchSet(round*2, group*2-1, numAlliances)
		if err != nil {
			return []AllianceTeam{}, err
		}
	}
	if len(blueAlliance) == 0 {
		blueAlliance, err = database.buildEliminationMatchSet(round*2, group*2, numAlliances)
		if err != nil {
			return []AllianceTeam{}, err
		}
	}

	// Bail if the rounds below are not yet complete and we don't know either alliance competing this round.
	if len(redAlliance) == 0 && len(blueAlliance) == 0 {
		return []AllianceTeam{}, nil
	}

	// Check if the match set exists already and if it has been won.
	var redWins, blueWins, numIncomplete int
	var ties []*Match
	matches, err := database.GetMatchesByElimRoundGroup(round, group)
	if err != nil {
		return []AllianceTeam{}, err
	}
	var unplayedMatches []*Match
	for _, match := range matches {
		// Update the teams in the match if they are not yet set or are incorrect.
		if len(redAlliance) != 0 && !(teamInAlliance(match.Red1, redAlliance) &&
			teamInAlliance(match.Red2, redAlliance) && teamInAlliance(match.Red3, redAlliance)) {
			positionRedTeams(&match, redAlliance)
			database.SaveMatch(&match)
		} else if len(blueAlliance) != 0 && !(teamInAlliance(match.Blue1, blueAlliance) &&
			teamInAlliance(match.Blue2, blueAlliance) && teamInAlliance(match.Blue3, blueAlliance)) {
			positionBlueTeams(&match, blueAlliance)
			database.SaveMatch(&match)
		}

		if match.Status != "complete" {
			unplayedMatches = append(unplayedMatches, &match)
			numIncomplete += 1
			continue
		}

		// Check who won.
		switch match.Winner {
		case "R":
			redWins += 1
		case "B":
			blueWins += 1
		case "T":
			ties = append(ties, &match)
		default:
			return []AllianceTeam{}, fmt.Errorf("Completed match %d has invalid winner '%s'", match.Id, match.Winner)
		}
	}

	// Delete any superfluous matches if the round is won.
	if redWins == 2 || blueWins == 2 {
		for _, match := range unplayedMatches {
			err = database.DeleteMatch(match)
			if err != nil {
				return []AllianceTeam{}, err
			}
		}

		// Bail out and announce the winner of this round.
		if redWins == 2 {
			return redAlliance, nil
		} else {
			return blueAlliance, nil
		}
	}

	// Create initial set of matches or recreate any superfluous matches that were deleted but now are needed
	// due to a revision in who won.
	if len(matches) == 0 || len(ties) == 0 && numIncomplete == 0 {
		// Fill in zeroes if only one alliance is known.
		if len(redAlliance) == 0 {
			redAlliance = []AllianceTeam{AllianceTeam{}, AllianceTeam{}, AllianceTeam{}}
		} else if len(blueAlliance) == 0 {
			blueAlliance = []AllianceTeam{AllianceTeam{}, AllianceTeam{}, AllianceTeam{}}
		}
		if len(redAlliance) < 3 || len(blueAlliance) < 3 {
			// Raise an error if the alliance selection process gave us less than 3 teams per alliance.
			return []AllianceTeam{}, fmt.Errorf("Alliances must consist of at least 3 teams")
		}
		if len(matches) < 1 {
			err = database.CreateMatch(createMatch(roundName, round, group, 1, redAlliance, blueAlliance))
			if err != nil {
				return []AllianceTeam{}, err
			}
		}
		if len(matches) < 2 {
			err = database.CreateMatch(createMatch(roundName, round, group, 2, redAlliance, blueAlliance))
			if err != nil {
				return []AllianceTeam{}, err
			}
		}
		if len(matches) < 3 {
			err = database.CreateMatch(createMatch(roundName, round, group, 3, redAlliance, blueAlliance))
			if err != nil {
				return []AllianceTeam{}, err
			}
		}
	}

	// Duplicate any ties if we have run out of matches. Don't change the team positions, so queueing
	// personnel can reuse any tied matches without having to print new schedules.
	if numIncomplete == 0 {
		for index, tie := range ties {
			match := createMatch(roundName, round, group, len(matches)+index+1, redAlliance, blueAlliance)
			match.Red1, match.Red2, match.Red3 = tie.Red1, tie.Red2, tie.Red3
			match.Blue1, match.Blue2, match.Blue3 = tie.Blue1, tie.Blue2, tie.Blue3
			err = database.CreateMatch(match)
			if err != nil {
				return []AllianceTeam{}, err
			}
		}
	}

	return []AllianceTeam{}, nil
}

// Creates a match at the given point in the elimination bracket and populates the teams.
func createMatch(roundName string, round int, group int, instance int, redAlliance []AllianceTeam, blueAlliance []AllianceTeam) *Match {
	match := Match{Type: "elimination", DisplayName: fmt.Sprintf("%s-%d", roundName, instance),
		ElimRound: round, ElimGroup: group, ElimInstance: instance}
	positionRedTeams(&match, redAlliance)
	positionBlueTeams(&match, blueAlliance)
	return &match
}

// Assigns the first three teams from the alliance into the red team slots for the match.
func positionRedTeams(match *Match, alliance []AllianceTeam) {
	// For the 2015 game, the alliance captain is in the middle, first pick on the left, second on the right.
	match.Red1 = alliance[1].TeamId
	match.Red2 = alliance[0].TeamId
	match.Red3 = alliance[2].TeamId
}

// Assigns the first three teams from the alliance into the blue team slots for the match.
func positionBlueTeams(match *Match, alliance []AllianceTeam) {
	// For the 2015 game, the alliance captain is in the middle, first pick on the left, second on the right.
	match.Blue1 = alliance[1].TeamId
	match.Blue2 = alliance[0].TeamId
	match.Blue3 = alliance[2].TeamId
}

// Returns true if the given team is part of the given alliance.
func teamInAlliance(teamId int, alliance []AllianceTeam) bool {
	for _, allianceTeam := range alliance {
		if teamId == allianceTeam.TeamId {
			return true
		}
	}
	return false
}
