package server

import (
	"errors"
	"io"
	"net/http"
	"time"
)

func writeSSE(w http.ResponseWriter, deadline time.Duration, payload string) error {
	controller := http.NewResponseController(w)
	if deadline > 0 {
		if err := controller.SetWriteDeadline(time.Now().Add(deadline)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return err
		}
	}
	if _, err := io.WriteString(w, payload); err != nil {
		return err
	}
	return controller.Flush()
}
