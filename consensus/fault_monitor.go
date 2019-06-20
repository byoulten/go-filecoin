package consensus

import (
	"context"
	"github.com/filecoin-project/go-filecoin/types"
)

// TSIter is an iterator over a TipSet
type TSIter interface {
	Complete() bool
	Next() error
	Value() types.TipSet
}

// FaultMonitor checks each new tipset for faults
type FaultMonitor struct {
}

// HandleNewTipSet receives an iterator over the current chain, and a new tipset
// and checks the new tipset for fault errors, iteratoring over chnIter
func (fm *FaultMonitor) HandleNewTipSet(ctx context.Context, iter TSIter, newTs types.TipSet) {

}
