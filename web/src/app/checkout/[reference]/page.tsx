"use client";

// Halaman simulasi pembayaran (mock payment gateway).
// Di produksi halaman ini digantikan halaman payment gateway sungguhan
// yang memanggil webhook /confirm dengan verifikasi signature.

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useConfirmPayment } from "@/lib/queries";
import { apiErrorMessage } from "@/lib/api";

export default function CheckoutPage() {
  const { reference } = useParams<{ reference: string }>();
  const router = useRouter();
  const confirm = useConfirmPayment();
  const submitting = useRef(false);

  useEffect(() => {
    if (!confirm.isSuccess) return;
    const timer = setTimeout(() => router.replace("/pricing"), 1200);
    return () => clearTimeout(timer);
  }, [confirm.isSuccess, router]);

  const onPay = () => {
    if (submitting.current || confirm.isSuccess) return;
    submitting.current = true;
    confirm.mutate(reference, {
      onSettled: () => {
        submitting.current = false;
      },
    });
  };

  const subscription = confirm.data?.subscription;
  const active = subscription?.status === "active" && subscription.endsAt
    && Date.parse(subscription.endsAt) > Date.now();

  return (
    <main className="mx-auto flex min-h-[70vh] max-w-md items-center p-8">
      <Card className="w-full">
        <CardHeader>
          <CardTitle>Simulasi Pembayaran</CardTitle>
          <CardDescription>
            Payment gateway mock — tidak ada uang sungguhan.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">Referensi</span>
            <span className="font-mono">{reference}</span>
          </div>
          {confirm.isSuccess && (
            <p role="status" className="text-center text-sm font-medium text-green-600">
              Pembayaran berhasil! {active && "Langganan Anda aktif. "}Mengalihkan…
            </p>
          )}
          {confirm.isError && (
            <p role="alert" className="text-sm text-destructive">
              {apiErrorMessage(confirm.error, "Pembayaran belum dapat dikonfirmasi. Coba lagi.")}
            </p>
          )}
        </CardContent>
        <CardFooter className="flex gap-2">
          <Button className="flex-1" onClick={onPay} disabled={confirm.isPending || confirm.isSuccess}>
            {confirm.isSuccess ? "Lunas" : confirm.isPending ? "Memproses…" : "Bayar Sekarang (Simulasi)"}
          </Button>
          <Button asChild variant="ghost">
            <Link href="/pricing">Batal</Link>
          </Button>
        </CardFooter>
      </Card>
    </main>
  );
}
