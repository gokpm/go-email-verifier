package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

func New(ctx context.Context, c *Conf) (*verifier, error) {
	v := &verifier{
		disposableDomains: map[string]struct{}{},
	}
	if c.DisposableCheck {
		disposableDomains, err := getDisposableDomainsMap(ctx)
		if err != nil {
			return nil, err
		}
		v.disposableDomains = disposableDomains
	}
	return v, nil
}

func getDisposableDomainsMap(ctx context.Context) (map[string]struct{}, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, disposableDomainsURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	bytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	domains := []string{}
	err = json.Unmarshal(bytes, &domains)
	if err != nil {
		return nil, err
	}
	disposableDomains := map[string]struct{}{}
	for _, domain := range domains {
		disposableDomains[domain] = struct{}{}
	}
	return disposableDomains, nil
}
