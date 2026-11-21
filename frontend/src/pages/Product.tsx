import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api, currentUser, formatBRL } from "../api";
import type { Product } from "../types";

export default function ProductPage() {
  const { id } = useParams();
  const nav = useNavigate();
  const [product, setProduct] = useState<Product | null>(null);
  const [qty, setQty] = useState(1);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!id) return;
    api.product(id).then(setProduct).catch((e: Error) => setErr(e.message));
  }, [id]);

  async function add() {
    if (!product) return;
    if (!currentUser()) {
      nav("/entrar");
      return;
    }
    setBusy(true);
    setErr("");
    try {
      await api.addItem(product.id, qty);
      nav("/carrinho");
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!product) return <p className="muted">{err || "Carregando..."}</p>;

  return (
    <div className="detail">
      <div className="stage">{product.emoji}</div>
      <div>
        <div className="status">{product.category}</div>
        <h1>{product.name}</h1>
        <p className="muted">{product.description}</p>
        <p className="price">{formatBRL(product.priceCents)}</p>
        <p className="stock">
          {product.available} em estoque livre · {product.reserved} reservados em carrinhos · {product.sold} vendidos
        </p>
        {err && <div className="banner bad">{err}</div>}
        <div className="row">
          <input
            type="number"
            min={1}
            max={product.available}
            value={qty}
            onChange={(e) => setQty(Number(e.target.value))}
            style={{ width: 80 }}
          />
          <button className="btn" disabled={busy || product.available < 1} onClick={add}>
            Reservar no carrinho
          </button>
          <Link className="btn ghost" to="/">
            Voltar
          </Link>
        </div>
      </div>
    </div>
  );
}
