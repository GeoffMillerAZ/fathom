// The TypeScript consumer in the system-graph fixture fleet: HTTP calls to
// orders-api and a gRPC-web call to inventory-grpc.
import axios from "axios";
import { InventoryClient } from "./gen/inventory_grpc_web_pb";

const API = "https://orders.fauxtown.example";

export async function listOrders() {
  return fetch(`${API}/orders`);
}

export async function createOrder(body: string) {
  return fetch(`${API}/orders`, { method: "POST", body });
}

export async function getOrder(id: string) {
  return axios.get(`${API}/orders/${id}`);
}

export async function releaseStock(sku: string) {
  return new InventoryClient(API).release({ sku });
}

// Seeded negative: a string that looks like a URL, never called.
export const help = "see https://orders.fauxtown.example/orders/archive for details";
