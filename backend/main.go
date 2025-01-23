package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
)

type Game struct {
	ID      string
	Board   []string
	Players map[string]*websocket.Conn
	Turn    string
	mutex   sync.Mutex
}

type GameState struct {
	Type       string   `json:"type"`
	Board      []string `json:"board,omitempty"`
	NextPlayer string   `json:"nextPlayer,omitempty"`
	Winner     string   `json:"winner,omitempty"`
}

type MoveMessage struct {
	Type     string `json:"type"`
	Position int    `json:"position"`
	Symbol   string `json:"symbol"`
}

var (
	games    = make(map[string]*Game)
	mutex    sync.Mutex
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
)

func main() {
	r := mux.NewRouter()

	// Enable CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	})

	// Routes
	r.HandleFunc("/game/create", createGame).Methods("POST")
	r.HandleFunc("/game/join/{id}", joinGame).Methods("POST")
	r.HandleFunc("/ws/{gameId}", handleWebSocket)

	handler := c.Handler(r)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func createGame(w http.ResponseWriter, r *http.Request) {
	gameID := generateGameID()
	game := &Game{
		ID:      gameID,
		Board:   make([]string, 9),
		Players: make(map[string]*websocket.Conn),
		Turn:    "X",
	}

	mutex.Lock()
	games[gameID] = game
	mutex.Unlock()

	json.NewEncoder(w).Encode(map[string]string{"gameId": gameID})
}

func joinGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	mutex.Lock()
	game, exists := games[gameID]
	mutex.Unlock()

	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	if len(game.Players) >= 2 {
		http.Error(w, "Game is full", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["gameId"]

	mutex.Lock()
	game, exists := games[gameID]
	mutex.Unlock()

	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	symbol := "O"
	if len(game.Players) == 0 {
		symbol = "X"
	}

	game.mutex.Lock()
	game.Players[symbol] = conn
	game.mutex.Unlock()

	go handlePlayer(conn, game, symbol)
}

func handlePlayer(conn *websocket.Conn, game *Game, symbol string) {
	// Send initial game state to the player
	state := GameState{
		Type:       "MOVE",
		Board:      game.Board,
		NextPlayer: game.Turn,
	}
	conn.WriteJSON(state)

	for {
		var msg MoveMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		if msg.Type != "MOVE" || game.Turn != msg.Symbol {
			continue
		}

		game.mutex.Lock()
		if game.Board[msg.Position] == "" {
			game.Board[msg.Position] = msg.Symbol
			game.Turn = switchPlayer(game.Turn)

			winner := checkWinner(game.Board)
			state := GameState{
				Type:       "MOVE",
				Board:      game.Board,
				NextPlayer: game.Turn,
			}

			if winner != "" || isBoardFull(game.Board) {
				state.Type = "GAME_OVER"
				state.Winner = winner
			}

			// Broadcast to all players
			for _, playerConn := range game.Players {
				err := playerConn.WriteJSON(state)
				if err != nil {
					log.Printf("Error sending message: %v", err)
				}
			}
		}
		game.mutex.Unlock()
	}
}

func generateGameID() string {
	mutex.Lock()
	id := len(games) + 1
	mutex.Unlock()
	return fmt.Sprintf("%d", id)
}

func switchPlayer(current string) string {
	if current == "X" {
		return "O"
	}
	return "X"
}

func checkWinner(board []string) string {
	lines := [][]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // Rows
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // Columns
		{0, 4, 8}, {2, 4, 6}, // Diagonals
	}

	for _, line := range lines {
		if board[line[0]] != "" &&
			board[line[0]] == board[line[1]] &&
			board[line[0]] == board[line[2]] {
			return board[line[0]]
		}
	}
	return ""
}

func isBoardFull(board []string) bool {
	for _, cell := range board {
		if cell == "" {
			return false
		}
	}
	return true
}
