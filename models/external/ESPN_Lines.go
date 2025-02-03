package external

type ESPN_Lines struct {
	Count     int         `json:"count"`
	PageIndex int         `json:"pageIndex"`
	PageSize  int         `json:"pageSize"`
	PageCount int         `json:"pageCount"`
	Items     []ESPN_Line `json:"items"`
}

type ESPN_Line struct {
	Ref      string `json:"$ref"`
	Provider struct {
		Ref      string `json:"$ref"`
		ID       string `json:"id"`
		Name     string `json:"name"`
		Priority int    `json:"priority"`
	} `json:"provider"`
	Details      string   `json:"details"`
	OverUnder    float64  `json:"overUnder"`
	Spread       float64  `json:"spread"`
	OverOdds     float64  `json:"overOdds"`
	UnderOdds    float64  `json:"underOdds"`
	AwayTeamOdds TeamOdds `json:"awayTeamOdds,omitempty"`
	HomeTeamOdds TeamOdds `json:"homeTeamOdds,omitempty"`
	Links        []struct {
		Language   string   `json:"language"`
		Rel        []string `json:"rel"`
		Href       string   `json:"href"`
		Text       string   `json:"text"`
		ShortText  string   `json:"shortText"`
		IsExternal bool     `json:"isExternal"`
		IsPremium  bool     `json:"isPremium"`
	} `json:"links"`
	MoneylineWinner bool `json:"moneylineWinner"`
	SpreadWinner    bool `json:"spreadWinner"`
	Open            struct {
		Over  OverUnderTotal `json:"over"`
		Under OverUnderTotal `json:"under"`
		Total OverUnderTotal `json:"total"`
	} `json:"open"`
	Close struct {
		Over  OverUnderTotal `json:"over"`
		Under OverUnderTotal `json:"under"`
		Total OverUnderTotal `json:"total"`
	} `json:"close,omitempty"`
	Current struct {
		Over  OverUnderTotal `json:"over"`
		Under OverUnderTotal `json:"under"`
		Total OverUnderTotal `json:"total"`
	} `json:"current"`
}

type TeamOdds struct {
	Favorite   bool    `json:"favorite"`
	Underdog   bool    `json:"underdog"`
	MoneyLine  int     `json:"moneyLine"`
	SpreadOdds float64 `json:"spreadOdds"`
	Open       struct {
		Favorite    bool         `json:"favorite"`
		PointSpread PointsSpread `json:"pointSpread"`
		Spread      Spread       `json:"spread"`
		MoneyLine   MoneyLine    `json:"moneyLine"`
	} `json:"open"`
	Close struct {
		PointSpread PointsSpread `json:"pointSpread"`
		Spread      Spread       `json:"spread"`
		MoneyLine   MoneyLine    `json:"moneyLine"`
	} `json:"close"`
	Current struct {
		PointSpread PointsSpread `json:"pointSpread"`
		Spread      Spread       `json:"spread"`
		MoneyLine   MoneyLine    `json:"moneyLine"`
	} `json:"current"`
	Team struct {
		Ref string `json:"$ref"`
	} `json:"team"`
}

type PointsSpread struct {
	AlternateDisplayValue string `json:"alternateDisplayValue"`
	American              string `json:"american"`
}
type Spread struct {
	Value                 float64 `json:"value"`
	DisplayValue          string  `json:"displayValue"`
	AlternateDisplayValue string  `json:"alternateDisplayValue"`
	Decimal               float64 `json:"decimal"`
	Fraction              string  `json:"fraction"`
	American              string  `json:"american"`
	Outcome               struct {
		Type string `json:"type"`
	} `json:"outcome"`
}
type MoneyLine struct {
	Value                 float64 `json:"value"`
	DisplayValue          string  `json:"displayValue"`
	AlternateDisplayValue string  `json:"alternateDisplayValue"`
	Decimal               float64 `json:"decimal"`
	Fraction              string  `json:"fraction"`
	American              string  `json:"american"`
	Outcome               struct {
		Type string `json:"type"`
	} `json:"outcome"`
}
type OverUnderTotal struct {
	Value                 float64 `json:"value"`
	DisplayValue          string  `json:"displayValue"`
	AlternateDisplayValue string  `json:"alternateDisplayValue"`
	Decimal               float64 `json:"decimal"`
	Fraction              string  `json:"fraction"`
	American              string  `json:"american"`
	Outcome               struct {
		Type string `json:"type"`
	} `json:"outcome"`
}
