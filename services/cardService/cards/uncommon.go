package cards

import (
	"perfectOddsBot/models"
)

func registerUncommonCards(deck *[]models.Card) {
	uncommonCards := []models.Card{
		{
			ID:          4,
			Name:        "Petty Theft",
			Description: "Choose a user. Steal 50 of their points.",
			Rarity:      "Uncommon",
			Weight:      W_Uncommon,
			Handler:     handlePettyTheft,
		},
		{
			ID:             25,
			Name:           "The Shield",
			Description:    "Blocks the next 'Steal' or negative effect played against you.",
			Rarity:         "Uncommon",
			Weight:         W_Uncommon,
			Handler:        handleShield,
			AddToInventory: true,
		},
		{
			ID:                   26,
			Name:                 "Major Glitch",
			Description:          "Everyone in the server gets 100 points.",
			Rarity:               "Uncommon",
			Weight:               W_Uncommon,
			Handler:              handleMajorGlitch,
			RoyaltyDiscordUserID: &[]string{"195444122578845696"}[0],
		},
		{
			ID:                   27,
			Name:                 "Double Down",
			Description:          "The payout of your next winning bet is increased by 2x",
			Rarity:               "Uncommon",
			Weight:               W_Uncommon,
			Handler:              handleDoubleDown,
			AddToInventory:       true,
			RoyaltyDiscordUserID: &[]string{"130863485969104896"}[0],
		},
		{
			ID:             28,
			Name:           "Generous Donation",
			Description:    "You pay for the next (standard cost) card bought by another user.",
			Rarity:         "Uncommon",
			Weight:         W_Uncommon,
			Handler:        handleGenerousDonation,
			AddToInventory: true,
		},
		{
			ID:          29,
			Name:        "The Jester",
			Description: "Choose a user to mute for 15 minutes.",
			Rarity:      "Uncommon",
			Weight:      W_Uncommon,
			Handler:     handleJester,
		},
		{
			ID:          30,
			Name:        "Stimulus Check",
			Description: "Everyone in the server gets 50 points.",
			Rarity:      "Uncommon",
			Weight:      W_Uncommon,
			Handler:     handleStimulusCheck,
		},
		// // {
		// // 	ID:          32,
		// // 	Name:        "Identity Theft",
		// // 	Description: "Swap nicknames with another user for 24 hours.",
		// // 	Rarity:      "Uncommon",
		// // 	Weight:      W_Uncommon,
		// // 	Handler:     handleIdentityTheft,
		// // },
		// {
		// 	ID:             33,
		// 	Name:           "Pay It Forward",
		// 	Description:    "Select a user to give a free Card Draw to.",
		// 	Rarity:         "Uncommon",
		// 	Weight:         W_Uncommon,
		// 	Handler:        handlePayItForward,
		// 	AddToInventory: true,
		// },
		// {
		// 	ID:          34,
		// 	Name:        "Bet Freeze",
		// 	Description: "Prevent a specific user from placing bets for 2 hours.",
		// 	Rarity:      "Uncommon",
		// 	Weight:      W_Uncommon,
		// 	Handler:     handleFreeze,
		// },
		// {
		// 	ID:          35,
		// 	Name:        "Bet Insurance",
		// 	Description: "If you lose your next bet, get 50% of your wager back.",
		// 	Rarity:      "Uncommon",
		// 	Weight:      W_Uncommon,
		// 	Handler:     handleBetInsurance,
		// },
		{
			ID:          36,
			Name:        "Quick Flip",
			Description: "Flip a coin. Heads: Double your card cost back. Tails: Get nothing.",
			Rarity:      "Uncommon",
			Weight:      W_Uncommon,
			Handler:     handleQuickFlip,
		},
		// {
		// 	ID:                   63,
		// 	Name:                 "Green Shells",
		// 	Description:          "3 people are randomly selected to randomly lose between 1-25 points.",
		// 	Rarity:               "Uncommon",
		// 	Weight:               W_Uncommon,
		// 	Handler:              handleGreenShells,
		// 	RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		// },
		// {
		// 	ID:                   65,
		// 	Name:                 "Whack-a-Mole",
		// 	Description:          "Bonk 3-5 random players who each randomly lose between 1-10 points",
		// 	Rarity:               "Uncommon",
		// 	Weight:               W_Uncommon,
		// 	Handler:              handleWhackAMole,
		// 	RoyaltyDiscordUserID: &[]string{"447827835797766144"}[0],
		// },
	}

	// Add to deck
	*deck = append(*deck, uncommonCards...)
}
