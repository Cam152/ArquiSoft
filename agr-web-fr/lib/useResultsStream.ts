"use client";

// Suscripción a GET /results/{id}/stream (SSE) a través del gateway.
//
// EventSource ya reintenta solo cuando se corta la red. Pero si el gateway
// responde algo que no es un stream (por ejemplo 502 porque agr-results-lo está
// caído), el navegador se rinde y cierra la conexión. Para ese caso se vuelve a
// conectar a mano con espera creciente (2 s, 4 s, 8 s... hasta 30 s).

import { useEffect, useRef, useState } from "react";
import { parseCountsEvent, type Votes } from "./results";

export type StreamStatus = "off" | "connecting" | "live" | "reconnecting";

const MAX_DELAY_MS = 30_000;

export function useResultsStream(
  url: string | null,
  electionId: number,
  onCounts: (votes: Votes) => void,
): StreamStatus {
  const [status, setStatus] = useState<StreamStatus>(url ? "connecting" : "off");
  const onCountsRef = useRef(onCounts);

  useEffect(() => {
    onCountsRef.current = onCounts;
  }, [onCounts]);

  useEffect(() => {
    if (!url) {
      setStatus("off");
      return;
    }

    let source: EventSource | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let failures = 0;
    let stopped = false;

    const connect = () => {
      setStatus(failures === 0 ? "connecting" : "reconnecting");
      source = new EventSource(url);

      source.onopen = () => {
        failures = 0;
        setStatus("live");
      };

      source.addEventListener("counts", (event) => {
        const votes = parseCountsEvent((event as MessageEvent<string>).data, electionId);
        if (votes) onCountsRef.current(votes);
      });

      source.onerror = () => {
        if (stopped || !source) return;
        if (source.readyState === EventSource.CLOSED) {
          source.close();
          failures += 1;
          setStatus("reconnecting");
          const delay = Math.min(MAX_DELAY_MS, 2000 * 2 ** (failures - 1));
          retryTimer = setTimeout(connect, delay);
        } else {
          // CONNECTING: el navegador está reintentando por su cuenta.
          setStatus("reconnecting");
        }
      };
    };

    connect();
    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      source?.close();
    };
  }, [url, electionId]);

  return status;
}
