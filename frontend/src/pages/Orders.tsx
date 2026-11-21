import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api, currentUser, formatBRL } from "../api";
import { matchesFilter, orderLabel, orderTone, type OrderFilter } from "../status";
import type { Order } from "../types";

const filters: { id: OrderFilter; label: string }[] = [
  { id: "all", label: "Todas" },
  { id: "paid", label: "Pagas" },
  { id: "open", label: "Em andamento" },
  { id: "failed", label: "Não concluídas" },
];

export default function Orders() {
  const nav = useNavigate();
  const [orders, setOrders] = useState<Order[]>([]);
  const [filter, setFilter] = useState<OrderFilter>("all");
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!currentUser()) {
      nav("/entrar", { state: { from: "/compras" } });
      return;
    }
    let cancel = false;
    const load = () =>
      api
        .orders()
        .then((list) => {
          if (!cancel) setOrders(list);
        })
        .catch((e: Error) => {
          if (!cancel) setErr(e.message);
        });
    load();
    const id = setInterval(load, 4000);
    return () => {
      cancel = true;
      clearInterval(id);
    };
  }, [nav]);

  const shown = useMemo(
    () => orders.filter((o) => matchesFilter(o.status, filter)),
    [orders, filter],
  );
  const paidCount = orders.filter((o) => o.status === "paid").length;

  return (
    <>
      <section className="hero">
        <h1>Minhas compras</h1>
        <p>
          Pedidos desta sessão de visitante. Os pagos baixaram estoque; PIX expirado e cartão
          recusado devolveram as unidades.
        </p>
      </section>
      {err && <div className="banner bad">{err}</div>}
      <div className="filters">
        {filters.map((f) => (
          <button key={f.id} className={f.id === filter ? "on" : ""} onClick={() => setFilter(f.id)}>
            {f.label}
            {f.id === "paid" && paidCount > 0 ? ` (${paidCount})` : ""}
          </button>
        ))}
      </div>
      {shown.length === 0 && (
        <p className="muted">
          {orders.length === 0
            ? "Nenhuma compra ainda. Monte um carrinho e faça o checkout."
            : "Nenhum pedido neste filtro."}
        </p>
      )}
      {shown.map((order) => (
        <Link className="order-row" key={order.id} to={`/pedido/${order.id}`}>
          <div>
            <div className={`status tone-${orderTone(order.status)}`}>{orderLabel(order.status)}</div>
            <strong>{order.id}</strong>
            <div className="muted">
              {new Date(order.createdAt).toLocaleString("pt-BR")} · {order.method.toUpperCase()}
              {order.cardLast4 ? ` final ${order.cardLast4}` : ""}
            </div>
          </div>
          <div className="order-items muted">
            {order.items.map((item) => (
              <span key={item.productId}>
                {item.emoji} {item.name} × {item.quantity}
              </span>
            ))}
          </div>
          <strong className="price">{formatBRL(order.totalCents)}</strong>
        </Link>
      ))}
    </>
  );
}
