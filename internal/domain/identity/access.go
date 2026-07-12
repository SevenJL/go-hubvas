package identity

// AccessIdentity is the server-verified identity carried by an access token.
// SecurityVersion binds a token to the current account credential state so a
// password change or administrative suspension can invalidate it immediately.
type AccessIdentity struct {
	UserID          UserID
	SecurityVersion int64
}
