package services

import (
	"fmt"
	"perfectOddsBot/services/betService"
	cardService "perfectOddsBot/services/cardService"
	"perfectOddsBot/services/extService"
	"perfectOddsBot/services/guildService"
	"perfectOddsBot/services/interactionService"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

type commandInfo struct {
	name        string
	description string
	isAdmin     bool
	isPremium   bool
}

func HandleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	switch i.ApplicationCommandData().Name {
	case "help":
		ShowHelp(s, i, db)
	case "my-points":
		ShowPoints(s, i, db)
	case "my-stats":
		ShowStats(s, i, db)
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
		extService.ListCFBGames(s, i, db)
	case "create-cfb-bet":
		betService.CreateCFBBetSelector(s, i, db)
	case "set-betting-channel":
		guildService.SetBettingChannel(s, i, db)
	case "set-points-per-message":
		guildService.SetPointsPerMessage(s, i, db)
	case "set-starting-points":
		guildService.SetStartingPoints(s, i, db)
	case "list-cbb-games":
		extService.ListCBBGames(s, i, db)
	case "create-cbb-bet":
		betService.CreateCBBBetSelector(s, i, db)
	case "subscribe-to-team":
		interactionService.TeamSubscriptionMessage(s, i, db)
	case "create-parlay":
		betService.CreateParlaySelector(s, i, db)
	case "my-parlays":
		betService.MyParlays(s, i, db)
	case "draw-card":
		cardService.DrawCard(s, i, db)
	case "my-inventory":
		cardService.MyInventory(s, i, db)
	}
}

func ShowHelp(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	commands := []commandInfo{
		{"help", "Show this help message with all available commands", false, false},
		{"my-points", "Show your current points", false, false},
		{"my-stats", "Show your betting statistics", false, false},
		{"leaderboard", "Show the top users by points", false, false},
		{"my-bets", "Show your current open, active bets", false, false},
		{"my-parlays", "Show your active parlays", false, false},
		{"create-parlay", "Create a parlay by combining multiple open bets", false, false},
		{"draw-card", "Draw a random card from the deck (Costs X points, adds to pool)", false, false},
		{"my-inventory", "View the cards currently in your hand", false, false},
		{"create-bet", "Create a new bet", true, false},
		{"give-points", "Give points to a user", true, false},
		{"reset-points", "Reset all users' points to a default value", true, false},
		{"set-betting-channel", "Set the current channel as the main channel for payouts", true, false},
		{"set-points-per-message", "Set the amount of points users get per message", true, false},
		{"set-starting-points", "Set the amount of points new users start with", true, false},
		{"list-cfb-games", "List this week's CFB games and their current lines", false, true},
		{"list-cbb-games", "List the currently open CBB games", false, true},
		{"create-cfb-bet", "Create a new College Football bet", false, true},
		{"create-cbb-bet", "Create a new College Basketball bet", false, true},
		{"subscribe-to-team", "Choose a College team to subscribe to all CFB & CBB events for", true, true},
	}

	var fields []*discordgo.MessageEmbedField
	for _, cmd := range commands {
		description := cmd.description
		badges := ""
		if cmd.isAdmin {
			badges += "🛡 Admin "
		}
		if cmd.isPremium {
			badges += "★ Premium"
		}
		if badges != "" {
			description += " | " + badges
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("/%s", cmd.name),
			Value:  description,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📚 Available Commands",
		Description: "Here are all the available slash commands:",
		Fields:      fields,
		Color:       0x3498db,
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return
	}
}

func RegisterCommands(s *discordgo.Session) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "help",
			Description: "Show all available commands and their descriptions",
		},
		{
			Name:        "list-cfb-games",
			Description: "★ List this weeks CFB games and their current lines (PREMIUM)",
		},
		{
			Name:        "list-cbb-games",
			Description: "★ List the currently open CBB games (PREMIUM)",
		},
		{
			Name:        "create-cfb-bet",
			Description: "★ Create a new College Football bet (PREMIUM)",
		},
		{
			Name:        "create-cbb-bet",
			Description: "★ Create a new College Basketball bet (PREMIUM)",
		},
		{
			Name:        "leaderboard",
			Description: "Show the top users by points",
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
			Name:        "set-points-per-message",
			Description: "🛡 Sets the amount of points a user gets for each message they send in the server - ADMIN ONLY",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "points",
					Description: "Amount to reset points to (default 0.5)",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "my-points",
			Description: "Show your current points",
		},
		{
			Name:        "my-stats",
			Description: "Show your betting statistics",
		},
		{
			Name:        "my-bets",
			Description: "Show your current open, active bets",
		},
		{
			Name:        "subscribe-to-team",
			Description: "★ Choose a College team to subscribe to all CFB & CBB events for (PREMIUM)",
		},
		{
			Name:        "create-parlay",
			Description: "Create a parlay by combining multiple open bets",
		},
		{
			Name:        "my-parlays",
			Description: "Show your active parlays",
		},
		{
			Name:        "draw-card",
			Description: "Draw a random card from the deck (Costs 50 points, adds to pool)",
		},
		{
			Name:        "my-inventory",
			Description: "View the cards currently in your hand",
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
