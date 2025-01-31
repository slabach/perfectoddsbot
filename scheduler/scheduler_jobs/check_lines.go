package scheduler_jobs

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/cfbdService"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/messageService"
	"strconv"
	"time"
)

func CheckCFBLines(s *discordgo.Session, db *gorm.DB) error {
	var betList []models.Bet

	result := db.Where("paid = 0 AND active = 1 AND cfbd_id IS NOT NULL").Find(&betList)
	if result.Error != nil {
		return result.Error
	}

	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		return err
	}

	// Get the current time in EST
	currentTimeEST := time.Now().In(est)
	formattedTime := currentTimeEST.Format("Mon 03:04 pm MST")

	cfbdList, err := cfbdService.GetCFBGames()
	if err != nil {
		return err
	}

	betMap := make(map[int]external.CFBD_BettingLines)
	for _, obj := range cfbdList {
		betMap[obj.ID] = obj
	}

	for _, bet := range betList {
		betCfbdId, _ := strconv.Atoi(*bet.CfbdID)
		if obj, found := betMap[betCfbdId]; found {
			line, lineErr := common.PickLine(obj.Lines)
			if lineErr != nil {
				fmt.Println(lineErr)
				continue
			}

			lineValue, err := strconv.ParseFloat(line.Spread, 64)
			if err != nil {
				return err
			}

			bet.Option1 = fmt.Sprintf("%s %s", obj.HomeTeam, common.FormatOdds(lineValue))
			bet.Option2 = fmt.Sprintf("%s %s", obj.AwayTeam, common.FormatOdds(lineValue*-1))
			bet.Spread = &lineValue
			db.Save(&bet)

			buttons := messageService.GetBetOnlyButtonsList(bet.Option1, bet.Option2, bet.ID)
			embed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("📢 Bet Lines Updated %s (Will Auto Close & Resolve)", formattedTime),
				Description: bet.Description,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  fmt.Sprintf("1️⃣ %s", bet.Option1),
						Value: fmt.Sprintf("Odds: %s", common.FormatOdds(-110)),
					},
					{
						Name:  fmt.Sprintf("2️⃣ %s", bet.Option2),
						Value: fmt.Sprintf("Odds: %s", common.FormatOdds(-110)),
					},
				},
				Color: 0x3498db,
			}

			_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
				ID:      *bet.MessageID,
				Channel: bet.ChannelID,
				Embeds:  &[]*discordgo.MessageEmbed{embed},
				Components: &[]discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: buttons,
					},
				},
			})

			var secondaryMsgs []models.BetMessage
			secondaryResult := db.Where("active = 1 AND bet_id = ?", bet.ID).Find(&secondaryMsgs)
			if secondaryResult.Error != nil {
				return errors.New(fmt.Sprintf("Error finding secondary messageService: %v", secondaryResult.Error))
			}
			if len(secondaryMsgs) > 0 {
				for _, msg := range secondaryMsgs {
					_, err = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
						ID:      *msg.MessageID,
						Channel: msg.ChannelID,
						Embeds:  &[]*discordgo.MessageEmbed{embed},
						Components: &[]discordgo.MessageComponent{
							discordgo.ActionsRow{
								Components: buttons,
							},
						},
					})
				}
			}

			if err != nil {
				continue
			}
		}
	}

	return nil
}
