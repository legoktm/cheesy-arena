// Copyright 2016 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)
//
// Web routes for conducting the team defense selection process.

package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"text/template"
)

// Shows the defense selection page.
func DefenseSelectionGetHandler(w http.ResponseWriter, r *http.Request) {
	if !UserIsAdmin(w, r) {
		return
	}

	renderDefenseSelection(w, r, "")
}

// Updates the cache with the latest input from the client.
func DefenseSelectionPostHandler(w http.ResponseWriter, r *http.Request) {
	if !UserIsAdmin(w, r) {
		return
	}

	matchId, _ := strconv.Atoi(r.PostFormValue("matchId"))
	match, err := db.GetMatchById(matchId)
	if err != nil {
		handleWebErr(w, err)
		return
	}

	// Make sure audience selected defense is the same for both.
	if r.PostFormValue("redDefense3") != r.PostFormValue("blueDefense3") &&
		r.PostFormValue("redDefense3") != "" &&
		r.PostFormValue("blueDefense3") != "" {
		renderDefenseSelection(w, r, "Audience-selected defenses are not the same!")
		return
	}

	redErr := validateDefenseSelection([]string{r.PostFormValue("redDefense3"),
		r.PostFormValue("redDefense4"), r.PostFormValue("redDefense5")})
	if redErr == nil {
		match.RedDefense1 = "LB"
		match.RedDefense2 = "CDF"
		match.RedDefense3 = r.PostFormValue("redDefense3")
		match.RedDefense4 = r.PostFormValue("redDefense4")
		match.RedDefense5 = r.PostFormValue("redDefense5")
	}
	blueErr := validateDefenseSelection([]string{r.PostFormValue("blueDefense3"),
		r.PostFormValue("blueDefense4"), r.PostFormValue("blueDefense5")})

	if blueErr == nil {
		match.BlueDefense1 = "LB"
		match.BlueDefense2 = "CDF"
		match.BlueDefense3 = r.PostFormValue("blueDefense3")
		match.BlueDefense4 = r.PostFormValue("blueDefense4")
		match.BlueDefense5 = r.PostFormValue("blueDefense5")
	}
	if redErr == nil || blueErr == nil {
		err = db.SaveMatch(match)
		if err != nil {
			handleWebErr(w, err)
			return
		}
		mainArena.defenseSelectionNotifier.Notify(nil)
	}
	if redErr != nil {
		renderDefenseSelection(w, r, redErr.Error())
		return
	}
	if blueErr != nil {
		renderDefenseSelection(w, r, blueErr.Error())
		return
	}

	http.Redirect(w, r, "/setup/defense_selection", 302)
}

// c = 3, d = 4, e = 5
func incrDefenseGroup(group int) int {
	group++
	if group > 5 {
		group = 3
	}
	return group
}

func renderDefenseSelection(w http.ResponseWriter, r *http.Request, errorMessage string) {
	template := template.New("").Funcs(templateHelpers)
	_, err := template.ParseFiles("templates/setup_defense_selection.html")
	if err != nil {
		handleWebErr(w, err)
		return
	}

	var start int

	// Lazy, don't create a whole db structure for this...
	fname := "defense.txt"
	if _, err := os.Stat(fname); err == nil {
		b, err := ioutil.ReadFile(fname)
		if err != nil {
			panic(err)
		}
		start, _ = strconv.Atoi(string(b))
	} else {
		start = rand.Intn(3) + 3
		err = ioutil.WriteFile(fname, []byte(strconv.Itoa(start)), 0644)
		if err != nil {
			panic(err)
		}
	}

	// QF = 4, SF = 2, F = 1, reset every eliminstance
	humanName := map[int]string{4: "QF", 2: "SF", 1: "F"}
	audDefPick := make(map[string]string)
	defGroup := map[int]string{3: "C (M, R)", 4: "D (SP, DB)", 5: "E (RT, RW)"}
	audDefPick["QF round 1"] = defGroup[start]

	matches, err := db.GetMatchesByType("elimination")
	if err != nil {
		handleWebErr(w, err)
		return
	}
	var unplayedMatches []Match
	for _, match := range matches {
		if match.Status != "complete" {
			unplayedMatches = append(unplayedMatches, match)
		}
		groupStr := fmt.Sprintf("%s round %d", humanName[match.ElimRound], match.ElimInstance)
		fmt.Println(groupStr)
		_, ok := audDefPick[groupStr]
		if !ok {
			// Not set
			start = incrDefenseGroup(start)
			audDefPick[groupStr] = defGroup[start]
		}
	}

	data := struct {
		*EventSettings
		Matches        []Match
		DefenseNames   map[string]string
		ErrorMessage   string
		RandomDefenses map[string]string
	}{eventSettings, unplayedMatches, defenseNames, errorMessage, audDefPick}
	err = template.ExecuteTemplate(w, "setup_defense_selection.html", data)
	if err != nil {
		handleWebErr(w, err)
		return
	}
}

func inSet(defense string, defenses []string) bool {
	for _, name := range defenses {
		if name == defense {
			return true
		}
	}

	return false
}

// Takes a slice of the defenses in positions 3-5 and returns an error if they are not valid.
func validateDefenseSelection(defenses []string) error {
	// Build map to track which defenses have been used.
	// FIXME UPDATE THIS
	defenseCounts := make(map[string]int)
	cCounts := 0
	dCounts := 0
	eCounts := 0
	for _, defense := range placeableDefenses {
		defenseCounts[defense] = 0
	}
	numBlankDefenses := 0

	for _, defense := range defenses {
		if defense == "" {
			numBlankDefenses++
			continue
		}

		defenseCount, ok := defenseCounts[defense]
		if !ok {
			return fmt.Errorf("Invalid defense type: %s", defense)
		}
		if defenseCount != 0 {
			return fmt.Errorf("Defense used more than once: %s", defenseNames[defense])
		}
		if inSet(defense, cDefenses) {
			if cCounts != 0 {
				return fmt.Errorf("Can only use one defense from group C (M, R)")
			}
			cCounts++
		}
		if inSet(defense, dDefenses) {
			if dCounts != 0 {
				return fmt.Errorf("Can only use one defense from group D (SP, DB)")
			}
			dCounts++
		}
		if inSet(defense, eDefenses) {
			if eCounts != 0 {
				return fmt.Errorf("Can only use one defense from group E (RT, RW)")
			}
			eCounts++
		}
		defenseCounts[defense]++
	}

	if numBlankDefenses > 0 && numBlankDefenses < 3 {
		return fmt.Errorf("Cannot leave defenses blank.")
	}

	return nil
}
