"use client";

import React, { useState, useEffect, useCallback, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useSearchParams } from "next/navigation";

const TicTacToe = () => {
  const [board, setBoard] = useState(Array(9).fill(""));
  const [gameId, setGameId] = useState<string | null>(null);
  const [playerSymbol, setPlayerSymbol] = useState<string>("");
  const [isMyTurn, setIsMyTurn] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const searchParams = useSearchParams();

  const initWebSocket = useCallback((id: string, symbol: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    const ws = new WebSocket(`ws://localhost:8080/ws/${id}`);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      console.log("Received data:", data, "My symbol:", symbol);
      setBoard(data.board);

      if (data.type === "MOVE") {
        const nextTurn = data.nextPlayer === symbol;
        console.log("Setting turn to:", nextTurn);
        setIsMyTurn(nextTurn);
      }

      if (data.type === "GAME_OVER") {
        alert(data.winner ? `Player ${data.winner} wins!` : "It's a draw!");
      }
    };

    ws.onopen = () => {
      console.log("WebSocket connected for player:", symbol);
    };
  }, []);

  useEffect(() => {
    const id = searchParams.get("game");

    const setupGame = async () => {
      try {
        if (id) {
          const response = await fetch(
            `http://localhost:8080/game/join/${id}`,
            {
              method: "POST",
            },
          );
          if (response.ok) {
            setGameId(id);
            const symbol = "O";
            setPlayerSymbol(symbol);
            setIsMyTurn(false);
            initWebSocket(id, symbol);
          }
        } else {
          const response = await fetch("http://localhost:8080/game/create", {
            method: "POST",
          });
          const data = await response.json();
          setGameId(data.gameId);
          const symbol = "X";
          setPlayerSymbol(symbol);
          setIsMyTurn(true);
          initWebSocket(data.gameId, symbol);
        }
      } catch (error) {
        console.error("Game setup error:", error);
      }
    };

    setupGame();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [searchParams, initWebSocket]);

  const handleClick = (index: number) => {
    if (!isMyTurn || board[index] || !wsRef.current || !gameId) return;

    wsRef.current.send(
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
      disabled={!isMyTurn || !!board[index]}
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
            <p className="text-sm text-gray-500 mb-2">
              You are playing as: {playerSymbol}
            </p>
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
