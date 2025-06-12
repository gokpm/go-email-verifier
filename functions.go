package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

func New(conf *ConfVerifier) (*verifier, error) {
	v := &verifier{
		conf: conf,
		mu:   &sync.RWMutex{},
		tk:   time.NewTicker(1 * time.Hour),
	}
	v.ctx, v.cancel = context.WithCancel(context.TODO())
	if conf.CheckDisposableDomains {
		disposableDomains, err := getDisposableDomains()
		if err != nil {
			return nil, err
		}
		v.disposableDomains = disposableDomains
		go v.loop()
	}
	return v, nil
}

func getDisposableDomains() (map[string]struct{}, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
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
