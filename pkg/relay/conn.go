package relay

import "net"

// SessionConn implements net.Conn. It is used to read relay packets.
type SessionConn struct {
	net.Conn
	id uint32
}

// Read reads a packet of len(buf) bytes from the relay packet
func (c *SessionConn) Read(buf []byte) (int, error) {
	relayBuf := make([]byte, headerLength+len(buf))

	// Unmarshal relay packet
	p := &Packet{}
	if err := p.Unmarshal(relayBuf); err != nil {
		return 0, err
	}

	copy(buf, p.Payload)

	return len(p.Payload), nil
}

// Write writes relay packet to the conn
func (c *SessionConn) Write(buf []byte) (n int, err error) {
	p := &Packet{
		Header: Header{
			SessionID: c.id,
		},
		Payload: buf,
	}

	bin, err := p.Marshal()
	if err != nil {
		return 0, err
	}

	return c.Conn.Write(bin)
}

// ID returns session id for conn
func (c *SessionConn) ID() uint32 {
	return c.id
}
