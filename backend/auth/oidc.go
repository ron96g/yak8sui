package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/ron96g/yak8sui/backend/config"
	"golang.org/x/oauth2"
)

type OIDC struct {
	verifier    *oidc.IDTokenVerifier
	oauth       oauth2.Config
	groupsClaim string
}

func NewOIDC(ctx context.Context, cfg *config.IdentityConfig) (*OIDC, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC issuer %q: %w", cfg.IssuerURL, err)
	}
	return &OIDC{verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}), groupsClaim: cfg.GroupsClaim, oauth: oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: cfg.RedirectURL, Scopes: []string{oidc.ScopeOpenID, "profile", "email", "groups"}}}, nil
}

func (o *OIDC) AuthorizationURL(state, challenge, nonce string) string {
	return o.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("code_challenge", challenge), oauth2.SetAuthURLParam("code_challenge_method", "S256"), oidc.Nonce(nonce))
}

func (o *OIDC) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	token, err := o.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, fmt.Errorf("OIDC token response did not contain id_token")
	}
	idToken, err := o.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("verify ID token: %w", err)
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("decode ID token claims: %w", err)
	}
	if stringClaim(claims, "nonce") != nonce {
		return Identity{}, fmt.Errorf("verified ID token nonce did not match sign-in attempt")
	}
	identity := Identity{Subject: stringClaim(claims, "sub"), Name: stringClaim(claims, "name"), Email: stringClaim(claims, "email")}
	groups, ok := claims[o.groupsClaim].([]any)
	if ok {
		for _, group := range groups {
			if value, ok := group.(string); ok {
				identity.Groups = append(identity.Groups, value)
			}
		}
	} else if typed, ok := claims[o.groupsClaim].([]string); ok {
		identity.Groups = typed
	}
	if identity.Subject == "" {
		return Identity{}, fmt.Errorf("verified ID token has no subject")
	}
	return identity, nil
}

func stringClaim(claims map[string]any, key string) string {
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}
	return value
}
