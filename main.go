package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Player struct {
	Id        int
	Snek      Snek
	Direction chan string
	Score     int
	Mu        *sync.Mutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
}

type Res struct {
	Direction string `json:"direction"`
}

type Lobby struct {
	Game Game
	Id   string
}
type Index struct {
	Text  string
	Games map[string]*Game
}

type Header struct {
	Text string
	Id   string
}

var games = make(map[string]*Game)

func main() {

	http.HandleFunc("/connect/", handleNewPlayer)

	http.HandleFunc("/create", func(w http.ResponseWriter, r *http.Request) {
		id := uuid.New().String()
		games[id] = new(Game)
		games[id].Id = id
		games[id].PlayersMu = new(sync.Mutex)
		games[id].GameMu = new(sync.Mutex)
		w.Header().Add("HX-Redirect", "/game/"+id)

	})

	http.HandleFunc("/game/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/game/"):]
		val, ok := games[id]
		if ok {
			tmpl := Render("lobby", val)
			w.Write(tmpl.Bytes())
		} else {
			tmpl := Render("index", Index{Text: "This lobby doesn't exist!", Games: games})
			w.Write(tmpl.Bytes())
			w.Header().Add("HX-Redirect", "/")
		}

	})

	http.HandleFunc("/start/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/start/"):]
		game, ok := games[id]
		if ok {
			if game.Time < 1 {
				log.Printf("game %s started", id)
				go gameLoop(game)
				go timeLoop(game)
			}
		} else {
			tmpl := Render("index", "Game already started!")
			w.Write(tmpl.Bytes())
			w.Header().Add("HX-Redirect", "/")
		}

	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		tmpl := Render("index", Index{Text: "Welcome to Snek!", Games: games})
		w.Write(tmpl.Bytes())
	})

	go removeEmptyGames()

	log.Printf("server ready listening @ %s", "0.0.0.0:10000")
	err := http.ListenAndServe("0.0.0.0:10000", nil)
	if err != nil {
		log.Fatalf("Server died: %s", err)
	}
}

func timeLoop(game *Game) {
	for {
		if len(game.Players) == 0 {
			game.Time = 0
			game.GameMu.Lock()
			delete(games, game.Id)
			game.GameMu.Unlock()
			log.Printf("game %s was deleted!", game.Id)
			break
		}
		time.Sleep(1 * time.Second)
		game.Time++
		tmpl := Render("time", game.Time)
		broadcastTmpl(tmpl.Bytes(), game)
	}
}

