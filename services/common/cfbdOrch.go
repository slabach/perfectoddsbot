package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"perfectOddsBot/models/external"
)

func ListCFBGames(s *discordgo.Session, i *discordgo.InteractionCreate) {
	cfbUrl := "https://api.collegefootballdata.com/lines?"
	pfWeekUrl := "https://api.perfectfall.com/week-season"
	conferenceList := []string{"Big Ten", "ACC", "SEC", "Big 12"}

	weekResp, err := PFWrapper(pfWeekUrl)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer weekResp.Body.Close()

	var calendar external.CalendarData
	err = json.NewDecoder(weekResp.Body).Decode(&calendar)
	if err != nil {
		fmt.Printf("error parsing json err: %v", err)
		return
	}

	var weekNum = calendar.Week.WeekNum
	if weekNum > calendar.MaxRegWeek {
		weekNum = 1
	}
	linesUrl := fmt.Sprintf("%syear=%d&seasonType=%s&week=%d", cfbUrl, calendar.Season.Year, calendar.Week.WeekType, weekNum)
	linesResp, err := CFBDWrapper(linesUrl)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer linesResp.Body.Close()

	var bettingLines []external.CFBD_BettingLines
	err = json.NewDecoder(linesResp.Body).Decode(&bettingLines)
	if err != nil {
		fmt.Printf("error parsing json err: %v", err)
		return
	}

	var response string
	if len(bettingLines) == 0 {
		response = "There are no lines for this week"
	} else {
		response = fmt.Sprintf("Lines for week %d - \n", calendar.Week.WeekNum)
		for _, bet := range bettingLines {
			if Contains(conferenceList, bet.HomeConference) || Contains(conferenceList, bet.AwayConference) {
				line, lineErr := PickLine(bet.Lines)
				lineText := fmt.Sprintf("* `%s @ %s`", bet.AwayTeam, bet.HomeTeam)
				if lineErr != nil {
					lineText += "- No line available \n"
				} else {
					lineText += fmt.Sprintf(" (%d):  %s \n", bet.ID, line.FormattedSpread)
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

	weekResp, err := PFWrapper(pfWeekUrl)
	if err != nil {
		return []external.CFBD_BettingLines{}, err
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
	linesResp, err := CFBDWrapper(linesUrl)
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

	weekResp, err := PFWrapper(pfWeekUrl)
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
	fmt.Println(linesUrl)
	linesResp, err := CFBDWrapper(linesUrl)
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
