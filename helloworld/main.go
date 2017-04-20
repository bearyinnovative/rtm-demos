package main

import (
	"flag"
	"log"

	bc "github.com/bearyinnovative/bearychat-go"
)

var rtmToken string

func init() {
	flag.StringVar(&rtmToken, "rtmToken", "", "BearyChat RTM token")
}

func main() {
	flag.Parse()

	if rtmToken == "" {
		log.Fatal("need rtm token")
		return
	}

	context, err := bc.NewRTMContext(rtmToken)
	if err != nil {
		log.Fatal(err)
		return
	}

	err, messageC, _ := context.Run()
	if err != nil {
		log.Fatal(err)
		return
	}

	for {
		select {
		case message := <-messageC:
			// ignore other types
			if !message.IsChatMessage() {
				continue
			}

			// ignore self message
			if message.IsFromUID(context.UID()) {
				continue
			}

			if err := context.Loop.Send(message.Refer(message["text"].(string))); err != nil {
				log.Fatal(err)
			}
		}
	}
}
