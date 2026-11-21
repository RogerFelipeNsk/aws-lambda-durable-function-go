export type Product = {
  id: string;
  name: string;
  description: string;
  category: string;
  priceCents: number;
  currency: string;
  emoji: string;
  available: number;
  reserved: number;
  sold: number;
};

export type User = {
  id: string;
  name: string;
  email: string;
};

export type TimelineEvent = {
  at: string;
  type: string;
  message: string;
};

export type CartItem = {
  productId: string;
  name: string;
  emoji: string;
  unitCents: number;
  quantity: number;
};

export type Cart = {
  id?: string;
  status: string;
  items: CartItem[];
  totalCents?: number;
  expiresAt?: string;
  orderId?: string;
  events?: TimelineEvent[];
};

export type PixCharge = {
  txId: string;
  copiaECola: string;
  expiresAt: string;
};

export type Order = {
  id: string;
  cartId: string;
  status: string;
  method: string;
  items: CartItem[];
  totalCents: number;
  currency: string;
  pix?: PixCharge;
  cardLast4?: string;
  events?: TimelineEvent[];
  createdAt: string;
};
