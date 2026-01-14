package cards

import "perfectOddsBot/models"

func registerUncommonCards(deck *[]models.Card) {
	uncommonCards := []models.Card{
		{
			ID:          4,
			Name:        "Petty Theft",
			Description: "Choose a user. Steal 5% of their points.",
			Rarity:      "Epic",
			Weight:      W_Uncommon,
			Handler:     handlePettyTheft,
		},
		// {
		// 	ID:             25,
		// 	Name:           "The Shield",
		// 	Description:    "Blocks the next 'Steal' or negative effect played against you.",
		// 	Rarity:         "Uncommon",
		// 	Weight:         W_Uncommon,
		// 	Handler:        handleShield,
		// 	AddToInventory: true,
		// },
		// {
		// 	ID:                   27,
		// 	Name:                 "Major Glitch",
		// 	Description:          "Everyone in the server gets 100 points.",
		// 	Rarity:               "Uncommon",
		// 	Weight:               W_Uncommon,
		// 	Handler:              handleMajorGlitch,
		// 	RoyaltyDiscordUserID: &[]string{"195444122578845696"}[0],
		// },
		// {
		// 	ID:             27,
		// 	Name:           "Double Down",
		// 	Description:    "The payout of your next winning bet is increased by 2x",
		// 	Rarity:         "Uncommon",
		// 	Weight:         W_Uncommon,
		// 	Handler:        handleDoubleDown,
		// 	AddToInventory: true,
		// },
		// {
		// 	ID:          28,
		// 	Name:        "Slippery Slope",
		// 	Description: "The next person to buy a card pays YOUR fee for it.",
		// 	Rarity:      "Uncommon",
		// 	Weight:      W_Uncommon,
		// 	Handler:     handleSlipperySlope,
		// },
		// {
		// 	ID:          29,
		// 	Name:        "The Jester",
		// 	Description: "Choose a user to mute for 5 minutes.",
		// 	Rarity:      "Uncommon",
		// 	Weight:      W_Uncommon,
		// 	Handler:     handleJester,
		// },
		// {
		// 	ID:          30,
		// 	Name:        "Stimulus Check",
		// 	Description: "Everyone in the server gets 50 points.",
		// 	Rarity:      "Uncommon",
		// 	Weight:      W_Uncommon,
		// 	Handler:     handleStimulusCheck,
		// },
		// // {
		// // 	ID:          31,
		// // 	Name:        "Grand Larceny",
		// // 	Description: "Steal 150 points from a chosen user.",
		// // 	Rarity:      "Uncommon",
		// // 	Weight:      W_Uncommon,
		// // 	Handler:     handleGrandLarceny,
		// // },
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
		// {
		// 	ID:          36,
		// 	Name:        "Quick Flip",
		// 	Description: "Flip a coin. Heads: Double your card cost back. Tails: Get nothing.",
		// 	Rarity:      "Uncommon",
		// 	Weight:      W_Uncommon,
		// 	Handler:     handleQuickFlip,
		// },
		// {
		// 	ID:          37,
		// 	Name:        "Loan Shark",
		// 	Description: "Get 500 points now, but you automatically lose 600 points in 3 days.",
		// 	Rarity:      "Uncommon",
		// 	Weight:      W_Uncommon,
		// 	Handler:     handleLoanShark,
		// },
	}

	// Add to deck
	*deck = append(*deck, uncommonCards...)
}
