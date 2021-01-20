package baconclient

import (
	"sync"

	"github.com/goat-systems/go-tezos/v4/rpc"
)

type BaconClient struct {
	Current *rpc.Client
	Primary *rpc.Client
	Backup  *rpc.Client

	IsPrimary bool
	lock sync.Mutex
}

func (r *BaconClient) UseBackup() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.Current = r.Backup
	r.IsPrimary = false
}

func (r *BaconClient) UsePrimary() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.Current = r.Primary
	r.IsPrimary = true
}
