package interactionService

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestHandleCardUserSelection_InvalidCustomID(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionMessageComponent,
			Data: discordgo.MessageComponentInteractionData{
				CustomID: "bad_format",
			},
		},
	}

	if err := HandleCardUserSelection(nil, i, nil); err == nil {
		t.Fatalf("expected error for invalid custom ID")
	}
}

func TestHandleCardBetSelection_InvalidCustomID(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionMessageComponent,
			Data: discordgo.MessageComponentInteractionData{
				CustomID: "bad_format",
			},
		},
	}

	if err := HandleCardBetSelection(nil, i, nil); err == nil {
		t.Fatalf("expected error for invalid custom ID")
	}
}

func TestHandleCardOptionSelection_InvalidCustomID(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionMessageComponent,
			Data: discordgo.MessageComponentInteractionData{
				CustomID: "bad_format",
			},
		},
	}

	if err := HandleCardOptionSelection(nil, i, nil); err == nil {
		t.Fatalf("expected error for invalid custom ID")
	}
}

func TestHandlePlayCardSelection_InvalidCustomID(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionMessageComponent,
			Data: discordgo.MessageComponentInteractionData{
				CustomID: "bad_format",
			},
		},
	}

	if err := HandlePlayCardSelection(nil, i, nil); err == nil {
		t.Fatalf("expected error for invalid custom ID")
	}
}

func TestHandlePlayCardBetSelection_InvalidCustomID(t *testing.T) {
	i := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionMessageComponent,
			Data: discordgo.MessageComponentInteractionData{
				CustomID: "bad_format",
			},
		},
	}

	if err := HandlePlayCardBetSelection(nil, i, nil); err == nil {
		t.Fatalf("expected error for invalid custom ID")
	}
}
