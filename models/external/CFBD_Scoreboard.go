package external

import "time"

type CFBD_Scoreboard []struct {
	ID             int       `json:"id"`
	StartDate      time.Time `json:"startDate"`
	StartTimeTBD   bool      `json:"startTimeTBD"`
	Tv             string    `json:"tv"`
	NeutralSite    bool      `json:"neutralSite"`
	ConferenceGame bool      `json:"conferenceGame"`
	Status         string    `json:"status"`
	Period         *int      `json:"period"`
	Clock          *string   `json:"clock"`
	Situation      *string   `json:"situation"`
	Possession     *string   `json:"possession"`
	Venue          struct {
		Name  string `json:"name"`
		City  string `json:"city"`
		State string `json:"state"`
	} `json:"venue"`
	HomeTeam struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		Conference     string `json:"conference"`
		Classification string `json:"classification"`
		Points         int    `json:"points"`
	} `json:"homeTeam"`
	AwayTeam struct {
		ID             int    `json:"id"`
		Name           string `json:"name"`
		Conference     string `json:"conference"`
		Classification string `json:"classification"`
		Points         int    `json:"points"`
	} `json:"awayTeam"`
	Weather struct {
		Temperature   *string `json:"temperature"`
		Description   *string `json:"description"`
		WindSpeed     *string `json:"windSpeed"`
		WindDirection *string `json:"windDirection"`
	} `json:"weather"`
	Betting struct {
		Spread        string `json:"spread"`
		OverUnder     string `json:"overUnder"`
		HomeMoneyline int    `json:"homeMoneyline"`
		AwayMoneyline int    `json:"awayMoneyline"`
	} `json:"betting"`
}
