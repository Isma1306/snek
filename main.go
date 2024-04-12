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
	Conn      *websocket.Conn
	Direction string
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
var game = new(Game)
var clients = make(map[*websocket.Conn]*Client)
var broadcast = make(chan string)

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
		game = new(Game)
		game.newSnek()
		game.generateApple()
		game.generateBoard()
		appleTemplate := Render("apple", game.Apple)
		broadcastTmpl(appleTemplate.Bytes())
		go timeLoop(game)
		go gameLoop()
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		tmpl := Render("index", "Welcome to Snek!")
		w.Write(tmpl.Bytes())
	})

	http.ListenAndServe("0.0.0.0:10000", nil)
}

func timeLoop(game *Game) {
	for {
		time.Sleep(1 * time.Second)
		game.Time++
		tmpl := Render("time", game.Time)
		broadcastTmpl(tmpl.Bytes())
	}
}

func gameLoop() {
	for {
		time.Sleep(300 * time.Millisecond)
		eatApple := game.isEatingApple(game.Snek, game.Apple)
		templateToRender := []byte{}
		tail := game.Snek.Body[len(game.Snek.Body)-1]

		game.Snek.move(*game, eatApple)
		if game.checkCollision(game.Snek) {
			msg := Render("header", "You died!")
			broadcastTmpl(msg.Bytes())

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
			tailTemplate := Render("empty", tail)
			templateToRender = append(templateToRender, tailTemplate.Bytes()...)
		}
		snekTemplate := Render("apple", game.Snek.Body[0])
		templateToRender = append(templateToRender, snekTemplate.Bytes()...)
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
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, tmpl)
		if err != nil {
			log.Println(err)
			client.Close()
			delete(clients, client)
		}
	}
}

func handleNewPlayer(w http.ResponseWriter, r *http.Request) {

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading: %s", err)
	}
	client := Client{Conn: conn, Direction: "right"}

	clients[conn] = &client
	defer conn.Close()
	conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("connection lost with client: %s", conn.RemoteAddr())
		return fmt.Errorf("connection close")
	})

	go func() {
		for {
			game.Snek.Direction = <-broadcast

		}
	}()

	// game loop

	// read messages
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			delete(clients, conn)
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
		if CheckDirection(game.Snek.Direction, "vertical") && CheckDirection(response.Direction, "horizontal") {
			broadcast <- response.Direction
		}
		if CheckDirection(game.Snek.Direction, "horizontal") && CheckDirection(response.Direction, "vertical") {
			broadcast <- response.Direction
		}
	}

}
