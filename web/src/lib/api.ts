import axios, { type InternalAxiosRequestConfig } from "axios";

export const api = axios.create({
  baseURL: process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  withCredentials: true, // kirim cookie access/refresh token
  headers: { "Content-Type": "application/json" },
});

const refreshPath = "/api/v1/auth/refresh";
const logoutPath = "/api/v1/auth/logout";
const publicAuthPaths = new Set([refreshPath, "/api/v1/auth/login", "/api/v1/auth/register"]);
let refreshRequest: Promise<unknown> | undefined;
let loggingOut = false;

api.interceptors.response.use(
  (res) => res,
  async (error) => {
    const original = error.config as (InternalAxiosRequestConfig & { _retried?: boolean }) | undefined;
    if (
      error.response?.status === 401 &&
      original &&
      !original._retried &&
      !original.signal?.aborted &&
      !publicAuthPaths.has(original.url ?? "") &&
      (!loggingOut || original.url === logoutPath)
    ) {
      original._retried = true;
      refreshRequest ??= api.post(refreshPath).finally(() => {
        refreshRequest = undefined;
      });
      try {
        await refreshRequest;
        return api(original);
      } catch {
        // refresh gagal → biarkan error 401 asli naik.
      }
    }
    return Promise.reject(error);
  },
);

export async function logoutSession() {
  loggingOut = true;
  try {
    await refreshRequest?.catch(() => undefined);
    await api.post(logoutPath);
  } finally {
    loggingOut = false;
  }
}

// Ambil field `data` dari envelope backend; lempar error bila kosong
// supaya mutasi/query memasuki state error, bukan mengembalikan undefined.
export function unwrap<T>(envelope: { data?: { data?: T } }): T {
  const data = envelope.data?.data;
  if (data === undefined || data === null) {
    throw new Error("respons server tidak berisi data");
  }
  return data;
}

// Ambil pesan error dari envelope backend (axios error).
export function apiErrorMessage(err: unknown, fallback: string): string {
  const message = (
    err as { response?: { data?: { message?: unknown } } } | undefined
  )?.response?.data?.message;
  return typeof message === "string" && message ? message : fallback;
}
