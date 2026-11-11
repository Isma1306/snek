# Snek

Snake game made with golang, websockets and htmx, because why not?

Snek is a multiplayer snake game that runs in the browser. Up to 4 players share one board in real time. The server holds all the game state and sends HTML fragments over a WebSocket, and htmx swaps them into the page. The only JavaScript is a small key listener.

## Features

- **Lobbies:** create a lobby, share its URL, and see open games with their player count and running time on the main page.
- **Up to 4 players per lobby:** each player gets their own color and starting position.
- **20×20 board that wraps around:** going off one edge brings you back on the opposite side.

## ⚠️ This is a bad architecture (on purpose)

Don't use this as a reference for building a real-time game. htmx is built for hypermedia: the user does something, the server answers with HTML, and the page updates. A game needs to push state to every player many times per second, which htmx was never designed for. This project is an experiment to see if I could make it work anyway.

Why it's a bad fit:

- **Too much data per update.** The server sends HTML elements (`<div class="..." id="..." hx-swap-oob="...">`) instead of a few bytes of JSON or binary. I keep this small by never sending the whole board. On each tick it sends only the cells that changed: each snake's new head, the tails it clears, and any new apples. Full snake bodies and apples are only sent when a player joins. Even so, each changed cell is a full HTML element.
- **Rendering on the server on every tick.** The server executes Go templates for every snake, apple and tail on every tick, for every lobby. Drawing is work the client should do.
- **Slow DOM updates.** Each cell is a `<div>`, and htmx has to find and replace each one by id. A `<canvas>` would redraw the whole board much more cheaply.
- **Lag you can see.** The client has no game logic, so it can't predict or smooth out movement. Every frame waits on the network, so any latency shows up as stutter.

It does work, though.

## Tech stack

| Layer    | What                                                                     |
| -------- | ------------------------------------------------------------------------ |
| Backend  | Go 1.22, `net/http`, `html/template`                                     |
| Realtime | [gorilla/websocket](https://github.com/gorilla/websocket)                |
| Frontend | [htmx](https://htmx.org) 1.9.11 + the `ws` extension (loaded from unpkg) |
| Dev      | [air](https://github.com/air-verse/air) for live reload                  |

## Getting started

### Run locally

You need Go 1.22 or newer.

```sh
go run .
```

Then open <http://localhost:10000>.

The server loads the templates from `views/` (relative to the current directory) every time it renders, so run it from the repository root.

### Live reload with air

```sh
air
```

The [.air.toml](.air.toml) config rebuilds to `./tmp/main` whenever a `.go` or `.html` file changes.

### Docker

```sh
docker build -t snek .
docker run --rm -p 10000:10000 snek
```

This is a multi-stage build: it compiles a static binary with `golang:latest` and then copies the binary and the `views/` folder into an `alpine` image.

## How to play

1. Open the main page and click **Create Lobby!**. You are sent to `/game/<id>`.
2. Share the lobby URL with friends (up to 4 players in total).
3. Click **Start the game!**.
4. Steer with **WASD** or the **arrow keys**. You can only make 90° turns, so you can't reverse straight back into yourself.

## How it works

```
Browser (htmx + ws ext)  ───────────────────  Go server
──────────────────                               ─────────
GET /                      ───────────────────▶  index page + lobby list
GET /create  (hx-get)   ───────────────────▶  new game, HX-Redirect → /game/<id>
GET /game/<id>          ───────────────────▶  lobby page with empty 20×20 grid
WS  /connect/<id>       ◀──────────────────▶  player joins; snakes and apples sent
GET /start/<id>         ───────────────────▶  starts gameLoop + timeLoop goroutines

keypress → ws-send {"direction":"up"}  ───▶  player's Direction channel
                    ◀── OOB HTML fragments   (each tick: heads, cleared tails, apples, score, time)
```

- **Rendering:** every cell on the board has an id like `block-x3y7`. Each tick, the server renders only the cells that changed (new head positions, cleared tails, apples) as `hx-swap-oob` fragments and broadcasts them to every player in the lobby. htmx swaps each fragment into the matching cell.
- **Game loop** (`gameLoop` in [main.go](main.go)): sleeps `600 - level*50` ms per tick (350 ms at the default level 5). On each tick it moves every snake, checks for apples and collisions, rebuilds the board, and broadcasts the changes.
- **Time loop** (`timeLoop`): updates the timer once per second, and deletes the game when no players are left.
- **Concurrency:** each game has mutexes for its player map and for the global games map, and each player has a mutex for writing to their WebSocket.

## Project structure

```
.
├── main.go          # HTTP routes, WebSocket handling, game and time loops, broadcasting
├── game.go          # Game/Board/Snek types, movement, apples, collisions, spawn points
├── views/
│   ├── index.html   # Main page
│   ├── lobbyList.html # Table of open games
│   ├── lobby.html   # Game page: board, score, timer, key handler, ws connection
│   └── board.html   # Fragments sent over the socket (unit, empty, apple, score, time, header…)
├── Dockerfile
└── .air.toml
```

## Configuration

These values are hardcoded for now:

| Setting        | Value           | Where                      |
| -------------- | --------------- | -------------------------- |
| Listen address | `0.0.0.0:10000` | `main()` in `main.go`      |
| Max players    | 4               | `startGame()` in `main.go` |
| Level (speed)  | 5               | `startGame()` in `main.go` |
| Board size     | 20×20           | `Board` type in `game.go`  |

## Known limitations / TODO

- Collision detection still has rough edges (see the `TODO` in `gameLoop`), for example when two heads move into the same cell on the same tick.
- Synchronization uses mutexes for now. The plan is to move to channels.
