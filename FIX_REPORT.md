# Laporan perbaikan Meme Portal

## TKT-101 · Langganan kedaluwarsa

**Akar masalah.** Query yang dipakai aplikasi hanya memeriksa status `active`. ID langganan yang sudah melewati `ends_at` masih dianggap aktif sampai proses pembersihan berkala mengubah statusnya. Pembacaan status juga menyamarkan gangguan database sebagai akun tanpa langganan.

**Perbaikan.** PostgreSQL membandingkan `ends_at` dengan waktu saat query berjalan. Status Paket dan izin membuat meme memakai pemeriksaan yang sama. Gangguan database dikembalikan sebagai error.

**Bukti.** `TestStatusIgnoresExpiredSubscriptionBeforeCleanup` membuat langganan aktif di database uji, lalu memajukan batas akhirnya ke waktu saat ini tanpa menjalankan pembersihan. Sebelum patch, hasilnya `Subscribed: true`. Setelah patch, langganan masih terbaca saat valid dan langsung tidak aktif setelah melewati batas.

## TKT-102 · Status langganan tersimpan di cache publik

**Akar masalah.** Endpoint status berisi data akun dan mengirim `Cache-Control: public, max-age=86400`. Cache dapat menyimpan respons personal selama satu hari.

**Perbaikan.** Respons status selalu mengirim `Cache-Control: private, no-store`, termasuk saat autentikasi gagal. Daftar paket publik tetap tersedia. Saat merilis patch ke deployment yang memakai cache bersama, hapus entri status yang pernah tersimpan di cache tersebut.

**Bukti.** `TestSubscriptionStatusIsNotCacheable` memeriksa header pada respons akun uji. Sebelum patch, header membolehkan cache publik. Setelah patch, respons akun uji mengembalikan 200 dengan `private, no-store`; guest mendapat 401 dengan header yang sama.
