import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { recordSource } from "./dataSources";
import { subscribeSocket, socketConnected } from "./websocket";

export function useOrderStream(orderID?: string, enabled = true, market = false) {
  const qc = useQueryClient();
  const [connected, setConnected] = useState(socketConnected);
  useEffect(() => {
    if (!enabled) return;
    const page = window.location.pathname;
    const show = (state: string) => recordSource(page, { key: "order-stream", kind: "WebSocket", label: `/v1/realtime/ws · заявки${market ? ", котировки и стакан" : ""}`, state });
    show(socketConnected() ? "Подключено" : "Подключение…");
    let timer: ReturnType<typeof setTimeout> | undefined;
    const unsubscribe = subscribeSocket(message => {
      if (["ready", "connection", "disconnected"].includes(message.type)) {
        const online = message.type !== "disconnected";
        setConnected(online);
        show(online ? "Подключено" : "Переподключение…");
      }
      if (message.type !== "event" || !message.event_type?.startsWith("order") || (orderID && message.order_id !== orderID) || timer) return;
      timer = setTimeout(() => {
        timer = undefined;
        ["load-order-tracking", "load-test-requests", "active-load-orders-count", "load-test-request-errors"].forEach(key => { void qc.invalidateQueries({ queryKey: [key] }); });
        void qc.invalidateQueries({ predicate: query => typeof query.queryKey[0] === "string" && query.queryKey[0].startsWith("/v1/orders/") });
      }, 250);
    });
    return () => { unsubscribe(); clearTimeout(timer); show("Подписка закрыта"); };
  }, [qc, orderID, enabled, market]);
  return connected;
}
