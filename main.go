package main

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
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
	// Create zip file for backup
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	backupFileName := fmt.Sprintf("valheim-backup-%s.zip", time.Now().Format("Mon Jan _2 15:04:05 MST 2006"))

	file, err := os.OpenFile(backupFileName, flags, 0644)
	if err != nil {
		log.Fatalf("Failed to open zip for writing: %s", err)
	}
	defer file.Close()

	// setup new zip writer
	zipw := zip.NewWriter(file)
	appendFilesToZip(zipw)
	zipw.Close()

	finalBackupFile, err := os.Open(backupFileName)
	if err != nil {
		log.Fatalf("Failed to open final backup zip: %s", err)
	}
	defer file.Close()

	_, err = dg.ChannelMessageSendComplex(discordChannel, &discordgo.MessageSend{
		Content: "Backing up all Valheim worlds",
		Files: []*discordgo.File{
			{
				Name:        file.Name(),
				ContentType: "application/zip",
				Reader:      finalBackupFile,
			},
		},
	})
	if err != nil {
		_, _ = dg.ChannelMessageSend(discordChannel, fmt.Sprintf("Error backing saved files: %s", err))
		return
	}
}

func appendFilesToZip(zipw *zip.Writer) {
	_ = filepath.Walk(savedGameLocation, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			wr, err := zipw.Create(info.Name())
			if err != nil {
				return err
			}

			if _, err := io.Copy(wr, file); err != nil {
				return fmt.Errorf("Failed to write %s to zip: %s", info.Name(), err)
			}
		}

		return nil
	})
}
