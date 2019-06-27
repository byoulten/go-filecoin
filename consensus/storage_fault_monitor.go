package consensus

import (
	"context"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
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
	MinerGetProvingPeriod(context.Context, address.Address) (*types.BlockHeight, *types.BlockHeight, error)
	MinerGetGenerationAttackThreshold(context.Context, address.Address) (*types.BlockHeight, error)
}

// StorageFaultMonitor checks each new tipset for storage faults, a.k.a. market faults.
// Storage faults are distinct from consensus faults.
// See https://github.com/filecoin-project/specs/blob/master/faults.md
type StorageFaultMonitor struct {
	minerAddr address.Address
	porc      MonitorPorcelain
	pdStart   *types.BlockHeight
	pdEnd     *types.BlockHeight
	gat       *types.BlockHeight
	log       logging.EventLogger
}

// NewStorageFaultMonitor creates a new StorageFaultMonitor with the provided porcelain
func NewStorageFaultMonitor(porcelain MonitorPorcelain, minerAddr address.Address) *StorageFaultMonitor {
	return &StorageFaultMonitor{
		minerAddr: minerAddr,
		porc:      porcelain,
		log:       logging.Logger("StorageFaultMonitor"),
	}
}

// HandleNewTipSet receives an iterator over the current chain, and a new tipset
// and checks the new tipset for fault errors, iterating over iter
func (sfm *StorageFaultMonitor) HandleNewTipSet(ctx context.Context, iter TSIter, newTs types.TipSet) error {
	var err error

	// iterate over blocks in the new tipset and detect faults
	head := iter.Value()
	bh, err := head.Height()
	if err != nil {
		return err
	}

	sfm.pdStart, sfm.pdEnd, err = sfm.porc.MinerGetProvingPeriod(ctx, sfm.minerAddr)
	if err != nil {
		return err
	}

	sfm.gat, err = sfm.porc.MinerGetGenerationAttackThreshold(ctx, sfm.minerAddr)
	if err != nil {
		return err
	}

	for i := 0; i < head.Len(); i++ {
		blk := head.At(i)
		for _, msg := range blk.Messages {
			m := msg.MeteredMessage.Method
			miner := msg.MeteredMessage.From
			switch m {
			case "submitPost":
				sfm.log.Debug("GOT submitPoSt message")
				missing, err := isMissingProof(&msg.MeteredMessage)
				if err != nil {
					return err
				}
				if missing {
					sfm.log.Debug("submitPost message missing proof at blockheight %d for miner %s", bh, miner.String())
				}

				//curHeight := types.NewBlockHeight(bh)

				// check provided proof(s) for late submission
				//lateSubmissionLimit := curHeight.Sub(sfm.pdStart)
				//lastSeen, err := MinerLastSeen(miner, iter, lateSubmissionLimit)
				//if err != nil {
				//	return err
				//}
				//if lastSeen == nil {
				//	sfm.log.Debug("submitPost message submitted late")
				//}

				// check for submission before generation attack threshold
				// this continues the iterator where we left off and looks an additional
				// generation-attack-threshold block height.
				//lastSeen, err = MinerLastSeen(miner, iter, sfm.gat)
				//if err != nil {
				//	return err
				//}
				//if lastSeen == nil {
				//	sfm.log.Debug("submitPoSt not seen within generation attack threshold")
				//}

				// check for missing sectors  -- how?
				// check for early sector removal -- how?
			default:
				continue
			}
		}
	}
	return nil
}

func isMissingProof(msg *types.MeteredMessage) (bool, error) {
	params, err := abi.DecodeValues(msg.Params, []abi.Type{abi.Parameters})
	if err != nil {
		return false, err
	}
	if len(params) == 0 {
		return true, nil
	}
	return false, nil
}

// MinerLastSeen returns the block height at which miner last sent a `submitPost` message, or
// nil if it was not seen within lookBackLimit blocks ago, not counting the head.
func MinerLastSeen(miner address.Address, iter TSIter, lookBackLimit *types.BlockHeight) (*types.BlockHeight, error) {
	// iterate in the rest of the chain and check head against rest of chain for faults
	var err error

	for i := uint64(0); !iter.Complete() && lookBackLimit.GreaterThan(types.NewBlockHeight(i)); i++ {
		if err = iter.Next(); err != nil {
			return nil, err
		}

		ts := iter.Value()
		for j := 0; j < ts.Len(); j++ {
			blk := ts.At(j)
			if msg := poStMessageFrom(miner, blk.Messages); msg != nil {
				h, err := ts.Height()
				if err != nil {
					return nil, err
				}
				return types.NewBlockHeight(h), nil
			}
		}
	}
	return nil, nil
}

func poStMessageFrom(miner address.Address, msgs []*types.SignedMessage) (msg *types.SignedMessage) {
	for _, msg = range msgs {
		meth := msg.MeteredMessage.Method
		from := msg.MeteredMessage.From.String()
		if from == miner.String() && meth == "submitPost" {
			return msg
		}
	}
	return nil
}
