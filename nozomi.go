package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/exp/slices"

	tgbot "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var HELP_MSG = `
Hi, I'm Nozomi.
I re-send any media that was forwarded from another channnel.

Just add me to your group, make me admin (to allow deleting the forwards), and I'll do my work.
`

func main() {
	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatalln("Unable to read bot token. Make sure you export $TOKEN in the environment.")
	}

	bot, err := tgbot.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	debug := os.Getenv("DEBUG")
	if !slices.Contains([]string{"", "0", "false"}, strings.ToLower(debug)) {
		log.Println("Running in debug mode due to DEBUG environment variable.")
		bot.Debug = true
	}

	bot.Send(tgbot.DeleteWebhookConfig{DropPendingUpdates: false})

	var updates tgbot.UpdatesChannel

	domain := os.Getenv("DOMAIN")
	if domain == "" {
		log.Println("Unable to read domain for incoming requests. Make sure to set $DOMAIN in the environment if you want to use webhooks. Falling back to the polling method.")

		u := tgbot.NewUpdate(0)
		u.Timeout = 60

		channel := bot.GetUpdatesChan(u)

		updates = channel

		setupHealthcheck(bot, false)
	} else {
		log.Printf("Using %s as a domain name", domain)

		wh, _ := tgbot.NewWebhook("https://" + domain + ":443/" + bot.Token)
		wh.AllowedUpdates = []string{}

		_, err = bot.Request(wh)
		if err != nil {
			log.Fatal(err)
		}

		info, err := bot.GetWebhookInfo()
		if err != nil {
			log.Fatal(err)
		}

		if info.LastErrorDate != 0 {
			log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
		}

		port := os.Getenv("PORT")
		if port == "" {
			port = "3000"
		}
		log.Println("Listening on port", port)
		go http.ListenAndServe(":"+port, nil)

		updates = bot.ListenForWebhook("/" + bot.Token)

		setupHealthcheck(bot, true)
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		msg := tgbot.NewMessage(update.Message.Chat.ID, "")
		msg.ParseMode = "html"
		msg.ReplyToMessageID = update.Message.MessageID

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "resend":
				resendMedia(bot, update.Message.ReplyToMessage)
				continue
			case "start":
				msg.Text = HELP_MSG
			case "help":
				msg.Text = HELP_MSG
			case "vibecheck":
				msg.Text = "yeah, I'm good, thanks"
			}
			bot.Send(msg)
		}

		if update.Message.ForwardFromChat == nil {
			continue
		}
		go resendMedia(bot, update.Message)
	}
}

func resendMedia(bot *tgbot.BotAPI, message *tgbot.Message) {
	var err error

	if message.Photo != nil {
		photoSizes := message.Photo
		if len(photoSizes) > 0 {
			photoMsg := tgbot.NewPhoto(
				message.Chat.ID,
				tgbot.FileID(photoSizes[len(photoSizes)-1].FileID),
			)
			_, err = bot.Send(photoMsg)
		}
	} else if message.Video != nil {
		videoMsg := tgbot.NewVideo(
			message.Chat.ID,
			tgbot.FileID(message.Video.FileID),
		)
		_, err = bot.Send(videoMsg)
	} else if message.Animation != nil {
		gifMsg := tgbot.NewAnimation(
			message.Chat.ID,
			tgbot.FileID(message.Animation.FileID),
		)
		_, err = bot.Send(gifMsg)
	} else {
		return // To avoid deleting all messages
	}

	if err == nil {
		bot.Send(tgbot.NewDeleteMessage(message.Chat.ID, message.MessageID))
	}
}

func setupHealthcheck(bot *tgbot.BotAPI, withWebHooks bool) {
	var healthcheck func(http.ResponseWriter, *http.Request)
	if withWebHooks {
		healthcheck = func(w http.ResponseWriter, r *http.Request) {
			wh, err := bot.GetWebhookInfo()
			if err != nil {
				http.Error(w, "Cant get webhook info", http.StatusInternalServerError)
			} else if !wh.IsSet() {
				http.Error(w, "Webhook is not set", http.StatusInternalServerError)
			} else if wh.PendingUpdateCount > 0 {
				http.Error(w, wh.LastErrorMessage, http.StatusServiceUnavailable)
			} else {
				w.Write([]byte("OK\n"))
			}
		}
	} else {
		healthcheck = func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Cant check health from this end, webhooks are disabled\n"))
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthcheck)
	go http.ListenAndServe("localhost:9000", mux)
}
