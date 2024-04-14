package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	Snek      Snek
	Broadcast chan string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var game = new(Game)

type Res struct {
	Direction string `json:"direction"`
}

func main() {

	http.HandleFunc("/connect", handleNewPlayer)

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		tmpl := Render("start", game)
		w.Write(tmpl.Bytes())

	})

	http.HandleFunc("/newgame", func(w http.ResponseWriter, r *http.Request) {
		if game.Time < 1 {
			go gameLoop()
			go timeLoop(game)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		tmpl := Render("index", "Welcome to Snek!")
		w.Write(tmpl.Bytes())
	})

	http.ListenAndServe("0.0.0.0:10000", nil)

}

func timeLoop(game *Game) {
	for {
		if len(game.Players) == 0 {
			game.Time = 0
			break
		}
		time.Sleep(1 * time.Second)
		game.Time++
		tmpl := Render("time", game.Time)
		broadcastTmpl(tmpl.Bytes())
	}
}

func gameLoop() {
	for {
		time.Sleep(300 * time.Millisecond)
		eatApple := false

		templateToRender := []byte{}
		tailTemplate := []byte{}
		snekTemplate := []byte{}

		for _, client := range game.Players {
			tail := client.Snek.Body[len(client.Snek.Body)-1]
			tailToRemove := Render("empty", tail)
			tailTemplate = append(tailTemplate, tailToRemove.Bytes()...)

			head := Render("apple", client.Snek.Body[0])
			snekTemplate = append(snekTemplate, head.Bytes()...)
			client.Snek.move(*game, eatApple)
			if game.checkCollision(client.Snek) {
				msg := Render("header", "You died!")
				broadcastTmpl(msg.Bytes())

			}
		}

		game.generateBoard()
		if eatApple {
			game.Score += 100
			score := Render("score", game.Score)
			broadcastTmpl(score.Bytes())

			game.generateApple()
			newApple := Render("apple", game.Apple)
			templateToRender = append(templateToRender, newApple.Bytes()...)
		} else {
			templateToRender = append(templateToRender, tailTemplate...)
		}

		templateToRender = append(templateToRender, snekTemplate...)
		broadcastTmpl(templateToRender)
	}
}

func Render(blockName string, data any) bytes.Buffer {
	tmpl := template.Must(template.ParseGlob("views/*.html"))
	buffer := bytes.Buffer{}
	tmpl.ExecuteTemplate(&buffer, blockName, data)
	return buffer
}

func CheckDirection(item, direction string) bool {
	if direction == "vertical" {
		return item == "up" || item == "down"
	}
	if direction == "horizontal" {
		return item == "left" || item == "right"
	}
	return false
}

func broadcastTmpl(tmpl []byte) {
	for client := range game.Players {
		err := client.WriteMessage(websocket.TextMessage, tmpl)
		if err != nil {
			log.Println(err)
			client.Close()
			delete(game.Players, client)
		}
	}
}

func handleNewPlayer(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading: %s", err)
	}
	if len(game.Players) == 0 {
		startGame()
	}
	player := Client{Snek: game.newSnek(), Broadcast: make(chan string)}
	game.Players[conn] = &player

	defer conn.Close()
	conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("connection lost with client: %s", conn.RemoteAddr())
		conn.Close()
		delete(game.Players, conn)
		return fmt.Errorf("connection close")
	})

	go func() {
		for {
			player.Snek.Direction = <-player.Broadcast

		}
	}()

	// read messages
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			deletedSnek := Render("deleteSnek", player.Snek)
			delete(game.Players, conn)
			broadcastTmpl(deletedSnek.Bytes())
			log.Println("Error reading message")
			return
		}
		response := Res{}
		err = json.Unmarshal([]byte(msg), &response)
		if err != nil {
			log.Println("Error parsing json")
			return

		}

		log.Printf("Recieve: %s From: %s", response.Direction, conn.RemoteAddr())
		if CheckDirection(player.Snek.Direction, "vertical") && CheckDirection(response.Direction, "horizontal") {
			player.Broadcast <- response.Direction
		}
		if CheckDirection(player.Snek.Direction, "horizontal") && CheckDirection(response.Direction, "vertical") {
			player.Broadcast <- response.Direction
		}
	}

}

func startGame() {
	game = new(Game)
	game.Players = make(map[*websocket.Conn]*Client)
	game.generateApple()
	game.generateBoard()
	appleTemplate := Render("apple", game.Apple)
	broadcastTmpl(appleTemplate.Bytes())

}
