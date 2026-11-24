import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, logoutSession, unwrap } from "@/lib/api";

export interface AuthUser {
  id: string;
  email: string;
  name: string;
}

export interface AuthResponse {
  user: AuthUser;
}

interface AuthInput {
  email: string;
  password: string;
  name?: string;
}

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: async ({ signal }) => unwrap<AuthUser>(await api.get("/api/v1/auth/me", { signal })),
    retry: false,
  });
}

export function useRegister() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: AuthInput) =>
      unwrap<AuthResponse>(await api.post("/api/v1/auth/register", input)),
    onSuccess: async ({ user }) => {
      await qc.cancelQueries({ queryKey: ["me"] });
      qc.setQueryData(["me"], user);
    },
  });
}

export function useLogin() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: Omit<AuthInput, "name">) =>
      unwrap<AuthResponse>(await api.post("/api/v1/auth/login", input)),
    onSuccess: async ({ user }) => {
      await qc.cancelQueries({ queryKey: ["me"] });
      qc.setQueryData(["me"], user);
    },
  });
}

export function useLogout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: logoutSession,
    onSuccess: () => {
      qc.clear();
      window.location.href = "/";
    },
  });
}
