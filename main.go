package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
)

var (
	discordToken      string
	discordChannel    string
	savedGameLocation string
	once              bool
)

func init() {
	flag.StringVar(&discordToken, "discord-token", "", "discord api token")
	flag.StringVar(&discordChannel, "discord-channel", "", "discord channel")
	flag.StringVar(&savedGameLocation, "saved-game-location", "", "filepath to the saved games")
	flag.BoolVar(&once, "once", false, "only run the backup once and exit program")
	flag.Parse()
}

func main() {
	dg, err := discordgo.New(fmt.Sprintf("Bot %s", discordToken))
	if err != nil {
		log.Panicf("Unable to create discord client")
	}

	err = dg.Open()
	if err != nil {
		log.Panicf("unable to create discord session")
	}

	backupNow(dg)
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
