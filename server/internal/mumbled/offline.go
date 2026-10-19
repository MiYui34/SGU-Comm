package mumbled

// offlineAccess answers for a registered user who is not connected (push
// notifications): groups and roles by user id count, a certificate hash
// or access tokens do not. It is in no channel.
func (s *Server) offlineAccess(user, channel uint32, may func(u *User) bool) bool {
	if _, ok := s.UserNames([]uint32{user})[user]; !ok {
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
