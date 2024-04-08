package main

import (
	"errors"
	"math/rand/v2"
)

type Game struct {
	Time  int
	Board Board
	Snek  Snek
	Apple Unit
	Level int
	Score int
}

type Board [10][10]Square

type Square struct {
	IsOcupied bool
	Fill      string
}

type Unit struct {
	Position [2]int
}

type Snek struct {
	Direction string
	Body      []Unit
}

func (g *Game) isEatingApple(snek Snek, apple Unit) bool {
	return apple.Position[0] == snek.Body[0].Position[0] && apple.Position[1] == snek.Body[0].Position[1]
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
	for _, unit := range g.Snek.Body {
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

func (g *Game) newSnek() {
	newSnek := new(Snek)
	newSnek.Direction = "right"
	newSnek.Body = []Unit{newUnit([2]int{2, 5}), newUnit([2]int{1, 5}), newUnit([2]int{0, 5})}
	g.Snek = *newSnek
}
