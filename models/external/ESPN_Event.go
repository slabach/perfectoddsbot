package external

type ESPN_Event struct {
	ID        string `json:"id"`
	UID       string `json:"uid"`
	Date      string `json:"date"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	Season    struct {
		Year int    `json:"year"`
		Type int    `json:"type"`
		Slug string `json:"slug"`
	} `json:"season"`
	Competitions []ESPN_Comp `json:"competitions"`
	Links        []struct {
		Language   string   `json:"language"`
		Rel        []string `json:"rel"`
		Href       string   `json:"href"`
		Text       string   `json:"text"`
		ShortText  string   `json:"shortText"`
		IsExternal bool     `json:"isExternal"`
		IsPremium  bool     `json:"isPremium"`
	} `json:"links"`
	Status struct {
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
	Weather struct {
		DisplayValue    string `json:"displayValue"`
		Temperature     int    `json:"temperature"`
		HighTemperature int    `json:"highTemperature"`
		ConditionID     string `json:"conditionId"`
	} `json:"weather,omitempty"`
}
