---
title: "Lesson Learned: Nginx Reverse Proxy & Keepalive Connection Bug"
date: 2026-07-10
tags: [nginx, reverse-proxy, http, http-keepalive, netbird, bug-fix]
---

# Lesson Learned: Nginx Reverse Proxy & Keepalive Connection Bug

**Konteks:** Debugging error intermiten `ERR_HTTP2_PROTOCOL_ERROR` / `upstream sent no valid HTTP/1.0 header` / `Connection reset by peer` pada deployment NetBird self-hosted (combined container) di belakang nginx reverse proxy.

---

## 1. Gejala Awal

- Browser sesekali menampilkan `ERR_HTTP2_PROTOCOL_ERROR` saat mengakses dashboard NetBird, terutama pada endpoint `/api/*` dan `/oauth2/*`.
- Error terjadi **intermiten** — tidak konsisten, dan biasanya hilang setelah refresh/retry.
- Log nginx menunjukkan:
  ```
  upstream sent no valid HTTP/1.0 header while reading response header from upstream
  readv() failed (104: Connection reset by peer) while reading upstream
  ```
- Endpoint yang mengalami: `/api/users/current`, `/oauth2/mfa/totp`.
- Endpoint gRPC (`/signalexchange.SignalExchange/`, `/management.ManagementService/`) dan WebSocket (`/relay`, `/ws-proxy/*`) **tidak pernah bermasalah**.

---

## 2. Proses Investigasi (Ringkas)

| Langkah | Hasil | Kesimpulan |
|---|---|---|
| Cek log aplikasi backend (`netbird-server`) di rentang waktu error | Tidak ada panic/error yang relevan | Backend sehat, bukan bug aplikasi |
| Curl 500x langsung ke backend (`192.168.16.5:81`), skip nginx | 100% sukses, konsisten | Backend & jaringan Docker sehat |
| Curl lewat nginx dari server yang sama, request identik, diulang | Percobaan 1: **gagal** (`Empty reply from server`)<br>Percobaan 2: **sukses** | Masalah ada di hop **nginx → backend**, sifatnya "sekali gagal, retry sukses" |

Pola "gagal sekali lalu sukses saat diulang persis" adalah ciri khas bug **stale keepalive connection reuse**.

---

## 3. Root Cause

Konfigurasi nginx punya:
```nginx
upstream netbird-server {
    server 192.168.16.5:81;
    keepalive 10;   # <- menyimpan koneksi untuk di-reuse
}

location /oauth2/ {
    proxy_pass http://netbird-server;
    # tidak ada proxy_http_version 1.1;
    # tidak ada proxy_set_header Connection "";
}
```

**Masalahnya:**
1. `keepalive 10;` di upstream block memberi tahu nginx: "simpan sampai 10 koneksi ke backend untuk dipakai ulang".
2. Tapi fitur reuse-koneksi ini **hanya berfungsi benar di HTTP/1.1**. Tanpa `proxy_http_version 1.1;`, nginx default memakai HTTP/1.0 ke backend — dan protokol 1.0 punya default `Connection: close` (koneksi ditutup setiap selesai satu request).
3. Bahkan setelah `proxy_http_version 1.1;` ditambahkan sendirian, nginx **tetap** mengirim header `Connection: close` ke backend secara default (ini bukan otomatis hilang hanya karena naik versi protokol) — kecuali di-override manual dengan `proxy_set_header Connection "";`.
4. Akibatnya: backend menutup koneksi sesuai instruksi header, tapi nginx tetap menyimpan soket itu di pool `keepalive` untuk dipakai ulang. Request berikutnya mengambil koneksi "bekas" yang sebenarnya sudah mati di sisi backend → nginx membaca EOF/connection reset saat mencoba baca response → muncul error yang kita lihat.

---

## 4. Konsep Kunci yang Dipelajari

### a. Ada 3 koneksi terpisah dalam satu request lewat reverse proxy
```
[Browser] --HTTP/2 (TLS)--> [nginx] --HTTP/1.x (plain, terpisah!)--> [backend]
```
Protokol di hop client↔nginx **tidak ada hubungannya** dengan protokol di hop nginx↔backend. Masing-masing diatur independen (`http2 on;` untuk sisi client, `proxy_http_version` untuk sisi backend).

### b. Default nginx ke upstream adalah HTTP/1.0
Ini quirk lama nginx — walaupun semua servis modern (termasuk Go `net/http`) mendukung 1.1 dengan baik, nginx tidak otomatis memakainya. Harus di-set eksplisit dengan `proxy_http_version 1.1;`.

