package external

type ESPN_Team struct {
	ID               string `json:"id"`
	UID              string `json:"uid"`
	Location         string `json:"location"`
	Name             string `json:"name"`
	Abbreviation     string `json:"abbreviation"`
	DisplayName      string `json:"displayName"`
	ShortDisplayName string `json:"shortDisplayName"`
	Color            string `json:"color"`
	AlternateColor   string `json:"alternateColor"`
	IsActive         bool   `json:"isActive"`
	Venue            struct {
		ID string `json:"id"`
	} `json:"venue"`
	Links []struct {
		Rel        []string `json:"rel"`
		Href       string   `json:"href"`
		Text       string   `json:"text"`
		IsExternal bool     `json:"isExternal"`
		IsPremium  bool     `json:"isPremium"`
	} `json:"links"`
	Logo         string `json:"logo"`
	ConferenceID string `json:"conferenceId"`
}
