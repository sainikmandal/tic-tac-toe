"use client";

import React, { useState, useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useSearchParams } from "next/navigation";

// CORS proxy for local development
const useCorsProxy = (url) => {
  // Only use the proxy in development and only for remote URLs
  const isDev = process.env.NODE_ENV === "development";
  const isRemoteUrl = url && !url.includes("localhost");

  if (isDev && isRemoteUrl) {
    return `https://cors-anywhere.herokuapp.com/${url}`;
  }

  return url;
};

const TicTacToe = () => {
  const [board, setBoard] = useState(Array(9).fill(""));
  const [gameId, setGameId] = useState<string | null>(null);
  const [playerSymbol, setPlayerSymbol] = useState<string>("");
  const [isMyTurn, setIsMyTurn] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<
    "connecting" | "connected" | "error"
  >("connecting");
  const wsRef = useRef<WebSocket | null>(null);
  const searchParams = useSearchParams();

  // Get API URL with fallback
  const baseApiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  const apiUrl = useCorsProxy(baseApiUrl);

  // Log environment for debugging
  useEffect(() => {
    console.log("Environment:", {
      NODE_ENV: process.env.NODE_ENV,
      API_URL: process.env.NEXT_PUBLIC_API_URL,
      WS_URL: process.env.NEXT_PUBLIC_WS_URL,
      USING_API_URL: apiUrl,
    });
  }, [apiUrl]);

  // Get WebSocket URL with fallback
  const getWsUrl = () => {
    if (process.env.NEXT_PUBLIC_WS_URL) return process.env.NEXT_PUBLIC_WS_URL;

    // If running in browser and in production (https protocol), use wss://
    if (typeof window !== "undefined") {
      const isSecure = window.location.protocol === "https:";
      return isSecure
        ? `wss://${window.location.host}`
        : `ws://${window.location.host}`;
    }

    return "ws://localhost:8080"; // Fallback for server-side rendering
  };

  useEffect(() => {
    const id = searchParams.get("game");
    if (id) {
      console.log("Joining game:", id);
      joinGame(id);
    } else {
      console.log("Creating new game");
      createGame();
    }

    // Cleanup WebSocket connection when component unmounts
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [searchParams]);

  useEffect(() => {
    if (!playerSymbol || !gameId) return;
    console.log("Initializing WebSocket with:", { gameId, playerSymbol });
    initWebSocket(gameId);
  }, [gameId, playerSymbol]);

  const createGame = async () => {
    try {
      setConnectionStatus("connecting");
      console.log(`Making API request to: ${apiUrl}/game/create`);

      const response = await fetch(`${apiUrl}/game/create`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        // Use no-cors mode in development for debugging
        mode: process.env.NODE_ENV === "development" ? "cors" : undefined,
      });

      console.log("Response status:", response.status);

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Server error: ${response.status}, ${errorText}`);
      }

      const data = await response.json();
      console.log("Game created:", data);
      setGameId(data.gameId);
      setPlayerSymbol("X");
      setIsMyTurn(true);
    } catch (error) {
      console.error("Error creating game:", error);
      setConnectionStatus("error");
    }
  };

  const joinGame = async (id: string) => {
    try {
      setConnectionStatus("connecting");
      console.log(`Making API request to: ${apiUrl}/game/join/${id}`);

      const response = await fetch(`${apiUrl}/game/join/${id}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        // Use no-cors mode in development for debugging
        mode: process.env.NODE_ENV === "development" ? "cors" : undefined,
      });

      console.log("Response status:", response.status);

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(
          `Failed to join game: ${response.status}, ${errorText}`,
        );
      }

      setGameId(id);
      setPlayerSymbol("O");
    } catch (error) {
      console.error("Error joining game:", error);
      setConnectionStatus("error");
    }
  };

  const initWebSocket = (id: string) => {
    // Close existing connection if any
    if (wsRef.current) {
      wsRef.current.close();
    }

    const wsUrl = getWsUrl();
    console.log(`Connecting to WebSocket: ${wsUrl}/ws/${id}`);

    const ws = new WebSocket(`${wsUrl}/ws/${id}`);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log("WebSocket connected, player symbol:", playerSymbol);
      setConnectionStatus("connected");
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log("Received message:", data);

        if (data.board) {
          setBoard(data.board);
        }

        if (data.nextPlayer && playerSymbol) {
          const isTurn = data.nextPlayer === playerSymbol;
          setIsMyTurn(isTurn);
          console.log("Is my turn:", isTurn);
        }

        if (data.type === "GAME_OVER") {
          alert(data.winner ? `Player ${data.winner} wins!` : "It's a draw!");
        }
      } catch (error) {
        console.error("Error processing WebSocket message:", error);
      }
    };

    ws.onerror = (error) => {
      console.error("WebSocket error:", error);
      setConnectionStatus("error");
    };

    ws.onclose = (event) => {
      console.log(
        `WebSocket connection closed: ${event.code}, ${event.reason}`,
      );
      setConnectionStatus("error");

      // Attempt to reconnect after a delay if component is still mounted
      setTimeout(() => {
        if (gameId && playerSymbol) {
          console.log("Attempting to reconnect WebSocket...");
          initWebSocket(gameId);
        }
      }, 3000);
    };
  };

  const handleClick = (index: number) => {
    if (
      !isMyTurn ||
      board[index] ||
      !wsRef.current ||
      wsRef.current.readyState !== WebSocket.OPEN
    )
      return;

    console.log(`Sending move: position ${index}, symbol ${playerSymbol}`);

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
      disabled={!isMyTurn || !!board[index] || connectionStatus !== "connected"}
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
          {connectionStatus === "connecting"
            ? "Connecting to game..."
            : connectionStatus === "error"
              ? "Connection error! Please try again."
              : isMyTurn
                ? "Your turn"
                : "Opponent's turn"}
        </div>
        <div className="grid grid-cols-3 gap-2">
          {board.map((_, index) => (
            <div key={index}>{renderSquare(index)}</div>
          ))}
        </div>
        {gameId && (
          <div className="mt-4 text-center">
            <p className="text-sm text-gray-500 mb-2">
              You are playing as: <strong>{playerSymbol}</strong>
            </p>
            <p className="text-sm text-gray-500 mb-2">Game ID: {gameId}</p>
            <p className="text-sm text-gray-500">Share this link to play:</p>
            <code className="block mt-2 p-2 bg-gray-100 rounded text-sm break-all">
              {shareableLink}
            </code>
          </div>
        )}
      </CardContent>
    </Card>
  );
};

export default TicTacToe;
