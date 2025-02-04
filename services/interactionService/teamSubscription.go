package interactionService

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"perfectOddsBot/models/external"
	"perfectOddsBot/services/common"
	"perfectOddsBot/services/guildService"
	"strconv"
	"strings"
)

var paginatedOptions [][]discordgo.SelectMenuOption

func TeamSubscriptionMessage(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB) {
	if !common.IsAdmin(s, i) {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "You are not authorized to use this command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if err != nil {
			common.SendError(s, i, err, db)
			return
		}
		return
	}

	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	if !guild.PremiumEnabled {
		common.SendError(s, i, fmt.Errorf("Your server must have the premium subscription in order to enable this feature"), db)
		return
	}

	pfTeamUrl := "https://api.perfectfall.com/school-list"

	teamListResp, err := common.PFWrapper(pfTeamUrl)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}
	defer teamListResp.Body.Close()

	var teamList external.TeamList
	err = json.NewDecoder(teamListResp.Body).Decode(&teamList)
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	var selectOptions []discordgo.SelectMenuOption
	for _, team := range teamList {
		selectOptions = append(selectOptions, discordgo.SelectMenuOption{
			Label:       team.Name,
			Value:       team.Name,
			Description: fmt.Sprintf("%s %s", team.Name, team.Mascot),
			Emoji:       nil,
			Default:     false,
		})
	}

	minValues := 1
	for i := 0; i < len(selectOptions); i += 25 {
		end := i + 25
		if end > len(selectOptions) {
			end = len(selectOptions)
		}
		paginatedOptions = append(paginatedOptions, selectOptions[i:end])
	}

	currentPage := 0
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Select a team (Page %d/%d):", currentPage+1, len(paginatedOptions)),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    "subscribe_to_team_submit",
							Placeholder: "Select a team",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     paginatedOptions[currentPage],
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Previous",
							CustomID: "subscribe_to_team_previous_page_0",
							Style:    discordgo.PrimaryButton,
							Disabled: true,
						},
						discordgo.Button{
							Label:    "Next",
							CustomID: "subscribe_to_team_next_page_0",
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == len(paginatedOptions)-1,
						},
					},
				},
			},
		},
	})
	if err != nil {
		common.SendError(s, i, err, db)
		return
	}

	return
}

func HandlePagination(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	if i.Type != discordgo.InteractionMessageComponent {
		return nil
	}

	currentPage := 0

	if strings.HasPrefix(customID, "subscribe_to_team_previous_page") {
		pageStr := strings.TrimPrefix(customID, "subscribe_to_team_previous_page_")
		currentPage, _ = strconv.Atoi(pageStr)
		currentPage--
	}
	if strings.HasPrefix(customID, "subscribe_to_team_next_page") {
		pageStr := strings.TrimPrefix(customID, "subscribe_to_team_next_page_")
		currentPage, _ = strconv.Atoi(pageStr)
		currentPage++
	}
	fmt.Println(currentPage)

	if currentPage < 0 {
		currentPage = 0
	}
	if currentPage >= len(paginatedOptions) {
		currentPage = len(paginatedOptions) - 1
	}

	minValues := 1
	content := fmt.Sprintf("Select a team (Page %d/%d):", currentPage+1, len(paginatedOptions))
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    "subscribe_to_team_submit",
							Placeholder: "Select a team",
							MinValues:   &minValues,
							MaxValues:   1,
							Options:     paginatedOptions[currentPage],
						},
					},
				},
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    "Previous",
							CustomID: fmt.Sprintf("subscribe_to_team_previous_page_%d", currentPage),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == 0,
						},
						discordgo.Button{
							Label:    "Next",
							CustomID: fmt.Sprintf("subscribe_to_team_next_page_%d", currentPage),
							Style:    discordgo.PrimaryButton,
							Disabled: currentPage == len(paginatedOptions)-1,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func HandleTeamSubscribeSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, db *gorm.DB, customID string) error {
	selectedOption := i.MessageComponentData().Values[0]

	guild, err := guildService.GetGuildInfo(s, db, i.GuildID, i.ChannelID)
	if err != nil {
		common.SendError(s, i, err, db)
	} else {
		guild.SubscribedTeam = &selectedOption
		db.Save(guild)
	}

	minValues := 1
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content: "Subscription set successfully",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.SelectMenu{
							MenuType:    discordgo.StringSelectMenu,
							CustomID:    "subscribe_to_team_submit",
							Placeholder: selectedOption,
							MinValues:   &minValues,
							MaxValues:   1,
							Disabled:    true,
							Options: []discordgo.SelectMenuOption{
								{
									Label: selectedOption,
									Value: selectedOption,
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return err
	}

	return nil
}
