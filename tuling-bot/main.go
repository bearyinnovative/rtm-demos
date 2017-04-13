package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	bc "github.com/bearyinnovative/bearychat-go"
	"github.com/bitly/go-simplejson"
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

	context, err := bc.NewRTMContext(rtmToken)
	if err != nil {
		log.Fatal(err)
		return
	}

	err, messageC, errC := context.Run()
	if err != nil {
		log.Fatal(err)
		return
	}

	for {
		select {
		case err := <-errC:
			log.Printf("rtm loop error: %+v", err)
			if err := context.Loop.Stop(); err != nil {
				log.Fatal(err)
			}
			return
		case message := <-messageC:
			// ignore other types
			if !message.IsChatMessage() {
				continue
			}

			// ignore self message
			if message.IsFromUID(context.UID()) {
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
			mentioned, text := message.ParseMentionUID(context.UID())
			if !mentioned {
				continue
			}
			reply, err := replyContent(uid, text)
			if reply == "" || uid == "" || err != nil {
				// ignore this
				continue
			}

			if err := context.Loop.Send(message.Refer(reply)); err != nil {
				log.Fatal(err)
			}
		}
	}
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

func checkErr(err error) bool {
	if err != nil {
		log.Fatalln(err)
		return false
		// panic(err)
	}

	return true
}
