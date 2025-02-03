package external

type ESPN_Comp struct {
	ID         string `json:"id"`
	UID        string `json:"uid"`
	Date       string `json:"date"`
	Attendance int    `json:"attendance"`
	Type       struct {
		ID           string `json:"id"`
		Abbreviation string `json:"abbreviation"`
	} `json:"type"`
	TimeValid             bool              `json:"timeValid"`
	NeutralSite           bool              `json:"neutralSite"`
	ConferenceCompetition bool              `json:"conferenceCompetition"`
	PlayByPlayAvailable   bool              `json:"playByPlayAvailable"`
	Recent                bool              `json:"recent"`
	Venue                 ESPN_Venue        `json:"venue"`
	Competitors           []ESPN_Competitor `json:"competitors"`
	Notes                 []any             `json:"notes"`
	Status                struct {
		Clock        float64 `json:"clock"`
		DisplayClock string  `json:"displayClock"`
		Period       int     `json:"period"`
		Type         struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			State       string `json:"state"`
			Completed   bool   `json:"completed"`
			Description string `json:"description"`
			Detail      string `json:"detail"`
			ShortDetail string `json:"shortDetail"`
		} `json:"type"`
	} `json:"status"`
	Broadcasts []struct {
		Market string   `json:"market"`
		Names  []string `json:"names"`
	} `json:"broadcasts"`
	Format struct {
		Regulation struct {
			Periods int `json:"periods"`
		} `json:"regulation"`
	} `json:"format"`
	Tickets []struct {
		Summary         string `json:"summary"`
		NumberAvailable int    `json:"numberAvailable"`
		Links           []struct {
			Href string `json:"href"`
		} `json:"links"`
	} `json:"tickets"`
	StartDate     string `json:"startDate"`
	Broadcast     string `json:"broadcast"`
	GeoBroadcasts []struct {
		Type struct {
			ID        string `json:"id"`
			ShortName string `json:"shortName"`
		} `json:"type"`
		Market struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"market"`
		Media struct {
			ShortName string `json:"shortName"`
		} `json:"media"`
		Lang   string `json:"lang"`
		Region string `json:"region"`
	} `json:"geoBroadcasts"`
	Odds       []ESPN_Odds `json:"odds"`
	Highlights []any       `json:"highlights"`
}

type ESPN_Competitor struct {
	ID          string    `json:"id"`
	UID         string    `json:"uid"`
	Type        string    `json:"type"`
	Order       int       `json:"order"`
	HomeAway    string    `json:"homeAway"`
	Team        ESPN_Team `json:"team"`
	Score       string    `json:"score"`
	CuratedRank struct {
		Current int `json:"current"`
	} `json:"curatedRank"`
	Statistics []any `json:"statistics"`
	Records    []struct {
		Name         string `json:"name"`
		Abbreviation string `json:"abbreviation"`
		Type         string `json:"type"`
		Summary      string `json:"summary"`
	} `json:"records"`
}
