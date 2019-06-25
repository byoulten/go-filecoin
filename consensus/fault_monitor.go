package consensus

import (
	"context"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

// TSIter is an iterator over a TipSet
type TSIter interface {
	Complete() bool
	Next() error
	Value() types.TipSet
}

// MonitorPorcelain is an interface for the functionality StorageFaultMonitor needs
type MonitorPorcelain interface {
	GetProvingPeriod(exec.VMContext) (*types.BlockHeight, *types.BlockHeight, uint8, error)
}

// StorageFaultMonitor checks each new tipset for storage faults, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type StorageFaultMonitor struct {
	ctx  exec.VMContext
	porc MonitorPorcelain
	log  logging.EventLogger
}

// NewStorageFaultMonitor creates a new StorageFaultMonitor with the provided porcelain
func NewStorageFaultMonitor(porcelain MonitorPorcelain) *StorageFaultMonitor {
	return &StorageFaultMonitor{
		porc: porcelain,
		log:  logging.Logger("StorageFaultMonitor"),
	}
}

// HandleNewTipSet receives an iterator over the current chain, and a new tipset
// and checks the new tipset for fault errors, iterating over iter
func (sfm *StorageFaultMonitor) HandleNewTipSet(ctx context.Context, iter TSIter, newTs types.TipSet) error {
	//var err error
	// iterate over blocks in the new tipset and detect faults
	head := iter.Value()
	for i := 0; i < head.Len(); i++ {
		blk := head.At(i)
		for _, msg := range blk.Messages {
			m := msg.MeteredMessage.Method
			switch m {
			case "submitPost":
				sfm.log.Debug("GOT submitPoSt message")
				// check for late submission
				// check for submission before generation attack threshold
				// check for missing sectors
				// check for early sector removal
			default:
				continue
			}
		}
	}
	// iterate in the rest of the chain and check head against rest of chain for faults
	//ts := iter.Next()
	//for ts := iter.Value(); !iter.Complete(); err = iter.Next() {
	//	if err != nil {
	//		return err
	//	}
	//}
	return nil
}

func (sfm *StorageFaultMonitor) getProvingPeriod() (*types.BlockHeight, *types.BlockHeight, uint8, error) {
	return sfm.porc.GetProvingPeriod(sfm.ctx)
}
