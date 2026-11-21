import { Navigate, Route, Routes } from "react-router-dom";
import Header from "./components/Header";
import Home from "./pages/Home";
import Product from "./pages/Product";
import Cart from "./pages/Cart";
import Checkout from "./pages/Checkout";
import Order from "./pages/Order";
import Orders from "./pages/Orders";
import Login from "./pages/Login";

export default function App() {
  return (
    <>
      <Header />
      <main className="shell">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/produto/:id" element={<Product />} />
          <Route path="/carrinho" element={<Cart />} />
          <Route path="/checkout" element={<Checkout />} />
          <Route path="/pedido/:id" element={<Order />} />
          <Route path="/compras" element={<Orders />} />
          <Route path="/entrar" element={<Login />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </>
  );
}
