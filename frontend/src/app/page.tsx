import TicTacToe from "@/components/TicTacToe";
import { Suspense } from "react";
import Head from "next/head";

export default function Home() {
  return (
    <>
      <Head>
        <title>Multiplayer Tic-Tac-Toe</title>
        <meta
          name="description"
          content="Play a multiplayer tic-tac-toe game with friends online."
        />
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      </Head>
      <main className="min-h-screen flex items-center justify-center p-8 bg-gray-50">
        <Suspense fallback={<div>Loading Tic-Tac-Toe...</div>}>
          <TicTacToe />
        </Suspense>
      </main>
    </>
  );
}
