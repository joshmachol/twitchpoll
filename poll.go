package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/husio/go-irc"
)

var (
	address = flag.String("address", "irc.twitch.tv:6667", "IRC server address")
	pass    = flag.String("pass", "oauth:izdw5tvzp7101xlafca3i073hhmx2em", "User pass (OAuth token)")
	nick    = flag.String("nick", "jmachol", "User nick")
	prof    = flag.String("prof", *nick, "Profile chat to poll")
	verbose = flag.Bool("verbose", false, "Print all messages to stdout")

	streamsURL = "https://api.twitch.tv/kraken/streams/featured?limit=1"
)

type Channel struct {
	Name string
}

type Response struct {
	Featured []struct {
		Stream struct {
			Channel Channel
		}
	}
}

func getFirstFeaturedStream() string {
	resp, err := http.Get(streamsURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var r Response
	err = json.Unmarshal(body, &r)
	if err != nil {
		log.Fatal(err)
	}
	if len(r.Featured) != 1 {
		log.Fatal("No featured streamers.")
	}
	return r.Featured[0].Stream.Channel.Name
}

func arrayInsert(a []string, i int, value string) {
	var temp string
	for i < len(a) {
		temp = a[i]
		a[i] = value
		value = temp
		i++
	}
}

func getMaxN(m map[string]int, n int) map[string]int {
	if len(m) < n {
		n = len(m)
	}
	var top = make([]string, n)
	for k, v := range m {
		for i := 0; i < n; i++ {
			if m[top[i]] < v {
				arrayInsert(top, 0, k)
				break
			}
		}
	}
	var ret = make(map[string]int, n)
	for _, v := range top {
		ret[v] = m[v]
	}
	return ret
}

func main() {
	flag.Parse()
	conn, err := irc.Connect(*address)
	if err != nil {
		log.Fatal(err)
	}

	conn.Send("PASS %s", *pass)
	conn.Send("NICK %s", *nick)
	time.Sleep(time.Millisecond * 20)

	// join the top featured stream
	stream := getFirstFeaturedStream()
	conn.Send("JOIN #%s", stream)
	log.Println("Joined #", stream)

	// read data from stdin and send it through the wire
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			line = strings.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			conn.Send(line)
		}
	}()

	fmt.Print(`

For IRC protocol description, read rfc1459: https://tools.ietf.org/html/rfc1459
Some basics:

	JOIN    <channel>{,<channel>} [<key>{,<key>}]
	PRIVMSG <receiver>{,<receiver>} <text to be sent>
	PART    <channel>{,<channel>}

`)
	polltotal := 0
	var poll = make(map[string]int)

	// periodically print the poll info
	timer := time.Tick(time.Second * 5)
	go func() {
		for {
			<-timer
			fmt.Println("====")
			fmt.Println("Total:", polltotal)
			for k, v := range getMaxN(poll, 5) {
				fmt.Println(k, v)
			}
		}
	}()

	// handle incomming messages
	for {
		message, err := conn.ReadMessage()
		if err != nil {
			log.Fatal(err)
			return
		}

		// user chat is sent as private messages
		if message.Command() == "PRIVMSG" {
			fields := strings.Fields(message.Trailing())
			if len(fields) == 0 {
				continue
			}
			msg := fields[0]
			if _, ok := poll[msg]; ok {
				poll[msg] = poll[msg] + 1
			} else {
				poll[msg] = 1
			}
			polltotal = polltotal + 1
		}

		if *verbose {
			log.Println("Command:", message.Command())
			log.Println("Params:", message.Params())
			log.Println("Prefix:", message.Prefix())
			log.Println("String:", message.String())
			log.Println("Trailing:", message.Trailing())
		}
	}
}
