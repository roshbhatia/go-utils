package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
)

// Keyed names the per-repository entry for root under dir as
// <dir>/<base>-<first 16 hex of sha256(root)>. The base keeps the name
// readable; the digest keeps two checkouts of the same repository apart.
// Readers outside Go address the same layout, so the bytes must not change.
func Keyed(dir, root string) string {
	sum := sha256.Sum256([]byte(root))
	digest := hex.EncodeToString(sum[:])[:16]
	return fmt.Sprintf("%s/%s-%s", dir, filepath.Base(root), digest)
}
