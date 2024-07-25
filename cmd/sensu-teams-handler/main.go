package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	adaptivecard "github.com/atc0005/go-teams-notify/v2/adaptivecard"
	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	teamsWebhook  string
	teamsMentions string
	sensuUrl      string
}

const (
	maxOutputLength        = 500
	maxOutputLengthMessage = "Output truncated because it was too long, check Event log in sensu: \n"
)

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-teams-handler",
			Short:    "MS Teams handler for Sensu",
			Keyspace: "sensu.io/plugins/sensu-teams-handler/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "teams-webhook",
			Env:       "TEAMS_WEBHOOK_URL",
			Argument:  "teams-webhook",
			Shorthand: "w",
			Secret:    true,
			Usage:     "URL of the teams webhook",
			Value:     &plugin.teamsWebhook,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "sensu-url",
			Env:       "SENSU_URL",
			Argument:  "sensu-url",
			Shorthand: "d",
			Usage:     "URL for the link to Sensu",
			Default:   "http://localhost:3000",
			Value:     &plugin.sensuUrl,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "mentions",
			Env:       "TEAMS_MENTIONS",
			Argument:  "mentions",
			Shorthand: "m",
			Secret:    true,
			Usage:     "A space separated list of ms teams email addresses that should be mentioned in the notifications",
			Value:     &plugin.teamsMentions,
		},
	}
)

func main() {
	handler := sensu.NewGoHandler(&plugin.PluginConfig, options, validateFunction, executeFunction)
	handler.Execute()
}

func validateFunction(event *types.Event) error {
	if len(plugin.teamsWebhook) == 0 {
		return fmt.Errorf("webhook url is not defined in flags nor environment")
	}
	return nil
}

func generateAdaptiveCard(event *types.Event) (adaptivecard.Card, error) {
	card := adaptivecard.NewCard()
	card.AddElement(false, *generateCardTitle(event))
	card.AddElement(false, *generateCardFacts(event))
	card.Actions = append(card.Actions, generateCardActions(event)...)
	card.MSTeams.Width = "full"

	// Sadly the teams mentioning feature seems broken at the moment, so we disable it
	mentions := generateCardMentions()
	if len(mentions) > 0 {
		card.MSTeams.Entities = append(card.MSTeams.Entities, mentions...)
	}

	return card, nil
}

func generateCardTitle(event *types.Event) *adaptivecard.Element {
	title := fmt.Sprintf("%c %s: %s - %s", eventStatusIcon(event.Check.GetStatus()), eventStatusString(event.Check.GetStatus()), event.Entity.Name, event.Check.GetName())
	textblock := adaptivecard.NewTitleTextBlock(title, false)
	textblock.Size = "extraLarge"
	return &textblock
}

func generateCardFacts(event *types.Event) *adaptivecard.Element {
	facts := adaptivecard.NewFactSet()
	facts.AddFact(adaptivecard.Fact{
		Title: "History (past \U00002192 now)",
		Value: eventStatusHistory(event),
	})
	facts.AddFact(adaptivecard.Fact{
		Title: "Namespace",
		Value: event.Entity.Namespace,
	})
	facts.AddFact(adaptivecard.Fact{
		Title: "Entity",
		Value: event.Entity.Name,
	})
	facts.AddFact(adaptivecard.Fact{
		Title: "Check",
		Value: event.Check.Name,
	})
	facts.AddFact(adaptivecard.Fact{
		Title: "Status",
		Value: eventStatusString(event.Check.GetStatus()),
	})
	facts.AddFact(adaptivecard.Fact{
		Title: "Last Ok",
		Value: time.Unix(event.Check.LastOK, 0).Local().String(),
	})
	facts.AddFact(adaptivecard.Fact{
		Title: "Event created",
		Value: time.Unix(event.Check.Issued, 0).Local().String(),
	})

	// Mentions
	mentions := generateCardMentionString()
	if mentions != "" {
		facts.AddFact(adaptivecard.Fact{
			Title: "Mentioned",
			Value: mentions,
		})
	}

	facts.IsSubtle = true
	element := adaptivecard.Element(facts)
	return &element
}

