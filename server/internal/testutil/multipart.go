package testutil

import (
	"io"
	"mime/multipart"
)

func newMultipart(w io.Writer) *multipart.Writer {
	return multipart.NewWriter(w)
}
