package consensus

import (
	"encoding/hex"

	"github.com/tjfoc/gmsm/sm3"
)

func Hash(content []byte) string {
	h := sm3.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}
