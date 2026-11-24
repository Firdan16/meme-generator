import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { api, unwrap } from "@/lib/api";
import { useMe } from "@/lib/auth";
import { useRef } from "react";
import type {
  CheckoutResponse,
  Meme,
  Plan,
  SubscriptionStatus,
  UploadResponse,
} from "@/lib/types";

export const queryKeys = {
  plans: ["plans"] as const,
  subscription: ["subscription"] as const,
  memes: (limit: number, offset: number) => ["memes", limit, offset] as const,
};

export function usePlans() {
  return useQuery({
    queryKey: queryKeys.plans,
    queryFn: async () =>
      unwrap<Plan[]>(await api.get("/api/v1/subscription/plans")) ?? [],
  });
}

export function subscriptionStatusOptions(userID: string | undefined) {
  return queryOptions({
    queryKey: [...queryKeys.subscription, userID] as const,
    queryFn: async ({ signal }) =>
      unwrap<SubscriptionStatus>(await api.get("/api/v1/subscription/status", { signal })),
    enabled: Boolean(userID),
    staleTime: 0,
    refetchOnWindowFocus: true,
  });
}

export function useSubscriptionStatus() {
  const me = useMe();
  return useQuery(subscriptionStatusOptions(me.data?.id));
}

export function useCheckout() {
  const keys = useRef(new Map<string, string>());
  return useMutation({
    mutationFn: async (planCode: string) => {
      let key = keys.current.get(planCode);
      if (!key) {
        key = crypto.randomUUID();
        keys.current.set(planCode, key);
      }
      return unwrap<CheckoutResponse>(
        await api.post("/api/v1/subscription/checkout", { planCode }, {
          headers: { "Idempotency-Key": key },
        }),
      );
    },
  });
}

export function useConfirmPayment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (reference: string) =>
      unwrap<Pick<CheckoutResponse, "subscription" | "payment">>(
        await api.post(
          `/api/v1/subscription/payments/${reference}/confirm`,
        ),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.subscription }),
  });
}

export function useMemes(limit = 20, offset = 0) {
  return useQuery({
    queryKey: queryKeys.memes(limit, offset),
    queryFn: async () =>
      unwrap<{ items: Meme[]; total: number }>(
        await api.get("/api/v1/memes", { params: { limit, offset } }),
      ),
  });
}

export function usePrepareUpload() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { title: string; filename: string; contentType: string; size: number }) =>
      unwrap<UploadResponse>(
        await api.post("/api/v1/memes/upload", input),
      ),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.subscription });
    },
  });
}

export function useCompleteMeme() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; resultUrl: string }) =>
      unwrap<Meme>(
        await api.post(`/api/v1/memes/${input.id}/complete`, {
          resultUrl: input.resultUrl,
        }),
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["memes"] }),
  });
}

export function useDeleteMeme() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => api.delete(`/api/v1/memes/${id}`),
    onSuccess: () => Promise.all([
      qc.invalidateQueries({ queryKey: ["memes"] }),
      qc.invalidateQueries({ queryKey: queryKeys.subscription }),
    ]),
  });
}
