package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Player struct {
	Id        int
	Snek      Snek
	Broadcast chan string
	Score     int
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

	http.HandleFunc("/lobby", func(w http.ResponseWriter, r *http.Request) {
		tmpl := Render("lobby", game)
		w.Write(tmpl.Bytes())

	})

	http.HandleFunc("/newgame", func(w http.ResponseWriter, r *http.Request) {
		// TODO: remove go routines after the game finishes
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
		time.Sleep(600 * time.Millisecond)
		isAppleEated := false

		templateToRender := []byte{}
		tailTemplate := []byte{}
		snekTemplate := []byte{}

		for conn, player := range game.Players {
			isEatingApple := game.isEatingApple(player.Snek, game.Apple)
			tail := player.Snek.Body[len(player.Snek.Body)-1]
			player.Snek.move(*game, isEatingApple)
			if isEatingApple {
				isAppleEated = true
				player.Score += 100
				score := Render("score", player.Score)
				conn.WriteMessage(websocket.TextMessage, score.Bytes())
			} else {
				tailToRemove := Render("empty", tail)
				tailTemplate = append(tailTemplate, tailToRemove.Bytes()...)
			}
			head := Render("unit", player.Snek.Body[0])
			snekTemplate = append(snekTemplate, head.Bytes()...)
		}

		// check if a player hit something after all the snek moved
		// TODO: fix collision, is still wonky
		for conn, player := range game.Players {
			if game.checkCollision(player.Snek) {
				msg := Render("header", "You died!")
				err := conn.WriteMessage(websocket.TextMessage, msg.Bytes())
				if err != nil {
					log.Println(err)
					conn.Close()
					delete(game.Players, conn)
				}

			}
		}

		game.generateBoard()

		if isAppleEated {
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
	if len(game.Players) > 3 {
		msg := Render("header", "The lobby is Full!")
		err := conn.WriteMessage(websocket.TextMessage, msg.Bytes())
		if err != nil {
			log.Println(err)
		}
		conn.Close()
	} else {
		newId := getUnusedId()
		player := Player{Snek: game.newSnek(fmt.Sprintf("player%v", newId)), Broadcast: make(chan string), Score: 0, Id: newId}
		game.Players[conn] = &player

		sneksToRender := []byte{}
		for _, player := range game.Players {
			snekTemp := Render("snek", player.Snek.Body)
			sneksToRender = append(sneksToRender, snekTemp.Bytes()...)
		}
		appleTemplate := Render("apple", game.Apple)
		broadcastTmpl(append(appleTemplate.Bytes(), sneksToRender...))
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
			if CheckDirection(player.Snek.Direction, "vertical") && CheckDirection(response.Direction, "horizontal") {
				player.Broadcast <- response.Direction
			}
			if CheckDirection(player.Snek.Direction, "horizontal") && CheckDirection(response.Direction, "vertical") {
				player.Broadcast <- response.Direction
			}
		}
	}

}

func startGame() {
	game = new(Game)
	game.Players = make(map[*websocket.Conn]*Player)
	game.generateApple()
	game.generateBoard()

}

func getUnusedId() int {
	id := rand.IntN(4)
	for _, player := range game.Players {
		if id == player.Id {
			id = getUnusedId()
		}
	}
	return id

}
