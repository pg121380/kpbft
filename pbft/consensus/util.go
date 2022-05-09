package consensus

import (
	"github.com/tjfoc/gmsm/sm3"
	"encoding/hex"
)

func Hash(content []byte) string {
	h := sm3.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

