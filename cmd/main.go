package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:    1024,
	WriteBufferSize:   1024,
	EnableCompression: true,
}

type Board [10][10]Square

type Res struct {
	Direction string `json:"direction"`
}

type Game struct {
	Time  int
	Board Board
	Apple Unit
	Level int
	Score int
}

type Square struct {
	IsOcupied bool
	Fill      string
}

type Unit struct {
	Position [2]int
}

type Snek struct {
	Body []Unit
}

func (g *Game) isEatingApple(snek Snek, apple Unit) bool {
	return apple.Position[0] == snek.Body[0].Position[0] && apple.Position[1] == snek.Body[0].Position[1]
}

func (s *Snek) move(direction string, game Game, eatApple bool) error {
	var dx, dy int
	lastPosition := len(s.Body) - 1
	endOfBoard := len(game.Board) - 1
	switch direction {
	case "up":
		dx, dy = 0, 1
	case "down":
		dx, dy = 0, -1
	case "right":
		dx, dy = 1, 0
	case "left":
		dx, dy = -1, 0
	default:
		return errors.New("snek moving in an impossible direction")
	}
	for i := lastPosition; i >= 0; i-- {
		if i == 0 {
			// Move the head
			s.Body[i].Position[0] += dx
			s.Body[i].Position[1] += dy
			if s.Body[i].Position[0] < 0 {
				s.Body[i].Position[0] = endOfBoard
			}
			if s.Body[i].Position[0] > endOfBoard {

				s.Body[i].Position[0] = 0
			}
			if s.Body[i].Position[1] < 0 {
				s.Body[i].Position[1] = endOfBoard
			}
			if s.Body[i].Position[1] > endOfBoard {
				s.Body[i].Position[1] = 0
			}
		} else if eatApple && i == lastPosition {
			s.Body = append(s.Body, s.Body[i])
			s.Body[i].Position = s.Body[i-1].Position
		} else {
			// Move the body
			s.Body[i].Position = s.Body[i-1].Position
		}
	}

	return nil
}

func (g *Game) generateBoard(snek Snek) {
	g.Board = *new(Board)
	for _, unit := range snek.Body {
		g.Board[unit.Position[0]][unit.Position[1]].Fill = "snek"
		g.Board[unit.Position[0]][unit.Position[1]].IsOcupied = true
	}
	g.Board[g.Apple.Position[0]][g.Apple.Position[1]].Fill = "apple"
	g.Board[g.Apple.Position[0]][g.Apple.Position[1]].IsOcupied = true
}

func (g *Game) generateApple() {
	newPostion := [2]int{rand.IntN(len(g.Board)), rand.IntN(len(g.Board))}
	if g.Board[newPostion[0]][newPostion[1]].IsOcupied {
		g.generateApple()
	} else {
		g.Apple = newUnit(newPostion)
	}
}
func (g *Game) checkCollision(snek Snek) bool {
	for i := 1; i < len(snek.Body); i++ {
		if snek.Body[i].Position[0] == snek.Body[0].Position[0] && snek.Body[i].Position[1] == snek.Body[0].Position[1] {
			return true
		}
	}
	return false
}

func newUnit(position [2]int) Unit {
	return Unit{Position: position}
}

func newSnek() Snek {
	newSnek := new(Snek)
	newSnek.Body = []Unit{newUnit([2]int{2, 5}), newUnit([2]int{1, 5}), newUnit([2]int{0, 5})}
	return *newSnek
}

func render(blockName string, data any) bytes.Buffer {
	tmpl := template.Must(template.ParseGlob("views/*.html"))
	buffer := bytes.Buffer{}
	tmpl.ExecuteTemplate(&buffer, blockName, data)
	return buffer
}

func checkDirection(item, direction string) bool {
	if direction == "vertical" {
		return item == "up" || item == "down"
	}
	if direction == "horizontal" {
		return item == "left" || item == "right"
	}
	return false
}

func main() {

	http.HandleFunc("/game", func(w http.ResponseWriter, r *http.Request) {

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Error upgrading: %s", err)
		}
		conn.EnableWriteCompression(true)
		conn.SetCompressionLevel(9)
		defer conn.Close()
		conn.SetCloseHandler(func(code int, text string) error {
			log.Printf("connection lost with client: %s", conn.RemoteAddr())
			return fmt.Errorf("connection close")
		})
		game := new(Game)
		snek := newSnek()
		game.generateBoard(snek)
		game.generateApple()
		appleTemplate := render("apple", game.Apple)
		if err = conn.WriteMessage(websocket.TextMessage, appleTemplate.Bytes()); err != nil {
			return
		}
		direction := "right"

		// timer
		go func() {
			for {
				time.Sleep(1 * time.Second)
				game.Time++
				msg := render("time", game.Time)
				if err = conn.WriteMessage(websocket.TextMessage, msg.Bytes()); err != nil {
					return
				}
			}
		}()

		// game loop
		go func() {
			for {
				time.Sleep(300 * time.Millisecond)
				eatApple := game.isEatingApple(snek, game.Apple)
				templateToRender := []byte{}
				tail := snek.Body[len(snek.Body)-1]

				snek.move(direction, *game, eatApple)
				if game.checkCollision(snek) {
					msg := render("dead", "You died!")
					if err = conn.WriteMessage(websocket.TextMessage, msg.Bytes()); err != nil {
						return
					}
					conn.Close()
					return
				}
				game.generateBoard(snek)
				if eatApple {
					oldApple := render("empty", game.Apple)
					game.Score += 100
					score := render("score", game.Score)
					if err = conn.WriteMessage(websocket.TextMessage, score.Bytes()); err != nil {
						return
					}
					game.generateApple()
					newApple := render("apple", game.Apple)
					templateToRender = append(append(templateToRender, oldApple.Bytes()...), newApple.Bytes()...)
				}
				tailTemplate := render("empty", tail)
				snekTemplate := render("snek", snek.Body)

				templateToRender = append(append(templateToRender, tailTemplate.Bytes()...), snekTemplate.Bytes()...)
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
			if checkDirection(direction, "vertical") && checkDirection(response.Direction, "horizontal") {
				direction = response.Direction
			}
			if checkDirection(direction, "horizontal") && checkDirection(response.Direction, "vertical") {
				direction = response.Direction
			}
		}

	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		game := new(Game)
		tmpl := render("index", game)
		w.Write(tmpl.Bytes())
	})

	http.ListenAndServe(":8080", nil)
}
