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
	w.Header().Set("Content-Type", "application/json")

	mu.Lock()
	gameID := generateGameID()
	games[gameID] = &Game{
		Board:      make([]string, 9),
		NextPlayer: "X",
	}
	mu.Unlock()

	log.Printf("Created new game: %s", gameID)
	json.NewEncoder(w).Encode(map[string]string{
		"gameId": gameID,
	})
}

func handleJoinGame(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	gameID := vars["id"]

	mu.RLock()
	_, exists := games[gameID]
	mu.RUnlock()

	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	log.Printf("Player joined game: %s", gameID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "joined",
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, upgrader *websocket.Upgrader) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	// Check if game exists
	mu.RLock()
	game, exists := games[gameID]
	mu.RUnlock()

	if !exists {
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Add this connection to the game
	mu.Lock()
	connections[gameID] = append(connections[gameID], conn)
	mu.Unlock()

	log.Printf("WebSocket connection established for game: %s", gameID)

	// Send initial game state
	if err := conn.WriteJSON(game); err != nil {
		log.Printf("Error sending initial state: %v", err)
		removeConnection(gameID, conn)
		return
	}

	// Handle incoming messages
	for {
		var move Move
		err := conn.ReadJSON(&move)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			removeConnection(gameID, conn)
			break
		}

		log.Printf("Received move: %+v", move)

		if move.Type == "MOVE" {
			makeMove(gameID, move.Position, move.Symbol)
		}
	}
}

func makeMove(gameID string, position int, symbol string) {
	mu.Lock()
	defer mu.Unlock()

	game := games[gameID]
	if game == nil {
		log.Printf("Game not found: %s", gameID)
		return
	}

	if game.GameOver {
		log.Printf("Game is already over")
		return
	}

	if position < 0 || position >= 9 {
		log.Printf("Invalid position: %d", position)
		return
	}

	if game.Board[position] != "" {
		log.Printf("Position already occupied: %d", position)
		return
	}

	if symbol != game.NextPlayer {
		log.Printf("Not player's turn. Expected %s, got %s", game.NextPlayer, symbol)
		return
	}

	game.Board[position] = symbol
	game.NextPlayer = nextPlayer(symbol)

	if checkWin(game.Board) {
		game.GameOver = true
		game.Winner = symbol
		log.Printf("Game %s won by %s", gameID, symbol)
		broadcastGameState(gameID, "GAME_OVER")
	} else if checkDraw(game.Board) {
		game.GameOver = true
		log.Printf("Game %s ended in a draw", gameID)
		broadcastGameState(gameID, "GAME_OVER")
	} else {
		broadcastGameState(gameID, "MOVE")
	}
}

func generateGameID() string {
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

	message := map[string]interface{}{
		"type":       messageType,
		"board":      game.Board,
		"nextPlayer": game.NextPlayer,
		"gameOver":   game.GameOver,
		"winner":     game.Winner,
	}

	mu.RLock()
	gameCons := connections[gameID]
	mu.RUnlock()

	for _, conn := range gameCons {
		if err := conn.WriteJSON(message); err != nil {
			log.Printf("Error broadcasting to client: %v", err)
			removeConnection(gameID, conn)
		}
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

	conn.Close()
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func main() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Load environment variables
	godotenv.Load()

	// Debug environment variables
	log.Printf("Environment variables:")
	log.Printf("PORT: %s", os.Getenv("PORT"))
	log.Printf("CORS_ORIGIN: %s", os.Getenv("CORS_ORIGIN"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get allowed origins
	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3000"
	}

	// Enable verbose logging
	log.Printf("Starting server with CORS origin: %s", corsOrigin)

	r := mux.NewRouter()

	// CORS setup with very permissive settings for debugging
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // Allow all origins for now
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		Debug:            true,
	})

	// WebSocket upgrader with relaxed origin check
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			log.Printf("WebSocket connection attempt from: %s", origin)
			return true // Allow all origins for now
		},
	}

	// Routes
	r.HandleFunc("/game/create", handleCreateGame).Methods("POST", "OPTIONS")
	r.HandleFunc("/game/join/{id}", handleJoinGame).Methods("POST", "OPTIONS")
	r.HandleFunc("/ws/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(w, r, &upgrader)
	})
	r.HandleFunc("/health", handleHealth).Methods("GET")

	// Handler with CORS
	handler := c.Handler(r)

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
