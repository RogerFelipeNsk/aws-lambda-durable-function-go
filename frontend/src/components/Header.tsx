import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api, clearSession, currentUser } from "../api";
import type { Cart, User } from "../types";

export default function Header() {
  const [user, setUser] = useState<User | null>(currentUser());
  const [count, setCount] = useState(0);

  useEffect(() => {
    const onStorage = () => setUser(currentUser());
    window.addEventListener("arena-auth", onStorage);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener("arena-auth", onStorage);
      window.removeEventListener("storage", onStorage);
    };
  }, []);

  useEffect(() => {
    if (!user) {
      setCount(0);
      return;
    }
    let cancel = false;
    const load = () => {
      api
        .cart()
        .then((cart: Cart) => {
          if (!cancel) setCount(cart.items?.reduce((n, i) => n + i.quantity, 0) ?? 0);
        })
        .catch(() => {
          if (!cancel) setCount(0);
        });
    };
    load();
    const id = setInterval(load, 4000);
    return () => {
      cancel = true;
      clearInterval(id);
    };
  }, [user]);

  return (
    <header className="top">
      <div className="shell nav">
        <Link className="brand" to="/">
          <span className="logo">A</span>
          Arena Store
        </Link>
        <div className="spacer" />
        <nav className="nav-actions" aria-label="Principal">
          {user && (
            <Link className="pill" to="/compras">
              Minhas compras
            </Link>
          )}
          <Link className="pill" to="/carrinho">
            Carrinho
            {count > 0 && <span className="badge">{count}</span>}
          </Link>
          {user ? (
            <>
              <span className="user-name" title={user.name}>
                {user.name}
              </span>
              <button
                className="pill"
                onClick={() => {
                  clearSession();
                  setUser(null);
                  window.dispatchEvent(new Event("arena-auth"));
                }}
              >
                Sair
              </button>
            </>
          ) : (
            <Link className="pill" to="/entrar">
              Entrar
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}
