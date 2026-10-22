import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { logoutToLogin } from "./api";
import { recordSource } from "./dataSources";
import { subscribeSocket } from "./websocket";

export function useSocketCache() {
  const qc = useQueryClient();
  useEffect(() => subscribeSocket(message => {
    if (message.type === "unauthorized") { logoutToLogin(); return; }
    if (message.type !== "snapshot" || !message.path || message.status !== 200) return;
    const path = message.path;
    if (path === "/v1/market") qc.setQueryData(["market"], message.data);
    else if (path === "/v1/positions") qc.setQueryData(["positions"], message.data);
    else if (path === "/v1/orders") { qc.setQueryData(["orders"], message.data); qc.setQueryData([path], message.data); }
    else { const book = path.match(/^\/v1\/market\/([^/]+)\/book$/); if (book) qc.setQueryData(["book", book[1]], message.data); }
    const page = window.location.pathname;
    if ((page === "/positions" && path === "/v1/positions") || (["/trading", "/order-book"].includes(page) && ["/v1/market", "/v1/positions", "/v1/orders"].some(prefix => path.startsWith(prefix))) || (page === "/orders" && path === "/v1/orders")) {
      recordSource(page, { key: `GET ${path}`, kind: "WebSocket", label: `GET ${path}`, state: "Обновлено из потока" });
    }
  }), [qc]);
}
