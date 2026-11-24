# Laporan perbaikan Meme Portal

Paket berhenti saat masa aktif habis, status dan kuota mengikuti penggunaan, dan pengguna hanya menerima satu tagihan pending untuk paket yang sama. Halaman pembayaran memberi tahu hasilnya. Pengguna tetap bisa membuat meme dengan alur yang sama dan melihat hasil miliknya di Studio.

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

## TKT-106 · Pembayaran gagal tanpa umpan balik

**Akar masalah.** Halaman checkout menelan error dari API dan tombol tetap bisa ditekan selama permintaan berjalan. API juga menolak konfirmasi yang sebelumnya sudah berhasil sehingga respons pertama yang hilang terlihat seperti pembayaran gagal.

**Perbaikan.** Halaman checkout menampilkan proses, pesan gagal yang dapat dibaca, dan konfirmasi berhasil. Tombol dinonaktifkan ketika permintaan berjalan atau sudah berhasil. Pengulangan untuk referensi yang sudah dibayar mengembalikan hasil awal. Konfirmasi dan aktivasi tetap satu transaksi, dengan penguncian akun untuk menjaga satu langganan aktif.

**Bukti.** Pada browser sebelum patch, referensi uji yang tidak tersedia mendapat HTTP 402 dan halaman tidak menampilkan pesan. `TestMockPaymentConfirmationCanBeSafelyRetried` menunjukkan permintaan ulang awalnya ditolak. Setelah patch, permintaan ulang mengembalikan pembayaran lunas dengan tanggal mulai dan berakhir yang sama.

## TKT-107 · Pembatasan akses meme ke pemilik

**Akar masalah.** Repository yang benar-benar dipakai aplikasi belum membatasi semua operasi per meme dengan `user_id`. Query baca juga memilih sepuluh kolom tetapi hanya memindai sembilan. Konten gambar memakai alamat langsung dari website Garage, di luar pemeriksaan pemilik.

**Perbaikan.** Baca, ubah hasil, dan hapus kini dibatasi oleh ID meme serta akun dari sesi. Meme yang tidak tersedia untuk sesi mendapat 404. Studio mengambil gambar melalui API yang memeriksa pemilik. Aplikasi membaca gambar lama yang didukung dari objek Garage yang sama. Tautan baru dibuat oleh API dan membutuhkan sesi pemilik.

**Bukti.** `TestMemeOwnerCanReadUpdateAndDelete` memakai meme milik akun uji. Sebelum patch, pembacaan oleh pemilik gagal karena jumlah kolom berbeda. Setelah patch, baca, ubah, dan hapus meme milik sendiri berhasil. Pemeriksaan kode menunjukkan setiap query operasi per-meme wajib memakai `user_id`; pengujian memakai data lokal buatan.

## TKT-108 · Objek non-gambar dan unggahan besar

**Akar masalah.** Browser menerima URL tulis langsung ke Garage. API meneruskan tipe dari klien dan tidak memeriksa ukuran atau isi file. Alur ini berlaku juga untuk hasil render.

**Perbaikan.** Unggahan melewati API supaya format, dimensi, dan ukuran bisa diperiksa sebelum file masuk Garage. URL PUT memakai izin bertanda tangan yang berlaku 15 menit untuk satu akun, satu meme, dan satu jenis gambar. API membatasi unggahan hingga 10 MiB, termasuk ketika panjang tidak diketahui di awal. Gambar dasar mendukung JPEG, PNG, GIF, dan WebP, maksimal 40 megapiksel. Hasil render wajib PNG dan harus ada di Garage sebelum meme bisa diselesaikan.

**Bukti.** `TestPrepareImageReturnsValidationEndpoint` memakai PNG uji. Sebelum patch, URL PUT mengarah langsung ke storage. Setelah patch, URL mengarah ke API. `TestImageReaderValidatesFormatAndBodyLimit` menerima PNG valid serta menolak fixture teks biasa, header PNG terpotong, dan stream yang melebihi batas.

## TKT-109 · Kuota terlampaui saat banyak permintaan

**Akar masalah.** Pemeriksaan kuota dan pembuatan draft berjalan terpisah. Beberapa permintaan bisa membaca jumlah yang sama sebelum salah satunya menyimpan draft. Hitungan memakai rentang yang bergeser dari hari ini, sehingga bisa memasukkan meme dari langganan sebelumnya. Pemeriksaan yang sama juga keliru memblokir render ketika draft baru saja memakai slot terakhir.

**Perbaikan.** PostgreSQL mengunci baris akun, memeriksa masa aktif, menghitung slot terpakai, dan menyimpan draft dalam satu transaksi. Hitungan memakai waktu mulai dan berakhir langganan aktif. Draft yang sudah memakai satu slot bisa diselesaikan selama sesi pemilik dan langganannya masih berlaku. Paket tanpa batas tetap berfungsi. Sisa kuota yang ditampilkan minimal nol.

