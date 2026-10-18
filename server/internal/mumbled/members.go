package mumbled

import "log/slog"

// Registered users for the Kallisti member list, which shows them while
// they are offline too (see Config.OnUserChanged).

// Users: the names of the registered users but SuperUser, by id.
func (s *Server) Users() map[uint32]string {
	out := map[uint32]string{}
	if s.cfg.DB == nil {
		return out
	}
	users, err := s.cfg.DB.Users()
	if err != nil {
		slog.Error("listing users", "err", err)
	}
	for _, r := range users {
		out[r.ID] = r.Name
	}
	return out
}

// Bio is a registered user's saved comment ("": none, or no such user).
func (s *Server) Bio(id uint32) string {
	if s.cfg.DB == nil {
		return ""
	}
	comment, err := s.cfg.DB.comment(id)
	if err != nil {
		slog.Error("reading a comment", "user", id, "err", err)
	}
	return comment
}

// userChanged tells the Kallisti server about a registration made or
// renamed (name), or removed (""), in order and outside the server lock.
func (s *Server) userChanged(id uint32, name string) {
	if f := s.cfg.OnUserChanged; f != nil {
		s.events.add(func() { f(id, name) })
	}
}
