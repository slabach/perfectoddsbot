package external

type ESPN_Scoreboard struct {
	Leagues []struct {
		ID           string `json:"id"`
		UID          string `json:"uid"`
		Name         string `json:"name"`
		Abbreviation string `json:"abbreviation"`
		MidsizeName  string `json:"midsizeName"`
		Slug         string `json:"slug"`
		Season       struct {
			Year        int    `json:"year"`
			StartDate   string `json:"startDate"`
			EndDate     string `json:"endDate"`
			DisplayName string `json:"displayName"`
			Type        struct {
				ID           string `json:"id"`
				Type         int    `json:"type"`
				Name         string `json:"name"`
				Abbreviation string `json:"abbreviation"`
			} `json:"type"`
		} `json:"season"`
		Logos []struct {
			Href        string   `json:"href"`
			Width       int      `json:"width"`
			Height      int      `json:"height"`
			Alt         string   `json:"alt"`
			Rel         []string `json:"rel"`
			LastUpdated string   `json:"lastUpdated"`
		} `json:"logos"`
		CalendarType        string   `json:"calendarType"`
		CalendarIsWhitelist bool     `json:"calendarIsWhitelist"`
		CalendarStartDate   string   `json:"calendarStartDate"`
		CalendarEndDate     string   `json:"calendarEndDate"`
		Calendar            []string `json:"calendar"`
	} `json:"leagues"`
	Day struct {
		Date string `json:"date"`
	} `json:"day"`
	Events     []ESPN_Event `json:"events"`
	EventsDate struct {
		Date       string `json:"date"`
		SeasonType int    `json:"seasonType"`
	} `json:"eventsDate"`
}
