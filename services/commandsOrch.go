package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func HandleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	switch i.ApplicationCommandData().Name {
	case "points":
		ShowPoints(s, i, db)
	case "leaderboard":
		ShowLeaderboard(s, i, db)
	case "createbet":
		CreateBet(s, i, db)
	case "givepoints":
		GivePoints(s, i, db)
	case "resetpoints":
		ResetPoints(s, i, db)
	case "mybets":
		MyOpenBets(s, i, db)
	}
}

func RegisterCommands(s *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "points",
			Description: "Show your current points",
		},
		{
			Name:        "leaderboard",
			Description: "Show the top users by points",
		},
		{
			Name:        "createbet",
			Description: "Create a new bet",
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
					Description: "Odds for option 1 (e.g., +150 or -200)",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
				{
					Name:        "odds2",
					Description: "Odds for option 2 (e.g., +150 or -200)",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
		{
			Name:        "placebet",
			Description: "Place a bet on an active bet",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "betid",
					Description: "ID of the bet",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
				{
					Name:        "option",
					Description: "Option to bet on (1 or 2)",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Option 1", Value: 1},
						{Name: "Option 2", Value: 2},
					},
				},
				{
					Name:        "amount",
					Description: "Amount of points to bet",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
		{
			Name:        "givepoints",
			Description: "Give points to a user",
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
			Name:        "resetpoints",
			Description: "Reset all users' points to a default value",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "amount",
					Description: "Amount to reset points to (default 1000)",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    false,
				},
			},
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
