package internal

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"nrmodule/atserial"
	"nrmodule/smsmanager"

	"github.com/bwmarrin/discordgo"
)

type DiscordBot struct {
	session   *discordgo.Session
	channelID string

	nri        *atserial.NRInterface
	smsManager *smsmanager.Manager

	commandPrefix string
}

func (bot *DiscordBot) OnNewSMS(sms atserial.NRModuleSMS) {

	embed := &discordgo.MessageEmbed{
		Title:       "New SMS Received",
		Color:       0x00ff00,
		Timestamp:   sms.Date.Format(time.RFC3339),
		Description: fmt.Sprintf(" Sender: %s\n Content: %s", sms.Sender, sms.Text),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Status", Value: sms.Status, Inline: true},
			{Name: "Internal Indices", Value: strconv.Itoa(sms.Indices), Inline: true},
		},
	}
	log.Println(embed)
}

func (bot *DiscordBot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {

	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.ChannelID != bot.channelID {
		return
	}

	if !strings.HasPrefix(m.Content, bot.commandPrefix) {
		return
	}

	log.Println("[DiscordBot] received message", m.Content)

	return
}

func NewDiscordBot(token string, channelID string, nri *atserial.NRInterface, smsManager *smsmanager.Manager) (*DiscordBot, error) {

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("Discord Bot session creation failed: %s", err)
	}

	bot := &DiscordBot{
		session:       dg,
		channelID:     channelID,
		nri:           nri,
		smsManager:    smsManager,
		commandPrefix: "/",
	}

	dg.AddHandler(bot.handleMessage)

	return bot, nil
}
