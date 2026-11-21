const labels: Record<string, string> = {
  paid: "Pago",
  awaiting_pix: "Aguardando PIX",
  processing_card: "Processando cartão",
  pending_payment: "Pendente",
  pix_expired: "PIX expirado",
  declined: "Recusado",
  cancelled: "Cancelado",
};

export function orderLabel(status: string): string {
  return labels[status] ?? status;
}

export function orderTone(status: string): "ok" | "info" | "bad" | "muted" {
  switch (status) {
    case "paid":
      return "ok";
    case "awaiting_pix":
    case "processing_card":
    case "pending_payment":
      return "info";
    case "pix_expired":
    case "declined":
    case "cancelled":
      return "bad";
    default:
      return "muted";
  }
}

export type OrderFilter = "all" | "paid" | "open" | "failed";

export function matchesFilter(status: string, filter: OrderFilter): boolean {
  switch (filter) {
    case "paid":
      return status === "paid";
    case "open":
      return status === "awaiting_pix" || status === "processing_card" || status === "pending_payment";
    case "failed":
      return status === "pix_expired" || status === "declined" || status === "cancelled";
    default:
      return true;
  }
}
