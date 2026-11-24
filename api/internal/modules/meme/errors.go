package meme

import "errors"

var (
	ErrNotFound          = errors.New("meme tidak ditemukan")
	ErrInvalidFile       = errors.New("file gambar tidak valid")
	ErrFileTooLarge      = errors.New("ukuran gambar maksimum 10 MiB")
	ErrUploadGrant       = errors.New("izin unggah tidak valid atau kedaluwarsa")
	ErrUploadIncomplete  = errors.New("unggah hasil render sebelum menyimpan meme")
	ErrNotSubscribedGate = errors.New("butuh langganan aktif untuk membuat meme")
)
