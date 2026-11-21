import type { Cart, Order, Product, User } from "./types";

const base = import.meta.env.VITE_API_URL ?? "";

type GuestLogin = { token: string; user: User; expiresAt: string };

function token(): string | null {
  return localStorage.getItem("arena.token");
}

export function saveSession(t: string, user: User) {
  localStorage.setItem("arena.token", t);
  localStorage.setItem("arena.user", JSON.stringify(user));
}

export function clearSession() {
  localStorage.removeItem("arena.token");
  localStorage.removeItem("arena.user");
}

export function currentUser(): User | null {
  const raw = localStorage.getItem("arena.user");
  if (!raw) return null;
  try {
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const t = token();
  if (t) headers.set("Authorization", `Bearer ${t}`);
  const res = await fetch(`${base}${path}`, { ...init, headers });
  if (res.status === 401) {
    clearSession();
  }
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  products: () => request<Product[]>("/api/products"),
  product: (id: string) => request<Product>(`/api/products/${id}`),
  guest: () => request<GuestLogin>("/api/auth/guest", { method: "POST" }),
  me: () => request<User>("/api/me"),
  cart: () => request<Cart>("/api/cart"),
  addItem: (productId: string, quantity = 1) =>
    request<Cart>("/api/cart/items", {
      method: "POST",
      body: JSON.stringify({ productId, quantity }),
    }),
  updateItem: (id: string, quantity: number) =>
    request<Cart>(`/api/cart/items/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ quantity }),
    }),
  removeItem: (id: string) =>
    request<Cart>(`/api/cart/items/${id}`, { method: "DELETE" }),
  checkout: (body: {
    method: "pix" | "card";
    card?: { number: string; exp: string; cvc: string };
    simulate?: string;
  }) => request<Order>("/api/checkout", { method: "POST", body: JSON.stringify(body) }),
  orders: () => request<Order[]>("/api/orders"),
  order: (id: string) => request<Order>(`/api/orders/${id}`),
  confirmPix: (id: string) =>
    request<Order>(`/api/orders/${id}/pix/confirm`, { method: "POST" }),
  expirePix: (id: string) =>
    request<Order>(`/api/orders/${id}/pix/expire`, { method: "POST" }),
};

export function formatBRL(cents: number): string {
  return (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}
