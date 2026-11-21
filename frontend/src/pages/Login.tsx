import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { api, saveSession } from "../api";

export default function Login() {
  const nav = useNavigate();
  const loc = useLocation();
  const from = typeof loc.state === "object" && loc.state && "from" in loc.state ? String((loc.state as { from: string }).from) : "/";
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function enter() {
    setBusy(true);
    setErr("");
    try {
      const res = await api.guest();
      saveSession(res.token, res.user);
      window.dispatchEvent(new Event("arena-auth"));
      nav(from);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="hero">
      <h1>Entrar como visitante</h1>
      <p>
        Cada clique cria um usuário fictício único na base. Sem senha, sem e-mail real — só o
        suficiente para testar o fluxo durable de carrinho e pagamento.
      </p>
      {err && <div className="banner bad">{err}</div>}
      <p>
        <button className="btn" disabled={busy} onClick={enter}>
          Gerar visitante e entrar
        </button>
      </p>
    </section>
  );
}
