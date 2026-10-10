import { Link, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

export default function Layout() {
  const { session, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = () => {
    logout();
    navigate("/");
  };

  return (
    <>
      <header className="site-header">
        <div className="site-header-inner">
          <Link to="/" className="brand">
            Votaciones
          </Link>
          <div className="session">
            {session ? (
              <>
                <span className="session-user">Documento {session.document}</span>
                <button type="button" className="btn btn-ghost" onClick={handleLogout}>
                  Salir
                </button>
              </>
            ) : (
              <Link to="/ingresar" className="btn btn-secondary btn-small">
                Ingresar
              </Link>
            )}
          </div>
        </div>
      </header>
      <main className="page">
        <Outlet />
      </main>
    </>
  );
}
