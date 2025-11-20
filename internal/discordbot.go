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

func (bot *DiscordBot) sendSMSRecord(m *discordgo.MessageCreate, record *smsmanager.SMSRecord) {
	
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("SMS Details [ID: %d]", record.DBID),
		Color:       0xff9900,
		Description: fmt.Sprintf(" Sender: %s\n Content: %s", record.Sender, record.Text),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Status", Value: record.Status, Inline: true},
			{Name: "Reception Time", Value: record.Date.Format("2006-01-02 15:04:05"), Inline: true},
			{Name: "Module Indices", Value: strconv.Itoa(record.Indices), Inline: true},
			{Name: "Warehousing Time", Value: record.CreatAt.Format("2006-01-02 15:04:05"), Inline: true},
		},
	}
	
	_, _ = bot.session.ChannelMessageSendEmbed(m.ChannelID, embed)
}

func (bot *DiscordBot) sendSMSRecordList(m *discordgo.MessageCreate, records []*smsmanager.SMSRecord, start int64, end int64) {

	if len(records) == 0 {
		bot.session.ChannelMessageSend(m.ChannelID, fmt.Sprintf("No SMS message found within the ID range: %d-%d", start, end))
		return
	}

	batchSize := 5
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]

		embed := &discordgo.MessageEmbed{
			Title: fmt.Sprintf("SMS Record List [%d-%d] (Total: %d)", i, end, len(records)),
			Color: 0xff9900,
		}

		for _, record := range batch {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name: fmt.Sprintf("ID: %d | From: %s", record.DBID, record.Sender),
				Value: fmt.Sprintf("Content: %s\nTime: %s",record.Text,
					record.Date.Format("01-02 15:04")),
				Inline: false,
			})
		}

		_, _ = bot.session.ChannelMessageSendEmbed(m.ChannelID, embed)
	}
}

func (bot *DiscordBot) formatInfoEmbed(infoType string, data map[string]interface{}) *discordgo.MessageEmbed {

	fields := make([]*discordgo.MessageEmbedField, 0, len(data))
	for k, v := range data {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   k,
			Value:  fmt.Sprintf("%v", v),
			Inline: true,
		})
	}

	return &discordgo.MessageEmbed{
		Title:  fmt.Sprintf("%s Info", strings.ToUpper(infoType)),
		Color:  0x0099ff,
		Fields: fields,
	}
}

func (bot *DiscordBot) handleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {

	help := "*List of Available Commands*\n**Information Query:**\n!info module - Query module information\n!info network - Query network information\n!info signal - Query signal information\n!info <key> - Query specific information (e.g., ModuleName)\n**SMS Operations:**\n!sms send <phone number> <content> - Send an SMS message\n!sms count - Query the total number of SMS messages\n!sms get <DBID> - Query SMS messages with a specific ID\n!sms list <DBID_start> <DBID_end> - Query SMS messages within a range of IDs\n**Other:**\n!help - Display this help information\n!check - Send new SMS detection trigger signal"

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

	case "sms":
		bot.processSMSCmd(s, m, args[1:])

	case "help":
		s.ChannelMessageSend(m.ChannelID, help)
		
	default:
		bot.session.ChannelMessageSend(m.ChannelID, "unknown command, type !help to view help") 
	}
	
	return
}

func (bot *DiscordBot) processInfoCmd(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	
	if len(args) == 0 {
		bot.session.ChannelMessageSend(m.ChannelID, "Please specify the information type: module, network, signal")
		return
	}

	infoType := strings.ToLower(args[0])
	var result map[string]interface{}
	var err error

	switch infoType {

	case "module":
		result, err = bot.nri.FetchModuleInfo()

	case "network":
		result, err = bot.nri.FetchNetworkInfo()

	case "signal":
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

	bot.session.ChannelMessageSendEmbed(m.ChannelID, bot.formatInfoEmbed(infoType, result))
}

func (bot *DiscordBot) processSMSCmd(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {

	if len(args) == 0 {
		bot.session.ChannelMessageSend(m.ChannelID, "Please specify the subcommand type: send, count, get, list")
		return
	}

	cmdType := strings.ToLower(args[0])

	switch cmdType {

	case "send":
		if len(args) < 3 {
			bot.session.ChannelMessageSend(m.ChannelID, "Usage: !sms send <phone number> <content>")
			return
		}
		phone := args[1]
		msg := strings.Join(args[2:], " ")
		err := bot.nri.SendRawSMS(phone, msg)
		if err != nil {
			bot.session.ChannelMessageSend(m.ChannelID, fmt.Sprintf("SMS Failed To Send: %v", err))
		} else {
			bot.session.ChannelMessageSend(m.ChannelID, "SMS Sent Successfully")
		}

	case "count":
		count, err := bot.smsManager.GetDBStats()
		if err != nil {
			bot.session.ChannelMessageSend(m.ChannelID, fmt.Sprintf("SMS Database Failed To Query: %v", err))
		} else {
			bot.session.ChannelMessageSend(m.ChannelID, fmt.Sprintf("SMS Count In Database: %d", count))
		}

	case "get":
		if len(args) < 2 {
			bot.session.ChannelMessageSend(m.ChannelID, "Usage: !sms get <DBID>")
			return
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			bot.session.ChannelMessageSend(m.ChannelID, "DBID Must Be An Integer")
			return
		}
		record, err := bot.smsManager.GetSMSByID(id)
		if err != nil {
			bot.session.ChannelMessageSend(m.ChannelID, fmt.Sprintf("SMS Failed To Query: %v", err))
		} else {
			bot.sendSMSRecord(m, record)
		}

	case "list":
		if len(args) < 3 {
			bot.session.ChannelMessageSend(m.ChannelID, "Usage: !sms list <DBID_start> <DBID_end>")
			return
		}
		startid, err1 := strconv.ParseInt(args[1], 10, 64)
		endid, err2 := strconv.ParseInt(args[2], 10, 64)
		if err1 != nil || err2 != nil {
			bot.session.ChannelMessageSend(m.ChannelID, "DBID Must Be An Integer")
			return
		}
		records, err := bot.smsManager.GetSMSByIDRange(startid, endid)
		if err != nil {
			bot.session.ChannelMessageSend(m.ChannelID, fmt.Sprintf("SMS Failed To Query By Range: %v", err))
		} else {
			bot.sendSMSRecordList(m, records, startid, endid)
		}
		
	default:
		bot.session.ChannelMessageSend(m.ChannelID, "unknown command, type !help to view help") 
	}
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
		commandPrefix: "!",
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
