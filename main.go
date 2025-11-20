package main

import (
	"os"
	"log"
	"syscall"
	"os/signal"

	"nrmodule/config"
	"nrmodule/internal"
	"nrmodule/atserial"
	"nrmodule/smsmanager"
)

func main() {

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	port := atserial.NRInterfacePort{
		LocalPort:     cfg.Serial.Port,
		LocalBaudRate: cfg.Serial.BaudRate,
	}
	nri := atserial.NewNRInterface(port, cfg.Serial.IsLocal)
	defer nri.Close()

	smsManager, err := smsmanager.NewManager(nri, cfg.SMS.DBPath, cfg.SMS.CheckInterval)
	if err != nil {
		log.Fatalf("Failed to create SMS manager: %v", err)
	}
	defer smsManager.Close()

	if err := smsManager.Start(); err != nil {
		log.Fatalf("Failed to start SMS manager: %v", err)
	}

	bot, err := internal.NewDiscordBot(cfg.Discord.BotToken, cfg.Discord.ChannelID, nri, smsManager)
	if err != nil {
		log.Fatalf("Failed to create Discord bot: %v", err)
	}

	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start Discord bot: %v", err)
	}
	defer bot.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}
