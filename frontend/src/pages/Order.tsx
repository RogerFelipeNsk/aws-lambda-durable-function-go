import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import PixQR from "../components/PixQR";
import Timeline from "../components/Timeline";
import { api, formatBRL } from "../api";
import { orderLabel } from "../status";
import type { Order } from "../types";

export default function OrderPage() {
  const { id } = useParams();
  const [order, setOrder] = useState<Order | null>(null);
  const [err, setErr] = useState("");
  const [now, setNow] = useState(Date.now());
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!id) return;
    let cancel = false;
    const load = () =>
      api
        .order(id)
        .then((o) => {
          if (!cancel) setOrder(o);
        })
        .catch((e: Error) => {
          if (!cancel) setErr(e.message);
        });
    load();
    const poll = setInterval(load, 2000);
    const tick = setInterval(() => setNow(Date.now()), 1000);
    return () => {
      cancel = true;
      clearInterval(poll);
      clearInterval(tick);
    };
  }, [id]);

  if (!order) return <p className="muted">{err || "Carregando pedido..."}</p>;

  const pixLeft = order.pix ? Math.max(0, new Date(order.pix.expiresAt).getTime() - now) : 0;
  const awaiting = order.status === "awaiting_pix";

  async function confirm() {
    if (!order) return;
    setBusy(true);
    setErr("");
    try {
      setOrder(await api.confirmPix(order.id));
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function expire() {
    if (!order) return;
    setBusy(true);
    setErr("");
    try {
      setOrder(await api.expirePix(order.id));
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <section className="hero">
        <div className="status">{orderLabel(order.status)}</div>
        <h1>Pedido {order.id}</h1>
        <p>
          {order.method.toUpperCase()} · {formatBRL(order.totalCents)}
          {order.cardLast4 ? ` · final ${order.cardLast4}` : ""}
        </p>
      </section>
      {err && <div className="banner bad">{err}</div>}
      {order.status === "paid" && <div className="banner ok">Pagamento confirmado. Estoque baixado.</div>}
      {order.status === "pix_expired" && (
        <div className="banner bad">PIX expirado. As unidades voltaram ao estoque.</div>
      )}
      {order.status === "declined" && <div className="banner bad">Cartão recusado. Estoque devolvido.</div>}
      {order.status === "processing_card" && (
        <div className="banner info">Operadora fictícia processando o cartão…</div>
      )}

      {awaiting && order.pix && (
        <div className="panel">
          <h3>Pague com PIX</h3>
          <p className="muted">
            Expira em {Math.floor(pixLeft / 60000)}m {Math.floor((pixLeft / 1000) % 60)}s
          </p>
          <PixQR payload={order.pix.copiaECola} />
          <div className="row" style={{ marginTop: 16 }}>
            <button className="btn" disabled={busy} onClick={confirm}>
              Simular pagamento PIX
            </button>
            <button className="btn danger" disabled={busy} onClick={expire}>
              Simular expiração
            </button>
          </div>
        </div>
      )}

      <div className="panel">
        <h3>Itens</h3>
        {order.items.map((item) => (
          <p key={item.productId}>
            {item.emoji} {item.name} × {item.quantity}
          </p>
        ))}
      </div>
      <div className="panel">
        <h3>Eventos da orquestração</h3>
        <Timeline events={order.events} />
      </div>
      <p className="row">
        <Link className="btn ghost" to="/compras">
          Minhas compras
        </Link>
        <Link className="btn ghost" to="/">
          Voltar à loja
        </Link>
      </p>
    </>
  );
}
