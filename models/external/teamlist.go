package external

type TeamList []struct {
	Abbreviation   string      `json:"abbreviation"`
	ApiId          int         `json:"apiID"`
	Conference     string      `json:"conference"`
	CreatedAt      string      `json:"createdAt"`
	DefPfi         float64     `json:"defPfi"`
	DeletedAt      DeletedAt   `json:"deletedAt"`
	ID             int         `json:"id"`
	LogoURL        string      `json:"logoUrl"`
	Mascot         string      `json:"mascot"`
	Movement       int         `json:"movement"`
	Name           string      `json:"name"`
	OffPfi         float64     `json:"offPfi"`
	Pfi            float64     `json:"pfi"`
	PrimaryColor   string      `json:"primaryColor"`
	Scheme         float64     `json:"scheme"`
	SchoolVenue    SchoolVenue `json:"schoolVenue"`
	SchoolVenueID  int         `json:"schoolVenueID"`
	SecondaryColor string      `json:"secondaryColor"`
	TalentMod      float64     `json:"talentMod"`
	UpdatedAt      string      `json:"updatedAt"`
}

type SchoolVenue struct {
	City      string    `json:"city"`
	CreatedAt string    `json:"createdAt"`
	DeletedAt DeletedAt `json:"deletedAt"`
	ID        int       `json:"id"`
	IsDome    bool      `json:"isDome"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Name      string    `json:"name"`
	State     string    `json:"state"`
	UpdatedAt string    `json:"updatedAt"`
	VenueID   int       `json:"venueID"`
	ZipCode   string    `json:"zipCode"`
}

type DeletedAt struct {
	Time  string `json:"time"`
	Valid bool   `json:"valid"`
}
