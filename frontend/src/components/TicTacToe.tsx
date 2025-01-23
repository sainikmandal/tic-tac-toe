"use client";
import React, { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useSearchParams } from "next/navigation";

const TicTacToe = () => {
  const [board, setBoard] = useState(Array(9).fill(""));
  const [gameId, setGameId] = useState<string | null>(null);
  const [playerSymbol, setPlayerSymbol] = useState<string>("");
  const [wsConnection, setWsConnection] = useState<WebSocket | null>(null);
  const [isMyTurn, setIsMyTurn] = useState(false);
  const searchParams = useSearchParams();

  useEffect(() => {
    const id = searchParams.get("game");
    if (id) {
      joinGame(id);
    } else {
      createGame();
    }
  }, []);

  useEffect(() => {
    if (gameId && playerSymbol) {
      initWebSocket(gameId);
    }
    return () => {
      if (wsConnection) {
        wsConnection.close();
      }
    };
  }, [gameId, playerSymbol]);

  const createGame = async () => {
    try {
      const response = await fetch("http://localhost:8080/game/create", {
        method: "POST",
      });
      const data = await response.json();
      setGameId(data.gameId);
      setPlayerSymbol("X");
      setIsMyTurn(true);
      initWebSocket(data.gameId);
    } catch (error) {
      console.error("Error creating game:", error);
    }
  };

  const joinGame = async (id: string) => {
    try {
      const response = await fetch(`http://localhost:8080/game/join/${id}`, {
        method: "POST",
      });
      if (response.ok) {
        setGameId(id);
        setPlayerSymbol("O");
        initWebSocket(id);
      }
    } catch (error) {
      console.error("Error joining game:", error);
    }
  };

  const initWebSocket = (id: string) => {
    const ws = new WebSocket(`ws://localhost:8080/ws/${id}`);

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      console.log("Received message:", data); // Debug log
      console.log("Player Symbol:", playerSymbol); // Debug log
      console.log("Next Player:", data.nextPlayer); // Debug log

      setBoard(data.board);
      if (playerSymbol) {
        setIsMyTurn(data.nextPlayer === playerSymbol);
        console.log("Is my turn:", data.nextPlayer === playerSymbol); // Debug log
      }

      if (data.type === "GAME_OVER") {
        alert(data.winner ? `Player ${data.winner} wins!` : "It's a draw!");
      }
    };

    ws.onopen = () => {
      console.log("WebSocket connected, player symbol:", playerSymbol); // Debug log
    };

    ws.onerror = (error) => {
      console.error("WebSocket error:", error);
    };

    setWsConnection(ws);
  };

  const handleClick = (index: number) => {
    if (!isMyTurn || board[index] || !wsConnection) return;

    wsConnection.send(
      JSON.stringify({
        type: "MOVE",
        position: index,
        symbol: playerSymbol,
        gameId: gameId,
      }),
    );
  };

  const renderSquare = (index: number) => (
    <Button
      variant="outline"
      className="h-20 w-20 text-2xl font-bold"
      onClick={() => handleClick(index)}
    >
      {board[index]}
    </Button>
  );

  const shareableLink =
    typeof window !== "undefined"
      ? `${window.location.origin}?game=${gameId}`
      : "";

  return (
    <Card className="w-full max-w-md mx-auto">
      <CardContent className="p-6">
        <div className="text-lg font-medium mb-4 text-center">
          {isMyTurn ? "Your turn" : "Opponent's turn"}
        </div>
        <div className="grid grid-cols-3 gap-2">
          {board.map((_, index) => (
            <div key={index}>{renderSquare(index)}</div>
          ))}
        </div>
        {gameId && (
          <div className="mt-4 text-center">
            <p className="text-sm text-gray-500 mb-2">Game ID: {gameId}</p>
            <p className="text-sm text-gray-500">Share this link to play:</p>
            <code className="block mt-2 p-2 bg-gray-100 rounded text-sm">
              {shareableLink}
            </code>
          </div>
        )}
      </CardContent>
    </Card>
  );
};

export default TicTacToe;
