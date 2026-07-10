package clientserver

import (
	"context"
	"errors"
	"net/url"
)

type sourceURLPolicy interface {
	Validate(context.Context, clientIdentity, *url.URL) error
}

type allowSourceURLPolicy struct{}

func (allowSourceURLPolicy) Validate(_ context.Context, _ clientIdentity, target *url.URL) error {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return errors.New("source URL must use HTTP or HTTPS")
	}
	return nil
}
