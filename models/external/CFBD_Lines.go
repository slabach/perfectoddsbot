package external

import (
	"time"
)

type CFBD_BettingLines struct {
	ID                 int         `json:"id"`
	Season             int         `json:"season"`
	SeasonType         string      `json:"seasonType"`
	Week               int         `json:"week"`
	StartDate          time.Time   `json:"startDate"`
	HomeTeam           string      `json:"homeTeam"`
	HomeConference     string      `json:"homeConference"`
	HomeClassification string      `json:"homeClassification"`
	HomeScore          *int        `json:"homeScore"`
	AwayTeam           string      `json:"awayTeam"`
	AwayConference     string      `json:"awayConference"`
	AwayClassification string      `json:"awayClassification"`
	AwayScore          *int        `json:"awayScore"`
	Lines              []CFBD_Line `json:"lines"`
}

type CFBD_Line struct {
	Provider        string   `json:"provider"`
	Spread          *float64 `json:"spread"`
	FormattedSpread string   `json:"formattedSpread"`
	SpreadOpen      *float64 `json:"spreadOpen"`
	OverUnder       *float64 `json:"overUnder"`
	OverUnderOpen   *float64 `json:"overUnderOpen"`
	HomeMoneyline   *int     `json:"homeMoneyline"`
	AwayMoneyline   *int     `json:"awayMoneyline"`
}
