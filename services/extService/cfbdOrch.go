package extService

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
)

func ListCFBGames(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	cfbUrl := "https://api.collegefootballdata.com/lines?"
	pfWeekUrl := "https://api.perfectfall.com/week-season"
	conferenceList := []string{"Big Ten", "ACC", "SEC", "Big 12"}

	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if !guild.PremiumEnabled {
		common.SendError(s, i, fmt.Errorf("Your server must have the premium subscription in order to enable this feature"), db)
		return
	}

	weekResp, err := common.PFWrapper(pfWeekUrl)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	defer weekResp.Body.Close()

	var calendar external.CalendarData
	err = json.NewDecoder(weekResp.Body).Decode(&calendar)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var weekNum = calendar.Week.WeekNum
	if weekNum > calendar.MaxRegWeek {
		weekNum = 1
	}
	linesUrl := fmt.Sprintf("%syear=%d&seasonType=%s&week=%d", cfbUrl, calendar.Season.Year, calendar.Week.WeekType, weekNum)
	linesResp, err := common.CFBDWrapper(linesUrl)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	defer linesResp.Body.Close()

	var bettingLines []external.CFBD_BettingLines
	err = json.NewDecoder(linesResp.Body).Decode(&bettingLines)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var response string
	if len(bettingLines) == 0 {
		response = "There are no lines for this week"
	} else {
		response = fmt.Sprintf("Lines for week %d - \n", calendar.Week.WeekNum)
		for _, bet := range bettingLines {
			if common.Contains(conferenceList, bet.HomeConference) || common.Contains(conferenceList, bet.AwayConference) {
				line, lineErr := common.PickLine(bet.Lines)
				lineText := fmt.Sprintf("* `%s @ %s`", bet.AwayTeam, bet.HomeTeam)
				if bet.HomeScore != nil && bet.AwayScore != nil {
					lineText += "- FINAL \n"
				} else {
					if lineErr != nil {
						lineText += "- No line available \n"
					} else {
						lineText += fmt.Sprintf(" (%d):  %s \n", bet.ID, line.FormattedSpread)
					}
				}

				response += lineText
			}
		}
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: response,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
}

func GetCFBGames() ([]external.CFBD_BettingLines, error) {
	cfbUrl := "https://api.collegefootballdata.com/lines?"
	pfWeekUrl := "https://api.perfectfall.com/week-season"

	weekResp, err := common.PFWrapper(pfWeekUrl)
	if err != nil {
		return []external.CFBD_BettingLines{}, err
	}
	if weekResp == nil {
		return []external.CFBD_BettingLines{}, fmt.Errorf("GetCFBGames: weekResp is empty")
	}
	if weekResp.Body == nil {
		return []external.CFBD_BettingLines{}, fmt.Errorf("GetCFBGames: weekResp.Body is empty")
	}
	defer weekResp.Body.Close()

	var calendar external.CalendarData
	err = json.NewDecoder(weekResp.Body).Decode(&calendar)
	if err != nil {
		return []external.CFBD_BettingLines{}, err
	}

	var weekNum = calendar.Week.WeekNum
	if weekNum > calendar.MaxRegWeek {
		weekNum = 1
	}
	linesUrl := fmt.Sprintf("%syear=%d&seasonType=%s&week=%d", cfbUrl, calendar.Season.Year, calendar.Week.WeekType, weekNum)
	linesResp, err := common.CFBDWrapper(linesUrl)
	if err != nil {
		return []external.CFBD_BettingLines{}, err
	}
	if linesResp != nil {
		if linesResp.Body != nil {
			defer linesResp.Body.Close()
		} else {
			return nil, errors.New("CFB Lines Empty")
		}
	} else {
		return nil, errors.New("CFB Lines Empty")
	}

	var bettingLines []external.CFBD_BettingLines
	err = json.NewDecoder(linesResp.Body).Decode(&bettingLines)
	if err != nil {
		return []external.CFBD_BettingLines{}, err
	}

	return bettingLines, nil
}

func GetCfbdBet(betid int) (external.CFBD_BettingLines, error) {
	cfbUrl := "https://api.collegefootballdata.com/lines?"
	pfWeekUrl := "https://api.perfectfall.com/week-season"

	weekResp, err := common.PFWrapper(pfWeekUrl)
	if err != nil {
		return external.CFBD_BettingLines{}, err
	}
	defer weekResp.Body.Close()

	var calendar external.CalendarData
	err = json.NewDecoder(weekResp.Body).Decode(&calendar)
	if err != nil {
		fmt.Printf("error parsing json err: %v", err)
		return external.CFBD_BettingLines{}, err
	}

	var weekNum = calendar.Week.WeekNum
	if weekNum > calendar.MaxRegWeek {
		weekNum = 1
	}
	linesUrl := fmt.Sprintf("%syear=%d&seasonType=%s&week=%d", cfbUrl, calendar.Season.Year, calendar.Week.WeekType, weekNum)
	linesResp, err := common.CFBDWrapper(linesUrl)
	if err != nil {
		return external.CFBD_BettingLines{}, err
	}
	defer linesResp.Body.Close()

	var bettingLines []external.CFBD_BettingLines
	err = json.NewDecoder(linesResp.Body).Decode(&bettingLines)
	if err != nil {
		fmt.Printf("error parsing json err: %v", err)
		return external.CFBD_BettingLines{}, err
	}

	for _, bet := range bettingLines {
		if bet.ID == betid {
			return bet, nil
		}
	}

	return external.CFBD_BettingLines{}, errors.New("bet not found")
}
