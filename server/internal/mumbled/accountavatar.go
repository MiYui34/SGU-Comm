package mumbled

import (
	"log/slog"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

// SetAccountAvatar gives a web account an avatar from the server (its
// fediverse account's), as if its user had set it: an account that never
// visited is registered now (as on its first visit, see identifyAccount).
// onlyIfNone: an avatar it has stays. set: the avatar was set. Only
// allowed pictures are taken (see avatarType).
func (s *Server) SetAccountAvatar(name, certHash string, texture []byte, onlyIfNone bool) (set bool, err error) {
	db := s.cfg.DB
	if db == nil || certHash == "" || avatarType(texture) == "" || len(texture) > maxTexture {
		return false, nil
	}
	// Under regMu: a first visit reads the saved avatar after this one.
	s.regMu.Lock()
	defer s.regMu.Unlock()
	r, err := db.UserByHash(certHash)
	if err != nil {
		return false, err
	}
	if r == nil {
		if other, err := db.UserByName(name); err != nil || other != nil {
			if other != nil {
				slog.Warn("a web account's name is registered by a Mumble user", "account", name, "user", other.ID)
			}
			return false, err
		}
		id, err := db.RegisterUser(name, certHash)
		if err != nil {
			return false, err
		}
		s.userChanged(id, name)
		r = &Registered{ID: id, Name: name}
	}
	s.saveMu.Lock() // saves keep the order of the changes
	defer s.saveMu.Unlock()
	s.mu.Lock()
	var online []*User
	for _, u := range s.state.Users {
		if u.Registered && u.UserID == r.ID {
			online = append(online, u)
		}
	}
	if len(online) == 0 {
		s.mu.Unlock()
		comment, old, err := db.UserContent(r.ID)
		if err != nil || onlyIfNone && len(old) > 0 {
			return false, err
		}
		s.saveContent(content{registered: true, userID: r.ID, comment: comment, texture: texture, newTexture: true})
		return true, nil
	}
	if onlyIfNone && len(online[0].Texture) > 0 {
		s.mu.Unlock()
		return false, nil
	}
	m := &pb.UserState{Texture: texture}
	for _, u := range online {
		out := &pb.UserState{Session: proto.Uint32(u.Session)}
		applyContent(u, m, out)
		s.sendUserLocked(u, out)
	}
	saved := contentOf(online[0], true)
	s.mu.Unlock()
	s.saveContent(saved)
	return true, nil
}