func generateCardActions(event *types.Event) []adaptivecard.Action {
	actions := []adaptivecard.Action{
		{
			Type:  adaptivecard.TypeActionShowCard,
			Title: "Show check output",
			Card:  generateCardEventOutput(event),
		},
		{
			Type:  adaptivecard.TypeActionShowCard,
			Title: "Show event annotations",
			Card:  generateCardEventAnnotations(event),
		},
		{
			Type:  adaptivecard.TypeActionOpenURL,
			Title: "Open in Sensu",
			URL:   eventSensuUrl(event),
		},
	}
	return actions
}

func generateCardEventOutput(event *types.Event) *adaptivecard.Card {
	card := adaptivecard.NewCard()
	card.AddElement(false, adaptivecard.NewTextBlock(eventOutputTruncated(event), false))
	return &card
}

func generateCardEventAnnotations(event *types.Event) *adaptivecard.Card {
	card := adaptivecard.NewCard()
	card.AddElement(false, adaptivecard.NewTextBlock(eventAnnotationsTruncated(event), false))
	return &card
}

func generateCardMentions() adaptivecard.Mentions {
	users := strings.Fields(plugin.teamsMentions)

	mentions := adaptivecard.Mentions{}
	for _, user := range users {
		mentions = append(mentions, adaptivecard.Mention{
			Type: adaptivecard.TypeMention,
			Text: fmt.Sprintf("<at>%s</at>", user),
			Mentioned: adaptivecard.Mentioned{
				ID:   user,
				Name: user,
			},
		})
	}
	return mentions
}

func generateCardMentionString() string {
	mentionstring := ""
	for _, mention := range generateCardMentions() {
		mentionstring = fmt.Sprintf("%s%s  ", mentionstring, mention.Text)
	}
	return strings.TrimSpace(mentionstring)
}

func eventStatusString(status uint32) string {
	switch status {
	case 0:
		return "Resolved"
	case 1:
		return "Warning"
	case 2:
		return "Critical"
	default:
		return "Undefined"
	}
}

func eventStatusIcon(status uint32) rune {
	switch status {
	case 0:
		return '\U00002705'
	case 1:
		return '\U000026A0'
	case 2:
		return '\U0000274C'
	default:
		return '\U000026A0'
	}
}

func eventStatusHistory(event *types.Event) string {
	history := ""
	for _, item := range event.Check.History {
		history = fmt.Sprintf("%s%c ", history, eventStatusIcon(item.GetStatus()))
	}
	return history
}

func eventOutputTruncated(event *types.Event) string {
	output := strings.TrimSpace(event.Check.Output)
	output = strings.ReplaceAll(output, "\n", "\n\n")
	if len(output) > maxOutputLength {
		output = fmt.Sprintf("%s%s\n[...]", maxOutputLengthMessage, output[:maxOutputLength-len(maxOutputLengthMessage)])
	}
	return output
}

func eventAnnotationsTruncated(event *types.Event) string {
	output := ""
	for key, value := range event.GetObjectMeta().Annotations {
		output = fmt.Sprintf("%s%s\n%s\n\n", output, key, value)
	}
	if len(output) > maxOutputLength {
		output = fmt.Sprintf("%s%s\n[...]", maxOutputLengthMessage, output[:maxOutputLength-len(maxOutputLengthMessage)])
	}
	return output
}

func eventSensuUrl(event *types.Event) string {
	return fmt.Sprintf("%s/c/~/n/%s/events/%s/%s", plugin.sensuUrl, event.Entity.Namespace, event.Entity.Name, event.Check.GetName())
}

func executeFunction(event *types.Event) error {
	client := goteamsnotify.NewTeamsClient()

	card, err := generateAdaptiveCard(event)
	if err != nil {
		return fmt.Errorf("could not generate Adaptive Card because of error %s", err)
	}

	message, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return fmt.Errorf("could not generate Teams message because of error %s \n card: %v", err, card)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.SendWithContext(ctx, plugin.teamsWebhook, message)

	if err != nil {
		message.Prepare()
		messageStr := message.PrettyPrint()
		return fmt.Errorf("could not send Teams message because of error %s \n message (len %d): %s", err, len(messageStr), messageStr)
	}
	return nil
}
