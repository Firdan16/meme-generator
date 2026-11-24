# Laporan perbaikan Meme Portal

## TKT-101 · Langganan kedaluwarsa

**Akar masalah.** Query yang dipakai aplikasi hanya memeriksa status `active`. ID langganan yang sudah melewati `ends_at` masih dianggap aktif sampai proses pembersihan berkala mengubah statusnya. Pembacaan status juga menyamarkan gangguan database sebagai akun tanpa langganan.

**Perbaikan.** PostgreSQL membandingkan `ends_at` dengan waktu saat query berjalan. Status Paket dan izin membuat meme memakai pemeriksaan yang sama. Gangguan database dikembalikan sebagai error.

**Bukti.** `TestStatusIgnoresExpiredSubscriptionBeforeCleanup` membuat langganan aktif di database uji, lalu memajukan batas akhirnya ke waktu saat ini tanpa menjalankan pembersihan. Sebelum patch, hasilnya `Subscribed: true`. Setelah patch, langganan masih terbaca saat valid dan langsung tidak aktif setelah melewati batas.

## TKT-102 · Status langganan tersimpan di cache publik

**Akar masalah.** Endpoint status berisi data akun dan mengirim `Cache-Control: public, max-age=86400`. Cache dapat menyimpan respons personal selama satu hari.

**Perbaikan.** Respons status selalu mengirim `Cache-Control: private, no-store`, termasuk saat autentikasi gagal. Daftar paket publik tetap tersedia. Saat merilis patch ke deployment yang memakai cache bersama, hapus entri status yang pernah tersimpan di cache tersebut.

**Bukti.** `TestSubscriptionStatusIsNotCacheable` memeriksa header pada respons akun uji. Sebelum patch, header membolehkan cache publik. Setelah patch, respons akun uji mengembalikan 200 dengan `private, no-store`; guest mendapat 401 dengan header yang sama.

## TKT-103 · Logout dan pengalihan halaman login

**Akar masalah.** Login memasang cookie pada path `/`, tetapi logout mencoba menghapus cookie pada path `/api/v1/auth`. Halaman login dan daftar mengalihkan pengguna hanya karena nama cookie masih terlihat, sekalipun sesi sudah tidak valid.

**Perbaikan.** Gunakan helper penghapusan cookie yang memakai path yang sama dengan saat pemasangan. Cookie menjadi `HttpOnly`. Halaman login dan daftar memakai hasil `/auth/me`. Klien menunggu refresh yang sedang berjalan sebelum meminta logout dan menggabungkan refresh yang berlangsung serentak.

**Bukti.** `TestClearAuthCookiesRemovesSessionAtRoot` memasang kedua cookie pada penyimpanan cookie standar, lalu menerapkan header logout. Sebelum patch, kedua cookie tertinggal. Setelah patch, penyimpanan kosong dan cookie login memiliki atribut `HttpOnly`.

## TKT-104 · Navbar dan kuota menampilkan status lama

**Akar masalah.** TanStack Query menganggap status segar selamanya. Masuk kembali ke halaman atau kembali ke tab dapat mempertahankan status lama. Perubahan jumlah meme tidak memperbarui data kuota. Cache HTTP pada TKT-102 juga bisa menyajikan status lama kepada permintaan ulang setelah pembayaran.

**Perbaikan.** Navbar dan halaman Paket memakai status yang sama dengan kunci cache per akun. Status dibaca ulang ketika layar dibuka, tab kembali aktif, masa berlaku habis, atau mutasi mengubah status maupun kuota. Konfirmasi pembayaran menunggu invalidasi status sebelum menampilkan hasil.

**Bukti.** `bun test` membuka observer dengan data status lama. Sebelum patch, observer diam (`idle`). Setelah patch, observer meminta status terbaru dan mendapatkan paket aktif.

## TKT-105 · Checkout ganda

**Akar masalah.** Klien membuat kunci baru tiap kali tombol ditekan. Server membaca kunci di luar transaksi pembuatan tagihan dan bisa membuat beberapa checkout pending untuk paket yang sama. Kunci lama juga dicari secara global. Di browser lintas origin, preflight tidak mengizinkan header `Idempotency-Key`.

**Perbaikan.** Halaman Paket menahan klik serentak dan menyimpan satu kunci per paket selama percobaan itu. Server mengunci baris akun dalam transaksi, mengembalikan tagihan pending yang sama untuk paket tersebut, dan mencatat pemetaan kunci ke tagihan per akun. Migrasi mempertahankan pemetaan kunci yang sudah tersimpan. API mengizinkan header idempotensi untuk origin tepercaya.

**Bukti.** `TestCheckoutReusesPendingPaymentAcrossRequests` mengirim satu checkout dan delapan percobaan ulang dari akun uji. Sebelum patch ada sembilan tagihan. Setelah patch semua panggilan memakai tagihan yang sama, termasuk percobaan ulang setelah pembayaran. `TestCheckoutKeysAreScopedToAccountAndPlan` dan `TestCheckoutPreflightAllowsIdempotencyKey` memeriksa batas akun, konflik paket, dan izin header. Browser sebelum patch menolak checkout pada preflight karena header tersebut.
