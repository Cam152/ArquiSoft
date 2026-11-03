"use server";

// Server Action del login. El navegador envía el formulario al servidor de Next.js
// (mismo origen) y este llama a POST /auth/login del gateway por la red interna.
// Así el navegador solo habla con el gateway para votar y para el SSE.

import { ApiError } from "@/lib/api/errors";
import { login } from "@/lib/api/server";

export type LoginState =
  | { status: "idle" }
  | { status: "error"; message: string }
  | { status: "success"; token: string; expiresIn: number; document: string };

const MAX_LENGTH = 64;

export async function loginAction(_previous: LoginState, formData: FormData): Promise<LoginState> {
  const document = String(formData.get("document") ?? "").trim();
  const password = String(formData.get("password") ?? "");

  if (!document || !password) {
    return { status: "error", message: "Ingresa tu documento y tu clave." };
  }
  if (document.length > MAX_LENGTH || password.length > MAX_LENGTH) {
    return { status: "error", message: "Documento o clave incorrectos." };
  }

  try {
    const response = await login(document, password);
    return {
      status: "success",
      token: response.access_token,
      expiresIn: response.expires_in,
      document,
    };
  } catch (error) {
    if (error instanceof ApiError) return { status: "error", message: error.message };
    console.error(error);
    return { status: "error", message: "No se pudo iniciar sesión. Intenta de nuevo." };
  }
}
