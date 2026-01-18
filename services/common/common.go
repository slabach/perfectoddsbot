package common

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"perfectOddsBot/models"
	"perfectOddsBot/models/external"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gorm.io/gorm"
)

func IsAdmin(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	// Use member data from the interaction - no privileged intent needed
	if i.Member == nil {
		return false
	}

	for _, roleID := range i.Member.Roles {
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

// CalculateSimplePayout calculates payout at even odds (+100), returning 2x the original amount
func CalculateSimplePayout(amount float64) float64 {
	return amount * 2.0
}

// CalculateParlayOddsMultiplier calculates the combined odds multiplier for a parlay
// Takes a slice of odds (as integers in American format) and returns the multiplier
func CalculateParlayOddsMultiplier(oddsList []int) float64 {
	if len(oddsList) == 0 {
		return 1.0
	}

	multiplier := 1.0
	for _, odds := range oddsList {
		if odds > 0 {
			// Positive odds: multiplier = (odds/100) + 1
			multiplier *= (float64(odds) / 100.0) + 1.0
		} else {
			// Negative odds: multiplier = (100/abs(odds)) + 1
			multiplier *= (100.0 / float64(-odds)) + 1.0
		}
	}

	return multiplier
}

// CalculateParlayPayout calculates the payout for a parlay given the amount and odds multiplier
func CalculateParlayPayout(amount int, oddsMultiplier float64) float64 {
	return float64(amount) * oddsMultiplier
}

// GetOddsFromBet returns the odds for a specific option in a bet
func GetOddsFromBet(bet models.Bet, option int) int {
	if option == 1 {
		return bet.Odds1
	}
	return bet.Odds2
}

// GetUsernameFromUser extracts username from a discordgo.User object
func GetUsernameFromUser(user *discordgo.User) string {
	if user == nil {
		return "Unknown User"
	}
	username := user.GlobalName
	if username == "" {
		username = user.Username
	}
	if username == "" {
		return "Unknown User"
	}
	return username
}

// UpdateUserUsername updates the username field in the database if it's different
func UpdateUserUsername(db *gorm.DB, user *models.User, username string) {
	if user.Username == nil || *user.Username != username {
		user.Username = &username
		db.Save(user)
	}
}

func GetUsername(s *discordgo.Session, guildId string, userId string) string {
	// This function is deprecated - use GetUsernameWithDB instead
	// Try to get from guild member if available in state (without members intent, this may be empty)
	if guild, err := s.State.Guild(guildId); err == nil && guild != nil {
		for _, member := range guild.Members {
			if member.User != nil && member.User.ID == userId {
				return GetUsernameFromUser(member.User)
			}
		}
	}
	return "Unknown User"
}

// GetUsernameWithDB gets username from database first, then falls back to state cache
func GetUsernameWithDB(db *gorm.DB, s *discordgo.Session, guildId string, userId string) string {
	// First try to get from database (most reliable)
	var user models.User
	if err := db.Where("discord_id = ? AND guild_id = ?", userId, guildId).First(&user).Error; err == nil {
		if user.Username != nil && *user.Username != "" {
			return *user.Username
		}
	}

	// Fallback to state cache (limited without members intent)
	if guild, err := s.State.Guild(guildId); err == nil && guild != nil {
		for _, member := range guild.Members {
			if member.User != nil && member.User.ID == userId {
				return GetUsernameFromUser(member.User)
			}
		}
	}

	return "Unknown User"
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
	preferredProviders := []string{"ESPN Bet", "Draft Kings", "DraftKings", "Bovada"}

	// First pass: prefer lines with both spread and moneyline data
	for _, provider := range preferredProviders {
		for _, line := range lines {
			if line.Provider == provider && line.Spread != nil && line.HomeMoneyline != nil && line.AwayMoneyline != nil {
				return &line, nil
			}
		}
	}

	// Second pass: any line from preferred provider with spread
	for _, provider := range preferredProviders {
		for _, line := range lines {
			if line.Provider == provider && line.Spread != nil {
				return &line, nil
			}
		}
	}

	return nil, errors.New("no line selected")
}

func PickESPNLine(lines external.ESPN_Lines) (*external.ESPN_Line, error) {
	preferredProviders := []string{"ESPN BET", "Draft Kings", "DraftKings", "Bovada"}

	// First pass: prefer lines with both spread and moneyline data
	for _, provider := range preferredProviders {
		for _, line := range lines.Items {
			if line.Provider.Name == provider && line.HomeTeamOdds.MoneyLine != 0 && line.AwayTeamOdds.MoneyLine != 0 {
				return &line, nil
			}
		}
	}

	// Second pass: any line from preferred provider
	for _, provider := range preferredProviders {
		for _, line := range lines.Items {
			if line.Provider.Name == provider {
				return &line, nil
			}
		}
	}

	return nil, errors.New("no line selected")
}

func GetSchoolName(s string) string {
	parts := strings.Fields(s)
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		// Check if last starts with + or - and is a number
		if strings.HasPrefix(last, "+") || strings.HasPrefix(last, "-") {
			// Try parsing
			_, err := strconv.ParseFloat(last, 64)
			if err == nil {
				return strings.TrimSuffix(s, " "+last)
			}
		}
	}
	return s
}

// CalculateBetEntryWin determines if a bet entry wins based on the option, score difference, and spread.
// Parameters:
//   - option: 1 for home team + spread, 2 for away team - spread
//   - scoreDiff: homeScore - awayScore
//   - spread: spread value stored from home team's perspective
//   - If away team is favored by 3.5, spread = +3.5
//   - If home team is favored by 3.5, spread = -3.5
//
// Returns true if the bet entry wins, false otherwise.
func CalculateBetEntryWin(option int, scoreDiff int, spread float64) bool {
	if option == 1 {
		// Option 1: homeTeam + spread wins if (homeScore + spread) > awayScore
		// i.e., if scoreDiff > -spread
		return float64(scoreDiff) > -spread
	} else {
		// Option 2: awayTeam - spread wins if (awayScore - spread) > homeScore
		// i.e., if (awayScore - homeScore) > spread
		// i.e., if -scoreDiff > spread
		return float64(-scoreDiff) > spread
	}
}
