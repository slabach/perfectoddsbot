package external

import "time"

type CFBD_BettingLines struct {
	ID             int         `json:"id"`
	Season         int         `json:"season"`
	SeasonType     string      `json:"seasonType"`
	Week           int         `json:"week"`
	StartDate      time.Time   `json:"startDate"`
	HomeTeam       string      `json:"homeTeam"`
	HomeConference string      `json:"homeConference"`
	HomeScore      *int        `json:"homeScore"`
	AwayTeam       string      `json:"awayTeam"`
	AwayConference string      `json:"awayConference"`
	AwayScore      *int        `json:"awayScore"`
	Lines          []CFBD_Line `json:"lines"`
}

type CFBD_Line struct {
	Provider        string  `json:"provider"`
	Spread          string  `json:"spread"`
	FormattedSpread string  `json:"formattedSpread"`
	SpreadOpen      string  `json:"spreadOpen"`
	OverUnder       string  `json:"overUnder"`
	OverUnderOpen   *string `json:"overUnderOpen"`
	HomeMoneyline   int     `json:"homeMoneyline"`
	AwayMoneyline   int     `json:"awayMoneyline"`
}
