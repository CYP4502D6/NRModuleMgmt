package internal

import (
	"fmt"
	"log"
	"time"
	"strconv"
	"strings"

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

	_, err := bot.session.ChannelMessageSendEmbed(bot.channelID, embed)
	if err != nil {
		log.Println("[DiscordBot] send new sms failed,", err)
	}
}

func (bot *DiscordBot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {

	log.Println("[DiscordBot]", m.Content, m.ChannelID, m.Author)
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.ChannelID != bot.channelID {
		log.Println("[DiscordBot] channel id wrong")
		return
	}

	if !strings.HasPrefix(m.Content, bot.commandPrefix) {
		log.Println("[DiscordBot] didn't have prefix")
		return
	}

	args := strings.Fields(m.Content[len(bot.commandPrefix):])
	if len(args) == 0 {
		return
	}

	command := strings.ToLower(args[0])

	switch command {

	case "info":
		bot.processInfoCmd(s, m, args[1:])

	case "check":
		checkingMsg, _ := s.ChannelMessageSend(m.ChannelID, "Manual check trigger signal sent, checking in progress")

		bot.smsManager.TriggerCheck()
		time.Sleep(2 * time.Second)
		
		s.ChannelMessageEdit(m.ChannelID, checkingMsg.ID, "Signal has been triggered, a new SMS will be sent")

	default:
		bot.session.ChannelMessageSend(m.ChannelID, "unknown command, type !help to view help") 
	}
	
	return
}

func (bot *DiscordBot) processInfoCmd(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) == 0 {
		bot.session.ChannelMessageSend(m.ChannelID, "Please Please specify the information type: module, network, signal")
		return
	}

	infoType := strings.ToLower(args[0])
	var result map[string]interface{}
	var err error
	var resultStr string

	switch infoType {

	case "module":
		resultStr = "module info:\n"
		result, err = bot.nri.FetchModuleInfo()

	case "network":
		resultStr = "network info:\n"
		result, err = bot.nri.FetchNetworkInfo()

	case "signal":
		resultStr = "signal info:\n"
		networkMode, _ := bot.nri.GetInfo("NetworkMode")
		networkModeStr, ok := networkMode.(string)

		if ok {
			if strings.Contains(networkMode.(string), "NR") || strings.Contains(networkMode.(string), "LTE") {
				result, err = bot.nri.FetchSignalInfo(networkModeStr)
			} else {
				err = fmt.Errorf("Unrecognized network mode")
			}
		} else {
			err = fmt.Errorf("Unable to obtain network mode")
		}
		
	default:
		resultStr = fmt.Sprintf("%s info:\n", args[0])
		var info interface{}
		info, err = bot.nri.GetInfo(args[0])
		if err == nil {
			result = map[string]interface{}{args[0]: info}
		}
	}

	if err != nil {
		bot.session.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Information retrieval failed: %v", err))
		return
	}

	for key, value := range result {
		resultStr += fmt.Sprintf("    %s: %v\n", key, value)
	}

	bot.session.ChannelMessageSend(m.ChannelID, resultStr)
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

func (bot *DiscordBot) Start() error {

	err := bot.session.Open()
	if err != nil {
		return fmt.Errorf("Discord connect failed! %w", err)
	}
	log.Println("[DiscordBot] listening channel", bot.channelID)
	bot.smsManager.RegisterObserver(bot)

	return nil
}

func (bot *DiscordBot) Stop() error {

	return bot.session.Close()
}
