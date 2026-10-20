package mumbled

import (
	"log/slog"
	"net/netip"
)

// offlineAccess answers for a registered user who is not connected (push
// notifications): groups and roles by user id count, a certificate hash
// or access tokens do not. It is in no channel. A banned user has none.
func (s *Server) offlineAccess(user, channel uint32, may func(u *User) bool) bool {
	if r := s.registered(user); r == nil || s.certBanned(r) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	u := &User{Registered: true, UserID: user, Channel: NoChannel}
	return s.state.Channels[channel] != nil && may(u)
}

// UserSees is Sees for a registered user by id.
func (s *Server) UserSees(user, channel uint32) bool {
	return s.offlineAccess(user, channel, func(u *User) bool { return s.state.Sees(u, channel) })
}

// UserMayRead is MayRead for a registered user by id.
func (s *Server) UserMayRead(user, channel uint32) bool {
	return s.offlineAccess(user, channel, func(u *User) bool { return s.state.Has(u, channel, Enter) })
}

// UserBanned: a registered user's certificate is banned. An address ban
// cannot apply to a user who is not connected.
func (s *Server) UserBanned(user uint32) bool {
	r := s.registered(user)
	return r != nil && s.certBanned(r)
}

func (s *Server) registered(user uint32) *Registered {
	if s.cfg.DB == nil {
		return nil
	}
	r, err := s.cfg.DB.UserByID(user)
	if err != nil {
		slog.Error("looking up a user", "err", err)
	}
	return r
}

func (s *Server) certBanned(r *Registered) bool {
	return r.CertHash != "" && s.banned(netip.Addr{}, r.CertHash) != nil
}
