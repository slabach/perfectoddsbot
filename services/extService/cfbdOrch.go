package extService

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"runtime/debug"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func ListCFBGames(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in ListCFBGames", r)
			debug.PrintStack()
			common.SendError(s, i, fmt.Errorf("panic in ListCFBGames: %v", r), db)
		}
	}()

	cfbUrl := "https://api.collegefootballdata.com/lines?"
	pfWeekUrl := "https://api.perfectfall.com/week-season"
	conferenceList := []string{"Big Ten", "ACC", "SEC"}

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

	if calendar.Week == nil {
		common.SendError(s, i, fmt.Errorf("API did not return week data"), db)
		return
	}
	if calendar.Season == nil {
		common.SendError(s, i, fmt.Errorf("API did not return season data"), db)
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
					continue
				} else {
					if lineErr != nil {
						continue
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

func GetCFBGames() (_ []external.CFBD_BettingLines, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in GetCFBGames", r)
			debug.PrintStack()
			err = fmt.Errorf("panic recovered in GetCFBGames: %v", r)
		}
	}()

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

	if calendar.Week == nil {
		return []external.CFBD_BettingLines{}, fmt.Errorf("GetCFBGames: calendar.Week is nil - API did not return week data")
	}
	if calendar.Season == nil {
		return []external.CFBD_BettingLines{}, fmt.Errorf("GetCFBGames: calendar.Season is nil - API did not return season data")
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

func GetCfbdBet(betid int) (_ external.CFBD_BettingLines, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in GetCfbdBet", r)
			debug.PrintStack()
			err = fmt.Errorf("panic recovered in GetCfbdBet: %v", r)
		}
	}()

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

	if calendar.Week == nil {
		return external.CFBD_BettingLines{}, fmt.Errorf("GetCfbdBet: calendar.Week is nil - API did not return week data")
	}
	if calendar.Season == nil {
		return external.CFBD_BettingLines{}, fmt.Errorf("GetCfbdBet: calendar.Season is nil - API did not return season data")
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
