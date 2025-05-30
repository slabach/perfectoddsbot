package common

import (
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"perfectOddsBot/models"
	"perfectOddsBot/models/external"
	"strconv"
)

func IsAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	member, err := s.GuildMember(i.GuildID, i.Member.User.ID)
	if err != nil {
		log.Printf("Error fetching guild member: %v", err)
		return false
	}

	for _, roleID := range member.Roles {
		role, err := s.State.Role(i.GuildID, roleID)
		if err != nil || role == nil {
			roles, err := s.GuildRoles(i.GuildID)
			if err != nil {
				log.Printf("Error fetching roles from API: %v", err)
				continue
			}

			for _, r := range roles {
				if r.ID == roleID {
					role = r
					break
				}
			}

			if role == nil {
				log.Printf("Role %s not found in guild %s", roleID, i.GuildID)
				continue
			}
		}

		if role.Permissions&discordgo.PermissionAdministrator != 0 {
			return true
		}
	}

	return false
}

func SendError(s *discordgo.Session, i *discordgo.InteractionCreate, err error, db *gorm.DB) {
	fmt.Println(err)

	guildId := ""
	if i != nil {
		guildId = i.GuildID
		localErr := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("An error occured: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		if localErr != nil {
			log.Printf("Error sending interaction: %v", localErr)
		}
	}
	errLog := models.ErrorLog{
		GuildID: guildId,
		Message: fmt.Sprintf("%v", err),
	}
	db.Create(&errLog)
}

func FormatOdds(odds float64) string {
	response := ""

	if odds == float64(int(odds)) {
		response = strconv.Itoa(int(odds))
	} else {
		response = fmt.Sprintf("%.1f", odds)
	}

	if odds > 0 {
		return fmt.Sprintf("+%s", response)
	}
	return response
}

func CalculatePayout(amount int, option int, bet models.Bet) float64 {
	var odds int
	if option == 1 {
		odds = bet.Odds1
	} else {
		odds = bet.Odds2
	}

	if odds > 0 {
		return float64(amount + (amount*odds)/100)
	}

	return float64(amount + (amount*100)/-odds)
}

func GetUsername(s *discordgo.Session, guildId string, userId string) string {
	member, err := s.GuildMember(guildId, userId)
	username := "Unknown User"
	if err == nil {
		username = member.User.GlobalName
	}
	if username == "" {
		username = member.User.Username
	}

	return username
}

func CFBDWrapper(requestUrl string) (*http.Response, error) {
	var cfbdKey string
	getEnv, ok := os.LookupEnv("ENV")
	if ok == false {
		return nil, fmt.Errorf("ENV not found")
	}

	if getEnv == "production" {
		cfbdKey, ok = os.LookupEnv("CFBD_TOKEN")
		if ok == false {
			return nil, fmt.Errorf("CFBD_TOKEN not set in environment variables")
		}
	} else {
		cfbdKey = os.Getenv("CFBD_TOKEN")
		if cfbdKey == "" {
			return nil, fmt.Errorf("CFBD_TOKEN not set in environment variables")
		}
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", cfbdKey))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func ESPNWrapper(requestUrl string) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func PFWrapper(requestUrl string) (*http.Response, error) {
	var pfKey string
	getEnv, ok := os.LookupEnv("ENV")
	if ok == false {
		return nil, fmt.Errorf("ENV not found")
	}

	if getEnv == "production" {
		pfKey, ok = os.LookupEnv("PF_Token")
		if ok == false {
			return nil, fmt.Errorf("PF_Token not set in environment variables")
		}
	} else {
		pfKey = os.Getenv("PF_Token")
		if pfKey == "" {
			return nil, fmt.Errorf("PF_Token not set in environment variables")
		}
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-Api-Key", fmt.Sprintf("%s", pfKey))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		fmt.Println(resp.StatusCode)
		resp.Body.Close()
		return nil, err
	}
	return resp, nil
}

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func PickLine(lines []external.CFBD_Line) (*external.CFBD_Line, error) {
	preferredProviders := []string{"ESPN Bet", "DraftKings", "Bovada"}

	for _, provider := range preferredProviders {
		for _, line := range lines {
			if line.Provider == provider {
				return &line, nil
			}
		}
	}

	return nil, errors.New("no line selected")
}

func PickESPNLine(lines external.ESPN_Lines) (*external.ESPN_Line, error) {
	preferredProviders := []string{"ESPN BET", "DraftKings", "Bovada"}

	for _, provider := range preferredProviders {
		for _, line := range lines.Items {
			if line.Provider.Name == provider {
				return &line, nil
			}
		}
	}

	return nil, errors.New("no line selected")
}
