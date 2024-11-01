package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/services/betService"
	"perfectOddsBot/services/common"
)

func HandleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	switch i.ApplicationCommandData().Name {
	case "my-points":
		ShowPoints(s, i, db)
	case "leaderboard":
		ShowLeaderboard(s, i, db)
	case "create-bet":
		betService.CreateCustomBet(s, i, db)
	case "give-points":
		GivePoints(s, i, db)
	case "reset-points":
		ResetPoints(s, i, db)
	case "my-bets":
		betService.MyOpenBets(s, i, db)
	case "list-cfb-games":
		common.ListCFBGames(s, i)
	case "create-cfb-bet":
		betService.CreateCFBBet(s, i, db)
	case "set-betting-channel":
		SetBettingChannel(s, i, db)
	}
}

func RegisterCommands(s *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "list-cfb-games",
			Description: "List this weeks CFB games and their current lines",
		},
		{
			Name:        "leaderboard",
			Description: "Show the top users by points",
		},
		{
			Name:        "create-cfb-bet",
			Description: "Create a new College Football bet",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "game-id",
					Description: "Game ID",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "create-bet",
			Description: "🛡 Create a new bet - ADMIN ONLY",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "description",
					Description: "Description of the bet",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "option1",
					Description: "First betting option",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "option2",
					Description: "Second betting option",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "odds1",
					Description: "Odds for option 1 (e.g., +150 or -200) // *Optional: Default -110",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    false,
				},
				{
					Name:        "odds2",
					Description: "Odds for option 2 (e.g., +150 or -200) // *Optional: Default -110",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    false,
				},
			},
		},
		{
			Name:        "give-points",
			Description: "🛡 Give points to a user - ADMIN ONLY",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "user",
					Description: "User to give points to",
					Type:        discordgo.ApplicationCommandOptionUser,
					Required:    true,
				},
				{
					Name:        "amount",
					Description: "Amount of points to give",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
		{
			Name:        "reset-points",
			Description: "🛡 Reset all users' points to a default value - ADMIN ONLY",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "amount",
					Description: "Amount to reset points to (default 1000)",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    false,
				},
			},
		},
		{
			Name:        "set-betting-channel",
			Description: "🛡 Sets the current channel to the main channel for payouts to be sent to - ADMIN ONLY",
		},
		{
			Name:        "my-points",
			Description: "Show your current points",
		},
		{
			Name:        "my-bets",
			Description: "Show your current open, active bets",
		},
	}

	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))

	for i, cmd := range commands {
		rcmd, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
		if err != nil {
			return fmt.Errorf("cannot create '%v' command: %v", cmd.Name, err)
		}
		registeredCommands[i] = rcmd
	}

	return nil
}
