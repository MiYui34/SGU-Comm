package mumbled

import (
	"errors"
	"slices"
)

// Captcha (Kallisti, not Mumble): a saved server setting, changed by
// admins: how much work a browser does before it creates an account or a
// guest account (the web server's auth checks it). Mumble clients do none.

// CaptchaLevels, saved by index.
var CaptchaLevels = []string{"off", "normal", "hard"}

// Captcha reports the setting's level.
func (s *Server) Captcha() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CaptchaLevels[s.state.Captcha]
}

// SetCaptcha changes the level (admins only) and saves it.
func (s *Server) SetCaptcha(actor uint32, level string) error {
	i := slices.Index(CaptchaLevels, level)
	if i < 0 {
		return errors.New("unknown level")
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	u := s.state.Users[actor]
	admin := u != nil && s.isAdmin(u)
	s.mu.Unlock()
	if !admin {
		return errors.New("only admins change this setting")
	}
	if s.cfg.DB != nil {
		if err := s.cfg.DB.setMeta("captcha", uint32(i)); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.state.Captcha = uint32(i)
	s.mu.Unlock()
	return nil
}
