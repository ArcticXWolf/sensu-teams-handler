package main

import (
	"fmt"
	"strings"
	"time"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	messagecard "github.com/atc0005/go-teams-notify/v2/messagecard"
	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	teamsWebhook string
	sensuUrl     string
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

func generateMessageCard(event *types.Event) (*messagecard.MessageCard, error) {
	message := messagecard.NewMessageCard()
	message.Title = generateMessageTitle(event)
	message.Summary = generateMessageSummary(event)
	message.ThemeColor = messageColorFromEventStatus(event.Check.GetStatus())
	for _, section := range generateMessageSections(event) {
		err := message.AddSection(section)
		if err != nil {
			return nil, fmt.Errorf("error during section add: %s", err)
		}
	}
	for _, action := range generateMessageActions(event) {
		err := message.AddPotentialAction(action)
		if err != nil {
			return nil, fmt.Errorf("error during potential action add: %s", err)
		}
	}

	return message, nil
}

func generateMessageTitle(event *types.Event) string {
	return fmt.Sprintf("%s: %s - %s", eventStatusString(event.Check.GetStatus()), event.Entity.Name, event.Check.GetName())
}

func generateMessageSummary(event *types.Event) string {
	return fmt.Sprintf("Entity %s changed status, triggering check is %s", event.Entity.Name, event.Check.GetName())
}

func generateMessageSections(event *types.Event) []*messagecard.Section {
	return []*messagecard.Section{
		{
			ActivityTitle: "Check history",
			ActivityText:  eventHistoryString(event),
			ActivityImage: eventStatusImageURL(event.Check.GetStatus()),
			Markdown:      true,
		},
		{
			StartGroup: true,
			Facts: []messagecard.SectionFact{
				{
					Name:  "Namespace",
					Value: event.Entity.Namespace,
				},
				{
					Name:  "Entity",
					Value: event.Entity.Name,
				},
				{
					Name:  "Check",
					Value: event.Check.Name,
				},
				{
					Name:  "Status",
					Value: eventStatusString(event.Check.GetStatus()),
				},
				{
					Name:  "Last Ok",
					Value: time.Unix(event.Check.LastOK, 0).Local().String(),
				},
				{
					Name:  "Event created",
					Value: time.Unix(event.Check.Issued, 0).Local().String(),
				},
			},
		},
	}
}

func generateMessageActions(event *types.Event) []*messagecard.PotentialAction {
	return []*messagecard.PotentialAction{
		{
			Type: "ActionCard",
			Name: "Show check output",
			PotentialActionActionCard: messagecard.PotentialActionActionCard{
				Inputs: []messagecard.PotentialActionActionCardInput{
					{
						ID:   "check-output",
						Type: "TextInput",
						PotentialActionActionCardInputTextInput: messagecard.PotentialActionActionCardInputTextInput{
							IsMultiline: true,
						},
						Title: "Check Output",
						Value: eventOutputTruncated(event),
					},
				},
			},
		},
		{
			Type: "OpenUri",
			Name: "Open Event in Sensu",
			PotentialActionOpenURI: messagecard.PotentialActionOpenURI{
				Targets: []messagecard.PotentialActionOpenURITarget{
					{
						OS:  "default",
						URI: fmt.Sprintf("%s/c/~/n/%s/events/%s/%s", plugin.sensuUrl, event.Entity.Namespace, event.Entity.Name, event.Check.GetName()),
					},
				},
			},
		},
		{
			Type: "OpenUri",
			Name: "Open Sensu",
			PotentialActionOpenURI: messagecard.PotentialActionOpenURI{
				Targets: []messagecard.PotentialActionOpenURITarget{
					{
						OS:  "default",
						URI: plugin.sensuUrl,
					},
				},
			},
		},
	}
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

func messageColorFromEventStatus(status uint32) string {
	switch status {
	case 0:
		return "00FF00"
	case 1:
		return "FFFF00"
	case 2:
		return "FF0000"
	default:
		return "FFFF00"
	}
}

func eventStatusImageURL(status uint32) string {
	switch status {
	case 0:
		return "https://iconmonstr.com/wp-content/g/gd/png.php?size=240&padding=0&icon=releases/source/7.2.0/png/iconmonstr-check-mark-circle-filled.png&in=iconmonstr-check-mark-circle-filled.png&bgShape=&bgColorR=200&bgColorG=200&bgColorB=200&iconColorR=0&iconColorG=255&iconColorB=0"
	case 1:
		return "https://iconmonstr.com/wp-content/g/gd/png.php?size=240&padding=0&icon=releases/source/7.0.0/png/iconmonstr-warning-filled.png&in=iconmonstr-warning-filled.png&bgShape=&bgColorR=200&bgColorG=200&bgColorB=200&iconColorR=255&iconColorG=255&iconColorB=0"
	case 2:
		return "https://iconmonstr.com/wp-content/g/gd/png.php?size=240&padding=0&icon=releases/source/7.2.0/png/iconmonstr-x-mark-circle-filled.png&in=iconmonstr-x-mark-circle-filled.png&bgShape=&bgColorR=200&bgColorG=200&bgColorB=200&iconColorR=255&iconColorG=0&iconColorB=0"
	default:
		return "https://iconmonstr.com/wp-content/g/gd/png.php?size=240&padding=0&icon=releases/source/2012/png/iconmonstr-help-2.png&in=iconmonstr-help-2.png&bgShape=&bgColorR=200&bgColorG=200&bgColorB=200&iconColorR=140&iconColorG=140&iconColorB=140"
	}
}

func eventHistoryString(event *types.Event) string {
	history := ""
	for _, item := range event.Check.History {
		history = fmt.Sprintf("%s<div style=\"background-color: #%s; margin: 0px 2px; float:left; width: 10px; height: 10px;\">&nbsp;</div>", history, messageColorFromEventStatus(item.GetStatus()))
	}
	return history
}

func eventOutputTruncated(event *types.Event) string {
	output := strings.TrimSpace(event.Check.Output)
	if len(output) > maxOutputLength {
		output = fmt.Sprintf("%s%s\n[...]", maxOutputLengthMessage, output[:maxOutputLength-len(maxOutputLengthMessage)])
	}
	return output
}

func executeFunction(event *types.Event) error {
	client := goteamsnotify.NewTeamsClient()

	message, err := generateMessageCard(event)
	if err != nil {
		return fmt.Errorf("could not generate Teams message because of error %s", err)
	}

	err = client.Send(plugin.teamsWebhook, message)
	if err != nil {
		message.Prepare()
		messageStr := message.PrettyPrint()
		return fmt.Errorf("could not send Teams message because of error %s \n message (len %d): %s", err, len(messageStr), messageStr)
	}
	return nil
}
