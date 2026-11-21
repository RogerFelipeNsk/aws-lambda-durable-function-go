import { Link } from "react-router-dom";
import { formatBRL } from "../api";
import type { Product } from "../types";

export default function ProductCard({ product }: { product: Product }) {
  return (
    <Link className="card" to={`/produto/${product.id}`}>
      <div className="emoji">{product.emoji}</div>
      <h3>{product.name}</h3>
      <div className="grow muted">{product.description}</div>
      <div className="price">{formatBRL(product.priceCents)}</div>
      <div className="stock">
        {product.available} disponíveis
        {product.reserved > 0 ? ` · ${product.reserved} reservados` : ""}
      </div>
    </Link>
  );
}
