package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	bc "github.com/bcho/bearychat.go"
	"github.com/bitly/go-simplejson"
)

const (
	RTM_API_BASE = "https://rtm.bearychat.com"
)

const (
	CODE_TEXT = 100000
	CODE_LINK = 200000
	CODE_NEWS = 302000
	CODE_MENU = 308000
)

var rtmToken string
var tulingToken string

func init() {
	flag.StringVar(&rtmToken, "rtmToken", "", "BearyChat RTM token")
	flag.StringVar(&tulingToken, "tulingToken", "", "http://www.tuling123.com/openapi/api token")
}

func main() {
	flag.Parse()

	if rtmToken == "" {
		log.Fatal("need rtm token")
		return
	}

	if tulingToken == "" {
		log.Fatal("need tuling token")
		return
	}

	rtmClient, err := bc.NewRTMClient(
		rtmToken,
		bc.WithRTMAPIBase(RTM_API_BASE),
	)
	if err != nil {
		log.Fatal(err)
	}

	user, wsHost, err := rtmClient.Start()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("rtm connected as %s\n", user.Name)

	rtmLoop, err := bc.NewRTMLoop(wsHost)
	if err != nil {
		log.Fatal(err)
	}

	if err := rtmLoop.Start(); err != nil {
		log.Fatal(err)
	}
	defer rtmLoop.Stop()

	go rtmLoop.Keepalive(time.NewTicker(10 * time.Second))

	errC := rtmLoop.ErrC()
	messageC, err := rtmLoop.ReadC()
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case err := <-errC:
			log.Printf("rtm loop error: %+v", err)
			if err := rtmLoop.Stop(); err != nil {
				log.Fatal(err)
			}
			return
		case message := <-messageC:
			// ignore other types
			if !message.IsChatMessage() {
				continue
			}

			// ignore self message
			if message.IsFromMe(*user) {
				continue
			}

			log.Printf(
				"[%s] '%s' from '%s'",
				message.Type(),
				message["text"],
				message["uid"],
			)

			uid, ok := message["uid"].(string)
			if !ok {
				continue
			}
			text := parseTextContent(user, message)
			if text == "" {
				continue
			}
			reply, err := replyContent(uid, text)
			if reply == "" || uid == "" || err != nil {
				// ignore this
				continue
			}

			if err := rtmLoop.Send(message.Refer(reply)); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func parseTextContent(user *bc.User, message bc.RTMMessage) string {
	if !message.IsChatMessage() {
		return "NOT SUPPORT"
	}

	mt := message.Type()
	text, ok := message["text"].(string)
	if !ok {
		return ""
	}
	if mt == bc.RTMMessageTypeChannelMessage {
		var isAtMe bool
		isAtMe, text = parseAtUserAtBeginning(user, text)
		if !isAtMe {
			return ""
		}
	}

	return text
}

func replyContent(uid, content string) (reply string, err error) {
	// Request (POST http://www.tuling123.com/openapi/api)

	dic := map[string]string{
		"key":    tulingToken,
		"info":   content,
		"userid": uid,
	}
	jsonValue, err := json.Marshal(dic)
	if !checkErr(err) {
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "http://www.tuling123.com/openapi/api", bytes.NewBuffer(jsonValue))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json; charset=utf-8")

	resp, err := client.Do(req)
	if !checkErr(err) {
		return
	}

	defer resp.Body.Close()

	j, err := simplejson.NewFromReader(resp.Body)
	if !checkErr(err) {
		return
	}

	code, err := j.GetPath("code").Int()
	if !checkErr(err) {
		return
	}

	reply, err = j.GetPath("text").String()
	if !checkErr(err) {
		return
	}

	switch code {
	case CODE_LINK:
		var url string
		url, err = j.GetPath("url").String()
		if !checkErr(err) {
			return
		}

		return fmt.Sprintf("[%s](%s)", reply, url), nil
	default:
		return
	}
}

func parseAtUserAtBeginning(user *bc.User, text string) (bool, string) {
	r, _ := regexp.Compile("@<=(.*)=>")
	loc := r.FindStringIndex(text)

	if len(loc) != 2 {
		return false, text
	}

	if text[loc[0]+3:loc[1]-2] == user.Id {
		return true, text[loc[1]+1:]
	}

	return false, text
}

func checkErr(err error) bool {
	if err != nil {
		log.Fatalln(err)
		return false
		// panic(err)
	}

	return true
}
