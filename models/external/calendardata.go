package external

import (
	"gorm.io/gorm"
	"time"
)

type CalendarData = struct {
	Week           *Week     `json:"curWeek"`
	PrevWeek       *Week     `json:"prevWeek"`
	Season         *Season   `json:"curSeason"`
	PreviousYears  []Season  `json:"prevSeasons"`
	Week1StartDate time.Time `json:"curSeasonWeekOneStartDate"`
	MaxRegWeek     uint      `json:"maxRegSeasonWeekNum"`
	NextSeason     *Season   `json:"nextSeason"`
}

type Week struct {
	gorm.Model
	WeekNum   uint `json:"weekNum"`
	SeasonId  uint
	Season    Season    `json:"season,omitempty" gorm:"foreignKey:SeasonId"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
	WeekType  string    `json:"weekType" gorm:"type:varchar(20);default:'regular'"`
	SvalRun   bool      `json:"pfiRun" gorm:"type:tinyint(1);default:0"`
}

type Season struct {
	gorm.Model
	Year      uint      `json:"year"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}
