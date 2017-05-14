package main

import (
	"fmt"
	"log"
	"time"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"regexp"
	"strconv"

	"github.com/lib/pq"
	"database/sql"
	"github.com/jmoiron/sqlx"
)

type Stream struct {
	Id int
	Views int
	User int
	Name string
	Logo sql.NullString
	Status string
	Checked bool
}

type StreamCenter struct {
	sync.RWMutex
	s map[int]*Stream
}

func (s Stream) FakeFetch() (string, error) {
	fmt.Println("fake fetching")

	resp, err := http.Get("https://test-server.address/?query=" + s.Name)
	if err != nil {
		return "", fmt.Errorf("%s: api query error", s.Name)
	}
	defer resp.Body.Close()

	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%s: response body parse error", s.Name)
	}
	return string(responseData), nil
}

func checUrls(streamCenter *StreamCenter) {
	streamCenter.RLock()
	defer streamCenter.RUnlock()

	for k := range streamCenter.s {
		if _, err := streamCenter.s[k].FakeFetch(); err != nil {
			streamCenter.Lock()
				streamCenter.s[k].Status = "offline"
			streamCenter.Unlock()
			continue
		}

		// here we can do smth with successfully fetched streams
	}
}

func getStreams(db *sqlx.DB, streamCenter *StreamCenter) {
	rows, err := db.Queryx("SELECT * FROM stream")
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer rows.Close()

	for rows.Next() {
		stream := Stream{}
		err := rows.StructScan(&stream)
		if err != nil {
			log.Fatalln(err.Error())
		}
		streamCenter.Lock()
			streamCenter.s[stream.Id] = &stream
		streamCenter.Unlock()
	}
}

func waitForNotification(l *pq.Listener, streamCenter *StreamCenter) {
	select {
		case s := <-l.Notify:
			var changedStream Stream
			var pattern string = "^delete_(.+)"

			re := regexp.MustCompile(pattern)
			if matched, _ := regexp.MatchString(pattern, s.Extra); matched {
				found := re.FindAllStringSubmatch(s.Extra, -1)
				deletedElement := found[0][len(found[0])-1]
				if deletedInt, err := strconv.Atoi(deletedElement); err != nil {
					delete(streamCenter.s, deletedInt)
				}
				break
			}

			bytes := []byte(s.Extra)
			err := json.Unmarshal(bytes, &changedStream)
			if err != nil {
				panic(err)
			}
			streamCenter.Lock()
				streamCenter.s[changedStream.Id] = &changedStream
			streamCenter.Unlock()

		case <-time.After(20 * time.Second):
			go l.Ping()
	}
}

func main() {
	var conninfo string = "user=user password=password dbname=db sslmode=disable"
	var streams = make(map[int]*Stream)
	var streamCenter = StreamCenter{s: streams}

	db, err := sqlx.Open("postgres", conninfo)
	if err != nil {
		panic(err)
	}

	go getStreams(db, &streamCenter)

	listener := pq.NewListener(conninfo, 10 * time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {})
	err = listener.Listen("streamwatcher")
	if err != nil {
		panic(err)
	}

	for {
		fmt.Println("begin for loop")

		checUrls(&streamCenter)
		waitForNotification(listener, &streamCenter)
	}
}
