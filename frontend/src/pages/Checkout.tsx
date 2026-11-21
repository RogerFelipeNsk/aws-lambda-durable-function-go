import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, currentUser, formatBRL } from "../api";
import type { Cart } from "../types";

export default function Checkout() {
  const nav = useNavigate();
  const [cart, setCart] = useState<Cart | null>(null);
  const [method, setMethod] = useState<"pix" | "card">("pix");
  const [number, setNumber] = useState("1111111111111111");
  const [exp, setExp] = useState("01/2040");
  const [cvc, setCvc] = useState("111");
  const [simulate, setSimulate] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!currentUser()) {
      nav("/entrar");
      return;
    }
    api.cart().then(setCart).catch((e: Error) => setErr(e.message));
  }, [nav]);

  async function pay() {
    setBusy(true);
    setErr("");
    try {
      const order = await api.checkout({
        method,
        simulate: simulate || undefined,
        card: method === "card" ? { number, exp, cvc } : undefined,
      });
      nav(`/pedido/${order.id}`);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!cart) return <p className="muted">{err || "Carregando..."}</p>;

  return (
    <>
      <section className="hero">
        <h1>Checkout</h1>
        <p>Total {formatBRL(cart.totalCents ?? 0)}. Escolha PIX (QR 15 min) ou cartão de teste.</p>
      </section>
      {err && <div className="banner bad">{err}</div>}
      <div className="pay">
        <div className="panel">
          <h3>Pagamento</h3>
          <div className="row">
            <button className={method === "pix" ? "btn" : "btn ghost"} onClick={() => setMethod("pix")}>
              PIX
            </button>
            <button className={method === "card" ? "btn" : "btn ghost"} onClick={() => setMethod("card")}>
              Cartão
            </button>
          </div>
          {method === "card" && (
            <form className="stack" style={{ marginTop: 16 }} onSubmit={(e) => e.preventDefault()}>
              <label>Número (teste: 1111111111111111)</label>
              <input value={number} onChange={(e) => setNumber(e.target.value)} />
              <label>Validade</label>
              <input value={exp} onChange={(e) => setExp(e.target.value)} />
              <label>CVC</label>
              <input value={cvc} onChange={(e) => setCvc(e.target.value)} />
              <label>Simulação</label>
              <select value={simulate} onChange={(e) => setSimulate(e.target.value)}>
                <option value="">aprovação normal (com atraso da operadora)</option>
                <option value="card_delay">atraso extra da operadora</option>
                <option value="card_fail">recusar cartão</option>
              </select>
            </form>
          )}
          {method === "pix" && (
            <p className="muted" style={{ marginTop: 16 }}>
              Vamos gerar um QR Code EMV (copia e cola) com expiração. Você pode confirmar o
              pagamento nesta demo, ou deixar expirar para devolver o estoque.
            </p>
          )}
          <p>
            <button className="btn" disabled={busy} onClick={pay}>
              Confirmar pedido
            </button>
          </p>
        </div>
        <div className="panel">
          <h3>Itens</h3>
          {(cart.items ?? []).map((item) => (
            <p key={item.productId}>
              {item.emoji} {item.name} × {item.quantity} — {formatBRL(item.unitCents * item.quantity)}
            </p>
          ))}
        </div>
      </div>
    </>
  );
}