**Bukti.** Sebelum patch, `TestStatusUsesActualSubscriptionStartForQuota` menghitung 2 padahal hanya 1 meme dibuat dalam periode aktif, dan `TestLastReservedSlotCanRequestResultUpload` mendapat 403. Sesudah patch keduanya lulus. `TestConcurrentReservationsRespectThePlanLimit` dijalankan 10 kali dengan race detector. Dalam setiap putaran, paket 10 slot menerima dan menyimpan tepat 10 dari 14 permintaan serentak. Di browser, slot kesepuluh menghasilkan PNG yang tampil di Studio, lalu halaman Paket menampilkan 10/10.

## Verifikasi lokal

Jalankan dari checkout ini dengan PostgreSQL, Redis, dan Garage sesuai README. Terapkan migrasi sebelum menjalankan tes integrasi. Pakai database kosong khusus pengujian. Tes menambah dan menghapus akun uji.

```sh
export TEST_DATABASE_URL='postgres://meme:meme@localhost:5432/meme_portal?sslmode=disable'
make migrate-up DATABASE_URL="$TEST_DATABASE_URL"
(cd api && go test -race ./... -count=1)
(cd api && go vet ./...)
(cd web && bun test && bun run check --incremental false && bun run lint && bun run build)
```

### Ulangi tes untuk tiket tertentu

Bagian **Bukti** di tiap tiket merekam hasil yang diamati sebelum dan sesudah perbaikan. Perintah berikut dijalankan pada kode hasil perbaikan. Jalankan perintah Go dari `api/` dan perintah Bun dari `web/`.

| Tiket | Tes | Perintah |
| --- | --- | --- |
| TKT-101 | Paket habis sebelum pembersihan berkala | `go test ./internal/modules/subscription -run TestStatusIgnoresExpiredSubscriptionBeforeCleanup -count=1` |
| TKT-102 | Status akun dan respons guest diberi aturan cache yang sama | `go test ./internal/modules/subscription -run TestSubscriptionStatusIsNotCacheable -count=1` |
| TKT-103 | Logout menghapus kedua cookie sesi | `go test ./internal/modules/auth -run TestClearAuthCookiesRemovesSessionAtRoot -count=1` |
| TKT-104 | Status paket diperbarui saat layar dibuka kembali | `bun test tests/subscription-status.test.mjs` |
| TKT-105 | Percobaan ulang checkout, cakupan kunci per akun dan paket, serta izin header browser | `go test ./internal/modules/subscription ./internal/middleware -run TestCheckout -count=1` |
| TKT-106 | Konfirmasi ulang mengembalikan pembayaran dan masa aktif yang sama | `go test ./internal/modules/subscription -run TestMockPaymentConfirmationCanBeSafelyRetried -count=1` |
| TKT-107 | Pemilik dapat membaca, memperbarui, dan menghapus meme sendiri | `go test ./internal/modules/meme -run TestMemeOwnerCanReadUpdateAndDelete -count=1` |
| TKT-108 | Rute unggah, format gambar, ukuran, dan operasi pemilik | `go test ./internal/modules/meme -count=1` |
| TKT-109 | Periode yang dihitung, render slot terakhir, dan permintaan serentak | `go test -race ./internal/server/v1 -count=1` dan `go test ./internal/modules/subscription -run TestStatusUsesActualSubscriptionStartForQuota -count=1` |

### Coba alur utama di browser

1. Daftar dengan akun uji, pilih **Premium 1 Hari**, lalu konfirmasi pembayaran simulasi. Buka info akun di navbar dan lihat status serta kuota di halaman **Paket**. Paket aktif dan sisa 10 slot langsung tampil.
2. Buka **Studio**, pilih PNG atau JPEG, isi judul serta teks, lalu klik **Render & Simpan**. Gambar muncul di **Meme Saya** dan sisa kuota turun menjadi 9.
3. Klik **Keluar**, buka **Masuk**, lalu login lagi. Halaman login tetap terbuka sampai formulir dikirim. Setelah login, meme dan kuota tetap tersedia.

### Hasil pengujian

- Uji Go dengan race detector, pemeriksaan Go, tes Bun, TypeScript, lint, dan build produksi Next.js lulus pada pengujian lokal.
- Upgrade dari skema versi 5 ke 6 mempertahankan pemetaan kunci dari tagihan yang sudah tersimpan.
- Browser dengan PostgreSQL, Redis, dan Garage menjalankan pendaftaran, checkout simulasi, tampilan error pembayaran, pembaruan paket di navbar, render canvas, penyimpanan PNG, tampilan gambar, logout, dan login ulang. Aplikasi melaporkan status baru tanpa hard refresh pada alur pembayaran dan render.
- Pemeriksaan portal penilai belum dijalankan. Bukti pada laporan ini berasal dari kode, tes, dan data buatan di lingkungan lokal.
