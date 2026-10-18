package sidecar

// Connection is one connection over the recent window, for the admin view
// (see mumbled.Connection, which converts to it).
type Connection struct {
	Session       uint32   `json:"session"`
	Name          string   `json:"name"`
	Transport     string   `json:"transport"`
	Ping          *float64 `json:"ping"`       // ms (null: none measured)
	FromClient    *float64 `json:"fromClient"` // late or lost voice in percent (null: too little voice)
	FromServer    *float64 `json:"fromServer"`
	Dropped       uint64   `json:"dropped"`
	TextOverflows uint64   `json:"overflows"`
	Online        int64    `json:"online"` // seconds
	Client        string   `json:"client"`
	OS            string   `json:"os"`
}

// connections answers an admin's poll of the connections table.
func (c *Conn) connections(session uint32) {
	if c.hub.voice == nil {
		c.manageError("connections need the built-in Mumble server")
		return
	}
	conns, err := c.hub.voice.Connections(session)
	if err != nil {
		c.manageError(err.Error())
		return
	}
	if conns == nil {
		conns = []Connection{}
	}
	c.send(map[string]any{"type": "connections", "connections": conns})
}
