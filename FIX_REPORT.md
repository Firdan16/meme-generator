# Laporan perbaikan Meme Portal

## TKT-101 · Langganan kedaluwarsa

**Akar masalah.** Query yang dipakai aplikasi hanya memeriksa status `active`. ID langganan yang sudah melewati `ends_at` masih dianggap aktif sampai proses pembersihan berkala mengubah statusnya. Pembacaan status juga menyamarkan gangguan database sebagai akun tanpa langganan.

**Perbaikan.** PostgreSQL membandingkan `ends_at` dengan waktu saat query berjalan. Status Paket dan izin membuat meme memakai pemeriksaan yang sama. Gangguan database dikembalikan sebagai error.

**Bukti.** `TestStatusIgnoresExpiredSubscriptionBeforeCleanup` membuat langganan aktif di database uji, lalu memajukan batas akhirnya ke waktu saat ini tanpa menjalankan pembersihan. Sebelum patch, hasilnya `Subscribed: true`. Setelah patch, langganan masih terbaca saat valid dan langsung tidak aktif setelah melewati batas.
