package sso

import "fmt"

type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

func (r *Registry) Register(name string, p Provider) {
	r.providers[name] = p
}

func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown sso provider: %s", name)
	}
	return p, nil
}
