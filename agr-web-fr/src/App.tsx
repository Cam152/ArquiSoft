import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import RequireAuth from "./components/RequireAuth";
import { AuthProvider } from "./context/AuthContext";
import ElectionsPage from "./pages/ElectionsPage";
import LoginPage from "./pages/LoginPage";
import ResultsPage from "./pages/ResultsPage";
import VotePage from "./pages/VotePage";

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<ElectionsPage />} />
            <Route path="ingresar" element={<LoginPage />} />
            <Route
              path="elecciones/:id/votar"
              element={
                <RequireAuth>
                  <VotePage />
                </RequireAuth>
              }
            />
            <Route path="elecciones/:id/resultados" element={<ResultsPage />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
