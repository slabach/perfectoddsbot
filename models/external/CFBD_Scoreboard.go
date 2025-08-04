package external

import "time"

type CFBD_Scoreboard struct {
	ID             int       `json:"id"`
	StartDate      time.Time `json:"startDate"`
	StartTimeTBD   bool      `json:"startTimeTBD"`
	Tv             *string   `json:"tv"`
	NeutralSite    bool      `json:"neutralSite"`
	ConferenceGame bool      `json:"conferenceGame"`
	Status         string    `json:"status"`
	Period         *int      `json:"period"`
	Clock          *string   `json:"clock"`
	Situation      *string   `json:"situation"`
	Possession     *string   `json:"possession"`
	LastPlay       *string   `json:"lastPlay"`
	Venue          struct {
		Name  string `json:"name"`
		City  string `json:"city"`
		State string `json:"state"`
	} `json:"venue"`
	HomeTeam CFBD_ScoreboardTeam `json:"homeTeam"`
	AwayTeam CFBD_ScoreboardTeam `json:"awayTeam"`
	Weather  struct {
		Temperature   *float64 `json:"temperature"`
		Description   *string  `json:"description"`
		WindSpeed     *float64 `json:"windSpeed"`
		WindDirection *float64 `json:"windDirection"`
	} `json:"weather"`
	Betting struct {
		Spread        *float64 `json:"spread"`
		OverUnder     *float64 `json:"overUnder"`
		HomeMoneyline *int     `json:"homeMoneyline"`
		AwayMoneyline *int     `json:"awayMoneyline"`
	} `json:"betting"`
}

type CFBD_ScoreboardTeam struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Conference     string  `json:"conference"`
	Classification string  `json:"classification"`
	Points         *int    `json:"points"`
	LineScores     []int   `json:"lineScores"`
	Logo           *string `json:"logo"`
}
