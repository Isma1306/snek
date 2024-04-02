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
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Board [10][10]Square

type Res struct {
	Direction string `json:"direction"`
}

type Game struct {
	Time  int
	Board Board
	Apple [2]int
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

func (g *Game) isEatingApple(snek Snek, apple [2]int) bool {
	return apple[0] == snek.Body[0].Position[0] && apple[1] == snek.Body[0].Position[1]
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
	g.Board[g.Apple[0]][g.Apple[1]].Fill = "apple"
	g.Board[g.Apple[0]][g.Apple[1]].IsOcupied = true
}

func (g *Game) generateApple() {
	newPostion := [2]int{rand.IntN(len(g.Board)), rand.IntN(len(g.Board))}
	if g.Board[newPostion[0]][newPostion[1]].IsOcupied {
		g.generateApple()
	} else {
		g.Apple = newPostion
	}
}

func newUnit(position [2]int) Unit {
	newUnit := new(Unit)
	newUnit.Position = position
	return *newUnit
}

func newSnek() Snek {
	newSnek := new(Snek)
	newSnek.Body = []Unit{newUnit([2]int{2, 5}), newUnit([2]int{1, 5}), newUnit([2]int{0, 5})}
	return *newSnek
}

func renderBoard(game Game) bytes.Buffer {
	tmpl := template.Must(template.ParseGlob("views/*.html"))
	buffer := bytes.Buffer{}
	tmpl.ExecuteTemplate(&buffer, "board", game)
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
		defer conn.Close()
		conn.SetCloseHandler(func(code int, text string) error {
			log.Printf("connection lost with client: %s", conn.RemoteAddr())
			return fmt.Errorf("connection close")
		})
		game := new(Game)
		snek := newSnek()
		game.generateBoard(snek)
		game.generateApple()
		direction := "right"

		// game loop
		go func() {
			for {
				time.Sleep(1000 * time.Millisecond)
				game.Time++
				eatApple := game.isEatingApple(snek, game.Apple)
				snek.move(direction, *game, eatApple)
				game.generateBoard(snek)
				if eatApple {
					game.Score += 100
					game.generateApple()
				}
				boardTemplate := renderBoard(*game)
				if err = conn.WriteMessage(websocket.TextMessage, boardTemplate.Bytes()); err != nil {
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
		http.ServeFile(w, r, "./views/index.html")
	})

	http.ListenAndServe(":8080", nil)
}
