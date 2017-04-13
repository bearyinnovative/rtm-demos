package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bc "github.com/bearyinnovative/bearychat-go"
)

var rtmToken string
var cmdPath string

func init() {
	flag.StringVar(&rtmToken, "rtmToken", "", "BearyChat RTM token")
	flag.StringVar(&cmdPath, "cmdPath", "./commands", "path of your scripts")
}

func main() {
	flag.Parse()

	if err := os.Chdir(cmdPath); err != nil {
		log.Fatal(err)
		os.Exit(1)
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
			if !message.IsChatMessage() {
				continue
			}

			// from self
			if message.IsFromUID(context.UID()) {
				continue
			}

			log.Printf(
				"received: %s from %s",
				message["text"],
				message["uid"],
			)

			text := excuteCommandsIfCould(context.UID(), message)
			if text == "" {
				// ignore this
				continue
			}

			if err := context.Loop.Send(message.Refer(text)); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func excuteCommandsIfCould(uid string, message bc.RTMMessage) string {
	mentioned, text := message.ParseMentionUID(uid)
	if !mentioned {
		return ""
	}

	cmds := strings.Split(text, " ")
	args := strings.Join(cmds[1:], " ")

	if !checkCommandExist(cmds[0]) {
		return fmt.Sprintf("no command: '%s'", cmds[0])
	}

	var cmd *exec.Cmd
	log.Println("exec:", cmds[0], args)
	if args == "" {
		cmd = exec.Command("./" + cmds[0])
	} else {
		cmd = exec.Command("./"+cmds[0], args)
	}

	env := os.Environ()
	env = append(env, fmt.Sprintf("vchannel=%s", message["vchannel_id"].(string)))
	env = append(env, fmt.Sprintf("rtmToken=%s", rtmToken))
	cmd.Env = env
	out, err := cmd.Output()

	if err != nil {
		return err.Error()
	}

	result := string(out)
	log.Println("output:\n", result)

	return result
}

func checkCommandExist(cmd string) bool {
	absPath, err := filepath.Abs(".")
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	d, err := os.Open(absPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer d.Close()
	fi, err := d.Readdir(-1)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for _, fi := range fi {
		if fi.Mode().IsDir() {
			continue
		}

		if fi.Name() == cmd {
			return true
		}
	}

	return false
}
