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

// Pictures: the registered users with a saved avatar that Avatars serves
// (see Config.OnPictureChanged).
func (s *Server) Pictures() map[uint32]bool {
	if s.cfg.DB == nil {
		return map[uint32]bool{}
	}
	out, err := s.cfg.DB.Pictures()
	if err != nil {
		slog.Error("listing pictures", "err", err)
		return map[uint32]bool{}
	}
	return out
}

// OnlineUser is a connected registered user (see Online).
type OnlineUser struct {
	Name    string
	Session uint32 // its voice session, else its oldest (see textonly.go)
}

// Online: the connected registered users but SuperUser, by id, in any
// channel or text-only (Kallisti lists them online also to users who may
// not see it).
func (s *Server) Online() map[uint32]OnlineUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[uint32]OnlineUser{}
	for _, u := range s.state.Users {
		if !u.Registered || u.UserID == 0 {
			continue
		}
		if o, ok := out[u.UserID]; !ok || s.state.Users[o.Session].textOnly() && (!u.textOnly() || u.Session < o.Session) {
			out[u.UserID] = OnlineUser{u.Name, u.Session}
		}
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
