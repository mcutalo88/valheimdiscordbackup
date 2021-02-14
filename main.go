package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

const discordTokenEnv = "DISCORD_TOKEN"

var (
	discordChannel    string
	savedGameLocation string
	backupInterval    int
	once              bool
)

func init() {
	flag.StringVar(&discordChannel, "discord-channel", "", "discord channel")
	flag.StringVar(&savedGameLocation, "saved-game-location", "", "filepath to the saved games")
	flag.IntVar(&backupInterval, "backup-interval", 60, "backup interval in minutes")
	flag.BoolVar(&once, "once", false, "only run the backup once and exit program")

	flag.Parse()
}

func main() {
	token := os.Getenv(discordTokenEnv)
	if len(token) == 0 {
		log.Panicf("DISCORD_TOKEN not set")
	}

	dg, err := discordgo.New(fmt.Sprintf("Bot %s", token))
	if err != nil {
		log.Panicf("Unable to create discord client")
	}

	err = dg.Open()
	if err != nil {
		log.Panicf("unable to create discord session")
	}

	ctx, ctxCancelFunc := context.WithCancel(context.Background())
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigc
		log.Print("Caught shutdown signal, shutting down")
		ctxCancelFunc()
	}()

	backupIntervalDuration := time.Minute * time.Duration(backupInterval)
	ticker := time.NewTicker(backupIntervalDuration)
	for {
		select {
		case <-ticker.C:
			backupNow(dg)
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func backupNow(dg *discordgo.Session) {
	allWorldFiles, err := prepareFilesForBackup()
	if err != nil {
		_, _ = dg.ChannelMessageSend(discordChannel, fmt.Sprintf("Error backing saved files: %s", err))
		return
	}

	sendFiles(dg, allWorldFiles)
}

func prepareFilesForBackup() ([]*discordgo.File, error) {
	allWorldFiles := []*discordgo.File{}
	err := filepath.Walk(savedGameLocation, func(path string, info os.FileInfo, err error) error {
		if path == savedGameLocation {
			return nil
		}

		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		allWorldFiles = append(allWorldFiles, &discordgo.File{
			Name:        file.Name(),
			ContentType: "text",
			Reader:      file,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return allWorldFiles, nil
}

func sendFiles(dg *discordgo.Session, allWorldFiles []*discordgo.File) {
	msg, err := dg.ChannelMessageSendComplex(discordChannel, &discordgo.MessageSend{
		Content: "Backing up all Valheim worlds",
		Files:   allWorldFiles,
	})
	log.Printf("%v", msg)
	log.Printf("%v", err)
}
