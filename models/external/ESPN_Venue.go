package external

type ESPN_Venue struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Address  struct {
		City  string `json:"city"`
		State string `json:"state"`
	} `json:"address"`
	Indoor bool `json:"indoor"`
}