func gameLoop(game *Game) {
	for {
		_, ok := games[game.Id]
		if !ok {
			break
		}
		time.Sleep(time.Duration(600-(game.Level*50)) * time.Millisecond)

		templateToRender := []byte{}
		tailTemplate := []byte{}
		snekTemplate := []byte{}
		applesToDelete := []Unit{}

		for conn, player := range game.Players {
			isEatingApple := game.isEatingApple(player.Snek)
			tail := player.Snek.Body[len(player.Snek.Body)-1]
			player.Snek.move(*game, isEatingApple)
			if isEatingApple {
				applesToDelete = append(applesToDelete, player.Snek.Body[1])

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
				msg := Render("header", Header{Text: "You died!", Id: game.Id})
				player.Mu.Lock()
				defer player.Mu.Unlock()
				err := conn.WriteMessage(websocket.TextMessage, msg.Bytes())
				if err != nil {
					log.Println(err)
				}
				conn.Close()
				deletedSnek := Render("deleteSnek", player.Snek)
				templateToRender = append(templateToRender, deletedSnek.Bytes()...)
				game.PlayersMu.Lock()
				delete(game.Players, conn)
				game.PlayersMu.Unlock()
				appleToRemove := Render("empty", game.Apples[len(game.Apples)-1])
				templateToRender = append(templateToRender, appleToRemove.Bytes()...)
				game.Apples = game.Apples[:len(game.Apples)-1]

			}
		}

		game.generateBoard()

		if len(applesToDelete) > 0 {
			applesToUpdate := []byte{}
			for _, apple := range applesToDelete {
				for index, gameApple := range game.Apples {
					if gameApple.Position[0] == apple.Position[0] && gameApple.Position[1] == apple.Position[1] {
						game.Apples = append(game.Apples[:index], game.Apples[index+1:]...)
					}
				}
				if len(game.Apples) < len(game.Players) {
					newApple := game.generateApple()
					newRender := Render("apple", newApple)
					applesToUpdate = append(applesToUpdate, newRender.Bytes()...)
				}
			}
			templateToRender = append(templateToRender, applesToUpdate...)
		}
		templateToRender = append(templateToRender, tailTemplate...)
		templateToRender = append(templateToRender, snekTemplate...)
		broadcastTmpl(templateToRender, game)
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

func broadcastTmpl(tmpl []byte, game *Game) {
	for client, player := range game.Players {
		player.Mu.Lock()
		defer player.Mu.Unlock()
		err := client.WriteMessage(websocket.TextMessage, tmpl)
		if err != nil {
			log.Println(err)
			client.Close()
			delete(game.Players, client)
		}
	}
}

func handleNewPlayer(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/connect/"):]
	game, ok := games[id]
	if !ok {
		tmpl := Render("index", Index{Text: "Couldn't connect with the game!", Games: games})
		w.Write(tmpl.Bytes())
	} else {

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Error upgrading: %s", err)
		}
		if len(game.Players) == 0 {
			startGame(game)
		}
		if len(game.Players) >= game.MaxPlayers {
			msg := Render("header", Header{Text: "The lobby is Full!", Id: game.Id})
			err := conn.WriteMessage(websocket.TextMessage, msg.Bytes())
			if err != nil {
				log.Println(err)
			}
			conn.Close()
		} else {
			newId := getUnusedId(game)
			player := Player{Snek: game.newSnek(fmt.Sprintf("player%v", newId)), Direction: make(chan string), Score: 0, Id: newId, Mu: &sync.Mutex{}}
			game.PlayersMu.Lock()
			game.Players[conn] = &player
			game.PlayersMu.Unlock()
			sneksToRender := []byte{}
			appleToRender := []byte{}
			if len(game.Apples) < len(game.Players) {
				game.generateApple()
				for _, apple := range game.Apples {
					appleTemp := Render("apple", apple)
					appleToRender = append(appleToRender, appleTemp.Bytes()...)
				}
			}

			for _, player := range game.Players {
				snekTemp := Render("snek", player.Snek.Body)
				sneksToRender = append(sneksToRender, snekTemp.Bytes()...)
			}
			broadcastTmpl(append(appleToRender, sneksToRender...), game)
			defer conn.Close()
			conn.SetCloseHandler(func(code int, text string) error {
				log.Printf("connection lost with client: %s", conn.RemoteAddr())
				conn.Close()
				game.PlayersMu.Lock()
				delete(game.Players, conn)
				game.PlayersMu.Unlock()
				return fmt.Errorf("connection close")
			})

			go func() {
				for {
					player.Snek.Direction = <-player.Direction
				}
			}()

			// read messages
			for {
				msgType, msg, err := conn.ReadMessage()
				if err != nil {
					if msgType == -1 {
						return
					}
					log.Printf("Error reading message: %s", err)
					return
				}
				response := Res{}
				err = json.Unmarshal([]byte(msg), &response)
				if err != nil {
					log.Println("Error parsing json")
					return

				}
				if CheckDirection(player.Snek.Direction, "vertical") && CheckDirection(response.Direction, "horizontal") {
					player.Direction <- response.Direction
				}
				if CheckDirection(player.Snek.Direction, "horizontal") && CheckDirection(response.Direction, "vertical") {
					player.Direction <- response.Direction
				}
			}
		}
	}
}

func startGame(game *Game) {
	game.MaxPlayers = 4
	game.Level = 5
	game.Players = make(map[*websocket.Conn]*Player)
	game.generateBoard()

}

func getUnusedId(game *Game) int {
	id := rand.IntN(game.MaxPlayers)
	for _, player := range game.Players {
		if id == player.Id {
			id = getUnusedId(game)
		}
	}
	return id

}

func removeEmptyGames() {
	for {
		time.Sleep(60 * time.Second)
		for _, game := range games {
			if len(game.Players) == 0 {
				log.Printf("Remove game %s because it's empty", game.Id)
				game.GameMu.Lock()
				delete(games, game.Id)
				game.GameMu.Unlock()
			}
		}
	}

}
