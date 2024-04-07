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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Res struct {
	Direction string `json:"direction"`
}

func main() {
	http.HandleFunc("/game", func(w http.ResponseWriter, r *http.Request) {

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Error upgrading: %s", err)
		}
		defer conn.Close()
		conn.SetCloseHandler(func(code int, text string) error {
			log.Printf("connection lost with client: %s", conn.RemoteAddr())
			return fmt.Errorf("connection close")
		})
		game := new(Game)
		game.newSnek()
		game.generateBoard()
		game.generateApple()
		appleTemplate := Render("apple", game.Apple)
		if err = conn.WriteMessage(websocket.TextMessage, appleTemplate.Bytes()); err != nil {
			return
		}

		// timer
		go timeLoop(game, conn)

		// game loop
		go func() {
			for {
				time.Sleep(300 * time.Millisecond)
				eatApple := game.isEatingApple(game.Snek, game.Apple)
				templateToRender := []byte{}
				tail := game.Snek.Body[len(game.Snek.Body)-1]

				game.Snek.move(*game, eatApple)
				if game.checkCollision(game.Snek) {
					msg := Render("dead", "You died!")
					if err = conn.WriteMessage(websocket.TextMessage, msg.Bytes()); err != nil {
						return
					}
					conn.Close()
					return
				}
				game.generateBoard()
				if eatApple {
					game.Score += 100
					score := Render("score", game.Score)
					if err = conn.WriteMessage(websocket.TextMessage, score.Bytes()); err != nil {
						return
					}
					game.generateApple()
					newApple := Render("apple", game.Apple)
					templateToRender = append(templateToRender, newApple.Bytes()...)
				} else {
					tailTemplate := Render("empty", tail)
					templateToRender = append(templateToRender, tailTemplate.Bytes()...)
				}
				snekTemplate := Render("apple", game.Snek.Body[0])
				templateToRender = append(templateToRender, snekTemplate.Bytes()...)
				if err = conn.WriteMessage(websocket.TextMessage, templateToRender); err != nil {
					return
				}
			}
		}()

		// read messages
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
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
				game.Snek.Direction = response.Direction
			}
			if CheckDirection(game.Snek.Direction, "horizontal") && CheckDirection(response.Direction, "vertical") {
				game.Snek.Direction = response.Direction
			}
		}

	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		game := new(Game)
		tmpl := Render("index", game)
		w.Write(tmpl.Bytes())
	})

	http.ListenAndServe(":8080", nil)
}

func timeLoop(game *Game, conn *websocket.Conn) {
	for {
		time.Sleep(1 * time.Second)
		game.Time++
		msg := Render("time", game.Time)
		if err := conn.WriteMessage(websocket.TextMessage, msg.Bytes()); err != nil {
			return
		}
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
