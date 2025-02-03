package scheduler_jobs

import (
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models"
	"perfectOddsBot/services/betService"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/extService"
	"strconv"
)

func CheckSubscribedCFBTeam(s *discordgo.Session, db *gorm.DB) error {
	cfbdList, err := extService.GetCFBGames()
	if err != nil {
		common.SendError(s, nil, err, db)
	}

	var guildList []models.Guild
	result := db.Where("subscribed_team IS NOT NULL").Find(&guildList)
	if result.Error != nil {
		return result.Error
	}

	for _, guild := range guildList {
		for _, game := range cfbdList {
			if game.HomeTeam == *guild.SubscribedTeam || game.AwayTeam == *guild.SubscribedTeam {
				err = betService.AutoCreateCFBBet(s, db, guild.GuildID, guild.BetChannelID, strconv.Itoa(game.ID))
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func CheckSubscribedCBBTeam(s *discordgo.Session, db *gorm.DB) error {
	espnList, err := extService.GetCbbGames()
	if err != nil {
		return err
	}

	var guildList []models.Guild
	result := db.Where("subscribed_team IS NOT NULL").Find(&guildList)
	if result.Error != nil {
		return result.Error
	}

	for _, guild := range guildList {
		for _, game := range espnList {
			for _, team := range game.Competitions[0].Competitors {
				if team.Team.ShortDisplayName == *guild.SubscribedTeam {
					err = betService.AutoCreateCBBBet(s, db, guild.GuildID, guild.BetChannelID, game.ID)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
