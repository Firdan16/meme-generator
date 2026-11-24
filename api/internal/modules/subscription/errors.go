package subscription

import "errors"

var (
	ErrPlanNotFound        = errors.New("paket langganan tidak ditemukan")
	ErrAlreadySubscribed   = errors.New("masih ada langganan aktif")
	ErrCheckoutKeyConflict = errors.New("Idempotency-Key sudah dipakai untuk paket lain")
	ErrNotSubscribed       = errors.New("belum berlangganan")
	ErrPaymentFailed       = errors.New("pembayaran gagal")
	ErrNotFound            = errors.New("langganan tidak ditemukan")
)
