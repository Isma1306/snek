package main

import (
	"errors"
	"math/rand/v2"

	"github.com/gorilla/websocket"
)

type Game struct {
	MaxPlayers int
	Id      string
	Time    int
	Board   Board
	Snek    Snek
	Apples  []Unit
	Level   int
	Score   int
	Players map[*websocket.Conn]*Player
}

type Board [20][20]Square

type Square struct {
	IsOcupied bool
	Fill      string
}

type Unit struct {
	Color    string
	Position [2]int
}

type Snek struct {
	Direction string
	Body      []Unit
}

func (g *Game) isEatingApple(snek Snek) bool {
	return g.Board[snek.Body[0].Position[0]][snek.Body[0].Position[1]].Fill == "apple"
}

func (s *Snek) move(game Game, eatApple bool) error {
	var dx, dy int
	lastPosition := len(s.Body) - 1
	endOfBoard := len(game.Board) - 1
	switch s.Direction {
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

func (g *Game) generateBoard() {
	g.Board = *new(Board)
	for _, player := range g.Players {
		for _, unit := range player.Snek.Body {
			g.Board[unit.Position[0]][unit.Position[1]].Fill = "snek"
			g.Board[unit.Position[0]][unit.Position[1]].IsOcupied = true
		}
	}
	for _, apple := range g.Apples {
		g.Board[apple.Position[0]][apple.Position[1]].Fill = "apple"
		g.Board[apple.Position[0]][apple.Position[1]].IsOcupied = true
	}

}

func (g *Game) generateApple() Unit {
	var unit = Unit{}
	newPostion := [2]int{rand.IntN(len(g.Board)), rand.IntN(len(g.Board))}
	if g.Board[newPostion[0]][newPostion[1]].IsOcupied {
		g.generateApple()
	} else {
		unit = newUnit(newPostion, "apple")
		g.Apples = append(g.Apples, unit)
	}
	return unit
}

func (g *Game) checkCollision(snek Snek) bool {
	return g.Board[snek.Body[0].Position[0]][snek.Body[0].Position[1]].Fill == "snek"
}

func newUnit(position [2]int, color string) Unit {
	return Unit{Position: position, Color: color}
}

func (g *Game) newSnek(player string) Snek {
	newSnek := new(Snek)
	switch player {
	case "player0":
		newSnek.Direction = "right"
		newSnek.Body = []Unit{newUnit([2]int{2, 5}, player), newUnit([2]int{1, 5}, player), newUnit([2]int{0, 5}, player)}
	case "player1":
		newSnek.Direction = "left"
		newSnek.Body = []Unit{newUnit([2]int{17, 10}, player), newUnit([2]int{18, 10}, player), newUnit([2]int{19, 10}, player)}
	case "player2":
		newSnek.Direction = "down"
		newSnek.Body = []Unit{newUnit([2]int{10, 17}, player), newUnit([2]int{10, 18}, player), newUnit([2]int{10, 19}, player)}
	case "player3":
		newSnek.Direction = "up"
		newSnek.Body = []Unit{newUnit([2]int{12, 2}, player), newUnit([2]int{12, 1}, player), newUnit([2]int{12, 0}, player)}

	}

	return *newSnek
}
