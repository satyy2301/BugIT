package archive

import (
	"github.com/bugit/dre-engine/pkg/drearchive"
)

type Snapshot = drearchive.Snapshot

func Open(path, key string) (*Snapshot, error) {
	return drearchive.OpenFile(path, key)
}
