package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

type Game struct {
	Board      []string `json:"board"`
	NextPlayer string   `json:"nextPlayer"`
	GameOver   bool     `json:"gameOver"`
	Winner     string   `json:"winner"`
}

type Move struct {
	Type     string `json:"type"`
	Position int    `json:"position"`
	Symbol   string `json:"symbol"`
	GameID   string `json:"gameId"`
}

var (
	games       = make(map[string]*Game)
	connections = make(map[string][]*websocket.Conn)
	mu          sync.RWMutex
)

func handleCreateGame(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	gameID := generateGameID()
	games[gameID] = &Game{
		Board:      make([]string, 9),
		NextPlayer: "X",
	}
	mu.Unlock()

	json.NewEncoder(w).Encode(map[string]string{
		"gameId": gameID,
	})
}

func handleJoinGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	mu.RLock()
	_, exists := games[gameID]
	mu.RUnlock()

	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, upgrader *websocket.Upgrader) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	mu.Lock()
	connections[gameID] = append(connections[gameID], conn)
	game := games[gameID]
	mu.Unlock()

	// Send initial game state
	conn.WriteJSON(game)

	for {
		var move Move
		err := conn.ReadJSON(&move)
		if err != nil {
			log.Printf("Read error: %v", err)
			removeConnection(gameID, conn)
			break
		}

		if move.Type == "MOVE" {
			makeMove(gameID, move.Position, move.Symbol)
		}
	}
}

func makeMove(gameID string, position int, symbol string) {
	mu.Lock()
	defer mu.Unlock()

	game := games[gameID]
	if game == nil || game.GameOver || position < 0 || position >= 9 || game.Board[position] != "" || symbol != game.NextPlayer {
		return
	}

	game.Board[position] = symbol
	game.NextPlayer = nextPlayer(symbol)

	if checkWin(game.Board) {
		game.GameOver = true
		game.Winner = symbol
		broadcastGameState(gameID, "GAME_OVER")
	} else if checkDraw(game.Board) {
		game.GameOver = true
		broadcastGameState(gameID, "GAME_OVER")
	} else {
		broadcastGameState(gameID, "MOVE")
	}
}

func generateGameID() string {
	// Simple implementation - replace with more robust solution
	return "game_" + randomString(6)
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func nextPlayer(currentPlayer string) string {
	if currentPlayer == "X" {
		return "O"
	}
	return "X"
}

func broadcastGameState(gameID string, messageType string) {
	game := games[gameID]
	if game == nil {
		return
	}

	for _, conn := range connections[gameID] {
		conn.WriteJSON(map[string]interface{}{
			"type":       messageType,
			"board":      game.Board,
			"nextPlayer": game.NextPlayer,
			"gameOver":   game.GameOver,
			"winner":     game.Winner,
		})
	}
}

func removeConnection(gameID string, conn *websocket.Conn) {
	mu.Lock()
	defer mu.Unlock()

	if conns, exists := connections[gameID]; exists {
		for i, c := range conns {
			if c == conn {
				connections[gameID] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
	}
}

func checkWin(board []string) bool {
	winPatterns := [][]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // Rows
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // Columns
		{0, 4, 8}, {2, 4, 6}, // Diagonals
	}

	for _, pattern := range winPatterns {
		if board[pattern[0]] != "" &&
			board[pattern[0]] == board[pattern[1]] &&
			board[pattern[1]] == board[pattern[2]] {
			return true
		}
	}
	return false
}

func checkDraw(board []string) bool {
	for _, cell := range board {
		if cell == "" {
			return false
		}
	}
	return true
}

func main() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3000"
	}

	r := mux.NewRouter()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{corsOrigin},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	})

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == corsOrigin
		},
	}

	r.HandleFunc("/game/create", handleCreateGame).Methods("POST", "OPTIONS")
	r.HandleFunc("/game/join/{id}", handleJoinGame).Methods("POST", "OPTIONS")
	r.HandleFunc("/ws/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, &upgrader)
	})

	handler := c.Handler(r)
	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
