package shanhu

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"time"
)

type states struct {
	key     []byte
	timeNow func() time.Time
}

func newStates(key []byte) *states {
	if key == nil {
		key = make([]byte, 32)
		_, err := rand.Read(key)
		if err != nil {
			panic(err)
		}
	}

	return &states{key: key}
}

const timestampLen = 8

func (s *states) now() time.Time {
	if s.timeNow == nil {
		return time.Now()
	}
	return s.timeNow()
}

func (s *states) hash(ts []byte) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write(ts)
	return m.Sum(nil)
}

func (s *states) New() string {
	buf := make([]byte, timestampLen+sha256.Size)
	ts := buf[:timestampLen]
	now := s.now().UnixNano()
	binary.LittleEndian.PutUint64(ts, uint64(now))
	h := s.hash(ts)
	copy(buf[timestampLen:], h) // append the hash to the end
	return base64.URLEncoding.EncodeToString(buf)
}

var stateTTL = time.Minute * 3

func (s *states) Check(state string) bool {
	bs, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return false
	}

	ts := bs[:timestampLen]
	got := bs[timestampLen:]
	want := s.hash(ts)
	if !hmac.Equal(got, want) {
		return false
	}
	t := int64(binary.LittleEndian.Uint64(ts))
	if t < 0 {
		return false
	}
	return s.now().Before(time.Unix(0, t).Add(stateTTL))
}
