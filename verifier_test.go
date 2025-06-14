package verifier

import (
	"context"
	"testing"
	"time"
)

func TestVerify(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	conf := &Conf{
		ValidNS:       true,
		NonDisposable: true,
	}
	ok, err := Verify(ctx, "mail.gokulpm@gmail.com", conf)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Verify() returned false, expected true")
	}
}

func TestVerifyDisposableDomain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	conf := &Conf{
		ValidNS:       true,
		NonDisposable: true,
	}
	ok, err := Verify(ctx, "mail.gokulpm@0uxpgdvol9n.gq", conf)
	if err != ErrDisposableEmail {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Verify() returned true, expected false")
	}
}
