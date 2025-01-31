package external

type ESPN_Odds struct {
	Provider struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Priority int    `json:"priority"`
	} `json:"provider"`
	Details      string  `json:"details"`
	OverUnder    float64 `json:"overUnder"`
	Spread       float64 `json:"spread"`
	AwayTeamOdds struct {
		Favorite bool `json:"favorite"`
		Underdog bool `json:"underdog"`
		Team     struct {
			ID           string `json:"id"`
			UID          string `json:"uid"`
			Abbreviation string `json:"abbreviation"`
			Name         string `json:"name"`
			DisplayName  string `json:"displayName"`
			Logo         string `json:"logo"`
		} `json:"team"`
	} `json:"awayTeamOdds"`
	HomeTeamOdds struct {
		Favorite bool `json:"favorite"`
		Underdog bool `json:"underdog"`
		Team     struct {
			ID           string `json:"id"`
			UID          string `json:"uid"`
			Abbreviation string `json:"abbreviation"`
			Name         string `json:"name"`
			DisplayName  string `json:"displayName"`
			Logo         string `json:"logo"`
		} `json:"team"`
	} `json:"homeTeamOdds"`
	Open struct {
		Over struct {
			Value                 float64 `json:"value"`
			DisplayValue          string  `json:"displayValue"`
			AlternateDisplayValue string  `json:"alternateDisplayValue"`
			Decimal               float64 `json:"decimal"`
			Fraction              string  `json:"fraction"`
			American              string  `json:"american"`
		} `json:"over"`
		Under struct {
			Value                 float64 `json:"value"`
			DisplayValue          string  `json:"displayValue"`
			AlternateDisplayValue string  `json:"alternateDisplayValue"`
			Decimal               float64 `json:"decimal"`
			Fraction              string  `json:"fraction"`
			American              string  `json:"american"`
		} `json:"under"`
		Total struct {
			Value                 float64 `json:"value"`
			DisplayValue          string  `json:"displayValue"`
			AlternateDisplayValue string  `json:"alternateDisplayValue"`
			Decimal               float64 `json:"decimal"`
			Fraction              string  `json:"fraction"`
			American              string  `json:"american"`
		} `json:"total"`
	} `json:"open"`
	Current struct {
		Over struct {
			Value                 float64 `json:"value"`
			DisplayValue          string  `json:"displayValue"`
			AlternateDisplayValue string  `json:"alternateDisplayValue"`
			Decimal               float64 `json:"decimal"`
			Fraction              string  `json:"fraction"`
			American              string  `json:"american"`
		} `json:"over"`
		Under struct {
			Value                 float64 `json:"value"`
			DisplayValue          string  `json:"displayValue"`
			AlternateDisplayValue string  `json:"alternateDisplayValue"`
			Decimal               float64 `json:"decimal"`
			Fraction              string  `json:"fraction"`
			American              string  `json:"american"`
		} `json:"under"`
		Total struct {
			AlternateDisplayValue string `json:"alternateDisplayValue"`
			American              string `json:"american"`
		} `json:"total"`
	} `json:"current"`
}
