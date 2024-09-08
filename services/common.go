package services

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"perfectOddsBot/models"
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

func FormatOdds(odds int) string {
	if odds > 0 {
		return fmt.Sprintf("+%d", odds)
	}
	return fmt.Sprintf("%d", odds)
}

func CalculatePayout(amount int, option int, bet models.Bet) int {
	var odds int
	if option == 1 {
		odds = bet.Odds1
	} else {
		odds = bet.Odds2
	}

	if odds > 0 {
		return amount + (amount*odds)/100
	}

	return amount + (amount*100)/-odds
}

func GetUsername(s *discordgo.Session, i *discordgo.InteractionCreate, userId string) string {
	member, err := s.GuildMember(i.GuildID, userId)
	username := "Unknown User"
	if err == nil {
		username = member.User.GlobalName
	}
	if username == "" {
		username = member.User.Username
	}

	return username
}