### c. `keepalive` di upstream block butuh 3 hal sekaligus
Ketiganya harus ada bersamaan, tidak bisa sebagian:
```nginx
upstream backend {
    server ...;
    keepalive 10;               # 1. aktifkan pool
}
location /path/ {
    proxy_pass http://backend;
    proxy_http_version 1.1;     # 2. pakai HTTP/1.1
    proxy_set_header Connection "";  # 3. jangan kirim Connection: close
}
```
Kalau salah satu hilang → entah keepalive tidak efektif (aman tapi tidak ada manfaat performa), atau — kasus terburuk — bug seperti yang kita alami (koneksi basi ter-reuse).

### d. `proxy_set_header Connection ""` ≠ tidak menulis apapun
- Tidak menulis directive sama sekali → nginx pakai default `Connection: close`.
- Menulis `Connection "";` (string kosong) → header benar-benar tidak dikirim, sehingga tidak ada instruksi "tutup koneksi" ke backend.

### e. WebSocket & gRPC punya mekanisme keepalive yang berbeda, tidak butuh fix yang sama
| Jenis | Directive | Kenapa aman tanpa `Connection ""` |
|---|---|---|
| WebSocket (`/relay`, `/ws-proxy/*`) | `Connection "Upgrade"` | Koneksi jadi *long-lived per sesi*, tidak pernah kembali ke pool untuk di-reuse |
| gRPC (`.../SignalExchange/`, `.../ManagementService/`) | `grpc_pass` (bukan `proxy_pass`) | Module berbeda (`ngx_http_grpc_module`), native HTTP/2, lifecycle koneksi ditangani berbeda |
| HTTP biasa (`/api`, `/oauth2`) | `proxy_pass` + `proxy_http_version 1.1` + `Connection ""` | Ini yang butuh fix — request-response pendek, rawan reuse koneksi basi |

### f. Dokumentasi resmi NetBird sengaja **tidak** memasang `keepalive` di upstream `management`
```nginx
upstream dashboard {
    server 127.0.0.1:8011;
    keepalive 10;   # hanya di dashboard (static assets, sering diakses)
}
upstream management {
    server 127.0.0.1:8012;
    # sengaja tanpa keepalive — request /api & /oauth2 tidak reuse koneksi
}
```
Ini kemungkinan pilihan desain yang **menghindari** kelas bug ini sama sekali, dengan trade-off overhead kecil (buka koneksi baru tiap request) yang sepadan untuk endpoint yang tidak high-frequency.

---

## 5. Perbaikan yang Diterapkan

**Opsi yang dipilih — ikuti dokumentasi resmi:** hapus `keepalive` dari upstream `netbird-server`, sehingga tidak ada connection pooling ke backend untuk `/api` dan `/oauth2` sama sekali.

```nginx
upstream netbird-server {
    server 192.168.16.5:81;
    # keepalive dihapus — samakan dengan dokumentasi resmi NetBird
}
```

*(Alternatif jika tetap ingin keepalive demi performa: pertahankan `keepalive 10;` tapi lengkapi setiap location HTTP biasa dengan `proxy_http_version 1.1;` dan `proxy_set_header Connection "";`.)*

---

## 6. Checklist Umum untuk Setup Reverse Proxy Serupa di Masa Depan

- [ ] Jangan asal copy snippet nginx dari sumber lama/tidak resmi — cek dulu apakah topologi (combined container vs multi-container) cocok dengan versi & arsitektur yang dipakai.
- [ ] Kalau pakai `keepalive N;` di upstream block, **selalu** pasangkan dengan `proxy_http_version 1.1;` dan `proxy_set_header Connection "";` di semua location yang memakai upstream itu.
- [ ] Jangan pasang `keepalive` di upstream backend kalau tidak benar-benar butuh (default aman = tanpa keepalive, kecuali ada bukti kebutuhan performa nyata).
- [ ] Selalu set `proxy_set_header Host $host;` secara eksplisit — default nginx mengirim nama upstream/`$proxy_host`, bukan domain asli client.
- [ ] Endpoint WebSocket/gRPC punya aturan keepalive sendiri (`Connection: Upgrade`, `grpc_pass`) — jangan disamakan dengan endpoint HTTP biasa.
- [ ] Kalau menemukan error intermiten "gagal sekali, sukses saat retry dengan request identik" → curigai stale keepalive connection duluan sebelum curiga ke bug aplikasi/jaringan.
- [ ] Cara isolasi cepat: bandingkan hasil curl **langsung ke backend** vs **lewat nginx**, diulang banyak kali. Kalau backend selalu sukses tapi lewat nginx kadang gagal → masalah di layer nginx↔backend, bukan di aplikasi.