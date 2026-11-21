import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import Timeline from "../components/Timeline";
import { api, currentUser, formatBRL } from "../api";
import type { Cart } from "../types";

export default function CartPage() {
  const nav = useNavigate();
  const [cart, setCart] = useState<Cart | null>(null);
  const [err, setErr] = useState("");
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    if (!currentUser()) {
      nav("/entrar");
      return;
    }
    let cancel = false;
    const load = () =>
      api
        .cart()
        .then((c) => {
          if (!cancel) setCart(c);
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
  }, [nav]);

  async function setQty(id: string, quantity: number) {
    setErr("");
    try {
      setCart(await api.updateItem(id, quantity));
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  if (!cart) return <p className="muted">{err || "Carregando carrinho..."}</p>;

  const remaining = cart.expiresAt ? Math.max(0, new Date(cart.expiresAt).getTime() - now) : 0;
  const expired = cart.status === "expired";

  return (
    <>
      <section className="hero">
        <h1>Carrinho</h1>
        <p>
          Status da sessão durable: <span className="status">{cart.status}</span>
        </p>
      </section>
      {cart.expiresAt && cart.status === "open" && (
        <div className="banner info">
          Reserva ativa. Expira em {Math.floor(remaining / 60000)}m {Math.floor((remaining / 1000) % 60)}s.
          Se expirar, as unidades voltam ao estoque.
        </div>
      )}
      {expired && (
        <div className="banner bad">Carrinho expirado. O estoque reservado foi devolvido.</div>
      )}
      {err && <div className="banner bad">{err}</div>}
      {(cart.items ?? []).length === 0 && <p className="muted">Carrinho vazio.</p>}
      {(cart.items ?? []).map((item) => (
        <div className="cart-item" key={item.productId}>
          <div className="emoji">{item.emoji}</div>
          <div>
            <strong>{item.name}</strong>
            <div className="muted">{formatBRL(item.unitCents)} un.</div>
          </div>
          <div className="qty">
            <button onClick={() => setQty(item.productId, item.quantity - 1)}>−</button>
            <span>{item.quantity}</span>
            <button onClick={() => setQty(item.productId, item.quantity + 1)}>+</button>
          </div>
          <strong>{formatBRL(item.unitCents * item.quantity)}</strong>
        </div>
      ))}
      <div className="panel">
        <div className="row">
          <h3 style={{ margin: 0 }}>Total {formatBRL(cart.totalCents ?? 0)}</h3>
          <div className="spacer" />
          <Link className="btn ghost" to="/">
            Continuar comprando
          </Link>
          <button
            className="btn"
            disabled={!cart.items?.length || cart.status !== "open"}
            onClick={() => nav("/checkout")}
          >
            Ir para checkout
          </button>
        </div>
      </div>
      <div className="panel">
        <h3>Linha do tempo da sessão</h3>
        <Timeline events={cart.events} />
      </div>
    </>
  );
}
