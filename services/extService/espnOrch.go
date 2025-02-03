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
	"strconv"
)

func ListCBBGames(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	scoreboardUrl := "http://site.api.espn.com/apis/site/v2/sports/basketball/mens-college-basketball/scoreboard"
	linesUrl := "https://sports.core.api.espn.com/v2/sports/basketball/leagues/mens-college-basketball/events/%s/competitions/%s/odds"

	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if !guild.PremiumEnabled {
		common.SendError(s, i, fmt.Errorf("Your server must have the premium subscription in order to enable this feature"), db)
		return
	}

	scoreboardResp, err := common.ESPNWrapper(scoreboardUrl)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	defer scoreboardResp.Body.Close()

	var scoreboard external.ESPN_Scoreboard
	err = json.NewDecoder(scoreboardResp.Body).Decode(&scoreboard)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var response string
	if len(scoreboard.Events) == 0 {
		response = "There are no games to list"
	} else {
		response = fmt.Sprintf("Lines for %s - \n", scoreboard.Day)
		for _, event := range scoreboard.Events {
			for _, game := range event.Competitions {
				if game.Status.Type.Name != "" { //STATUS_FINAL
					linesResp, err := common.ESPNWrapper(fmt.Sprintf(linesUrl, event.ID, event.ID))
					if err != nil {
						common.SendError(s, i, err, db)
						return
					}
					defer linesResp.Body.Close()

					var lines external.ESPN_Lines
					err = json.NewDecoder(linesResp.Body).Decode(&lines)
					if err != nil {
						common.SendError(s, i, err, db)
						return
					}

					lineText := "No Line Available"
					line, lineErr := common.PickESPNLine(lines)
					if lineErr == nil {
						lineText = line.Details
					}

					response += fmt.Sprintf("* `%s` (%s): %s\n", event.Name, event.ID, lineText)
				}
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

func GetCbbGames() ([]external.ESPN_Event, error) {
	scoreboardUrl := "http://site.api.espn.com/apis/site/v2/sports/basketball/mens-college-basketball/scoreboard"

	scoreboardResp, err := common.ESPNWrapper(scoreboardUrl)
	if err != nil {
		return []external.ESPN_Event{}, err
	}
	defer scoreboardResp.Body.Close()

	var scoreboard external.ESPN_Scoreboard
	err = json.NewDecoder(scoreboardResp.Body).Decode(&scoreboard)
	if err != nil {
		return []external.ESPN_Event{}, err
	}

	if len(scoreboard.Events) > 0 {
		return scoreboard.Events, nil
	}

	return []external.ESPN_Event{}, errors.New("Unable to fetch list of CBB games")
}

func GetCbbLines(betid int) (external.ESPN_Lines, error) {
	linesUrl := fmt.Sprintf("https://sports.core.api.espn.com/v2/sports/basketball/leagues/mens-college-basketball/events/%s/competitions/%s/odds", strconv.Itoa(betid), strconv.Itoa(betid))

	linesResp, err := common.CFBDWrapper(linesUrl)
	if err != nil {
		return external.ESPN_Lines{}, err
	}
	defer linesResp.Body.Close()

	var bettingLines external.ESPN_Lines
	err = json.NewDecoder(linesResp.Body).Decode(&bettingLines)
	if err != nil {
		return external.ESPN_Lines{}, errors.New(fmt.Sprintf("error parsing json err: %v", err))
	}

	if len(bettingLines.Items) > 0 {
		return bettingLines, nil
	}

	return external.ESPN_Lines{}, errors.New("bet not found")
}

func GetCbbGame(eventId string) (external.ESPN_Event, error) {
	scoreboardUrl := "https://site.api.espn.com/apis/site/v2/sports/basketball/mens-college-basketball/scoreboard"

	scoreboardResp, err := common.ESPNWrapper(scoreboardUrl)
	if err != nil {
		return external.ESPN_Event{}, err
	}
	defer scoreboardResp.Body.Close()

	var scoreboard external.ESPN_Scoreboard
	err = json.NewDecoder(scoreboardResp.Body).Decode(&scoreboard)
	if err != nil {
		return external.ESPN_Event{}, err
	}

	if len(scoreboard.Events) == 0 {
		return external.ESPN_Event{}, errors.New("There are no games to list")
	} else {
		for _, event := range scoreboard.Events {
			if event.ID == eventId {
				return event, nil
			}
		}
	}

	return external.ESPN_Event{}, errors.New("Unable to find game")
}
