import { useEffect, useMemo, useState } from "react";
import ProductCard from "../components/ProductCard";
import { api } from "../api";
import type { Product } from "../types";

export default function Home() {
  const [products, setProducts] = useState<Product[]>([]);
  const [category, setCategory] = useState("todos");
  const [err, setErr] = useState("");

  useEffect(() => {
    api.products().then(setProducts).catch((e: Error) => setErr(e.message));
  }, []);

  const categories = useMemo(() => {
    const set = new Set(products.map((p) => p.category));
    return ["todos", ...[...set].sort()];
  }, [products]);

  const shown = products.filter((p) => category === "todos" || p.category === category);

  return (
    <>
      <section className="hero">
        <h1>Equipamentos pra todos os esportes.</h1>
        <p>
          Loja demo da Arena Store. Carrinho e pagamento são orquestrados por AWS Lambda
          Durable Functions em Go: reserva de estoque, PIX com expiração e atraso de cartão.
        </p>
      </section>
      {err && <div className="banner bad">{err}</div>}
      <div className="filters">
        {categories.map((c) => (
          <button key={c} className={c === category ? "on" : ""} onClick={() => setCategory(c)}>
            {c}
          </button>
        ))}
      </div>
      <div className="grid">
        {shown.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>
    </>
  );
}
