package consensus

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// TSIter is an iterator over a TipSet
type TSIter interface {
	Complete() bool
	Next() error
	Value() types.TipSet
}

// monitorPlumbing is an interface for the functionality StorageFaultMonitor needs
type monitorPlumbing interface {
	MessageQuery(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error)
}

// StorageFaultMonitor checks each new tipset for storage faults, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type StorageFaultMonitor struct {
	log       logging.EventLogger
	msgSender address.Address
	porc      monitorPlumbing
}

// NewStorageFaultMonitor creates a new StorageFaultMonitor with the provided porcelain and function
// to get miner power
func NewStorageFaultMonitor(porcelain monitorPlumbing, msgSender address.Address) *StorageFaultMonitor {
	return &StorageFaultMonitor{
		porc:      porcelain,
		log:       logging.Logger("StorageFaultMonitor"),
		msgSender: msgSender,
	}
}

// HandleNewTipSet receives an iterator over the current chain, and a new tipset
// and looks for missing, expected submitPoSts
// Miners without power and those that posted proofs to newTs are skipped
func (sfm *StorageFaultMonitor) HandleNewTipSet(ctx context.Context, currentHeight *types.BlockHeight) error {
	res, err := sfm.porc.MessageQuery(ctx, sfm.msgSender, address.StorageMarketAddress, "getLateMiners", nil)
	if err != nil {
		return errors.FaultErrorWrap(err, "")
	}

	lateMiners, err := abi.Deserialize(res[0], abi.MinerPoStStates)
	if err != nil {
		return errors.FaultErrorWrap(err, "")
	}

	lms, ok := lateMiners.Val.(*map[string]uint64)
	if !ok {
		return errors.FaultErrorWrapf(err, "expected *map[string]uint64 but got %T", lms)
	}

	var errs []error
	// Slash late miners.
	for minerStr := range *lms {
		_, err := address.NewFromString(minerStr)
		if err != nil {
			errs = append(errs, errors.FaultErrorWrap(err, fmt.Sprintf("failed to convert %s to address.Address", minerStr)))
			continue
		}
		// TODO ensure toAddr is a signing addr
		// TODO use a non-bcast sending functionality
		//		_, err = sfm.porc.MessageSend(ctx, sfm.msgSender, toAddr, types.ZeroAttoFIL, types.ZeroAttoFIL,
		//types.NewGasUnits(0), "slashStorageFault", nil)
		if err != nil {
			// slashStorageFault messages are not broadcast, so we expect miners may receive
			// >1 slash message per fault. Either way, account actor doesn't get paid, but
			// don't log an error for MinerAlreadySlashed.
			if code != miner.ErrMinerAlreadySlashed {
				errs = append(errs, errors.FaultErrorWrap(err, ""))
			}
		}
	}
	return nil
}
